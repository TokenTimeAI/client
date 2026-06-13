package detectors

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ttime-ai/ttime/client/internal/scanner"
)

type vsCodeCopilotSession struct {
	SessionID       string                 `json:"sessionId"`
	CreationDate    int64                  `json:"creationDate"`
	LastMessageDate int64                  `json:"lastMessageDate"`
	CustomTitle     string                 `json:"customTitle"`
	Requests        []vsCodeCopilotRequest `json:"requests"`
}

type vsCodeCopilotRequest struct {
	RequestID string `json:"requestId"`
	ModelID   string `json:"modelId"`
	Timestamp int64  `json:"timestamp"`
	Message   struct {
		Text string `json:"text"`
	} `json:"message"`
	Response []struct {
		Kind   string `json:"kind"`
		ToolID string `json:"toolId"`
	} `json:"response"`
}

type vsCodeCopilotJSONLOp struct {
	Kind int               `json:"kind"`
	K    []json.RawMessage `json:"k"`
	V    json.RawMessage   `json:"v"`
	I    *int              `json:"i"`
}

func collectVSCodeCopilotGenericSummaries(ctx context.Context, root string) ([]agentsViewGenericSummary, error) {
	paths, err := collectVSCodeCopilotSessionFiles(ctx, root)
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
		if summary, ok := summarizeVSCodeCopilotSession(path); ok {
			summaries = append(summaries, summary)
		}
	}
	return summaries, nil
}

