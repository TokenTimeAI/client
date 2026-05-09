package detectors

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSummarizeClaudeSessionIncludesCacheTokens(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")

	// Two assistant turns with realistic Claude Code usage objects that
	// include cache_creation_input_tokens and cache_read_input_tokens.
	lines := []string{
		`{"type":"user","sessionId":"abc","cwd":"/tmp","timestamp":"2026-05-09T10:00:00Z","message":{"content":"hi"}}`,
		`{"type":"assistant","sessionId":"abc","timestamp":"2026-05-09T10:00:05Z","message":{"model":"claude-sonnet-4-6","usage":{"input_tokens":6,"cache_creation_input_tokens":13625,"cache_read_input_tokens":16004,"output_tokens":261}}}`,
		`{"type":"assistant","sessionId":"abc","timestamp":"2026-05-09T10:00:10Z","message":{"model":"claude-sonnet-4-6","usage":{"input_tokens":1,"cache_creation_input_tokens":786,"cache_read_input_tokens":29629,"output_tokens":102}}}`,
	}
	content := ""
	for _, l := range lines {
		content += l + "\n"
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	summary, ok := summarizeClaudeSession(path)
	if !ok {
		t.Fatal("summarizeClaudeSession returned !ok")
	}

	// Per-turn input = input + cache_creation + cache_read.
	// Turn 1: 6 + 13625 + 16004 = 29635
	// Turn 2: 1 + 786 + 29629   = 30416
	wantPrompt := 29635 + 30416
	wantCompletion := 261 + 102
	wantTotal := wantPrompt + wantCompletion

	if summary.PromptTokens != wantPrompt {
		t.Errorf("PromptTokens = %d, want %d (cache tokens must be summed)", summary.PromptTokens, wantPrompt)
	}
	if summary.CompletionTokens != wantCompletion {
		t.Errorf("CompletionTokens = %d, want %d", summary.CompletionTokens, wantCompletion)
	}
	if summary.TotalTokens != wantTotal {
		t.Errorf("TotalTokens = %d, want %d", summary.TotalTokens, wantTotal)
	}
	if summary.Model != "claude-sonnet-4-6" {
		t.Errorf("Model = %q, want claude-sonnet-4-6", summary.Model)
	}
}
