package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"miro-cli/internal/miro"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cmd, _ := newRootCmd()
	err := miro.RunWithRecover(os.Stderr, func() error {
		return cmd.ExecuteContext(ctx)
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "miro: "+err.Error())
	}
	os.Exit(miro.ExitCode(err))
}
