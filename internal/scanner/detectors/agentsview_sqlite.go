package detectors

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ttime-ai/ttime/client/internal/scanner"
)

func isAgentsViewSQLiteDetector(name string) bool {
	switch name {
	case "antigravity", "antigravity_cli", "forge", "kiro", "piebald", "warp", "zed":
		return true
	default:
		return false
	}
}

func collectSQLiteGenericSummaries(ctx context.Context, name, root string) ([]agentsViewGenericSummary, error) {
	if name == "antigravity" {
		return collectAntigravitySQLiteSummaries(ctx, root)
	}
	if name == "antigravity_cli" {
		return collectAntigravityCLISQLiteSummaries(ctx, root)
	}

	dbs, err := collectGenericSQLiteFiles(ctx, root, sqliteDatabaseNamesForAgent(name))
	if err != nil {
		return nil, err
	}

	var summaries []agentsViewGenericSummary
	for _, dbPath := range dbs {
		var parsed []agentsViewGenericSummary
		switch name {
		case "forge":
			parsed, err = summarizeForgeSQLite(dbPath)
		case "kiro":
			parsed, err = summarizeKiroSQLite(dbPath)
		case "piebald":
			parsed, err = summarizePiebaldSQLite(dbPath)
		case "warp":
			parsed, err = summarizeWarpSQLite(dbPath)
		case "zed":
			parsed, err = summarizeZedSQLite(dbPath)
		}
		if err != nil {
			continue
		}
		summaries = append(summaries, parsed...)
	}
	return summaries, nil
}

func sqliteDatabaseNamesForAgent(name string) map[string]bool {
	switch name {
	case "forge":
		return map[string]bool{".forge.db": true}
	case "kiro":
		return map[string]bool{"data.sqlite3": true}
	case "piebald":
		return map[string]bool{"app.db": true}
	case "warp":
		return map[string]bool{"warp.sqlite": true}
	case "zed":
		return map[string]bool{"threads.db": true}
	default:
		return map[string]bool{}
	}
}

func collectGenericSQLiteFiles(ctx context.Context, root string, names map[string]bool) ([]string, error) {
	paths := make([]string, 0, 4)
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
			if strings.HasPrefix(entry.Name(), ".git") || entry.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if names[entry.Name()] {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk sqlite sessions: %w", err)
	}
	sort.Strings(paths)
	return paths, nil
}

