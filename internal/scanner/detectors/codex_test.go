package detectors

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSummarizeCodexSessionTracksCachedTokens(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")
	content := "" +
		`{"timestamp":"2026-04-15T10:00:00Z","type":"session_meta","payload":{"id":"codex-session","timestamp":"2026-04-15T10:00:00Z","cwd":"/Users/pz/w/ttime"}}` + "\n" +
		`{"timestamp":"2026-04-15T10:00:05Z","type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":1200,"cached_input_tokens":400,"output_tokens":50,"total_tokens":1250}}}}` + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write codex session: %v", err)
	}

	summary, ok := summarizeCodexSession(path, nil)
	if !ok {
		t.Fatal("expected session summary")
	}

	if summary.PromptTokens != 800 {
		t.Fatalf("PromptTokens = %d, want 800", summary.PromptTokens)
	}
	if summary.CachedTokens != 400 {
		t.Fatalf("CachedTokens = %d, want 400", summary.CachedTokens)
	}
	if summary.CompletionTokens != 50 {
		t.Fatalf("CompletionTokens = %d, want 50", summary.CompletionTokens)
	}
	if summary.TotalTokens != 1250 {
		t.Fatalf("TotalTokens = %d, want 1250", summary.TotalTokens)
	}
}
