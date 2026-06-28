package llm

import "encoding/json"

// Message is a single OpenAI-compatible chat message. Content is a pointer so an
// assistant tool-call message can carry content:null (nil) distinctly from an
// empty string, and is omitted from the request when nil.
type Message struct {
	Role       string     `json:"role"`
	Content    *string    `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	Name       string     `json:"name,omitempty"`
}

// ToolCall is a function tool call requested by the model.
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

// FunctionCall names the called function and carries its JSON-encoded arguments
// as a string, matching the OpenAI tool-calling wire format.
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ToolDefinition advertises a callable tool to the model. Function is the raw
// JSON schema owned by each tool.
type ToolDefinition struct {
	Type     string          `json:"type"`
	Function json.RawMessage `json:"function"`
}

// ChatCompletionRequest is the POST body for /v1/chat/completions.
type ChatCompletionRequest struct {
	Model    string           `json:"model"`
	Messages []Message        `json:"messages"`
	Tools    []ToolDefinition `json:"tools,omitempty"`
}

// ChatCompletionResponse is the non-streaming completion response.
type ChatCompletionResponse struct {
	Choices []Choice `json:"choices"`
}

// Choice is one completion candidate.
type Choice struct {
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}
