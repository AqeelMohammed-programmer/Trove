package web

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestTrashItemFlow_X13_X1_X4(t *testing.T) {
	t.Parallel()

	srv, authSvc := newTestServer(t)
	h := srv.Handler()
	cookie := setupInstance(t, h, "hunter2hunter2")
	csrf := csrfFor(t, authSvc, cookie)
	id := createItem(t, h, authSvc, cookie, url.Values{"title": {"Disposable"}})

	// Moving to trash needs no confirmation; the form just posts (X-13).
	rec := request(t, h, http.MethodPost, "/items/"+id+"/trash", url.Values{"csrf": {csrf}}, cookie)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("trash status = %d, want 303 (body %q)", rec.Code, rec.Body.String())
	}

	// Gone from the normal list (B-6), present in the trash (X-1).
	if body := request(t, h, http.MethodGet, "/", nil, cookie).Body.String(); strings.Contains(body, "Disposable") {
		t.Error("trashed item still appears in the list (B-6)")
	}
	trashBody := request(t, h, http.MethodGet, "/trash", nil, cookie).Body.String()
	if !strings.Contains(trashBody, "Disposable") {
		t.Errorf("trashed item missing from the trash view (X-1):\n%s", trashBody)
	}

	// Restore brings it back (X-4).
	if rec := request(t, h, http.MethodPost, "/items/"+id+"/restore", url.Values{"csrf": {csrf}}, cookie); rec.Code != http.StatusSeeOther {
		t.Fatalf("restore status = %d, want 303", rec.Code)
	}
	if body := request(t, h, http.MethodGet, "/", nil, cookie).Body.String(); !strings.Contains(body, "Disposable") {
		t.Error("restored item is not back in the list (X-4)")
	}
}

func TestPermanentDelete_X6(t *testing.T) {
	t.Parallel()

	srv, authSvc := newTestServer(t)
	h := srv.Handler()
	cookie := setupInstance(t, h, "hunter2hunter2")
	csrf := csrfFor(t, authSvc, cookie)
	id := createItem(t, h, authSvc, cookie, url.Values{"title": {"Goner"}})

	if rec := request(t, h, http.MethodPost, "/items/"+id+"/trash", url.Values{"csrf": {csrf}}, cookie); rec.Code != http.StatusSeeOther {
		t.Fatalf("trash status = %d", rec.Code)
	}
	if rec := request(t, h, http.MethodPost, "/items/"+id+"/delete", url.Values{"csrf": {csrf}}, cookie); rec.Code != http.StatusSeeOther {
		t.Fatalf("delete status = %d", rec.Code)
	}
	if rec := request(t, h, http.MethodGet, "/items/"+id, nil, cookie); rec.Code != http.StatusNotFound {
		t.Errorf("permanently deleted item is still reachable: %d (X-6)", rec.Code)
	}
}

func TestEmptyTrash_X8(t *testing.T) {
	t.Parallel()

	srv, authSvc := newTestServer(t)
	h := srv.Handler()
	cookie := setupInstance(t, h, "hunter2hunter2")
	csrf := csrfFor(t, authSvc, cookie)
	id := createItem(t, h, authSvc, cookie, url.Values{"title": {"Junk"}})

	if rec := request(t, h, http.MethodPost, "/items/"+id+"/trash", url.Values{"csrf": {csrf}}, cookie); rec.Code != http.StatusSeeOther {
		t.Fatalf("trash status = %d", rec.Code)
	}
	if rec := request(t, h, http.MethodPost, "/trash/empty", url.Values{"csrf": {csrf}}, cookie); rec.Code != http.StatusSeeOther {
		t.Fatalf("empty trash status = %d", rec.Code)
	}
	body := request(t, h, http.MethodGet, "/trash", nil, cookie).Body.String()
	if !strings.Contains(body, "trash is empty") {
		t.Errorf("trash not empty after Empty trash (X-8):\n%s", body)
	}
}

func TestTrashAttachment_A9_X2_X5(t *testing.T) {
	t.Parallel()

	srv, authSvc := newTestServer(t)
	h := srv.Handler()
	cookie := setupInstance(t, h, "hunter2hunter2")
	csrf := csrfFor(t, authSvc, cookie)

	rec := postMultipart(t, h, authSvc, cookie,
		url.Values{"title": {"Has a file"}}, nil,
		[]uploadFile{{name: "f.txt", body: "data"}})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("create status = %d", rec.Code)
	}
	itemID := strings.TrimPrefix(rec.Header().Get("Location"), "/items/")
	downloadURL := extractDownloadURL(t, request(t, h, http.MethodGet, "/items/"+itemID, nil, cookie).Body.String())
	attachmentID := strings.TrimSuffix(strings.TrimPrefix(downloadURL, "/attachments/"), "/download")

	// Remove (A-9, X-2).
	if rec := request(t, h, http.MethodPost, "/attachments/"+attachmentID+"/trash", url.Values{"csrf": {csrf}, "next": {"/items/" + itemID}}, cookie); rec.Code != http.StatusSeeOther {
		t.Fatalf("attachment trash status = %d", rec.Code)
	}
	if download := request(t, h, http.MethodGet, downloadURL, nil, cookie); download.Code != http.StatusNotFound {
		t.Errorf("trashed attachment still downloads: %d", download.Code)
	}
	trashBody := request(t, h, http.MethodGet, "/trash", nil, cookie).Body.String()
	if !strings.Contains(trashBody, "f.txt") {
		t.Errorf("individually trashed attachment missing from trash (X-3):\n%s", trashBody)
	}

	// Restore (X-5).
	if rec := request(t, h, http.MethodPost, "/attachments/"+attachmentID+"/restore", url.Values{"csrf": {csrf}}, cookie); rec.Code != http.StatusSeeOther {
		t.Fatalf("attachment restore status = %d", rec.Code)
	}
	if download := request(t, h, http.MethodGet, downloadURL, nil, cookie); download.Code != http.StatusOK {
		t.Errorf("restored attachment does not download: %d (X-5)", download.Code)
	}
}

func TestTrashActions_RequireCSRF(t *testing.T) {
	t.Parallel()

	srv, authSvc := newTestServer(t)
	h := srv.Handler()
	cookie := setupInstance(t, h, "hunter2hunter2")
	id := createItem(t, h, authSvc, cookie, url.Values{"title": {"x"}})

	for _, path := range []string{"/items/" + id + "/trash", "/trash/empty"} {
		rec := request(t, h, http.MethodPost, path, url.Values{}, cookie)
		if rec.Code != http.StatusForbidden {
			t.Errorf("POST %s without CSRF = %d, want 403", path, rec.Code)
		}
	}
}
