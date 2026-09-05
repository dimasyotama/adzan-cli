// Command adzand is the adzan background daemon. It is normally started by
// `adzan start`, by a systemd user unit, or by a launchd agent - not by hand.
package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/dimasyotama/adzan-cli/internal/config"
	"github.com/dimasyotama/adzan-cli/internal/daemon"
)

func main() {
	logger := log.New(os.Stderr, "", log.LstdFlags)

	cfg, err := config.Load()
	if errors.Is(err, config.ErrNotConfigured) {
		logger.Println("no configuration found; run `adzan setup` first")
		os.Exit(1)
	}
	if err != nil {
		logger.Printf("cannot read configuration: %v", err)
		os.Exit(1)
	}

	d, err := daemon.New(cfg, logger)
	if err != nil {
		logger.Printf("cannot start: %v", err)
		os.Exit(1)
	}

	writePID(logger)
	defer removePID()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)
	go func() {
		sig := <-sigs
		logger.Printf("received %s, shutting down", sig)
		d.Shutdown()
		cancel()
	}()

	if err := d.Run(ctx); err != nil {
		logger.Printf("daemon stopped: %v", err)
		os.Exit(1)
	}
	logger.Println("daemon stopped")
}

func writePID(logger *log.Logger) {
	path, err := config.PIDPath()
	if err != nil {
		return
	}
	if err := os.WriteFile(path, []byte(strconv.Itoa(os.Getpid())), 0o644); err != nil {
		logger.Printf("could not write pid file: %v", err)
	}
}

func removePID() {
	if path, err := config.PIDPath(); err == nil {
		_ = os.Remove(path)
	}
}
