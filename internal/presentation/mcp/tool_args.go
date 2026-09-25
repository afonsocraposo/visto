package mcp

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/afonsocosta/visto/internal/domain"
)

func stringArg(args map[string]any, key string) string {
	value, _ := args[key].(string)
	return strings.TrimSpace(value)
}

func intArg(args map[string]any, key string) int {
	switch value := args[key].(type) {
	case json.Number:
		parsed, _ := value.Int64()
		return int(parsed)
	case float64:
		return int(value)
	case int:
		return value
	default:
		return 0
	}
}

func stringListArg(args map[string]any, key string, maximum int) ([]string, error) {
	values, ok := args[key].([]any)
	if !ok || len(values) == 0 {
		return nil, fmt.Errorf("%s must be a non-empty list of IDs", key)
	}
	if maximum > 0 && len(values) > maximum {
		return nil, fmt.Errorf("%s must contain no more than %d IDs", key, maximum)
	}
	result := make([]string, 0, len(values))
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		id, ok := value.(string)
		if !ok || strings.TrimSpace(id) == "" {
			return nil, fmt.Errorf("%s must contain non-empty string IDs", key)
		}
		id = strings.TrimSpace(id)
		if seen[id] {
			return nil, fmt.Errorf("%s must contain unique IDs", key)
		}
		seen[id] = true
		result = append(result, id)
	}
	return result, nil
}

func dateArg(args map[string]any, key string) (time.Time, error) {
	value := stringArg(args, key)
	if value == "" {
		return time.Time{}, nil
	}
	parsed, err := time.Parse(time.DateOnly, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("%s must use YYYY-MM-DD", key)
	}
	return parsed, nil
}

func timestampArg(args map[string]any, key string) (time.Time, error) {
	value := stringArg(args, key)
	if value == "" {
		return time.Time{}, nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("%s must be an RFC3339 timestamp", key)
	}
	return parsed, nil
}

func ratingArg(args map[string]any) (*int, error) {
	if args["rating"] == nil {
		return nil, nil
	}
	value := intArg(args, "rating")
	if value < 1 || value > 5 {
		return nil, fmt.Errorf("rating must be from 1 to 5")
	}
	return &value, nil
}

func mediaArg(args map[string]any) (domain.MediaSearchResult, error) {
	var item domain.MediaSearchResult
	data, err := json.Marshal(args["media"])
	if err != nil || json.Unmarshal(data, &item) != nil {
		return item, fmt.Errorf("media must be a result returned by search_media")
	}
	if item.TMDBID <= 0 || item.Title == "" || (item.Type != domain.MovieMediaType && item.Type != domain.TVMediaType) {
		return item, fmt.Errorf("media must include a valid TMDB ID, type, and title")
	}
	return item, nil
}
