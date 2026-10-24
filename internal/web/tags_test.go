package web

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestTagsScreen_M1(t *testing.T) {
	t.Parallel()

	srv, authSvc := newTestServer(t)
	h := srv.Handler()
	cookie := setupInstance(t, h, "hunter2hunter2")

	createItem(t, h, authSvc, cookie, url.Values{"title": {"A"}, "tags": {"reading, rust"}})
	createItem(t, h, authSvc, cookie, url.Values{"title": {"B"}, "tags": {"reading"}})

	body := request(t, h, http.MethodGet, "/tags", nil, cookie).Body.String()
	for _, want := range []string{"reading", "rust", "Rename", "Merge", "Delete"} {
		if !strings.Contains(body, want) {
			t.Errorf("tags screen is missing %q:\n%s", want, body)
		}
	}
	// reading is used by two items.
	if !strings.Contains(body, ">2<") {
		t.Errorf("tag count is not shown (M-1):\n%s", body)
	}
	// Count ordering is offered.
	if !strings.Contains(body, "sort=count") {
		t.Errorf("count sort option missing (M-1):\n%s", body)
	}
}

// TestTagSuggest_T2 covers the autocomplete endpoint: it returns active tags
// that start with the query, in name order, and returns nothing for an empty
// query instead of the whole list.
func TestTagSuggest_T2(t *testing.T) {
	t.Parallel()

	srv, authSvc := newTestServer(t)
	h := srv.Handler()
	cookie := setupInstance(t, h, "hunter2hunter2")

	createItem(t, h, authSvc, cookie, url.Values{"title": {"A"}, "tags": {"reading, rust"}})
	createItem(t, h, authSvc, cookie, url.Values{"title": {"B"}, "tags": {"react"}})

	rec := request(t, h, http.MethodGet, "/tags/suggest?q=rea", nil, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("suggest status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", got)
	}
	var body struct {
		Tags []string `json:"tags"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decoding suggestions: %v (body %q)", err, rec.Body.String())
	}
	if got := strings.Join(body.Tags, ","); got != "react,reading" {
		t.Errorf("suggestions for %q = %v, want [react reading]", "rea", body.Tags)
	}

	// An empty query returns nothing, not the whole tag list.
	empty := request(t, h, http.MethodGet, "/tags/suggest?q=", nil, cookie)
	if strings.Contains(empty.Body.String(), "reading") {
		t.Errorf("empty query returned suggestions: %s", empty.Body.String())
	}
}

// TestItemForm_TagAutocomplete_T2 checks the form wires the tag input to the
// suggestion endpoint and keeps the datalist as a no-JavaScript fallback.
func TestItemForm_TagAutocomplete_T2(t *testing.T) {
	t.Parallel()

	srv, _ := newTestServer(t)
	h := srv.Handler()
	cookie := setupInstance(t, h, "hunter2hunter2")

	body := request(t, h, http.MethodGet, "/items/new", nil, cookie).Body.String()
	if !strings.Contains(body, `data-tag-suggest="/tags/suggest"`) {
		t.Errorf("tag input is not wired to the suggestion endpoint (T-2):\n%s", body)
	}
	if !strings.Contains(body, `<datalist id="tag-options">`) {
		t.Errorf("tag input lost its no-JavaScript datalist fallback (T-2):\n%s", body)
	}
}

func TestRenameTagFlow_M2(t *testing.T) {
	t.Parallel()

	srv, authSvc := newTestServer(t)
	h := srv.Handler()
	cookie := setupInstance(t, h, "hunter2hunter2")
	csrf := csrfFor(t, authSvc, cookie)
	createItem(t, h, authSvc, cookie, url.Values{"title": {"A"}, "tags": {"reading"}})

	id := tagID(t, h, cookie, "reading")
	rec := request(t, h, http.MethodPost, "/tags/"+id+"/rename", url.Values{"csrf": {csrf}, "name": {"books"}}, cookie)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("rename status = %d, want 303 (body %q)", rec.Code, rec.Body.String())
	}
	body := request(t, h, http.MethodGet, "/?tag=books", nil, cookie).Body.String()
	if !strings.Contains(body, "A") {
		t.Errorf("renamed tag filter does not find the item:\n%s", body)
	}
}

func TestRenameTag_ConflictSuggestsMerge_M2(t *testing.T) {
	t.Parallel()

	srv, authSvc := newTestServer(t)
	h := srv.Handler()
	cookie := setupInstance(t, h, "hunter2hunter2")
	csrf := csrfFor(t, authSvc, cookie)
	createItem(t, h, authSvc, cookie, url.Values{"title": {"A"}, "tags": {"books"}})
	createItem(t, h, authSvc, cookie, url.Values{"title": {"B"}, "tags": {"reading"}})

	id := tagID(t, h, cookie, "reading")
	rec := request(t, h, http.MethodPost, "/tags/"+id+"/rename", url.Values{"csrf": {csrf}, "name": {"BOOKS"}}, cookie)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("conflicting rename status = %d, want 400", rec.Code)
	}
	if !strings.Contains(strings.ToLower(rec.Body.String()), "merge") {
		t.Errorf("conflict message does not suggest a merge (M-2):\n%s", rec.Body.String())
	}
}

func TestMergeTagFlow_M3(t *testing.T) {
	t.Parallel()

	srv, authSvc := newTestServer(t)
	h := srv.Handler()
	cookie := setupInstance(t, h, "hunter2hunter2")
	csrf := csrfFor(t, authSvc, cookie)
	createItem(t, h, authSvc, cookie, url.Values{"title": {"A"}, "tags": {"books"}})
	createItem(t, h, authSvc, cookie, url.Values{"title": {"B"}, "tags": {"reading"}})

	source := tagID(t, h, cookie, "reading")
	target := tagID(t, h, cookie, "books")
	rec := request(t, h, http.MethodPost, "/tags/"+source+"/merge", url.Values{"csrf": {csrf}, "target": {target}}, cookie)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("merge status = %d, want 303 (body %q)", rec.Code, rec.Body.String())
	}
	// Only one tag remains, covering both items.
	body := request(t, h, http.MethodGet, "/tags", nil, cookie).Body.String()
	if strings.Contains(body, "reading") {
		t.Errorf("source tag survived the merge (M-3):\n%s", body)
	}
	if !strings.Contains(body, ">2<") {
		t.Errorf("merged tag does not count both items (M-3):\n%s", body)
	}
}

func TestDeleteTagFlow_M4(t *testing.T) {
	t.Parallel()

	srv, authSvc := newTestServer(t)
	h := srv.Handler()
	cookie := setupInstance(t, h, "hunter2hunter2")
	csrf := csrfFor(t, authSvc, cookie)
	createItem(t, h, authSvc, cookie, url.Values{"title": {"A"}, "tags": {"doomed"}})

	id := tagID(t, h, cookie, "doomed")
	rec := request(t, h, http.MethodPost, "/tags/"+id+"/delete", url.Values{"csrf": {csrf}}, cookie)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("delete status = %d, want 303", rec.Code)
	}
	// The item remains (M-4).
	if body := request(t, h, http.MethodGet, "/", nil, cookie).Body.String(); !strings.Contains(body, "A") {
		t.Error("tag delete removed the item (M-4)")
	}
	if body := request(t, h, http.MethodGet, "/tags", nil, cookie).Body.String(); strings.Contains(body, "doomed") {
		t.Error("tag still listed after delete (M-4)")
	}
}

func TestTagActions_RequireCSRF(t *testing.T) {
	t.Parallel()

	srv, authSvc := newTestServer(t)
	h := srv.Handler()
	cookie := setupInstance(t, h, "hunter2hunter2")
	createItem(t, h, authSvc, cookie, url.Values{"title": {"A"}, "tags": {"t"}})
	id := tagID(t, h, cookie, "t")

	rec := request(t, h, http.MethodPost, "/tags/"+id+"/delete", url.Values{}, cookie)
	if rec.Code != http.StatusForbidden {
		t.Errorf("tag delete without CSRF = %d, want 403", rec.Code)
	}
}

// tagID finds a tag's id on the management screen by its name.
func tagID(t *testing.T, h http.Handler, cookie *http.Cookie, name string) string {
	t.Helper()
	body := request(t, h, http.MethodGet, "/tags", nil, cookie).Body.String()
	anchor := `<a href="/?tag=` + name + `">` + name + `</a>`
	at := strings.Index(body, anchor)
	if at < 0 {
		t.Fatalf("tag %q not found on the management screen:\n%s", name, body)
	}
	rest := body[at:]
	const marker = "/tags/"
	idx := strings.Index(rest, marker)
	if idx < 0 {
		t.Fatalf("no action link after tag %q", name)
	}
	rest = rest[idx+len(marker):]
	end := strings.Index(rest, "/")
	if end < 0 {
		t.Fatalf("malformed action link for tag %q", name)
	}
	return rest[:end]
}
