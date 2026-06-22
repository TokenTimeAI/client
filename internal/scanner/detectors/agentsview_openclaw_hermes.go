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

type openClawEnvelope struct {
	Type      string `json:"type"`
	ID        string `json:"id"`
	CWD       string `json:"cwd"`
	Timestamp string `json:"timestamp"`
	Message   struct {
		Role      string `json:"role"`
		Content   any    `json:"content"`
		Timestamp string `json:"timestamp"`
		Model     string `json:"model"`
		Usage     struct {
			Input      int `json:"input"`
			Output     int `json:"output"`
			CacheRead  int `json:"cacheRead"`
			CacheWrite int `json:"cacheWrite"`
		} `json:"usage"`
	} `json:"message"`
}

type hermesTranscriptRecord struct {
	Role      string `json:"role"`
	Content   string `json:"content"`
	Timestamp string `json:"timestamp"`
	Model     string `json:"model"`
	Platform  string `json:"platform"`
	Usage     struct {
		Input      int `json:"input"`
		Output     int `json:"output"`
		CacheRead  int `json:"cacheRead"`
		CacheWrite int `json:"cacheWrite"`
	} `json:"usage"`
	ToolCalls []struct {
		ID       string `json:"id"`
		Function struct {
			Name      string `json:"name"`
			Arguments any    `json:"arguments"`
		} `json:"function"`
	} `json:"tool_calls"`
}

func collectOpenClawGenericSummaries(ctx context.Context, root string) ([]agentsViewGenericSummary, error) {
	return collectClawGenericSummaries(ctx, root, summarizeOpenClawSession)
}

func collectQClawGenericSummaries(ctx context.Context, root string) ([]agentsViewGenericSummary, error) {
	return collectClawGenericSummaries(ctx, root, summarizeQClawSession)
}

func collectClawGenericSummaries(ctx context.Context, root string, summarize func(string) (agentsViewGenericSummary, bool)) ([]agentsViewGenericSummary, error) {
	paths, err := collectAgentJSONLFiles(ctx, root)
	if err != nil {
		return nil, err
	}
	summaries := make([]agentsViewGenericSummary, 0, len(paths))
	for _, path := range paths {
		if summary, ok := summarize(path); ok {
			summaries = append(summaries, summary)
		}
	}
	return summaries, nil
}

func collectHermesGenericSummaries(ctx context.Context, root string) ([]agentsViewGenericSummary, error) {
	paths, err := collectAgentJSONLFiles(ctx, root)
	if err != nil {
		return nil, err
	}
	summaries := make([]agentsViewGenericSummary, 0, len(paths))
	for _, path := range paths {
		if summary, ok := summarizeHermesTranscript(path); ok {
			summaries = append(summaries, summary)
		}
	}
	return summaries, nil
}

func collectAgentJSONLFiles(ctx context.Context, root string) ([]string, error) {
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
		return nil, fmt.Errorf("walk agent jsonl sessions: %w", err)
	}
	sort.Strings(paths)
	return paths, nil
}

func summarizeOpenClawSession(path string) (agentsViewGenericSummary, bool) {
	return summarizeClawSession(path, openClawSummaryID)
}

func summarizeQClawSession(path string) (agentsViewGenericSummary, bool) {
	return summarizeClawSession(path, qClawSummaryID)
}

func summarizeClawSession(path string, summaryID func(string, string) string) (agentsViewGenericSummary, bool) {
	file, err := os.Open(path)
	if err != nil {
		return agentsViewGenericSummary{}, false
	}
	defer file.Close()

	summary := agentsViewGenericSummary{
		SessionID: summaryID(path, ""),
		FileEdits: make(map[string]scanner.FileEdit),
	}
	hasContent := false
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for scanner.Scan() {
		var record openClawEnvelope
		if err := json.Unmarshal([]byte(strings.TrimSpace(scanner.Text())), &record); err != nil {
			continue
		}
		observeClawEnvelope(record, path, summaryID, &summary, &hasContent)
	}
	fillFileTimeBounds(path, &summary)
	if summary.Project == "" {
		summary.Project = projectNameFromPath(summary.CWD)
	}
	if summary.TotalTokens == 0 {
		summary.TotalTokens = summary.PromptTokens + summary.CompletionTokens
	}
	if summary.SessionID == "" || summary.EndedAt.IsZero() || !hasContent {
		return agentsViewGenericSummary{}, false
	}
	return summary, true
}

func observeClawEnvelope(record openClawEnvelope, path string, summaryID func(string, string) string, summary *agentsViewGenericSummary, hasContent *bool) {
	if record.Type == "session" {
		if record.ID != "" {
			summary.SessionID = summaryID(path, record.ID)
		}
		if summary.CWD == "" {
			summary.CWD = strings.TrimSpace(record.CWD)
		}
	}
	observeAgentTimestamp(record.Timestamp, summary)
	observeAgentTimestamp(record.Message.Timestamp, summary)
	switch record.Message.Role {
	case "user":
		if strings.TrimSpace(stringValue(record.Message.Content)) != "" {
			*hasContent = true
		}
	case "assistant":
		*hasContent = true
		if summary.Model == "" {
			summary.Model = strings.TrimSpace(record.Message.Model)
		}
		mergeFileEdits(summary.FileEdits, fileEditsFromContentBlocks(record.Message.Content))
		addUsageSummary(
			summary,
			record.Message.Usage.Input,
			record.Message.Usage.Output,
			record.Message.Usage.CacheRead,
			record.Message.Usage.CacheWrite,
		)
	}
}

