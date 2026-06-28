package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/byron1st/rein/pkg/httputil"
)

// maxRetries is the number of retries attempted after the initial request, for
// transport errors and 5xx responses, before giving up.
const maxRetries = 5

// defaultBaseDelay is the base backoff delay; each subsequent retry doubles it.
const defaultBaseDelay = 500 * time.Millisecond

var (
	ErrFailedToParseLLMBaseURL   = errors.New("failed to parse llm base url")
	ErrFailedToEncodeLLMRequest  = errors.New("failed to encode llm request")
	ErrInternalLLMServerStatus   = errors.New("internal llm server status")
	ErrBadRequestToLLMStatus     = errors.New("bad request tollm status")
	ErrFailedToDecodeLLMResponse = errors.New("failed to decode llm response")
	ErrLLMContextCanceled        = errors.New("llm context canceled")
	ErrLLMRetriesExhausted       = errors.New("llm retries exhausted")
)

// Client is the OpenAI-compatible chat completions client the agent loop
// depends on.
type Client interface {
	CreateChatCompletion(ctx context.Context, req ChatCompletionRequest) (*ChatCompletionResponse, error)
}

type openAICompatibleClient struct {
	baseURL    string
	apiKey     string
	httpClient httputil.Client
	baseDelay  time.Duration
}

// Option configures the client returned by New.
type Option func(*openAICompatibleClient)

// WithHTTPClient overrides the HTTP transport, primarily so tests can inject a
// mock in place of the default httputil.Client.
func WithHTTPClient(c httputil.Client) Option {
	return func(hc *openAICompatibleClient) { hc.httpClient = c }
}

// WithRetryBaseDelay overrides the base backoff delay between retries.
func WithRetryBaseDelay(d time.Duration) Option {
	return func(c *openAICompatibleClient) { c.baseDelay = d }
}

// MustNew builds a Client targeting baseURL. apiKey may be empty for local
// providers such as Ollama, in which case no Authorization header is sent.
func MustNew(baseURL, apiKey string, opts ...Option) Client {
	parsedUrl, err := url.Parse(baseURL)
	if err != nil {
		panic(errors.Join(ErrFailedToParseLLMBaseURL, err))
	}

	c := &openAICompatibleClient{
		baseURL:    parsedUrl.JoinPath("v1", "chat", "completions").String(),
		apiKey:     apiKey,
		httpClient: httputil.New(),
		baseDelay:  defaultBaseDelay,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func (c *openAICompatibleClient) CreateChatCompletion(ctx context.Context, req ChatCompletionRequest) (*ChatCompletionResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, errors.Join(ErrFailedToEncodeLLMRequest, err)
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
func (c *openAICompatibleClient) do(ctx context.Context, body []byte) (*ChatCompletionResponse, bool, error) {
	var headers map[string]string
	if c.apiKey != "" {
		headers = map[string]string{"Authorization": "Bearer " + c.apiKey}
	}

	httpResp, err := c.httpClient.PostJSON(ctx, c.baseURL, body, headers)
	if err != nil {
		if errors.Is(err, httputil.ErrFailedToBuildRequest) {
			return nil, false, err
		}
		return nil, true, err
	}
	defer func() { _ = httpResp.Body.Close() }()

	switch {
	case httpResp.StatusCode >= 500:
		return nil, true, errors.Join(ErrInternalLLMServerStatus, fmt.Errorf("status %d", httpResp.StatusCode))
	case httpResp.StatusCode >= 400:
		return nil, false, errors.Join(ErrBadRequestToLLMStatus, fmt.Errorf("status %d", httpResp.StatusCode))
	}

	var out ChatCompletionResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&out); err != nil {
		return nil, false, errors.Join(ErrFailedToDecodeLLMResponse, err)
	}
	return &out, false, nil
}

// backoff waits before retry number attempt, doubling the base delay each time,
// and returns early if the context is canceled.
func (c *openAICompatibleClient) backoff(ctx context.Context, attempt int) error {
	delay := c.baseDelay << (attempt - 1)
	select {
	case <-time.After(delay):
		return nil
	case <-ctx.Done():
		return errors.Join(ErrLLMContextCanceled, ctx.Err())
	}
}
