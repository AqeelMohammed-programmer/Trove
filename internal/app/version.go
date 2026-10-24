package app

import "fmt"

// Build information. The zero values are used by `go run` and tests; the
// Makefile injects real values with -ldflags at build time.
var (
	// Version is the semantic version of the build.
	Version = "dev"
	// Commit is the short git commit the binary was built from.
	Commit = "none"
	// BuildDate is the UTC date and time of the build.
	BuildDate = "unknown"
)

// BuildInfo returns a human-readable one-line build description.
func BuildInfo() string {
	return fmt.Sprintf("%s %s (commit %s, built %s)", Name, Version, Commit, BuildDate)
}
