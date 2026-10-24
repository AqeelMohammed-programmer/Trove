package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"Trove/internal/auth"
)

// csrfFor looks up the CSRF token for a session cookie.
func csrfFor(t *testing.T, svc *auth.Service, cookie *http.Cookie) string {
	t.Helper()
	sess, err := svc.Authenticate(context.Background(), cookie.Value)
	if err != nil {
		t.Fatalf("Authenticate(): %v", err)
	}
	return sess.CSRFToken
}

func createItem(t *testing.T, h http.Handler, svc *auth.Service, cookie *http.Cookie, form url.Values) string {
	t.Helper()
	form.Set("csrf", csrfFor(t, svc, cookie))
	rec := request(t, h, http.MethodPost, "/items", form, cookie)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("POST /items status = %d, want 303 (body %q)", rec.Code, rec.Body.String())
	}
	location := rec.Header().Get("Location")
	if !strings.HasPrefix(location, "/items/") {
		t.Fatalf("Location = %q, want an item URL", location)
	}
	return strings.TrimPrefix(location, "/items/")
}

func TestCreateItemFlow_I1_I2_I4(t *testing.T) {
	t.Parallel()

	srv, svc := newTestServer(t)
	h := srv.Handler()
	cookie := setupInstance(t, h, "hunter2hunter2")

	// A note-only item with a title is valid (I-4).
	id := createItem(t, h, svc, cookie, url.Values{
		"title":       {"My note"},
		"description": {"A summary"},
		"note":        {"# Hello"},
		"tags":        {"reading, rust"},
	})

	rec := request(t, h, http.MethodGet, "/items/"+id, nil, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET item = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{"My note", "A summary", "reading", "rust", "<h1"} {
		if !strings.Contains(body, want) {
			t.Errorf("item page is missing %q", want)
		}
	}
}

func TestCreateItem_RejectsEmptyTitle_I2(t *testing.T) {
	t.Parallel()

	srv, svc := newTestServer(t)
	h := srv.Handler()
	cookie := setupInstance(t, h, "hunter2hunter2")

	rec := request(t, h, http.MethodPost, "/items", url.Values{
		"csrf":  {csrfFor(t, svc, cookie)},
		"title": {"   "},
	}, cookie)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("empty title status = %d, want 400", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "title") {
		t.Errorf("error message does not mention the title: %q", rec.Body.String())
	}
}

func TestCreateItem_RequiresCSRF(t *testing.T) {
	t.Parallel()

	srv, _ := newTestServer(t)
	h := srv.Handler()
	cookie := setupInstance(t, h, "hunter2hunter2")

	rec := request(t, h, http.MethodPost, "/items", url.Values{"title": {"x"}}, cookie)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("create without CSRF = %d, want 403", rec.Code)
	}
}

func TestEditItemFlow_I6(t *testing.T) {
	t.Parallel()

	srv, svc := newTestServer(t)
	h := srv.Handler()
	cookie := setupInstance(t, h, "hunter2hunter2")
	csrf := csrfFor(t, svc, cookie)
	id := createItem(t, h, svc, cookie, url.Values{"title": {"Original"}})

	// The edit form is prefilled.
	rec := request(t, h, http.MethodGet, "/items/"+id+"/edit", nil, cookie)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Original") {
		t.Fatalf("edit form = %d, does not contain the title", rec.Code)
	}

	rec = request(t, h, http.MethodPost, "/items/"+id, url.Values{
		"csrf":  {csrf},
		"title": {"Renamed"},
		"note":  {"updated note"},
	}, cookie)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("update status = %d, want 303 (body %q)", rec.Code, rec.Body.String())
	}

	rec = request(t, h, http.MethodGet, "/items/"+id, nil, cookie)
	if body := rec.Body.String(); !strings.Contains(body, "Renamed") || strings.Contains(body, "Original") {
		t.Errorf("item detail did not reflect the edit:\n%s", body)
	}
}

