package detectors

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ttime-ai/ttime/client/internal/scanner"
)

func TestGenericAgentsViewDetectorImportsClaudeAIExportConversationsSeparately(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	body := `[
		{"uuid":"conv-001","name":"First chat","created_at":"2026-01-15T10:00:00.000000Z","updated_at":"2026-01-15T10:30:00.000000Z","chat_messages":[{"uuid":"msg-001","text":"Hello","content":[{"type":"text","text":"Hello"}],"sender":"human","created_at":"2026-01-15T10:00:00.000000Z"}]},
		{"uuid":"conv-empty","name":"Empty chat","created_at":"2026-01-16T10:00:00.000000Z","updated_at":"2026-01-16T10:00:00.000000Z","chat_messages":[]},
		{"uuid":"conv-002","name":"Second chat","created_at":"2026-01-17T08:00:00.000000Z","updated_at":"2026-01-17T08:10:00.000000Z","chat_messages":[{"uuid":"msg-002","text":"What is Go?","content":[{"type":"text","text":"What is Go?"}],"sender":"human","created_at":"2026-01-17T08:00:00.000000Z"}]}
	]`
	if err := os.WriteFile(filepath.Join(root, "conversations.json"), []byte(body), 0o644); err != nil {
		t.Fatalf("write export: %v", err)
	}

	results := scanGenericResults(t, "claude_ai", "Claude.ai exports", root)
	if len(results) != 2 {
		t.Fatalf("expected two non-empty conversations, got %d: %#v", len(results), results)
	}
	if results[0].ConversationID != "conv-001" || results[0].Project != "claude.ai" {
		t.Fatalf("first result = %q/%q, want conv-001/claude.ai", results[0].ConversationID, results[0].Project)
	}
	if results[1].ConversationID != "conv-002" || results[1].Project != "claude.ai" {
		t.Fatalf("second result = %q/%q, want conv-002/claude.ai", results[1].ConversationID, results[1].Project)
	}
}

func TestGenericAgentsViewDetectorImportsChatGPTExportConversationsSeparately(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	body := `[
		{"conversation_id":"chat-001","title":"First chat","create_time":1770000000,"update_time":1770000060,"current_node":"a1","mapping":{"u1":{"id":"u1","parent":null,"children":["a1"],"message":{"id":"u1","create_time":1770000000,"author":{"role":"user"},"content":{"content_type":"text","parts":["Hello"]}}},"a1":{"id":"a1","parent":"u1","children":[],"message":{"id":"a1","create_time":1770000060,"author":{"role":"assistant"},"metadata":{"model_slug":"gpt-4.1"},"content":{"content_type":"text","parts":["Hi"]}}}}},
		{"conversation_id":"chat-002","title":"Second chat","create_time":1770000100,"update_time":1770000160,"current_node":"u2","mapping":{"u2":{"id":"u2","parent":null,"children":[],"message":{"id":"u2","create_time":1770000100,"author":{"role":"user"},"content":{"content_type":"text","parts":["Ship it"]}}}}}
	]`
	if err := os.WriteFile(filepath.Join(root, "conversations.json"), []byte(body), 0o644); err != nil {
		t.Fatalf("write export: %v", err)
	}

	results := scanGenericResults(t, "chatgpt", "ChatGPT exports", root)
	if len(results) != 2 {
		t.Fatalf("expected two conversations, got %d: %#v", len(results), results)
	}
	if results[0].ConversationID != "chat-001" || results[0].Project != "chatgpt.com" {
		t.Fatalf("first result = %q/%q, want chat-001/chatgpt.com", results[0].ConversationID, results[0].Project)
	}
	if results[0].Model != "gpt-4.1" {
		t.Fatalf("model = %q, want gpt-4.1", results[0].Model)
	}
	if results[1].ConversationID != "chat-002" || results[1].Project != "chatgpt.com" {
		t.Fatalf("second result = %q/%q, want chat-002/chatgpt.com", results[1].ConversationID, results[1].Project)
	}
}

func scanGenericResults(t *testing.T, name, description, root string) []scanner.ScanResult {
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
	return results
}
