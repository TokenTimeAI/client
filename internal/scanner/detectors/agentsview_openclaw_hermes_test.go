package detectors

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ttime-ai/ttime/client/internal/scanner"
)

func TestGenericAgentsViewDetectorImportsOpenClawUsageAndFileEdits(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	sessionID := "666e4567-e89b-12d3-a456-426614174000"
	sessionPath := filepath.Join(root, "primary", "sessions", sessionID+".jsonl")
	if err := os.MkdirAll(filepath.Dir(sessionPath), 0o755); err != nil {
		t.Fatalf("mkdir session dir: %v", err)
	}
	content := "" +
		`{"type":"session","id":"` + sessionID + `","cwd":"/Users/pz/w/ttime","timestamp":"2026-06-01T10:00:00Z"}` + "\n" +
		`{"type":"message","timestamp":"2026-06-01T10:01:00Z","message":{"role":"user","content":"Update imports","timestamp":"2026-06-01T10:01:00Z"}}` + "\n" +
		`{"type":"message","timestamp":"2026-06-01T10:02:00Z","message":{"role":"assistant","content":[{"type":"tool_use","id":"tool_1","name":"edit_file","input":{"file_path":"app/services/heartbeat_ingestion.rb"}}],"timestamp":"2026-06-01T10:02:00Z","model":"openclaw-model","usage":{"input":31,"output":9,"cacheRead":4,"cacheWrite":2}}}` + "\n"
	if err := os.WriteFile(sessionPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write session: %v", err)
	}

	result := scanSingleGenericResult(t, "openclaw", "OpenClaw sessions", root)
	if result.ConversationID != "primary:"+sessionID {
		t.Fatalf("conversation id = %q, want primary:%s", result.ConversationID, sessionID)
	}
	if result.Project != "ttime" || result.Entity != "/Users/pz/w/ttime" {
		t.Fatalf("project/entity = %q/%q, want ttime//Users/pz/w/ttime", result.Project, result.Entity)
	}
	if result.Model != "openclaw-model" {
		t.Fatalf("model = %q, want openclaw-model", result.Model)
	}
	if result.PromptTokens != 31 || result.CachedTokens != 4 || result.CacheCreationTokens != 2 || result.CompletionTokens != 9 || result.TotalTokens != 46 {
		t.Fatalf("tokens = P%d CR%d CW%d C%d T%d, want P31 CR4 CW2 C9 T46", result.PromptTokens, result.CachedTokens, result.CacheCreationTokens, result.CompletionTokens, result.TotalTokens)
	}
	if len(result.FileEdits) != 1 || result.FileEdits[0].Path != "app/services/heartbeat_ingestion.rb" {
		t.Fatalf("file edits = %#v, want OpenClaw edit target", result.FileEdits)
	}
}

func TestGenericAgentsViewDetectorImportsQClawUsageAndFileEdits(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	sessionID := "888e4567-e89b-12d3-a456-426614174000"
	sessionPath := filepath.Join(root, "main", "sessions", sessionID+".jsonl")
	if err := os.MkdirAll(filepath.Dir(sessionPath), 0o755); err != nil {
		t.Fatalf("mkdir session dir: %v", err)
	}
	content := "" +
		`{"type":"session","id":"` + sessionID + `","cwd":"/Users/pz/w/ttime","timestamp":"2026-06-01T10:00:00Z"}` + "\n" +
		`{"type":"message","timestamp":"2026-06-01T10:01:00Z","message":{"role":"user","content":"Update imports","timestamp":"2026-06-01T10:01:00Z"}}` + "\n" +
		`{"type":"message","timestamp":"2026-06-01T10:02:00Z","message":{"role":"assistant","content":[{"type":"tool_use","id":"tool_1","name":"write_file","input":{"path":"app/controllers/api/v1/heartbeats_controller.rb"}}],"timestamp":"2026-06-01T10:02:00Z","model":"qclaw-model","usage":{"input":21,"output":7,"cacheRead":3,"cacheWrite":1}}}` + "\n"
	if err := os.WriteFile(sessionPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write session: %v", err)
	}

	result := scanSingleGenericResult(t, "qclaw", "QClaw sessions", root)
	if result.ConversationID != "main:"+sessionID {
		t.Fatalf("conversation id = %q, want main:%s", result.ConversationID, sessionID)
	}
	if result.Project != "ttime" || result.Entity != "/Users/pz/w/ttime" {
		t.Fatalf("project/entity = %q/%q, want ttime//Users/pz/w/ttime", result.Project, result.Entity)
	}
	if result.Model != "qclaw-model" {
		t.Fatalf("model = %q, want qclaw-model", result.Model)
	}
	if result.PromptTokens != 21 || result.CachedTokens != 3 || result.CacheCreationTokens != 1 || result.CompletionTokens != 7 || result.TotalTokens != 32 {
		t.Fatalf("tokens = P%d CR%d CW%d C%d T%d, want P21 CR3 CW1 C7 T32", result.PromptTokens, result.CachedTokens, result.CacheCreationTokens, result.CompletionTokens, result.TotalTokens)
	}
	if len(result.FileEdits) != 1 || result.FileEdits[0].Path != "app/controllers/api/v1/heartbeats_controller.rb" {
		t.Fatalf("file edits = %#v, want QClaw write_file target", result.FileEdits)
	}
}

