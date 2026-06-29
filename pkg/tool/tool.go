package tool

import (
	"context"
	"encoding/json"
)

// Tool is the contract every agent tool implements. A tool carries its own
// OpenAI function schema and executes against JSON-encoded arguments.
type Tool interface {
	// Name uniquely identifies the tool; it matches the OpenAI function name.
	Name() string
	// Schema returns the OpenAI function object's INNER JSON: an object with
	// "name", "description" and "parameters" keys. The agent loop wraps this as
	// {"type":"function","function": <schema>} when sending tools to the LLM.
	Schema() json.RawMessage
	// Execute runs the tool against the JSON-encoded arguments and returns the
	// output string or an error. The caller is responsible for capping output.
	Execute(ctx context.Context, args json.RawMessage) (string, error)
}
