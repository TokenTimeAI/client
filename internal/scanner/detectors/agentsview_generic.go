package detectors

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ttime-ai/ttime/client/internal/scanner"
)

type agentsViewGenericDefinition struct {
	Name        string
	Description string
	Paths       []string
}

type agentsViewGenericDetector struct {
	scanner.BaseDetector
	def       agentsViewGenericDefinition
	foundPath string
}

type agentsViewGenericSummary struct {
	SessionID        string
	CWD              string
	Project          string
	StartedAt        time.Time
	EndedAt          time.Time
	Model            string
	PromptTokens        int
	CompletionTokens    int
	CachedTokens        int
	CacheCreationTokens int
	ReasoningTokens     int
	TotalTokens         int
	FileEdits        map[string]scanner.FileEdit
}

var (
	agentsViewWorkingDirectoryPattern = regexp.MustCompile(`(?m)Working directory:\s+([^\r\n]+)`)
	agentsViewCurrentWorkingDirTag    = regexp.MustCompile(`(?s)<current_working_directory>\s*([^<]+?)\s*</current_working_directory>`)
)

func newAgentsViewGenericDetector(def agentsViewGenericDefinition) scanner.Detector {
	return &agentsViewGenericDetector{
		BaseDetector: scanner.NewBaseDetector(def.Name, def.Description, def.Paths, 25),
		def:          def,
	}
}

func (d *agentsViewGenericDetector) Detect(ctx context.Context) (bool, error) {
	for _, path := range d.DefaultPaths() {
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		default:
		}

		expanded, err := scanner.ExpandHome(path)
		if err != nil {
			continue
		}
		if scanner.DirExists(expanded) {
			d.foundPath = expanded
			d.SetFoundPath(expanded)
			return true, nil
		}
	}
	return false, nil
}

func (d *agentsViewGenericDetector) Scan(ctx context.Context, state scanner.SourceState) ([]scanner.ScanResult, scanner.SourceState, error) {
	if d.foundPath == "" {
		return nil, state, nil
	}

	summaries, err := d.collectSummaries(ctx)
	if err != nil {
		return nil, state, err
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
		project := strings.TrimSpace(summary.Project)
		if project == "" {
			project = projectNameFromPath(summary.CWD)
		}
		results = append(results, scanner.ScanResult{
			AgentType:              d.Name(),
			Type:                   "conversation",
			Entity:                 summary.CWD,
			Time:                   float64(endUnix),
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
				"parser": "agentsview_generic",
			},
		})

		newState.LastScanTime = endUnix
		newState.LastRecordID = summary.SessionID
	}

	return results, newState, nil
}

func (d *agentsViewGenericDetector) collectSummaries(ctx context.Context) ([]agentsViewGenericSummary, error) {
	if summaries, handled, err := d.collectSpecialSummaries(ctx); handled || err != nil {
		return summaries, err
	}

	paths, err := collectGenericSessionFiles(ctx, d.foundPath)
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

		summary, ok := summarizeAgentsViewGenericSession(path)
		if !ok {
			continue
		}
		summaries = append(summaries, summary)
	}
	return summaries, nil
}

func visitGenericJSONFile(path string, summary *agentsViewGenericSummary) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	var body any
	if err := json.NewDecoder(file).Decode(&body); err == nil {
		visitGenericJSON(body, summary)
	}
}

func collectGenericSessionFiles(ctx context.Context, root string) ([]string, error) {
	paths := make([]string, 0, 64)
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
			if strings.HasPrefix(name, ".git") || name == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if (ext == ".json" || ext == ".jsonl") && looksLikeGenericSessionPath(path) {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk generic sessions: %w", err)
	}
	sort.Strings(paths)
	return paths, nil
}

func looksLikeGenericSessionPath(path string) bool {
	normalized := strings.ToLower(filepath.ToSlash(path))
	for _, marker := range []string{
		"session", "sessions", "conversation", "conversations", "thread",
		"threads", "chat", "chats", "trajectory", "message", "messages",
		"history", "projects",
	} {
		if strings.Contains(normalized, marker) {
			return true
		}
	}
	return false
}

