package detectors

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ttime-ai/ttime/client/internal/scanner"
)

func collectOpenHandsGenericSummaries(ctx context.Context, root string) ([]agentsViewGenericSummary, error) {
	sessionDirs := make([]string, 0, 16)
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if !entry.IsDir() {
			return nil
		}
		if entry.Name() == "events" {
			return filepath.SkipDir
		}
		if scanner.DirExists(filepath.Join(path, "events")) {
			sessionDirs = append(sessionDirs, path)
			return filepath.SkipDir
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk openhands sessions: %w", err)
	}
	sort.Strings(sessionDirs)

	summaries := make([]agentsViewGenericSummary, 0, len(sessionDirs))
	for _, dir := range sessionDirs {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		summary, ok := summarizeOpenHandsGenericSession(dir)
		if ok {
			summaries = append(summaries, summary)
		}
	}
	return summaries, nil
}

func summarizeOpenHandsGenericSession(sessionDir string) (agentsViewGenericSummary, bool) {
	summary := agentsViewGenericSummary{
		SessionID: strings.TrimSpace(filepath.Base(sessionDir)),
		FileEdits: make(map[string]scanner.FileEdit),
	}

	visitGenericJSONFile(filepath.Join(sessionDir, "base_state.json"), &summary)

	eventPaths, err := filepath.Glob(filepath.Join(sessionDir, "events", "*.json"))
	if err == nil {
		sort.Strings(eventPaths)
		for _, path := range eventPaths {
			visitGenericJSONFile(path, &summary)
		}
	}

	if summary.EndedAt.IsZero() {
		if info, err := os.Stat(sessionDir); err == nil {
			summary.EndedAt = info.ModTime().UTC()
		}
	}
	if summary.StartedAt.IsZero() {
		summary.StartedAt = summary.EndedAt
	}
	if summary.SessionID == "" || summary.EndedAt.IsZero() {
		return agentsViewGenericSummary{}, false
	}
	return summary, true
}
