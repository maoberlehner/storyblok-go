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
//	FORM_SECRET              signs form tokens; all instances need the same
//	                         value (required)
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
	"net"
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

const (
	// readHeaderTimeout and readTimeout bound how long a client may take to
	// send a request, so slow clients cannot hold connections.
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 30 * time.Second
	// writeTimeout bounds handling and writing a response. It exceeds the
	// Storyblok client's 10-second operation timeout, so a stalled API call
	// ends in an error page instead of a dropped connection.
	writeTimeout = 30 * time.Second
	// idleTimeout closes keep-alive connections without requests.
	idleTimeout = 2 * time.Minute
	// shutdownTimeout bounds waiting for running requests on shutdown.
	shutdownTimeout = 10 * time.Second
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	// A second signal ends the process without waiting for the shutdown.
	context.AfterFunc(ctx, stop)
	err := run(ctx, logger)
	stop()
	if err != nil {
		logger.Error("exiting", "err", err)
		os.Exit(1)
	}
}

// run serves the site until ctx ends, then shuts down.
func run(ctx context.Context, logger *slog.Logger) error {
	previewToken := os.Getenv("STORYBLOK_PREVIEW_TOKEN")
	if previewToken == "" {
		return errors.New("STORYBLOK_PREVIEW_TOKEN is required")
	}
	formSecret := os.Getenv("FORM_SECRET")
	if formSecret == "" {
		return errors.New("FORM_SECRET is required")
	}
	siteURL, err := parseSiteURL(os.Getenv("SITE_URL"))
	if err != nil {
		return err
	}
	apiURL := cmp.Or(os.Getenv("STORYBLOK_API_URL"), storyblok.DefaultBaseURL)
	addr := cmp.Or(os.Getenv("ADDR"), ":8080")
	metricsAddr := cmp.Or(os.Getenv("METRICS_ADDR"), ":9090")
	certFile, keyFile := os.Getenv("TLS_CERT_FILE"), os.Getenv("TLS_KEY_FILE")

	client := storyblok.NewClient(apiURL, previewToken)
	buildID, err := executableHash()
	if err != nil {
		return err
	}
	// Canonical URLs change page markup without changing the build.
	siteHash := sha256.Sum256([]byte(siteURL))
	buildID += "-" + hex.EncodeToString(siteHash[:4])
	var rendererOpts []components.RendererOption
	if os.Getenv("DEV_TOOLBAR") == "1" {
		// The toolbar changes page markup without changing the build.
		buildID += "-dev"
		spaceCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		spaceID, err := client.SpaceID(spaceCtx)
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
		server.WithBuildID(buildID), server.WithSiteURL(siteURL), server.WithVitals(m.RecordVital), server.WithFormSecret([]byte(formSecret)))
	httpServer := newHTTPServer(m.Middleware(srv.Handler()))
	metricsServer := newHTTPServer(m.Handler())

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	metricsLn, err := net.Listen("tcp", metricsAddr)
	if err != nil {
		ln.Close()
		return err
	}
	scheme := "http"
	if certFile != "" {
		scheme = "https"
	}
	logger.Info("listening", "addr", ln.Addr().String(), "url", localURL(scheme, ln.Addr()), "metrics", metricsLn.Addr().String())

	errs := make(chan error, 2)
	go func() {
		if err := metricsServer.Serve(metricsLn); !errors.Is(err, http.ErrServerClosed) {
			errs <- err
		}
	}()
	go func() {
		var err error
		if certFile != "" {
			err = httpServer.ServeTLS(ln, certFile, keyFile)
		} else {
			err = httpServer.Serve(ln)
		}
		if !errors.Is(err, http.ErrServerClosed) {
			errs <- err
		}
	}()

	select {
	case err := <-errs:
		_ = metricsServer.Close()
		_ = httpServer.Close()
		return err
	case <-ctx.Done():
		logger.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		_ = metricsServer.Shutdown(shutdownCtx)
		return httpServer.Shutdown(shutdownCtx)
	}
}

func newHTTPServer(handler http.Handler) *http.Server {
	return &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}
}

// localURL is the address to open in a browser on this machine.
func localURL(scheme string, addr net.Addr) string {
	if tcp, ok := addr.(*net.TCPAddr); ok {
		return fmt.Sprintf("%s://localhost:%d", scheme, tcp.Port)
	}
	return scheme + "://" + addr.String()
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
