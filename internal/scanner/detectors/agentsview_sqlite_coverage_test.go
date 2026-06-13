package detectors

import (
	"bytes"
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/klauspost/compress/zstd"
	_ "github.com/mattn/go-sqlite3"
	"github.com/ttime-ai/ttime/client/internal/scanner"
)

func TestGenericAgentsViewDetectorImportsForgeSQLite(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	dbPath := filepath.Join(root, ".forge.db")
	db := openFixtureSQLite(t, dbPath)
	defer db.Close()
	execFixtureSQL(t, db, `CREATE TABLE conversations (
		conversation_id TEXT PRIMARY KEY,
		title TEXT,
		context TEXT,
		created_at TEXT,
		updated_at TEXT,
		metrics TEXT
	)`)
	contextJSON := `{"messages":[
		{"message":{"text":{"role":"system","content":"<current_working_directory>/Users/pz/w/ttime</current_working_directory>","timestamp":"2026-05-02T08:00:00Z"}}},
		{"message":{"text":{"role":"user","content":"Scan Forge sessions","timestamp":"2026-05-02T08:01:00Z"}}},
		{"message":{"text":{"role":"assistant","content":"","timestamp":"2026-05-02T08:02:00Z","tool_calls":[{"call_id":"tool_3","name":"edit","arguments":{"file_path":"client/internal/scanner/detectors/agentsview_generic.go"}}]}}}
	]}`
	execFixtureSQL(t, db, `INSERT INTO conversations (conversation_id, title, context, created_at, updated_at, metrics) VALUES (?, ?, ?, ?, ?, ?)`,
		"forge-session", "Forge import", contextJSON, "2026-05-02T08:00:00Z", "2026-05-02T08:03:00Z", `{"input_tokens":10,"output_tokens":5}`)

	result := scanSingleGenericResult(t, "forge", "Forge sessions", root)
	if result.ConversationID != "forge-session" {
		t.Fatalf("conversation id = %q, want forge-session", result.ConversationID)
	}
	if result.Project != "ttime" {
		t.Fatalf("project = %q, want ttime", result.Project)
	}
	if len(result.FileEdits) != 1 || result.FileEdits[0].Path != "client/internal/scanner/detectors/agentsview_generic.go" {
		t.Fatalf("file edits = %#v, want Forge edit target", result.FileEdits)
	}
}