func collectVSCodeCopilotSessionFiles(ctx context.Context, root string) ([]string, error) {
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
		if filepath.Base(filepath.Dir(path)) != "chatSessions" {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext == ".json" || ext == ".jsonl" {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk vscode copilot sessions: %w", err)
	}
	sort.Strings(paths)
	return paths, nil
}

func summarizeVSCodeCopilotSession(path string) (agentsViewGenericSummary, bool) {
	data, err := readVSCodeCopilotSessionData(path)
	if err != nil {
		return agentsViewGenericSummary{}, false
	}
	var session vsCodeCopilotSession
	if err := json.Unmarshal(data, &session); err != nil || len(session.Requests) == 0 {
		return agentsViewGenericSummary{}, false
	}
	cwd := vsCodeWorkspacePath(filepath.Dir(filepath.Dir(path)))
	summary := agentsViewGenericSummary{
		SessionID: session.SessionID,
		CWD:       cwd,
		Project:   projectNameFromPath(cwd),
		FileEdits: make(map[string]scanner.FileEdit),
	}
	if summary.SessionID == "" {
		summary.SessionID = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	if session.CreationDate > 0 {
		summary.StartedAt = time.UnixMilli(session.CreationDate).UTC()
	}
	if session.LastMessageDate > 0 {
		summary.EndedAt = time.UnixMilli(session.LastMessageDate).UTC()
	}
	for _, request := range session.Requests {
		if summary.Model == "" {
			summary.Model = strings.TrimSpace(request.ModelID)
		}
		if request.Timestamp > 0 {
			ts := time.UnixMilli(request.Timestamp).UTC()
			if summary.StartedAt.IsZero() || ts.Before(summary.StartedAt) {
				summary.StartedAt = ts
			}
			if ts.After(summary.EndedAt) {
				summary.EndedAt = ts
			}
		}
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

func readVSCodeCopilotSessionData(path string) ([]byte, error) {
	if !strings.EqualFold(filepath.Ext(path), ".jsonl") {
		return os.ReadFile(path)
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return reconstructVSCodeCopilotJSONL(file)
}

func reconstructVSCodeCopilotJSONL(reader io.Reader) ([]byte, error) {
	var state any
	lineScanner := bufio.NewScanner(reader)
	lineScanner.Buffer(make([]byte, 0, 64*1024), 64*1024*1024)
	for lineScanner.Scan() {
		line := strings.TrimSpace(lineScanner.Text())
		if line == "" {
			continue
		}
		var op vsCodeCopilotJSONLOp
		if err := json.Unmarshal([]byte(line), &op); err != nil {
			continue
		}
		switch op.Kind {
		case 0:
			_ = json.Unmarshal(op.V, &state)
		case 1:
			vsCodeJSONLSet(state, vsCodeJSONLKeys(op.K), op.V)
		case 2:
			vsCodeJSONLPush(state, vsCodeJSONLKeys(op.K), op.V, op.I)
		case 3:
			vsCodeJSONLDelete(state, vsCodeJSONLKeys(op.K))
		}
	}
	if err := lineScanner.Err(); err != nil {
		return nil, err
	}
	if state == nil {
		return nil, fmt.Errorf("empty vscode copilot jsonl")
	}
	return json.Marshal(state)
}

func vsCodeJSONLKeys(raw []json.RawMessage) []string {
	keys := make([]string, 0, len(raw))
	for _, part := range raw {
		var key string
		if err := json.Unmarshal(part, &key); err == nil {
			keys = append(keys, key)
			continue
		}
		keys = append(keys, strings.TrimSpace(string(part)))
	}
	return keys
}

func vsCodeJSONLSet(state any, keys []string, raw json.RawMessage) {
	parent, key := vsCodeJSONLParent(state, keys)
	if parent == nil {
		return
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return
	}
	switch typed := parent.(type) {
	case map[string]any:
		typed[key] = value
	case []any:
		if idx, ok := vsCodeJSONLIndex(key, len(typed)); ok {
			typed[idx] = value
		}
	}
}

func vsCodeJSONLPush(state any, keys []string, raw json.RawMessage, spliceIndex *int) {
	parent, key := vsCodeJSONLParent(state, keys)
	if parent == nil {
		return
	}
	var items []any
	if err := json.Unmarshal(raw, &items); err != nil {
		return
	}
	switch typed := parent.(type) {
	case map[string]any:
		typed[key] = vsCodeJSONLAppend(typed[key], items, spliceIndex)
	case []any:
		if idx, ok := vsCodeJSONLIndex(key, len(typed)); ok {
			typed[idx] = vsCodeJSONLAppend(typed[idx], items, spliceIndex)
		}
	}
}

func vsCodeJSONLDelete(state any, keys []string) {
	parent, key := vsCodeJSONLParent(state, keys)
	if typed, ok := parent.(map[string]any); ok {
		delete(typed, key)
	}
}

func vsCodeJSONLParent(state any, keys []string) (any, string) {
	if len(keys) == 0 {
		return nil, ""
	}
	current := state
	for _, key := range keys[:len(keys)-1] {
		current = vsCodeJSONLChild(current, key)
		if current == nil {
			return nil, ""
		}
	}
	return current, keys[len(keys)-1]
}

func vsCodeJSONLChild(value any, key string) any {
	switch typed := value.(type) {
	case map[string]any:
		return typed[key]
	case []any:
		if idx, ok := vsCodeJSONLIndex(key, len(typed)); ok {
			return typed[idx]
		}
	}
	return nil
}

func vsCodeJSONLAppend(value any, items []any, spliceIndex *int) any {
	existing, ok := value.([]any)
	if !ok {
		return value
	}
	if spliceIndex == nil {
		return append(existing, items...)
	}
	idx := max(0, min(*spliceIndex, len(existing)))
	out := make([]any, 0, len(existing)+len(items))
	out = append(out, existing[:idx]...)
	out = append(out, items...)
	return append(out, existing[idx:]...)
}

func vsCodeJSONLIndex(raw string, length int) (int, bool) {
	idx, err := strconv.Atoi(raw)
	return idx, err == nil && idx >= 0 && idx < length
}

func vsCodeWorkspacePath(hashDir string) string {
	data, err := os.ReadFile(filepath.Join(hashDir, "workspace.json"))
	if err != nil {
		return ""
	}
	var workspace struct {
		Folder    string `json:"folder"`
		Workspace string `json:"workspace"`
	}
	if err := json.Unmarshal(data, &workspace); err != nil {
		return ""
	}
	raw := firstNonEmptyString(workspace.Folder, workspace.Workspace)
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "file" {
		return raw
	}
	return parsed.Path
}
