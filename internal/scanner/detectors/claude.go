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
	"time"

	"github.com/ttime-ai/ttime/client/internal/scanner"
)

type ClaudeDetector struct {
	scanner.BaseDetector
	dataDir string
}

func NewClaudeDetector() scanner.Detector {
	return &ClaudeDetector{
		BaseDetector: scanner.NewBaseDetector("claude_code", "Claude Code CLI conversations", []string{"~/.claude", "~/.config/claude", "~/Library/Application Support/Claude"}, 50),
	}
}

func (d *ClaudeDetector) Detect(ctx context.Context) (bool, error) {
	for _, path := range d.DefaultPaths() {
		expanded, err := scanner.ExpandHome(path)
		if err != nil {
			continue
		}
		if scanner.DirExists(expanded) {
			d.dataDir = expanded
			d.SetFoundPath(expanded)
			return true, nil
		}
	}
	return false, nil
}

type claudeSessionSummary struct {
	SessionID        string
	CWD              string
	Title            string
	Model            string
	StartedAt        time.Time
	EndedAt          time.Time
	AgentActive      time.Duration
	HumanActive      time.Duration
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	FileEdits        map[string]scanner.FileEdit
}

type claudeRoleEvent struct {
	Role      string
	Timestamp time.Time
	Content   string
}

func (d *ClaudeDetector) Scan(ctx context.Context, state scanner.SourceState) ([]scanner.ScanResult, scanner.SourceState, error) {
	if d.dataDir == "" {
		return nil, state, nil
	}

	paths := make([]string, 0, 128)
	projectsDir := filepath.Join(d.dataDir, "projects")
	if err := filepath.WalkDir(projectsDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if entry.IsDir() {
			if entry.Name() == "subagents" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(entry.Name(), ".jsonl") {
			paths = append(paths, path)
		}
		return nil
	}); err != nil && !os.IsNotExist(err) {
		return nil, state, fmt.Errorf("walk claude sessions: %w", err)
	}

	summaries := make([]claudeSessionSummary, 0, len(paths))
	for _, path := range paths {
		select {
		case <-ctx.Done():
			return nil, state, ctx.Err()
		default:
		}

		summary, ok := summarizeClaudeSession(path)
		if !ok || summary.SessionID == "" || summary.EndedAt.IsZero() {
			continue
		}
		summaries = append(summaries, summary)
	}

	sort.Slice(summaries, func(i, j int) bool {
		if !summaries[i].EndedAt.Equal(summaries[j].EndedAt) {
			return summaries[i].EndedAt.Before(summaries[j].EndedAt)
		}
		return summaries[i].SessionID < summaries[j].SessionID
	})

	results := make([]scanner.ScanResult, 0, len(summaries))
	newState := state

	for _, summary := range summaries {
		endUnix := summary.EndedAt.Unix()
		if endUnix < state.LastScanTime || (endUnix == state.LastScanTime && summary.SessionID <= state.LastRecordID) {
			continue
		}

		sessionSeconds := durationSeconds(summary.StartedAt, summary.EndedAt)
		agentSeconds := int(summary.AgentActive.Round(time.Second).Seconds())
		humanSeconds := int(summary.HumanActive.Round(time.Second).Seconds())
		idleSeconds := sessionSeconds - agentSeconds - humanSeconds
		if idleSeconds < 0 {
			idleSeconds = 0
		}

		results = append(results, scanner.ScanResult{
			AgentType:              "claude_code",
			Type:                   "conversation",
			Entity:                 summary.CWD,
			Time:                   float64(endUnix),
			Timestamp:              summary.EndedAt,
			Duration:               float64(sessionSeconds),
			SessionStartedAt:       timePtr(summary.StartedAt),
			SessionEndedAt:         timePtr(summary.EndedAt),
			SessionDurationSeconds: intPtr(sessionSeconds),
			AgentActiveSeconds:     intPtr(agentSeconds),
			HumanActiveSeconds:     intPtr(humanSeconds),
			IdleSeconds:            intPtr(idleSeconds),
			ConversationID:         summary.SessionID,
			MessageID:              summary.SessionID,
			PromptTokens:           summary.PromptTokens,
			CompletionTokens:       summary.CompletionTokens,
			TotalTokens:            summary.TotalTokens,
			Model:                  summary.Model,
			FileEdits:              flattenFileEdits(summary.FileEdits),
			Project:                projectNameFromPath(summary.CWD),
			Metadata: map[string]any{
				"title": summary.Title,
			},
		})

		newState.LastScanTime = endUnix
		newState.LastRecordID = summary.SessionID
	}

	return results, newState, nil
}

