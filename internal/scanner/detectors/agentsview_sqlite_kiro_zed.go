package detectors

import (
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"github.com/klauspost/compress/zstd"
	"github.com/ttime-ai/ttime/client/internal/scanner"
)

func summarizeKiroSQLite(dbPath string) ([]agentsViewGenericSummary, error) {
	db, err := sql.Open("sqlite3", "file:"+dbPath+"?mode=ro&_busy_timeout=3000")
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query(`
		SELECT key,
		       conversation_id,
		       value,
		       created_at,
		       updated_at
		FROM conversations_v2
		ORDER BY updated_at, conversation_id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var summaries []agentsViewGenericSummary
	for rows.Next() {
		var key, id, value string
		var createdAt, updatedAt int64
		if err := rows.Scan(&key, &id, &value, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		summary := agentsViewGenericSummary{
			SessionID: strings.TrimSpace(id),
			CWD:       strings.TrimSpace(key),
			Project:   projectNameFromPath(key),
			StartedAt: time.UnixMilli(createdAt).UTC(),
			EndedAt:   time.UnixMilli(updatedAt).UTC(),
			FileEdits: make(map[string]scanner.FileEdit),
		}
		visitGenericJSONString(value, &summary)
		if summary.CWD == "" {
			summary.CWD = firstKiroWorkingDirectory(value)
			summary.Project = projectNameFromPath(summary.CWD)
		}
		if summary.TotalTokens == 0 {
			summary.TotalTokens = summary.PromptTokens + summary.CompletionTokens
		}
		if summary.SessionID != "" && !summary.EndedAt.IsZero() {
			summaries = append(summaries, summary)
		}
	}
	return summaries, rows.Err()
}

func firstKiroWorkingDirectory(raw string) string {
	var body any
	if err := json.Unmarshal([]byte(raw), &body); err != nil {
		return ""
	}
	return firstNestedString(body, "current_working_directory")
}

func firstNestedString(value any, key string) string {
	switch typed := value.(type) {
	case map[string]any:
		if found := strings.TrimSpace(stringValue(typed[key])); found != "" {
			return found
		}
		for _, child := range typed {
			if found := firstNestedString(child, key); found != "" {
				return found
			}
		}
	case []any:
		for _, child := range typed {
			if found := firstNestedString(child, key); found != "" {
				return found
			}
		}
	}
	return ""
}

func summarizeZedSQLite(dbPath string) ([]agentsViewGenericSummary, error) {
	db, err := sql.Open("sqlite3", "file:"+dbPath+"?mode=ro&_busy_timeout=3000")
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query(`
		SELECT id,
		       COALESCE(updated_at, ''),
		       COALESCE(data_type, ''),
		       data,
		       COALESCE(folder_paths, ''),
		       COALESCE(created_at, '')
		FROM threads
		WHERE COALESCE(parent_id, '') = ''
		ORDER BY updated_at, id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var summaries []agentsViewGenericSummary
	for rows.Next() {
		var id, updatedAt, dataType, folderPaths, createdAt string
		var payload []byte
		if err := rows.Scan(&id, &updatedAt, &dataType, &payload, &folderPaths, &createdAt); err != nil {
			return nil, err
		}
		payload, ok := decodeZedSQLitePayload(dataType, payload)
		if !ok {
			continue
		}
		cwd := firstFolderPath(folderPaths)
		summary := agentsViewGenericSummary{
			SessionID: strings.TrimSpace(id),
			CWD:       cwd,
			Project:   projectNameFromPath(cwd),
			StartedAt: parseGenericDatabaseTimestamp(createdAt),
			EndedAt:   parseGenericDatabaseTimestamp(updatedAt),
			FileEdits: make(map[string]scanner.FileEdit),
		}
		visitZedThreadPayload(payload, &summary)
		if summary.TotalTokens == 0 {
			summary.TotalTokens = summary.PromptTokens + summary.CompletionTokens
		}
		if summary.EndedAt.IsZero() {
			summary.EndedAt = summary.StartedAt
		}
		if summary.SessionID != "" && !summary.EndedAt.IsZero() {
			summaries = append(summaries, summary)
		}
	}
	return summaries, rows.Err()
}

func decodeZedSQLitePayload(dataType string, payload []byte) ([]byte, bool) {
	switch strings.ToLower(strings.TrimSpace(dataType)) {
	case "", "json":
		return payload, true
	case "zstd":
		decoder, err := zstd.NewReader(nil)
		if err != nil {
			return nil, false
		}
		defer decoder.Close()
		decoded, err := decoder.DecodeAll(payload, nil)
		return decoded, err == nil
	default:
		return nil, false
	}
}

func visitZedThreadPayload(payload []byte, summary *agentsViewGenericSummary) {
	var body any
	if err := json.Unmarshal(payload, &body); err != nil {
		return
	}
	if root, ok := body.(map[string]any); ok {
		if model, ok := root["model"].(map[string]any); ok && summary.Model == "" {
			summary.Model = strings.TrimSpace(stringValue(model["model"]))
		}
	}
	visitGenericJSON(body, summary)
}

func firstFolderPath(paths string) string {
	paths = strings.TrimSpace(paths)
	if paths == "" {
		return ""
	}
	for _, sep := range []string{"\n", "\x00"} {
		if before, _, ok := strings.Cut(paths, sep); ok {
			return strings.TrimSpace(before)
		}
	}
	return paths
}
