// Package web implements the HTTP interface: routing, middleware and template
// rendering. Handlers validate input, call a service and render a template;
// business rules live in the service packages.
package web

import (
	"context"
	"embed"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"Trove/internal/app"
	"Trove/internal/auth"
	"Trove/internal/service"
)

// contentSecurityPolicy forbids anything that is not served by this
// application: no inline scripts or styles, no third-party origins.
const contentSecurityPolicy = "default-src 'self'; base-uri 'none'; " +
	"form-action 'self'; frame-ancestors 'none'; object-src 'none'; " +
	"img-src 'self'; style-src 'self'; script-src 'self'"

// maxFormBytes caps ordinary form bodies. File uploads stream separately and
// are not subject to this cap (spec N-1, milestone M3).
const maxFormBytes = 1 << 20

// Options configures a Server.
type Options struct {
	// Logger receives request and error logs. Nil means slog.Default().
	Logger *slog.Logger
	// Auth is the authentication service. Required.
	Auth *auth.Service
	// Items is the item and tag service. Required.
	Items *service.ItemService
	// SecureCookie marks the session cookie Secure (HTTPS deployments).
	SecureCookie bool
}

// Server is the application's HTTP handler.
type Server struct {
	logger       *slog.Logger
	auth         *auth.Service
	items        *service.ItemService
	markdown     *Markdown
	secureCookie bool
	mux          *http.ServeMux
	templates    map[string]*template.Template
	registered   []route
}

// route is one registered endpoint. Tests use the list to prove that every
// non-public route rejects unauthenticated requests (U-3).
type route struct {
	method  string
	pattern string
	sample  string
	public  bool
}

// New builds a Server with all routes registered.
func New(opts Options) *Server {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	if opts.Auth == nil {
		panic("web: Options.Auth is required")
	}
	if opts.Items == nil {
		panic("web: Options.Items is required")
	}
	s := &Server{
		logger:       logger,
		auth:         opts.Auth,
		items:        opts.Items,
		markdown:     NewMarkdown(),
		secureCookie: opts.SecureCookie,
		mux:          http.NewServeMux(),
		templates:    mustParseTemplates(),
	}
	s.routes()
	return s
}

// routes registers every endpoint. Public routes are limited to health,
// static assets, first-run setup and login (AGENTS.md section 7).
func (s *Server) routes() {
	s.handle("GET", "/healthz", "/healthz", true, s.handleHealthz)

	staticFS, err := fs.Sub(assets, "static")
	if err != nil {
		panic("web: static assets are missing: " + err.Error())
	}
	files := http.StripPrefix("/static/", http.FileServerFS(staticFS))
	s.handle("GET", "/static/", "/static/style.css", true, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=300")
		files.ServeHTTP(w, r)
	})

	s.handle("GET", "/setup", "/setup", true, s.handleSetupPage)
	s.handle("POST", "/setup", "/setup", true, s.handleSetup)
	s.handle("GET", "/login", "/login", true, s.handleLoginPage)
	s.handle("POST", "/login", "/login", true, s.handleLogin)

	s.handle("GET", "/", "/", false, s.authenticated(s.handleList))
	s.handle("GET", "/items/new", "/items/new", false, s.authenticated(s.handleNewItem))
	s.handle("POST", "/items", "/items", false, s.authenticated(s.handleCreateItem))
	s.handle("GET", "/items/{id}", "/items/example", false, s.authenticated(s.handleShowItem))
	s.handle("GET", "/items/{id}/edit", "/items/example/edit", false, s.authenticated(s.handleEditItem))
	s.handle("POST", "/items/{id}", "/items/example", false, s.authenticated(s.handleUpdateItem))
	s.handle("POST", "/items/{id}/favorite", "/items/example/favorite", false, s.authenticated(s.handleFavorite))
	s.handle("GET", "/attachments/{id}/download", "/attachments/example/download", false, s.authenticated(s.handleDownload))
	s.handle("GET", "/form/link-row", "/form/link-row", false, s.authenticated(s.handleLinkRow))
	s.handle("GET", "/trash", "/trash", false, s.authenticated(s.handleTrash))
	s.handle("POST", "/trash/empty", "/trash/empty", false, s.authenticated(s.handleEmptyTrash))
	s.handle("POST", "/items/{id}/trash", "/items/example/trash", false, s.authenticated(s.handleTrashItem))
	s.handle("POST", "/items/{id}/restore", "/items/example/restore", false, s.authenticated(s.handleRestoreItem))
	s.handle("POST", "/items/{id}/delete", "/items/example/delete", false, s.authenticated(s.handleDeleteItem))
	s.handle("POST", "/attachments/{id}/trash", "/attachments/example/trash", false, s.authenticated(s.handleTrashAttachment))
	s.handle("POST", "/attachments/{id}/restore", "/attachments/example/restore", false, s.authenticated(s.handleRestoreAttachment))
	s.handle("POST", "/attachments/{id}/delete", "/attachments/example/delete", false, s.authenticated(s.handleDeleteAttachment))
	s.handle("POST", "/logout", "/logout", false, s.authenticated(s.handleLogout))
	s.handle("GET", "/change-password", "/change-password", false, s.authenticated(s.handleChangePasswordPage))
	s.handle("POST", "/change-password", "/change-password", false, s.authenticated(s.handleChangePassword))
}

func (s *Server) handle(method, pattern, sample string, public bool, h http.HandlerFunc) {
	s.mux.Handle(method+" "+pattern, h)
	s.registered = append(s.registered, route{method: method, pattern: pattern, sample: sample, public: public})
}

// Handler returns the server wrapped in the outer middleware.
func (s *Server) Handler() http.Handler {
	return s.logRequests(securityHeaders(sameOriginOnly(s.mux)))
}