func summarizeAgentsViewGenericSession(path string) (agentsViewGenericSummary, bool) {
	info, err := os.Stat(path)
	if err != nil {
		return agentsViewGenericSummary{}, false
	}

	summary := agentsViewGenericSummary{
		FileEdits: make(map[string]scanner.FileEdit),
	}

	file, err := os.Open(path)
	if err != nil {
		return agentsViewGenericSummary{}, false
	}
	defer file.Close()

	if strings.EqualFold(filepath.Ext(path), ".jsonl") {
		scanGenericJSONLines(file, &summary)
	} else {
		var body any
		if err := json.NewDecoder(file).Decode(&body); err == nil {
			visitGenericJSON(body, &summary)
		}
	}

	applyPathBasedGenericIdentity(path, &summary)

	if summary.EndedAt.IsZero() {
		summary.EndedAt = info.ModTime().UTC()
	}
	if summary.StartedAt.IsZero() {
		summary.StartedAt = summary.EndedAt
	}
	if summary.CWD == "" {
		summary.CWD = strings.TrimSpace(summary.Project)
	}
	if summary.TotalTokens == 0 {
		summary.TotalTokens = summary.PromptTokens + summary.CompletionTokens
	}
	if summary.SessionID == "" {
		summary.SessionID = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	if summary.SessionID == "" || summary.EndedAt.IsZero() {
		return agentsViewGenericSummary{}, false
	}
	return summary, true
}

func applyPathBasedGenericIdentity(path string, summary *agentsViewGenericSummary) {
	if filepath.Base(path) != "wire.jsonl" {
		return
	}

	sessionDir := filepath.Dir(path)
	sessionID := filepath.Base(sessionDir)
	project := filepath.Base(filepath.Dir(sessionDir))
	if project == "." || project == string(filepath.Separator) || sessionID == "." || sessionID == string(filepath.Separator) {
		return
	}
	summary.SessionID = project + ":" + sessionID
	if summary.Project == "" {
		summary.Project = project
	}
}

func scanGenericJSONLines(reader io.Reader, summary *agentsViewGenericSummary) {
	lineScanner := bufio.NewScanner(reader)
	lineScanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for lineScanner.Scan() {
		line := strings.TrimSpace(lineScanner.Text())
		if line == "" {
			continue
		}
		var body any
		if err := json.Unmarshal([]byte(line), &body); err != nil {
			continue
		}
		visitGenericJSON(body, summary)
	}
}

func visitGenericJSON(value any, summary *agentsViewGenericSummary) {
	switch typed := value.(type) {
	case map[string]any:
		observeGenericMap(typed, summary)
		for _, child := range typed {
			visitGenericJSON(child, summary)
		}
	case []any:
		for _, child := range typed {
			visitGenericJSON(child, summary)
		}
	}
}

func observeGenericMap(record map[string]any, summary *agentsViewGenericSummary) {
	if summary.SessionID == "" {
		summary.SessionID = firstString(record, "sessionId", "session_id", "conversation_id", "conversationId", "thread_id", "threadId", "id")
	}
	if summary.CWD == "" {
		summary.CWD = firstString(record, "cwd", "workspace", "workspace_path", "workspacePath", "repoPath", "working_directory", "workingDirectory", "git_root", "gitRoot")
	}
	if summary.Project == "" {
		summary.Project = firstString(record, "project", "project_name", "projectName", "workspaceName")
	}
	if summary.Model == "" {
		summary.Model = firstString(record, "model", "model_id", "modelId")
	}
	if summary.CWD == "" {
		summary.CWD = workingDirectoryFromGenericContent(record["content"])
	}

	if ts := firstTimestamp(record, "timestamp", "created_at", "createdAt", "started_at", "startedAt", "time"); !ts.IsZero() {
		if summary.StartedAt.IsZero() || ts.Before(summary.StartedAt) {
			summary.StartedAt = ts
		}
		if ts.After(summary.EndedAt) {
			summary.EndedAt = ts
		}
	}
	if ts := firstTimestamp(record, "updated_at", "updatedAt", "ended_at", "endedAt", "completed_at", "completedAt", "modified"); !ts.IsZero() && ts.After(summary.EndedAt) {
		summary.EndedAt = ts
	}

	summary.PromptTokens += firstInt(record, "prompt_tokens", "input_tokens", "input")
	summary.CompletionTokens += firstInt(record, "completion_tokens", "output_tokens", "output")
	summary.TotalTokens += firstInt(record, "total_tokens", "tokens_used", "totalTokens")

	if toolName := firstString(record, "name", "toolName", "tool_name", "toolId"); toolName != "" {
		if input, ok := genericToolInput(record); ok {
			mergeFileEdits(summary.FileEdits, fileEditsFromToolCall(toolName, input))
		}
	}
	if toolName := firstString(record, "tool_name"); toolName == "file_editor" {
		if action, ok := record["action"].(map[string]any); ok {
			mergeFileEdits(summary.FileEdits, fileEditsFromToolCall(firstString(action, "command"), action))
		}
	}
}

func workingDirectoryFromGenericContent(value any) string {
	text := stringValue(value)
	if text == "" {
		return ""
	}
	for _, pattern := range []*regexp.Regexp{
		agentsViewWorkingDirectoryPattern,
		agentsViewCurrentWorkingDirTag,
	} {
		match := pattern.FindStringSubmatch(text)
		if len(match) == 2 {
			return strings.TrimSpace(match[1])
		}
	}
	return ""
}

func genericToolInput(record map[string]any) (map[string]any, bool) {
	for _, key := range []string{"input", "arguments", "args", "parameters"} {
		value, ok := record[key]
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case map[string]any:
			return typed, true
		case string:
			var decoded map[string]any
			if err := json.Unmarshal([]byte(typed), &decoded); err == nil {
				return decoded, true
			}
			return map[string]any{"input": typed}, true
		}
	}
	return record, true
}

func firstString(record map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(stringValue(record[key])); value != "" {
			return value
		}
	}
	return ""
}

func firstInt(record map[string]any, keys ...string) int {
	for _, key := range keys {
		if value := intValue(record[key]); value != 0 {
			return value
		}
	}
	return 0
}

func firstTimestamp(record map[string]any, keys ...string) time.Time {
	for _, key := range keys {
		if ts := genericTimestamp(record[key]); !ts.IsZero() {
			return ts
		}
	}
	return time.Time{}
}

func genericTimestamp(value any) time.Time {
	switch typed := value.(type) {
	case string:
		if ts := parseRFC3339Any(typed); !ts.IsZero() {
			return ts.UTC()
		}
		if parsed, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64); err == nil {
			return unixFlexible(parsed)
		}
	case float64:
		return unixFlexible(int64(typed))
	case int64:
		return unixFlexible(typed)
	case int:
		return unixFlexible(int64(typed))
	}
	return time.Time{}
}

func unixFlexible(value int64) time.Time {
	if value <= 0 {
		return time.Time{}
	}
	if value > 1_000_000_000_000 {
		return time.UnixMilli(value).UTC()
	}
	return time.Unix(value, 0).UTC()
}
