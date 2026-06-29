package tool

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

// Sentinels for the filesystem tools. Each is used at exactly one call site.
var (
	ErrReadFileOpen = errors.New("failed to read file")
	ErrReadFileArgs = errors.New("invalid read_file arguments")

	ErrWriteFile     = errors.New("failed to write file")
	ErrWriteFileArgs = errors.New("invalid write_file arguments")

	ErrEditFileRead       = errors.New("failed to read file for edit")
	ErrEditFileNoMatch    = errors.New("old_str not found")
	ErrEditFileMultiMatch = errors.New("multiple matches found")
	ErrEditFileWrite      = errors.New("failed to write edited file")
	ErrEditFileArgs       = errors.New("invalid edit_file arguments")
)

// readFileTool reads a file from the local filesystem, with optional
// 1-indexed line offset and line limit, and caps its output.
type readFileTool struct{}

// NewReadFileTool returns a Tool that reads files.
func NewReadFileTool() Tool { return &readFileTool{} }

func (r *readFileTool) Name() string { return "read_file" }

func (r *readFileTool) Schema() json.RawMessage {
	return json.RawMessage(`{
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
	}`)
}

type readFileArgs struct {
	Path   string `json:"path"`
	Offset *int   `json:"offset"`
	Limit  *int   `json:"limit"`
}

func (r *readFileTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var a readFileArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", errors.Join(ErrReadFileArgs, err)
	}
	if a.Path == "" {
		return "", errors.Join(ErrReadFileArgs, fmt.Errorf("path is required"))
	}

	data, err := os.ReadFile(a.Path)
	if err != nil {
		return "", errors.Join(ErrReadFileOpen, err)
	}

	content := string(data)
	if a.Offset != nil || a.Limit != nil {
		content = sliceLines(content, a.Offset, a.Limit)
	}
	return capOutput(content)
}

// sliceLines returns the substring of content spanning the 1-indexed line
// [offset, offset+limit). offset defaults to 1 and limit to all remaining
// lines when nil or non-positive. An offset past the last line yields "".
func sliceLines(content string, offset, limit *int) string {
	lines := strings.Split(content, "\n")

	start := 1
	if offset != nil && *offset > 0 {
		start = *offset
	}
	if start > len(lines) {
		return ""
	}
	startIdx := start - 1

	endIdx := len(lines)
	if limit != nil && *limit > 0 {
		endIdx = min(startIdx+*limit, len(lines))
	}
	return strings.Join(lines[startIdx:endIdx], "\n")
}

// writeFileTool creates or overwrites a file with the given content.
type writeFileTool struct{}

// NewWriteFileTool returns a Tool that writes files.
func NewWriteFileTool() Tool { return &writeFileTool{} }

func (w *writeFileTool) Name() string { return "write_file" }

func (w *writeFileTool) Schema() json.RawMessage {
	return json.RawMessage(`{
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
	}`)
}

type writeFileArgs struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

func (w *writeFileTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var a writeFileArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", errors.Join(ErrWriteFileArgs, err)
	}
	if a.Path == "" {
		return "", errors.Join(ErrWriteFileArgs, fmt.Errorf("path is required"))
	}

	if err := os.WriteFile(a.Path, []byte(a.Content), 0o644); err != nil {
		return "", errors.Join(ErrWriteFile, err)
	}
	return fmt.Sprintf("wrote %d bytes to %s", len(a.Content), a.Path), nil
}

// editFileTool replaces a unique occurrence of old_str with new_str in a file.
// It refuses to edit when old_str does not match exactly once.
type editFileTool struct{}

// NewEditFileTool returns a Tool that edits files by unique-match replacement.
func NewEditFileTool() Tool { return &editFileTool{} }

func (e *editFileTool) Name() string { return "edit_file" }

func (e *editFileTool) Schema() json.RawMessage {
	return json.RawMessage(`{
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
	}`)
}

type editFileArgs struct {
	Path   string `json:"path"`
	OldStr string `json:"old_str"`
	NewStr string `json:"new_str"`
}

func (e *editFileTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var a editFileArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", errors.Join(ErrEditFileArgs, err)
	}
	if a.Path == "" {
		return "", errors.Join(ErrEditFileArgs, fmt.Errorf("path is required"))
	}

	data, err := os.ReadFile(a.Path)
	if err != nil {
		return "", errors.Join(ErrEditFileRead, err)
	}

	content := string(data)
	n := strings.Count(content, a.OldStr)
	switch n {
	case 0:
		return "", errors.Join(ErrEditFileNoMatch, fmt.Errorf("old_str not found in %s", a.Path))
	case 1:
		updated := strings.Replace(content, a.OldStr, a.NewStr, 1)
		if err := os.WriteFile(a.Path, []byte(updated), 0o644); err != nil {
			return "", errors.Join(ErrEditFileWrite, err)
		}
		return fmt.Sprintf("edited %s", a.Path), nil
	default:
		return "", errors.Join(ErrEditFileMultiMatch, fmt.Errorf("%d matches found; provide more surrounding context", n))
	}
}