func TestGenericAgentsViewDetectorImportsWarpSQLite(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	dbPath := filepath.Join(root, "warp.sqlite")
	db := openFixtureSQLite(t, dbPath)
	defer db.Close()
	execFixtureSQL(t, db, `CREATE TABLE agent_conversations (
		conversation_id TEXT PRIMARY KEY,
		conversation_data TEXT,
		last_modified_at TEXT
	)`)
	execFixtureSQL(t, db, `CREATE TABLE ai_queries (
		exchange_id TEXT,
		conversation_id TEXT,
		start_ts TEXT,
		input TEXT,
		model_id TEXT,
		working_directory TEXT,
		output_status TEXT
	)`)
	execFixtureSQL(t, db, `INSERT INTO agent_conversations (conversation_id, conversation_data, last_modified_at) VALUES (?, ?, ?)`,
		"warp-session", `{"conversation_usage_metadata":{"token_usage":[{"warp_tokens":21,"byok_tokens":4}]}}`, "2026-05-03 09:05:00")
	execFixtureSQL(t, db, `INSERT INTO ai_queries (exchange_id, conversation_id, start_ts, input, model_id, working_directory, output_status) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"exchange-1", "warp-session", "2026-05-03 09:01:00", `[{"Query":{"text":"Scan Warp sessions"}}]`, "auto", "/Users/pz/w/ttime", "success")

	result := scanSingleGenericResult(t, "warp", "Warp sessions", root)
	if result.ConversationID != "warp-session" {
		t.Fatalf("conversation id = %q, want warp-session", result.ConversationID)
	}
	if result.Project != "ttime" {
		t.Fatalf("project = %q, want ttime", result.Project)
	}
	if result.TotalTokens != 25 {
		t.Fatalf("total tokens = %d, want 25", result.TotalTokens)
	}
	if result.Model != "auto" {
		t.Fatalf("model = %q, want auto", result.Model)
	}
}

func TestGenericAgentsViewDetectorImportsPiebaldSQLite(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	dbPath := filepath.Join(root, "app.db")
	db := openFixtureSQLite(t, dbPath)
	defer db.Close()
	execFixtureSQL(t, db, `CREATE TABLE chats (
		id INTEGER PRIMARY KEY,
		title TEXT,
		created_at TEXT,
		updated_at TEXT,
		message_count INTEGER,
		current_directory TEXT,
		worktree_path TEXT,
		branch_name TEXT,
		project_id INTEGER,
		is_deleted INTEGER
	)`)
	execFixtureSQL(t, db, `CREATE TABLE projects (id INTEGER PRIMARY KEY, directory TEXT, name TEXT)`)
	execFixtureSQL(t, db, `CREATE TABLE messages (
		id INTEGER PRIMARY KEY,
		parent_chat_id INTEGER,
		parent_message_id INTEGER,
		enabled INTEGER,
		role TEXT,
		model TEXT,
		created_at TEXT,
		updated_at TEXT,
		input_tokens INTEGER,
		output_tokens INTEGER,
		reasoning_tokens INTEGER,
		cache_read_tokens INTEGER,
		cache_write_tokens INTEGER,
		status TEXT,
		finish_reason TEXT,
		error TEXT
	)`)
	execFixtureSQL(t, db, `CREATE TABLE message_parts (
		id INTEGER PRIMARY KEY,
		parent_chat_message_id INTEGER,
		part_type TEXT,
		part_index INTEGER
	)`)
	execFixtureSQL(t, db, `CREATE TABLE message_part_tool_call (
		message_part_id INTEGER,
		provider_tool_use_id TEXT,
		tool_name TEXT,
		tool_input TEXT,
		tool_result TEXT,
		tool_error TEXT,
		tool_state TEXT,
		sub_agent_chat_id INTEGER
	)`)
	execFixtureSQL(t, db, `INSERT INTO projects (id, directory, name) VALUES (?, ?, ?)`, 1, "/Users/pz/w/ttime", "ttime")
	execFixtureSQL(t, db, `INSERT INTO chats (id, title, created_at, updated_at, message_count, current_directory, worktree_path, branch_name, project_id, is_deleted) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		7, "Piebald import", "2026-05-04T10:00:00Z", "2026-05-04T10:05:00Z", 2, "/Users/pz/w/ttime", "", "main", 1, 0)
	execFixtureSQL(t, db, `INSERT INTO messages (id, parent_chat_id, enabled, role, model, created_at, updated_at, input_tokens, output_tokens) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		11, 7, 1, "assistant", "piebald-model", "2026-05-04T10:01:00Z", "2026-05-04T10:02:00Z", 8, 13)
	execFixtureSQL(t, db, `INSERT INTO message_parts (id, parent_chat_message_id, part_type, part_index) VALUES (?, ?, ?, ?)`, 21, 11, "tool_call", 0)
	execFixtureSQL(t, db, `INSERT INTO message_part_tool_call (message_part_id, provider_tool_use_id, tool_name, tool_input, tool_result, tool_state) VALUES (?, ?, ?, ?, ?, ?)`,
		21, "tool_4", "piebald_editfile", `{"file_path":"app/controllers/api/v1/heartbeats_controller.rb"}`, "ok", "completed")

	result := scanSingleGenericResult(t, "piebald", "Piebald sessions", root)
	if result.ConversationID != "7" {
		t.Fatalf("conversation id = %q, want 7", result.ConversationID)
	}
	if result.Project != "ttime" {
		t.Fatalf("project = %q, want ttime", result.Project)
	}
	if result.PromptTokens != 8 || result.CompletionTokens != 13 || result.TotalTokens != 21 {
		t.Fatalf("tokens = P%d C%d T%d, want P8 C13 T21", result.PromptTokens, result.CompletionTokens, result.TotalTokens)
	}
	if len(result.FileEdits) != 1 || result.FileEdits[0].Path != "app/controllers/api/v1/heartbeats_controller.rb" {
		t.Fatalf("file edits = %#v, want Piebald edit target", result.FileEdits)
	}
}

func TestGenericAgentsViewDetectorImportsZedSQLite(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	dbPath := filepath.Join(root, "threads", "threads.db")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	db := openFixtureSQLite(t, dbPath)
	defer db.Close()
	execFixtureSQL(t, db, `CREATE TABLE threads (
		id TEXT PRIMARY KEY,
		summary TEXT,
		updated_at TEXT,
		data_type TEXT,
		data BLOB,
		parent_id TEXT,
		folder_paths TEXT,
		created_at TEXT
	)`)
	payload := `{
		"model":{"model":"claude-sonnet-4"},
		"request_token_usage":{"req-1":{"input_tokens":30,"output_tokens":12}},
		"messages":[
			{"User":{"content":[{"Text":"Track Zed threads"}]}},
			{"Agent":{"content":[{"ToolUse":{"id":"tool_5","name":"edit","input":{"file_path":"app/models/heartbeat_event.rb"}}}]}}
		]
	}`
	execFixtureSQL(t, db, `INSERT INTO threads (id, summary, updated_at, data_type, data, parent_id, folder_paths, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"zed-session", "Zed import", "2026-05-05T11:05:00Z", "json", []byte(payload), "", "/Users/pz/w/ttime", "2026-05-05T11:00:00Z")

	result := scanSingleGenericResult(t, "zed", "Zed assistant sessions", root)
	if result.ConversationID != "zed-session" {
		t.Fatalf("conversation id = %q, want zed-session", result.ConversationID)
	}
	if result.Project != "ttime" {
		t.Fatalf("project = %q, want ttime", result.Project)
	}
	if result.Model != "claude-sonnet-4" {
		t.Fatalf("model = %q, want claude-sonnet-4", result.Model)
	}
	if result.PromptTokens != 30 || result.CompletionTokens != 12 || result.TotalTokens != 42 {
		t.Fatalf("tokens = P%d C%d T%d, want P30 C12 T42", result.PromptTokens, result.CompletionTokens, result.TotalTokens)
	}
	if len(result.FileEdits) != 1 || result.FileEdits[0].Path != "app/models/heartbeat_event.rb" {
		t.Fatalf("file edits = %#v, want Zed edit target", result.FileEdits)
	}
}

func TestGenericAgentsViewDetectorImportsZedCompressedSQLite(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	dbPath := filepath.Join(root, "threads", "threads.db")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	db := openFixtureSQLite(t, dbPath)
	defer db.Close()
	execFixtureSQL(t, db, `CREATE TABLE threads (
		id TEXT PRIMARY KEY,
		summary TEXT,
		updated_at TEXT,
		data_type TEXT,
		data BLOB,
		parent_id TEXT,
		folder_paths TEXT,
		created_at TEXT
	)`)
	payload := `{
		"model":{"model":"gpt-5-codex"},
		"request_token_usage":{"req-1":{"input_tokens":11,"output_tokens":6}},
		"messages":[
			{"User":{"content":[{"Text":"Track compressed Zed threads"}]}},
			{"Agent":{"content":[{"ToolUse":{"id":"tool_zstd","name":"write_file","input":{"path":"app/views/home/index.html.erb"}}}]}}
		]
	}`
	execFixtureSQL(t, db, `INSERT INTO threads (id, summary, updated_at, data_type, data, parent_id, folder_paths, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"zed-zstd-session", "Compressed Zed import", "2026-05-07T11:05:00Z", "zstd", zstdFixtureBytes(t, payload), "", "/Users/pz/w/ttime", "2026-05-07T11:00:00Z")

	result := scanSingleGenericResult(t, "zed", "Zed assistant sessions", root)
	if result.ConversationID != "zed-zstd-session" {
		t.Fatalf("conversation id = %q, want zed-zstd-session", result.ConversationID)
	}
	if result.Model != "gpt-5-codex" {
		t.Fatalf("model = %q, want gpt-5-codex", result.Model)
	}
	if result.TotalTokens != 17 {
		t.Fatalf("total tokens = %d, want 17", result.TotalTokens)
	}
	if len(result.FileEdits) != 1 || result.FileEdits[0].Path != "app/views/home/index.html.erb" {
		t.Fatalf("file edits = %#v, want compressed Zed write target", result.FileEdits)
	}
}

func TestGenericAgentsViewDetectorImportsKiroSQLite(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	dbPath := filepath.Join(root, "data.sqlite3")
	db := openFixtureSQLite(t, dbPath)
	defer db.Close()
	execFixtureSQL(t, db, `CREATE TABLE conversations_v2 (
		key TEXT,
		conversation_id TEXT,
		value TEXT,
		created_at INTEGER,
		updated_at INTEGER
	)`)
	payload := `{"history":[
		{
			"user":{
				"env_context":{"env_state":{"current_working_directory":"/Users/pz/w/ttime"}},
				"content":{"Prompt":{"prompt":"Track Kiro sqlite"}},
				"timestamp":"2026-05-06T12:00:00Z"
			},
			"assistant":{
				"ToolUse":{
					"content":"",
					"tool_uses":[{"id":"tool_6","name":"write","args":{"command":"strReplace","path":"app/services/heartbeat_ingestion.rb"}}]
				}
			},
			"request_metadata":{"stream_end_timestamp_ms":1770206405000}
		}
	]}`
	execFixtureSQL(t, db, `INSERT INTO conversations_v2 (key, conversation_id, value, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		"/Users/pz/w/ttime", "kiro-sqlite-session", payload, int64(1770206400000), int64(1770206405000))

	result := scanSingleGenericResult(t, "kiro", "Kiro sessions", root)
	if result.ConversationID != "kiro-sqlite-session" {
		t.Fatalf("conversation id = %q, want kiro-sqlite-session", result.ConversationID)
	}
	if result.Project != "ttime" {
		t.Fatalf("project = %q, want ttime", result.Project)
	}
	if len(result.FileEdits) != 1 || result.FileEdits[0].Path != "app/services/heartbeat_ingestion.rb" {
		t.Fatalf("file edits = %#v, want Kiro sqlite edit target", result.FileEdits)
	}
}

func TestGenericAgentsViewDetectorImportsAntigravitySQLite(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	sessionID := "123e4567-e89b-12d3-a456-426614174000"
	dbPath := filepath.Join(root, "conversations", sessionID+".db")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	db := openFixtureSQLite(t, dbPath)
	defer db.Close()
	execFixtureSQL(t, db, `CREATE TABLE steps (
		idx INTEGER PRIMARY KEY,
		step_type INTEGER,
		step_payload BLOB
	)`)
	execFixtureSQL(t, db, `INSERT INTO steps (idx, step_type, step_payload) VALUES (?, ?, ?)`,
		1, 14, protoStringFixture(17, "Update the import scanner for Antigravity"))
	execFixtureSQL(t, db, `INSERT INTO steps (idx, step_type, step_payload) VALUES (?, ?, ?)`,
		2, 3, protoStringFixture(17, "I will inspect the workspace and update the relevant files."))

	result := scanSingleGenericResult(t, "antigravity", "Antigravity sessions", root)
	if result.ConversationID != sessionID {
		t.Fatalf("conversation id = %q, want %s", result.ConversationID, sessionID)
	}
	if result.Project != "antigravity" {
		t.Fatalf("project = %q, want antigravity", result.Project)
	}
	if result.Duration < 0 {
		t.Fatalf("duration = %f, want non-negative", result.Duration)
	}
}

func TestGenericAgentsViewDetectorImportsAntigravityCLISQLite(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	sessionID := "223e4567-e89b-12d3-a456-426614174000"
	dbPath := filepath.Join(root, "conversations", sessionID+".db")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "history.jsonl"), []byte(
		`{"conversationId":"223e4567-e89b-12d3-a456-426614174000","workspace":"/Users/pz/w/ttime","display":"Update CLI import","timestamp":1770465600000}`+"\n",
	), 0o644); err != nil {
		t.Fatalf("write history: %v", err)
	}
	db := openFixtureSQLite(t, dbPath)
	defer db.Close()
	execFixtureSQL(t, db, `CREATE TABLE steps (
		idx INTEGER PRIMARY KEY,
		step_type INTEGER,
		step_payload BLOB
	)`)
	execFixtureSQL(t, db, `CREATE TABLE gen_metadata (
		idx INTEGER PRIMARY KEY,
		data BLOB
	)`)
	execFixtureSQL(t, db, `INSERT INTO steps (idx, step_type, step_payload) VALUES (?, ?, ?)`,
		1, 14, protoStringFixture(17, "Update CLI import"))
	execFixtureSQL(t, db, `INSERT INTO steps (idx, step_type, step_payload) VALUES (?, ?, ?)`,
		2, 3, protoStringFixture(17, "I updated the Antigravity CLI importer."))
	tokenBlock := appendProtoVarintField(nil, 1, 1020)
	tokenBlock = appendProtoVarintField(tokenBlock, 2, 7)
	tokenBlock = appendProtoVarintField(tokenBlock, 3, 2)
	tokenBlock = appendProtoVarintField(tokenBlock, 5, 10)
	genMetadata := protoStringFixture(21, "gemini-3-pro")
	genMetadata = appendProtoBytesField(genMetadata, 4, tokenBlock)
	execFixtureSQL(t, db, `INSERT INTO gen_metadata (idx, data) VALUES (?, ?)`, 2, genMetadata)

	result := scanSingleGenericResult(t, "antigravity_cli", "Antigravity CLI sessions", root)
	if result.ConversationID != sessionID {
		t.Fatalf("conversation id = %q, want %s", result.ConversationID, sessionID)
	}
	if result.Project != "ttime" || result.Entity != "/Users/pz/w/ttime" {
		t.Fatalf("project/entity = %q/%q, want ttime//Users/pz/w/ttime", result.Project, result.Entity)
	}
	if result.Model != "gemini-3-pro" {
		t.Fatalf("model = %q, want gemini-3-pro", result.Model)
	}
	if result.PromptTokens != 10 || result.CompletionTokens != 9 || result.TotalTokens != 19 {
		t.Fatalf("tokens = P%d C%d T%d, want P10 C9 T19", result.PromptTokens, result.CompletionTokens, result.TotalTokens)
	}
}

