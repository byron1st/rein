package tool

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const maxBytes = 50 * 1024

const maxLines = 2000

var (
	ErrFailedToCreateOffloadDir = errors.New("failed to create offload dir")
	ErrFailedToWriteOffload     = errors.New("failed to write offload")
)

// capOutput bounds tool output to roughly maxBytes bytes or maxLines lines.
// Input at or under both bounds is returned unchanged. Oversized input is
// written in full to a temp file (cleanup delegated to the OS) and a head
// preview is returned with a trailing marker reporting the original size and
// the absolute path to the offloaded full content.
func capOutput(s string) (string, error) {
	if len(s) <= maxBytes && lineCount(s) <= maxLines {
		return s, nil
	}

	dir, err := os.MkdirTemp("", "rein-*")
	if err != nil {
		return "", errors.Join(ErrFailedToCreateOffloadDir, err)
	}
	path := filepath.Join(dir, "output.txt")
	if err := os.WriteFile(path, []byte(s), 0o644); err != nil {
		return "", errors.Join(ErrFailedToWriteOffload, err)
	}

	head := truncateHead(s)
	return head + fmt.Sprintf("\n[output truncated: %d bytes/%d lines. full output: %s]", len(s), lineCount(s), path), nil
}

// lineCount reports the number of lines in s. An empty string is zero lines;
// otherwise it is the newline count plus one (a trailing line with no newline
// still counts).
func lineCount(s string) int {
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
}

// truncateHead returns a preview of s bounded first by maxLines (keeping the
// leading lines) and then by maxBytes (cutting the byte tail).
func truncateHead(s string) string {
	result := s
	if lineCount(s) > maxLines {
		lines := strings.Split(s, "\n")
		result = strings.Join(lines[:maxLines], "\n")
	}
	if len(result) > maxBytes {
		result = result[:maxBytes]
	}
	return result
}
