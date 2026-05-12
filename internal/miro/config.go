package miro

import (
	"os"
	"strings"
)

// DefaultBaseURL is the production Miro REST API endpoint.
const DefaultBaseURL = "https://api.miro.com"

// EnvAccessToken is the environment variable consulted for the bearer
// token. Matches the variable name documented in the existing CLI's README
// and SKILL.md so users don't have to reconfigure when the binary swaps.
const EnvAccessToken = "MIRO_ACCESS_TOKEN"

// Config captures everything Client.New needs to make authenticated
// requests. Tokens travel by value inside this struct; callers should not
// log or serialize a Config. Config does not embed any io.Writer / logger
// — redaction is the caller's responsibility at the I/O boundary.
type Config struct {
	Token   string
	BaseURL string
}

// LoadConfig resolves credentials in precedence order:
//
//  1. tokenFlag (the value of --token if explicitly set on the CLI)
//  2. $MIRO_ACCESS_TOKEN
//
// A config file is intentionally out of scope for the foundation; add it
// in a later phase if a real use case appears. CLI tokens are usually
// short-lived and machine-passed, which env handles cleanly.
//
// Returns *ConfigError if no token can be found. Maps to ExitConfig.
func LoadConfig(tokenFlag string) (*Config, error) {
	token := strings.TrimSpace(tokenFlag)
	if token == "" {
		token = strings.TrimSpace(os.Getenv(EnvAccessToken))
	}
	if token == "" {
		return nil, &ConfigError{Reason: "no token: pass --token or set " + EnvAccessToken}
	}
	return &Config{
		Token:   token,
		BaseURL: DefaultBaseURL,
	}, nil
}
