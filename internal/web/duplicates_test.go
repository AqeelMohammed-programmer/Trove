package web

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestItemDetail_DuplicateWarning_D1_D4_D5(t *testing.T) {
	t.Parallel()

	srv, authSvc := newTestServer(t)
	h := srv.Handler()
	cookie := setupInstance(t, h, "hunter2hunter2")
	csrf := csrfFor(t, authSvc, cookie)

	first := postMultipart(t, h, authSvc, cookie,
		url.Values{"title": {"First"}},
		[]uploadFile{{name: "https://example.org/shared", body: ""}}, nil)
	firstID := strings.TrimPrefix(first.Header().Get("Location"), "/items/")

	second := postMultipart(t, h, authSvc, cookie,
		url.Values{"title": {"Second"}},
		[]uploadFile{{name: "https://example.org/shared", body: ""}}, nil)
	secondID := strings.TrimPrefix(second.Header().Get("Location"), "/items/")

	body := request(t, h, http.MethodGet, "/items/"+secondID, nil, cookie).Body.String()
	if !strings.Contains(body, "Possible duplicates") {
		t.Fatalf("no duplicate warning on the detail page (D-1):\n%s", body)
	}
	if !strings.Contains(body, "First") || !strings.Contains(body, "/items/"+firstID) {
		t.Errorf("warning does not link to the matching item (D-1):\n%s", body)
	}

	// Trash the first item; the warning must mark it and offer a restore (D-5).
	if rec := request(t, h, http.MethodPost, "/items/"+firstID+"/trash", url.Values{"csrf": {csrf}}, cookie); rec.Code != http.StatusSeeOther {
		t.Fatalf("trash status = %d", rec.Code)
	}
	body = request(t, h, http.MethodGet, "/items/"+secondID, nil, cookie).Body.String()
	if !strings.Contains(body, "trashed") || !strings.Contains(body, "Restore item") {
		t.Errorf("trashed duplicate is not marked with a restore action (D-5):\n%s", body)
	}
}

func TestItemDetail_FileDuplicateWarning_D2(t *testing.T) {
	t.Parallel()

	srv, authSvc := newTestServer(t)
	h := srv.Handler()
	cookie := setupInstance(t, h, "hunter2hunter2")

	postMultipart(t, h, authSvc, cookie, url.Values{"title": {"A"}}, nil, []uploadFile{{name: "a.bin", body: "same bytes"}})
	second := postMultipart(t, h, authSvc, cookie, url.Values{"title": {"B"}}, nil, []uploadFile{{name: "b.bin", body: "same bytes"}})
	secondID := strings.TrimPrefix(second.Header().Get("Location"), "/items/")

	body := request(t, h, http.MethodGet, "/items/"+secondID, nil, cookie).Body.String()
	if !strings.Contains(body, "Possible duplicates") {
		t.Errorf("no SHA-256 duplicate warning (D-2):\n%s", body)
	}
}

func TestDuplicateReportPage_R1_R4_R6(t *testing.T) {
	t.Parallel()

	srv, authSvc := newTestServer(t)
	h := srv.Handler()
	cookie := setupInstance(t, h, "hunter2hunter2")

	// No duplicates yet (R-6).
	body := request(t, h, http.MethodGet, "/duplicates", nil, cookie).Body.String()
	if !strings.Contains(body, "No duplicates found") {
		t.Errorf("empty report does not say so (R-6):\n%s", body)
	}

	postMultipart(t, h, authSvc, cookie, url.Values{"title": {"One"}}, []uploadFile{{name: "https://dup.example/x", body: ""}}, nil)
	postMultipart(t, h, authSvc, cookie, url.Values{"title": {"Two"}}, []uploadFile{{name: "https://dup.example/x#frag", body: ""}}, nil)

	body = request(t, h, http.MethodGet, "/duplicates", nil, cookie).Body.String()
	if strings.Contains(body, "No duplicates found") {
		t.Errorf("report still says no duplicates after adding a duplicate:\n%s", body)
	}
	if !strings.Contains(body, "One") || !strings.Contains(body, "Two") {
		t.Errorf("report does not list both owning items (R-4):\n%s", body)
	}
	if !strings.Contains(body, "https://dup.example/x") {
		t.Errorf("report does not show the matching key (R-2):\n%s", body)
	}
}
