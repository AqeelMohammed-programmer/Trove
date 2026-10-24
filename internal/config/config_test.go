package config

import (
	"errors"
	"flag"
	"io"
	"strings"
	"testing"
)

func envFrom(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

func TestLoad_Defaults(t *testing.T) {
	t.Parallel()

	got, err := Load(nil, envFrom(nil), io.Discard)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	want := Config{ListenAddr: DefaultListenAddr, DataDir: DefaultDataDir}
	if got != want {
		t.Fatalf("Load() = %+v, want %+v", got, want)
	}
}

func TestLoad_EnvOverridesDefaults(t *testing.T) {
	t.Parallel()

	got, err := Load(nil, envFrom(map[string]string{
		"TROVE_LISTEN_ADDR":   "0.0.0.0:9000",
		"TROVE_DATA_DIR":      "/var/lib/trove",
		"TROVE_SECURE_COOKIE": "true",
	}), io.Discard)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	want := Config{ListenAddr: "0.0.0.0:9000", DataDir: "/var/lib/trove", SecureCookie: true}
	if got != want {
		t.Fatalf("Load() = %+v, want %+v", got, want)
	}
}

func TestLoad_FlagsOverrideEnv(t *testing.T) {
	t.Parallel()

	got, err := Load(
		[]string{"-listen", "127.0.0.1:9001", "-data-dir", "/tmp/trove-test", "-secure-cookie=false"},
		envFrom(map[string]string{
			"TROVE_LISTEN_ADDR":   "0.0.0.0:9000",
			"TROVE_DATA_DIR":      "/var/lib/trove",
			"TROVE_SECURE_COOKIE": "true",
		}),
		io.Discard,
	)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	want := Config{ListenAddr: "127.0.0.1:9001", DataDir: "/tmp/trove-test"}
	if got != want {
		t.Fatalf("Load() = %+v, want %+v", got, want)
	}
}

func TestLoad_NormalizesDataDir(t *testing.T) {
	t.Parallel()

	got, err := Load([]string{"-data-dir", "  /tmp/one/../two  "}, envFrom(nil), io.Discard)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if got.DataDir != "/tmp/two" {
		t.Fatalf("DataDir = %q, want %q", got.DataDir, "/tmp/two")
	}
}

func TestLoad_InvalidInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
		env  map[string]string
		want string
	}{
		{name: "missing port", args: []string{"-listen", "localhost"}, want: "listen address"},
		{name: "port out of range", args: []string{"-listen", "localhost:70000"}, want: "between 0 and 65535"},
		{name: "non-numeric port", args: []string{"-listen", "localhost:http"}, want: "between 0 and 65535"},
		{name: "empty listen flag", args: []string{"-listen", "   "}, want: "listen address must not be empty"},
		{name: "empty data dir", args: []string{"-data-dir", "   "}, want: "data directory must not be empty"},
		{name: "positional argument", args: []string{"extra"}, want: "unexpected argument"},
		{name: "unknown flag", args: []string{"-nope"}, want: "flag provided but not defined"},
		{name: "invalid env bool", env: map[string]string{"TROVE_SECURE_COOKIE": "maybe"}, want: "TROVE_SECURE_COOKIE"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := Load(tt.args, envFrom(tt.env), io.Discard)
			if err == nil {
				t.Fatal("Load() error = nil, want an error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Load() error = %q, want it to contain %q", err, tt.want)
			}
		})
	}
}

func TestLoad_Help(t *testing.T) {
	t.Parallel()

	_, err := Load([]string{"-h"}, envFrom(nil), io.Discard)
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("Load(-h) error = %v, want flag.ErrHelp", err)
	}
}
