package domain

import "errors"

// ErrNotFound means the requested entity does not exist (or is trashed where
// only active entities are visible).
var ErrNotFound = errors.New("domain: not found")
