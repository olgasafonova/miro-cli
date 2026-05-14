package miro

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// newTestClient wires a Client to a httptest server with a 1-second
// timeout (overrides the 30s default so cancellation tests don't hang).
func newTestClient(t *testing.T, handler http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c := New(
		&Config{Token: "tok-test", BaseURL: srv.URL},
		WithHTTPClient(&http.Client{Timeout: time.Second}),
	)
	return c, srv
}

func TestClientSendsBearerToken(t *testing.T) {
	var gotAuth string
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(200)
	})
	if err := c.Get(context.Background(), "/v2/boards", nil); err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer tok-test" {
		t.Errorf("Authorization = %q, want %q", gotAuth, "Bearer tok-test")
	}
}

func TestClientSendsJSONBody(t *testing.T) {
	var gotBody map[string]string
	var gotCT string
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotCT = r.Header.Get("Content-Type")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(200)
	})
	if err := c.Post(context.Background(), "/v2/boards", map[string]string{"name": "hello"}, nil); err != nil {
		t.Fatal(err)
	}
	if gotCT != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", gotCT)
	}
	if gotBody["name"] != "hello" {
		t.Errorf("body = %v, want name=hello", gotBody)
	}
}

func TestClientDecodesResponse(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"board-1","name":"Demo"}`)
	})
	var out struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := c.Get(context.Background(), "/v2/boards/board-1", &out); err != nil {
		t.Fatal(err)
	}
	if out.ID != "board-1" || out.Name != "Demo" {
		t.Errorf("decoded = %+v", out)
	}
}

func TestClientReturnsAPIErrorOnNon2xx(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		wantExit   int
		wantMethod string
		wantPath   string
	}{
		{"not found", 404, ExitNotFound, "GET", "/v2/boards/missing"},
		{"unauthorized", 401, ExitAuth, "GET", "/v2/boards/missing"},
		{"rate limited", 429, ExitRateLimited, "GET", "/v2/boards/missing"},
		{"server error", 500, ExitAPI, "GET", "/v2/boards/missing"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = io.WriteString(w, `{"error":"reason"}`)
			})
			err := c.Get(context.Background(), tt.wantPath, nil)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			var api *APIError
			if !errors.As(err, &api) {
				t.Fatalf("err type = %T, want *APIError", err)
			}
			if api.Status != tt.status {
				t.Errorf("Status = %d, want %d", api.Status, tt.status)
			}
			if api.Method != tt.wantMethod || api.Path != tt.wantPath {
				t.Errorf("APIError = {%s %s}, want {%s %s}", api.Method, api.Path, tt.wantMethod, tt.wantPath)
			}
			if got := ExitCode(err); got != tt.wantExit {
				t.Errorf("ExitCode = %d, want %d", got, tt.wantExit)
			}
		})
	}
}

func TestClientDoesNotPutTokenInErrors(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		_, _ = io.WriteString(w, `boom`)
	})
	err := c.Get(context.Background(), "/v2/boards", nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), "tok-test") {
		t.Errorf("error leaked token: %q", err.Error())
	}
}

func TestClientHonorsContextCancellation(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(2 * time.Second):
		}
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := c.Get(ctx, "/v2/boards", nil)
	if err == nil {
		t.Fatal("expected context cancellation error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}

func TestClientCapsResponseBody(t *testing.T) {
	// A malicious or buggy upstream that streams gigabytes back must not
	// blow up the CLI's memory. The cap is 10MB; this test just verifies
	// the read terminates rather than hanging or OOMing.
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		// 1MB of zeros — fits under the cap, just confirming read terminates.
		_, _ = w.Write(make([]byte, 1024*1024))
	})
	if err := c.Get(context.Background(), "/v2/boards", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClientDeleteSendsNoBody(t *testing.T) {
	var gotMethod string
	var gotCT string
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotCT = r.Header.Get("Content-Type")
		w.WriteHeader(204)
	})
	if err := c.Delete(context.Background(), "/v2/boards/x"); err != nil {
		t.Fatal(err)
	}
	if gotMethod != "DELETE" {
		t.Errorf("Method = %q", gotMethod)
	}
	if gotCT != "" {
		t.Errorf("Content-Type = %q, want empty", gotCT)
	}
}

func TestClientNilReceiver(t *testing.T) {
	var c *Client
	if err := c.Get(context.Background(), "/", nil); err == nil {
		t.Error("expected error on nil receiver")
	}
}

func TestClientCachesGet(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"b1","name":"first"}`)
	}))
	t.Cleanup(srv.Close)

	c := New(
		&Config{Token: "t", BaseURL: srv.URL},
		WithHTTPClient(&http.Client{Timeout: time.Second}),
		WithCache(NewCache(8, time.Minute)),
	)

	var first, second struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := c.Get(context.Background(), "/v2/boards/b1", &first); err != nil {
		t.Fatalf("first Get: %v", err)
	}
	if err := c.Get(context.Background(), "/v2/boards/b1", &second); err != nil {
		t.Fatalf("second Get: %v", err)
	}
	if hits != 1 {
		t.Errorf("upstream hits = %d, want 1 (second call must serve from cache)", hits)
	}
	if first != second {
		t.Errorf("cached decode mismatch: first=%+v second=%+v", first, second)
	}
}

