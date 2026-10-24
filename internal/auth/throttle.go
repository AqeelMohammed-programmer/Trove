package auth

import (
	"context"
	"fmt"
	"time"
)

// ThrottleStore persists login failure state per client. It is implemented by
// internal/store so throttling survives a restart.
type ThrottleStore interface {
	LoginThrottle(ctx context.Context, client string) (failures int, blockedUntil *time.Time, err error)
	SetLoginThrottle(ctx context.Context, client string, failures int, blockedUntil *time.Time) error
	ResetLoginThrottle(ctx context.Context, client string) error
}

// ThrottlePolicy implements the defaults in spec section 8.2: after every five
// consecutive failures the block starts at one minute and doubles, capped at
// fifteen minutes.
type ThrottlePolicy struct {
	FailuresPerStep int
	InitialBlock    time.Duration
	MaxBlock        time.Duration
}

// DefaultThrottlePolicy is the policy from spec section 8.2.
var DefaultThrottlePolicy = ThrottlePolicy{
	FailuresPerStep: 5,
	InitialBlock:    60 * time.Second,
	MaxBlock:        15 * time.Minute,
}

// BlockedError reports an active login block (U-9).
type BlockedError struct {
	Until time.Time
}

func (e *BlockedError) Error() string {
	return fmt.Sprintf("auth: login blocked until %s", e.Until.UTC().Format(time.RFC3339))
}

// Throttler records failures and enforces blocks.
type Throttler struct {
	store  ThrottleStore
	policy ThrottlePolicy
	now    func() time.Time
}

// NewThrottler returns a throttler with the given store and policy.
func NewThrottler(store ThrottleStore, policy ThrottlePolicy, now func() time.Time) *Throttler {
	if now == nil {
		now = time.Now
	}
	return &Throttler{store: store, policy: policy, now: now}
}

// Check returns a *BlockedError when the client is currently blocked.
func (t *Throttler) Check(ctx context.Context, client string) error {
	_, blockedUntil, err := t.store.LoginThrottle(ctx, client)
	if err != nil {
		return fmt.Errorf("reading login throttle: %w", err)
	}
	if blockedUntil != nil && blockedUntil.After(t.now()) {
		return &BlockedError{Until: *blockedUntil}
	}
	return nil
}

// Failure records one failed attempt and starts a block when the threshold is
// reached. It returns the block that is now in force, if any.
func (t *Throttler) Failure(ctx context.Context, client string) (*BlockedError, error) {
	failures, _, err := t.store.LoginThrottle(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("reading login throttle: %w", err)
	}
	failures++

	var blockedUntil *time.Time
	blocked := false
	if failures%t.policy.FailuresPerStep == 0 {
		until := t.now().Add(t.policy.blockFor(failures))
		blockedUntil = &until
		blocked = true
	}
	if err := t.store.SetLoginThrottle(ctx, client, failures, blockedUntil); err != nil {
		return nil, fmt.Errorf("writing login throttle: %w", err)
	}
	if blocked {
		return &BlockedError{Until: *blockedUntil}, nil
	}
	return nil, nil
}

// Success clears the failure count after a correct password.
func (t *Throttler) Success(ctx context.Context, client string) error {
	if err := t.store.ResetLoginThrottle(ctx, client); err != nil {
		return fmt.Errorf("resetting login throttle: %w", err)
	}
	return nil
}

// blockFor returns the block duration for the given failure count.
func (p ThrottlePolicy) blockFor(failures int) time.Duration {
	steps := failures / p.FailuresPerStep // 1 after 5 failures, 2 after 10, ...
	if steps < 1 {
		steps = 1
	}
	d := p.InitialBlock
	for i := 1; i < steps; i++ {
		d *= 2
		if d >= p.MaxBlock {
			return p.MaxBlock
		}
	}
	if d > p.MaxBlock {
		return p.MaxBlock
	}
	return d
}
