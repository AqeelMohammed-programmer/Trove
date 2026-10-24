package web

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"Trove/internal/auth"
	"Trove/internal/store"
	"Trove/migrations"
)

// testHasher is deliberately cheap: these tests exercise flows, not Argon2id.
func testHasher() *auth.Hasher {
	return auth.NewHasher(auth.Params{Memory: 64, Time: 1, Threads: 1, KeyLen: 32, SaltLen: 16})
}

func newTestServer(t *testing.T) (*Server, *auth.Service) {
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

	svc := auth.NewService(auth.ServiceOptions{
		Settings: st,
		Hasher:   testHasher(),
		Sessions: auth.NewSessionManager(st, time.Now),
		Throttle: auth.NewThrottler(st, auth.DefaultThrottlePolicy, time.Now),
	})
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return New(Options{Logger: logger, Auth: svc}), svc
}

// request performs a request with an optional session cookie and form values.
func request(t *testing.T, h http.Handler, method, path string, form url.Values, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()

	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	req := httptest.NewRequest(method, path, body)
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// sessionCookie extracts the session cookie from a response.
func sessionCookie(t *testing.T, rec *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, c := range rec.Result().Cookies() {
		if c.Name == auth.CookieName {
			return c
		}
	}
	t.Fatal("response did not set a session cookie")
	return nil
}

// setupInstance runs first-run setup and returns the session cookie.
func setupInstance(t *testing.T, h http.Handler, password string) *http.Cookie {
	t.Helper()
	rec := request(t, h, http.MethodPost, "/setup",
		url.Values{"password": {password}, "confirm": {password}}, nil)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("POST /setup status = %d, want 303 (body %q)", rec.Code, rec.Body.String())
	}
	return sessionCookie(t, rec)
}

func TestSetupFlow_U1_U2_U5(t *testing.T) {
	t.Parallel()

	srv, _ := newTestServer(t)
	h := srv.Handler()

	if rec := request(t, h, http.MethodGet, "/setup", nil, nil); rec.Code != http.StatusOK {
		t.Fatalf("GET /setup status = %d, want 200", rec.Code)
	}
	// /login redirects to setup before a password exists (U-1).
	if rec := request(t, h, http.MethodGet, "/login", nil, nil); rec.Code != http.StatusSeeOther ||
		rec.Header().Get("Location") != "/setup" {
		t.Fatalf("GET /login before setup = %d %q, want 303 /setup", rec.Code, rec.Header().Get("Location"))
	}

	// Too short is rejected with a message (U-2).
	rec := request(t, h, http.MethodPost, "/setup",
		url.Values{"password": {"short"}, "confirm": {"short"}}, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("short password status = %d, want 400", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "at least 8 characters") {
		t.Errorf("short password response does not explain the rule: %q", rec.Body.String())
	}

	// Mismatched confirmation is rejected.
	rec = request(t, h, http.MethodPost, "/setup",
		url.Values{"password": {"longenough"}, "confirm": {"different"}}, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("mismatch status = %d, want 400", rec.Code)
	}

	cookie := setupInstance(t, h, "hunter2hunter2")
	if !cookie.HttpOnly {
		t.Error("session cookie is not HttpOnly")
	}
	if cookie.SameSite != http.SameSiteLaxMode {
		t.Errorf("session cookie SameSite = %v, want Lax", cookie.SameSite)
	}
	if !cookie.Expires.IsZero() || cookie.MaxAge != 0 {
		t.Errorf("session cookie is persistent (Expires=%v, MaxAge=%d); U-5 requires a browser-session cookie",
			cookie.Expires, cookie.MaxAge)
	}
	if cookie.Secure {
		t.Error("session cookie is Secure on plain HTTP configuration")
	}

	// Setup cannot be repeated.
	if rec := request(t, h, http.MethodGet, "/setup", nil, cookie); rec.Code != http.StatusSeeOther {
		t.Errorf("GET /setup after setup = %d, want 303", rec.Code)
	}

	// The new session works.
	if rec := request(t, h, http.MethodGet, "/", nil, cookie); rec.Code != http.StatusOK {
		t.Errorf("GET / with session = %d, want 200", rec.Code)
	}
}

func TestUnauthenticatedRoutesRejected_U3(t *testing.T) {
	t.Parallel()

	srv, _ := newTestServer(t)
	h := srv.Handler()

	for _, route := range srv.RegisteredRoutes() {
		if route.Public {
			continue
		}
		t.Run(route.Method+" "+route.Pattern, func(t *testing.T) {
			rec := request(t, h, route.Method, route.Sample, nil, nil)
			if rec.Code != http.StatusSeeOther {
				t.Fatalf("unauthenticated %s %s status = %d, want 303", route.Method, route.Sample, rec.Code)
			}
			if got := rec.Header().Get("Location"); got != "/login" {
				t.Fatalf("redirect Location = %q, want /login", got)
			}
		})
	}
}

func TestLoginFlow_U4(t *testing.T) {
	t.Parallel()

	srv, _ := newTestServer(t)
	h := srv.Handler()
	setupInstance(t, h, "hunter2hunter2")

	rec := request(t, h, http.MethodPost, "/login", url.Values{"password": {"wrong"}}, nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("wrong password status = %d, want 401", rec.Code)
	}
	if len(rec.Result().Cookies()) != 0 {
		t.Error("wrong password set a session cookie")
	}

	rec = request(t, h, http.MethodPost, "/login", url.Values{"password": {"hunter2hunter2"}}, nil)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("correct password status = %d, want 303", rec.Code)
	}
	if got := rec.Header().Get("Location"); got != "/" {
		t.Errorf("Location = %q, want /", got)
	}
	cookie := sessionCookie(t, rec)
	if rec := request(t, h, http.MethodGet, "/", nil, cookie); rec.Code != http.StatusOK {
		t.Errorf("GET / with login cookie = %d, want 200", rec.Code)
	}
}

