package miro

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestAPIErrorDoesNotLeakBody(t *testing.T) {
	// The Body field captures the response for diagnostics, but stringer
	// output is the channel that lands in stderr / logs / CI output; that
	// channel must never include the body. Tokens, query strings, customer
	// data — none of it belongs in default error rendering.
	apiErr := &APIError{
		Method: "POST",
		Path:   "/v2/boards/abc/items",
		Status: 401,
		Body:   `{"error": "unauthorized", "token_hint": "sk-secret-leaked"}`,
	}
	msg := apiErr.Error()
	if strings.Contains(msg, "sk-secret-leaked") {
		t.Fatalf("APIError.Error() leaked Body: %q", msg)
	}
	if !strings.Contains(msg, "POST") || !strings.Contains(msg, "/v2/boards/abc/items") || !strings.Contains(msg, "401") {
		t.Fatalf("APIError.Error() missing expected fields: %q", msg)
	}
}

func TestConfigErrorMessage(t *testing.T) {
	err := &ConfigError{Reason: "no token"}
	if got := err.Error(); got != "config: no token" {
		t.Errorf("ConfigError.Error() = %q, want %q", got, "config: no token")
	}
}

func TestExitCode(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{"nil", nil, ExitOK},
		{"config", &ConfigError{Reason: "x"}, ExitConfig},
		{"api 401", &APIError{Status: 401}, ExitAuth},
		{"api 403", &APIError{Status: 403}, ExitAuth},
		{"api 404", &APIError{Status: 404}, ExitNotFound},
		{"api 429", &APIError{Status: 429}, ExitRateLimited},
		{"api 500", &APIError{Status: 500}, ExitAPI},
		{"api 400", &APIError{Status: 400}, ExitAPI},
		{"wrapped api 404", fmt.Errorf("call site: %w", &APIError{Status: 404}), ExitNotFound},
		{"wrapped config", fmt.Errorf("startup: %w", &ConfigError{}), ExitConfig},
		{"unknown error", errors.New("network"), ExitAPI},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExitCode(tt.err); got != tt.want {
				t.Errorf("ExitCode(%v) = %d, want %d", tt.err, got, tt.want)
			}
		})
	}
}
