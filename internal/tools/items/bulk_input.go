package items

import (
	"fmt"

	"github.com/olgasafonova/miro-cli/internal/tools/clictx"
)

// requireExactlyOneSource enforces the file-or-inline contract shared by
// the bulk commands: exactly one of --<fileFlag> or --<inlineFlag> must
// be set. Flag names are passed without the leading dashes.
func requireExactlyOneSource(fileFlag, inlineFlag, fileVal, inlineVal string) error {
	if fileVal == "" && inlineVal == "" {
		return fmt.Errorf("one of --%s or --%s is required", fileFlag, inlineFlag)
	}
	if fileVal != "" && inlineVal != "" {
		return fmt.Errorf("--%s and --%s are mutually exclusive", fileFlag, inlineFlag)
	}
	return nil
}

// readRawJSONSource returns the raw payload bytes from the file flag
// (a path, or - for stdin) when set, otherwise from the inline value.
// Callers enforce the exactly-one contract before calling this.
func readRawJSONSource(fileFlag, fileVal, inlineVal string) ([]byte, error) {
	if fileVal == "" {
		return []byte(inlineVal), nil
	}
	raw, err := clictx.ReadFileOrStdin(fileVal)
	if err != nil {
		return nil, fmt.Errorf("read --%s: %w", fileFlag, err)
	}
	return raw, nil
}
