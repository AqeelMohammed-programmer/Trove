// Package migrations embeds the forward-only SQL migrations so the binary is
// self-contained. Files are named NNNN_snake_case_name.sql and are applied in
// ascending version order at startup.
//
// The directory currently holds no .sql files: M0 provides the runner only.
// The first migration arrives with the authentication milestone (M1).
package migrations

import "embed"

// FS holds the migration directory. Only *.sql files are read at runtime;
// this file and README.md are embedded too but ignored by the migrator.
//
//go:embed *
var FS embed.FS