// RegisteredRoutes exposes the registered routes to tests in other packages.
func (s *Server) RegisteredRoutes() []RouteInfo {
	out := make([]RouteInfo, len(s.registered))
	for i, r := range s.registered {
		out[i] = RouteInfo{Method: r.method, Pattern: r.pattern, Sample: r.sample, Public: r.public}
	}
	return out
}

// RouteInfo is a read-only description of a registered route.
type RouteInfo struct {
	Method  string
	Pattern string
	Sample  string
	Public  bool
}

// handleHealthz reports liveness and nothing else: no data, no version.
func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte("ok\n")); err != nil {
		s.logger.WarnContext(r.Context(), "writing health response", "error", err)
	}
}

// render executes the layout for a page. Callers pass the page name without
// the .html suffix.
func (s *Server) render(w http.ResponseWriter, status int, page string, data map[string]any) {
	t, ok := s.templates[page]
	if !ok {
		s.logger.Error("rendering unknown page", "page", page)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if data == nil {
		data = map[string]any{}
	}
	data["App"] = app.Name
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := t.ExecuteTemplate(w, "layout", data); err != nil {
		s.logger.Error("rendering page", "page", page, "error", err)
	}
}

// renderFragment executes a named template without the layout. It is used for
// HTMX responses such as the favorite button.
func (s *Server) renderFragment(w http.ResponseWriter, name string, data any) {
	t, ok := s.templates["list"]
	if !ok {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if err := t.ExecuteTemplate(w, name, data); err != nil {
		s.logger.Error("rendering fragment", "template", name, "error", err)
	}
}

// mustParseTemplates parses every page together with the layout and shared
// partials. A failure is a programming error found at startup, not in a
// request.
func mustParseTemplates() map[string]*template.Template {
	pages := []string{"list", "item", "item-form", "trash", "login", "setup", "change-password"}
	out := make(map[string]*template.Template, len(pages))
	for _, page := range pages {
		t, err := template.ParseFS(assets,
			"templates/layout.html",
			"templates/partials.html",
			"templates/"+page+".html")
		if err != nil {
			panic("web: parsing template " + page + ": " + err.Error())
		}
		out[page] = t
	}
	return out
}

type sessionContextKey struct{}

// authenticated requires a live session and puts it in the request context.
func (s *Server) authenticated(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sess, ok := s.currentSession(r)
		if !ok {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		// Authenticated pages contain private data and must not sit in a
		// shared cache.
		w.Header().Set("Cache-Control", "no-store")
		next(w, r.WithContext(context.WithValue(r.Context(), sessionContextKey{}, sess)))
	}
}

func (s *Server) currentSession(r *http.Request) (auth.Session, bool) {
	cookie, err := r.Cookie(auth.CookieName)
	if err != nil || cookie.Value == "" {
		return auth.Session{}, false
	}
	sess, err := s.auth.Authenticate(r.Context(), cookie.Value)
	if err != nil {
		return auth.Session{}, false
	}
	return sess, true
}

func sessionFromContext(ctx context.Context) (auth.Session, bool) {
	sess, ok := ctx.Value(sessionContextKey{}).(auth.Session)
	return sess, ok
}

// pageData seeds template data with the session and CSRF token when present.
func pageData(r *http.Request) map[string]any {
	data := map[string]any{"Session": false, "CSRF": "", "Title": "", "Nav": "", "Search": ""}
	if sess, ok := sessionFromContext(r.Context()); ok {
		data["Session"] = true
		data["CSRF"] = sess.CSRFToken
	}
	return data
}

// setSessionCookie writes the session cookie. It has no Expires or Max-Age on
// purpose: the session ends with the browser (U-5).
func (s *Server) setSessionCookie(w http.ResponseWriter, token string) {
	// #nosec G124 -- Secure is configuration-driven; HttpOnly and SameSite are always set.
	http.SetCookie(w, &http.Cookie{
		Name:     auth.CookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   s.secureCookie,
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *Server) clearSessionCookie(w http.ResponseWriter) {
	// #nosec G124 -- Secure is configuration-driven; HttpOnly and SameSite are always set.
	http.SetCookie(w, &http.Cookie{
		Name:     auth.CookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   s.secureCookie,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

// rawToken returns the session token from the request cookie.
func rawToken(r *http.Request) string {
	cookie, err := r.Cookie(auth.CookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}

// parseForm reads a form with a small body cap. It writes a 400 response and
// reports false on failure.
func (s *Server) parseForm(w http.ResponseWriter, r *http.Request) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxFormBytes)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "form too large or malformed", http.StatusBadRequest)
		return false
	}
	return true
}

// checkCSRF validates the form token against the session and writes 403 on
// failure.
func (s *Server) checkCSRF(w http.ResponseWriter, r *http.Request, sess auth.Session) bool {
	if !auth.ValidateCSRF(sess, r.PostFormValue("csrf")) {
		http.Error(w, "invalid form token", http.StatusForbidden)
		return false
	}
	return true
}

// sameOriginOnly rejects state-changing requests coming from another origin.
// It complements the per-form CSRF token.
func sameOriginOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" {
			u, err := url.Parse(origin)
			if err != nil || !strings.EqualFold(u.Host, r.Host) {
				http.Error(w, "cross-origin request rejected", http.StatusForbidden)
				return
			}
		}
		if site := r.Header.Get("Sec-Fetch-Site"); site != "" &&
			site != "same-origin" && site != "none" {
			http.Error(w, "cross-site request rejected", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
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

// assets holds templates and vendored static files.
//
//go:embed templates/*.html static/*
var assets embed.FS
