package auth

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"unicode/utf8"
)

// passwordHashKey is the settings row holding the encoded Argon2id hash.
const passwordHashKey = "password_hash"

var (
	// ErrNotConfigured means no password has been set yet (first run).
	ErrNotConfigured = errors.New("auth: no password configured")
	// ErrAlreadyConfigured means setup was attempted twice.
	ErrAlreadyConfigured = errors.New("auth: password already configured")
	// ErrWrongPassword means the supplied password did not match.
	ErrWrongPassword = errors.New("auth: wrong password")
	// ErrPasswordTooShort means the password fails U-2.
	ErrPasswordTooShort = fmt.Errorf("auth: password must be at least %d characters", MinPasswordLength)
	// ErrPasswordMismatch means confirmation did not match.
	ErrPasswordMismatch = errors.New("auth: password confirmation does not match")
	// ErrInvalidCSRF means the form token did not match the session.
	ErrInvalidCSRF = errors.New("auth: invalid CSRF token")
)

// SettingsStore persists small key/value settings. Implemented by
// internal/store.
type SettingsStore interface {
	Setting(ctx context.Context, key string) (value string, ok bool, err error)
	SetSetting(ctx context.Context, key, value string) error
}

// Credentials is what a successful setup or login hands to the web layer: the
// raw token for the cookie, the CSRF token for forms, and the stored session.
type Credentials struct {
	Token   string
	CSRF    string
	Session Session
}

// Service implements password, session and throttling use cases. It has no
// knowledge of HTTP.
type Service struct {
	settings SettingsStore
	hasher   *Hasher
	sessions *SessionManager
	throttle *Throttler
}

// ServiceOptions wires a Service.
type ServiceOptions struct {
	Settings SettingsStore
	Hasher   *Hasher
	Sessions *SessionManager
	Throttle *Throttler
}

// NewService builds the authentication service.
func NewService(opts ServiceOptions) *Service {
	return &Service{
		settings: opts.Settings,
		hasher:   opts.Hasher,
		sessions: opts.Sessions,
		throttle: opts.Throttle,
	}
}

// Configured reports whether a password has been set (U-1).
func (s *Service) Configured(ctx context.Context) (bool, error) {
	_, ok, err := s.settings.Setting(ctx, passwordHashKey)
	if err != nil {
		return false, fmt.Errorf("reading password state: %w", err)
	}
	return ok, nil
}

// Setup stores the first password and starts a session (U-1, U-4).
func (s *Service) Setup(ctx context.Context, password, confirm string) (Credentials, error) {
	configured, err := s.Configured(ctx)
	if err != nil {
		return Credentials{}, err
	}
	if configured {
		return Credentials{}, ErrAlreadyConfigured
	}
	if err := ValidateNewPassword(password, confirm); err != nil {
		return Credentials{}, err
	}
	if err := s.storeHash(ctx, password); err != nil {
		return Credentials{}, err
	}
	return s.start(ctx)
}

// Login verifies a password and starts a session, applying throttling (U-9).
// client identifies the source of the attempt, normally the remote address.
func (s *Service) Login(ctx context.Context, client, password string) (Credentials, error) {
	if err := s.throttle.Check(ctx, client); err != nil {
		return Credentials{}, err
	}
	encoded, ok, err := s.settings.Setting(ctx, passwordHashKey)
	if err != nil {
		return Credentials{}, fmt.Errorf("reading password hash: %w", err)
	}
	if !ok {
		return Credentials{}, ErrNotConfigured
	}
	match, err := s.hasher.Verify(encoded, password)
	if err != nil {
		return Credentials{}, fmt.Errorf("verifying password: %w", err)
	}
	if !match {
		if _, err := s.throttle.Failure(ctx, client); err != nil {
			return Credentials{}, err
		}
		return Credentials{}, ErrWrongPassword
	}
	if err := s.throttle.Success(ctx, client); err != nil {
		return Credentials{}, err
	}
	return s.start(ctx)
}

// Authenticate resolves a cookie token to a session.
func (s *Service) Authenticate(ctx context.Context, token string) (Session, error) {
	return s.sessions.Authenticate(ctx, token)
}

// Logout ends the session for the given token.
func (s *Service) Logout(ctx context.Context, token string) error {
	return s.sessions.End(ctx, token)
}

// ChangePassword verifies the current password, stores a new hash, ends every
// session and starts a fresh one for the caller (U-7, U-10 behaviour).
func (s *Service) ChangePassword(ctx context.Context, token, current, next, confirm string) (Credentials, error) {
	if _, err := s.sessions.Authenticate(ctx, token); err != nil {
		return Credentials{}, err
	}
	encoded, ok, err := s.settings.Setting(ctx, passwordHashKey)
	if err != nil {
		return Credentials{}, fmt.Errorf("reading password hash: %w", err)
	}
	if !ok {
		return Credentials{}, ErrNotConfigured
	}
	match, err := s.hasher.Verify(encoded, current)
	if err != nil {
		return Credentials{}, fmt.Errorf("verifying password: %w", err)
	}
	if !match {
		return Credentials{}, ErrWrongPassword
	}
	if err := ValidateNewPassword(next, confirm); err != nil {
		return Credentials{}, err
	}
	if err := s.storeHash(ctx, next); err != nil {
		return Credentials{}, err
	}
	if err := s.sessions.EndAll(ctx); err != nil {
		return Credentials{}, err
	}
	return s.start(ctx)
}

// ResetPassword stores a new password and ends every session. It is used by
// the local CLI command only, never by an HTTP handler (U-10).
func (s *Service) ResetPassword(ctx context.Context, next, confirm string) error {
	if err := ValidateNewPassword(next, confirm); err != nil {
		return err
	}
	if err := s.storeHash(ctx, next); err != nil {
		return err
	}
	return s.sessions.EndAll(ctx)
}

// ValidateCSRF compares a submitted form token with the session token in
// constant time.
func ValidateCSRF(session Session, token string) bool {
	return subtle.ConstantTimeCompare([]byte(session.CSRFToken), []byte(token)) == 1
}

// ValidateNewPassword enforces the password rules shared by setup, change and
// reset (U-2).
func ValidateNewPassword(password, confirm string) error {
	if utf8.RuneCountInString(password) < MinPasswordLength {
		return ErrPasswordTooShort
	}
	if password != confirm {
		return ErrPasswordMismatch
	}
	return nil
}

func (s *Service) storeHash(ctx context.Context, password string) error {
	encoded, err := s.hasher.Hash(password)
	if err != nil {
		return fmt.Errorf("hashing password: %w", err)
	}
	if err := s.settings.SetSetting(ctx, passwordHashKey, encoded); err != nil {
		return fmt.Errorf("storing password hash: %w", err)
	}
	return nil
}

func (s *Service) start(ctx context.Context) (Credentials, error) {
	token, csrf, session, err := s.sessions.Start(ctx)
	if err != nil {
		return Credentials{}, err
	}
	return Credentials{Token: token, CSRF: csrf, Session: session}, nil
}

// SessionAge is exposed for tests and logs; sessions have no inactivity
// timeout (U-5), so this is informational only.
