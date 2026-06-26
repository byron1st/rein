package llm_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/byron1st/rein/pkg/httputil"
	"github.com/byron1st/rein/pkg/llm"
	"github.com/byron1st/rein/pkg/mocks/mockhttputil"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// totalAttempts is one initial request plus the five retries the client makes.
const totalAttempts = 6

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

func sampleRequest() llm.ChatCompletionRequest {
	return llm.ChatCompletionRequest{
		Model:    "gpt-test",
		Messages: []llm.Message{{Role: "user", Content: new("hi")}},
	}
}

// newTestClient builds a client wired to the mock transport with a negligible
// backoff so retry tests stay fast and deterministic.
func newTestClient(apiKey string, doer httputil.Client) llm.Client {
	return llm.New("http://provider.test", apiKey,
		llm.WithHTTPClient(doer),
		llm.WithRetryBaseDelay(time.Microsecond),
	)
}

func TestCreateChatCompletion_ParsesContent(t *testing.T) {
	doer := mockhttputil.NewMockClient(t)
	doer.EXPECT().PostJSON(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(jsonResponse(200, `{"choices":[{"message":{"role":"assistant","content":"hello"},"finish_reason":"stop"}]}`), nil).
		Once()

	client := newTestClient("key", doer)
	resp, err := client.CreateChatCompletion(context.Background(), sampleRequest())

	require.NoError(t, err)
	require.Len(t, resp.Choices, 1)
	require.NotNil(t, resp.Choices[0].Message.Content)
	require.Equal(t, "hello", *resp.Choices[0].Message.Content)
	require.Equal(t, "stop", resp.Choices[0].FinishReason)
}

func TestCreateChatCompletion_ParsesToolCalls(t *testing.T) {
	body := `{"choices":[{"message":{"role":"assistant","content":null,` +
		`"tool_calls":[{"id":"call_1","type":"function",` +
		`"function":{"name":"read_file","arguments":"{\"path\":\"foo.go\"}"}}]},` +
		`"finish_reason":"tool_calls"}]}`
	doer := mockhttputil.NewMockClient(t)
	doer.EXPECT().PostJSON(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(jsonResponse(200, body), nil).Once()

	client := newTestClient("key", doer)
	resp, err := client.CreateChatCompletion(context.Background(), sampleRequest())

	require.NoError(t, err)
	msg := resp.Choices[0].Message
	require.Nil(t, msg.Content, "assistant tool-call message should decode content:null as nil")
	require.Len(t, msg.ToolCalls, 1)
	require.Equal(t, "call_1", msg.ToolCalls[0].ID)
	require.Equal(t, "function", msg.ToolCalls[0].Type)
	require.Equal(t, "read_file", msg.ToolCalls[0].Function.Name)
	require.JSONEq(t, `{"path":"foo.go"}`, msg.ToolCalls[0].Function.Arguments)
}

func TestCreateChatCompletion_SendsRequest(t *testing.T) {
	doer := mockhttputil.NewMockClient(t)
	doer.EXPECT().PostJSON(mock.Anything, mock.Anything, mock.Anything, mock.Anything).RunAndReturn(func(ctx context.Context, url string, body []byte, headers map[string]string) (*http.Response, error) {
		require.Equal(t, "http://provider.test/v1/chat/completions", url)

		var decoded map[string]any
		require.NoError(t, json.Unmarshal(body, &decoded))
		require.Equal(t, "gpt-test", decoded["model"])
		require.Contains(t, decoded, "messages")
		require.Contains(t, decoded, "tools")
		return jsonResponse(200, `{"choices":[]}`), nil
	}).Once()

	client := newTestClient("key", doer)
	req := sampleRequest()
	req.Tools = []llm.ToolDefinition{{Type: "function", Function: json.RawMessage(`{"name":"read_file"}`)}}
	_, err := client.CreateChatCompletion(context.Background(), req)

	require.NoError(t, err)
}

func TestCreateChatCompletion_OmitsToolsWhenEmpty(t *testing.T) {
	doer := mockhttputil.NewMockClient(t)
	doer.EXPECT().PostJSON(mock.Anything, mock.Anything, mock.Anything, mock.Anything).RunAndReturn(func(ctx context.Context, url string, body []byte, headers map[string]string) (*http.Response, error) {
		var decoded map[string]any
		require.NoError(t, json.Unmarshal(body, &decoded))
		require.NotContains(t, decoded, "tools", "tools must be omitted when no tools are registered")
		return jsonResponse(200, `{"choices":[]}`), nil
	}).Once()

	client := newTestClient("key", doer)
	_, err := client.CreateChatCompletion(context.Background(), sampleRequest())

	require.NoError(t, err)
}

func TestCreateChatCompletion_RetriesThenSucceeds(t *testing.T) {
	doer := mockhttputil.NewMockClient(t)
	doer.EXPECT().PostJSON(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(jsonResponse(500, ""), nil).Times(2)
	doer.EXPECT().PostJSON(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(jsonResponse(200, `{"choices":[{"message":{"role":"assistant","content":"ok"}}]}`), nil).
		Once()

	client := newTestClient("key", doer)
	resp, err := client.CreateChatCompletion(context.Background(), sampleRequest())

	require.NoError(t, err)
	require.Equal(t, "ok", *resp.Choices[0].Message.Content)
	doer.AssertNumberOfCalls(t, "PostJSON", 3)
}

func TestCreateChatCompletion_RetriesExhaustedOnServerError(t *testing.T) {
	doer := mockhttputil.NewMockClient(t)
	doer.EXPECT().PostJSON(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(jsonResponse(500, ""), nil).Times(totalAttempts)

	client := newTestClient("key", doer)
	_, err := client.CreateChatCompletion(context.Background(), sampleRequest())

	require.ErrorIs(t, err, llm.ErrLLMRetriesExhausted)
	require.ErrorIs(t, err, llm.ErrLLMServerStatus)
	doer.AssertNumberOfCalls(t, "PostJSON", totalAttempts)
}

func TestCreateChatCompletion_RetriesExhaustedOnTransportError(t *testing.T) {
	doer := mockhttputil.NewMockClient(t)
	doer.EXPECT().PostJSON(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("dial tcp: connection refused")).Times(totalAttempts)

	client := newTestClient("key", doer)
	_, err := client.CreateChatCompletion(context.Background(), sampleRequest())

	require.ErrorIs(t, err, llm.ErrLLMRetriesExhausted)
	require.ErrorIs(t, err, llm.ErrLLMTransport)
	doer.AssertNumberOfCalls(t, "PostJSON", totalAttempts)
}

func TestCreateChatCompletion_ClientErrorNoRetry(t *testing.T) {
	doer := mockhttputil.NewMockClient(t)
	doer.EXPECT().PostJSON(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(jsonResponse(400, `{"error":"bad request"}`), nil).Once()

	client := newTestClient("key", doer)
	_, err := client.CreateChatCompletion(context.Background(), sampleRequest())

	require.ErrorIs(t, err, llm.ErrLLMStatus)
	require.NotErrorIs(t, err, llm.ErrLLMRetriesExhausted)
	doer.AssertNumberOfCalls(t, "PostJSON", 1)
}

func TestCreateChatCompletion_DecodeError(t *testing.T) {
	doer := mockhttputil.NewMockClient(t)
	doer.EXPECT().PostJSON(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(jsonResponse(200, `{not json`), nil).Once()

	client := newTestClient("key", doer)
	_, err := client.CreateChatCompletion(context.Background(), sampleRequest())

	require.ErrorIs(t, err, llm.ErrLLMDecode)
	doer.AssertNumberOfCalls(t, "PostJSON", 1)
}

func TestCreateChatCompletion_EncodeError(t *testing.T) {
	doer := mockhttputil.NewMockClient(t)
	// No PostJSON expectation: marshalling must fail before any transport call,
	// so the mock's cleanup also asserts the transport was never reached.

	client := newTestClient("key", doer)
	req := sampleRequest()
	// An invalid json.RawMessage tool schema makes json.Marshal(req) fail.
	req.Tools = []llm.ToolDefinition{{Type: "function", Function: json.RawMessage(`{invalid`)}}
	_, err := client.CreateChatCompletion(context.Background(), req)

	require.ErrorIs(t, err, llm.ErrLLMEncodeRequest)
}

func TestCreateChatCompletion_BuildRequestErrorNoRetry(t *testing.T) {
	doer := mockhttputil.NewMockClient(t)
	doer.EXPECT().PostJSON(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, errors.Join(httputil.ErrBuildRequest, errors.New("bad url"))).Once()

	client := newTestClient("key", doer)
	_, err := client.CreateChatCompletion(context.Background(), sampleRequest())

	require.ErrorIs(t, err, llm.ErrLLMBuildRequest)
	require.NotErrorIs(t, err, llm.ErrLLMTransport)
	require.NotErrorIs(t, err, llm.ErrLLMRetriesExhausted)
	doer.AssertNumberOfCalls(t, "PostJSON", 1)
}

func TestCreateChatCompletion_AuthorizationHeader(t *testing.T) {
	tests := []struct {
		name       string
		apiKey     string
		wantHeader string
	}{
		{name: "with api key sends bearer", apiKey: "secret", wantHeader: "Bearer secret"},
		{name: "without api key omits header", apiKey: "", wantHeader: ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			doer := mockhttputil.NewMockClient(t)
			doer.EXPECT().PostJSON(mock.Anything, mock.Anything, mock.Anything, mock.Anything).RunAndReturn(func(ctx context.Context, url string, body []byte, headers map[string]string) (*http.Response, error) {
				require.Equal(t, tc.wantHeader, headers["Authorization"])
				return jsonResponse(200, `{"choices":[]}`), nil
			}).Once()

			client := newTestClient(tc.apiKey, doer)
			_, err := client.CreateChatCompletion(context.Background(), sampleRequest())

			require.NoError(t, err)
		})
	}
}

func TestCreateChatCompletion_HonorsContextCancellationDuringBackoff(t *testing.T) {
	doer := mockhttputil.NewMockClient(t)
	doer.EXPECT().PostJSON(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(jsonResponse(500, ""), nil).Once()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	client := llm.New("http://provider.test", "key",
		llm.WithHTTPClient(doer),
		llm.WithRetryBaseDelay(time.Hour), // backoff would block; cancellation must win
	)
	_, err := client.CreateChatCompletion(ctx, sampleRequest())

	require.ErrorIs(t, err, llm.ErrLLMCanceled)
	doer.AssertNumberOfCalls(t, "PostJSON", 1)
}
