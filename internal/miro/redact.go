package miro

import (
	"fmt"
	"strings"
)

// redactedPlaceholder is what RedactToken substitutes in for the secret.
const redactedPlaceholder = "<redacted>"

// RedactToken returns s with every occurrence of token replaced by
// "<redacted>". Empty token is a no-op so callers don't have to special-case
// the unauthenticated path. The function is defensive: anything that builds
// a user-facing string from a Config, a request, or a logger should pass it
// through RedactToken before writing.
func RedactToken(s, token string) string {
	if token == "" {
		return s
	}
	return strings.ReplaceAll(s, token, redactedPlaceholder)
}

// String returns a human-readable description of the Config with the token
// replaced by "<redacted>". Stops the obvious foot-gun: fmt.Println(cfg) or
// a logger printing the value never leaks the bearer.
func (c *Config) String() string {
	if c == nil {
		return "<nil Config>"
	}
	return fmt.Sprintf("Config{Token: %s, BaseURL: %q}", redactedPlaceholder, c.BaseURL)
}

// String returns a human-readable description of the Client without
// revealing the token. Mirrors Config.String.
func (c *Client) String() string {
	if c == nil {
		return "<nil Client>"
	}
	return fmt.Sprintf("Client{BaseURL: %q, Token: %s}", c.baseURL, redactedPlaceholder)
}
