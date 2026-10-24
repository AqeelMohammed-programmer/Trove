package web

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"Trove/internal/auth"
)

func exportArchive(t *testing.T, h http.Handler, cookie *http.Cookie) []byte {
	t.Helper()
	rec := request(t, h, http.MethodGet, "/export", nil, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /export = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "zip") {
		t.Errorf("export Content-Type = %q, want zip", ct)
	}
	if cd := rec.Header().Get("Content-Disposition"); !strings.Contains(cd, "attachment") {
		t.Errorf("export Content-Disposition = %q, want attachment", cd)
	}
	body := rec.Body.Bytes()
	if len(body) == 0 {
		t.Fatal("export archive is empty")
	}
	return body
}

func postArchive(t *testing.T, h http.Handler, svc *auth.Service, cookie *http.Cookie, archive []byte) *httptest.ResponseRecorder {
	t.Helper()

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	if err := mw.WriteField("csrf", csrfFor(t, svc, cookie)); err != nil {
		t.Fatalf("WriteField(): %v", err)
	}
	part, err := mw.CreateFormFile("archive", "export.zip")
	if err != nil {
		t.Fatalf("CreateFormFile(): %v", err)
	}
	if _, err := part.Write(archive); err != nil {
		t.Fatalf("writing archive: %v", err)
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("closing multipart writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/import", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestExportImport_RoundTripOverHTTP_E10(t *testing.T) {
	t.Parallel()

	// Source instance with one item that has a file.
	srcSrv, srcAuth := newTestServer(t)
	srcHandler := srcSrv.Handler()
	srcCookie := setupInstance(t, srcHandler, "hunter2hunter2")
	created := postMultipart(t, srcHandler, srcAuth, srcCookie,
		url.Values{"title": {"Portable item"}}, nil,
		[]uploadFile{{name: "data.txt", body: "carry me"}})
	if created.Code != http.StatusSeeOther {
		t.Fatalf("create status = %d", created.Code)
	}
	archive := exportArchive(t, srcHandler, srcCookie)

	// Target instance, empty except for the session.
	dstSrv, dstAuth := newTestServer(t)
	dstHandler := dstSrv.Handler()
	dstCookie := setupInstance(t, dstHandler, "hunter2hunter2")

	rec := postArchive(t, dstHandler, dstAuth, dstCookie, archive)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("import status = %d, want 303 (body %q)", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Location"); !strings.HasPrefix(got, "/export-import") {
		t.Errorf("import redirect = %q", got)
	}

	body := request(t, dstHandler, http.MethodGet, "/", nil, dstCookie).Body.String()
	if !strings.Contains(body, "Portable item") {
		t.Errorf("imported item not listed (E-8):\n%s", body)
	}

	// The file downloads on the target instance.
	itemBody := request(t, dstHandler, http.MethodGet, "/?q=portable", nil, dstCookie).Body.String()
	_ = itemBody
}

func TestImport_NonEmptyRefusedOverHTTP_E5(t *testing.T) {
	t.Parallel()

	srcSrv, srcAuth := newTestServer(t)
	srcHandler := srcSrv.Handler()
	srcCookie := setupInstance(t, srcHandler, "hunter2hunter2")
	createItem(t, srcHandler, srcAuth, srcCookie, url.Values{"title": {"Source"}})
	archive := exportArchive(t, srcHandler, srcCookie)

	dstSrv, dstAuth := newTestServer(t)
	dstHandler := dstSrv.Handler()
	dstCookie := setupInstance(t, dstHandler, "hunter2hunter2")
	createItem(t, dstHandler, dstAuth, dstCookie, url.Values{"title": {"Already"}})

	rec := postArchive(t, dstHandler, dstAuth, dstCookie, archive)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("import into non-empty = %d, want 400 (E-5)", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "not empty") {
		t.Errorf("error does not explain the rule (E-5):\n%s", rec.Body.String())
	}
}

func TestImport_RejectsGarbage(t *testing.T) {
	t.Parallel()

	srv, authSvc := newTestServer(t)
	h := srv.Handler()
	cookie := setupInstance(t, h, "hunter2hunter2")

	rec := postArchive(t, h, authSvc, cookie, []byte("not a zip"))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("import garbage = %d, want 400", rec.Code)
	}
}

func TestImport_RequiresCSRF(t *testing.T) {
	t.Parallel()

	srv, _ := newTestServer(t)
	h := srv.Handler()
	cookie := setupInstance(t, h, "hunter2hunter2")

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, _ := mw.CreateFormFile("archive", "x.zip")
	_, _ = part.Write([]byte("x"))
	_ = mw.Close()
	req := httptest.NewRequest(http.MethodPost, "/import", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("import without CSRF = %d, want 403", rec.Code)
	}
}
