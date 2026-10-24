package web

import (
	"errors"
	"net"
	"net/http"
	"strings"

	"Trove/internal/auth"
)

// handleSetupPage shows first-run password setup (U-1).
func (s *Server) handleSetupPage(w http.ResponseWriter, r *http.Request) {
	configured, err := s.auth.Configured(r.Context())
	if err != nil {
		s.serverError(w, r, "checking password state", err)
		return
	}
	if configured {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	data := pageData(r)
	data["Title"] = "Set up"
	s.render(w, http.StatusOK, "setup", data)
}

// handleSetup stores the first password and starts a session (U-1, U-2).
func (s *Server) handleSetup(w http.ResponseWriter, r *http.Request) {
	configured, err := s.auth.Configured(r.Context())
	if err != nil {
		s.serverError(w, r, "checking password state", err)
		return
	}
	if configured {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if !s.parseForm(w, r) {
		return
	}
	password := r.PostFormValue("password")
	confirm := r.PostFormValue("confirm")

	creds, err := s.auth.Setup(r.Context(), password, confirm)
	if err != nil {
		data := pageData(r)
		data["Title"] = "Set up"
		data["Error"] = friendlyAuthError(err)
		s.render(w, http.StatusBadRequest, "setup", data)
		return
	}
	s.setSessionCookie(w, creds.Token)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// handleLoginPage shows the login form.
func (s *Server) handleLoginPage(w http.ResponseWriter, r *http.Request) {
	configured, err := s.auth.Configured(r.Context())
	if err != nil {
		s.serverError(w, r, "checking password state", err)
		return
	}
	if !configured {
		http.Redirect(w, r, "/setup", http.StatusSeeOther)
		return
	}
	data := pageData(r)
	data["Title"] = "Log in"
	s.render(w, http.StatusOK, "login", data)
}

// handleLogin verifies the password and starts a session (U-4, U-9).
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	configured, err := s.auth.Configured(r.Context())
	if err != nil {
		s.serverError(w, r, "checking password state", err)
		return
	}
	if !configured {
		http.Redirect(w, r, "/setup", http.StatusSeeOther)
		return
	}
	if !s.parseForm(w, r) {
		return
	}

	creds, err := s.auth.Login(r.Context(), clientIP(r), r.PostFormValue("password"))
	if err != nil {
		data := pageData(r)
		data["Title"] = "Log in"
		var blocked *auth.BlockedError
		if errors.As(err, &blocked) {
			data["Error"] = "Too many failed attempts. Try again in a few minutes."
			s.render(w, http.StatusTooManyRequests, "login", data)
			return
		}
		if errors.Is(err, auth.ErrWrongPassword) {
			data["Error"] = "Wrong password."
			s.render(w, http.StatusUnauthorized, "login", data)
			return
		}
		s.serverError(w, r, "logging in", err)
		return
	}
	s.setSessionCookie(w, creds.Token)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// handleHome is the placeholder start page until items arrive in M2.
func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	data := pageData(r)
	data["Title"] = "Home"
	if r.URL.Query().Get("changed") == "1" {
		data["Notice"] = "Password changed. Other sessions were signed out."
	}
	s.render(w, http.StatusOK, "home", data)
}

// handleLogout ends the session and clears the cookie (U-6).
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	sess, ok := sessionFromContext(r.Context())
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if !s.parseForm(w, r) {
		return
	}
	if !s.checkCSRF(w, r, sess) {
		return
	}
	if err := s.auth.Logout(r.Context(), rawToken(r)); err != nil {
		s.serverError(w, r, "logging out", err)
		return
	}
	s.clearSessionCookie(w)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// handleChangePasswordPage shows the change-password form (U-7).
func (s *Server) handleChangePasswordPage(w http.ResponseWriter, r *http.Request) {
	data := pageData(r)
	data["Title"] = "Change password"
	s.render(w, http.StatusOK, "change-password", data)
}

// handleChangePassword verifies the current password, stores the new one and
// rotates the session (U-7).
func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	sess, ok := sessionFromContext(r.Context())
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if !s.parseForm(w, r) {
		return
	}
	if !s.checkCSRF(w, r, sess) {
		return
	}

	creds, err := s.auth.ChangePassword(r.Context(), rawToken(r),
		r.PostFormValue("current"), r.PostFormValue("password"), r.PostFormValue("confirm"))
	if err != nil {
		data := pageData(r)
		data["Title"] = "Change password"
		if errors.Is(err, auth.ErrWrongPassword) {
			data["Error"] = "Current password is wrong."
		} else {
			data["Error"] = friendlyAuthError(err)
		}
		s.render(w, http.StatusBadRequest, "change-password", data)
		return
	}
	s.setSessionCookie(w, creds.Token)
	http.Redirect(w, r, "/?changed=1", http.StatusSeeOther)
}

// serverError logs the real error and shows a plain message, never internals.
func (s *Server) serverError(w http.ResponseWriter, r *http.Request, action string, err error) {
	s.logger.ErrorContext(r.Context(), action, "error", err)
	http.Error(w, "internal error", http.StatusInternalServerError)
}

// friendlyAuthError maps service errors to user-facing text. It never echoes
// the password.
func friendlyAuthError(err error) string {
	switch {
	case errors.Is(err, auth.ErrPasswordTooShort):
		return "Password must be at least 8 characters."
	case errors.Is(err, auth.ErrPasswordMismatch):
		return "The two passwords do not match."
	case errors.Is(err, auth.ErrNotConfigured):
		return "No password is set up yet."
	case errors.Is(err, auth.ErrAlreadyConfigured):
		return "A password is already set up."
	default:
		return "Something went wrong. Please try again."
	}
}

// clientIP extracts the peer address used for login throttling. Forwarded
// headers are deliberately ignored: they are user-controlled unless a trusted
// proxy is configured, which is a deployment concern.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return strings.TrimSpace(host)
}
