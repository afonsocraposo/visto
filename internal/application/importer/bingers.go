package importer

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"path"
	"strconv"
	"strings"
	"time"
)

const MaxUpload = 5 << 20
const maxExtracted = 25 << 20
const maxRows = 20000

type Title struct {
	Type, Name, OriginalTitle, Year, TVDBID, Status string
	TMDBID                                          int64
	AddedAt                                         time.Time
}

type Watch struct {
	Type, Name, TVDBID string
	TMDBID             int64
	Season, Episode    int
	WatchedAt          time.Time
}

type Rating struct {
	Type, Name, TVDBID     string
	TMDBID                 int64
	Season, Episode, Value int
}

type Issue struct {
	Item   string `json:"item"`
	Reason string `json:"reason"`
}

type Data struct {
	Titles      []Title
	Watches     []Watch
	Ratings     []Rating
	Unsupported int
	Issues      []Issue
}

type Result struct {
	Titles      int     `json:"titles"`
	Watches     int     `json:"watches"`
	Ratings     int     `json:"ratings"`
	Skipped     int     `json:"skipped"`
	Unsupported int     `json:"unsupported"`
	Issues      []Issue `json:"issues"`
}

func (d *Data) skip(item, reason string) {
	d.Issues = append(d.Issues, Issue{Item: item, Reason: reason})
}

