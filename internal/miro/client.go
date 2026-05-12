package miro

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	defaultTimeout  = 30 * time.Second
	maxResponseBody = 10 * 1024 * 1024
	userAgent       = "miro-cli/0.0"
)

// Client is the hand-authored Miro REST client. It owns no goroutines and
// is safe for concurrent use across requests. One Client per process is
// the common case; tests build per-test Clients pointing at httptest
// servers via WithHTTPClient + WithBaseURL.
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
	userAgent  string
}

// Option configures a Client.
type Option func(*Client)

// WithBaseURL overrides DefaultBaseURL. Used by tests and by users
// pointing at a sandbox/staging Miro environment.
func WithBaseURL(u string) Option {
	return func(c *Client) {
		c.baseURL = strings.TrimRight(u, "/")
	}
}

// WithHTTPClient injects an http.Client. Tests use this to point at
// httptest servers; callers in production can use it to install custom
// transports (connection pooling, instrumentation) once Phase 4 lands.
func WithHTTPClient(h *http.Client) Option {
	return func(c *Client) {
		c.httpClient = h
	}
}

// WithUserAgent overrides the User-Agent header. Empty string disables.
func WithUserAgent(ua string) Option {
	return func(c *Client) {
		c.userAgent = ua
	}
}

// New constructs a Client from a Config. The returned Client is ready
// for use; close-equivalent is not required.
func New(cfg *Config, opts ...Option) *Client {
	c := &Client{
		baseURL:    cfg.BaseURL,
		token:      cfg.Token,
		httpClient: &http.Client{Timeout: defaultTimeout},
		userAgent:  userAgent,
	}
	for _, opt := range opts {
		opt(c)
	}
	if c.baseURL == "" {
		c.baseURL = DefaultBaseURL
	}
	return c
}

// Do issues an HTTP request to path (joined onto the base URL) with the
// given method and body. The bearer token is attached automatically.
// Non-2xx responses are returned as *APIError; 2xx responses are decoded
// into out (which may be nil to discard the body).
//
// body may be:
//   - nil: no request body
//   - []byte: sent as-is with Content-Type: application/octet-stream
//   - everything else: JSON-encoded with Content-Type: application/json
//
// Context cancellation is honored end-to-end.
func (c *Client) Do(ctx context.Context, method, path string, body, out any) error {
	if c == nil {
		return errors.New("miro: nil client")
	}
	url := c.baseURL + path

	var (
		bodyReader  io.Reader
		contentType string
	)
	switch v := body.(type) {
	case nil:
		// no body
	case []byte:
		bodyReader = bytes.NewReader(v)
		contentType = "application/octet-stream"
	default:
		buf, err := json.Marshal(v)
		if err != nil {
			return fmt.Errorf("miro: marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(buf)
		contentType = "application/json"
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return fmt.Errorf("miro: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("miro: %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	limited := io.LimitReader(resp.Body, maxResponseBody)
	respBody, err := io.ReadAll(limited)
	if err != nil {
		return fmt.Errorf("miro: read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &APIError{
			Method: method,
			Path:   path,
			Status: resp.StatusCode,
			Body:   string(respBody),
		}
	}

	if out == nil || len(respBody) == 0 {
		return nil
	}
	if err := json.Unmarshal(respBody, out); err != nil {
		return fmt.Errorf("miro: decode response: %w", err)
	}
	return nil
}

// Get is a convenience for Do(ctx, GET, path, nil, out).
func (c *Client) Get(ctx context.Context, path string, out any) error {
	return c.Do(ctx, http.MethodGet, path, nil, out)
}

// Post is a convenience for Do(ctx, POST, path, body, out).
func (c *Client) Post(ctx context.Context, path string, body, out any) error {
	return c.Do(ctx, http.MethodPost, path, body, out)
}

// Patch is a convenience for Do(ctx, PATCH, path, body, out).
func (c *Client) Patch(ctx context.Context, path string, body, out any) error {
	return c.Do(ctx, http.MethodPatch, path, body, out)
}

// Put is a convenience for Do(ctx, PUT, path, body, out).
func (c *Client) Put(ctx context.Context, path string, body, out any) error {
	return c.Do(ctx, http.MethodPut, path, body, out)
}

// Delete is a convenience for Do(ctx, DELETE, path, nil, nil).
func (c *Client) Delete(ctx context.Context, path string) error {
	return c.Do(ctx, http.MethodDelete, path, nil, nil)
}
