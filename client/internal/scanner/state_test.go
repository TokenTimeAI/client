package scanner_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ttime-ai/ttime/client/internal/scanner"
)

func TestStateManagerLoadSave(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	statePath := filepath.Join(tempDir, "scanner-state.json")

	manager := scanner.NewStateManager(statePath)

	// Load non-existent state should return empty state
	state, err := manager.Load()
	if err != nil {
		t.Fatalf("load empty state: %v", err)
	}
	if len(state.Sources) != 0 {
		t.Fatalf("expected empty sources, got %d", len(state.Sources))
	}

	// Update state
	newState := scanner.SourceState{
		LastScanTime: 1700000000,
		LastRecordID: "test-id",
	}
	manager.UpdateSource(&state, "cosine", newState)

	// Save state
	if err := manager.Save(state); err != nil {
		t.Fatalf("save state: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(statePath); os.IsNotExist(err) {
		t.Fatal("state file not created")
	}

	// Load and verify
	loaded, err := manager.Load()
	if err != nil {
		t.Fatalf("load saved state: %v", err)
	}

	cosineState := manager.GetSource(loaded, "cosine")
	if cosineState.LastScanTime != 1700000000 {
		t.Fatalf("expected last scan time 1700000000, got %d", cosineState.LastScanTime)
	}
	if cosineState.LastRecordID != "test-id" {
		t.Fatalf("expected record id 'test-id', got %s", cosineState.LastRecordID)
	}
}

func TestStateManagerMultipleSources(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	statePath := filepath.Join(tempDir, "scanner-state.json")

	manager := scanner.NewStateManager(statePath)
	state, _ := manager.Load()

	// Add multiple sources
	manager.UpdateSource(&state, "cosine", scanner.SourceState{LastScanTime: 100, LastRecordID: "cos-1"})
	manager.UpdateSource(&state, "cline", scanner.SourceState{LastScanTime: 200, LastRecordID: "cli-1"})
	manager.UpdateSource(&state, "cursor", scanner.SourceState{LastScanTime: 300, LastRecordID: "cur-1"})

	manager.Save(state)

	// Load and verify
	loaded, _ := manager.Load()

	tests := []struct {
		name         string
		expectedTime int64
		expectedID   string
	}{
		{"cosine", 100, "cos-1"},
		{"cline", 200, "cli-1"},
		{"cursor", 300, "cur-1"},
	}

	for _, tt := range tests {
		s := manager.GetSource(loaded, tt.name)
		if s.LastScanTime != tt.expectedTime {
			t.Errorf("%s: expected time %d, got %d", tt.name, tt.expectedTime, s.LastScanTime)
		}
		if s.LastRecordID != tt.expectedID {
			t.Errorf("%s: expected id %s, got %s", tt.name, tt.expectedID, s.LastRecordID)
		}
	}
}