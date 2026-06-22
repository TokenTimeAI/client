package updater

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestIsArchive(t *testing.T) {
	t.Parallel()

	tests := map[string]bool{
		"ttime_Darwin_arm64.tar.gz": true,
		"ttime_Linux_x86_64.tgz":    true,
		"ttime_Windows_x86_64.zip":  true,
		"ttime":                     false,
		"checksums.txt":             false,
	}

	for name, want := range tests {
		if got := isArchive(name); got != want {
			t.Fatalf("isArchive(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestPrepareInstallBinaryFromTarGz(t *testing.T) {
	t.Parallel()

	archivePath := writeTarGz(t, map[string][]byte{
		"README.md": []byte("docs"),
		"ttime":     []byte("#!/bin/sh\necho updated\n"),
	})

	binaryPath, cleanup, err := prepareInstallBinary(archivePath, "ttime_Darwin_arm64.tar.gz")
	if err != nil {
		t.Fatalf("prepareInstallBinary: %v", err)
	}
	defer cleanup()

	data, err := os.ReadFile(binaryPath)
	if err != nil {
		t.Fatalf("read extracted binary: %v", err)
	}
	if string(data) != "#!/bin/sh\necho updated\n" {
		t.Fatalf("unexpected binary contents: %q", string(data))
	}

	info, err := os.Stat(binaryPath)
	if err != nil {
		t.Fatalf("stat extracted binary: %v", err)
	}
	if info.Mode().Perm()&0755 == 0 {
		t.Fatalf("expected executable permissions, got %v", info.Mode())
	}
}

func TestPrepareInstallBinaryFromZip(t *testing.T) {
	t.Parallel()

	binaryName := expectedBinaryName()
	archivePath := writeZip(t, map[string][]byte{
		"README.md":        []byte("docs"),
		binaryName:         []byte("updated-binary"),
	})

	binaryPath, cleanup, err := prepareInstallBinary(archivePath, "ttime_Windows_x86_64.zip")
	if err != nil {
		t.Fatalf("prepareInstallBinary: %v", err)
	}
	defer cleanup()

	data, err := os.ReadFile(binaryPath)
	if err != nil {
		t.Fatalf("read extracted binary: %v", err)
	}
	if string(data) != "updated-binary" {
		t.Fatalf("unexpected binary contents: %q", string(data))
	}
}

func TestPrepareInstallBinaryPassesThroughRawBinary(t *testing.T) {
	t.Parallel()

	rawPath := filepath.Join(t.TempDir(), "ttime")
	if err := os.WriteFile(rawPath, []byte("raw-binary"), 0755); err != nil {
		t.Fatalf("write raw binary: %v", err)
	}

	binaryPath, cleanup, err := prepareInstallBinary(rawPath, "ttime")
	if err != nil {
		t.Fatalf("prepareInstallBinary: %v", err)
	}
	defer cleanup()

	if binaryPath != rawPath {
		t.Fatalf("expected passthrough path %q, got %q", rawPath, binaryPath)
	}
}

func TestReplaceBinaryAfterArchiveExtract(t *testing.T) {
	archivePath := writeTarGz(t, map[string][]byte{
		"README.md": []byte("docs"),
		"ttime":     []byte("updated-ttime-binary"),
	})

	targetDir := t.TempDir()
	targetPath := filepath.Join(targetDir, "ttime")
	if err := os.WriteFile(targetPath, []byte("old-binary"), 0755); err != nil {
		t.Fatalf("write target binary: %v", err)
	}

	installBinary, cleanup, err := prepareInstallBinary(archivePath, "ttime_Darwin_arm64.tar.gz")
	if err != nil {
		t.Fatalf("prepareInstallBinary: %v", err)
	}
	defer cleanup()

	if err := (&Updater{}).replaceBinary(targetPath, installBinary); err != nil {
		t.Fatalf("replaceBinary: %v", err)
	}

	data, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("read installed binary: %v", err)
	}
	if string(data) != "updated-ttime-binary" {
		t.Fatalf("unexpected installed binary: %q", string(data))
	}
}

func writeTarGzFile(t *testing.T, contents []byte) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "release.tar.gz")
	if err := os.WriteFile(path, contents, 0644); err != nil {
		t.Fatalf("write archive file: %v", err)
	}
	return path
}

func writeTarGz(t *testing.T, files map[string][]byte) string {
	t.Helper()
	return writeTarGzFile(t, buildTarGz(files))
}

func buildTarGz(files map[string][]byte) []byte {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)

	for name, content := range files {
		header := &tar.Header{
			Name: name,
			Mode: 0755,
			Size: int64(len(content)),
		}
		_ = tw.WriteHeader(header)
		_, _ = tw.Write(content)
	}

	_ = tw.Close()
	_ = gz.Close()
	return buf.Bytes()
}

func writeZip(t *testing.T, files map[string][]byte) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "release.zip")
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create zip: %v", err)
	}
	defer file.Close()

	zipWriter := zip.NewWriter(file)
	for name, content := range files {
		writer, err := zipWriter.Create(name)
		if err != nil {
			t.Fatalf("create zip entry: %v", err)
		}
		if _, err := io.Copy(writer, bytes.NewReader(content)); err != nil {
			t.Fatalf("write zip entry: %v", err)
		}
	}
	if err := zipWriter.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}

	return path
}