func summarizeClaudeSession(path string) (claudeSessionSummary, bool) {
	file, err := os.Open(path)
	if err != nil {
		return claudeSessionSummary{}, false
	}
	defer file.Close()

	var summary claudeSessionSummary
	summary.FileEdits = make(map[string]scanner.FileEdit)
	roleEvents := make([]claudeRoleEvent, 0, 32)
	lineScanner := bufio.NewScanner(file)

	for lineScanner.Scan() {
		var record map[string]any
		if err := json.Unmarshal(lineScanner.Bytes(), &record); err != nil {
			continue
		}

		if summary.SessionID == "" {
			summary.SessionID = strings.TrimSpace(stringValue(record["sessionId"]))
		}
		if summary.CWD == "" {
			summary.CWD = strings.TrimSpace(stringValue(record["cwd"]))
		}

		timestamp := parseRFC3339Any(strings.TrimSpace(stringValue(record["timestamp"]))).UTC()
		if timestamp.IsZero() {
			continue
		}
		if summary.StartedAt.IsZero() || timestamp.Before(summary.StartedAt) {
			summary.StartedAt = timestamp
		}
		if timestamp.After(summary.EndedAt) {
			summary.EndedAt = timestamp
		}

		switch strings.TrimSpace(stringValue(record["type"])) {
		case "user":
			content := extractClaudeText(record["message"])
			roleEvents = append(roleEvents, claudeRoleEvent{Role: "user", Timestamp: timestamp, Content: content})
			if summary.Title == "" && strings.TrimSpace(content) != "" {
				summary.Title = truncateString(content, 80)
			}
		case "assistant":
			if message, ok := record["message"].(map[string]any); ok {
				recordClaudeToolUseFileEdits(summary.FileEdits, message)
				if model := strings.TrimSpace(stringValue(message["model"])); model != "" {
					summary.Model = model
				}
				if usage, ok := message["usage"].(map[string]any); ok {
					input := intValue(usage["input_tokens"])
					output := intValue(usage["output_tokens"])
					summary.PromptTokens += input
					summary.CompletionTokens += output
					summary.TotalTokens += input + output
				}
			}
			roleEvents = append(roleEvents, claudeRoleEvent{Role: "assistant", Timestamp: timestamp})
		}
	}

	for i := 0; i < len(roleEvents)-1; i++ {
		current := roleEvents[i]
		next := roleEvents[i+1]
		diff := next.Timestamp.Sub(current.Timestamp)
		if diff < 0 {
			continue
		}
		if current.Role == "user" && next.Role == "assistant" {
			summary.AgentActive += diff
		}
		if current.Role == "assistant" && next.Role == "user" {
			summary.HumanActive += diff
		}
	}

	if summary.TotalTokens == 0 {
		summary.TotalTokens = summary.PromptTokens + summary.CompletionTokens
	}
	if summary.SessionID == "" || summary.EndedAt.IsZero() {
		return claudeSessionSummary{}, false
	}
	return summary, true
}

func extractClaudeText(raw any) string {
	message, ok := raw.(map[string]any)
	if !ok {
		return ""
	}
	switch content := message["content"].(type) {
	case string:
		return content
	case []any:
		parts := make([]string, 0, len(content))
		for _, item := range content {
			if m, ok := item.(map[string]any); ok {
				if text := strings.TrimSpace(stringValue(m["text"])); text != "" {
					parts = append(parts, text)
				}
			}
		}
		return strings.Join(parts, "\n")
	default:
		return ""
	}
}

func recordClaudeToolUseFileEdits(target map[string]scanner.FileEdit, message map[string]any) {
	content, ok := message["content"].([]any)
	if !ok {
		return
	}
	for _, item := range content {
		entry, ok := item.(map[string]any)
		if !ok || strings.TrimSpace(stringValue(entry["type"])) != "tool_use" {
			continue
		}
		name := strings.TrimSpace(stringValue(entry["name"]))
		if name != "Edit" && name != "MultiEdit" && name != "Write" && name != "NotebookEdit" {
			continue
		}
		input, _ := entry["input"].(map[string]any)
		path := strings.TrimSpace(stringValue(input["file_path"]))
		if path == "" {
			path = strings.TrimSpace(stringValue(input["notebook_path"]))
		}
		if path == "" {
			continue
		}
		current := target[path]
		current.Path = path
		current.EditCount++
		target[path] = current
	}
}

func init() {
	scanner.Register(NewClaudeDetector)
}
