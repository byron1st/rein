package tool_test

import (
	"os"
	"strings"
	"testing"

	"github.com/byron1st/rein/pkg/tool"
	"github.com/stretchr/testify/require"
)

// parseOffloadPath extracts the absolute temp-file path printed in the
// truncation marker so the test can read the offloaded full content back and
// compare it to the original input.
func parseOffloadPath(t *testing.T, result string) string {
	t.Helper()
	const marker = "full output: "
	idx := strings.Index(result, marker)
	require.GreaterOrEqualf(t, idx, 0, "marker %q not found in result", marker)
	rest := result[idx+len(marker):]
	end := strings.LastIndex(rest, "]")
	require.GreaterOrEqualf(t, end, 0, "trailing ] not found after marker")
	return rest[:end]
}

func TestCapOutput_UnderLimit_ReturnedUnchanged(t *testing.T) {
	s := "hello world"
	result, err := tool.CapOutput(s)

	require.NoError(t, err)
	require.Equal(t, s, result, "under-limit input must be returned verbatim")
	require.NotContains(t, result, "[output truncated", "no truncation marker when under limit")
}

func TestCapOutput_OverBytes_TruncatesAndOffloads(t *testing.T) {
	s := strings.Repeat("a", tool.MaxBytes+1)
	result, err := tool.CapOutput(s)

	require.NoError(t, err)
	require.Truef(t, strings.HasPrefix(result, s[:tool.MaxBytes]),
		"head must start with the first %d bytes", tool.MaxBytes)
	require.Contains(t, result, "[output truncated", "byte-overflow must add truncation marker")

	path := parseOffloadPath(t, result)
	full, err := os.ReadFile(path)
	require.NoError(t, err, "offloaded file must be readable at %s", path)
	require.Equal(t, s, string(full), "offloaded file must contain the full original input")
}

func TestCapOutput_OverLines_TruncatesAndOffloads(t *testing.T) {
	lines := make([]string, tool.MaxLines+1)
	for i := range lines {
		lines[i] = "x"
	}
	s := strings.Join(lines, "\n")
	result, err := tool.CapOutput(s)

	require.NoError(t, err)
	wantHead := strings.Join(lines[:tool.MaxLines], "\n")
	require.Truef(t, strings.HasPrefix(result, wantHead),
		"head must start with the first %d lines", tool.MaxLines)
	require.Contains(t, result, "[output truncated", "line-overflow must add truncation marker")

	path := parseOffloadPath(t, result)
	full, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, s, string(full), "offloaded file must contain the full original input")
}

func TestCapOutput_BoundaryExactlyMaxBytes_NotTruncated(t *testing.T) {
	s := strings.Repeat("a", tool.MaxBytes)
	result, err := tool.CapOutput(s)

	require.NoError(t, err)
	require.Equal(t, s, result, "exactly maxBytes must not exceed the bound")
	require.NotContains(t, result, "[output truncated")
}

func TestCapOutput_BoundaryExactlyMaxLines_NotTruncated(t *testing.T) {
	s := strings.Repeat("x\n", tool.MaxLines-1) + "x"
	result, err := tool.CapOutput(s)

	require.NoError(t, err)
	require.Equal(t, s, result, "exactly maxLines must not exceed the bound")
	require.NotContains(t, result, "[output truncated")
}
