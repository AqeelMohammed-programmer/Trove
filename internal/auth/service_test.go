package auth

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// fakeStore is an in-memory implementation of the auth persistence
// interfaces. Integration with real SQLite is covered by the web tests.
type fakeStore struct {
	mu       sync.Mutex
	settings map[string]string
	sessions map[string]storedSession
	throttle map[string]storedThrottle
}

type storedSession struct {
	csrf      string
	createdAt time.Time
}

type storedThrottle struct {
	failures int
	blocked  *time.Time
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		settings: make(map[string]string),
		sessions: make(map[string]storedSession),
		throttle: make(map[string]storedThrottle),
	}
}

func (f *fakeStore) Setting(_ context.Context, key string) (string, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	v, ok := f.settings[key]
	return v, ok, nil
}

func (f *fakeStore) SetSetting(_ context.Context, key, value string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.settings[key] = value
	return nil
}

func (f *fakeStore) SaveSession(_ context.Context, hash, csrf string, createdAt time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sessions[hash] = storedSession{csrf: csrf, createdAt: createdAt}
	return nil
}

func (f *fakeStore) SessionByTokenHash(_ context.Context, hash string) (string, time.Time, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s, ok := f.sessions[hash]
	if !ok {
		return "", time.Time{}, ErrInvalidSession
	}
	return s.csrf, s.createdAt, nil
}

func (f *fakeStore) DeleteSession(_ context.Context, hash string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.sessions, hash)
	return nil
}

func (f *fakeStore) DeleteAllSessions(_ context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sessions = make(map[string]storedSession)
	return nil
}

func (f *fakeStore) LoginThrottle(_ context.Context, client string) (int, *time.Time, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	t := f.throttle[client]
	return t.failures, t.blocked, nil
}

func (f *fakeStore) SetLoginThrottle(_ context.Context, client string, failures int, blockedUntil *time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.throttle[client] = storedThrottle{failures: failures, blocked: blockedUntil}
	return nil
}

func (f *fakeStore) ResetLoginThrottle(_ context.Context, client string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.throttle, client)
	return nil
}

type testClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *testClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *testClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

func newTestService(now *testClock) *Service {
	store := newFakeStore()
	return NewService(ServiceOptions{
		Settings: store,
		Hasher:   testHasher(),
		Sessions: NewSessionManager(store, now.Now),
		Throttle: NewThrottler(store, DefaultThrottlePolicy, now.Now),
	})
}

func TestService_SetupAndLogin_U1_U4(t *testing.T) {
	t.Parallel()

	now := &testClock{now: time.Date(2026, 10, 24, 9, 0, 0, 0, time.UTC)}
	svc := newTestService(now)
	ctx := context.Background()

	configured, err := svc.Configured(ctx)
	if err != nil {
		t.Fatalf("Configured(): %v", err)
	}
	if configured {
		t.Fatal("Configured() = true on a fresh instance")
	}

	creds, err := svc.Setup(ctx, "hunter2hunter2", "hunter2hunter2")
	if err != nil {
		t.Fatalf("Setup(): %v", err)
	}
	if creds.Token == "" || creds.CSRF == "" {
		t.Fatal("Setup() returned empty credentials")
	}

	configured, err = svc.Configured(ctx)
	if err != nil {
		t.Fatalf("Configured(): %v", err)
	}
	if !configured {
		t.Fatal("Configured() = false after setup")
	}

	if _, err := svc.Setup(ctx, "anotherpassword", "anotherpassword"); !errors.Is(err, ErrAlreadyConfigured) {
		t.Fatalf("second Setup() error = %v, want ErrAlreadyConfigured", err)
	}

	session, err := svc.Authenticate(ctx, creds.Token)
	if err != nil {
		t.Fatalf("Authenticate(): %v", err)
	}
	if session.CSRFToken != creds.CSRF {
		t.Fatalf("session CSRF = %q, want %q", session.CSRFToken, creds.CSRF)
	}

	login, err := svc.Login(ctx, "127.0.0.1", "hunter2hunter2")
	if err != nil {
		t.Fatalf("Login(): %v", err)
	}
	if login.Token == creds.Token {
		t.Fatal("login reused the previous token; sessions are not regenerated")
	}

	if _, err := svc.Authenticate(ctx, login.Token); err != nil {
		t.Fatalf("Authenticate(login): %v", err)
	}
}

