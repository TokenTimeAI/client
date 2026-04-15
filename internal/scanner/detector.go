package scanner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// FindDetectors returns all detectors that detected their agent
// Uses the global registry
func FindDetectors(ctx context.Context) ([]Detector, error) {
	var detected []Detector
	for _, name := range globalRegistry.List() {
		d, ok := globalRegistry.Get(name)
		if !ok {
			continue
		}
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

// GetDetectedInfos returns info about all detectors and their detection status
func GetDetectedInfos(ctx context.Context) []DetectorInfo {
	var infos []DetectorInfo
	for _, name := range globalRegistry.List() {
		d, ok := globalRegistry.Get(name)
		if !ok {
			continue
		}
		
		detected, _ := d.Detect(ctx)
		info := DetectorInfo{
			Name:        d.Name(),
			Description: d.Description(),
			Detected:    detected,
			Paths:       d.DefaultPaths(),
		}
		
		// If detected and has a base detector with found path
		if bd, ok := d.(interface{ FoundPath() string }); ok && detected {
			info.FoundPath = bd.FoundPath()
		}
		
		infos = append(infos, info)
	}
	return infos
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

// FindFirstPath returns the first path that exists from a list
func FindFirstPath(paths []string) string {
	for _, p := range paths {
		expanded, err := ExpandHome(p)
		if err != nil {
			continue
		}
		if DirExists(expanded) || FileExists(expanded) {
			return expanded
		}
	}
	return ""
}