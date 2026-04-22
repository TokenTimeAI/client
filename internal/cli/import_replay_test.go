package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ttime-ai/ttime/client/internal/scanner"
)

func TestMergeScannerStateKeepsNewestCheckpoint(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	livePath := filepath.Join(dir, "live.json")
	tempPath := filepath.Join(dir, "temp.json")

	liveManager := scanner.NewStateManager(livePath)
	tempManager := scanner.NewStateManager(tempPath)

	liveState := scanner.NewState()
	liveState.Sources["codex"] = scanner.SourceState{LastScanTime: 10, LastRecordID: "a"}
	if err := liveManager.Save(liveState); err != nil {
		t.Fatalf("save live state: %v", err)
	}

	tempState := scanner.NewState()
	tempState.Sources["codex"] = scanner.SourceState{LastScanTime: 20, LastRecordID: "b"}
	if err := tempManager.Save(tempState); err != nil {
		t.Fatalf("save temp state: %v", err)
	}

	if err := mergeScannerState(livePath, tempPath); err != nil {
		t.Fatalf("merge scanner state: %v", err)
	}

	merged, err := liveManager.Load()
	if err != nil {
		t.Fatalf("load merged state: %v", err)
	}
	if merged.Sources["codex"].LastScanTime != 20 {
		t.Fatalf("expected latest scan time to win, got %#v", merged.Sources["codex"])
	}
}

func TestMergeScannerStateCreatesLiveStateIfMissing(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	livePath := filepath.Join(dir, "missing-live.json")
	tempPath := filepath.Join(dir, "temp.json")

	tempManager := scanner.NewStateManager(tempPath)
	tempState := scanner.NewState()
	tempState.Sources["claude_code"] = scanner.SourceState{LastScanTime: 30, LastRecordID: "session-1"}
	if err := tempManager.Save(tempState); err != nil {
		t.Fatalf("save temp state: %v", err)
	}

	if err := mergeScannerState(livePath, tempPath); err != nil {
		t.Fatalf("merge scanner state: %v", err)
	}
	if _, err := os.Stat(livePath); err != nil {
		t.Fatalf("expected live state file to be created: %v", err)
	}
}
