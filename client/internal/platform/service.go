package platform

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

const (
	launchdLabel = "ai.ttime.daemon"
	systemdUnit  = "ttime.service"
)

var (
	currentGOOS   = runtime.GOOS
	userHomeDir   = os.UserHomeDir
	userConfigDir = os.UserConfigDir
	commandRunner = runCommand
)

type ServiceStatus struct {
	Installed bool
	Manager   string
	UnitPath  string
}

type UserServiceManager struct{}

func NewUserServiceManager() *UserServiceManager {
	return &UserServiceManager{}
}

func (m *UserServiceManager) Status() (ServiceStatus, error) {
	switch currentGOOS {
	case "darwin":
		unitPath, err := m.launchdPath()
		if err != nil {
			return ServiceStatus{}, err
		}
		_, err = os.Stat(unitPath)
		return ServiceStatus{
			Installed: err == nil,
			Manager:   "launchd",
			UnitPath:  unitPath,
		}, nil
	case "linux":
		unitPath, err := m.systemdPath()
		if err != nil {
			return ServiceStatus{}, err
		}
		_, err = os.Stat(unitPath)
		installed := err == nil
		if installed {
			installed = m.systemdUnitEnabled()
		}
		return ServiceStatus{
			Installed: installed,
			Manager:   "systemd",
			UnitPath:  unitPath,
		}, nil
	default:
		return ServiceStatus{
			Installed: false,
			Manager:   "unsupported",
		}, nil
	}
}

func (m *UserServiceManager) Install(binaryPath string) error {
	switch currentGOOS {
	case "darwin":
		return m.installLaunchd(binaryPath)
	case "linux":
		return m.installSystemd(binaryPath)
	default:
		return fmt.Errorf("unsupported platform: %s", currentGOOS)
	}
}

func (m *UserServiceManager) Uninstall() error {
	switch currentGOOS {
	case "darwin":
		return m.uninstallLaunchd()
	case "linux":
		return m.uninstallSystemd()
	default:
		return fmt.Errorf("unsupported platform: %s", currentGOOS)
	}
}

func (m *UserServiceManager) installLaunchd(binaryPath string) error {
	unitPath, err := m.launchdPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(unitPath), 0o755); err != nil {
		return err
	}

	logDir, err := userHomeDir()
	if err != nil {
		return err
	}
	stdoutPath := filepath.Join(logDir, "Library", "Logs", "ttime.stdout.log")
	stderrPath := filepath.Join(logDir, "Library", "Logs", "ttime.stderr.log")

	content := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
  <dict>
    <key>Label</key>
    <string>%s</string>
    <key>ProgramArguments</key>
    <array>
      <string>%s</string>
      <string>daemon</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>%s</string>
    <key>StandardErrorPath</key>
    <string>%s</string>
  </dict>
</plist>
`, launchdLabel, binaryPath, stdoutPath, stderrPath)

	if err := os.WriteFile(unitPath, []byte(content), 0o644); err != nil {
		return err
	}

	uid := strconv.Itoa(os.Getuid())
	_ = commandRunner("launchctl", "bootout", "gui/"+uid, unitPath)
	if err := commandRunner("launchctl", "bootstrap", "gui/"+uid, unitPath); err != nil {
		return err
	}
	return commandRunner("launchctl", "enable", "gui/"+uid+"/"+launchdLabel)
}

func (m *UserServiceManager) uninstallLaunchd() error {
	unitPath, err := m.launchdPath()
	if err != nil {
		return err
	}

	uid := strconv.Itoa(os.Getuid())
	_ = commandRunner("launchctl", "disable", "gui/"+uid+"/"+launchdLabel)
	_ = commandRunner("launchctl", "bootout", "gui/"+uid, unitPath)

	if err := os.Remove(unitPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (m *UserServiceManager) installSystemd(binaryPath string) error {
	unitPath, err := m.systemdPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(unitPath), 0o755); err != nil {
		return err
	}

	content := fmt.Sprintf(`[Unit]
Description=ttime local heartbeat daemon
After=network-online.target

[Service]
ExecStart=%s daemon
Restart=always
RestartSec=10

[Install]
WantedBy=default.target
`, binaryPath)

	if err := os.WriteFile(unitPath, []byte(content), 0o644); err != nil {
		return err
	}

	if err := commandRunner("systemctl", "--user", "daemon-reload"); err != nil {
		_ = os.Remove(unitPath)
		return err
	}
	if err := commandRunner("systemctl", "--user", "enable", "--now", systemdUnit); err != nil {
		_ = commandRunner("systemctl", "--user", "disable", "--now", systemdUnit)
		_ = os.Remove(unitPath)
		_ = commandRunner("systemctl", "--user", "daemon-reload")
		return err
	}
	return nil
}

func (m *UserServiceManager) uninstallSystemd() error {
	unitPath, err := m.systemdPath()
	if err != nil {
		return err
	}

	_ = commandRunner("systemctl", "--user", "disable", "--now", systemdUnit)
	if err := os.Remove(unitPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return commandRunner("systemctl", "--user", "daemon-reload")
}

func (m *UserServiceManager) launchdPath() (string, error) {
	homeDir, err := userHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, "Library", "LaunchAgents", launchdLabel+".plist"), nil
}

func (m *UserServiceManager) systemdPath() (string, error) {
	userConfigDir, err := userConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(userConfigDir, "systemd", "user", systemdUnit), nil
}

func (m *UserServiceManager) systemdUnitEnabled() bool {
	if err := commandRunner("systemctl", "--user", "is-enabled", systemdUnit); err == nil {
		return true
	}
	if err := commandRunner("systemctl", "--user", "is-active", systemdUnit); err == nil {
		return true
	}
	return false
}

func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if stderr.Len() == 0 {
			return err
		}
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}
