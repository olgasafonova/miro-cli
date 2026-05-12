// Package miro is the hand-authored foundation for miro-cli: HTTP client,
// auth/token handling, typed errors, and exit-code mapping for the
// miro.com REST API.
package miro

import (
	"errors"
	"fmt"
)

// Exit codes match the contract documented in README.md and SKILL.md.
// Match the existing CLI's exit codes so external tooling (Make targets,
// CI scripts, agent harnesses) doesn't break when binaries swap.
const (
	ExitOK          = 0
	ExitUsage       = 2
	ExitNotFound    = 3
	ExitAuth        = 4
	ExitAPI         = 5
	ExitRateLimited = 7
	ExitConfig      = 10
)

// APIError is the canonical error returned by Client.Do for any non-2xx
// response. Body is captured for diagnostics but is never included in
// Error()'s default rendering — keep tokens, query strings, and other
// sensitive content out of stderr by default. Callers that need the body
// can read it from the field directly.
type APIError struct {
	Method string
	Path   string
	Status int
	Body   string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("%s %s: HTTP %d", e.Method, e.Path, e.Status)
}

// ConfigError signals that the CLI cannot determine how to authenticate or
// where to talk to. Maps to ExitConfig.
type ConfigError struct {
	Reason string
}

func (e *ConfigError) Error() string {
	return "config: " + e.Reason
}

// ExitCode classifies an error into the README's exit-code contract.
// Unknown errors map to ExitAPI so agents and scripts can still react
// programmatically; only ExitOK means "succeeded."
func ExitCode(err error) int {
	if err == nil {
		return ExitOK
	}
	var cfg *ConfigError
	if errors.As(err, &cfg) {
		return ExitConfig
	}
	var api *APIError
	if errors.As(err, &api) {
		switch api.Status {
		case 401, 403:
			return ExitAuth
		case 404:
			return ExitNotFound
		case 429:
			return ExitRateLimited
		default:
			return ExitAPI
		}
	}
	return ExitAPI
}