func TestClientCacheKeysOnPath(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"path":"`+r.URL.Path+`"}`)
	}))
	t.Cleanup(srv.Close)

	c := New(
		&Config{Token: "t", BaseURL: srv.URL},
		WithHTTPClient(&http.Client{Timeout: time.Second}),
		WithCache(NewCache(8, time.Minute)),
	)

	for _, p := range []string{"/v2/boards/a", "/v2/boards/b", "/v2/boards/a"} {
		if err := c.Get(context.Background(), p, nil); err != nil {
			t.Fatalf("Get %s: %v", p, err)
		}
	}
	if hits != 2 {
		t.Errorf("upstream hits = %d, want 2 (distinct paths miss, repeat hits)", hits)
	}
}

func TestClientDoesNotCachePostOrError(t *testing.T) {
	var hits int
	nextStatus := 200
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(nextStatus)
		_, _ = io.WriteString(w, `{"ok":true}`)
	}))
	t.Cleanup(srv.Close)

	c := New(
		&Config{Token: "t", BaseURL: srv.URL},
		WithHTTPClient(&http.Client{Timeout: time.Second}),
		WithCache(NewCache(8, time.Minute)),
	)

	// POST twice: cache must not kick in for mutating methods.
	if err := c.Post(context.Background(), "/v2/items", map[string]string{"x": "y"}, nil); err != nil {
		t.Fatal(err)
	}
	if err := c.Post(context.Background(), "/v2/items", map[string]string{"x": "y"}, nil); err != nil {
		t.Fatal(err)
	}
	if hits != 2 {
		t.Errorf("POST cached: hits = %d, want 2", hits)
	}

	// GET that errors must not be cached.
	nextStatus = 500
	if err := c.Get(context.Background(), "/v2/boards/err", nil); err == nil {
		t.Fatal("expected 500")
	}
	if err := c.Get(context.Background(), "/v2/boards/err", nil); err == nil {
		t.Fatal("expected 500 on retry")
	}
	if hits != 4 {
		t.Errorf("error response cached: hits = %d, want 4", hits)
	}
}

func TestClientCacheDisabledWhenNil(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(200)
		_, _ = io.WriteString(w, `{}`)
	}))
	t.Cleanup(srv.Close)

	// No WithCache: every call hits upstream.
	c := New(
		&Config{Token: "t", BaseURL: srv.URL},
		WithHTTPClient(&http.Client{Timeout: time.Second}),
	)
	for i := 0; i < 3; i++ {
		if err := c.Get(context.Background(), "/v2/boards/x", nil); err != nil {
			t.Fatal(err)
		}
	}
	if hits != 3 {
		t.Errorf("nil cache cached responses: hits = %d, want 3", hits)
	}
}
