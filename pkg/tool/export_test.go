package tool

// export_test.go bridges the package-internal cap helper and its bounds to the
// external tool_test package. The sub-plan deliberately keeps capOutput,
// maxBytes and maxLines unexported (they are a shared helper invoked by the
// same-package tools in Step 3/4), while go-conventions.md requires test
// functions to live in the external package. These aliases are visible only to
// the test build and contain no test logic.

var CapOutput = capOutput

const (
	MaxBytes = maxBytes
	MaxLines = maxLines
)
