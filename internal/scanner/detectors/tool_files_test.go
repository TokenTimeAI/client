package detectors

import "testing"

func TestFileEditsFromToolCallExtractsCommonWriteInputs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		toolName string
		input    map[string]any
		wantPath string
	}{
		{
			name:     "claude edit file path",
			toolName: "Edit",
			input:    map[string]any{"file_path": "app/models/user.rb"},
			wantPath: "app/models/user.rb",
		},
		{
			name:     "gemini write path",
			toolName: "write_file",
			input:    map[string]any{"path": "src/main.ts"},
			wantPath: "src/main.ts",
		},
		{
			name:     "notebook path",
			toolName: "NotebookEdit",
			input:    map[string]any{"notebook_path": "analysis/report.ipynb"},
			wantPath: "analysis/report.ipynb",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			edits := fileEditsFromToolCall(tt.toolName, tt.input)
			edit, ok := edits[tt.wantPath]
			if !ok {
				t.Fatalf("expected path %q in edits %#v", tt.wantPath, edits)
			}
			if edit.EditCount != 1 {
				t.Fatalf("edit count = %d, want 1", edit.EditCount)
			}
		})
	}
}

func TestFileEditsFromToolCallIgnoresReadOnlyTools(t *testing.T) {
	t.Parallel()

	edits := fileEditsFromToolCall("read_file", map[string]any{"file_path": "src/main.ts"})
	if len(edits) != 0 {
		t.Fatalf("expected no edits for read-only tool, got %#v", edits)
	}
}

func TestFileEditsFromToolCallParsesPatchArgument(t *testing.T) {
	t.Parallel()

	edits := fileEditsFromToolCall("apply_patch", map[string]any{
		"patch": "*** Begin Patch\n*** Update File: src/app.tsx\n@@\n-old\n+new\n*** End Patch\n",
	})

	edit, ok := edits["src/app.tsx"]
	if !ok {
		t.Fatalf("expected src/app.tsx in edits %#v", edits)
	}
	if edit.EditCount != 1 || edit.LinesAdded != 1 || edit.LinesDeleted != 1 {
		t.Fatalf("edit = %#v, want one edit with one added and one deleted line", edit)
	}
}
