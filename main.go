// Command storyblok-go-website serves a marketing website from Storyblok.
//
// Configuration is read from the environment:
//
//	STORYBLOK_PREVIEW_TOKEN  preview access token of the space (required)
//	SITE_URL                 public origin for canonical URLs and the sitemap,
//	                         e.g. https://www.example.com (required)
//	STORYBLOK_API_URL        Content Delivery API base URL (default: EU region)
//	ADDR                     listen address (default: :8080)
//	TLS_CERT_FILE            certificate for serving HTTPS, which the Visual
//	TLS_KEY_FILE             Editor requires for preview URLs (optional)
//	DEV_TOOLBAR              "1" adds a toolbar to open blocks in the editor
//	METRICS_ADDR             listen address for Prometheus metrics, never
//	                         proxied (default: :9090)
package main

import (
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"storyblok-go-website/internal/components"
	"storyblok-go-website/internal/metrics"
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
	siteURL, err := parseSiteURL(os.Getenv("SITE_URL"))
	if err != nil {
		return err
	}
	apiURL := cmp.Or(os.Getenv("STORYBLOK_API_URL"), storyblok.DefaultBaseURL)
	addr := cmp.Or(os.Getenv("ADDR"), ":8080")
	certFile, keyFile := os.Getenv("TLS_CERT_FILE"), os.Getenv("TLS_KEY_FILE")

	client := storyblok.NewClient(apiURL, previewToken)
	buildID, err := executableHash()
	// Canonical URLs change page markup without changing the build.
	siteHash := sha256.Sum256([]byte(siteURL))
	buildID += "-" + hex.EncodeToString(siteHash[:4])
	if err != nil {
		return err
	}
	var rendererOpts []components.RendererOption
	if os.Getenv("DEV_TOOLBAR") == "1" {
		// The toolbar changes page markup without changing the build.
		buildID += "-dev"
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		spaceID, err := client.SpaceID(ctx)
		cancel()
		if err != nil {
			return err
		}
		rendererOpts = append(rendererOpts, components.WithDevToolbar(spaceID))
	}
	renderer, err := components.NewRenderer(rendererOpts...)
	if err != nil {
		return err
	}
	m := metrics.New()
	srv := server.New(m.Content(client), renderer, static.FS, server.LogInbox{Logger: logger}, previewToken, logger,
		server.WithBuildID(buildID), server.WithSiteURL(siteURL), server.WithVitals(m.RecordVital))
	httpServer := &http.Server{
		Addr:              addr,
		Handler:           m.Middleware(srv.Handler()),
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errs := make(chan error, 2)
	metricsServer := &http.Server{
		Addr:              cmp.Or(os.Getenv("METRICS_ADDR"), ":9090"),
		Handler:           m.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		if err := metricsServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errs <- err
		}
	}()
	go func() {
		if certFile != "" {
			logger.Info("listening", "url", "https://localhost"+addr)
			errs <- httpServer.ListenAndServeTLS(certFile, keyFile)
			return
		}
		logger.Info("listening", "url", "http://localhost"+addr)
		errs <- httpServer.ListenAndServe()
	}()

	select {
	case err := <-errs:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		_ = metricsServer.Shutdown(shutdownCtx)
		return httpServer.Shutdown(shutdownCtx)
	}
}

// parseSiteURL validates the public origin: an absolute HTTP(S) URL without
// path, query, or fragment.
func parseSiteURL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if raw == "" || err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" ||
		strings.Trim(u.Path, "/") != "" || u.RawQuery != "" || u.Fragment != "" {
		return "", fmt.Errorf("SITE_URL must be an origin such as https://www.example.com, got %q", raw)
	}
	return u.Scheme + "://" + u.Host, nil
}

// executableHash identifies the build, which embeds all templates and assets.
func executableHash() (string, error) {
	path, err := os.Executable()
	if err != nil {
		return "", err
	}
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)[:8]), nil
}
