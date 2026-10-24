package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Setting returns a settings value and whether it exists.
func (s *Store) Setting(ctx context.Context, key string) (string, bool, error) {
	var value string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = ?`, key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("reading setting %q: %w", key, err)
	}
	return value, true, nil
}

// SetSetting inserts or replaces a settings value.
func (s *Store) SetSetting(ctx context.Context, key, value string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO settings (key, value) VALUES (?, ?)
		 ON CONFLICT (key) DO UPDATE SET value = excluded.value`,
		key, value)
	if err != nil {
		return fmt.Errorf("writing setting %q: %w", key, err)
	}
	return nil
}

// SaveSession stores a new session. Only the token hash is persisted.
func (s *Store) SaveSession(ctx context.Context, tokenHash, csrfToken string, createdAt time.Time) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO sessions (token_hash, csrf_token, created_at) VALUES (?, ?, ?)`,
		tokenHash, csrfToken, formatTime(createdAt))
	if err != nil {
		return fmt.Errorf("saving session: %w", err)
	}
	return nil
}

// SessionByTokenHash returns the CSRF token and creation time for a session.
func (s *Store) SessionByTokenHash(ctx context.Context, tokenHash string) (string, time.Time, error) {
	var csrf, createdAt string
	err := s.db.QueryRowContext(ctx,
		`SELECT csrf_token, created_at FROM sessions WHERE token_hash = ?`, tokenHash).
		Scan(&csrf, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return "", time.Time{}, ErrNotFound
	}
	if err != nil {
		return "", time.Time{}, fmt.Errorf("reading session: %w", err)
	}
	created, err := parseTime(createdAt)
	if err != nil {
		return "", time.Time{}, err
	}
	return csrf, created, nil
}

// DeleteSession removes one session. Unknown tokens are not an error.
func (s *Store) DeleteSession(ctx context.Context, tokenHash string) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = ?`, tokenHash); err != nil {
		return fmt.Errorf("deleting session: %w", err)
	}
	return nil
}

// DeleteAllSessions removes every session (password change or reset).
func (s *Store) DeleteAllSessions(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM sessions`); err != nil {
		return fmt.Errorf("deleting all sessions: %w", err)
	}
	return nil
}

// LoginThrottle returns the failure count and active block for a client.
func (s *Store) LoginThrottle(ctx context.Context, client string) (int, *time.Time, error) {
	var (
		failures int
		blocked  sql.NullString
	)
	err := s.db.QueryRowContext(ctx,
		`SELECT failures, blocked_until FROM login_throttle WHERE client = ?`, client).
		Scan(&failures, &blocked)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil, nil
	}
	if err != nil {
		return 0, nil, fmt.Errorf("reading login throttle: %w", err)
	}
	if !blocked.Valid || blocked.String == "" {
		return failures, nil, nil
	}
	until, err := parseTime(blocked.String)
	if err != nil {
		return 0, nil, err
	}
	return failures, &until, nil
}

// SetLoginThrottle stores the failure count and optional block time.
func (s *Store) SetLoginThrottle(ctx context.Context, client string, failures int, blockedUntil *time.Time) error {
	var blocked sql.NullString
	if blockedUntil != nil {
		blocked = sql.NullString{String: formatTime(*blockedUntil), Valid: true}
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO login_throttle (client, failures, blocked_until) VALUES (?, ?, ?)
		 ON CONFLICT (client) DO UPDATE
		   SET failures = excluded.failures, blocked_until = excluded.blocked_until`,
		client, failures, blocked)
	if err != nil {
		return fmt.Errorf("writing login throttle: %w", err)
	}
	return nil
}

// ResetLoginThrottle clears the failure state after a successful login.
func (s *Store) ResetLoginThrottle(ctx context.Context, client string) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM login_throttle WHERE client = ?`, client); err != nil {
		return fmt.Errorf("resetting login throttle: %w", err)
	}
	return nil
}

func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

func parseTime(s string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("parsing stored timestamp %q: %w", s, err)
	}
	return t.UTC(), nil
}
