package platform

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallSystemdRemovesUnitFileWhenEnableFails(t *testing.T) {
	// Note: This test modifies global state, cannot run in parallel
	tempDir := t.TempDir()
	manager := &UserServiceManager{}

	restore := stubPlatformTestEnv(t, tempDir, func(name string, args ...string) error {
		// daemon-reload should succeed
		if name == "systemctl" && len(args) >= 2 && args[0] == "--user" && args[1] == "daemon-reload" {
			return nil
		}
		// enable --now should fail
		if name == "systemctl" && len(args) >= 3 && args[0] == "--user" && args[1] == "enable" {
			return fmt.Errorf("failed to connect to bus")
		}
		// disable --now during cleanup should succeed
		if name == "systemctl" && len(args) >= 3 && args[0] == "--user" && args[1] == "disable" {
			return nil
		}
		return nil
	})
	defer restore()

	err := manager.installSystemd("/usr/local/bin/ttime")
	if err == nil {
		t.Fatal("expected installSystemd to fail")
	}

	unitPath, pathErr := manager.systemdPath()
	if pathErr != nil {
		t.Fatalf("systemdPath: %v", pathErr)
	}
	if _, statErr := os.Stat(unitPath); !os.IsNotExist(statErr) {
		t.Fatalf("expected unit file cleanup after failed install, stat err=%v", statErr)
	}
}

func TestStatusLinuxRequiresEnabledOrActiveUnit(t *testing.T) {
	// Note: This test modifies global state, cannot run in parallel
	tempDir := t.TempDir()
	manager := &UserServiceManager{}

	restore := stubPlatformTestEnv(t, tempDir, func(name string, args ...string) error {
		// Both is-enabled and is-active should fail to simulate unconfirmed unit
		if name == "systemctl" && len(args) >= 3 && args[0] == "--user" && (args[1] == "is-enabled" || args[1] == "is-active") {
			return fmt.Errorf("failed to connect to bus")
		}
		return nil
	})
	defer restore()

	unitPath, err := manager.systemdPath()
	if err != nil {
		t.Fatalf("systemdPath: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(unitPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(unitPath, []byte("[Unit]\nDescription=test\n"), 0o644); err != nil {
		t.Fatalf("write unit: %v", err)
	}

	status, err := manager.Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if status.Installed {
		t.Fatal("expected status to report installed=false when systemd cannot confirm enablement")
	}
}

func TestStatusLinuxReportsInstalledWhenSystemdConfirmsUnit(t *testing.T) {
	// Note: This test modifies global state, cannot run in parallel
	tempDir := t.TempDir()
	manager := &UserServiceManager{}

	callCount := make(map[string]int)
	restore := stubPlatformTestEnv(t, tempDir, func(name string, args ...string) error {
		cmdKey := name + " " + strings.Join(args, " ")
		callCount[cmdKey]++

		// is-enabled should succeed - this confirms the unit is installed
		if name == "systemctl" && len(args) >= 3 && args[0] == "--user" && args[1] == "is-enabled" && args[2] == systemdUnit {
			return nil
		}
		// is-active will fail (unit not running) - that's ok
		if name == "systemctl" && len(args) >= 3 && args[0] == "--user" && args[1] == "is-active" {
			return fmt.Errorf("inactive")
		}
		return nil
	})
	defer restore()

	unitPath, err := manager.systemdPath()
	if err != nil {
		t.Fatalf("systemdPath: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(unitPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(unitPath, []byte("[Unit]\nDescription=test\n"), 0o644); err != nil {
		t.Fatalf("write unit: %v", err)
	}

	status, err := manager.Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if !status.Installed {
		t.Logf("is-enabled call count: %d", callCount["systemctl --user is-enabled ttime.service"])
		t.Fatal("expected status to report installed=true when systemd confirms the unit")
	}
}

func stubPlatformTestEnv(t *testing.T, tempDir string, runner func(name string, args ...string) error) func() {
	t.Helper()

	previousGOOS := currentGOOS
	previousUserConfigDir := userConfigDir
	previousUserHomeDir := userHomeDir
	previousCommandRunner := commandRunner

	currentGOOS = "linux"
	userConfigDir = func() (string, error) {
		return filepath.Join(tempDir, "config"), nil
	}
	userHomeDir = func() (string, error) {
		return filepath.Join(tempDir, "home"), nil
	}
	commandRunner = func(name string, args ...string) error {
		if strings.TrimSpace(name) == "" {
			t.Fatal("expected command name")
		}
		return runner(name, args...)
	}

	return func() {
		currentGOOS = previousGOOS
		userConfigDir = previousUserConfigDir
		userHomeDir = previousUserHomeDir
		commandRunner = previousCommandRunner
	}
}