func TestLoginThrottle_U9(t *testing.T) {
	t.Parallel()

	srv, _ := newTestServer(t)
	h := srv.Handler()
	setupInstance(t, h, "hunter2hunter2")

	for i := 0; i < 5; i++ {
		rec := request(t, h, http.MethodPost, "/login", url.Values{"password": {"wrong"}}, nil)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d status = %d, want 401", i+1, rec.Code)
		}
	}
	rec := request(t, h, http.MethodPost, "/login", url.Values{"password": {"hunter2hunter2"}}, nil)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("blocked attempt status = %d, want 429", rec.Code)
	}
}

func TestLogout_U6(t *testing.T) {
	t.Parallel()

	srv, svc := newTestServer(t)
	h := srv.Handler()
	cookie := setupInstance(t, h, "hunter2hunter2")

	sess, err := svc.Authenticate(context.Background(), cookie.Value)
	if err != nil {
		t.Fatalf("Authenticate(): %v", err)
	}

	// Without the CSRF token the logout is refused.
	rec := request(t, h, http.MethodPost, "/logout", url.Values{}, cookie)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("logout without CSRF = %d, want 403", rec.Code)
	}

	rec = request(t, h, http.MethodPost, "/logout", url.Values{"csrf": {sess.CSRFToken}}, cookie)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("logout status = %d, want 303", rec.Code)
	}
	if _, err := svc.Authenticate(context.Background(), cookie.Value); err == nil {
		t.Fatal("session is still valid after logout")
	}
}

func TestChangePassword_U7(t *testing.T) {
	t.Parallel()

	srv, svc := newTestServer(t)
	h := srv.Handler()
	cookie := setupInstance(t, h, "hunter2hunter2")
	sess, err := svc.Authenticate(context.Background(), cookie.Value)
	if err != nil {
		t.Fatalf("Authenticate(): %v", err)
	}

	form := url.Values{
		"csrf":     {sess.CSRFToken},
		"current":  {"wrong"},
		"password": {"newpassword1"},
		"confirm":  {"newpassword1"},
	}
	rec := request(t, h, http.MethodPost, "/change-password", form, cookie)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("wrong current password status = %d, want 400", rec.Code)
	}

	form.Set("current", "hunter2hunter2")
	rec = request(t, h, http.MethodPost, "/change-password", form, cookie)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("change password status = %d, want 303 (body %q)", rec.Code, rec.Body.String())
	}
	newCookie := sessionCookie(t, rec)

	if rec := request(t, h, http.MethodGet, "/", nil, cookie); rec.Code != http.StatusSeeOther {
		t.Error("old session still works after password change")
	}
	if rec := request(t, h, http.MethodGet, "/", nil, newCookie); rec.Code != http.StatusOK {
		t.Error("new session does not work after password change")
	}
	if rec := request(t, h, http.MethodPost, "/login", url.Values{"password": {"hunter2hunter2"}}, nil); rec.Code != http.StatusUnauthorized {
		t.Error("old password still works after change")
	}
	if rec := request(t, h, http.MethodPost, "/login", url.Values{"password": {"newpassword1"}}, nil); rec.Code != http.StatusSeeOther {
		t.Errorf("new password rejected: status %d", rec.Code)
	}
}

func TestCrossOriginPostRejected(t *testing.T) {
	t.Parallel()

	srv, _ := newTestServer(t)
	h := srv.Handler()

	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader("password=x"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "https://evil.example")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("cross-origin POST status = %d, want 403", rec.Code)
	}
}

func TestHealthz_ReturnsOK(t *testing.T) {
	t.Parallel()

	srv, _ := newTestServer(t)
	rec := request(t, srv.Handler(), http.MethodGet, "/healthz", nil, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if rec.Body.String() != "ok\n" {
		t.Errorf("body = %q, want %q", rec.Body.String(), "ok\n")
	}
	if got := rec.Header().Get("Content-Type"); got != "text/plain; charset=utf-8" {
		t.Errorf("Content-Type = %q", got)
	}
}

func TestHealthz_OnlyAllowsGET(t *testing.T) {
	t.Parallel()

	srv, _ := newTestServer(t)
	for _, method := range []string{http.MethodPost, http.MethodDelete} {
		rec := request(t, srv.Handler(), method, "/healthz", nil, nil)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s /healthz = %d, want 405", method, rec.Code)
		}
	}
}

func TestSecurityHeaders_AppliedToEveryResponse(t *testing.T) {
	t.Parallel()

	srv, _ := newTestServer(t)
	want := map[string]string{
		"Content-Security-Policy":    contentSecurityPolicy,
		"Cross-Origin-Opener-Policy": "same-origin",
		"Referrer-Policy":            "same-origin",
		"X-Content-Type-Options":     "nosniff",
		"X-Frame-Options":            "DENY",
	}
	for _, path := range []string{"/healthz", "/missing", "/login"} {
		rec := request(t, srv.Handler(), http.MethodGet, path, nil, nil)
		for header, value := range want {
			if got := rec.Header().Get(header); got != value {
				t.Errorf("%s: %s = %q, want %q", path, header, got, value)
			}
		}
	}
}

func TestStaticAssetServed(t *testing.T) {
	t.Parallel()

	srv, _ := newTestServer(t)
	rec := request(t, srv.Handler(), http.MethodGet, "/static/style.css", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /static/style.css = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/css") {
		t.Errorf("Content-Type = %q, want text/css", ct)
	}
	if rec.Body.Len() == 0 {
		t.Error("stylesheet is empty")
	}
}
