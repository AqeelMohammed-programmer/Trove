// Package clock provides the time source used by services. Injecting it keeps
// tests deterministic instead of sleeping or comparing against wall time.
package clock

import "time"

// Clock supplies the current time.
type Clock interface {
	Now() time.Time
}

// System is the real clock.
type System struct{}

// Now returns the current local time. Callers store UTC.
func (System) Now() time.Time { return time.Now() }

// Fixed is a test clock that always returns the same instant. It is safe to
// read from parallel tests as long as the value is not changed.
type Fixed time.Time

// Now returns the fixed instant.
func (f Fixed) Now() time.Time { return time.Time(f) }