func Parse(source string, payload []byte) (Data, error) {
	if source != "bingers" {
		return Data{}, fmt.Errorf("unsupported import source")
	}
	if len(payload) == 0 || len(payload) > MaxUpload {
		return Data{}, fmt.Errorf("archive must be 5 MB or smaller")
	}
	archive, err := zip.NewReader(bytes.NewReader(payload), int64(len(payload)))
	if err != nil {
		return Data{}, fmt.Errorf("invalid ZIP archive")
	}
	files := map[string][][]string{}
	total := uint64(0)
	expanded := 0
	for _, file := range archive.File {
		if file.FileInfo().IsDir() || path.Base(file.Name) != file.Name {
			return Data{}, fmt.Errorf("archive must contain CSV files at its root")
		}
		if file.Name != "library.csv" && file.Name != "watches.csv" && file.Name != "ratings.csv" && file.Name != "lists.csv" {
			return Data{}, fmt.Errorf("unexpected archive file %q", file.Name)
		}
		if _, exists := files[file.Name]; exists {
			return Data{}, fmt.Errorf("duplicate archive file %q", file.Name)
		}
		total += file.UncompressedSize64
		if total > maxExtracted {
			return Data{}, fmt.Errorf("archive expands beyond 25 MB")
		}
		reader, err := file.Open()
		if err != nil {
			return Data{}, fmt.Errorf("open %s: %w", file.Name, err)
		}
		contents, err := io.ReadAll(io.LimitReader(reader, maxExtracted+1))
		reader.Close()
		if err != nil || len(contents) > maxExtracted {
			return Data{}, fmt.Errorf("archive expands beyond 25 MB")
		}
		expanded += len(contents)
		if expanded > maxExtracted {
			return Data{}, fmt.Errorf("archive expands beyond 25 MB")
		}
		csvReader := csv.NewReader(bytes.NewReader(contents))
		csvReader.FieldsPerRecord = -1
		rows, err := csvReader.ReadAll()
		if err != nil {
			return Data{}, fmt.Errorf("invalid %s: %w", file.Name, err)
		}
		if len(rows) == 0 || len(rows) > maxRows+1 {
			return Data{}, fmt.Errorf("invalid row count in %s", file.Name)
		}
		rows[0][0] = strings.TrimPrefix(rows[0][0], "\ufeff")
		files[file.Name] = rows
	}
	if files["library.csv"] == nil || files["watches.csv"] == nil {
		return Data{}, fmt.Errorf("archive needs library.csv and watches.csv")
	}
	d := Data{Titles: []Title{}, Watches: []Watch{}, Ratings: []Rating{}, Issues: []Issue{}}
	for _, name := range []string{"library.csv", "watches.csv", "ratings.csv", "lists.csv"} {
		rows := files[name]
		if rows == nil {
			continue
		}
		header := map[string]int{}
		for i, column := range rows[0] {
			header[column] = i
		}
		required := map[string][]string{
			"library.csv": {"type", "title", "tmdb_id", "tvdb_id", "list_status", "added_at"},
			"watches.csv": {"type", "title", "tmdb_id", "tvdb_id", "season_number", "episode_number", "last_watched_at"},
			"ratings.csv": {"type", "title", "tmdb_id", "tvdb_id", "season_number", "episode_number", "rating"},
			"lists.csv":   {"list_name", "title", "type"},
		}[name]
		for _, column := range required {
			if _, ok := header[column]; !ok {
				return Data{}, fmt.Errorf("%s is missing %s", name, column)
			}
		}
		get := func(row []string, key string) string {
			i, ok := header[key]
			if !ok || i >= len(row) {
				return ""
			}
			return strings.TrimSpace(row[i])
		}
		for index, row := range rows[1:] {
			label := fmt.Sprintf("%s row %d", name, index+2)
			if name == "lists.csv" {
				d.Unsupported++
				continue
			}
			kind := get(row, "type")
			if kind == "show" {
				kind = "tv"
			}
			if kind != "movie" && kind != "tv" && kind != "episode" {
				d.skip(label, "unsupported media type")
				continue
			}
			id, err := strconv.ParseInt(get(row, "tmdb_id"), 10, 64)
			if get(row, "tmdb_id") == "" {
				id = 0
				err = nil
			}
			if err != nil || id < 0 {
				d.skip(label, "invalid TMDB ID")
				continue
			}
			nameValue := get(row, "title")
			if nameValue == "" {
				d.skip(label, "missing title")
				continue
			}
			switch name {
			case "library.csv":
				if kind == "episode" {
					d.skip(label, "episode is not a library title")
					continue
				}
				status := get(row, "list_status")
				switch status {
				case "watching":
					if kind == "movie" {
						status = "watchlist"
					}
				case "stopped":
					status = "dropped"
				case "for_later", "watchlist":
					status = "watchlist"
				case "paused", "dropped", "completed":
				default:
					d.skip(label, "unsupported library state")
					continue
				}
				if kind == "movie" && status != "watchlist" && status != "completed" {
					d.skip(label, "unsupported movie state")
					continue
				}
				added, err := time.Parse(time.RFC3339Nano, get(row, "added_at"))
				if err != nil {
					d.skip(label, "invalid added date")
					continue
				}
				d.Titles = append(d.Titles, Title{Type: kind, Name: nameValue, OriginalTitle: get(row, "original_title"), Year: get(row, "year"), TVDBID: get(row, "tvdb_id"), TMDBID: id, Status: status, AddedAt: added})
			case "watches.csv":
				season, episode := 0, 0
				if kind == "episode" {
					season, err = strconv.Atoi(get(row, "season_number"))
					if err == nil {
						episode, err = strconv.Atoi(get(row, "episode_number"))
					}
					if err != nil || season < 0 || episode <= 0 {
						d.skip(label, "invalid episode number")
						continue
					}
				} else if kind != "movie" {
					d.skip(label, "unsupported watch type")
					continue
				}
				watched, err := time.Parse(time.RFC3339Nano, get(row, "last_watched_at"))
				if err != nil || watched.After(time.Now().Add(time.Minute)) {
					d.skip(label, "invalid watch date")
					continue
				}
				d.Watches = append(d.Watches, Watch{Type: kind, Name: nameValue, TVDBID: get(row, "tvdb_id"), TMDBID: id, Season: season, Episode: episode, WatchedAt: watched})
			case "ratings.csv":
				value, err := strconv.Atoi(get(row, "rating"))
				if err != nil || value < 1 || value > 5 {
					d.skip(label, "rating must be 1–5")
					continue
				}
				season, episode := 0, 0
				if kind == "episode" {
					season, err = strconv.Atoi(get(row, "season_number"))
					if err == nil {
						episode, err = strconv.Atoi(get(row, "episode_number"))
					}
					if err != nil || season < 0 || episode <= 0 {
						d.skip(label, "invalid episode number")
						continue
					}
				}
				d.Ratings = append(d.Ratings, Rating{Type: kind, Name: nameValue, TVDBID: get(row, "tvdb_id"), TMDBID: id, Season: season, Episode: episode, Value: value})
			}
		}
	}
	return d, nil
}
