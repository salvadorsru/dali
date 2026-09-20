package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/salvadorsru/dali/embed"
)

// cmdServe starts the managed embedding server in the foreground and keeps it
// alive until interrupted, so subsequent commands reuse it.
func cmdServe(args []string) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	m, ok := embed.FromEnv().(*embed.Managed)
	if !ok {
		return errors.New("serve needs the managed llama provider (unset DALI_LLAMA_URL and DALI_EMBED_URL)")
	}
	return m.Serve(ctx)
}

// cmdStop stops a managed server left running by a previous command.
func cmdStop(args []string) error {
	if err := embed.StopServer(os.Getenv("DALI_CACHE_DIR")); err != nil {
		return err
	}
	fmt.Println("dali: server stopped")
	return nil
}