func scanSingleGenericResult(t *testing.T, name, description, root string) scanner.ScanResult {
	t.Helper()

	detector := newAgentsViewGenericDetector(agentsViewGenericDefinition{
		Name:        name,
		Description: description,
		Paths:       []string{root},
	})
	detected, err := detector.Detect(context.Background())
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	if !detected {
		t.Fatal("expected detector to find temp root")
	}

	results, _, err := detector.Scan(context.Background(), scanner.SourceState{})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected one result, got %d", len(results))
	}
	return results[0]
}

func openFixtureSQLite(t *testing.T, path string) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	return db
}

func execFixtureSQL(t *testing.T, db *sql.DB, query string, args ...any) {
	t.Helper()

	if _, err := db.Exec(query, args...); err != nil {
		t.Fatalf("exec sqlite: %v", err)
	}
}

func zstdFixtureBytes(t *testing.T, payload string) []byte {
	t.Helper()

	var buf bytes.Buffer
	writer, err := zstd.NewWriter(&buf)
	if err != nil {
		t.Fatalf("create zstd writer: %v", err)
	}
	if _, err := writer.Write([]byte(payload)); err != nil {
		t.Fatalf("write zstd payload: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close zstd writer: %v", err)
	}
	return buf.Bytes()
}

func protoStringFixture(fieldNumber int, value string) []byte {
	return appendProtoBytesField(nil, fieldNumber, []byte(value))
}

func appendProtoBytesField(out []byte, fieldNumber int, value []byte) []byte {
	out = appendProtoVarint(out, uint64(fieldNumber<<3|2))
	out = appendProtoVarint(out, uint64(len(value)))
	return append(out, value...)
}

func appendProtoVarintField(out []byte, fieldNumber int, value uint64) []byte {
	out = appendProtoVarint(out, uint64(fieldNumber<<3))
	return appendProtoVarint(out, value)
}

func appendProtoVarint(out []byte, value uint64) []byte {
	for value >= 0x80 {
		out = append(out, byte(value)|0x80)
		value >>= 7
	}
	return append(out, byte(value))
}
