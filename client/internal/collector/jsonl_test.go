package collector_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ttime-ai/ttime/client/internal/collector"
)

func TestJSONLCollectorParsesAndTracksOffsets(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	inboxDir := filepath.Join(tempDir, "inbox")
	if err := os.MkdirAll(inboxDir, 0o755); err != nil {
		t.Fatalf("mkdir inbox: %v", err)
	}

	filePath := filepath.Join(inboxDir, "events.jsonl")
	content := `{"entity":"main.go","type":"file","project":"demo","agent_type":"codex","time":1700000000,"metadata":{"tool":"write"}}
{"entity":"app.rb","type":"file","project":"demo","agent_type":"claude_code","time":"1700000001.5","is_write":true,"lines_added":4}
`
	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	collectorState := filepath.Join(tempDir, "collector-state.json")
	c := collector.NewJSONLCollector(inboxDir, collectorState)

	events, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("collect events: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].Metadata["tool"] != "write" {
		t.Fatalf("expected metadata to survive parsing, got %#v", events[0].Metadata)
	}
	if events[1].LinesAdded != 4 {
		t.Fatalf("expected lines added to parse, got %d", events[1].LinesAdded)
	}

	events, err = c.Collect(context.Background())
	if err != nil {
		t.Fatalf("collect events second time: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("expected offset tracking to avoid duplicates, got %d events", len(events))
	}
}
