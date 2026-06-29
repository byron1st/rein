package tool_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/byron1st/rein/pkg/tool"
	"github.com/stretchr/testify/require"
)

// jsonArgs builds a json.RawMessage from the given value, failing the test on
// marshal error so call sites stay one-liners.
func jsonArgs(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err, "test fixture: marshal args")
	return json.RawMessage(b)
}

func TestReadFile_BasicRead(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.txt")
	require.NoError(t, os.WriteFile(path, []byte("hello world"), 0o644))

	out, err := tool.NewReadFileTool().Execute(context.Background(), jsonArgs(t, map[string]any{"path": path}))
	require.NoError(t, err)
	require.Equal(t, "hello world", out, "read_file must return the full file content when no offset/limit")
}

func TestReadFile_OffsetLimit_LineSlicing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "lines.txt")
	content := "line1\nline2\nline3\nline4\nline5"
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	out, err := tool.NewReadFileTool().Execute(context.Background(), jsonArgs(t, map[string]any{
		"path":   path,
		"offset": 2,
		"limit":  2,
	}))
	require.NoError(t, err)
	require.Equal(t, "line2\nline3", out, "offset=2 limit=2 must return the 2nd and 3rd lines (1-indexed)")
}

func TestReadFile_LineSlicing_EdgeCases(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "lines.txt")
	content := "l1\nl2\nl3\nl4\nl5"
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	cases := []struct {
		name   string
		offset any
		limit  any
		want   string
	}{
		{"offset only returns to end", 3, nil, "l3\nl4\nl5"},
		{"limit only returns first N", nil, 2, "l1\nl2"},
		{"offset past end returns empty", 99, nil, ""},
		{"offset zero defaults to 1", 0, 2, "l1\nl2"},
		{"limit zero defaults to all", 2, 0, "l2\nl3\nl4\nl5"},
		{"offset negative defaults to 1", -1, 1, "l1"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			args := map[string]any{"path": path}
			if c.offset != nil {
				args["offset"] = c.offset
			}
			if c.limit != nil {
				args["limit"] = c.limit
			}
			out, err := tool.NewReadFileTool().Execute(context.Background(), jsonArgs(t, args))
			require.NoError(t, err)
			require.Equal(t, c.want, out, "slicing must match the expected substring")
		})
	}
}

func TestReadFile_MissingFile_ReturnsErrReadFileOpen(t *testing.T) {
	_, err := tool.NewReadFileTool().Execute(context.Background(), jsonArgs(t, map[string]any{
		"path": filepath.Join(t.TempDir(), "nope.txt"),
	}))
	require.ErrorIs(t, err, tool.ErrReadFileOpen)
}

func TestReadFile_LargeFile_TruncatesAndOffloads(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "big.txt")
	// Exceed both bounds comfortably: > 50KB and > 2000 lines.
	lines := make([]string, tool.MaxLines+500)
	for i := range lines {
		lines[i] = strings.Repeat("x", 30)
	}
	content := strings.Join(lines, "\n")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	out, err := tool.NewReadFileTool().Execute(context.Background(), jsonArgs(t, map[string]any{"path": path}))
	require.NoError(t, err)
	require.Contains(t, out, "[output truncated", "large file must trigger truncation marker")

	offloadPath := parseOffloadPath(t, out)
	full, err := os.ReadFile(offloadPath)
	require.NoError(t, err, "offloaded file must be readable")
	require.Equal(t, content, string(full), "offloaded file must contain the full original content")
}

func TestReadFile_MalformedArgs_ReturnsErrReadFileArgsParse(t *testing.T) {
	_, err := tool.NewReadFileTool().Execute(context.Background(), json.RawMessage(`{not json`))
	require.ErrorIs(t, err, tool.ErrReadFileArgsParse, "expected ErrReadFileArgsParse, got %v", err)
}

func TestReadFile_MissingPath_ReturnsErrReadFileMissingPath(t *testing.T) {
	_, err := tool.NewReadFileTool().Execute(context.Background(), jsonArgs(t, map[string]any{"path": ""}))
	require.ErrorIs(t, err, tool.ErrReadFileMissingPath, "expected ErrReadFileMissingPath, got %v", err)
}

func TestReadFile_Schema_ReturnsInnerFunctionObject(t *testing.T) {
	schema := tool.NewReadFileTool().Schema()
	require.JSONEq(t, `{
		"name": "read_file",
		"description": "Read the contents of a file from the local filesystem. Supports optional 1-indexed line offset and line limit.",
		"parameters": {
			"type": "object",
			"properties": {
				"path": {"type": "string", "description": "Absolute or relative path of the file to read."},
				"offset": {"type": "integer", "description": "1-indexed line number to start reading from. Defaults to 1."},
				"limit": {"type": "integer", "description": "Maximum number of lines to return. Defaults to all remaining lines."}
			},
			"required": ["path"]
		}
	}`, string(schema))
}