func TestGenericAgentsViewDetectorImportsHermesToolCalls(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	sessionPath := filepath.Join(root, "20260403_153620_5a3e2ff1.jsonl")
	content := "" +
		`{"role":"session_meta","model":"hermes-model","platform":"cli","timestamp":"2026-04-03T15:36:20Z"}` + "\n" +
		`{"role":"user","content":"Patch the model","timestamp":"2026-04-03T15:36:22Z"}` + "\n" +
		`{"role":"assistant","content":"","timestamp":"2026-04-03T15:36:24Z","model":"hermes-model","usage":{"input":22,"output":8,"cacheRead":3,"cacheWrite":1},"tool_calls":[{"id":"call_1","function":{"name":"write_file","arguments":"{\"path\":\"app/models/heartbeat_event.rb\"}"}}]}` + "\n"
	if err := os.WriteFile(sessionPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write session: %v", err)
	}

	result := scanSingleGenericResult(t, "hermes", "Hermes Agent sessions", root)
	if result.ConversationID != "20260403_153620_5a3e2ff1" {
		t.Fatalf("conversation id = %q, want 20260403_153620_5a3e2ff1", result.ConversationID)
	}
	if result.Project != "hermes-cli" {
		t.Fatalf("project = %q, want hermes-cli", result.Project)
	}
	if result.Model != "hermes-model" {
		t.Fatalf("model = %q, want hermes-model", result.Model)
	}
	if result.PromptTokens != 22 || result.CachedTokens != 3 || result.CacheCreationTokens != 1 || result.CompletionTokens != 8 || result.TotalTokens != 34 {
		t.Fatalf("tokens = P%d CR%d CW%d C%d T%d, want P22 CR3 CW1 C8 T34", result.PromptTokens, result.CachedTokens, result.CacheCreationTokens, result.CompletionTokens, result.TotalTokens)
	}
	if len(result.FileEdits) != 1 || result.FileEdits[0].Path != "app/models/heartbeat_event.rb" {
		t.Fatalf("file edits = %#v, want Hermes write_file target", result.FileEdits)
	}
}

