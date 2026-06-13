package detectors

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSummarizeCodexSessionRecordsFunctionCallFileEdits(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")
	content := "" +
		`{"timestamp":"2026-04-15T10:00:00Z","type":"session_meta","payload":{"id":"codex-session","timestamp":"2026-04-15T10:00:00Z","cwd":"/Users/pz/w/ttime"}}` + "\n" +
		`{"timestamp":"2026-04-15T10:00:05Z","type":"response_item","payload":{"type":"function_call","call_id":"call-1","name":"apply_patch","arguments":"{\"patch\":\"*** Begin Patch\n*** Update File: app/views/home/index.html.erb\n@@\n-old\n+new\n*** End Patch\n\"}"}}` + "\n" +
		`{"timestamp":"2026-04-15T10:00:10Z","type":"event_msg","payload":{"type":"task_complete","completed_at":1776247210,"duration_ms":5000}}` + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write codex session: %v", err)
	}

	summary, ok := summarizeCodexSession(path, nil)
	if !ok {
		t.Fatal("expected session summary")
	}

	edit, ok := summary.FileEdits["app/views/home/index.html.erb"]
	if !ok {
		t.Fatalf("expected patched file in edits %#v", summary.FileEdits)
	}
	if edit.EditCount != 1 || edit.LinesAdded != 1 || edit.LinesDeleted != 1 {
		t.Fatalf("edit = %#v, want one edit with one added and one deleted line", edit)
	}
}
