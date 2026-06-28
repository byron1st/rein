package httputil

import (
	"bytes"
	"context"
	"errors"
	"net/http"
)

var (
	ErrFailedToBuildRequest  = errors.New("failed to build request")
	ErrFailedToDoHTTPRequest = errors.New("failed to do http request")
)

// Client abstracts a JSON POST round trip so callers can mock the transport in
// tests.
//
//go:generate mockery
type Client interface {
	PostJSON(ctx context.Context, url string, body []byte, headers map[string]string) (*http.Response, error)
}

type httpClient struct {
	http *http.Client
}

// New returns a Client backed by a default *http.Client.
func New() Client {
	return &httpClient{http: &http.Client{}}
}

// PostJSON builds a POST request with the given body and headers and performs
// the round trip. Content-Type defaults to application/json and can be
// overridden through headers. It returns the raw response without inspecting
// its status or reading its body.
func (c *httpClient) PostJSON(ctx context.Context, url string, body []byte, headers map[string]string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, errors.Join(ErrFailedToBuildRequest, err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, errors.Join(ErrFailedToDoHTTPRequest, err)
	}
	return resp, nil
}
