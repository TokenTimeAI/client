package detectors

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ttime-ai/ttime/client/internal/scanner"
)

func collectCopilotGenericSummaries(ctx context.Context, root string) ([]agentsViewGenericSummary, error) {
	paths, err := collectCopilotSessionFiles(ctx, root)
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
		if summary, ok := summarizeCopilotEventStream(path); ok {
			summaries = append(summaries, summary)
		}
	}
	return summaries, nil
}

func collectCopilotSessionFiles(ctx context.Context, root string) ([]string, error) {
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
			if entry.Name() == "node_modules" || strings.HasPrefix(entry.Name(), ".git") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.EqualFold(filepath.Ext(entry.Name()), ".jsonl") && strings.Contains(filepath.ToSlash(path), "session") {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk copilot sessions: %w", err)
	}
	sort.Strings(paths)
	return paths, nil
}

func summarizeCopilotEventStream(path string) (agentsViewGenericSummary, bool) {
	file, err := os.Open(path)
	if err != nil {
		return agentsViewGenericSummary{}, false
	}
	defer file.Close()

	summary := agentsViewGenericSummary{
		FileEdits: make(map[string]scanner.FileEdit),
	}
	scanCopilotEventLines(file, &summary)
	if summary.SessionID == "" {
		summary.SessionID = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	if summary.EndedAt.IsZero() {
		if info, err := os.Stat(path); err == nil {
			summary.EndedAt = info.ModTime().UTC()
		}
	}
	if summary.StartedAt.IsZero() {
		summary.StartedAt = summary.EndedAt
	}
	if summary.Project == "" {
		summary.Project = projectNameFromPath(summary.CWD)
	}
	if summary.TotalTokens == 0 {
		summary.TotalTokens = summary.PromptTokens + summary.CompletionTokens
	}
	if summary.SessionID == "" || summary.EndedAt.IsZero() {
		return agentsViewGenericSummary{}, false
	}
	return summary, true
}

func scanCopilotEventLines(reader io.Reader, summary *agentsViewGenericSummary) {
	lineScanner := bufio.NewScanner(reader)
	lineScanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for lineScanner.Scan() {
		line := strings.TrimSpace(lineScanner.Text())
		if line == "" {
			continue
		}
		var event struct {
			Type      string          `json:"type"`
			Timestamp string          `json:"timestamp"`
			Data      json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}
		ts := parseRFC3339Any(event.Timestamp)
		if !ts.IsZero() {
			if summary.StartedAt.IsZero() || ts.Before(summary.StartedAt) {
				summary.StartedAt = ts.UTC()
			}
			if ts.After(summary.EndedAt) {
				summary.EndedAt = ts.UTC()
			}
		}
		switch event.Type {
		case "session.start":
			visitCopilotSessionStart(event.Data, summary)
		case "session.model_change":
			visitCopilotModelChange(event.Data, summary)
		case "assistant.message":
			visitCopilotAssistantMessage(event.Data, summary)
		case "session.shutdown":
			visitCopilotShutdown(event.Data, summary)
		}
	}
}

func visitCopilotSessionStart(raw json.RawMessage, summary *agentsViewGenericSummary) {
	var data struct {
		SessionID string `json:"sessionId"`
		Context   struct {
			CWD string `json:"cwd"`
		} `json:"context"`
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		return
	}
	if summary.SessionID == "" {
		summary.SessionID = strings.TrimSpace(data.SessionID)
	}
	if summary.CWD == "" {
		summary.CWD = strings.TrimSpace(data.Context.CWD)
	}
	if summary.Project == "" {
		summary.Project = projectNameFromPath(summary.CWD)
	}
}

func visitCopilotModelChange(raw json.RawMessage, summary *agentsViewGenericSummary) {
	var data struct {
		NewModel string `json:"newModel"`
	}
	if err := json.Unmarshal(raw, &data); err == nil && strings.TrimSpace(data.NewModel) != "" {
		summary.Model = strings.TrimSpace(data.NewModel)
	}
}

func visitCopilotAssistantMessage(raw json.RawMessage, summary *agentsViewGenericSummary) {
	var data struct {
		ToolRequests []struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		} `json:"toolRequests"`
		OutputTokens int `json:"outputTokens"`
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		return
	}
	for _, request := range data.ToolRequests {
		mergeFileEdits(summary.FileEdits, fileEditsFromToolCall(request.Name, request.Arguments))
	}
	if summary.CompletionTokens == 0 {
		summary.CompletionTokens = data.OutputTokens
	}
}

func visitCopilotShutdown(raw json.RawMessage, summary *agentsViewGenericSummary) {
	var data struct {
		ModelMetrics map[string]struct {
			Usage struct {
				InputTokens      int `json:"inputTokens"`
				CacheReadTokens  int `json:"cacheReadTokens"`
				CacheWriteTokens int `json:"cacheWriteTokens"`
				OutputTokens     int `json:"outputTokens"`
				ReasoningTokens  int `json:"reasoningTokens"`
			} `json:"usage"`
		} `json:"modelMetrics"`
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		return
	}
	var promptTokens, completionTokens int
	for model, metrics := range data.ModelMetrics {
		if summary.Model == "" {
			summary.Model = strings.TrimSpace(model)
		}
		promptTokens += metrics.Usage.InputTokens
		completionTokens += metrics.Usage.OutputTokens + metrics.Usage.ReasoningTokens
	}
	if promptTokens != 0 || completionTokens != 0 {
		summary.PromptTokens = promptTokens
		summary.CompletionTokens = completionTokens
		summary.TotalTokens = promptTokens + completionTokens
	}
}
