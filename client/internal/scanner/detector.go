package scanner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// Detector is implemented by each agent scanner
type Detector interface {
	// Name returns the agent identifier (e.g., "cosine", "cline")
	Name() string

	// Detect returns true if this agent's data exists on the system
	Detect(ctx context.Context) (bool, error)

	// Scan retrieves new results since the provided state
	Scan(ctx context.Context, state SourceState) ([]ScanResult, SourceState, error)

	// DefaultPaths returns the expected paths for this agent's data
	DefaultPaths() []string
}

// Registry holds all registered detectors
var Registry []Detector

// Register adds a detector to the registry
func Register(d Detector) {
	Registry = append(Registry, d)
}

// FindDetectors returns all detectors that detected their agent
func FindDetectors(ctx context.Context) ([]Detector, error) {
	var detected []Detector
	for _, d := range Registry {
		ok, err := d.Detect(ctx)
		if err != nil {
			// Log but continue - don't fail entire scan for one agent
			continue
		}
		if ok {
			detected = append(detected, d)
		}
	}
	return detected, nil
}

// ExpandHome replaces ~ with home directory
func ExpandHome(path string) (string, error) {
	if path == "" || path[0] != '~' {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}
	return filepath.Join(home, path[1:]), nil
}

// FileExists checks if a file exists and is readable
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// DirExists checks if a directory exists
func DirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}