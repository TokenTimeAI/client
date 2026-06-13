package detectors

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGenericAgentsViewDetectorImportsWorkBuddyFunctionCallUsage(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	sessionID := "555e4567-e89b-12d3-a456-426614174000"
	sessionPath := filepath.Join(root, "ttime", sessionID+".jsonl")
	if err := os.MkdirAll(filepath.Dir(sessionPath), 0o755); err != nil {
		t.Fatalf("mkdir session dir: %v", err)
	}
	content := "" +
		`{"type":"message","role":"user","content":"Update WorkBuddy imports","cwd":"/Users/pz/w/ttime","timestamp":1770984000000}` + "\n" +
		`{"type":"function_call","name":"write_file","callId":"call_1","arguments":{"path":"app/models/heartbeat_event.rb"},"cwd":"/Users/pz/w/ttime","timestamp":1770984060000,"model":"claude-sonnet-4","usage":{"input":40,"output":12,"cacheRead":5,"cacheWrite":3}}` + "\n" +
		`{"type":"function_call_result","callId":"call_1","output":"ok","cwd":"/Users/pz/w/ttime","timestamp":1770984070000}` + "\n"
	if err := os.WriteFile(sessionPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write session: %v", err)
	}

	result := scanSingleGenericResult(t, "workbuddy", "WorkBuddy sessions", root)
	if result.ConversationID != sessionID {
		t.Fatalf("conversation id = %q, want %s", result.ConversationID, sessionID)
	}
	if result.Project != "ttime" || result.Entity != "/Users/pz/w/ttime" {
		t.Fatalf("project/entity = %q/%q, want ttime//Users/pz/w/ttime", result.Project, result.Entity)
	}
	if result.Model != "claude-sonnet-4" {
		t.Fatalf("model = %q, want claude-sonnet-4", result.Model)
	}
	if result.PromptTokens != 48 || result.CompletionTokens != 12 || result.TotalTokens != 60 {
		t.Fatalf("tokens = P%d C%d T%d, want P48 C12 T60", result.PromptTokens, result.CompletionTokens, result.TotalTokens)
	}
	if len(result.FileEdits) != 1 || result.FileEdits[0].Path != "app/models/heartbeat_event.rb" {
		t.Fatalf("file edits = %#v, want WorkBuddy function call target", result.FileEdits)
	}
}
