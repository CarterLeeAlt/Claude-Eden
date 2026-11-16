package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/sylvinus/agent-vm/internal/installfiles"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := installfiles.CLI(ctx, os.Args[1:], os.Stdout, os.Stderr)
	cancel()
	os.Exit(code)
}
