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

	// Turn 1: input=6, cache_create=13625, cache_read=16004, output=261
	// Turn 2: input=1, cache_create=786, cache_read=29629, output=102
	wantPrompt := 6 + 1
	wantCacheCreation := 13625 + 786
	wantCached := 16004 + 29629
	wantCompletion := 261 + 102
	wantTotal := wantPrompt + wantCacheCreation + wantCached + wantCompletion

	if summary.PromptTokens != wantPrompt {
		t.Errorf("PromptTokens = %d, want %d", summary.PromptTokens, wantPrompt)
	}
	if summary.CacheCreationTokens != wantCacheCreation {
		t.Errorf("CacheCreationTokens = %d, want %d", summary.CacheCreationTokens, wantCacheCreation)
	}
	if summary.CachedTokens != wantCached {
		t.Errorf("CachedTokens = %d, want %d", summary.CachedTokens, wantCached)
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