func TestOpenClawDetectorImportsAgentsViewTranscriptWithoutIndex(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	sessionID := "777e4567-e89b-12d3-a456-426614174000"
	sessionPath := filepath.Join(root, "agents", "primary", "sessions", sessionID+".jsonl")
	if err := os.MkdirAll(filepath.Dir(sessionPath), 0o755); err != nil {
		t.Fatalf("mkdir session dir: %v", err)
	}
	content := "" +
		`{"type":"session","id":"` + sessionID + `","cwd":"/Users/pz/w/ttime","timestamp":"2026-06-01T10:00:00Z"}` + "\n" +
		`{"type":"message","timestamp":"2026-06-01T10:01:00Z","message":{"role":"user","content":"Update imports","timestamp":"2026-06-01T10:01:00Z"}}` + "\n" +
		`{"type":"message","timestamp":"2026-06-01T10:02:00Z","message":{"role":"assistant","content":[{"type":"tool_use","id":"tool_1","name":"edit_file","input":{"file_path":"app/services/heartbeat_ingestion.rb"}}],"timestamp":"2026-06-01T10:02:00Z","model":"openclaw-model","usage":{"input":31,"output":9,"cacheRead":4,"cacheWrite":2}}}` + "\n"
	if err := os.WriteFile(sessionPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write session: %v", err)
	}

	detector := &OpenClawDetector{BaseDetector: scanner.NewBaseDetector("openclaw", "OpenClaw agent conversations", []string{root}, 50)}
	detector.dataDir = root
	results, _, err := detector.Scan(context.Background(), scanner.SourceState{})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected one result, got %d", len(results))
	}
	if results[0].ConversationID != "primary:"+sessionID {
		t.Fatalf("conversation id = %q, want primary:%s", results[0].ConversationID, sessionID)
	}
	if results[0].Project != "ttime" || results[0].Entity != "/Users/pz/w/ttime" {
		t.Fatalf("project/entity = %q/%q, want ttime//Users/pz/w/ttime", results[0].Project, results[0].Entity)
	}
	if results[0].PromptTokens != 31 || results[0].CachedTokens != 4 || results[0].CacheCreationTokens != 2 || results[0].CompletionTokens != 9 || results[0].TotalTokens != 46 {
		t.Fatalf("tokens = P%d CR%d CW%d C%d T%d, want P31 CR4 CW2 C9 T46", results[0].PromptTokens, results[0].CachedTokens, results[0].CacheCreationTokens, results[0].CompletionTokens, results[0].TotalTokens)
	}
	if len(results[0].FileEdits) != 1 || results[0].FileEdits[0].Path != "app/services/heartbeat_ingestion.rb" {
		t.Fatalf("file edits = %#v, want OpenClaw edit target", results[0].FileEdits)
	}
}

func TestHermesDetectorImportsAgentsViewTranscript(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	sessionPath := filepath.Join(root, "sessions", "20260403_153620_5a3e2ff1.jsonl")
	if err := os.MkdirAll(filepath.Dir(sessionPath), 0o755); err != nil {
		t.Fatalf("mkdir sessions dir: %v", err)
	}
	content := "" +
		`{"role":"session_meta","model":"hermes-model","platform":"cli","timestamp":"2026-04-03T15:36:20Z"}` + "\n" +
		`{"role":"user","content":"Patch the model","timestamp":"2026-04-03T15:36:22Z"}` + "\n" +
		`{"role":"assistant","content":"","timestamp":"2026-04-03T15:36:24Z","model":"hermes-model","usage":{"input":22,"output":8,"cacheRead":3,"cacheWrite":1},"tool_calls":[{"id":"call_1","function":{"name":"write_file","arguments":"{\"path\":\"app/models/heartbeat_event.rb\"}"}}]}` + "\n"
	if err := os.WriteFile(sessionPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write session: %v", err)
	}

	detector := &HermesDetector{BaseDetector: scanner.NewBaseDetector("hermes", "Hermes Agent conversations", []string{root}, 50)}
	detector.dataDir = root
	results, _, err := detector.Scan(context.Background(), scanner.SourceState{})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected one result, got %d", len(results))
	}
	if results[0].ConversationID != "20260403_153620_5a3e2ff1" {
		t.Fatalf("conversation id = %q, want 20260403_153620_5a3e2ff1", results[0].ConversationID)
	}
	if results[0].Project != "hermes-cli" {
		t.Fatalf("project = %q, want hermes-cli", results[0].Project)
	}
	if results[0].PromptTokens != 22 || results[0].CachedTokens != 3 || results[0].CacheCreationTokens != 1 || results[0].CompletionTokens != 8 || results[0].TotalTokens != 34 {
		t.Fatalf("tokens = P%d CR%d CW%d C%d T%d, want P22 CR3 CW1 C8 T34", results[0].PromptTokens, results[0].CachedTokens, results[0].CacheCreationTokens, results[0].CompletionTokens, results[0].TotalTokens)
	}
	if len(results[0].FileEdits) != 1 || results[0].FileEdits[0].Path != "app/models/heartbeat_event.rb" {
		t.Fatalf("file edits = %#v, want Hermes write_file target", results[0].FileEdits)
	}
}
