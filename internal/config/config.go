// Package config loads and validates the runtime configuration from
// command-line flags and environment variables.
package config

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"path/filepath"
	"strconv"
	"strings"

	"Trove/internal/app"
)

// Defaults used when neither a flag nor an environment variable is set.
const (
	DefaultListenAddr = "127.0.0.1:8080"
	DefaultDataDir    = "data"
)

// Config holds validated runtime settings. It contains no secrets.
type Config struct {
	// ListenAddr is the host:port the HTTP server binds to.
	ListenAddr string
	// DataDir is the directory holding the database and copied files.
	DataDir string
	// SecureCookie marks the session cookie as Secure. Enable it when the
	// application is served over HTTPS.
	SecureCookie bool
}

// Load parses args (without the program name), applying environment
// variables as defaults so that explicit flags win over the environment.
// Diagnostics from flag parsing are written to stderr; pass io.Discard to
// keep tests quiet. It returns flag.ErrHelp when -h or -help is requested.
func Load(args []string, getenv func(string) string, stderr io.Writer) (Config, error) {
	cfg, err := fromEnv(getenv)
	if err != nil {
		return Config{}, err
	}

	fs := flag.NewFlagSet(app.Name, flag.ContinueOnError)
	fs.SetOutput(stderr)

	listen := fs.String("listen", cfg.ListenAddr,
		"host:port to listen on (env "+envName("LISTEN_ADDR")+")")
	dataDir := fs.String("data-dir", cfg.DataDir,
		"directory for the database and copied files (env "+envName("DATA_DIR")+")")
	secureCookie := fs.Bool("secure-cookie", cfg.SecureCookie,
		"mark the session cookie Secure; enable when served over HTTPS (env "+envName("SECURE_COOKIE")+")")

	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}
	if fs.NArg() > 0 {
		return Config{}, fmt.Errorf("unexpected argument %q", fs.Arg(0))
	}

	return newConfig(*listen, *dataDir, *secureCookie)
}

func newConfig(listen, dataDir string, secureCookie bool) (Config, error) {
	cfg := Config{
		ListenAddr:   strings.TrimSpace(listen),
		DataDir:      strings.TrimSpace(dataDir),
		SecureCookie: secureCookie,
	}
	if cfg.ListenAddr == "" {
		return Config{}, errors.New("listen address must not be empty")
	}
	if cfg.DataDir == "" {
		return Config{}, errors.New("data directory must not be empty")
	}
	if err := validateListenAddr(cfg.ListenAddr); err != nil {
		return Config{}, err
	}
	cfg.DataDir = filepath.Clean(cfg.DataDir)
	return cfg, nil
}

func validateListenAddr(addr string) error {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("listen address %q: %w", addr, err)
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 0 || n > 65535 {
		return fmt.Errorf("listen address %q: port must be a number between 0 and 65535", addr)
	}
	return nil
}

// fromEnv reads the environment-backed defaults. Malformed values are
// reported rather than silently ignored.
func fromEnv(getenv func(string) string) (Config, error) {
	cfg := Config{
		ListenAddr: DefaultListenAddr,
		DataDir:    DefaultDataDir,
	}
	if v := strings.TrimSpace(getenv(envName("LISTEN_ADDR"))); v != "" {
		cfg.ListenAddr = v
	}
	if v := strings.TrimSpace(getenv(envName("DATA_DIR"))); v != "" {
		cfg.DataDir = v
	}
	if v := strings.TrimSpace(getenv(envName("SECURE_COOKIE"))); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return Config{}, fmt.Errorf("parsing %s: %w", envName("SECURE_COOKIE"), err)
		}
		cfg.SecureCookie = b
	}
	return cfg, nil
}

// envName builds an environment variable name from the product name.
func envName(suffix string) string {
	return strings.ToUpper(app.Name) + "_" + suffix
}
