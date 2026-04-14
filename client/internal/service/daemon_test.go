package service_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/ttime-ai/ttime/client/internal/api"
	"github.com/ttime-ai/ttime/client/internal/collector"
	"github.com/ttime-ai/ttime/client/internal/queue"
	"github.com/ttime-ai/ttime/client/internal/service"
)

func TestDaemonRunOnceProcessesFixtureInbox(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	inboxDir := filepath.Join(tempDir, "inbox")
	if err := os.MkdirAll(inboxDir, 0o755); err != nil {
		t.Fatalf("mkdir inbox: %v", err)
	}

	fixture, err := os.ReadFile(filepath.Join("..", "..", "testdata", "inbox", "events.jsonl"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(inboxDir, "events.jsonl"), fixture, 0o644); err != nil {
		t.Fatalf("write inbox file: %v", err)
	}

	var received []api.Heartbeat
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/heartbeats/bulk" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"responses":[[201,{}],[201,{}]]}`))
	}))
	defer server.Close()

	daemon := service.Daemon{
		Collector:   collector.NewJSONLCollector(inboxDir, filepath.Join(tempDir, "collector-state.json")),
		Queue:       queue.New(filepath.Join(tempDir, "queue.jsonl")),
		Sender:      api.NewClient(server.URL, "tt_test"),
		MachineName: "builder",
	}

	result, err := daemon.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("run once: %v", err)
	}
	if result.Collected != 2 || result.Sent != 2 {
		t.Fatalf("unexpected result: %#v", result)
	}
	if len(received) != 2 {
		t.Fatalf("expected 2 uploaded heartbeats, got %d", len(received))
	}
	if received[0].Machine != "builder" {
		t.Fatalf("expected normalized machine name, got %q", received[0].Machine)
	}
}

func TestDaemonRetriesQueuedEvents(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	inboxDir := filepath.Join(tempDir, "inbox")
	if err := os.MkdirAll(inboxDir, 0o755); err != nil {
		t.Fatalf("mkdir inbox: %v", err)
	}
	if err := os.WriteFile(filepath.Join(inboxDir, "events.jsonl"), []byte(`{"entity":"main.go","type":"file","agent_type":"codex","time":1700000000}
`), 0o644); err != nil {
		t.Fatalf("write inbox file: %v", err)
	}

	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			http.Error(w, "upstream unavailable", http.StatusBadGateway)
			return
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"responses":[[201,{}]]}`))
	}))
	defer server.Close()

	spool := queue.New(filepath.Join(tempDir, "queue.jsonl"))
	daemon := service.Daemon{
		Collector:   collector.NewJSONLCollector(inboxDir, filepath.Join(tempDir, "collector-state.json")),
		Queue:       spool,
		Sender:      api.NewClient(server.URL, "tt_test"),
		MachineName: "builder",
	}

	if _, err := daemon.RunOnce(context.Background()); err == nil {
		t.Fatal("expected first run to fail")
	}

	queued, err := spool.ReadAll()
	if err != nil {
		t.Fatalf("read spool: %v", err)
	}
	if len(queued) != 1 {
		t.Fatalf("expected 1 queued heartbeat after failure, got %d", len(queued))
	}

	result, err := daemon.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("run once retry: %v", err)
	}
	if result.QueuedPreviously != 1 || result.Sent != 1 {
		t.Fatalf("unexpected retry result: %#v", result)
	}

	queued, err = spool.ReadAll()
	if err != nil {
		t.Fatalf("read spool after retry: %v", err)
	}
	if len(queued) != 0 {
		t.Fatalf("expected empty spool after successful retry, got %d", len(queued))
	}
}
