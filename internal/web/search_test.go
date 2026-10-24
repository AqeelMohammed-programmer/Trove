package web

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestSearchFlow_S2_S5(t *testing.T) {
	t.Parallel()

	srv, svc := newTestServer(t)
	h := srv.Handler()
	cookie := setupInstance(t, h, "hunter2hunter2")

	createItem(t, h, svc, cookie, url.Values{"title": {"Rust book"}})
	createItem(t, h, svc, cookie, url.Values{"title": {"Go book"}})

	rec := request(t, h, http.MethodGet, "/?q=rust", nil, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /?q=rust = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Rust book") {
		t.Errorf("search did not return the matching item (S-2):\n%s", body)
	}
	if strings.Contains(body, "Go book") {
		t.Errorf("search returned a non-matching item (S-2):\n%s", body)
	}
	if !strings.Contains(body, "1 item") {
		t.Errorf("search result count is wrong (B-7):\n%s", body)
	}
}

func TestSearch_DefaultsToRelevance_SearchBoxPresent(t *testing.T) {
	t.Parallel()

	srv, svc := newTestServer(t)
	h := srv.Handler()
	cookie := setupInstance(t, h, "hunter2hunter2")
	createItem(t, h, svc, cookie, url.Values{"title": {"Anything"}})

	rec := request(t, h, http.MethodGet, "/?q=any", nil, cookie)
	body := rec.Body.String()
	if !strings.Contains(body, `value="any"`) {
		t.Errorf("the search box does not keep the query:\n%s", body)
	}
	if !strings.Contains(body, "Relevance") {
		t.Errorf("relevance is not offered as a sort option while searching (B-2):\n%s", body)
	}
}

func TestSearch_CombinesWithFilters_S5(t *testing.T) {
	t.Parallel()

	srv, svc := newTestServer(t)
	h := srv.Handler()
	cookie := setupInstance(t, h, "hunter2hunter2")

	createItem(t, h, svc, cookie, url.Values{"title": {"Rust alpha"}, "tags": {"reading"}})
	createItem(t, h, svc, cookie, url.Values{"title": {"Rust beta"}, "tags": {"work"}})

	// Search plus a tag filter must apply both.
	rec := request(t, h, http.MethodGet, "/?q=rust&tag=reading", nil, cookie)
	body := rec.Body.String()
	if !strings.Contains(body, "Rust alpha") || strings.Contains(body, "Rust beta") {
		t.Errorf("search+filters did not combine with AND (S-5):\n%s", body)
	}
}
