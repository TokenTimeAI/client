package detectors

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ttime-ai/ttime/client/internal/scanner"
)

func TestGeminiCLIDetectorImportsCurrentChatsLayout(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	projectPath := "/Users/pz/w/ttime"
	hash := geminiProjectHash(projectPath)
	chatsDir := filepath.Join(root, "tmp", hash, "chats")
	if err := os.MkdirAll(chatsDir, 0o755); err != nil {
		t.Fatalf("mkdir chats: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "projects.json"), []byte(`{"projects":{"/Users/pz/w/ttime":"ttime"}}`), 0o644); err != nil {
		t.Fatalf("write projects: %v", err)
	}
	session := `{
		"sessionId":"gemini-session",
		"startTime":"2026-05-08T10:00:00Z",
		"lastUpdated":"2026-05-08T10:02:00Z",
		"messages":[
			{"id":"u1","type":"user","content":"Update the Rails model","timestamp":"2026-05-08T10:00:00Z"},
			{
				"id":"a1",
				"type":"gemini",
				"content":"I updated the file.",
				"timestamp":"2026-05-08T10:02:00Z",
				"model":"gemini-2.5-pro",
				"tokens":{"input":10,"cached":5,"output":8,"thoughts":2},
				"toolCalls":[{"id":"tool_1","name":"write_file","args":{"file_path":"app/models/heartbeat_event.rb"}}]
			}
		]
	}`
	if err := os.WriteFile(filepath.Join(chatsDir, "session-2026-05-08T10-00-gemini-session.json"), []byte(session), 0o644); err != nil {
		t.Fatalf("write session: %v", err)
	}

	detector := NewGeminiCLIDetector().(*GeminiCLIDetector)
	detector.configDir = root
	results, _, err := detector.Scan(context.Background(), scanner.SourceState{})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected one result, got %d", len(results))
	}

	result := results[0]
	if result.ConversationID != "gemini-session" || result.MessageID != "a1" {
		t.Fatalf("ids = %q/%q, want gemini-session/a1", result.ConversationID, result.MessageID)
	}
	if result.Project != "ttime" || result.Entity != projectPath {
		t.Fatalf("project/entity = %q/%q, want ttime/%s", result.Project, result.Entity, projectPath)
	}
	if result.Model != "gemini-2.5-pro" {
		t.Fatalf("model = %q, want gemini-2.5-pro", result.Model)
	}
	if result.PromptTokens != 15 || result.CompletionTokens != 10 || result.TotalTokens != 25 {
		t.Fatalf("tokens = P%d C%d T%d, want P15 C10 T25", result.PromptTokens, result.CompletionTokens, result.TotalTokens)
	}
	if len(result.FileEdits) != 1 || result.FileEdits[0].Path != "app/models/heartbeat_event.rb" {
		t.Fatalf("file edits = %#v, want Gemini tool edit target", result.FileEdits)
	}
}
