package detectors

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ttime-ai/ttime/client/internal/scanner"
)

const kiroIDEExecLogSubdir = "414d1636299d2b9e4ce7e17fb11f63e9"

type kiroIDEWorkspaceSession struct {
	SessionID          string                 `json:"sessionId"`
	Title              string                 `json:"title"`
	WorkspaceDirectory string                 `json:"workspaceDirectory"`
	History            []kiroIDEHistoryRecord `json:"history"`
}

type kiroIDEHistoryRecord struct {
	Message     kiroIDEHistoryMessage         `json:"message"`
	PromptLogs  []struct{ Completion string } `json:"promptLogs"`
	ExecutionID string                        `json:"executionId"`
}

type kiroIDEHistoryMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
	ID      string `json:"id"`
}

type kiroIDEChatSession struct {
	Chat []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"chat"`
	Metadata struct {
		ModelID   string `json:"modelId"`
		StartTime int64  `json:"startTime"`
		EndTime   int64  `json:"endTime"`
	} `json:"metadata"`
}

func collectKiroIDEGenericSummaries(ctx context.Context, root string) ([]agentsViewGenericSummary, error) {
	paths, err := collectKiroIDEWorkspaceSessionFiles(ctx, root)
	if err != nil {
		return nil, err
	}
	chatPaths, err := collectKiroIDEOldChatFiles(ctx, root)
	if err != nil {
		return nil, err
	}
	paths = append(paths, chatPaths...)
	sort.Strings(paths)

	summaries := make([]agentsViewGenericSummary, 0, len(paths))
	for _, path := range paths {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		var summary agentsViewGenericSummary
		var ok bool
		if strings.EqualFold(filepath.Ext(path), ".chat") {
			summary, ok = summarizeKiroIDEOldChatSession(root, path)
		} else {
			summary, ok = summarizeKiroIDEWorkspaceSession(root, path)
		}
		if ok {
			summaries = append(summaries, summary)
		}
	}
	return summaries, nil
}

func collectKiroIDEWorkspaceSessionFiles(ctx context.Context, root string) ([]string, error) {
	sessionRoot := filepath.Join(root, "workspace-sessions")
	if !scanner.DirExists(sessionRoot) {
		return nil, nil
	}
	paths := make([]string, 0, 16)
	err := filepath.WalkDir(sessionRoot, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if entry.IsDir() {
			return nil
		}
		if entry.Name() == "sessions.json" || !strings.EqualFold(filepath.Ext(entry.Name()), ".json") {
			return nil
		}
		paths = append(paths, path)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk kiro ide sessions: %w", err)
	}
	sort.Strings(paths)
	return paths, nil
}

func collectKiroIDEOldChatFiles(ctx context.Context, root string) ([]string, error) {
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
			if name == "workspace-sessions" || name == "node_modules" || strings.HasPrefix(name, ".git") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.EqualFold(filepath.Ext(entry.Name()), ".chat") {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk kiro ide chats: %w", err)
	}
	sort.Strings(paths)
	return paths, nil
}

func summarizeKiroIDEWorkspaceSession(root, path string) (agentsViewGenericSummary, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return agentsViewGenericSummary{}, false
	}
	var session kiroIDEWorkspaceSession
	if err := json.Unmarshal(data, &session); err != nil || session.SessionID == "" {
		return agentsViewGenericSummary{}, false
	}

	summary := agentsViewGenericSummary{
		SessionID: session.SessionID,
		CWD:       strings.TrimSpace(session.WorkspaceDirectory),
		Project:   projectNameFromPath(session.WorkspaceDirectory),
		FileEdits: make(map[string]scanner.FileEdit),
	}
	execIndex := kiroIDEExecutionLogIndex(root, session.WorkspaceDirectory)
	hasContent := false
	for _, record := range session.History {
		content := kiroIDEText(record.Message.Content)
		if strings.TrimSpace(content) != "" {
			hasContent = true
		}
		if record.Message.Role == "assistant" {
			visitKiroIDEExecutionLog(execIndex[record.ExecutionID], &summary)
			for _, promptLog := range record.PromptLogs {
				if strings.TrimSpace(promptLog.Completion) != "" {
					hasContent = true
				}
			}
		}
	}
	if info, err := os.Stat(path); err == nil {
		summary.StartedAt = info.ModTime().UTC()
		summary.EndedAt = summary.StartedAt
	}
	if summary.SessionID == "" || summary.EndedAt.IsZero() || (!hasContent && len(summary.FileEdits) == 0) {
		return agentsViewGenericSummary{}, false
	}
	return summary, true
}

func summarizeKiroIDEOldChatSession(root, path string) (agentsViewGenericSummary, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return agentsViewGenericSummary{}, false
	}
	var chat kiroIDEChatSession
	if err := json.Unmarshal(data, &chat); err != nil || len(chat.Chat) == 0 {
		return agentsViewGenericSummary{}, false
	}

	workspace := kiroIDEWorkspaceForHash(root, filepath.Base(filepath.Dir(path)))
	summary := agentsViewGenericSummary{
		SessionID: strings.TrimSuffix(filepath.Base(filepath.Dir(path)), filepath.Ext(filepath.Dir(path))) + ":" +
			strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)),
		CWD:       workspace,
		Project:   projectNameFromPath(workspace),
		Model:     strings.TrimSpace(chat.Metadata.ModelID),
		FileEdits: make(map[string]scanner.FileEdit),
	}
	hasContent := false
	for _, message := range chat.Chat {
		content := strings.TrimSpace(message.Content)
		if content == "" || content == "I will follow these instructions." {
			continue
		}
		if message.Role == "human" {
			content = strings.TrimPrefix(content, "<kiro-ide-message>")
			content = strings.TrimSuffix(content, "</kiro-ide-message>")
			content = strings.TrimSpace(content)
		}
		if content != "" {
			hasContent = true
		}
	}
	if chat.Metadata.StartTime > 0 {
		summary.StartedAt = time.UnixMilli(chat.Metadata.StartTime).UTC()
	}
	if chat.Metadata.EndTime > 0 {
		summary.EndedAt = time.UnixMilli(chat.Metadata.EndTime).UTC()
	}
	if info, err := os.Stat(path); err == nil {
		if summary.StartedAt.IsZero() {
			summary.StartedAt = info.ModTime().UTC()
		}
		if summary.EndedAt.IsZero() {
			summary.EndedAt = info.ModTime().UTC()
		}
	}
	if summary.Project == "" {
		summary.Project = "unknown"
	}
	if summary.SessionID == "" || summary.EndedAt.IsZero() || !hasContent {
		return agentsViewGenericSummary{}, false
	}
	return summary, true
}

func kiroIDEExecutionLogIndex(root, workspace string) map[string]string {
	if strings.TrimSpace(workspace) == "" {
		return nil
	}
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(workspace)))[:32]
	execDir := filepath.Join(root, hash, kiroIDEExecLogSubdir)
	entries, err := os.ReadDir(execDir)
	if err != nil {
		return nil
	}
	index := make(map[string]string)
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".json") {
			continue
		}
		path := filepath.Join(execDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var header struct {
			ExecutionID string `json:"executionId"`
		}
		if err := json.Unmarshal(data, &header); err == nil && header.ExecutionID != "" {
			index[header.ExecutionID] = path
		}
	}
	return index
}

func kiroIDEWorkspaceForHash(root, workspaceHash string) string {
	sessionRoot := filepath.Join(root, "workspace-sessions")
	entries, err := os.ReadDir(sessionRoot)
	if err != nil {
		return ""
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(sessionRoot, entry.Name(), "sessions.json"))
		if err != nil {
			continue
		}
		var rows []struct {
			WorkspaceDirectory string `json:"workspaceDirectory"`
		}
		if err := json.Unmarshal(data, &rows); err != nil {
			continue
		}
		for _, row := range rows {
			workspace := strings.TrimSpace(row.WorkspaceDirectory)
			hash := fmt.Sprintf("%x", sha256.Sum256([]byte(workspace)))[:32]
			if hash == workspaceHash {
				return workspace
			}
		}
	}
	return ""
}

func visitKiroIDEExecutionLog(path string, summary *agentsViewGenericSummary) {
	if path == "" {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var body struct {
		Actions []struct {
			ActionType string `json:"actionType"`
			Input      struct {
				File string `json:"file"`
			} `json:"input"`
		} `json:"actions"`
	}
	if err := json.Unmarshal(data, &body); err != nil {
		return
	}
	for _, action := range body.Actions {
		switch action.ActionType {
		case "replace", "create":
			upsertFileEdit(summary.FileEdits, action.Input.File, 1, 0, 0)
		}
	}
}

func kiroIDEText(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case []any:
		var parts []string
		for _, item := range typed {
			block, ok := item.(map[string]any)
			if !ok || stringValue(block["type"]) != "text" {
				continue
			}
			if text := strings.TrimSpace(stringValue(block["text"])); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "\n")
	default:
		return ""
	}
}
