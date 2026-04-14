package queue_test

import (
	"path/filepath"
	"testing"

	"github.com/ttime-ai/ttime/client/internal/api"
	"github.com/ttime-ai/ttime/client/internal/queue"
)

func TestSpoolAppendReadAndClear(t *testing.T) {
	t.Parallel()

	spoolPath := filepath.Join(t.TempDir(), "queue.jsonl")
	spool := queue.New(spoolPath)

	heartbeats := []api.Heartbeat{
		{Entity: "main.go", Type: "file", AgentType: "codex", Time: 1700000000},
		{Entity: "app.rb", Type: "file", AgentType: "claude_code", Time: 1700000060},
	}

	if err := spool.Append(heartbeats); err != nil {
		t.Fatalf("append heartbeats: %v", err)
	}

	loaded, err := spool.ReadAll()
	if err != nil {
		t.Fatalf("read queue: %v", err)
	}
	if len(loaded) != len(heartbeats) {
		t.Fatalf("expected %d heartbeats, got %d", len(heartbeats), len(loaded))
	}

	if err := spool.Clear(); err != nil {
		t.Fatalf("clear queue: %v", err)
	}

	loaded, err = spool.ReadAll()
	if err != nil {
		t.Fatalf("read queue after clear: %v", err)
	}
	if len(loaded) != 0 {
		t.Fatalf("expected empty queue after clear, got %d", len(loaded))
	}
}
