package scanner

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var gitContextCache sync.Map

type gitContext struct {
	root      string
	upstream  string
	available bool
}

func metadataWithGitContext(r ScanResult) map[string]any {
	metadata := cloneMetadata(r.Metadata)
	cwd := gitCandidatePath(r)
	if cwd == "" {
		return metadata
	}

	git := lookupGitContext(cwd)
	if !git.available {
		return metadata
	}

	if git.root != "" {
		metadata["git_root"] = git.root
	}
	if git.upstream != "" {
		metadata["git_upstream_url"] = git.upstream
	}
	return metadata
}

func cloneMetadata(metadata map[string]any) map[string]any {
	if len(metadata) == 0 {
		return map[string]any{}
	}
	cloned := make(map[string]any, len(metadata)+2)
	for key, value := range metadata {
		cloned[key] = value
	}
	return cloned
}

func gitCandidatePath(r ScanResult) string {
	for _, value := range []string{
		metadataString(r.Metadata, "cwd"),
		metadataString(r.Metadata, "git_root"),
		r.Entity,
	} {
		path := strings.TrimSpace(value)
		if path == "" || !filepath.IsAbs(path) {
			continue
		}
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			path = filepath.Dir(path)
		}
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			return path
		}
	}
	return ""
}

func metadataString(metadata map[string]any, key string) string {
	value, ok := metadata[key]
	if !ok {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	case interface{ String() string }:
		return typed.String()
	default:
		return ""
	}
}

func lookupGitContext(cwd string) gitContext {
	cleaned := filepath.Clean(cwd)
	if cached, ok := gitContextCache.Load(cleaned); ok {
		return cached.(gitContext)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancel()

	rootBytes, err := exec.CommandContext(ctx, "git", "-C", cleaned, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		result := gitContext{}
		gitContextCache.Store(cleaned, result)
		return result
	}

	root := strings.TrimSpace(string(rootBytes))
	upstream := firstGitConfigValue(ctx, root,
		"remote.origin.url",
		"branch."+currentGitBranch(ctx, root)+".remote",
	)
	if !looksLikeGitURL(upstream) {
		upstream = firstGitConfigValue(ctx, root, "remote.upstream.url")
	}

	result := gitContext{root: root, upstream: upstream, available: true}
	gitContextCache.Store(cleaned, result)
	return result
}

func firstGitConfigValue(ctx context.Context, root string, keys ...string) string {
	for _, key := range keys {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		valueBytes, err := exec.CommandContext(ctx, "git", "-C", root, "config", "--get", key).Output()
		if err == nil && strings.TrimSpace(string(valueBytes)) != "" {
			return strings.TrimSpace(string(valueBytes))
		}
	}
	return ""
}

func currentGitBranch(ctx context.Context, root string) string {
	branchBytes, err := exec.CommandContext(ctx, "git", "-C", root, "branch", "--show-current").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(branchBytes))
}

func looksLikeGitURL(value string) bool {
	return strings.Contains(value, "://") || strings.Contains(value, "@") || strings.Contains(value, "/")
}
