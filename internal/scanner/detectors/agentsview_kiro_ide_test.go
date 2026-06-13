package detectors

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestGenericAgentsViewDetectorImportsKiroIDEWorkspaceSession(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	workspace := "/Users/pz/w/ttime"
	sessionID := "333e4567-e89b-12d3-a456-426614174000"
	sessionDir := filepath.Join(root, "workspace-sessions", "encoded-workspace")
	sessionPath := filepath.Join(sessionDir, sessionID+".json")
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatalf("mkdir session dir: %v", err)
	}
	sessionsIndex := `[{"sessionId":"333e4567-e89b-12d3-a456-426614174000","workspaceDirectory":"/Users/pz/w/ttime","dateCreated":"2026-05-11T10:00:00Z"}]`
	if err := os.WriteFile(filepath.Join(sessionDir, "sessions.json"), []byte(sessionsIndex), 0o644); err != nil {
		t.Fatalf("write sessions index: %v", err)
	}
	sessionJSON := `{
		"sessionId":"333e4567-e89b-12d3-a456-426614174000",
		"title":"Kiro IDE import",
		"workspaceDirectory":"/Users/pz/w/ttime",
		"history":[
			{"message":{"role":"user","content":"Update the importer","id":"u1"}},
			{"message":{"role":"assistant","content":"","id":"a1"},"executionId":"exec-1"}
		]
	}`
	if err := os.WriteFile(sessionPath, []byte(sessionJSON), 0o644); err != nil {
		t.Fatalf("write session: %v", err)
	}
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(workspace)))[:32]
	execDir := filepath.Join(root, hash, "414d1636299d2b9e4ce7e17fb11f63e9")
	if err := os.MkdirAll(execDir, 0o755); err != nil {
		t.Fatalf("mkdir exec dir: %v", err)
	}
	execJSON := `{
		"executionId":"exec-1",
		"actions":[
			{"actionId":"action-1","actionType":"replace","input":{"file":"app/models/heartbeat_event.rb","originalContent":"old\n","modifiedContent":"new\n"}},
			{"actionId":"action-2","actionType":"say","output":{"message":"Updated the Rails model."}}
		]
	}`
	if err := os.WriteFile(filepath.Join(execDir, "exec-1.json"), []byte(execJSON), 0o644); err != nil {
		t.Fatalf("write exec log: %v", err)
	}

	result := scanSingleGenericResult(t, "kiro_ide", "Kiro IDE sessions", root)
	if result.ConversationID != sessionID {
		t.Fatalf("conversation id = %q, want %s", result.ConversationID, sessionID)
	}
	if result.Project != "ttime" || result.Entity != workspace {
		t.Fatalf("project/entity = %q/%q, want ttime/%s", result.Project, result.Entity, workspace)
	}
	if len(result.FileEdits) != 1 || result.FileEdits[0].Path != "app/models/heartbeat_event.rb" {
		t.Fatalf("file edits = %#v, want Kiro IDE replace target", result.FileEdits)
	}
}

func TestGenericAgentsViewDetectorImportsKiroIDEOldChatSession(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	workspace := "/Users/pz/w/ttime"
	workspaceHash := fmt.Sprintf("%x", sha256.Sum256([]byte(workspace)))[:32]
	fileHash := "444e4567e89b12d3a456426614174000"
	chatDir := filepath.Join(root, workspaceHash)
	if err := os.MkdirAll(chatDir, 0o755); err != nil {
		t.Fatalf("mkdir chat dir: %v", err)
	}
	sessionDir := filepath.Join(root, "workspace-sessions", "encoded-workspace")
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatalf("mkdir session dir: %v", err)
	}
	sessionsIndex := `[{"sessionId":"old-session","workspaceDirectory":"/Users/pz/w/ttime","dateCreated":"2026-05-12T10:00:00Z"}]`
	if err := os.WriteFile(filepath.Join(sessionDir, "sessions.json"), []byte(sessionsIndex), 0o644); err != nil {
		t.Fatalf("write sessions index: %v", err)
	}
	chatJSON := `{
		"executionId":"exec-old",
		"actionId":"action-old",
		"metadata":{"modelId":"claude-sonnet-4","startTime":1770638400000,"endTime":1770638700000},
		"chat":[
			{"role":"human","content":"<kiro-ide-message>Track old chat sessions</kiro-ide-message>"},
			{"role":"bot","content":"I tracked the old Kiro IDE chat."}
		]
	}`
	if err := os.WriteFile(filepath.Join(chatDir, fileHash+".chat"), []byte(chatJSON), 0o644); err != nil {
		t.Fatalf("write chat: %v", err)
	}

	result := scanSingleGenericResult(t, "kiro_ide", "Kiro IDE sessions", root)
	wantID := workspaceHash + ":" + fileHash
	if result.ConversationID != wantID {
		t.Fatalf("conversation id = %q, want %s", result.ConversationID, wantID)
	}
	if result.Project != "ttime" || result.Entity != workspace {
		t.Fatalf("project/entity = %q/%q, want ttime/%s", result.Project, result.Entity, workspace)
	}
	if result.Model != "claude-sonnet-4" {
		t.Fatalf("model = %q, want claude-sonnet-4", result.Model)
	}
	if result.Duration <= 0 {
		t.Fatalf("duration = %f, want positive old chat duration", result.Duration)
	}
}