func TestFavoriteToggle_F1(t *testing.T) {
	t.Parallel()

	srv, svc := newTestServer(t)
	h := srv.Handler()
	cookie := setupInstance(t, h, "hunter2hunter2")
	csrf := csrfFor(t, svc, cookie)
	id := createItem(t, h, svc, cookie, url.Values{"title": {"Fav me"}})

	// Plain form (no HTMX) redirects back to the detail view.
	rec := request(t, h, http.MethodPost, "/items/"+id+"/favorite", url.Values{
		"csrf":     {csrf},
		"favorite": {"1"},
		"next":     {"/items/" + id},
	}, cookie)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("favorite status = %d, want 303", rec.Code)
	}

	// The favorites view now contains the item (F-2).
	rec = request(t, h, http.MethodGet, "/?favorites=1", nil, cookie)
	if !strings.Contains(rec.Body.String(), "Fav me") {
		t.Error("favorites view does not list the favorited item (F-2)")
	}

	// HTMX request returns just the button fragment.
	body := strings.NewReader(url.Values{
		"csrf": {csrf}, "favorite": {"0"}, "next": {"/items/" + id},
	}.Encode())
	req := httptest.NewRequest(http.MethodPost, "/items/"+id+"/favorite", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	req.AddCookie(cookie)
	htmx := httptest.NewRecorder()
	h.ServeHTTP(htmx, req)
	if htmx.Code != http.StatusOK {
		t.Fatalf("htmx favorite status = %d, want 200", htmx.Code)
	}
	if !strings.Contains(htmx.Body.String(), "favorite-form") {
		t.Errorf("htmx response is not the favorite fragment:\n%s", htmx.Body.String())
	}
}

func TestFavoriteToggle_RejectsCrossOriginNext(t *testing.T) {
	t.Parallel()

	srv, svc := newTestServer(t)
	h := srv.Handler()
	cookie := setupInstance(t, h, "hunter2hunter2")
	csrf := csrfFor(t, svc, cookie)
	id := createItem(t, h, svc, cookie, url.Values{"title": {"x"}})

	rec := request(t, h, http.MethodPost, "/items/"+id+"/favorite", url.Values{
		"csrf":     {csrf},
		"favorite": {"1"},
		"next":     {"https://evil.example/phish"},
	}, cookie)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rec.Code)
	}
	if got := rec.Header().Get("Location"); got != "/items/"+id {
		t.Errorf("open redirect: Location = %q, want /items/%s", got, id)
	}
}

func TestItemList_FiltersAndCount_B7(t *testing.T) {
	t.Parallel()

	srv, svc := newTestServer(t)
	h := srv.Handler()
	cookie := setupInstance(t, h, "hunter2hunter2")

	createItem(t, h, svc, cookie, url.Values{"title": {"First"}, "tags": {"reading"}})
	createItem(t, h, svc, cookie, url.Values{"title": {"Second"}, "tags": {"reading"}})

	rec := request(t, h, http.MethodGet, "/", nil, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET / = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "First") || !strings.Contains(rec.Body.String(), "Second") {
		t.Error("list does not show both items")
	}
	if !strings.Contains(rec.Body.String(), "2 items") {
		t.Errorf("list does not report the total (B-7):\n%s", rec.Body.String())
	}

	// Tag filter (B-3): a tag that matches nothing yields the empty state.
	rec = request(t, h, http.MethodGet, "/?tag=nonexistent", nil, cookie)
	if !strings.Contains(rec.Body.String(), "Nothing here yet") {
		t.Error("filtering by an unused tag does not show the empty state")
	}
}

func TestItemDetail_RawHTMLNeverRendered_V2(t *testing.T) {
	t.Parallel()

	srv, svc := newTestServer(t)
	h := srv.Handler()
	cookie := setupInstance(t, h, "hunter2hunter2")

	id := createItem(t, h, svc, cookie, url.Values{
		"title": {"XSS"},
		"note":  {"<script>alert('xss')</script> and <b>bold</b>"},
	})

	rec := request(t, h, http.MethodGet, "/items/"+id, nil, cookie)
	body := rec.Body.String()
	if strings.Contains(body, "<script>alert('xss')</script>") {
		t.Errorf("raw script tag was rendered (V-2):\n%s", body)
	}
	if strings.Contains(body, "<b>bold</b>") {
		t.Errorf("raw inline HTML was rendered (V-2):\n%s", body)
	}
}

func TestMissingItem_Returns404(t *testing.T) {
	t.Parallel()

	srv, _ := newTestServer(t)
	h := srv.Handler()
	cookie := setupInstance(t, h, "hunter2hunter2")

	if rec := request(t, h, http.MethodGet, "/items/does-not-exist", nil, cookie); rec.Code != http.StatusNotFound {
		t.Fatalf("GET missing item = %d, want 404", rec.Code)
	}
}

func TestStaticHTMXAssetServed(t *testing.T) {
	t.Parallel()

	srv, _ := newTestServer(t)
	rec := request(t, srv.Handler(), http.MethodGet, "/static/htmx.min.js", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /static/htmx.min.js = %d, want 200", rec.Code)
	}
	if rec.Body.Len() == 0 {
		t.Error("vendored htmx is empty")
	}
}
