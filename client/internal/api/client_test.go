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
