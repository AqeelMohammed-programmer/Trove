package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

// CookieName is the session cookie name. It is deliberately generic: it does
// not reveal the product or the technology.
const CookieName = "trove_session"

// sessionTokenBytes is the entropy of a session token.
const sessionTokenBytes = 32

// ErrInvalidSession is returned when a token does not match a live session.
var ErrInvalidSession = errors.New("auth: invalid session")

// Session is a live server-side session. The raw token is never stored; only
// its SHA-256 is.
type Session struct {
	TokenHash string
	CSRFToken string
	CreatedAt time.Time
}

// SessionStore persists sessions. It is implemented by internal/store.
type SessionStore interface {
	SaveSession(ctx context.Context, tokenHash, csrfToken string, createdAt time.Time) error
	SessionByTokenHash(ctx context.Context, tokenHash string) (csrfToken string, createdAt time.Time, err error)
	DeleteSession(ctx context.Context, tokenHash string) error
	DeleteAllSessions(ctx context.Context) error
}

// SessionManager issues and resolves opaque session tokens.
type SessionManager struct {
	store SessionStore
	now   func() time.Time
}

// NewSessionManager returns a manager backed by store.
func NewSessionManager(store SessionStore, now func() time.Time) *SessionManager {
	if now == nil {
		now = time.Now
	}
	return &SessionManager{store: store, now: now}
}

// Start creates a session and returns the raw token (for the cookie), the CSRF
// token (for forms) and the stored session. Regenerating on every login
// prevents session fixation (U-4).
func (m *SessionManager) Start(ctx context.Context) (token, csrf string, sess Session, err error) {
	token, err = randomHex(sessionTokenBytes)
	if err != nil {
		return "", "", Session{}, err
	}
	csrf, err = randomHex(16)
	if err != nil {
		return "", "", Session{}, err
	}
	sess = Session{
		TokenHash: HashToken(token),
		CSRFToken: csrf,
		CreatedAt: m.now().UTC(),
	}
	if err := m.store.SaveSession(ctx, sess.TokenHash, sess.CSRFToken, sess.CreatedAt); err != nil {
		return "", "", Session{}, fmt.Errorf("saving session: %w", err)
	}
	return token, csrf, sess, nil
}

// Authenticate resolves a raw token to its session.
func (m *SessionManager) Authenticate(ctx context.Context, token string) (Session, error) {
	if token == "" {
		return Session{}, ErrInvalidSession
	}
	hash := HashToken(token)
	csrf, createdAt, err := m.store.SessionByTokenHash(ctx, hash)
	if err != nil {
		return Session{}, ErrInvalidSession
	}
	return Session{TokenHash: hash, CSRFToken: csrf, CreatedAt: createdAt}, nil
}

// End deletes the session for a raw token. Unknown tokens are ignored.
func (m *SessionManager) End(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	if err := m.store.DeleteSession(ctx, HashToken(token)); err != nil {
		return fmt.Errorf("ending session: %w", err)
	}
	return nil
}

// EndAll deletes every session. Used after a password change or reset.
func (m *SessionManager) EndAll(ctx context.Context) error {
	if err := m.store.DeleteAllSessions(ctx); err != nil {
		return fmt.Errorf("ending all sessions: %w", err)
	}
	return nil
}

// HashToken returns the hex SHA-256 of a raw session token.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("auth: reading random bytes: %w", err)
	}
	return hex.EncodeToString(b), nil
}
