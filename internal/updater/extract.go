package updater

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func expectedBinaryName() string {
	if runtime.GOOS == "windows" {
		return "ttime.exe"
	}
	return "ttime"
}

func isArchive(name string) bool {
	lower := strings.ToLower(name)
	return strings.HasSuffix(lower, ".tar.gz") ||
		strings.HasSuffix(lower, ".tgz") ||
		strings.HasSuffix(lower, ".zip")
}

func prepareInstallBinary(sourcePath, assetName string) (string, func(), error) {
	if !isArchive(assetName) {
		return sourcePath, func() {}, nil
	}

	extractDir, err := os.MkdirTemp("", "ttime-update-extract-*")
	if err != nil {
		return "", nil, fmt.Errorf("create extract dir: %w", err)
	}

	removeExtractDir := func() {
		os.RemoveAll(extractDir)
	}

	switch {
	case strings.HasSuffix(strings.ToLower(assetName), ".zip"):
		err = extractZip(sourcePath, extractDir)
	default:
		err = extractTarGz(sourcePath, extractDir)
	}
	if err != nil {
		removeExtractDir()
		return "", nil, err
	}

	extracted, err := findBinary(extractDir, expectedBinaryName())
	if err != nil {
		removeExtractDir()
		return "", nil, err
	}

	installedBinary, err := copyToTempBinary(extracted)
	if err != nil {
		removeExtractDir()
		return "", nil, err
	}

	return installedBinary, func() {
		os.Remove(installedBinary)
		removeExtractDir()
	}, nil
}

func copyToTempBinary(sourcePath string) (string, error) {
	source, err := os.Open(sourcePath)
	if err != nil {
		return "", fmt.Errorf("open extracted binary: %w", err)
	}
	defer source.Close()

	dest, err := os.CreateTemp("", "ttime-update-bin-*")
	if err != nil {
		return "", fmt.Errorf("create install binary: %w", err)
	}
	destPath := dest.Name()

	if _, err := io.Copy(dest, source); err != nil {
		dest.Close()
		os.Remove(destPath)
		return "", fmt.Errorf("copy extracted binary: %w", err)
	}

	if err := dest.Close(); err != nil {
		os.Remove(destPath)
		return "", fmt.Errorf("close install binary: %w", err)
	}

	if err := os.Chmod(destPath, 0755); err != nil {
		os.Remove(destPath)
		return "", fmt.Errorf("chmod install binary: %w", err)
	}

	return destPath, nil
}

func findBinary(root, name string) (string, error) {
	direct := filepath.Join(root, name)
	if info, err := os.Stat(direct); err == nil && !info.IsDir() {
		return direct, nil
	}

	var matches []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if entry.Name() == name {
			matches = append(matches, path)
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("search extracted archive: %w", err)
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("binary %q not found in release archive", name)
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("multiple %q binaries found in release archive", name)
	}

	return matches[0], nil
}

func extractTarGz(archivePath, destDir string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open archive: %w", err)
	}
	defer file.Close()

	gz, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("read gzip archive: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read tar archive: %w", err)
		}

		if header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeRegA {
			continue
		}

		name := filepath.Base(header.Name)
		if name != "ttime" && name != "ttime.exe" {
			if _, err := io.Copy(io.Discard, tr); err != nil {
				return fmt.Errorf("skip tar entry %q: %w", header.Name, err)
			}
			continue
		}

		targetPath := filepath.Join(destDir, name)
		if err := writeExtractedFile(targetPath, tr, os.FileMode(header.Mode)); err != nil {
			return err
		}
	}
}

func extractZip(archivePath, destDir string) error {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("open zip archive: %w", err)
	}
	defer reader.Close()

	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			continue
		}

		name := filepath.Base(file.Name)
		if name != "ttime" && name != "ttime.exe" {
			continue
		}

		source, err := file.Open()
		if err != nil {
			return fmt.Errorf("open zip entry: %w", err)
		}

		targetPath := filepath.Join(destDir, name)
		err = writeExtractedFile(targetPath, source, file.Mode())
		source.Close()
		if err != nil {
			return err
		}
	}

	return nil
}

func writeExtractedFile(targetPath string, source io.Reader, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return fmt.Errorf("create extract dir: %w", err)
	}

	dest, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("create extracted file: %w", err)
	}
	defer dest.Close()

	if _, err := io.Copy(dest, source); err != nil {
		return fmt.Errorf("extract file: %w", err)
	}

	if err := os.Chmod(targetPath, 0755); err != nil {
		return fmt.Errorf("chmod extracted file: %w", err)
	}

	return nil
}
