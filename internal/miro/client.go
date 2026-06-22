package miro

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	defaultTimeout  = 30 * time.Second
	maxResponseBody = 10 * 1024 * 1024
	userAgent       = "github.com/olgasafonova/miro-cli/0.0"

	// Retry policy for transient failures (429 + 5xx + transport errors).
	// Only applied to requests with a replayable body.
	maxRetryAttempts = 4 // 1 initial attempt + 3 retries
	baseRetryDelay   = 500 * time.Millisecond
	maxRetryDelay    = 30 * time.Second
)

// isRetryableStatus reports whether an HTTP status warrants a retry: server-side
// throttling (429) and transient gateway/availability errors (502/503/504).
func isRetryableStatus(status int) bool {
	switch status {
	case http.StatusTooManyRequests,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		return true
	}
	return false
}

// parseRetryAfter parses a Retry-After header given as an integer number of
// seconds. The HTTP-date form is not honored (returns 0). The result is
// clamped to maxRetryDelay so a hostile server can't pin the client.
func parseRetryAfter(h string) time.Duration {
	h = strings.TrimSpace(h)
	if h == "" {
		return 0
	}
	secs, err := strconv.Atoi(h)
	if err != nil || secs < 0 {
		return 0
	}
	if d := time.Duration(secs) * time.Second; d <= maxRetryDelay {
		return d
	}
	return maxRetryDelay
}

