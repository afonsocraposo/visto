package pagination

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
)

const (
	DefaultLimit = 30
	MaxLimit     = 100
)

var ErrInvalidCursor = errors.New("invalid cursor")
var ErrInvalidLimit = errors.New("limit must be from 1 to 100")

// Request describes one bounded keyset page.
type Request struct {
	Limit  int
	Cursor string
}
type Page[T any] struct {
	Items      []T     `json:"items"`
	NextCursor *string `json:"next_cursor"`
	TotalCount *int    `json:"total_count,omitempty"`
}
type Position struct {
	Version int      `json:"v"`
	Scope   string   `json:"s"`
	Keys    []string `json:"k"`
}

func Parse(r *http.Request) (Request, error) {
	limit := DefaultLimit
	if raw := r.URL.Query().Get("limit"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || value > MaxLimit {
			return Request{}, ErrInvalidLimit
		}
		limit = value
	}
	return Request{Limit: limit, Cursor: r.URL.Query().Get("cursor")}, nil
}
func Encode(scope string, keys ...string) string {
	data, _ := json.Marshal(Position{Version: 1, Scope: scope, Keys: keys})
	return base64.RawURLEncoding.EncodeToString(data)
}
func Decode(cursor, scope string, n int) ([]string, error) {
	if cursor == "" {
		return nil, nil
	}
	data, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return nil, ErrInvalidCursor
	}
	var p Position
	if err = json.Unmarshal(data, &p); err != nil || p.Version != 1 || p.Scope != scope || len(p.Keys) != n {
		return nil, ErrInvalidCursor
	}
	return p.Keys, nil
}
func Slice[T any](items []T, limit int, cursor func(T) string) Page[T] {
	page := Page[T]{Items: items}
	if len(items) > limit {
		page.Items = items[:limit]
		next := cursor(page.Items[limit-1])
		page.NextCursor = &next
	}
	return page
}
