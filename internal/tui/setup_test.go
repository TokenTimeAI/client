package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/ttime-ai/ttime/client/internal/api"
	"github.com/ttime-ai/ttime/client/internal/config"
)

func TestSetupModelKeepsPollingForAuthorizationPending(t *testing.T) {
	t.Parallel()

	model := NewSetupModel(config.Config{ServerURL: "https://ttime.example"}, config.Paths{})
	model.phase = phasePolling
	model.auth = api.DeviceAuthorization{IntervalSeconds: 1, ExpiresInSeconds: 60}
	model.authStartedAt = time.Now()

	nextModel, cmd := model.Update(pollResultMsg{
		result: api.DeviceAuthorizationStatus{State: "authorization_pending"},
	})

	updated := nextModel.(SetupModel)
	if updated.phase != phasePolling {
		t.Fatalf("expected phasePolling, got %s", updated.phase)
	}
	if updated.err != nil {
		t.Fatalf("expected no error, got %v", updated.err)
	}
	if cmd == nil {
		t.Fatal("expected another poll command to be scheduled")
	}
}

func TestSetupModelSurfacesClaimedAuthorizationAsTerminalError(t *testing.T) {
	t.Parallel()

	model := NewSetupModel(config.Config{ServerURL: "https://ttime.example"}, config.Paths{})
	model.phase = phasePolling

	nextModel, cmd := model.Update(pollResultMsg{
		result: api.DeviceAuthorizationStatus{State: "authorization_claimed"},
	})

	updated := nextModel.(SetupModel)
	if updated.phase != phaseError {
		t.Fatalf("expected phaseError, got %s", updated.phase)
	}
	if updated.err == nil {
		t.Fatal("expected terminal error for claimed authorization")
	}
	if !strings.Contains(updated.err.Error(), "authorization_claimed") {
		t.Fatalf("expected claimed state in error, got %v", updated.err)
	}
	if cmd != nil {
		t.Fatal("expected no further polling command after terminal error")
	}
}
