package detectors

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ttime-ai/ttime/client/internal/scanner"
)

func TestSummarizeRoderThreadImportsSessionMetadataAndEvents(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	threadDir := filepath.Join(dir, "threads", "655c5396-bb87-4606-936d-a9f938ce4cf4")
	if err := os.MkdirAll(threadDir, 0o755); err != nil {
		t.Fatalf("mkdir thread dir: %v", err)
	}

	metadata := `{
  "thread_id": "655c5396-bb87-4606-936d-a9f938ce4cf4",
  "title": "fix image input",
  "workspace": "/Users/pz/w/roder",
  "provider": "claude-code",
  "model": "opus",
  "created_at": "2026-06-22T14:11:41.952074Z",
  "updated_at": "2026-06-22T16:04:59.662383Z",
  "message_count": 16,
  "usage": {
    "prompt_tokens": 79332324,
    "completion_tokens": 161242,
    "total_tokens": 79493566
  }
}`
	if err := os.WriteFile(filepath.Join(threadDir, "metadata.json"), []byte(metadata), 0o644); err != nil {
		t.Fatalf("write metadata: %v", err)
	}

	events := "" +
		`{"kind":"turn.started","timestamp":"2026-06-22T14:11:50.294899Z","turn_id":"turn-1","event":{"TurnStarted":{"turn_id":"turn-1","timestamp":"2026-06-22T14:11:50.294893Z"}}}` + "\n" +
		`{"kind":"turn.completed","timestamp":"2026-06-22T14:15:44.277731Z","turn_id":"turn-1","event":{"TurnCompleted":{"turn_id":"turn-1","timestamp":"2026-06-22T14:15:44.277728Z"}}}` + "\n" +
		`{"kind":"workspace/changeObserved","timestamp":"2026-06-22T14:32:21.695041Z","turn_id":"turn-2","event":{"WorkspaceChangeObserved":{"change":{"files":[{"path":"/Users/pz/w/roder/crates/roder-ext-claude-code/Cargo.toml","status":"modified","additions":2,"deletions":1}]}}}}` + "\n" +
		`{"kind":"file.change_preview_ready","timestamp":"2026-06-22T14:32:36.501245Z","turn_id":"turn-2","event":{"FileChangePreviewReady":{"path":"/Users/pz/w/roder/crates/roder-ext-claude-code/src/options.rs","change_type":"modify"}}}` + "\n"
	if err := os.WriteFile(filepath.Join(threadDir, "events.jsonl"), []byte(events), 0o644); err != nil {
		t.Fatalf("write events: %v", err)
	}

	summary, ok, err := summarizeRoderThread(threadDir)
	if err != nil {
		t.Fatalf("summarizeRoderThread: %v", err)
	}
	if !ok {
		t.Fatal("expected thread summary")
	}

	if summary.ThreadID != "655c5396-bb87-4606-936d-a9f938ce4cf4" {
		t.Fatalf("thread id = %q", summary.ThreadID)
	}
	if summary.Workspace != "/Users/pz/w/roder" {
		t.Fatalf("workspace = %q", summary.Workspace)
	}
	if summary.Model != "claude-code/opus" {
		t.Fatalf("model = %q", summary.Model)
	}
	if summary.TotalTokens != 79493566 {
		t.Fatalf("total tokens = %d", summary.TotalTokens)
	}
	if summary.AgentActive < 3*time.Minute {
		t.Fatalf("agent active = %v, want at least 3 minutes", summary.AgentActive)
	}

	cargoEdit, ok := summary.FileEdits["/Users/pz/w/roder/crates/roder-ext-claude-code/Cargo.toml"]
	if !ok {
		t.Fatalf("file edits = %#v", summary.FileEdits)
	}
	if cargoEdit.LinesAdded != 2 || cargoEdit.LinesDeleted != 1 {
		t.Fatalf("cargo edit = %#v", cargoEdit)
	}

	optionsEdit, ok := summary.FileEdits["/Users/pz/w/roder/crates/roder-ext-claude-code/src/options.rs"]
	if !ok {
		t.Fatalf("file edits = %#v", summary.FileEdits)
	}
	if optionsEdit.EditCount != 1 {
		t.Fatalf("options edit = %#v", optionsEdit)
	}
}

func TestRoderDetectorScanImportsThreads(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	threadDir := filepath.Join(dir, "threads", "thread-a")
	if err := os.MkdirAll(threadDir, 0o755); err != nil {
		t.Fatalf("mkdir thread dir: %v", err)
	}

	metadata := `{
  "thread_id": "thread-a",
  "workspace": "/Users/pz/w/project",
  "provider": "codex",
  "model": "gpt-5.5",
  "created_at": "2026-06-14T10:00:00Z",
  "updated_at": "2026-06-14T10:05:00Z",
  "message_count": 2,
  "usage": {"prompt_tokens": 100, "completion_tokens": 20, "total_tokens": 120}
}`
	if err := os.WriteFile(filepath.Join(threadDir, "metadata.json"), []byte(metadata), 0o644); err != nil {
		t.Fatalf("write metadata: %v", err)
	}

	detector := &RoderDetector{dataDir: dir}
	results, state, err := detector.Scan(context.Background(), scanner.SourceState{})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("results = %#v", results)
	}
	if results[0].AgentType != "roder" {
		t.Fatalf("agent type = %q", results[0].AgentType)
	}
	if results[0].ConversationID != "thread-a" {
		t.Fatalf("conversation id = %q", results[0].ConversationID)
	}
	if results[0].TotalTokens != 120 {
		t.Fatalf("total tokens = %d", results[0].TotalTokens)
	}
	if state.LastRecordID != "thread-a" {
		t.Fatalf("state = %#v", state)
	}
}