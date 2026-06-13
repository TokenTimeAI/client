package detectors

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func collectCommandCodeGenericSummaries(ctx context.Context, root string) ([]agentsViewGenericSummary, error) {
	paths, err := collectCommandCodeSessionFiles(ctx, root)
	if err != nil {
		return nil, err
	}
	summaries := make([]agentsViewGenericSummary, 0, len(paths))
	for _, path := range paths {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		if summary, ok := summarizeAgentsViewGenericSession(path); ok {
			summaries = append(summaries, summary)
		}
	}
	return summaries, nil
}

func collectCommandCodeSessionFiles(ctx context.Context, root string) ([]string, error) {
	paths := make([]string, 0, 16)
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if entry.IsDir() {
			name := entry.Name()
			if name == "node_modules" || strings.HasPrefix(name, ".git") {
				return filepath.SkipDir
			}
			return nil
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".jsonl") ||
			strings.HasSuffix(name, ".checkpoints.jsonl") ||
			strings.HasSuffix(name, ".prompts.jsonl") {
			return nil
		}
		paths = append(paths, path)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk commandcode sessions: %w", err)
	}
	sort.Strings(paths)
	return paths, nil
}
