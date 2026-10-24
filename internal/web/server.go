// Package web implements the HTTP interface: routing, middleware and, from
// milestone M2 onwards, template rendering. Handlers validate input, call a
// service and render a template; business rules live in internal/service.
package web

import (
	"io"
	"log/slog"
	"net/http"
	"time"
)

// contentSecurityPolicy forbids anything that is not served by this
// application: no inline scripts or styles, no third-party origins. It is
// strict from day one so that later additions (HTMX, styles) have to fit it.
const contentSecurityPolicy = "default-src 'self'; base-uri 'none'; " +
	"form-action 'self'; frame-ancestors 'none'; object-src 'none'"

// Options configures a Server.
type Options struct {
	// Logger receives request logs. Nil means slog.Default().
	Logger *slog.Logger
}

// Server is the application's HTTP handler. Create it with New and expose it
// through Handler.
type Server struct {
	logger *slog.Logger
	mux    *http.ServeMux
}

// New builds a Server with all routes registered.
func New(opts Options) *Server {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	s := &Server{
		logger: logger,
		mux:    http.NewServeMux(),
	}
	s.routes()
	return s
}

// routes registers every endpoint. Method-and-path patterns give automatic
// 405 responses for the wrong method. Only /healthz exists at M0; it is the
// one route that is allowed to work without a session (AGENTS.md section 7).
// The route-protection test arrives with authentication (M1).
func (s *Server) routes() {
	s.mux.HandleFunc("GET /healthz", s.handleHealthz)
}

// Handler returns the server wrapped in the outer middleware.
func (s *Server) Handler() http.Handler {
	return s.logRequests(securityHeaders(s.mux))
}

// handleHealthz reports liveness and nothing else: no data, no version.
func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	if _, err := io.WriteString(w, "ok\n"); err != nil {
		s.logger.WarnContext(r.Context(), "writing health response", "error", err)
	}
}

// securityHeaders applies the response headers required by AGENTS.md
// section 7 on every response, including errors.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", contentSecurityPolicy)
		h.Set("Cross-Origin-Opener-Policy", "same-origin")
		h.Set("Referrer-Policy", "same-origin")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}

// logRequests writes one structured log line per request.
func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)

		s.logger.InfoContext(r.Context(), "request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"bytes", rec.bytes,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	})
}

// statusRecorder remembers the status code and byte count for logging. It
// implements Unwrap so http.ResponseController keeps working.
type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	n, err := r.ResponseWriter.Write(b)
	r.bytes += n
	return n, err
}

func (r *statusRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}
