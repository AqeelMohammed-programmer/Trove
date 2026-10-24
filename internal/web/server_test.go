package web

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestHandler() http.Handler {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return New(Options{Logger: logger}).Handler()
}

func TestHealthz_ReturnsOK(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	newTestHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	res := rec.Result()
	defer res.Body.Close() //nolint:errcheck // test response body

	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("reading body: %v", err)
	}
	if string(body) != "ok\n" {
		t.Errorf("body = %q, want %q", body, "ok\n")
	}
	if got := res.Header.Get("Content-Type"); got != "text/plain; charset=utf-8" {
		t.Errorf("Content-Type = %q, want text/plain; charset=utf-8", got)
	}
	if got := res.Header.Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", got)
	}
}

func TestHealthz_OnlyAllowsGET(t *testing.T) {
	t.Parallel()

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch} {
		t.Run(method, func(t *testing.T) {
			t.Parallel()

			rec := httptest.NewRecorder()
			newTestHandler().ServeHTTP(rec, httptest.NewRequest(method, "/healthz", nil))

			if rec.Code != http.StatusMethodNotAllowed {
				t.Errorf("%s /healthz status = %d, want 405", method, rec.Code)
			}
			if allow := rec.Header().Get("Allow"); !strings.Contains(allow, http.MethodGet) {
				t.Errorf("%s /healthz Allow = %q, want it to contain GET", method, allow)
			}
		})
	}
}

func TestUnknownRoute_NotFound(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	newTestHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET / status = %d, want 404 (no UI before M2)", rec.Code)
	}
}

func TestSecurityHeaders_AppliedToEveryResponse(t *testing.T) {
	t.Parallel()

	want := map[string]string{
		"Content-Security-Policy":    contentSecurityPolicy,
		"Cross-Origin-Opener-Policy": "same-origin",
		"Referrer-Policy":            "same-origin",
		"X-Content-Type-Options":     "nosniff",
		"X-Frame-Options":            "DENY",
	}
	for _, path := range []string{"/healthz", "/missing"} {
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			rec := httptest.NewRecorder()
			newTestHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))

			for header, value := range want {
				if got := rec.Header().Get(header); got != value {
					t.Errorf("%s: %s = %q, want %q", path, header, got, value)
				}
			}
		})
	}
}

func TestHealthz_RevealsNoData(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	newTestHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	body := rec.Body.String()
	for _, forbidden := range []string{"Trove", "version", "commit", "data"} {
		if strings.Contains(strings.ToLower(body), strings.ToLower(forbidden)) {
			t.Errorf("health body %q leaks %q", body, forbidden)
		}
	}
}