// retrySleep waits before the next attempt: the server's Retry-After when
// provided, otherwise capped exponential backoff. Returns ctx.Err() if the
// context is cancelled while waiting.
func retrySleep(ctx context.Context, attempt int, retryAfter time.Duration) error {
	delay := retryAfter
	if delay <= 0 {
		delay = baseRetryDelay << attempt // 500ms, 1s, 2s, ...
		if delay > maxRetryDelay {
			delay = maxRetryDelay
		}
	}
	t := time.NewTimer(delay)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// Client is the hand-authored Miro REST client. It owns no goroutines and
// is safe for concurrent use across requests. One Client per process is
// the common case; tests build per-test Clients pointing at httptest
// servers via WithHTTPClient + WithBaseURL.
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
	userAgent  string
	limiter    *Limiter
	cache      *Cache
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

// WithRateLimit installs a token-bucket rate limiter. Every call to Do
// blocks at most until a token is available (or ctx is cancelled).
// Pass NewLimiter(0, 0) to explicitly disable limiting; the default
// (no WithRateLimit option) also disables it so tests don't pay the
// pacing cost. CLI entry points should install one with DefaultRateLimit
// so scripted use stays under Miro's published per-org rate budget.
func WithRateLimit(l *Limiter) Option {
	return func(c *Client) {
		c.limiter = l
	}
}

// WithCache installs an LRU+TTL response cache for GET requests. Hits skip
// the network and the rate limiter; non-GET requests bypass the cache
// entirely. Pass nil (or omit the option) to disable caching — useful in
// tests and for one-shot CLI invocations where caching can't help.
func WithCache(cache *Cache) Option {
	return func(c *Client) {
		c.cache = cache
	}
}

// New constructs a Client from a Config. The returned Client is ready
// for use; close-equivalent is not required.
//
// Each Client gets its own *http.Transport (cloned from the package
// default) rather than sharing http.DefaultTransport. Two reasons:
//
//   - Production: one Client per CLI invocation, so behaviour is the
//     same as the default — connection pool is per-process.
//   - Tests: parallel tests each spin up an httptest.Server and close it
//     on cleanup. With a shared default transport, one test's srv.Close
//     can evict idle connections another test's in-flight request was
//     about to reuse, surfacing as "http: CloseIdleConnections called"
//     under CI timing. Per-Client transports isolate the pools.
func New(cfg *Config, opts ...Option) *Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	c := &Client{
		baseURL: cfg.BaseURL,
		token:   cfg.Token,
		httpClient: &http.Client{
			Timeout:   defaultTimeout,
			Transport: transport,
			// Make the base-URL pin explicit: never follow redirects, so
			// an upstream 3xx can't bounce an authenticated request to a
			// host outside api.miro.com. The base URL is a constant, but
			// pinning the policy keeps the guarantee from depending on it.
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		userAgent: userAgent,
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
//   - *MultipartBody: sent as-is with the supplied Content-Type (used by
//     file uploads in internal/tools/uploads)
//   - everything else: JSON-encoded with Content-Type: application/json
//
// Context cancellation is honored end-to-end.
func (c *Client) Do(ctx context.Context, method, path string, body, out any) error {
	if c == nil {
		return errors.New("miro: nil client")
	}
	if err := validatePath(path); err != nil {
		return err
	}
	url := c.baseURL + path

	cacheable := method == http.MethodGet && body == nil
	var cacheKey string
	if cacheable {
		cacheKey = method + " " + path
		if cached, ok := c.cache.Get(cacheKey); ok {
			return decodeCached(cached, out)
		}
	}

	rb, err := prepareBody(body)
	if err != nil {
		return err
	}

	status, respBody, err := c.executeWithRetry(ctx, method, path, url, rb)
	if err != nil {
		return err
	}

	if status < 200 || status >= 300 {
		return &APIError{Method: method, Path: path, Status: status, Body: string(respBody)}
	}

	if cacheable {
		// Copy the body — respBody backs LimitReader's buffer, and the
		// cache may outlive this call. Without the copy, a concurrent
		// reader would race the next call's read into the same buffer.
		stored := make([]byte, len(respBody))
		copy(stored, respBody)
		c.cache.Put(cacheKey, stored)
	}

	if out == nil || len(respBody) == 0 {
		return nil
	}
	if err := json.Unmarshal(respBody, out); err != nil {
		return fmt.Errorf("miro: decode response: %w", err)
	}
	return nil
}

// requestBody is a prepared request payload. When replayable, it can be
// re-sent across retries; a streamed (multipart) body is single-use.
type requestBody struct {
	bytes       []byte
	stream      io.Reader
	contentType string
	replayable  bool
}

// prepareBody normalizes the caller's body into a requestBody.
func prepareBody(body any) (requestBody, error) {
	switch v := body.(type) {
	case nil:
		return requestBody{replayable: true}, nil
	case []byte:
		return requestBody{bytes: v, contentType: "application/octet-stream", replayable: true}, nil
	case *MultipartBody:
		if v == nil || v.Body == nil {
			return requestBody{}, errors.New("miro: nil MultipartBody")
		}
		return requestBody{stream: v.Body, contentType: v.ContentType}, nil
	default:
		buf, err := json.Marshal(v)
		if err != nil {
			return requestBody{}, fmt.Errorf("miro: marshal request body: %w", err)
		}
		return requestBody{bytes: buf, contentType: "application/json", replayable: true}, nil
	}
}

// reader returns a fresh reader for one attempt (nil for an empty body).
func (rb requestBody) reader() io.Reader {
	if !rb.replayable {
		return rb.stream
	}
	if rb.bytes == nil {
		return nil
	}
	return bytes.NewReader(rb.bytes)
}

// newRequest builds an authenticated request for a single attempt.
func (c *Client) newRequest(ctx context.Context, method, url string, rb requestBody) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, rb.reader())
	if err != nil {
		return nil, fmt.Errorf("miro: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}
	if rb.contentType != "" {
		req.Header.Set("Content-Type", rb.contentType)
	}
	return req, nil
}

// sendOnce performs one request attempt and reads the bounded response body.
// The returned retryAfter is the parsed Retry-After header (0 when absent).
func (c *Client) sendOnce(ctx context.Context, method, url string, rb requestBody) (status int, body []byte, retryAfter time.Duration, err error) {
	req, err := c.newRequest(ctx, method, url, rb)
	if err != nil {
		return 0, nil, 0, err
	}
	if err := c.limiter.Wait(ctx); err != nil {
		return 0, nil, 0, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, nil, 0, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err = io.ReadAll(io.LimitReader(resp.Body, maxResponseBody))
	if err != nil {
		return 0, nil, 0, fmt.Errorf("miro: read response: %w", err)
	}
	return resp.StatusCode, body, parseRetryAfter(resp.Header.Get("Retry-After")), nil
}

// executeWithRetry runs sendOnce with bounded retry on transient transport
// errors and retryable statuses (429/5xx), honoring Retry-After. Only requests
// with a replayable body are retried. Context cancellation is never retried.
func (c *Client) executeWithRetry(ctx context.Context, method, path, url string, rb requestBody) (int, []byte, error) {
	maxAttempts := 1
	if rb.replayable {
		maxAttempts = maxRetryAttempts
	}
	for attempt := 0; ; attempt++ {
		last := attempt+1 >= maxAttempts
		status, body, retryAfter, err := c.sendOnce(ctx, method, url, rb)
		if err != nil {
			if last || ctx.Err() != nil {
				return 0, nil, fmt.Errorf("miro: %s %s: %w", method, path, err)
			}
			if waitErr := retrySleep(ctx, attempt, 0); waitErr != nil {
				return 0, nil, waitErr
			}
			continue
		}
		if !last && isRetryableStatus(status) {
			if waitErr := retrySleep(ctx, attempt, retryAfter); waitErr != nil {
				return 0, nil, waitErr
			}
			continue
		}
		return status, body, nil
	}
}

// decodeCached unmarshals a cached body into out, matching the contract of
// the fresh-response path: nil out or empty body returns nil without error.
func decodeCached(cached []byte, out any) error {
	if out == nil || len(cached) == 0 {
		return nil
	}
	if err := json.Unmarshal(cached, out); err != nil {
		return fmt.Errorf("miro: decode cached response: %w", err)
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
