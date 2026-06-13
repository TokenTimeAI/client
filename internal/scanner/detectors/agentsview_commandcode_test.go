package detectors

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGenericAgentsViewDetectorImportsCommandCodeSessionOnly(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	sessionID := "999e4567-e89b-12d3-a456-426614174000"
	projectDir := filepath.Join(root, "users-pz-w-ttime")
	sessionPath := filepath.Join(projectDir, sessionID+".jsonl")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("mkdir project dir: %v", err)
	}
	content := "" +
		`{"id":"m1","timestamp":"2026-06-02T10:00:00Z","sessionId":"` + sessionID + `","role":"user","content":[{"type":"text","text":"Update imports"}],"metadata":{"cwd":"/Users/pz/w/ttime"}}` + "\n" +
		`{"id":"m2","timestamp":"2026-06-02T10:01:00Z","sessionId":"` + sessionID + `","role":"assistant","content":[{"type":"tool-call","toolCallId":"call_1","toolName":"write_file","input":{"path":"app/services/heartbeat_ingestion.rb"}}],"metadata":{"cwd":"/Users/pz/w/ttime"}}` + "\n"
	if err := os.WriteFile(sessionPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write session: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, sessionID+".checkpoints.jsonl"), []byte(`{"sessionId":"checkpoint","timestamp":"2026-06-02T10:02:00Z","content":"sidecar"}`+"\n"), 0o644); err != nil {
		t.Fatalf("write checkpoint sidecar: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, sessionID+".prompts.jsonl"), []byte(`{"sessionId":"prompt","timestamp":"2026-06-02T10:03:00Z","content":"sidecar"}`+"\n"), 0o644); err != nil {
		t.Fatalf("write prompt sidecar: %v", err)
	}

	results := scanGenericResults(t, "commandcode", "Command Code sessions", root)
	if len(results) != 1 {
		t.Fatalf("expected one real session, got %d: %#v", len(results), results)
	}
	result := results[0]
	if result.ConversationID != sessionID {
		t.Fatalf("conversation id = %q, want %s", result.ConversationID, sessionID)
	}
	if result.Project != "ttime" || result.Entity != "/Users/pz/w/ttime" {
		t.Fatalf("project/entity = %q/%q, want ttime//Users/pz/w/ttime", result.Project, result.Entity)
	}
	if len(result.FileEdits) != 1 || result.FileEdits[0].Path != "app/services/heartbeat_ingestion.rb" {
		t.Fatalf("file edits = %#v, want Command Code write_file target", result.FileEdits)
	}
}
