package detectors

import (
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func intPtr(v int) *int {
	return &v
}

func timePtr(v time.Time) *time.Time {
	copied := v
	return &copied
}

func durationSeconds(startedAt, endedAt time.Time) int {
	if startedAt.IsZero() || endedAt.IsZero() || endedAt.Before(startedAt) {
		return 0
	}
	return int(endedAt.Sub(startedAt).Round(time.Second).Seconds())
}

func projectNameFromPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	base := filepath.Base(filepath.Clean(path))
	if base == "." || base == string(filepath.Separator) {
		return ""
	}
	return base
}

func parseRFC3339Any(raw string) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return parsed
		}
	}
	return time.Time{}
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case interface{ String() string }:
		return typed.String()
	default:
		return ""
	}
}

func intValue(value any) int {
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case int:
		return typed
	case int64:
		return int(typed)
	case string:
		parsed, _ := strconv.Atoi(strings.TrimSpace(typed))
		return parsed
	default:
		return 0
	}
}

func truncateString(value string, max int) string {
	value = strings.TrimSpace(value)
	if value == "" || max <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= max {
		return value
	}
	if max == 1 {
		return string(runes[:1])
	}
	return string(runes[:max-1]) + "…"
}
