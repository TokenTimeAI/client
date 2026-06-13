package detectors

import (
	"encoding/json"
	"strings"

	"github.com/ttime-ai/ttime/client/internal/scanner"
)

var mutatingToolNames = map[string]bool{
	"edit":                       true,
	"edit_file":                  true,
	"editfile":                   true,
	"multiedit":                  true,
	"notebookedit":               true,
	"write":                      true,
	"write_file":                 true,
	"writefile":                  true,
	"create_file":                true,
	"str_replace":                true,
	"replace":                    true,
	"apply_patch":                true,
	"apply_file_diff":            true,
	"patch":                      true,
	"multi_patch":                true,
	"copilot_replacestring":      true,
	"copilot_multireplacestring": true,
	"copilot_applypatch":         true,
	"vscode_editfile_internal":   true,
	"replace_file_content":       true,
	"multi_replace_file_content": true,
	"write_to_file":              true,
	"antigravity_write_to_file":  true,
	"antigravity_replace_file":   true,
	"antigravity_multi_replace":  true,
	"piebald_writefile":          true,
	"piebald_editfile":           true,
}

var toolPathKeys = []string{
	"path",
	"file",
	"file_path",
	"filepath",
	"filename",
	"notebook_path",
	"target_file",
	"target_path",
	"relative_path",
	"absolute_path",
}

var toolPathListKeys = []string{
	"paths",
	"files",
	"file_paths",
	"target_files",
}

var patchTextKeys = []string{
	"patch",
	"diff",
	"input",
}

func fileEditsFromToolCallJSON(toolName, raw string) map[string]scanner.FileEdit {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return map[string]scanner.FileEdit{}
	}

	var input map[string]any
	if err := json.Unmarshal([]byte(raw), &input); err == nil {
		return fileEditsFromToolCall(toolName, input)
	}

	if isPatchTool(toolName) {
		return parseApplyPatch(raw)
	}

	return map[string]scanner.FileEdit{}
}

func fileEditsFromToolCall(toolName string, input map[string]any) map[string]scanner.FileEdit {
	edits := make(map[string]scanner.FileEdit)
	if !isMutatingTool(toolName) {
		return edits
	}

	if isPatchTool(toolName) {
		for _, key := range patchTextKeys {
			mergeFileEdits(edits, parseApplyPatch(strings.TrimSpace(stringValue(input[key]))))
		}
	}

	for _, path := range toolInputPaths(input) {
		upsertFileEdit(edits, path, 1, 0, 0)
	}

	return edits
}

func isMutatingTool(toolName string) bool {
	normalized := normalizeToolName(toolName)
	if mutatingToolNames[normalized] {
		return true
	}
	if strings.Contains(normalized, "applypatch") ||
		strings.Contains(normalized, "apply_patch") ||
		strings.Contains(normalized, "editfile") ||
		strings.Contains(normalized, "edit_file") ||
		strings.Contains(normalized, "writefile") ||
		strings.Contains(normalized, "write_file") {
		return true
	}
	return false
}

func isPatchTool(toolName string) bool {
	normalized := normalizeToolName(toolName)
	return normalized == "apply_patch" ||
		normalized == "patch" ||
		normalized == "multi_patch" ||
		normalized == "apply_file_diff" ||
		strings.Contains(normalized, "applypatch")
}

func normalizeToolName(toolName string) string {
	normalized := strings.ToLower(strings.TrimSpace(toolName))
	normalized = strings.TrimPrefix(normalized, "mcp__")
	if strings.Contains(normalized, "__") {
		parts := strings.Split(normalized, "__")
		normalized = parts[len(parts)-1]
	}
	return normalized
}

func toolInputPaths(input map[string]any) []string {
	seen := make(map[string]bool)
	paths := make([]string, 0, 2)

	add := func(path string) {
		path = cleanToolPath(path)
		if path == "" || seen[path] {
			return
		}
		seen[path] = true
		paths = append(paths, path)
	}

	for _, key := range toolPathKeys {
		add(stringValue(input[key]))
	}

	for _, key := range toolPathListKeys {
		switch value := input[key].(type) {
		case []any:
			for _, item := range value {
				add(stringValue(item))
			}
		case []string:
			for _, item := range value {
				add(item)
			}
		case string:
			for _, item := range strings.Split(value, "\n") {
				add(item)
			}
		}
	}

	return paths
}

func cleanToolPath(path string) string {
	path = strings.TrimSpace(path)
	path = strings.Trim(path, "\"'")
	if path == "" || strings.Contains(path, "\n") {
		return ""
	}
	return path
}