func TestService_LoginRejectsWrongPassword_U9(t *testing.T) {
	t.Parallel()

	now := &testClock{now: time.Date(2026, 10, 24, 9, 0, 0, 0, time.UTC)}
	svc := newTestService(now)
	ctx := context.Background()
	client := "10.0.0.1"

	if _, err := svc.Setup(ctx, "hunter2hunter2", "hunter2hunter2"); err != nil {
		t.Fatalf("Setup(): %v", err)
	}

	// Four failures: still allowed (just slow/denied, not blocked).
	for i := 0; i < 4; i++ {
		if _, err := svc.Login(ctx, client, "wrong"); !errors.Is(err, ErrWrongPassword) {
			t.Fatalf("attempt %d error = %v, want ErrWrongPassword", i+1, err)
		}
	}

	// Fifth failure starts the block, but the response is still "wrong password".
	if _, err := svc.Login(ctx, client, "wrong"); !errors.Is(err, ErrWrongPassword) {
		t.Fatalf("fifth attempt error = %v, want ErrWrongPassword", err)
	}

	// The next attempt is blocked even with the correct password.
	_, err := svc.Login(ctx, client, "hunter2hunter2")
	var blocked *BlockedError
	if !errors.As(err, &blocked) {
		t.Fatalf("blocked attempt error = %v, want *BlockedError", err)
	}

	// After the block expires the correct password works and resets state.
	now.Advance(61 * time.Second)
	if _, err := svc.Login(ctx, client, "hunter2hunter2"); err != nil {
		t.Fatalf("Login() after block: %v", err)
	}
}

func TestService_Logout_U6(t *testing.T) {
	t.Parallel()

	now := &testClock{now: time.Date(2026, 10, 24, 9, 0, 0, 0, time.UTC)}
	svc := newTestService(now)
	ctx := context.Background()

	creds, err := svc.Setup(ctx, "hunter2hunter2", "hunter2hunter2")
	if err != nil {
		t.Fatalf("Setup(): %v", err)
	}
	if err := svc.Logout(ctx, creds.Token); err != nil {
		t.Fatalf("Logout(): %v", err)
	}
	if _, err := svc.Authenticate(ctx, creds.Token); !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("Authenticate() after logout error = %v, want ErrInvalidSession", err)
	}
}

func TestService_ChangePassword_U7(t *testing.T) {
	t.Parallel()

	now := &testClock{now: time.Date(2026, 10, 24, 9, 0, 0, 0, time.UTC)}
	svc := newTestService(now)
	ctx := context.Background()

	creds, err := svc.Setup(ctx, "hunter2hunter2", "hunter2hunter2")
	if err != nil {
		t.Fatalf("Setup(): %v", err)
	}

	if _, err := svc.ChangePassword(ctx, creds.Token, "wrong", "newpassword1", "newpassword1"); !errors.Is(err, ErrWrongPassword) {
		t.Fatalf("ChangePassword(wrong current) error = %v, want ErrWrongPassword", err)
	}

	changed, err := svc.ChangePassword(ctx, creds.Token, "hunter2hunter2", "newpassword1", "newpassword1")
	if err != nil {
		t.Fatalf("ChangePassword(): %v", err)
	}
	if changed.Token == creds.Token {
		t.Fatal("ChangePassword() reused the old session token")
	}

	// Every old session is gone; the new one works.
	if _, err := svc.Authenticate(ctx, creds.Token); !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("old session still valid: %v", err)
	}
	if _, err := svc.Authenticate(ctx, changed.Token); err != nil {
		t.Fatalf("new session invalid: %v", err)
	}

	if _, err := svc.Login(ctx, "127.0.0.1", "hunter2hunter2"); !errors.Is(err, ErrWrongPassword) {
		t.Fatalf("old password still works: %v", err)
	}
	if _, err := svc.Login(ctx, "127.0.0.1", "newpassword1"); err != nil {
		t.Fatalf("new password rejected: %v", err)
	}
}

func TestService_ResetPassword_U10(t *testing.T) {
	t.Parallel()

	now := &testClock{now: time.Date(2026, 10, 24, 9, 0, 0, 0, time.UTC)}
	svc := newTestService(now)
	ctx := context.Background()

	creds, err := svc.Setup(ctx, "hunter2hunter2", "hunter2hunter2")
	if err != nil {
		t.Fatalf("Setup(): %v", err)
	}

	if err := svc.ResetPassword(ctx, "short", "short"); !errors.Is(err, ErrPasswordTooShort) {
		t.Fatalf("ResetPassword(short) error = %v, want ErrPasswordTooShort", err)
	}
	if err := svc.ResetPassword(ctx, "resetpassword", "resetpassword"); err != nil {
		t.Fatalf("ResetPassword(): %v", err)
	}

	if _, err := svc.Authenticate(ctx, creds.Token); !errors.Is(err, ErrInvalidSession) {
		t.Fatal("ResetPassword() did not end existing sessions")
	}
	if _, err := svc.Login(ctx, "127.0.0.1", "resetpassword"); err != nil {
		t.Fatalf("Login() with reset password: %v", err)
	}
}