func TestReadFile_Name(t *testing.T) {
	require.Equal(t, "read_file", tool.NewReadFileTool().Name())
}

func TestWriteFile_CreateNewFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "new.txt")

	out, err := tool.NewWriteFileTool().Execute(context.Background(), jsonArgs(t, map[string]any{
		"path":    path,
		"content": "fresh content",
	}))
	require.NoError(t, err)
	require.Equal(t, "wrote 13 bytes to "+path, out, "summary must report byte count and path")

	got, err := os.ReadFile(path)
	require.NoError(t, err, "file must exist on disk after write_file")
	require.Equal(t, "fresh content", string(got), "on-disk content must match the written content")
}

func TestWriteFile_OverwriteExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "existing.txt")
	require.NoError(t, os.WriteFile(path, []byte("old"), 0o644))

	_, err := tool.NewWriteFileTool().Execute(context.Background(), jsonArgs(t, map[string]any{
		"path":    path,
		"content": "new",
	}))
	require.NoError(t, err)

	got, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, "new", string(got), "write_file must overwrite the existing content")
}

func TestWriteFile_WriteFailure_ReturnsErrWriteFile(t *testing.T) {
	// Writing to a path under a non-existent directory fails os.WriteFile.
	_, err := tool.NewWriteFileTool().Execute(context.Background(), jsonArgs(t, map[string]any{
		"path":    filepath.Join(t.TempDir(), "missing-dir", "f.txt"),
		"content": "x",
	}))
	require.ErrorIs(t, err, tool.ErrWriteFile)
}

func TestWriteFile_MalformedArgs_ReturnsErrWriteFileArgsParse(t *testing.T) {
	_, err := tool.NewWriteFileTool().Execute(context.Background(), json.RawMessage(`{not json`))
	require.ErrorIs(t, err, tool.ErrWriteFileArgsParse, "expected ErrWriteFileArgsParse, got %v", err)
}

func TestWriteFile_MissingRequiredFields_ReturnsErrWriteFileMissingPath(t *testing.T) {
	_, err := tool.NewWriteFileTool().Execute(context.Background(), jsonArgs(t, map[string]any{"path": "", "content": "x"}))
	require.ErrorIs(t, err, tool.ErrWriteFileMissingPath, "expected ErrWriteFileMissingPath, got %v", err)
}

func TestWriteFile_EmptyContent_CreatesEmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.txt")
	out, err := tool.NewWriteFileTool().Execute(context.Background(), jsonArgs(t, map[string]any{"path": path, "content": ""}))
	require.NoError(t, err, "empty content is a valid empty-file write")
	require.Contains(t, out, "wrote 0 bytes", "summary must report zero bytes for empty content")
	got, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Empty(t, string(got), "file must be empty on disk")
}

func TestWriteFile_Schema_ReturnsInnerFunctionObject(t *testing.T) {
	schema := tool.NewWriteFileTool().Schema()
	require.JSONEq(t, `{
		"name": "write_file",
		"description": "Create or overwrite a file with the given content on the local filesystem.",
		"parameters": {
			"type": "object",
			"properties": {
				"path": {"type": "string", "description": "Absolute or relative path of the file to write."},
				"content": {"type": "string", "description": "The full text content to write to the file."}
			},
			"required": ["path", "content"]
		}
	}`, string(schema))
}

func TestWriteFile_Name(t *testing.T) {
	require.Equal(t, "write_file", tool.NewWriteFileTool().Name())
}

func TestEditFile_UniqueMatch_ReplacesContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "e.txt")
	require.NoError(t, os.WriteFile(path, []byte("alpha\nbeta\nalpha"), 0o644))

	out, err := tool.NewEditFileTool().Execute(context.Background(), jsonArgs(t, map[string]any{
		"path":    path,
		"old_str": "beta",
		"new_str": "BETA",
	}))
	require.NoError(t, err)
	require.Equal(t, "edited "+path, out, "summary must report the edited path exactly")

	got, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, "alpha\nBETA\nalpha", string(got), "only the unique old_str occurrence must be replaced")
}

func TestEditFile_NoMatch_ReturnsErrEditFileNoMatch_FileUnchanged(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "e.txt")
	original := "alpha\nbeta"
	require.NoError(t, os.WriteFile(path, []byte(original), 0o644))

	_, err := tool.NewEditFileTool().Execute(context.Background(), jsonArgs(t, map[string]any{
		"path":    path,
		"old_str": "gamma",
		"new_str": "GAMMA",
	}))
	require.ErrorIs(t, err, tool.ErrEditFileNoMatch)

	got, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, original, string(got), "file must be unchanged when old_str is not found")
}

