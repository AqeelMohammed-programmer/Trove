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

// multipartItem builds a multipart form with fields, links and files.
type uploadFile struct {
	name string
	body string
}

func postMultipart(t *testing.T, h http.Handler, svc *auth.Service, cookie *http.Cookie, fields url.Values, links []uploadFile, files []uploadFile) *httptest.ResponseRecorder {
	t.Helper()

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fields.Set("csrf", csrfFor(t, svc, cookie))
	for key, values := range fields {
		for _, value := range values {
			if err := mw.WriteField(key, value); err != nil {
				t.Fatalf("WriteField(): %v", err)
			}
		}
	}
	for _, link := range links {
		if err := mw.WriteField("link_url", link.name); err != nil {
			t.Fatalf("WriteField(): %v", err)
		}
		if err := mw.WriteField("link_label", link.body); err != nil {
			t.Fatalf("WriteField(): %v", err)
		}
	}
	for _, file := range files {
		part, err := mw.CreateFormFile("files", file.name)
		if err != nil {
			t.Fatalf("CreateFormFile(): %v", err)
		}
		if _, err := part.Write([]byte(file.body)); err != nil {
			t.Fatalf("writing file part: %v", err)
		}
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("closing multipart writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/items", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestCreateItemWithAttachments_A1_A3_A4_A5_A6(t *testing.T) {
	t.Parallel()

	srv, svc := newTestServer(t)
	h := srv.Handler()
	cookie := setupInstance(t, h, "hunter2hunter2")

	rec := postMultipart(t, h, svc, cookie,
		url.Values{"title": {"With attachments"}},
		[]uploadFile{{name: "https://example.org/article", body: "source"}},
		[]uploadFile{{name: "one.txt", body: "first file"}, {name: "two.txt", body: "second file"}})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303 (body %q)", rec.Code, rec.Body.String())
	}
	id := strings.TrimPrefix(rec.Header().Get("Location"), "/items/")

	detail := request(t, h, http.MethodGet, "/items/"+id, nil, cookie)
	body := detail.Body.String()
	for _, want := range []string{"https://example.org/article", "one.txt", "two.txt", "source"} {
		if !strings.Contains(body, want) {
			t.Errorf("detail page is missing %q", want)
		}
	}
}

func TestCreateItem_DefaultsTitleToFirstAttachment_I2(t *testing.T) {
	t.Parallel()

	srv, svc := newTestServer(t)
	h := srv.Handler()
	cookie := setupInstance(t, h, "hunter2hunter2")

	rec := postMultipart(t, h, svc, cookie,
		url.Values{"title": {""}},
		[]uploadFile{{name: "https://example.org/first", body: ""}},
		nil)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303 (body %q)", rec.Code, rec.Body.String())
	}
	id := strings.TrimPrefix(rec.Header().Get("Location"), "/items/")
	detail := request(t, h, http.MethodGet, "/items/"+id, nil, cookie)
	if !strings.Contains(detail.Body.String(), "<h1>https://example.org/first</h1>") {
		t.Errorf("title was not defaulted to the link URL (I-2):\n%s", detail.Body.String())
	}
}

func TestCreateItem_RejectsBadScheme_A2(t *testing.T) {
	t.Parallel()

	srv, svc := newTestServer(t)
	h := srv.Handler()
	cookie := setupInstance(t, h, "hunter2hunter2")

	rec := postMultipart(t, h, svc, cookie,
		url.Values{"title": {"Bad link"}},
		[]uploadFile{{name: "javascript:alert(1)", body: ""}},
		nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "http") {
		t.Errorf("error does not explain the allowed schemes: %q", rec.Body.String())
	}
}

func TestDownloadAttachment_A7_A9(t *testing.T) {
	t.Parallel()

	srv, svc := newTestServer(t)
	h := srv.Handler()
	cookie := setupInstance(t, h, "hunter2hunter2")

	rec := postMultipart(t, h, svc, cookie,
		url.Values{"title": {"File item"}},
		nil,
		[]uploadFile{{name: "report.txt", body: "download me"}})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("create status = %d, want 303", rec.Code)
	}
	id := strings.TrimPrefix(rec.Header().Get("Location"), "/items/")

	detail := request(t, h, http.MethodGet, "/items/"+id, nil, cookie)
	downloadURL := extractDownloadURL(t, detail.Body.String())

	dl := request(t, h, http.MethodGet, downloadURL, nil, cookie)
	if dl.Code != http.StatusOK {
		t.Fatalf("download status = %d, want 200", dl.Code)
	}
	if got := dl.Body.String(); got != "download me" {
		t.Errorf("download body = %q, want %q", got, "download me")
	}
	if got := dl.Header().Get("Content-Disposition"); !strings.Contains(got, "attachment") || !strings.Contains(got, "report.txt") {
		t.Errorf("Content-Disposition = %q, want attachment with report.txt", got)
	}
	if got := dl.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want nosniff", got)
	}
}

func TestDownload_RequiresSession_U3(t *testing.T) {
	t.Parallel()

	srv, _ := newTestServer(t)
	h := srv.Handler()

	rec := request(t, h, http.MethodGet, "/attachments/whatever/download", nil, nil)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/login" {
		t.Fatalf("unauthenticated download = %d %q, want 303 /login", rec.Code, rec.Header().Get("Location"))
	}
}

func TestDownload_MissingIsNotFound(t *testing.T) {
	t.Parallel()

	srv, _ := newTestServer(t)
	h := srv.Handler()
	cookie := setupInstance(t, h, "hunter2hunter2")

	if rec := request(t, h, http.MethodGet, "/attachments/does-not-exist/download", nil, cookie); rec.Code != http.StatusNotFound {
		t.Fatalf("missing download = %d, want 404", rec.Code)
	}
}

// extractDownloadURL pulls the download link out of an item detail page.
func extractDownloadURL(t *testing.T, body string) string {
	t.Helper()
	const marker = "/attachments/"
	idx := strings.Index(body, marker)
	if idx < 0 {
		t.Fatalf("no download link in page:\n%s", body)
	}
	rest := body[idx:]
	end := strings.Index(rest, "/download")
	if end < 0 {
		t.Fatalf("malformed download link in page")
	}
	return rest[:end+len("/download")]
}
