package detectors

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ttime-ai/ttime/client/internal/scanner"
)

var agentsViewDetectorNames = []string{
	"claude_code",
	"codex",
	"copilot",
	"gemini_cli",
	"opencode",
	"openhands",
	"cursor",
	"iflow",
	"amp",
	"zencoder",
	"vscode_copilot",
	"pi",
	"qwen",
	"commandcode",
	"openclaw",
	"qclaw",
	"kimi",
	"claude_ai",
	"chatgpt",
	"kiro",
	"kiro_ide",
	"cortex",
	"hermes",
	"workbuddy",
	"forge",
	"piebald",
	"warp",
	"positron",
	"antigravity",
	"antigravity_cli",
	"zed",
}

func TestRegisteredDetectorsCoverAgentsViewRegistry(t *testing.T) {
	t.Parallel()

	registered := map[string]bool{}
	for _, name := range scanner.ListDetectors() {
		registered[name] = true
	}

	var missing []string
	for _, name := range agentsViewDetectorNames {
		if !registered[name] {
			missing = append(missing, name)
		}
	}

	if len(missing) > 0 {
		t.Fatalf("missing AgentsView detectors: %v", missing)
	}
}

func TestGenericAgentsViewDetectorImportsSessionFile(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	sessionPath := filepath.Join(root, "projects", "ttime", "session-1.jsonl")
	if err := os.MkdirAll(filepath.Dir(sessionPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	content := "" +
		`{"sessionId":"amp-session","cwd":"/Users/pz/w/ttime","timestamp":"2026-04-15T10:00:00Z","role":"user","content":"update dashboard"}` + "\n" +
		`{"id":"msg-1","timestamp":"2026-04-15T10:01:00Z","role":"assistant","model":"amp-model","input_tokens":12,"output_tokens":5,"toolCalls":[{"name":"edit_file","input":{"file_path":"app/views/home/index.html.erb"}}]}` + "\n"
	if err := os.WriteFile(sessionPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write session: %v", err)
	}

	detector := newAgentsViewGenericDetector(agentsViewGenericDefinition{
		Name:        "amp",
		Description: "Amp sessions",
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
		t.Fatalf("expected one session result, got %d", len(results))
	}

	result := results[0]
	if result.AgentType != "amp" {
		t.Fatalf("agent type = %q, want amp", result.AgentType)
	}
	if result.ConversationID != "amp-session" {
		t.Fatalf("conversation id = %q, want amp-session", result.ConversationID)
	}
	if result.Project != "ttime" {
		t.Fatalf("project = %q, want ttime", result.Project)
	}
	if result.TotalTokens != 17 {
		t.Fatalf("total tokens = %d, want 17", result.TotalTokens)
	}
	if len(result.FileEdits) != 1 || result.FileEdits[0].Path != "app/views/home/index.html.erb" {
		t.Fatalf("file edits = %#v, want home view edit", result.FileEdits)
	}
}

func TestGenericAgentsViewDetectorImportsCopilotEventStream(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	sessionPath := filepath.Join(root, "session-state", "copilot-session.jsonl")
	if err := os.MkdirAll(filepath.Dir(sessionPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	content := "" +
		`{"type":"session.start","timestamp":"2026-05-10T09:00:00Z","data":{"sessionId":"copilot-session","context":{"cwd":"/Users/pz/w/ttime","branch":"main"}}}` + "\n" +
		`{"type":"session.model_change","timestamp":"2026-05-10T09:00:01Z","data":{"newModel":"claude-sonnet-4"}}` + "\n" +
		`{"type":"user.message","timestamp":"2026-05-10T09:01:00Z","data":{"content":"Update the importer"}}` + "\n" +
		`{"type":"assistant.message","timestamp":"2026-05-10T09:02:00Z","data":{"content":"","toolRequests":[{"toolCallId":"tool_1","name":"edit_file","arguments":{"file_path":"app/services/heartbeat_ingestion.rb"}}],"outputTokens":7}}` + "\n" +
		`{"type":"session.shutdown","timestamp":"2026-05-10T09:03:00Z","data":{"modelMetrics":{"claude-sonnet-4":{"usage":{"inputTokens":20,"cacheReadTokens":3,"cacheWriteTokens":2,"outputTokens":7,"reasoningTokens":5}}}}}` + "\n"
	if err := os.WriteFile(sessionPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write session: %v", err)
	}

	result := scanSingleGenericResult(t, "copilot", "GitHub Copilot sessions", root)
	if result.ConversationID != "copilot-session" {
		t.Fatalf("conversation id = %q, want copilot-session", result.ConversationID)
	}
	if result.Project != "ttime" || result.Entity != "/Users/pz/w/ttime" {
		t.Fatalf("project/entity = %q/%q, want ttime//Users/pz/w/ttime", result.Project, result.Entity)
	}
	if result.Model != "claude-sonnet-4" {
		t.Fatalf("model = %q, want claude-sonnet-4", result.Model)
	}
	if result.PromptTokens != 20 || result.CachedTokens != 3 || result.CacheCreationTokens != 2 || result.CompletionTokens != 7 || result.ReasoningTokens != 5 || result.TotalTokens != 37 {
		t.Fatalf("tokens = P%d CR%d CW%d C%d R%d T%d, want P20 CR3 CW2 C7 R5 T37", result.PromptTokens, result.CachedTokens, result.CacheCreationTokens, result.CompletionTokens, result.ReasoningTokens, result.TotalTokens)
	}
	if len(result.FileEdits) != 1 || result.FileEdits[0].Path != "app/services/heartbeat_ingestion.rb" {
		t.Fatalf("file edits = %#v, want Copilot edit target", result.FileEdits)
	}
}

func TestGenericAgentsViewDetectorImportsIflowShape(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	sessionPath := filepath.Join(root, "docker-image-retagger", "session-5de701fc-7454-4858-a249-95cac4fd3b51.jsonl")
	if err := os.MkdirAll(filepath.Dir(sessionPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	content := "" +
		`{"uuid":"26ce631f-b136-4829-83af-0a4d901c9cae","parentUuid":null,"sessionId":"session-5de701fc-7454-4858-a249-95cac4fd3b51","timestamp":"2026-01-21T05:56:34.812Z","type":"user","message":{"role":"user","content":"启动app时确保环境变量"},"cwd":"C:\\exp\\docker-image-retagger"}` + "\n" +
		`{"uuid":"2e030941-5d37-4ba4-9006-ba60b6afabbb","parentUuid":"26ce631f-b136-4829-83af-0a4d901c9cae","sessionId":"session-5de701fc-7454-4858-a249-95cac4fd3b51","timestamp":"2026-01-21T05:57:01.968Z","type":"assistant","message":{"id":"response-1768975021968","role":"assistant","content":[{"type":"text","text":"I will modify lib.rs"},{"type":"tool_use","id":"call_74193e6261f04b4f9584ae11","name":"replace","input":{"file_path":"C:\\exp\\docker-image-retagger\\src-tauri\\src\\lib.rs","old_string":"old","new_string":"new"}}],"model":"glm-4.7","usage":{"input_tokens":11,"output_tokens":7}}}` + "\n"
	if err := os.WriteFile(sessionPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write session: %v", err)
	}

	detector := newAgentsViewGenericDetector(agentsViewGenericDefinition{
		Name:        "iflow",
		Description: "iFlow sessions",
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

	result := results[0]
	if result.ConversationID != "session-5de701fc-7454-4858-a249-95cac4fd3b51" {
		t.Fatalf("conversation id = %q", result.ConversationID)
	}
	if result.Project != "docker-image-retagger" {
		t.Fatalf("project = %q, want docker-image-retagger", result.Project)
	}
	if result.TotalTokens != 18 {
		t.Fatalf("total tokens = %d, want 18", result.TotalTokens)
	}
	if len(result.FileEdits) != 1 || result.FileEdits[0].Path != `C:\exp\docker-image-retagger\src-tauri\src\lib.rs` {
		t.Fatalf("file edits = %#v, want iFlow replace target", result.FileEdits)
	}
}

func TestGenericAgentsViewDetectorImportsPiUsageShape(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	sessionPath := filepath.Join(root, "--Users--alice--code--my-project", "pi-test-session-uuid.jsonl")
	if err := os.MkdirAll(filepath.Dir(sessionPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	content := "" +
		`{"type":"session","version":3,"id":"pi-test-session-uuid","timestamp":"2025-01-01T10:00:00Z","cwd":"/Users/alice/code/my-project"}` + "\n" +
		`{"type":"message","id":"entry-2","timestamp":"2025-01-01T10:00:02Z","message":{"role":"assistant","content":[{"type":"text","text":"Looking at the auth module."}],"model":"claude-opus-4-5","usage":{"input":100,"output":50,"totalTokens":150}}}` + "\n" +
		`{"type":"message","id":"entry-7","timestamp":"2025-01-01T10:00:07Z","message":{"role":"assistant","content":[{"type":"text","text":"Looks good!"}],"model":"claude-opus-4-5","usage":{"input":200,"output":10,"totalTokens":210}}}` + "\n"
	if err := os.WriteFile(sessionPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write session: %v", err)
	}

	detector := newAgentsViewGenericDetector(agentsViewGenericDefinition{
		Name:        "pi",
		Description: "Pi sessions",
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

	result := results[0]
	if result.ConversationID != "pi-test-session-uuid" {
		t.Fatalf("conversation id = %q", result.ConversationID)
	}
	if result.Project != "my-project" {
		t.Fatalf("project = %q, want my-project", result.Project)
	}
	if result.PromptTokens != 300 || result.CompletionTokens != 60 || result.TotalTokens != 360 {
		t.Fatalf("tokens = P%d C%d T%d, want P300 C60 T360", result.PromptTokens, result.CompletionTokens, result.TotalTokens)
	}
	if result.Model != "claude-opus-4-5" {
		t.Fatalf("model = %q, want claude-opus-4-5", result.Model)
	}
}

func TestGenericAgentsViewDetectorImportsZencoderShape(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	sessionPath := filepath.Join(root, "sessions", "zen-session.jsonl")
	if err := os.MkdirAll(filepath.Dir(sessionPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	content := "" +
		`{"id":"zen-session","createdAt":"2026-02-01T09:00:00Z","updatedAt":"2026-02-01T09:03:00Z"}` + "\n" +
		`{"role":"system","createdAt":"2026-02-01T09:00:01Z","content":"Environment\nWorking directory: /Users/pz/w/ttime"}` + "\n" +
		`{"role":"user","createdAt":"2026-02-01T09:00:02Z","content":[{"type":"text","tag":"user-input","text":"Add the import scanner."}]}` + "\n" +
		`{"role":"assistant","createdAt":"2026-02-01T09:01:00Z","content":[{"type":"tool-call","toolCallId":"call_1","toolName":"write_file","input":{"path":"internal/scanner/detectors/agentsview_generic.go"}}]}` + "\n"
	if err := os.WriteFile(sessionPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write session: %v", err)
	}

	detector := newAgentsViewGenericDetector(agentsViewGenericDefinition{
		Name:        "zencoder",
		Description: "Zencoder sessions",
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

	result := results[0]
	if result.ConversationID != "zen-session" {
		t.Fatalf("conversation id = %q, want zen-session", result.ConversationID)
	}
	if result.Project != "ttime" {
		t.Fatalf("project = %q, want ttime", result.Project)
	}
	if len(result.FileEdits) != 1 || result.FileEdits[0].Path != "internal/scanner/detectors/agentsview_generic.go" {
		t.Fatalf("file edits = %#v, want zencoder tool-call target", result.FileEdits)
	}
}

func TestGenericAgentsViewDetectorImportsOpenHandsConversationDirectory(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	sessionDir := filepath.Join(root, "open-session")
	eventsDir := filepath.Join(sessionDir, "events")
	if err := os.MkdirAll(eventsDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	baseState := `{"id":"open-session","agent":{"llm":{"model":"gpt-5-codex"}},"workspace":{"cwd":"/Users/pz/w/ttime"}}`
	if err := os.WriteFile(filepath.Join(sessionDir, "base_state.json"), []byte(baseState), 0o644); err != nil {
		t.Fatalf("write base state: %v", err)
	}
	userEvent := `{"kind":"MessageEvent","timestamp":"2026-03-01T12:00:00Z","llm_message":{"role":"user","content":"Make agent imports work."}}`
	actionEvent := `{"kind":"ActionEvent","timestamp":"2026-03-01T12:01:00Z","tool_call_id":"call_2","tool_name":"file_editor","action":{"command":"write","path":"app/models/heartbeat_event.rb"}}`
	if err := os.WriteFile(filepath.Join(eventsDir, "001.json"), []byte(userEvent), 0o644); err != nil {
		t.Fatalf("write user event: %v", err)
	}
	if err := os.WriteFile(filepath.Join(eventsDir, "002.json"), []byte(actionEvent), 0o644); err != nil {
		t.Fatalf("write action event: %v", err)
	}

	detector := newAgentsViewGenericDetector(agentsViewGenericDefinition{
		Name:        "openhands",
		Description: "OpenHands CLI sessions",
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
		t.Fatalf("expected one conversation result, got %d: %#v", len(results), results)
	}

	result := results[0]
	if result.ConversationID != "open-session" {
		t.Fatalf("conversation id = %q, want open-session", result.ConversationID)
	}
	if result.Project != "ttime" {
		t.Fatalf("project = %q, want ttime", result.Project)
	}
	if result.Model != "gpt-5-codex" {
		t.Fatalf("model = %q, want gpt-5-codex", result.Model)
	}
	if len(result.FileEdits) != 1 || result.FileEdits[0].Path != "app/models/heartbeat_event.rb" {
		t.Fatalf("file edits = %#v, want OpenHands file_editor target", result.FileEdits)
	}
}

func TestGenericAgentsViewDetectorImportsKimiWireShape(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	sessionPath := filepath.Join(root, "project-hash", "kimi-session-uuid", "wire.jsonl")
	if err := os.MkdirAll(filepath.Dir(sessionPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	content := "" +
		`{"type":"metadata"}` + "\n" +
		`{"timestamp":1770000000.0,"message":{"type":"TurnBegin","payload":{"user_input":[{"type":"text","text":"Update imports"}]}}}` + "\n" +
		`{"timestamp":1770000001.0,"message":{"type":"ToolCall","payload":{"id":"tool_1","function":{"name":"Write","arguments":"{\"file_path\":\"client/internal/scanner/detectors/agentsview_generic.go\"}"}}}}` + "\n" +
		`{"timestamp":1770000002.0,"message":{"type":"StatusUpdate","payload":{"token_usage":{"output":44},"context_tokens":1200}}}` + "\n"
	if err := os.WriteFile(sessionPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write session: %v", err)
	}

	detector := newAgentsViewGenericDetector(agentsViewGenericDefinition{
		Name:        "kimi",
		Description: "Kimi sessions",
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

	result := results[0]
	if result.ConversationID != "project-hash:kimi-session-uuid" {
		t.Fatalf("conversation id = %q, want project-hash:kimi-session-uuid", result.ConversationID)
	}
	if result.Project != "project-hash" {
		t.Fatalf("project = %q, want project-hash", result.Project)
	}
	if result.CompletionTokens != 44 {
		t.Fatalf("completion tokens = %d, want 44", result.CompletionTokens)
	}
	if len(result.FileEdits) != 1 || result.FileEdits[0].Path != "client/internal/scanner/detectors/agentsview_generic.go" {
		t.Fatalf("file edits = %#v, want Kimi Write target", result.FileEdits)
	}
}

func TestGenericAgentsViewDetectorImportsCortexWorkingDirectory(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	sessionPath := filepath.Join(root, "cortex-session.json")
	body := `{
		"session_id":"cortex-session",
		"working_directory":"/Users/pz/w/ttime",
		"created_at":"2026-05-01T08:00:00Z",
		"last_updated":"2026-05-01T08:05:00Z",
		"history":[
			{"role":"user","id":"m1","content":[{"type":"text","text":"Wire Cortex imports"}]},
			{"role":"assistant","id":"m2","content":[{"type":"tool_use","tool_use":{"tool_use_id":"tool_2","name":"edit","input":{"file_path":"app/services/heartbeat_ingestion.rb"}}}]}
		]
	}`
	if err := os.WriteFile(sessionPath, []byte(body), 0o644); err != nil {
		t.Fatalf("write session: %v", err)
	}

	detector := newAgentsViewGenericDetector(agentsViewGenericDefinition{
		Name:        "cortex",
		Description: "Cortex Code sessions",
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

	result := results[0]
	if result.ConversationID != "cortex-session" {
		t.Fatalf("conversation id = %q, want cortex-session", result.ConversationID)
	}
	if result.Project != "ttime" {
		t.Fatalf("project = %q, want ttime", result.Project)
	}
	if len(result.FileEdits) != 1 || result.FileEdits[0].Path != "app/services/heartbeat_ingestion.rb" {
		t.Fatalf("file edits = %#v, want Cortex edit target", result.FileEdits)
	}
}