func summarizeForgeSQLite(dbPath string) ([]agentsViewGenericSummary, error) {
	db, err := sql.Open("sqlite3", "file:"+dbPath+"?mode=ro&_busy_timeout=3000")
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query(`
		SELECT conversation_id,
		       COALESCE(context, ''),
		       COALESCE(created_at, ''),
		       COALESCE(updated_at, created_at),
		       COALESCE(metrics, '')
		FROM conversations
		WHERE context IS NOT NULL
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var summaries []agentsViewGenericSummary
	for rows.Next() {
		var id, contextRaw, createdAt, updatedAt, metricsRaw string
		if err := rows.Scan(&id, &contextRaw, &createdAt, &updatedAt, &metricsRaw); err != nil {
			return nil, err
		}
		summary := agentsViewGenericSummary{
			SessionID: strings.TrimSpace(id),
			StartedAt: parseGenericDatabaseTimestamp(createdAt),
			EndedAt:   parseGenericDatabaseTimestamp(updatedAt),
			FileEdits: make(map[string]scanner.FileEdit),
		}
		visitGenericJSONString(contextRaw, &summary)
		visitGenericJSONString(metricsRaw, &summary)
		if summary.EndedAt.IsZero() {
			summary.EndedAt = summary.StartedAt
		}
		if summary.SessionID != "" && !summary.EndedAt.IsZero() {
			summaries = append(summaries, summary)
		}
	}
	return summaries, rows.Err()
}

func summarizePiebaldSQLite(dbPath string) ([]agentsViewGenericSummary, error) {
	db, err := sql.Open("sqlite3", "file:"+dbPath+"?mode=ro&_busy_timeout=3000")
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query(`
		SELECT c.id,
		       c.created_at,
		       COALESCE(c.updated_at, c.created_at),
		       COALESCE(c.current_directory, ''),
		       COALESCE(c.worktree_path, ''),
		       COALESCE(p.directory, ''),
		       COALESCE(p.name, '')
		FROM chats c
		LEFT JOIN projects p ON p.id = c.project_id
		WHERE COALESCE(c.is_deleted, 0) = 0
		  AND c.message_count > 0
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var summaries []agentsViewGenericSummary
	for rows.Next() {
		var id int64
		var createdAt, updatedAt, currentDirectory, worktreePath, projectDirectory, projectName string
		if err := rows.Scan(&id, &createdAt, &updatedAt, &currentDirectory, &worktreePath, &projectDirectory, &projectName); err != nil {
			return nil, err
		}
		cwd := firstNonEmptyString(worktreePath, currentDirectory, projectDirectory)
		summary := agentsViewGenericSummary{
			SessionID: strconv.FormatInt(id, 10),
			CWD:       cwd,
			Project:   firstNonEmptyString(projectName, projectNameFromPath(projectDirectory), projectNameFromPath(cwd)),
			StartedAt: parseGenericDatabaseTimestamp(createdAt),
			EndedAt:   parseGenericDatabaseTimestamp(updatedAt),
			FileEdits: make(map[string]scanner.FileEdit),
		}
		visitPiebaldMessageUsage(db, id, &summary)
		visitPiebaldToolCalls(db, id, &summary)
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

func visitPiebaldMessageUsage(db *sql.DB, chatID int64, summary *agentsViewGenericSummary) {
	rows, err := db.Query(`
		SELECT COALESCE(model, ''),
		       created_at,
		       COALESCE(updated_at, created_at),
		       input_tokens,
		       output_tokens,
		       reasoning_tokens,
		       cache_read_tokens,
		       cache_write_tokens
		FROM messages
		WHERE parent_chat_id = ?
		  AND COALESCE(enabled, 1) != 0
		ORDER BY created_at, id
	`, chatID)
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var model, createdAt, updatedAt string
		var inputTokens, outputTokens, reasoningTokens, cacheReadTokens, cacheWriteTokens sql.NullInt64
		if err := rows.Scan(&model, &createdAt, &updatedAt, &inputTokens, &outputTokens, &reasoningTokens, &cacheReadTokens, &cacheWriteTokens); err != nil {
			return
		}
		if summary.Model == "" {
			summary.Model = strings.TrimSpace(model)
		}
		for _, ts := range []time.Time{parseGenericDatabaseTimestamp(createdAt), parseGenericDatabaseTimestamp(updatedAt)} {
			if ts.IsZero() {
				continue
			}
			if summary.StartedAt.IsZero() || ts.Before(summary.StartedAt) {
				summary.StartedAt = ts
			}
			if ts.After(summary.EndedAt) {
				summary.EndedAt = ts
			}
		}
		if inputTokens.Valid {
			summary.PromptTokens += int(inputTokens.Int64)
		}
		if cacheReadTokens.Valid {
			summary.PromptTokens += int(cacheReadTokens.Int64)
		}
		if cacheWriteTokens.Valid {
			summary.PromptTokens += int(cacheWriteTokens.Int64)
		}
		if outputTokens.Valid {
			summary.CompletionTokens += int(outputTokens.Int64)
		}
		if reasoningTokens.Valid {
			summary.CompletionTokens += int(reasoningTokens.Int64)
		}
	}
}

func visitPiebaldToolCalls(db *sql.DB, chatID int64, summary *agentsViewGenericSummary) {
	rows, err := db.Query(`
		SELECT COALESCE(mpt.tool_name, ''),
		       COALESCE(mpt.tool_input, '')
		FROM messages m
		JOIN message_parts mp ON mp.parent_chat_message_id = m.id
		JOIN message_part_tool_call mpt ON mpt.message_part_id = mp.id
		WHERE m.parent_chat_id = ?
		  AND COALESCE(m.enabled, 1) != 0
		ORDER BY m.created_at, m.id, mp.part_index, mp.id
	`, chatID)
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var toolName, toolInput string
		if err := rows.Scan(&toolName, &toolInput); err != nil {
			return
		}
		mergeFileEdits(summary.FileEdits, fileEditsFromToolCallJSON(toolName, toolInput))
	}
}

func summarizeWarpSQLite(dbPath string) ([]agentsViewGenericSummary, error) {
	db, err := sql.Open("sqlite3", "file:"+dbPath+"?mode=ro&_busy_timeout=3000")
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query(`
		SELECT conversation_id,
		       COALESCE(conversation_data, '{}'),
		       COALESCE(last_modified_at, '')
		FROM agent_conversations
		ORDER BY last_modified_at, conversation_id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var summaries []agentsViewGenericSummary
	for rows.Next() {
		var id, dataRaw, lastModified string
		if err := rows.Scan(&id, &dataRaw, &lastModified); err != nil {
			return nil, err
		}
		summary := agentsViewGenericSummary{
			SessionID: strings.TrimSpace(id),
			EndedAt:   parseGenericDatabaseTimestamp(lastModified),
			FileEdits: make(map[string]scanner.FileEdit),
		}
		visitWarpConversationData(dataRaw, &summary)
		visitWarpQueryRows(db, summary.SessionID, &summary)
		if summary.StartedAt.IsZero() {
			summary.StartedAt = summary.EndedAt
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

func visitGenericJSONString(raw string, summary *agentsViewGenericSummary) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return
	}
	var body any
	if err := json.Unmarshal([]byte(raw), &body); err == nil {
		visitGenericJSON(body, summary)
	}
}

func visitWarpConversationData(raw string, summary *agentsViewGenericSummary) {
	var body any
	if err := json.Unmarshal([]byte(raw), &body); err != nil {
		return
	}
	visitGenericJSON(body, summary)
	accumulateWarpTokens(body, summary)
}

func accumulateWarpTokens(value any, summary *agentsViewGenericSummary) {
	switch typed := value.(type) {
	case map[string]any:
		summary.TotalTokens += intValue(typed["warp_tokens"]) + intValue(typed["byok_tokens"])
		for _, child := range typed {
			accumulateWarpTokens(child, summary)
		}
	case []any:
		for _, child := range typed {
			accumulateWarpTokens(child, summary)
		}
	}
}

func visitWarpQueryRows(db *sql.DB, conversationID string, summary *agentsViewGenericSummary) {
	rows, err := db.Query(`
		SELECT COALESCE(start_ts, ''),
		       COALESCE(model_id, ''),
		       COALESCE(working_directory, '')
		FROM ai_queries
		WHERE conversation_id = ?
		ORDER BY start_ts
	`, conversationID)
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var startedAt, model, cwd string
		if err := rows.Scan(&startedAt, &model, &cwd); err != nil {
			return
		}
		ts := parseGenericDatabaseTimestamp(startedAt)
		if !ts.IsZero() {
			if summary.StartedAt.IsZero() || ts.Before(summary.StartedAt) {
				summary.StartedAt = ts
			}
			if ts.After(summary.EndedAt) {
				summary.EndedAt = ts
			}
		}
		if summary.Model == "" {
			summary.Model = strings.TrimSpace(model)
		}
		if summary.CWD == "" {
			summary.CWD = strings.TrimSpace(cwd)
		}
	}
}

func parseGenericDatabaseTimestamp(raw string) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}
	}
	if ts := parseRFC3339Any(raw); !ts.IsZero() {
		return ts.UTC()
	}
	for _, layout := range []string{
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05.999999",
		"2006-01-02 15:04:05",
	} {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return parsed.UTC()
		}
	}
	return time.Time{}
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