func TestEditFile_MultiMatch_ReturnsErrEditFileMultiMatch_FileUnchanged(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "e.txt")
	original := "dup\ndup\ndup"
	require.NoError(t, os.WriteFile(path, []byte(original), 0o644))

	_, err := tool.NewEditFileTool().Execute(context.Background(), jsonArgs(t, map[string]any{
		"path":    path,
		"old_str": "dup",
		"new_str": "DUP",
	}))
	require.ErrorIs(t, err, tool.ErrEditFileMultiMatch)
	require.Contains(t, err.Error(), "3 matches found", "error must report the match count and ask for more context")

	got, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, original, string(got), "file must be unchanged when old_str matches more than once")
}

func TestEditFile_MissingFile_ReturnsErrEditFileRead(t *testing.T) {
	_, err := tool.NewEditFileTool().Execute(context.Background(), jsonArgs(t, map[string]any{
		"path":    filepath.Join(t.TempDir(), "nope.txt"),
		"old_str": "x",
		"new_str": "y",
	}))
	require.ErrorIs(t, err, tool.ErrEditFileRead)
}

func TestEditFile_MalformedArgs_ReturnsErrEditFileArgsParse(t *testing.T) {
	_, err := tool.NewEditFileTool().Execute(context.Background(), json.RawMessage(`{not json`))
	require.ErrorIs(t, err, tool.ErrEditFileArgsParse, "expected ErrEditFileArgsParse, got %v", err)
}

func TestEditFile_MissingPath_ReturnsErrEditFileMissingPath(t *testing.T) {
	_, err := tool.NewEditFileTool().Execute(context.Background(), jsonArgs(t, map[string]any{
		"path":    "",
		"old_str": "a",
		"new_str": "b",
	}))
	require.ErrorIs(t, err, tool.ErrEditFileMissingPath, "expected ErrEditFileMissingPath, got %v", err)
}

func TestEditFile_Schema_ReturnsInnerFunctionObject(t *testing.T) {
	schema := tool.NewEditFileTool().Schema()
	require.JSONEq(t, `{
		"name": "edit_file",
		"description": "Replace a unique occurrence of old_str with new_str in a file. The match must be unique or the edit is rejected.",
		"parameters": {
			"type": "object",
			"properties": {
				"path": {"type": "string", "description": "Absolute or relative path of the file to edit."},
				"old_str": {"type": "string", "description": "The exact text to find; must appear exactly once in the file."},
				"new_str": {"type": "string", "description": "The replacement text."}
			},
			"required": ["path", "old_str", "new_str"]
		}
	}`, string(schema))
}

func TestEditFile_Name(t *testing.T) {
	require.Equal(t, "edit_file", tool.NewEditFileTool().Name())
}

func TestReadFile_OffsetExactlyLastLine_ReturnsLastLineOnly(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "lines.txt")
	require.NoError(t, os.WriteFile(path, []byte("l1\nl2\nl3"), 0o644))

	out, err := tool.NewReadFileTool().Execute(context.Background(), jsonArgs(t, map[string]any{
		"path":   path,
		"offset": 3,
	}))
	require.NoError(t, err)
	require.Equal(t, "l3", out, "offset equal to the last line number must return just the last line, not empty")
}

func TestReadFile_LimitExceedsRemaining_ReturnsToEnd(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "lines.txt")
	require.NoError(t, os.WriteFile(path, []byte("l1\nl2\nl3\nl4\nl5"), 0o644))

	out, err := tool.NewReadFileTool().Execute(context.Background(), jsonArgs(t, map[string]any{
		"path":   path,
		"offset": 4,
		"limit":  10,
	}))
	require.NoError(t, err)
	require.Equal(t, "l4\nl5", out, "limit greater than remaining lines must clamp to the end, not panic or over-read")
}

func TestEditFile_WriteFailure_ReturnsErrEditFileWrite_FileUnchanged(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "readonly.txt")
	original := "alpha\nbeta\nalpha"
	require.NoError(t, os.WriteFile(path, []byte(original), 0o644))
	// Make the file readable but not writable so os.ReadFile succeeds while
	// os.WriteFile fails with permission denied, isolating the ErrEditFileWrite path.
	require.NoError(t, os.Chmod(path, 0o444))
	t.Cleanup(func() { _ = os.Chmod(path, 0o644) })

	_, err := tool.NewEditFileTool().Execute(context.Background(), jsonArgs(t, map[string]any{
		"path":    path,
		"old_str": "beta",
		"new_str": "BETA",
	}))
	require.ErrorIs(t, err, tool.ErrEditFileWrite, "expected ErrEditFileWrite, got %v", err)

	got, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, original, string(got), "file must be unchanged when the edit write fails")
}
