package detectors

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ttime-ai/ttime/client/internal/scanner"
)

type workBuddyRecord struct {
	Type      string         `json:"type"`
	Role      string         `json:"role"`
	Content   any            `json:"content"`
	CWD       string         `json:"cwd"`
	Timestamp int64          `json:"timestamp"`
	Model     string         `json:"model"`
	Name      string         `json:"name"`
	CallID    string         `json:"callId"`
	Arguments map[string]any `json:"arguments"`
	Usage     struct {
		Input      int `json:"input"`
		Output     int `json:"output"`
		CacheRead  int `json:"cacheRead"`
		CacheWrite int `json:"cacheWrite"`
	} `json:"usage"`
}

func collectWorkBuddyGenericSummaries(ctx context.Context, root string) ([]agentsViewGenericSummary, error) {
	paths, err := collectWorkBuddySessionFiles(ctx, root)
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
		if summary, ok := summarizeWorkBuddySession(path); ok {
			summaries = append(summaries, summary)
		}
	}
	return summaries, nil
}

func collectWorkBuddySessionFiles(ctx context.Context, root string) ([]string, error) {
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
		if strings.EqualFold(filepath.Ext(entry.Name()), ".jsonl") {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk workbuddy sessions: %w", err)
	}
	sort.Strings(paths)
	return paths, nil
}

func summarizeWorkBuddySession(path string) (agentsViewGenericSummary, bool) {
	file, err := os.Open(path)
	if err != nil {
		return agentsViewGenericSummary{}, false
	}
	defer file.Close()

	summary := agentsViewGenericSummary{
		SessionID: workBuddySummaryID(path),
		Project:   projectNameFromPath(filepath.Dir(path)),
		FileEdits: make(map[string]scanner.FileEdit),
	}
	hasContent := false
	lineScanner := bufio.NewScanner(file)
	lineScanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for lineScanner.Scan() {
		line := strings.TrimSpace(lineScanner.Text())
		if line == "" {
			continue
		}
		var record workBuddyRecord
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			continue
		}
		observeWorkBuddyRecord(record, &summary, &hasContent)
	}
	if summary.Project == "" {
		summary.Project = projectNameFromPath(summary.CWD)
	}
	if summary.TotalTokens == 0 {
		summary.TotalTokens = summary.PromptTokens + summary.CompletionTokens
	}
	if info, err := os.Stat(path); err == nil {
		if summary.StartedAt.IsZero() {
			summary.StartedAt = info.ModTime().UTC()
		}
		if summary.EndedAt.IsZero() {
			summary.EndedAt = info.ModTime().UTC()
		}
	}
	if summary.SessionID == "" || summary.EndedAt.IsZero() || !hasContent {
		return agentsViewGenericSummary{}, false
	}
	return summary, true
}

func observeWorkBuddyRecord(record workBuddyRecord, summary *agentsViewGenericSummary, hasContent *bool) {
	if summary.CWD == "" {
		summary.CWD = strings.TrimSpace(record.CWD)
	}
	if summary.Project == "" {
		summary.Project = projectNameFromPath(summary.CWD)
	}
	if record.Timestamp > 0 {
		ts := unixFlexible(record.Timestamp)
		if summary.StartedAt.IsZero() || ts.Before(summary.StartedAt) {
			summary.StartedAt = ts
		}
		if ts.After(summary.EndedAt) {
			summary.EndedAt = ts
		}
	}
	switch record.Type {
	case "message":
		if strings.TrimSpace(workBuddyText(record.Content)) != "" {
			*hasContent = true
		}
	case "function_call":
		*hasContent = true
		if summary.Model == "" {
			summary.Model = strings.TrimSpace(record.Model)
		}
		mergeFileEdits(summary.FileEdits, fileEditsFromToolCall(record.Name, record.Arguments))
		summary.PromptTokens += record.Usage.Input + record.Usage.CacheRead + record.Usage.CacheWrite
		summary.CompletionTokens += record.Usage.Output
		summary.TotalTokens += record.Usage.Input + record.Usage.CacheRead + record.Usage.CacheWrite + record.Usage.Output
	}
}

func workBuddySummaryID(path string) string {
	stem := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	if filepath.Base(filepath.Dir(path)) != "subagents" {
		return stem
	}
	parent := filepath.Base(filepath.Dir(filepath.Dir(path)))
	return parent + ":subagent:" + stem
}

func workBuddyText(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case []any:
		var parts []string
		for _, item := range typed {
			block, ok := item.(map[string]any)
			if !ok {
				continue
			}
			for _, key := range []string{"text", "input_text", "output_text"} {
				if text := strings.TrimSpace(stringValue(block[key])); text != "" {
					parts = append(parts, text)
					break
				}
			}
		}
		return strings.Join(parts, "\n")
	default:
		return ""
	}
}
