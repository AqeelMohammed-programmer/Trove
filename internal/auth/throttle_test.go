package auth

import (
	"testing"
	"time"
)

func TestThrottlePolicy_BlockDoubling_U9(t *testing.T) {
	t.Parallel()

	p := DefaultThrottlePolicy
	tests := []struct {
		failures int
		want     time.Duration
	}{
		{failures: 5, want: 60 * time.Second},
		{failures: 10, want: 120 * time.Second},
		{failures: 15, want: 240 * time.Second},
		{failures: 20, want: 480 * time.Second},
		{failures: 25, want: 15 * time.Minute}, // 960 s capped at 15 min
		{failures: 100, want: 15 * time.Minute},
	}
	for _, tt := range tests {
		if got := p.blockFor(tt.failures); got != tt.want {
			t.Errorf("blockFor(%d) = %v, want %v", tt.failures, got, tt.want)
		}
	}
}

func TestThrottler_SuccessResets_U9(t *testing.T) {
	t.Parallel()

	now := &testClock{now: time.Date(2026, 10, 24, 9, 0, 0, 0, time.UTC)}
	store := newFakeStore()
	th := NewThrottler(store, DefaultThrottlePolicy, now.Now)
	ctx := t.Context()

	for i := 0; i < 4; i++ {
		if _, err := th.Failure(ctx, "client"); err != nil {
			t.Fatalf("Failure(): %v", err)
		}
	}
	if err := th.Success(ctx, "client"); err != nil {
		t.Fatalf("Success(): %v", err)
	}
	if err := th.Check(ctx, "client"); err != nil {
		t.Fatalf("Check() after success = %v, want nil", err)
	}
	// The count starts over: four more failures do not block.
	for i := 0; i < 4; i++ {
		blocked, err := th.Failure(ctx, "client")
		if err != nil {
			t.Fatalf("Failure(): %v", err)
		}
		if blocked != nil {
			t.Fatalf("failure %d blocked too early", i+1)
		}
	}
}
