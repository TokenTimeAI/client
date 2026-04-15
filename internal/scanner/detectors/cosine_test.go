package detectors

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ttime-ai/ttime/client/internal/scanner"
)

func TestCosineDetectorMarksLegacySessionsTokenUsageUnknown(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	sessionsDir := filepath.Join(root, "sessions", "2026", "04", "15", "cli_test")
	if err := os.MkdirAll(sessionsDir, 0o755); err != nil {
		t.Fatalf("mkdir sessions dir: %v", err)
	}

	index := `{"sessions":[{"session_id":"cli_test","title":"Legacy session","cwd":"/tmp/ttime","path":"` + sessionsDir + `","end_unix":1776244502,"branch":"main"}]}`
	if err := os.WriteFile(filepath.Join(root, "sessions.json"), []byte(index), 0o644); err != nil {
		t.Fatalf("write sessions.json: %v", err)
	}

	metadata := "" +
		"session_id = 'cli_test'\n" +
		"title = 'Legacy session'\n" +
		"cwd = '/tmp/ttime'\n" +
		"time_started = 2026-04-15T09:12:31Z\n" +
		"time_ended = 2026-04-15T09:15:02Z\n" +
		"duration_seconds = 151\n" +
		"model = 'lumen-outpost'\n"
	if err := os.WriteFile(filepath.Join(sessionsDir, "metadata.toml"), []byte(metadata), 0o644); err != nil {
		t.Fatalf("write metadata.toml: %v", err)
	}

	detector := NewCosineDetector().(*CosineDetector)
	detector.configDir = root

	results, _, err := detector.Scan(context.Background(), scanner.SourceState{})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].TokenUsageKnown {
		t.Fatal("expected token usage to be unknown when legacy metadata omits token fields")
	}
}

func TestCosineDetectorMarksPersistedTokenUsageKnown(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	sessionsDir := filepath.Join(root, "sessions", "2026", "04", "15", "cli_test")
	if err := os.MkdirAll(sessionsDir, 0o755); err != nil {
		t.Fatalf("mkdir sessions dir: %v", err)
	}

	index := `{"sessions":[{"session_id":"cli_test","title":"Persisted usage","cwd":"/tmp/ttime","path":"` + sessionsDir + `","end_unix":1776244502,"branch":"main"}]}`
	if err := os.WriteFile(filepath.Join(root, "sessions.json"), []byte(index), 0o644); err != nil {
		t.Fatalf("write sessions.json: %v", err)
	}

	metadata := "" +
		"session_id = 'cli_test'\n" +
		"title = 'Persisted usage'\n" +
		"cwd = '/tmp/ttime'\n" +
		"time_started = 2026-04-15T09:12:31Z\n" +
		"time_ended = 2026-04-15T09:15:02Z\n" +
		"duration_seconds = 151\n" +
		"prompt_tokens = 120\n" +
		"completion_tokens = 30\n" +
		"total_tokens = 150\n" +
		"model = 'lumen-outpost'\n"
	if err := os.WriteFile(filepath.Join(sessionsDir, "metadata.toml"), []byte(metadata), 0o644); err != nil {
		t.Fatalf("write metadata.toml: %v", err)
	}

	detector := NewCosineDetector().(*CosineDetector)
	detector.configDir = root

	results, _, err := detector.Scan(context.Background(), scanner.SourceState{})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].TokenUsageKnown {
		t.Fatal("expected token usage to be known when metadata includes token fields")
	}
	if results[0].TotalTokens != 150 {
		t.Fatalf("total tokens = %d, want 150", results[0].TotalTokens)
	}
}
