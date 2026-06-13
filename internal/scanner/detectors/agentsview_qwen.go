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

type qwenSessionRecord struct {
	SessionID     string            `json:"sessionId"`
	Timestamp     string            `json:"timestamp"`
	Type          string            `json:"type"`
	CWD           string            `json:"cwd"`
	Model         string            `json:"model"`
	Message       qwenMessage       `json:"message"`
	UsageMetadata qwenUsageMetadata `json:"usageMetadata"`
}

type qwenMessage struct {
	Role  string     `json:"role"`
	Parts []qwenPart `json:"parts"`
}

type qwenPart struct {
	Text             string            `json:"text"`
	Thought          bool              `json:"thought"`
	FunctionCall     *qwenFunctionCall `json:"functionCall"`
	FunctionResponse any               `json:"functionResponse"`
}

type qwenFunctionCall struct {
	ID   string         `json:"id"`
	Name string         `json:"name"`
	Args map[string]any `json:"args"`
}

type qwenUsageMetadata struct {
	PromptTokenCount        int `json:"promptTokenCount"`
	CandidatesTokenCount    int `json:"candidatesTokenCount"`
	CachedContentTokenCount int `json:"cachedContentTokenCount"`
	TotalTokenCount         int `json:"totalTokenCount"`
}

func collectQwenGenericSummaries(ctx context.Context, root string) ([]agentsViewGenericSummary, error) {
	paths, err := collectQwenSessionFiles(ctx, root)
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
		if summary, ok := summarizeQwenSession(path); ok {
			summaries = append(summaries, summary)
		}
	}
	return summaries, nil
}

func collectQwenSessionFiles(ctx context.Context, root string) ([]string, error) {
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
		if filepath.Base(filepath.Dir(path)) == "chats" && strings.EqualFold(filepath.Ext(entry.Name()), ".jsonl") {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk qwen sessions: %w", err)
	}
	sort.Strings(paths)
	return paths, nil
}

func summarizeQwenSession(path string) (agentsViewGenericSummary, bool) {
	file, err := os.Open(path)
	if err != nil {
		return agentsViewGenericSummary{}, false
	}
	defer file.Close()

	summary := agentsViewGenericSummary{
		FileEdits: make(map[string]scanner.FileEdit),
	}
	lineScanner := bufio.NewScanner(file)
	lineScanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for lineScanner.Scan() {
		line := strings.TrimSpace(lineScanner.Text())
		if line == "" {
			continue
		}
		var record qwenSessionRecord
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			continue
		}
		observeQwenRecord(record, &summary)
	}
	if summary.SessionID == "" {
		summary.SessionID = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
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
	if summary.SessionID == "" || summary.EndedAt.IsZero() {
		return agentsViewGenericSummary{}, false
	}
	return summary, true
}

func observeQwenRecord(record qwenSessionRecord, summary *agentsViewGenericSummary) {
	if summary.SessionID == "" {
		summary.SessionID = strings.TrimSpace(record.SessionID)
	}
	if summary.CWD == "" {
		summary.CWD = strings.TrimSpace(record.CWD)
	}
	if summary.Project == "" {
		summary.Project = projectNameFromPath(summary.CWD)
	}
	if ts := parseRFC3339Any(record.Timestamp); !ts.IsZero() {
		ts = ts.UTC()
		if summary.StartedAt.IsZero() || ts.Before(summary.StartedAt) {
			summary.StartedAt = ts
		}
		if ts.After(summary.EndedAt) {
			summary.EndedAt = ts
		}
	}
	if record.Type != "assistant" || record.Message.Role != "model" {
		return
	}
	if summary.Model == "" {
		summary.Model = strings.TrimSpace(record.Model)
	}
	for _, part := range record.Message.Parts {
		if part.FunctionCall == nil {
			continue
		}
		mergeFileEdits(summary.FileEdits, fileEditsFromToolCall(part.FunctionCall.Name, part.FunctionCall.Args))
	}
	if record.UsageMetadata.PromptTokenCount != 0 ||
		record.UsageMetadata.CandidatesTokenCount != 0 ||
		record.UsageMetadata.TotalTokenCount != 0 {
		summary.PromptTokens += record.UsageMetadata.PromptTokenCount
		summary.CompletionTokens += record.UsageMetadata.CandidatesTokenCount
		summary.TotalTokens += record.UsageMetadata.TotalTokenCount
	}
}
