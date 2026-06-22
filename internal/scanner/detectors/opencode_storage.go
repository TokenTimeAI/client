package detectors

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ttime-ai/ttime/client/internal/scanner"
)

type openCodeStorageSession struct {
	ID          string
	Directory   string
	TimeCreated int64
	TimeUpdated int64
}

type openCodeStorageTime struct {
	Created int64 `json:"created"`
	Updated int64 `json:"updated"`
	Start   int64 `json:"start"`
	End     int64 `json:"end"`
}

func (d *OpenCodeDetector) scanStorage(ctx context.Context, state scanner.SourceState) ([]scanner.ScanResult, scanner.SourceState, error) {
	sessionFiles, err := openCodeStorageSessionFiles(ctx, d.configDir)
	if err != nil {
		return nil, state, err
	}

	var results []scanner.ScanResult
	newState := state
	for _, path := range sessionFiles {
		session, ok := readOpenCodeStorageSession(path)
		if !ok {
			continue
		}
		messages, err := readOpenCodeStorageMessages(session.ID, d.configDir)
		if err != nil {
			continue
		}
		parts, err := readOpenCodeStorageParts(messages, d.configDir)
		if err != nil {
			continue
		}
		for _, msg := range messages {
			if msg.Role != "assistant" {
				continue
			}
			timestamp := time.UnixMilli(msg.TimeCreated).UTC()
			endUnix := timestamp.Unix()
			if endUnix < state.LastScanTime || (endUnix == state.LastScanTime && msg.ID <= state.LastRecordID) {
				continue
			}
			promptTokens := msg.InputTokens + msg.CacheReadTokens + msg.CacheWriteTokens
			completionTokens := msg.OutputTokens
			fileEdits := make(map[string]scanner.FileEdit)
			for _, part := range parts[msg.ID] {
				mergeFileEdits(fileEdits, openCodeFileEditsFromPart(part.Data))
			}
			results = append(results, scanner.ScanResult{
				AgentType:              "opencode",
				Type:                   "conversation",
				Entity:                 session.Directory,
				Time:                   float64(endUnix),
				Timestamp:              timestamp,
				ConversationID:         session.ID,
				MessageID:              msg.ID,
				PromptTokens:           promptTokens,
				CompletionTokens:       completionTokens,
				TotalTokens:            promptTokens + completionTokens,
				Model:                  msg.Model,
				FileEdits:              flattenFileEdits(fileEdits),
				Project:                projectNameFromPath(session.Directory),
				Metadata: map[string]any{
					"parser": "opencode_storage",
				},
			})
			if endUnix > newState.LastScanTime || (endUnix == newState.LastScanTime && msg.ID > newState.LastRecordID) {
				newState.LastScanTime = endUnix
				newState.LastRecordID = msg.ID
			}
		}
	}
	return results, newState, nil
}

func openCodeStorageSessionFiles(ctx context.Context, root string) ([]string, error) {
	sessionRoot := filepath.Join(root, "storage", "session")
	if !scanner.DirExists(sessionRoot) {
		return nil, nil
	}
	var paths []string
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
		if strings.EqualFold(filepath.Ext(entry.Name()), ".json") {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	return paths, nil
}

func readOpenCodeStorageSession(path string) (openCodeStorageSession, bool) {
	var body struct {
		ID        string              `json:"id"`
		Directory string              `json:"directory"`
		Time      openCodeStorageTime `json:"time"`
	}
	if data, err := os.ReadFile(path); err != nil {
		return openCodeStorageSession{}, false
	} else if err := json.Unmarshal(data, &body); err != nil {
		return openCodeStorageSession{}, false
	}
	if strings.TrimSpace(body.ID) == "" {
		return openCodeStorageSession{}, false
	}
	return openCodeStorageSession{
		ID:          strings.TrimSpace(body.ID),
		Directory:   strings.TrimSpace(body.Directory),
		TimeCreated: body.Time.created(),
		TimeUpdated: body.Time.updated(),
	}, true
}

func readOpenCodeStorageMessages(sessionID, root string) ([]openCodeSQLiteMessage, error) {
	messageRoot := filepath.Join(root, "storage", "message", sessionID)
	entries, err := os.ReadDir(messageRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	messages := make([]openCodeSQLiteMessage, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".json") {
			continue
		}
		path := filepath.Join(messageRoot, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var body struct {
			ID   string              `json:"id"`
			Time openCodeStorageTime `json:"time"`
		}
		if err := json.Unmarshal(data, &body); err != nil || strings.TrimSpace(body.ID) == "" {
			continue
		}
		message := openCodeSQLiteMessage{ID: strings.TrimSpace(body.ID), TimeCreated: body.Time.message()}
		applyOpenCodeSQLiteMessageData(&message, string(data))
		messages = append(messages, message)
	}
	sort.Slice(messages, func(i, j int) bool {
		if messages[i].TimeCreated == messages[j].TimeCreated {
			return messages[i].ID < messages[j].ID
		}
		return messages[i].TimeCreated < messages[j].TimeCreated
	})
	return messages, nil
}

func readOpenCodeStorageParts(messages []openCodeSQLiteMessage, root string) (map[string][]openCodeSQLitePart, error) {
	parts := make(map[string][]openCodeSQLitePart)
	for _, message := range messages {
		partRoot := filepath.Join(root, "storage", "part", message.ID)
		entries, err := os.ReadDir(partRoot)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".json") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(partRoot, entry.Name()))
			if err != nil {
				continue
			}
			var body struct {
				ID        string `json:"id"`
				MessageID string `json:"messageID"`
			}
			if err := json.Unmarshal(data, &body); err != nil || strings.TrimSpace(body.ID) == "" {
				continue
			}
			messageID := firstNonEmptyString(body.MessageID, message.ID)
			parts[messageID] = append(parts[messageID], openCodeSQLitePart{MessageID: messageID, Data: string(data)})
		}
	}
	return parts, nil
}

func (t openCodeStorageTime) created() int64 {
	return firstNonZeroInt64(t.Created, t.Start, t.Updated, t.End)
}

func (t openCodeStorageTime) updated() int64 {
	return firstNonZeroInt64(t.Updated, t.End, t.Created, t.Start)
}

func (t openCodeStorageTime) message() int64 {
	return firstNonZeroInt64(t.Created, t.Start, t.End, t.Updated)
}

func firstNonZeroInt64(values ...int64) int64 {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}

func newerOpenCodeState(left, right scanner.SourceState) scanner.SourceState {
	if right.LastScanTime > left.LastScanTime ||
		(right.LastScanTime == left.LastScanTime && right.LastRecordID > left.LastRecordID) {
		return right
	}
	return left
}
