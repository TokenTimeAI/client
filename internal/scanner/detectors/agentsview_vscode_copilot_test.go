package detectors

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGenericAgentsViewDetectorImportsVSCodeCopilotWorkspaceSession(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	workspace := "/Users/pz/w/ttime"
	hashDir := filepath.Join(root, "workspace-1")
	chatDir := filepath.Join(hashDir, "chatSessions")
	if err := os.MkdirAll(chatDir, 0o755); err != nil {
		t.Fatalf("mkdir chat dir: %v", err)
	}
	workspaceJSON := `{"folder":"file:///Users/pz/w/ttime"}`
	if err := os.WriteFile(filepath.Join(hashDir, "workspace.json"), []byte(workspaceJSON), 0o644); err != nil {
		t.Fatalf("write workspace: %v", err)
	}
	sessionJSON := `{
		"version":3,
		"sessionId":"vscode-copilot-session",
		"creationDate":1770724800000,
		"lastMessageDate":1770724860000,
		"requests":[{
			"requestId":"req-1",
			"message":{"text":"Update importer"},
			"response":[{"value":"Done."}],
			"timestamp":1770724860000,
			"modelId":"copilot/gpt-5"
		}]
	}`
	if err := os.WriteFile(filepath.Join(chatDir, "vscode-copilot-session.json"), []byte(sessionJSON), 0o644); err != nil {
		t.Fatalf("write session: %v", err)
	}

	result := scanSingleGenericResult(t, "vscode_copilot", "VS Code Copilot chat sessions", root)
	if result.ConversationID != "vscode-copilot-session" {
		t.Fatalf("conversation id = %q, want vscode-copilot-session", result.ConversationID)
	}
	if result.Project != "ttime" || result.Entity != workspace {
		t.Fatalf("project/entity = %q/%q, want ttime/%s", result.Project, result.Entity, workspace)
	}
	if result.Model != "copilot/gpt-5" {
		t.Fatalf("model = %q, want copilot/gpt-5", result.Model)
	}
	if result.Duration <= 0 {
		t.Fatalf("duration = %f, want positive VS Code Copilot duration", result.Duration)
	}
}

func TestGenericAgentsViewDetectorImportsVSCodeCopilotJSONLSession(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	workspace := "/Users/pz/w/ttime"
	hashDir := filepath.Join(root, "workspace-jsonl")
	chatDir := filepath.Join(hashDir, "chatSessions")
	if err := os.MkdirAll(chatDir, 0o755); err != nil {
		t.Fatalf("mkdir chat dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(hashDir, "workspace.json"), []byte(`{"folder":"file:///Users/pz/w/ttime"}`), 0o644); err != nil {
		t.Fatalf("write workspace: %v", err)
	}
	initialSnapshot := `{"kind":0,"v":{"version":3,"sessionId":"vscode-copilot-jsonl","creationDate":1770811200000,"lastMessageDate":1770811260000,"requests":[{"requestId":"req-jsonl","message":{"text":"Handle JSONL sessions"},"response":[{"value":"Handled."}],"timestamp":1770811260000,"modelId":"copilot/claude-sonnet-4"}]}}`
	if err := os.WriteFile(filepath.Join(chatDir, "vscode-copilot-jsonl.jsonl"), []byte(initialSnapshot+"\n"), 0o644); err != nil {
		t.Fatalf("write jsonl session: %v", err)
	}

	result := scanSingleGenericResult(t, "vscode_copilot", "VS Code Copilot chat sessions", root)
	if result.ConversationID != "vscode-copilot-jsonl" {
		t.Fatalf("conversation id = %q, want vscode-copilot-jsonl", result.ConversationID)
	}
	if result.Project != "ttime" || result.Entity != workspace {
		t.Fatalf("project/entity = %q/%q, want ttime/%s", result.Project, result.Entity, workspace)
	}
	if result.Model != "copilot/claude-sonnet-4" {
		t.Fatalf("model = %q, want copilot/claude-sonnet-4", result.Model)
	}
	if result.Duration <= 0 {
		t.Fatalf("duration = %f, want positive JSONL session duration", result.Duration)
	}
}
