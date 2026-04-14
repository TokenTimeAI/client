// Package updater handles checking for and performing self-updates
package updater

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	updateCheckURL = "https://ttime.ai/api/v1/releases/latest"
	defaultTimeout = 10 * time.Second
)

// ReleaseInfo holds information about the latest release
type ReleaseInfo struct {
	Version     string    `json:"version"`
	PublishedAt time.Time `json:"published_at"`
	Assets      []Asset   `json:"assets"`
}

// Asset represents a downloadable binary asset
type Asset struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	Size     int    `json:"size"`
	Platform string `json:"platform"`
	Arch     string `json:"arch"`
}

// Updater handles version checking and binary updates
type Updater struct {
	currentVersion string
	serverURL      string
	httpClient     *http.Client
}

// New creates a new Updater instance
func New(currentVersion, serverURL string) *Updater {
	return &Updater{
		currentVersion: normalizeVersion(currentVersion),
		serverURL:      serverURL,
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
	}
}

// CheckForUpdate queries the API for the latest release and returns update info if available
func (u *Updater) CheckForUpdate() (*UpdateResult, error) {
	release, err := u.fetchLatestRelease()
	if err != nil {
		return nil, fmt.Errorf("fetch release: %w", err)
	}

	latestVersion := normalizeVersion(release.Version)

	if !isNewer(latestVersion, u.currentVersion) {
		return &UpdateResult{
			CurrentVersion: u.currentVersion,
			LatestVersion:  latestVersion,
			UpdateAvailable: false,
		}, nil
	}

	asset := u.findMatchingAsset(release.Assets)
	if asset == nil {
		return nil, fmt.Errorf("no matching binary found for %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	return &UpdateResult{
		CurrentVersion:  u.currentVersion,
		LatestVersion:   latestVersion,
		UpdateAvailable: true,
		Asset:           asset,
		ReleaseURL:      fmt.Sprintf("https://github.com/tokentimeai/client/releases/tag/%s", release.Version),
	}, nil
}

// PerformUpdate downloads and installs the latest version
func (u *Updater) PerformUpdate(asset *Asset) error {
	binaryPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable path: %w", err)
	}
	binaryPath, err = filepath.EvalSymlinks(binaryPath)
	if err != nil {
		binaryPath = filepath.Clean(binaryPath)
	}

	// Download new binary
	tempFile, err := u.downloadBinary(asset.URL)
	if err != nil {
		return fmt.Errorf("download binary: %w", err)
	}
	defer os.Remove(tempFile)

	// On Windows, we can't replace a running binary directly
	if runtime.GOOS == "windows" {
		return u.updateWindows(binaryPath, tempFile)
	}

	// Replace binary atomically
	if err := u.replaceBinary(binaryPath, tempFile); err != nil {
		return fmt.Errorf("replace binary: %w", err)
	}

	return nil
}

// UpdateResult contains the result of a version check
type UpdateResult struct {
	CurrentVersion  string
	LatestVersion   string
	UpdateAvailable bool
	Asset           *Asset
	ReleaseURL      string
}

func (u *Updater) fetchLatestRelease() (*ReleaseInfo, error) {
	url := updateCheckURL
	if u.serverURL != "" {
		url = strings.TrimSuffix(u.serverURL, "/") + "/api/v1/releases/latest"
	}

	resp, err := u.httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var release ReleaseInfo
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &release, nil
}

func (u *Updater) findMatchingAsset(assets []Asset) *Asset {
	platform := normalizePlatform(runtime.GOOS)
	arch := normalizeArch(runtime.GOARCH)

	for _, asset := range assets {
		if asset.Platform == platform && asset.Arch == arch {
			return &asset
		}
	}
	return nil
}

func (u *Updater) downloadBinary(url string) (string, error) {
	resp, err := u.httpClient.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed: %d", resp.StatusCode)
	}

	tempFile, err := os.CreateTemp("", "ttime-update-*")
	if err != nil {
		return "", err
	}
	defer tempFile.Close()

	if _, err := io.Copy(tempFile, resp.Body); err != nil {
		os.Remove(tempFile.Name())
		return "", err
	}

	if err := os.Chmod(tempFile.Name(), 0755); err != nil {
		os.Remove(tempFile.Name())
		return "", err
	}

	return tempFile.Name(), nil
}

func (u *Updater) replaceBinary(targetPath, tempPath string) error {
	backupPath := targetPath + ".backup"

	// Create backup
	if err := os.Rename(targetPath, backupPath); err != nil {
		return fmt.Errorf("backup existing: %w", err)
	}

	// Move new binary into place
	if err := os.Rename(tempPath, targetPath); err != nil {
		// Restore backup on failure
		os.Rename(backupPath, targetPath)
		return fmt.Errorf("install new binary: %w", err)
	}

	// Remove backup on success
	os.Remove(backupPath)
	return nil
}

func (u *Updater) updateWindows(targetPath, tempPath string) error {
	// On Windows, we need to use a batch script to replace the running binary
	batchScript := fmt.Sprintf(`
@echo off
timeout /t 2 /nobreak >nul
move /Y "%s" "%s"
del "%%~f0"
`, tempPath, targetPath)

	batchPath := targetPath + ".update.bat"
	if err := os.WriteFile(batchPath, []byte(batchScript), 0755); err != nil {
		return fmt.Errorf("create update script: %w", err)
	}

	// Execute the batch script detached
	cmd := exec.Command("cmd", "/C", "start", "/b", batchPath)
	return cmd.Start()
}

func normalizeVersion(v string) string {
	return strings.TrimPrefix(v, "v")
}

func normalizePlatform(p string) string {
	switch p {
	case "darwin":
		return "darwin"
	case "linux":
		return "linux"
	case "windows":
		return "windows"
	default:
		return p
	}
}

func normalizeArch(a string) string {
	switch a {
	case "amd64":
		return "amd64"
	case "arm64":
		return "arm64"
	default:
		return a
	}
}

func isNewer(latest, current string) bool {
	// Simple semver comparison - split by dots and compare
	latestParts := strings.Split(latest, ".")
	currentParts := strings.Split(current, ".")

	for i := 0; i < len(latestParts) && i < len(currentParts); i++ {
		var l, c int
		fmt.Sscanf(latestParts[i], "%d", &l)
		fmt.Sscanf(currentParts[i], "%d", &c)

		if l > c {
			return true
		}
		if l < c {
			return false
		}
	}

	return len(latestParts) > len(currentParts)
}