func summarizeHermesTranscript(path string) (agentsViewGenericSummary, bool) {
	file, err := os.Open(path)
	if err != nil {
		return agentsViewGenericSummary{}, false
	}
	defer file.Close()

	summary := agentsViewGenericSummary{
		SessionID: strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)),
		FileEdits: make(map[string]scanner.FileEdit),
	}
	hasContent := false
	lineScanner := bufio.NewScanner(file)
	lineScanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for lineScanner.Scan() {
		var record hermesTranscriptRecord
		if err := json.Unmarshal([]byte(strings.TrimSpace(lineScanner.Text())), &record); err != nil {
			continue
		}
		observeHermesRecord(record, &summary, &hasContent)
	}
	fillFileTimeBounds(path, &summary)
	if summary.Project == "" {
		summary.Project = "hermes"
	}
	if summary.TotalTokens == 0 {
		summary.TotalTokens = summary.PromptTokens + summary.CompletionTokens
	}
	if summary.SessionID == "" || summary.EndedAt.IsZero() || !hasContent {
		return agentsViewGenericSummary{}, false
	}
	return summary, true
}

func observeHermesRecord(record hermesTranscriptRecord, summary *agentsViewGenericSummary, hasContent *bool) {
	observeAgentTimestamp(record.Timestamp, summary)
	switch record.Role {
	case "session_meta":
		if summary.Model == "" {
			summary.Model = strings.TrimSpace(record.Model)
		}
		if record.Platform != "" {
			summary.Project = "hermes-" + strings.TrimSpace(record.Platform)
		}
	case "user":
		if strings.TrimSpace(record.Content) != "" {
			*hasContent = true
		}
	case "assistant":
		if strings.TrimSpace(record.Content) != "" || len(record.ToolCalls) > 0 {
			*hasContent = true
		}
		if summary.Model == "" {
			summary.Model = strings.TrimSpace(record.Model)
		}
		for _, call := range record.ToolCalls {
			input := toolArgumentsMap(call.Function.Arguments)
			mergeFileEdits(summary.FileEdits, fileEditsFromToolCall(call.Function.Name, input))
		}
		addUsageSummary(
			summary,
			record.Usage.Input,
			record.Usage.Output,
			record.Usage.CacheRead,
			record.Usage.CacheWrite,
		)
	}
}

func openClawSummaryID(path, sessionID string) string {
	if sessionID == "" {
		sessionID = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	agentID := filepath.Base(filepath.Dir(filepath.Dir(path)))
	if agentID == "" || agentID == "." || agentID == string(filepath.Separator) {
		return sessionID
	}
	return agentID + ":" + sessionID
}

func qClawSummaryID(path, sessionID string) string {
	return openClawSummaryID(path, sessionID)
}

func fileEditsFromContentBlocks(content any) map[string]scanner.FileEdit {
	edits := make(map[string]scanner.FileEdit)
	blocks, ok := content.([]any)
	if !ok {
		return edits
	}
	for _, item := range blocks {
		block, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if block["type"] != "tool_use" {
			continue
		}
		input, ok := block["input"].(map[string]any)
		if !ok {
			input = map[string]any{}
		}
		mergeFileEdits(edits, fileEditsFromToolCall(stringValue(block["name"]), input))
	}
	return edits
}

func toolArgumentsMap(value any) map[string]any {
	switch typed := value.(type) {
	case map[string]any:
		return typed
	case string:
		var decoded map[string]any
		if err := json.Unmarshal([]byte(typed), &decoded); err == nil {
			return decoded
		}
	}
	return map[string]any{}
}

func addUsageSummary(summary *agentsViewGenericSummary, input, output, cacheRead, cacheWrite int) {
	summary.PromptTokens += input
	summary.CachedTokens += cacheRead
	summary.CacheCreationTokens += cacheWrite
	summary.CompletionTokens += output
	summary.TotalTokens += input + cacheRead + cacheWrite + output
}

func observeAgentTimestamp(raw string, summary *agentsViewGenericSummary) {
	ts := parseRFC3339Any(raw)
	if ts.IsZero() {
		return
	}
	if summary.StartedAt.IsZero() || ts.Before(summary.StartedAt) {
		summary.StartedAt = ts
	}
	if ts.After(summary.EndedAt) {
		summary.EndedAt = ts
	}
}

func fillFileTimeBounds(path string, summary *agentsViewGenericSummary) {
	info, err := os.Stat(path)
	if err != nil {
		return
	}
	if summary.StartedAt.IsZero() {
		summary.StartedAt = info.ModTime().UTC()
	}
	if summary.EndedAt.IsZero() {
		summary.EndedAt = info.ModTime().UTC()
	}
}

func scanResultFromAgentsViewSummary(agent string, summary agentsViewGenericSummary) scanner.ScanResult {
	sessionSeconds := durationSeconds(summary.StartedAt, summary.EndedAt)
	project := strings.TrimSpace(summary.Project)
	if project == "" {
		project = projectNameFromPath(summary.CWD)
	}
	return scanner.ScanResult{
		AgentType:              agent,
		Type:                   "conversation",
		Entity:                 summary.CWD,
		Time:                   float64(summary.EndedAt.Unix()),
		Timestamp:              summary.EndedAt,
		Duration:               float64(sessionSeconds),
		ConversationID:         summary.SessionID,
		MessageID:              summary.SessionID,
		PromptTokens:           summary.PromptTokens,
		CompletionTokens:       summary.CompletionTokens,
		CachedTokens:           summary.CachedTokens,
		CacheCreationTokens:    summary.CacheCreationTokens,
		ReasoningTokens:        summary.ReasoningTokens,
		TotalTokens:            summary.TotalTokens,
		Model:                  summary.Model,
		FileEdits:              flattenFileEdits(summary.FileEdits),
		Project:                project,
		Metadata: map[string]any{
			"parser": "agentsview_transcript",
		},
	}
}
