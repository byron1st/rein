package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/byron1st/rein/pkg/httputil"
)

// maxRetries is the number of retries attempted after the initial request, for
// transport errors and 5xx responses, before giving up.
const maxRetries = 5

// defaultBaseDelay is the base backoff delay; each subsequent retry doubles it.
const defaultBaseDelay = 500 * time.Millisecond

var (
	ErrLLMEncodeRequest    = errors.New("llm encode request")
	ErrLLMBuildRequest     = errors.New("llm build request")
	ErrLLMTransport        = errors.New("llm transport")
	ErrLLMServerStatus     = errors.New("llm server status")
	ErrLLMStatus           = errors.New("llm status")
	ErrLLMDecode           = errors.New("llm decode response")
	ErrLLMCanceled         = errors.New("llm canceled")
	ErrLLMRetriesExhausted = errors.New("llm retries exhausted")
)

// Client is the OpenAI-compatible chat completions client the agent loop
// depends on.
type Client interface {
	CreateChatCompletion(ctx context.Context, req ChatCompletionRequest) (*ChatCompletionResponse, error)
}

type httpClient struct {
	baseURL   string
	apiKey    string
	doer      httputil.Client
	baseDelay time.Duration
}

// Option configures the client returned by New.
type Option func(*httpClient)

// WithHTTPClient overrides the HTTP transport, primarily so tests can inject a
// mock in place of the default httputil.Client.
func WithHTTPClient(c httputil.Client) Option {
	return func(hc *httpClient) { hc.doer = c }
}

// WithRetryBaseDelay overrides the base backoff delay between retries.
func WithRetryBaseDelay(d time.Duration) Option {
	return func(c *httpClient) { c.baseDelay = d }
}

// New builds a Client targeting baseURL. apiKey may be empty for local
// providers such as Ollama, in which case no Authorization header is sent.
func New(baseURL, apiKey string, opts ...Option) Client {
	c := &httpClient{
		baseURL:   baseURL,
		apiKey:    apiKey,
		doer:      httputil.New(),
		baseDelay: defaultBaseDelay,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func (c *httpClient) CreateChatCompletion(ctx context.Context, req ChatCompletionRequest) (*ChatCompletionResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, errors.Join(ErrLLMEncodeRequest, err)
	}

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			if err := c.backoff(ctx, attempt); err != nil {
				return nil, err
			}
		}
		resp, retry, err := c.do(ctx, body)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		if !retry {
			return nil, err
		}
	}
	return nil, errors.Join(ErrLLMRetriesExhausted, lastErr)
}

// do performs a single request attempt. The bool reports whether the error is
// transient (transport failure or 5xx) and the request should be retried.
func (c *httpClient) do(ctx context.Context, body []byte) (*ChatCompletionResponse, bool, error) {
	url := strings.TrimRight(c.baseURL, "/") + "/v1/chat/completions"
	var headers map[string]string
	if c.apiKey != "" {
		headers = map[string]string{"Authorization": "Bearer " + c.apiKey}
	}

	httpResp, err := c.doer.PostJSON(ctx, url, body, headers)
	if err != nil {
		if errors.Is(err, httputil.ErrBuildRequest) {
			return nil, false, errors.Join(ErrLLMBuildRequest, err)
		}
		return nil, true, errors.Join(ErrLLMTransport, err)
	}
	defer httpResp.Body.Close()

	switch {
	case httpResp.StatusCode >= 500:
		return nil, true, errors.Join(ErrLLMServerStatus, fmt.Errorf("status %d", httpResp.StatusCode))
	case httpResp.StatusCode >= 400:
		return nil, false, errors.Join(ErrLLMStatus, fmt.Errorf("status %d", httpResp.StatusCode))
	}

	var out ChatCompletionResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&out); err != nil {
		return nil, false, errors.Join(ErrLLMDecode, err)
	}
	return &out, false, nil
}

// backoff waits before retry number attempt, doubling the base delay each time,
// and returns early if the context is canceled.
func (c *httpClient) backoff(ctx context.Context, attempt int) error {
	delay := c.baseDelay << (attempt - 1)
	select {
	case <-time.After(delay):
		return nil
	case <-ctx.Done():
		return errors.Join(ErrLLMCanceled, ctx.Err())
	}
}
