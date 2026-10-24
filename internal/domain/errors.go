package domain

import "errors"

// ErrNotFound means the requested entity does not exist (or is trashed where
// only active entities are visible).
var ErrNotFound = errors.New("domain: not found")

// ErrConflict means the requested change clashes with existing data, such as
// renaming a tag to a name another tag already uses (M-2).
var ErrConflict = errors.New("domain: conflict")
