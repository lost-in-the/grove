package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/lost-in-the/grove/cmd/grove/commands"
	"github.com/lost-in-the/grove/internal/log"
)

func main() {
	log.Init()
	defer log.Close()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if code := commands.Run(ctx); code != 0 {
		os.Exit(code)
	}
}
