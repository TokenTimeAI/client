package detectors

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGenericAgentsViewDetectorImportsQwenUsageAndFileEdits(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	sessionPath := filepath.Join(root, "-Users-pz-w-ttime", "chats", "qwen-session.jsonl")
	if err := os.MkdirAll(filepath.Dir(sessionPath), 0o755); err != nil {
		t.Fatalf("mkdir session dir: %v", err)
	}
	content := "" +
		`{"uuid":"u1","sessionId":"qwen-session","timestamp":"2026-05-13T10:00:00Z","type":"user","cwd":"/Users/pz/w/ttime","message":{"role":"user","parts":[{"text":"Update the importer"}]}}` + "\n" +
		`{"uuid":"u2","sessionId":"qwen-session","timestamp":"2026-05-13T10:01:00Z","type":"assistant","cwd":"/Users/pz/w/ttime","model":"qwen3-coder","message":{"role":"model","parts":[{"text":"I will update the file.","thought":true},{"functionCall":{"id":"tool_1","name":"write_file","args":{"path":"app/services/heartbeat_ingestion.rb"}}}]},"usageMetadata":{"promptTokenCount":100,"candidatesTokenCount":25,"cachedContentTokenCount":10,"totalTokenCount":125}}` + "\n"
	if err := os.WriteFile(sessionPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write session: %v", err)
	}

	result := scanSingleGenericResult(t, "qwen", "Qwen Code sessions", root)
	if result.ConversationID != "qwen-session" {
		t.Fatalf("conversation id = %q, want qwen-session", result.ConversationID)
	}
	if result.Project != "ttime" || result.Entity != "/Users/pz/w/ttime" {
		t.Fatalf("project/entity = %q/%q, want ttime//Users/pz/w/ttime", result.Project, result.Entity)
	}
	if result.Model != "qwen3-coder" {
		t.Fatalf("model = %q, want qwen3-coder", result.Model)
	}
	if result.PromptTokens != 100 || result.CompletionTokens != 25 || result.TotalTokens != 125 {
		t.Fatalf("tokens = P%d C%d T%d, want P100 C25 T125", result.PromptTokens, result.CompletionTokens, result.TotalTokens)
	}
	if len(result.FileEdits) != 1 || result.FileEdits[0].Path != "app/services/heartbeat_ingestion.rb" {
		t.Fatalf("file edits = %#v, want Qwen write_file target", result.FileEdits)
	}
}
