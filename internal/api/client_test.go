package api_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ttime-ai/ttime/client/internal/api"
)

func TestSendHeartbeatsFormatsBulkRequest(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/heartbeats/bulk" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer tt_test" {
			t.Fatalf("unexpected auth header: %s", got)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Fatalf("unexpected content type: %s", got)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}

		var payload []map[string]any
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if len(payload) != 1 {
			t.Fatalf("expected 1 heartbeat, got %d", len(payload))
		}
		if payload[0]["type"] != "file" {
			t.Fatalf("expected normalized heartbeat type, got %#v", payload[0]["type"])
		}
		if payload[0]["entity"] != "main.go" {
			t.Fatalf("expected entity main.go, got %#v", payload[0]["entity"])
		}

		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"responses":[[201,{"data":{"id":"1"}}]]}`))
	}))
	defer server.Close()

	client := api.NewClient(server.URL, "tt_test")
	err := client.SendHeartbeats(context.Background(), []api.Heartbeat{{
		Entity:    "main.go",
		Type:      "file",
		AgentType: "codex",
		Time:      1700000000,
	}})
	if err != nil {
		t.Fatalf("send heartbeats: %v", err)
	}
}

func TestSendHeartbeatsDetailedParsesCreatedVsUpdated(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"responses":[[201,{}],[200,{}]]}`))
	}))
	defer server.Close()

	client := api.NewClient(server.URL, "tt_test")
	result, err := client.SendHeartbeatsDetailed(context.Background(), []api.Heartbeat{
		{Entity: "a", Type: "conversation", AgentType: "codex", Time: 1},
		{Entity: "b", Type: "conversation", AgentType: "codex", Time: 2},
	})
	if err != nil {
		t.Fatalf("send heartbeats detailed: %v", err)
	}
	if len(result.Responses) != 2 || result.Responses[0].StatusCode != 201 || result.Responses[1].StatusCode != 200 {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestCreateAndUpdateImportRun(t *testing.T) {
	t.Parallel()

	var methodCalls []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methodCalls = append(methodCalls, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/import_runs":
			_, _ = w.Write([]byte(`{"data":{"id":"run-1","machine":"workstation","trigger_kind":"replay","status":"running","started_at":"2026-04-22T10:00:00Z","agent_filters":["codex"],"replay_all":false}}`))
		case r.Method == http.MethodPatch && r.URL.Path == "/api/v1/import_runs/run-1":
			_, _ = w.Write([]byte(`{"data":{"id":"run-1","machine":"workstation","trigger_kind":"replay","status":"completed","started_at":"2026-04-22T10:00:00Z","completed_at":"2026-04-22T10:05:00Z","sessions_seen":4,"sessions_imported":3,"sessions_updated":1,"sessions_skipped":0}}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	client := api.NewClient(server.URL, "tt_test")
	run, err := client.CreateImportRun(context.Background(), api.ImportRun{
		Machine:      "workstation",
		TriggerKind:  "replay",
		Status:       "running",
		AgentFilters: []string{"codex"},
	})
	if err != nil {
		t.Fatalf("create import run: %v", err)
	}
	if run.ID != "run-1" || run.Status != "running" {
		t.Fatalf("unexpected created run: %#v", run)
	}

	run, err = client.UpdateImportRun(context.Background(), api.ImportRun{
		ID:               "run-1",
		Status:           "completed",
		SessionsSeen:     4,
		SessionsImported: 3,
		SessionsUpdated:  1,
	})
	if err != nil {
		t.Fatalf("update import run: %v", err)
	}
	if run.Status != "completed" || run.SessionsImported != 3 {
		t.Fatalf("unexpected updated run: %#v", run)
	}
	if len(methodCalls) != 2 {
		t.Fatalf("expected two API calls, got %#v", methodCalls)
	}
}

func TestCreateDeviceAuthorizationParsesEnvelope(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/device_authorizations" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": {
				"user_code": "ABCD-EFGH",
				"verification_uri": "https://ttime.example/activate",
				"device_code": "device-123",
				"interval": 5,
				"expires_in": 600
			}
		}`))
	}))
	defer server.Close()

	client := api.NewClient(server.URL, "")
	auth, err := client.CreateDeviceAuthorization(context.Background(), "workstation")
	if err != nil {
		t.Fatalf("create device authorization: %v", err)
	}
	if auth.UserCode != "ABCD-EFGH" {
		t.Fatalf("expected user code, got %q", auth.UserCode)
	}
	if auth.VerificationURL != "https://ttime.example/activate" {
		t.Fatalf("expected verification url, got %q", auth.VerificationURL)
	}
	if auth.IntervalSeconds != 5 {
		t.Fatalf("expected interval 5, got %d", auth.IntervalSeconds)
	}
}

func TestPollDeviceAuthorizationTreatsPendingAsState(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/device_authorizations/poll" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		http.Error(w, `{"error":"authorization_pending"}`, http.StatusBadRequest)
	}))
	defer server.Close()

	client := api.NewClient(server.URL, "")
	status, err := client.PollDeviceAuthorization(context.Background(), api.DeviceAuthorization{
		DeviceCode: "device-123",
	})
	if err != nil {
		t.Fatalf("poll device authorization: %v", err)
	}
	if status.State != "authorization_pending" {
		t.Fatalf("expected authorization_pending state, got %q", status.State)
	}
}

func TestPollDeviceAuthorizationTreatsClaimedAsState(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusGone)
		_, _ = w.Write([]byte(`{"error":"authorization_claimed"}`))
	}))
	defer server.Close()

	client := api.NewClient(server.URL, "")
	status, err := client.PollDeviceAuthorization(context.Background(), api.DeviceAuthorization{
		DeviceCode: "device-123",
	})
	if err != nil {
		t.Fatalf("poll device authorization: %v", err)
	}
	if status.State != "authorization_claimed" {
		t.Fatalf("expected authorization_claimed state, got %q", status.State)
	}
}

func TestPollDeviceAuthorizationReturnsErrorForMalformedFailureBody(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "upstream exploded", http.StatusBadGateway)
	}))
	defer server.Close()

	client := api.NewClient(server.URL, "")
	_, err := client.PollDeviceAuthorization(context.Background(), api.DeviceAuthorization{
		DeviceCode: "device-123",
	})
	if err == nil {
		t.Fatal("expected malformed failure body to return an error")
	}
}
