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

	"golang.org/x/term"

	"Trove/internal/app"
	"Trove/internal/auth"
	"Trove/internal/blob"
	"Trove/internal/config"
	"Trove/internal/service"
	"Trove/internal/store"
	"Trove/internal/web"
	"Trove/migrations"
)

// shutdownTimeout bounds graceful shutdown so a stuck request cannot block
// exit forever.
const shutdownTimeout = 10 * time.Second

func main() {
	args := os.Args[1:]
	if len(args) > 0 && args[0] == "reset-password" {
		if err := runResetPassword(args[1:], os.Getenv); err != nil {
			if !errors.Is(err, flag.ErrHelp) {
				fmt.Fprintf(os.Stderr, "%s: %v\n", app.Name, err)
				os.Exit(1)
			}
		}
		return
	}
	if err := run(args, os.Getenv); err != nil {
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

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	st, err := openStore(ctx, cfg)
	if err != nil {
		return err
	}
	defer func() {
		if err := st.Close(); err != nil {
			logger.Error("closing database", "error", err)
		}
	}()

	blobs, err := blob.New(filepath.Join(cfg.DataDir, "files"))
	if err != nil {
		return err
	}
	itemService := service.NewItemService(st, blobs, time.Now)

	// Best-effort cleanup of content files whose attachment row is gone. It
	// never touches anything outside the content directory (X-12).
	if removed, err := itemService.CleanupOrphans(ctx); err != nil {
		logger.Warn("orphan cleanup failed", "error", err)
	} else if removed > 0 {
		logger.Info("removed orphaned content", "files", removed)
	}

	srv := &http.Server{
		Addr: cfg.ListenAddr,
		Handler: web.New(web.Options{
			Logger:       logger,
			Auth:         newAuthService(st),
			Items:        itemService,
			SecureCookie: cfg.SecureCookie,
		}).Handler(),
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

// runResetPassword is the only supported password recovery mechanism (U-10).
// It works locally, never over HTTP, and changes no repository data.
func runResetPassword(args []string, getenv func(string) string) error {
	cfg, err := config.Load(args, getenv, os.Stderr)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	st, err := openStore(ctx, cfg)
	if err != nil {
		return err
	}
	defer func() {
		if err := st.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "%s: closing database: %v\n", app.Name, err)
		}
	}()

	svc := newAuthService(st)
	configured, err := svc.Configured(ctx)
	if err != nil {
		return err
	}
	if !configured {
		return errors.New("no password is set up yet; open the web interface and use first-run setup")
	}

	if err := performReset(ctx, svc, promptPassword); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "Password updated. All sessions were signed out.\n")
	return nil
}

// performReset is separated from terminal handling so it can be tested.
func performReset(ctx context.Context, svc *auth.Service, prompt func(string) (string, error)) error {
	password, err := prompt("New password: ")
	if err != nil {
		return err
	}
	confirm, err := prompt("Confirm new password: ")
	if err != nil {
		return err
	}
	return svc.ResetPassword(ctx, password, confirm)
}

// promptPassword reads a password from the terminal without echoing it.
func promptPassword(prompt string) (string, error) {
	fmt.Fprint(os.Stderr, prompt)
	b, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", fmt.Errorf("reading password: %w", err)
	}
	return string(b), nil
}

// openStore creates the data directory, opens the database and migrates it.
func openStore(ctx context.Context, cfg config.Config) (*store.Store, error) {
	if err := os.MkdirAll(cfg.DataDir, 0o700); err != nil {
		return nil, fmt.Errorf("creating data directory %q: %w", cfg.DataDir, err)
	}
	st, err := store.Open(ctx, filepath.Join(cfg.DataDir, "repository.sqlite"))
	if err != nil {
		return nil, err
	}
	migrator := &store.Migrator{DB: st.DB(), FS: migrations.FS, Dir: "."}
	if err := migrator.Up(ctx); err != nil {
		if closeErr := st.Close(); closeErr != nil {
			return nil, fmt.Errorf("applying migrations: %w (and closing database: %w)", err, closeErr)
		}
		return nil, fmt.Errorf("applying migrations: %w", err)
	}
	return st, nil
}

func newAuthService(st *store.Store) *auth.Service {
	return auth.NewService(auth.ServiceOptions{
		Settings: st,
		Hasher:   auth.NewHasher(auth.DefaultParams),
		Sessions: auth.NewSessionManager(st, time.Now),
		Throttle: auth.NewThrottler(st, auth.DefaultThrottlePolicy, time.Now),
	})
}
