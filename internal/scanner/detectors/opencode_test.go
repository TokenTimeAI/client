package detectors

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ttime-ai/ttime/client/internal/scanner"
)

func TestOpenCodeDetectorImportsSQLiteDatabase(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	dbPath := filepath.Join(root, "opencode.db")
	db := openFixtureSQLite(t, dbPath)
	defer db.Close()
	execFixtureSQL(t, db, `CREATE TABLE project (
		id TEXT PRIMARY KEY,
		worktree TEXT
	)`)
	execFixtureSQL(t, db, `CREATE TABLE session (
		id TEXT PRIMARY KEY,
		project_id TEXT,
		parent_id TEXT,
		title TEXT,
		time_created INTEGER,
		time_updated INTEGER
	)`)
	execFixtureSQL(t, db, `CREATE TABLE message (
		id TEXT PRIMARY KEY,
		session_id TEXT,
		data TEXT,
		time_created INTEGER
	)`)
	execFixtureSQL(t, db, `CREATE TABLE part (
		id TEXT PRIMARY KEY,
		session_id TEXT,
		message_id TEXT,
		data TEXT,
		time_created INTEGER
	)`)
	execFixtureSQL(t, db, `INSERT INTO project (id, worktree) VALUES (?, ?)`, "proj_1", "/Users/pz/w/ttime")
	execFixtureSQL(t, db, `INSERT INTO session (id, project_id, parent_id, title, time_created, time_updated) VALUES (?, ?, ?, ?, ?, ?)`,
		"ses_agentsview", "proj_1", "", "Import OpenCode", int64(1770292800000), int64(1770293100000))
	execFixtureSQL(t, db, `INSERT INTO message (id, session_id, data, time_created) VALUES (?, ?, ?, ?)`,
		"msg_user", "ses_agentsview", `{"role":"user"}`, int64(1770292810000))
	execFixtureSQL(t, db, `INSERT INTO message (id, session_id, data, time_created) VALUES (?, ?, ?, ?)`,
		"msg_assistant", "ses_agentsview", `{"role":"assistant","modelID":"claude-sonnet-4","tokens":{"input":10,"output":7,"cache":{"read":3,"write":2}}}`, int64(1770292820000))
	execFixtureSQL(t, db, `INSERT INTO part (id, session_id, message_id, data, time_created) VALUES (?, ?, ?, ?, ?)`,
		"part_tool", "ses_agentsview", "msg_assistant", `{"type":"tool","tool":"edit","callID":"tool_1","state":{"input":{"file_path":"app/models/heartbeat_event.rb"}}}`, int64(1770292821000))

	detector := NewOpenCodeDetector().(*OpenCodeDetector)
	detector.configDir = root
	results, _, err := detector.Scan(context.Background(), scanner.SourceState{})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected one result, got %d", len(results))
	}

	result := results[0]
	if result.ConversationID != "ses_agentsview" || result.MessageID != "msg_assistant" {
		t.Fatalf("ids = %q/%q, want ses_agentsview/msg_assistant", result.ConversationID, result.MessageID)
	}
	if result.Project != "ttime" || result.Entity != "/Users/pz/w/ttime" {
		t.Fatalf("project/entity = %q/%q, want ttime//Users/pz/w/ttime", result.Project, result.Entity)
	}
	if result.Model != "claude-sonnet-4" {
		t.Fatalf("model = %q, want claude-sonnet-4", result.Model)
	}
	if result.PromptTokens != 15 || result.CompletionTokens != 7 || result.TotalTokens != 22 {
		t.Fatalf("tokens = P%d C%d T%d, want P15 C7 T22", result.PromptTokens, result.CompletionTokens, result.TotalTokens)
	}
	if len(result.FileEdits) != 1 || result.FileEdits[0].Path != "app/models/heartbeat_event.rb" {
		t.Fatalf("file edits = %#v, want OpenCode tool edit target", result.FileEdits)
	}
}

func TestOpenCodeDetectorImportsStorageLayout(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFixtureFile(t, filepath.Join(root, "storage", "session", "ttime", "ses_storage.json"), `{
		"id":"ses_storage",
		"directory":"/Users/pz/w/ttime",
		"title":"Storage import",
		"time":{"created":1770552000000,"updated":1770552300000}
	}`)
	writeFixtureFile(t, filepath.Join(root, "storage", "message", "ses_storage", "msg_user.json"), `{
		"id":"msg_user",
		"sessionID":"ses_storage",
		"role":"user",
		"time":{"created":1770552010000}
	}`)
	writeFixtureFile(t, filepath.Join(root, "storage", "message", "ses_storage", "msg_assistant.json"), `{
		"id":"msg_assistant",
		"sessionID":"ses_storage",
		"role":"assistant",
		"modelID":"gpt-5-codex",
		"tokens":{"input":20,"output":11,"cache":{"read":4,"write":1}},
		"time":{"created":1770552020000}
	}`)
	writeFixtureFile(t, filepath.Join(root, "storage", "part", "msg_assistant", "part_tool.json"), `{
		"id":"part_tool",
		"sessionID":"ses_storage",
		"messageID":"msg_assistant",
		"type":"tool",
		"tool":"write_file",
		"state":{"input":{"path":"client/internal/scanner/detectors/opencode.go"}},
		"time":{"created":1770552021000}
	}`)

	detector := NewOpenCodeDetector().(*OpenCodeDetector)
	detector.configDir = root
	results, _, err := detector.Scan(context.Background(), scanner.SourceState{})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected one result, got %d", len(results))
	}

	result := results[0]
	if result.ConversationID != "ses_storage" || result.MessageID != "msg_assistant" {
		t.Fatalf("ids = %q/%q, want ses_storage/msg_assistant", result.ConversationID, result.MessageID)
	}
	if result.Project != "ttime" || result.Entity != "/Users/pz/w/ttime" {
		t.Fatalf("project/entity = %q/%q, want ttime//Users/pz/w/ttime", result.Project, result.Entity)
	}
	if result.Model != "gpt-5-codex" {
		t.Fatalf("model = %q, want gpt-5-codex", result.Model)
	}
	if result.PromptTokens != 25 || result.CompletionTokens != 11 || result.TotalTokens != 36 {
		t.Fatalf("tokens = P%d C%d T%d, want P25 C11 T36", result.PromptTokens, result.CompletionTokens, result.TotalTokens)
	}
	if len(result.FileEdits) != 1 || result.FileEdits[0].Path != "client/internal/scanner/detectors/opencode.go" {
		t.Fatalf("file edits = %#v, want OpenCode storage tool target", result.FileEdits)
	}
}

func writeFixtureFile(t *testing.T, path string, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir fixture parent: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture file: %v", err)
	}
}
