package domain

import (
	"cmp"
	"slices"
	"time"
)

type ShowProgress struct {
	Cursor          *Episode
	NextEpisode     *Episode
	ReleasedEpisode *Episode
	IsCaughtUp      bool
}

// CalculateShowProgress derives progress directly from plays. It deliberately
// does not persist a cursor: correcting a play must always correct progress.
func CalculateShowProgress(episodes []Episode, plays []EpisodePlay, now time.Time) ShowProgress {
	played := make(map[string]struct{}, len(plays))
	for _, play := range plays {
		played[play.EpisodeID] = struct{}{}
	}

	regular := make([]Episode, 0, len(episodes))
	for _, episode := range episodes {
		if episode.IsRegular() {
			regular = append(regular, episode)
		}
	}
	slices.SortFunc(regular, compareEpisode)

	result := ShowProgress{IsCaughtUp: true}
	for index := range regular {
		episode := &regular[index]
		if _, ok := played[episode.ID]; ok {
			if result.Cursor == nil || compareEpisode(*result.Cursor, *episode) < 0 {
				result.Cursor = episode
			}
		}
		if episode.IsReleasedAt(now) {
			result.ReleasedEpisode = episode
		}
	}

	for index := range regular {
		episode := &regular[index]
		if !episode.IsReleasedAt(now) || result.Cursor != nil && compareEpisode(*episode, *result.Cursor) <= 0 {
			continue
		}
		result.NextEpisode = episode
		result.IsCaughtUp = false
		break
	}

	return result
}

// MissingPriorEpisodes returns released regular episodes before target that do
// not have a play. The application uses it to ask before a bulk mark action.
func MissingPriorEpisodes(episodes []Episode, plays []EpisodePlay, target Episode, now time.Time) []Episode {
	return missingEpisodesThrough(episodes, plays, target, now, false)
}

// MissingEpisodesThrough includes the target when it has not been watched.
func MissingEpisodesThrough(episodes []Episode, plays []EpisodePlay, target Episode, now time.Time) []Episode {
	return missingEpisodesThrough(episodes, plays, target, now, true)
}

func missingEpisodesThrough(episodes []Episode, plays []EpisodePlay, target Episode, now time.Time, includeTarget bool) []Episode {
	played := make(map[string]struct{}, len(plays))
	for _, play := range plays {
		played[play.EpisodeID] = struct{}{}
	}

	missing := make([]Episode, 0)
	for _, episode := range episodes {
		order := compareEpisode(episode, target)
		if !episode.IsRegular() || !episode.IsReleasedAt(now) || order > 0 || order == 0 && !includeTarget {
			continue
		}
		if _, ok := played[episode.ID]; !ok {
			missing = append(missing, episode)
		}
	}
	slices.SortFunc(missing, compareEpisode)
	return missing
}

func compareEpisode(left, right Episode) int {
	if difference := cmp.Compare(left.SeasonNumber, right.SeasonNumber); difference != 0 {
		return difference
	}
	return cmp.Compare(left.EpisodeNumber, right.EpisodeNumber)
}
