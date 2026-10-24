// Package app holds the application's identity constants.
package app

// Name is the product name shown to the user and used as the prefix for
// environment variables. It is the single source of truth for the name;
// the module path in go.mod is the only other place the literal appears.
// Renaming the product is a two-line change.
const Name = "Trove"
