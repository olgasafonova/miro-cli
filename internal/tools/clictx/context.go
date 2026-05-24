// Package clictx is the shared CLI context for the hand-authored miro
// command tree. Globals carries persistent-flag values, output writers,
// and an optional injected *miro.Client; subcommand packages depend on
// it but not on cmd/miro/, keeping the dependency arrow pointing inward.
package clictx

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/olgasafonova/miro-cli/internal/miro"
)

// Globals captures every persistent flag exposed by the root command,
// plus the output writers and an optional pre-built client used by tests
// to point the CLI at httptest servers.
//
// Globals is constructed once in cmd/miro/root.go and threaded through
// to subcommands via constructor parameters. Subcommands read flag values
// inside their RunE — Cobra has populated them by then.
type Globals struct {
	Token      string
	JSON       bool
	DryRun     bool
	Agent      bool
	Yes        bool
	Idempotent bool
	Select     string

	// RateLimit is the requests-per-second budget passed to the client's
	// token bucket. Negative means "use the package default"; 0 means
	// "no rate limiting"; positive sets an explicit rate. The root flag
	// initialises this to -1 so users opting out write --rate-limit=0.
	RateLimit float64

	// CacheTTL is the freshness window for the client's GET response
	// cache. Negative means "use the package default"; 0 means "no cache"
	// (equivalent to --no-cache). The root flag initialises this to -1.
	CacheTTL time.Duration

	// NoCache short-circuits cache construction regardless of CacheTTL.
	// Users typically set this for one-shot CLI invocations where the
	// cache can't help anyway and they want certainty about freshness.
	NoCache bool

	// StorePath overrides the default on-disk location of the local
	// SQLite store. Empty means "use store.DefaultPath()". The sync and
	// query commands honour this; commands that don't touch the store
	// ignore it.
	StorePath string

	Stdout io.Writer
	Stderr io.Writer

	// Client, if non-nil, is used instead of BuildClient constructing
	// a fresh one. Tests set this; the root command does not.
	Client *miro.Client
}

// New returns a Globals with sensible defaults wired to os.Stdout/Stderr.
// RateLimit and CacheTTL start at -1 so BuildClient applies the package
// defaults; the --rate-limit and --cache-ttl flags override these from
// cmd/miro-cli/root.go.
func New() *Globals {
	return &Globals{
		Stdout:    os.Stdout,
		Stderr:    os.Stderr,
		RateLimit: -1,
		CacheTTL:  -1,
	}
}

// Normalize expands shortcut flags into their components. --agent means
// "make this safe to call from a non-interactive agent": output JSON,
// don't pause for destructive-op confirmation. The expansion is one-way
// (we never un-set JSON if --agent was passed) so callers can layer
// flags freely.
func (g *Globals) Normalize() {
	if g.Agent {
		g.JSON = true
		g.Yes = true
	}
}

// BuildClient returns the injected *miro.Client if one is set, otherwise
// loads the token (flag or env) and constructs a fresh client pointed at
// DefaultBaseURL with a token-bucket rate limiter and an optional GET
// response cache.
//
// Rate-limit resolution:
//   - g.RateLimit > 0: use that exact rate
//   - g.RateLimit == 0: no limiting (the user opted out via --rate-limit=0)
//   - g.RateLimit < 0: use miro.DefaultRateLimit (the sentinel from New())
//
// Cache resolution:
//   - g.NoCache: no cache (overrides CacheTTL)
//   - g.CacheTTL > 0: use that TTL with miro.DefaultCacheEntries
//   - g.CacheTTL == 0: no cache
//   - g.CacheTTL < 0: use miro.DefaultCacheTTL
func (g *Globals) BuildClient() (*miro.Client, error) {
	if g.Client != nil {
		return g.Client, nil
	}
	cfg, err := miro.LoadConfig(g.Token)
	if err != nil {
		return nil, err
	}
	rate := g.RateLimit
	if rate < 0 {
		rate = miro.DefaultRateLimit
	}
	ttl := g.CacheTTL
	if ttl < 0 {
		ttl = miro.DefaultCacheTTL
	}
	if g.NoCache {
		ttl = 0
	}
	return miro.New(cfg,
		miro.WithRateLimit(miro.NewLimiter(rate, miro.DefaultRateBurst)),
		miro.WithCache(miro.NewCache(miro.DefaultCacheEntries, ttl)),
	), nil
}

// EmitJSON marshals v to JSON, applies --select if set, and writes to
// Stdout with a trailing newline. The output is indented for human
// readers; agents pipe through jq anyway and indentation doesn't change
// behavior. Single-line output is a Phase 4 consideration.
//
// --select takes a comma-separated list of top-level field names. Phase 2
// keeps the filter shallow on purpose: complex jq-style selectors are
// what `| jq` is for. Phase 3 may extend this to descend into a `data`
// array if a real need emerges.
func (g *Globals) EmitJSON(v any) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("miro: marshal output: %w", err)
	}
	if g.Select != "" {
		filtered, ferr := applySelect(raw, g.Select)
		if ferr != nil {
			return ferr
		}
		raw = filtered
	}
	var indented []byte
	indented, err = indentJSON(raw)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(g.Stdout, string(indented))
	return err
}

// EmitDryRun writes a single "DRY-RUN METHOD PATH" line to Stdout
// without making any API call. Used by subcommands to preview the
// request a real invocation would send.
func (g *Globals) EmitDryRun(method, path string) error {
	_, err := fmt.Fprintf(g.Stdout, "DRY-RUN %s %s\n", method, path)
	return err
}

func indentJSON(raw []byte) ([]byte, error) {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, fmt.Errorf("miro: reformat output: %w", err)
	}
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("miro: reformat output: %w", err)
	}
	return out, nil
}

// applySelect returns a copy of raw with only the requested top-level
// fields retained. If raw is an array, the filter is applied to each
// element. Unknown fields are silently dropped; empty result is "{}".
func applySelect(raw []byte, fields string) ([]byte, error) {
	wanted := parseFieldList(fields)
	if len(wanted) == 0 {
		return raw, nil
	}

	// Detect array vs object by first non-whitespace byte.
	trimmed := skipLeadingSpace(raw)
	if len(trimmed) == 0 {
		return raw, nil
	}
	if trimmed[0] == '[' {
		var arr []json.RawMessage
		if err := json.Unmarshal(raw, &arr); err != nil {
			return nil, fmt.Errorf("miro: apply --select to array: %w", err)
		}
		out := make([]json.RawMessage, 0, len(arr))
		for _, elem := range arr {
			filtered, err := filterObject(elem, wanted)
			if err != nil {
				return nil, err
			}
			out = append(out, filtered)
		}
		return json.Marshal(out)
	}
	return filterObject(raw, wanted)
}

func filterObject(raw []byte, wanted map[string]struct{}) ([]byte, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("miro: apply --select to object: %w", err)
	}
	out := make(map[string]json.RawMessage, len(wanted))
	for k := range wanted {
		if v, ok := m[k]; ok {
			out[k] = v
		}
	}
	return json.Marshal(out)
}

func parseFieldList(s string) map[string]struct{} {
	out := make(map[string]struct{})
	for _, f := range strings.Split(s, ",") {
		f = strings.TrimSpace(f)
		if f != "" {
			out[f] = struct{}{}
		}
	}
	return out
}

func skipLeadingSpace(b []byte) []byte {
	for i, c := range b {
		switch c {
		case ' ', '\t', '\n', '\r':
			continue
		default:
			return b[i:]
		}
	}
	return nil
}
