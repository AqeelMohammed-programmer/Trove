package main

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"Trove/internal/auth"
	"Trove/internal/store"
	"Trove/migrations"
)

func testService(t *testing.T) *auth.Service {
	t.Helper()

	st, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "test.sqlite"))
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	migrator := &store.Migrator{DB: st.DB(), FS: migrations.FS, Dir: "."}
	if err := migrator.Up(context.Background()); err != nil {
		t.Fatalf("migrating: %v", err)
	}
	return auth.NewService(auth.ServiceOptions{
		Settings: st,
		Hasher:   auth.NewHasher(auth.Params{Memory: 64, Time: 1, Threads: 1, KeyLen: 32, SaltLen: 16}),
		Sessions: auth.NewSessionManager(st, time.Now),
		Throttle: auth.NewThrottler(st, auth.DefaultThrottlePolicy, time.Now),
	})
}

func prompts(answers ...string) func(string) (string, error) {
	i := 0
	return func(string) (string, error) {
		if i >= len(answers) {
			return "", errors.New("test: out of answers")
		}
		answer := answers[i]
		i++
		return answer, nil
	}
}

func TestPerformReset_U10(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc := testService(t)
	creds, err := svc.Setup(ctx, "oldpassword", "oldpassword")
	if err != nil {
		t.Fatalf("Setup(): %v", err)
	}

	if err := performReset(ctx, svc, prompts("newpassword", "newpassword")); err != nil {
		t.Fatalf("performReset(): %v", err)
	}

	if _, err := svc.Authenticate(ctx, creds.Token); !errors.Is(err, auth.ErrInvalidSession) {
		t.Error("reset did not end existing sessions")
	}
	if _, err := svc.Login(ctx, "client", "oldpassword"); !errors.Is(err, auth.ErrWrongPassword) {
		t.Errorf("old password still works: %v", err)
	}
	if _, err := svc.Login(ctx, "client", "newpassword"); err != nil {
		t.Errorf("new password rejected: %v", err)
	}
}

func TestPerformReset_RejectsMismatch_U10(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc := testService(t)
	if _, err := svc.Setup(ctx, "oldpassword", "oldpassword"); err != nil {
		t.Fatalf("Setup(): %v", err)
	}

	if err := performReset(ctx, svc, prompts("newpassword", "different")); !errors.Is(err, auth.ErrPasswordMismatch) {
		t.Fatalf("performReset() error = %v, want ErrPasswordMismatch", err)
	}
	if _, err := svc.Login(ctx, "client", "oldpassword"); err != nil {
		t.Errorf("old password rejected after failed reset: %v", err)
	}
}
