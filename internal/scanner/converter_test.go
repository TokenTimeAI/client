package scanner

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestToEventAddsGitUpstreamMetadata(t *testing.T) {
	root := canonicalTempDir(t)
	runGit(t, root, "init")
	runGit(t, root, "remote", "add", "origin", "git@github.com:TokenTimeAI/ttime.git")

	result := ScanResult{
		AgentType: "codex",
		Entity:    root,
		Type:      "conversation",
		Time:      float64(time.Now().Unix()),
		Project:   "ttime-main",
		Metadata:  map[string]any{"parser": "test"},
	}

	event := result.ToEvent()

	if event.Metadata["parser"] != "test" {
		t.Fatalf("metadata parser = %#v, want test", event.Metadata["parser"])
	}
	if event.Metadata["git_root"] != root {
		t.Fatalf("git_root = %#v, want %q", event.Metadata["git_root"], root)
	}
	if event.Metadata["git_upstream_url"] != "git@github.com:TokenTimeAI/ttime.git" {
		t.Fatalf("git_upstream_url = %#v", event.Metadata["git_upstream_url"])
	}
}

func TestToEventUsesParentDirectoryForFileEntities(t *testing.T) {
	root := canonicalTempDir(t)
	runGit(t, root, "init")
	runGit(t, root, "remote", "add", "origin", "https://github.com/tokentimeai/client.git")

	filePath := filepath.Join(root, "app", "models", "user.rb")
	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filePath, []byte("class User; end\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	event := ScanResult{
		AgentType: "codex",
		Entity:    filePath,
		Type:      "file",
		Time:      float64(time.Now().Unix()),
	}.ToEvent()

	if event.Metadata["git_upstream_url"] != "https://github.com/tokentimeai/client.git" {
		t.Fatalf("git_upstream_url = %#v", event.Metadata["git_upstream_url"])
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
}

func canonicalTempDir(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	canonical, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatalf("eval symlinks: %v", err)
	}
	return canonical
}
