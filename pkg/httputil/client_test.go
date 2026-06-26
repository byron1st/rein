package httputil_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/byron1st/rein/pkg/httputil"
	"github.com/stretchr/testify/require"
)

func TestPostJSON_SendsRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "application/json", r.Header.Get("Content-Type"))
		require.Equal(t, "Bearer secret", r.Header.Get("Authorization"))

		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		require.Equal(t, "value", body["key"])

		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"ok":true}`)
	}))
	defer server.Close()

	// Content-Type is not supplied here: PostJSON must default it to application/json.
	headers := map[string]string{"Authorization": "Bearer secret"}
	resp, err := httputil.New().PostJSON(context.Background(), server.URL, []byte(`{"key":"value"}`), headers)

	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestPostJSON_HeadersOverrideDefaultContentType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "application/vnd.api+json", r.Header.Get("Content-Type"))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	headers := map[string]string{"Content-Type": "application/vnd.api+json"}
	resp, err := httputil.New().PostJSON(context.Background(), server.URL, []byte(`{}`), headers)

	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestPostJSON_BuildError(t *testing.T) {
	_, err := httputil.New().PostJSON(context.Background(), "://bad", []byte(`{}`), nil)

	require.ErrorIs(t, err, httputil.ErrBuildRequest)
}
