// Command storyblok-go-website serves a marketing website from Storyblok.
//
// Configuration is read from the environment:
//
//	STORYBLOK_PREVIEW_TOKEN  preview access token of the space (required)
//	STORYBLOK_API_URL        Content Delivery API base URL (default: EU region)
//	ADDR                     listen address (default: :8080)
package main

import (
	"cmp"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"storyblok-go-website/internal/components"
	"storyblok-go-website/internal/server"
	"storyblok-go-website/internal/storyblok"
	"storyblok-go-website/static"
)

const shutdownTimeout = 10 * time.Second

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := run(logger); err != nil {
		logger.Error("server stopped", "err", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	previewToken := os.Getenv("STORYBLOK_PREVIEW_TOKEN")
	if previewToken == "" {
		return errors.New("STORYBLOK_PREVIEW_TOKEN is required")
	}
	apiURL := cmp.Or(os.Getenv("STORYBLOK_API_URL"), storyblok.DefaultBaseURL)
	addr := cmp.Or(os.Getenv("ADDR"), ":8080")

	renderer, err := components.NewRenderer()
	if err != nil {
		return err
	}
	srv := server.New(storyblok.NewClient(apiURL, previewToken), renderer, static.FS, previewToken, logger)
	httpServer := &http.Server{
		Addr:              addr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errs := make(chan error, 1)
	go func() {
		logger.Info("listening", "addr", addr)
		errs <- httpServer.ListenAndServe()
	}()

	select {
	case err := <-errs:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		return httpServer.Shutdown(shutdownCtx)
	}
}
