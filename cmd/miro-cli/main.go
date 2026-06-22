package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/olgasafonova/miro-cli/internal/miro"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cmd, g := newRootCmd()
	err := miro.RunWithRecover(os.Stderr, func() error {
		return cmd.ExecuteContext(ctx)
	})
	if err != nil {
		// Redact the token before printing: an API error string or a
		// wrapped transport error can echo back a URL or header that
		// embeds the bearer token. Cover both the --token flag value and
		// the environment fallback.
		msg := err.Error()
		msg = miro.RedactToken(msg, strings.TrimSpace(g.Token))
		msg = miro.RedactToken(msg, strings.TrimSpace(os.Getenv(miro.EnvAccessToken)))
		fmt.Fprintln(os.Stderr, "miro: "+msg)
	}
	os.Exit(miro.ExitCode(err))
}
