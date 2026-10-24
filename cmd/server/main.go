// Command server is the Trove web application. It wires configuration,
// storage, migrations and the HTTP server together and does nothing else.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"Trove/internal/app"
	"Trove/internal/config"
	"Trove/internal/store"
	"Trove/internal/web"
	"Trove/migrations"
)

// shutdownTimeout bounds graceful shutdown so a stuck request cannot block
// exit forever.
const shutdownTimeout = 10 * time.Second

func main() {
	if err := run(os.Args[1:], os.Getenv); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return // -h printed usage; not a failure.
		}
		fmt.Fprintf(os.Stderr, "%s: %v\n", app.Name, err)
		os.Exit(1)
	}
}

func run(args []string, getenv func(string) string) error {
	cfg, err := config.Load(args, getenv, os.Stderr)
	if err != nil {
		return err
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	if err := os.MkdirAll(cfg.DataDir, 0o700); err != nil {
		return fmt.Errorf("creating data directory %q: %w", cfg.DataDir, err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := store.Open(ctx, filepath.Join(cfg.DataDir, "repository.sqlite"))
	if err != nil {
		return err
	}
	defer func() {
		if err := db.Close(); err != nil {
			logger.Error("closing database", "error", err)
		}
	}()

	migrator := &store.Migrator{DB: db, FS: migrations.FS, Dir: "."}
	if err := migrator.Up(ctx); err != nil {
		return fmt.Errorf("applying migrations: %w", err)
	}

	srv := &http.Server{
		Addr:    cfg.ListenAddr,
		Handler: web.New(web.Options{Logger: logger}).Handler(),
		// Uploads stream with no size cap (spec N-1), so there is deliberately
		// no ReadTimeout or WriteTimeout. ReadHeaderTimeout still protects
		// against slow-header clients.
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}

	logger.Info("starting",
		"build", app.BuildInfo(),
		"listen", cfg.ListenAddr,
		"data_dir", cfg.DataDir,
		"secure_cookie", cfg.SecureCookie,
	)

	serveErr := make(chan error, 1)
	go func() {
		serveErr <- srv.ListenAndServe()
	}()

	select {
	case err := <-serveErr:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("serving HTTP on %s: %w", cfg.ListenAddr, err)
	case <-ctx.Done():
	}

	logger.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutting down HTTP server: %w", err)
	}
	if err := <-serveErr; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serving HTTP on %s: %w", cfg.ListenAddr, err)
	}
	return nil
}
