package miro

import (
	"fmt"
	"strings"
	"unicode"
)

// MaxIDLength caps user-supplied resource IDs. Miro IDs in practice are
// short (board IDs ~14 chars, item IDs are decimal integers up to ~24
// digits). 256 leaves headroom for future ID schemes without giving an
// attacker room to craft an absurdly long path.
const MaxIDLength = 256

// ValidateID rejects user-supplied resource identifiers that would be unsafe
// to splice into a URL path. The check is paranoid on purpose: every
// rejected character is one that would either (a) change the URL's path
// structure (slashes), (b) hide content in a transcript (control chars,
// whitespace), or (c) trigger path traversal on a server that re-resolves
// the path. The function never inspects the value's semantic format; the
// Miro API will reject IDs of the wrong shape on its own.
//
// name is included in the error so callers don't have to wrap the result.
func ValidateID(name, val string) error {
	if val == "" {
		return fmt.Errorf("%s is required", name)
	}
	if len(val) > MaxIDLength {
		return fmt.Errorf("%s exceeds %d characters", name, MaxIDLength)
	}
	for _, r := range val {
		if r == '/' {
			return fmt.Errorf("%s must not contain '/'", name)
		}
		if r == '\x00' {
			return fmt.Errorf("%s must not contain null bytes", name)
		}
		if unicode.IsControl(r) {
			return fmt.Errorf("%s must not contain control characters", name)
		}
		if unicode.IsSpace(r) {
			return fmt.Errorf("%s must not contain whitespace", name)
		}
	}
	if strings.Contains(val, "..") {
		return fmt.Errorf("%s must not contain '..'", name)
	}
	return nil
}

// validatePath is a defense-in-depth check applied inside Client.Do before
// it composes the final URL. The command layer is supposed to validate
// every ID via ValidateID; this catches anyone who forgot. It is
// intentionally narrower than ValidateID because the path comes from
// trusted code: only reject path-traversal segments and null bytes.
func validatePath(path string) error {
	if strings.ContainsRune(path, '\x00') {
		return fmt.Errorf("miro: path contains null byte")
	}
	// "/..", "../" or "/../" anywhere in the path means a caller spliced
	// an unvalidated ID into the URL. Reject before sending so we don't
	// give the Miro API or any intermediate proxy a chance to resolve it.
	for _, seg := range strings.Split(path, "/") {
		if seg == ".." {
			return fmt.Errorf("miro: path contains '..' segment")
		}
	}
	return nil
}
