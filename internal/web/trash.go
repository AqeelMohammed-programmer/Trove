package web

import (
	"net/http"

	"Trove/internal/domain"
)

// trashedItem is one trashed item with all of its attachments (X-3).
type trashedItem struct {
	Item        domain.Item
	Deleted     string
	Attachments []attachmentView
}

// attachmentView is one attachment rendered in a detail or trash view.
type attachmentView struct {
	ID        string
	Kind      string
	Label     string
	URL       string
	FileName  string
	SizeBytes int64
	// Trashed is true for attachments that are not active.
	Trashed bool
	Deleted string
}

// trashedAttachment is an individually trashed attachment of an active item.
type trashedAttachment struct {
	Attachment attachmentView
	ItemID     string
	ItemTitle  string
}

// handleTrash renders the trash view (X-3).
func (s *Server) handleTrash(w http.ResponseWriter, r *http.Request) {
	list, err := s.items.Trash(r.Context())
	if err != nil {
		s.serverError(w, r, "loading trash", err)
		return
	}
	data, err := s.itemPageData(r.Context(), r)
	if err != nil {
		s.serverError(w, r, "loading tags", err)
		return
	}

	items := make([]trashedItem, 0, len(list.Items))
	for _, item := range list.Items {
		view := trashedItem{Item: item}
		if item.TrashedAt != nil {
			view.Deleted = item.TrashedAt.Format("2 Jan 2006")
		}
		for _, a := range item.Attachments {
			view.Attachments = append(view.Attachments, attachmentToView(a))
		}
		items = append(items, view)
	}
	attachments := make([]trashedAttachment, 0, len(list.Attachments))
	for _, a := range list.Attachments {
		attachments = append(attachments, trashedAttachment{
			Attachment: attachmentToView(a.Attachment),
			ItemID:     a.ItemID,
			ItemTitle:  a.ItemTitle,
		})
	}

	data["Title"] = "Trash"
	data["Nav"] = "trash"
	data["Items"] = items
	data["Attachments"] = attachments
	data["Empty"] = len(items) == 0 && len(attachments) == 0
	s.render(w, http.StatusOK, "trash", data)
}

// handleTrashItem moves an item to the trash (I-7, X-1, X-13).
func (s *Server) handleTrashItem(w http.ResponseWriter, r *http.Request) {
	if !s.postCSRF(w, r) {
		return
	}
	if err := s.items.TrashItem(r.Context(), r.PathValue("id")); err != nil {
		s.itemError(w, r, "trashing item", err)
		return
	}
	http.Redirect(w, r, localPath(r.PostFormValue("next"), "/"), http.StatusSeeOther)
}

// handleRestoreItem brings a trashed item back (X-4).
func (s *Server) handleRestoreItem(w http.ResponseWriter, r *http.Request) {
	if !s.postCSRF(w, r) {
		return
	}
	if err := s.items.RestoreItem(r.Context(), r.PathValue("id")); err != nil {
		s.itemError(w, r, "restoring item", err)
		return
	}
	http.Redirect(w, r, "/trash", http.StatusSeeOther)
}

// handleDeleteItem permanently deletes a trashed item (X-6).
func (s *Server) handleDeleteItem(w http.ResponseWriter, r *http.Request) {
	if !s.postCSRF(w, r) {
		return
	}
	if err := s.items.DeleteItemPermanent(r.Context(), r.PathValue("id")); err != nil {
		s.itemError(w, r, "deleting item", err)
		return
	}
	http.Redirect(w, r, "/trash", http.StatusSeeOther)
}

// handleTrashAttachment moves one attachment to the trash (A-9, X-2).
func (s *Server) handleTrashAttachment(w http.ResponseWriter, r *http.Request) {
	if !s.postCSRF(w, r) {
		return
	}
	if err := s.items.TrashAttachment(r.Context(), r.PathValue("id")); err != nil {
		s.itemError(w, r, "trashing attachment", err)
		return
	}
	http.Redirect(w, r, localPath(r.PostFormValue("next"), "/"), http.StatusSeeOther)
}

// handleRestoreAttachment brings an individually trashed attachment back (X-5).
func (s *Server) handleRestoreAttachment(w http.ResponseWriter, r *http.Request) {
	if !s.postCSRF(w, r) {
		return
	}
	if err := s.items.RestoreAttachment(r.Context(), r.PathValue("id")); err != nil {
		s.itemError(w, r, "restoring attachment", err)
		return
	}
	http.Redirect(w, r, "/trash", http.StatusSeeOther)
}

// handleDeleteAttachment permanently deletes a trashed attachment (X-7).
func (s *Server) handleDeleteAttachment(w http.ResponseWriter, r *http.Request) {
	if !s.postCSRF(w, r) {
		return
	}
	if err := s.items.DeleteAttachmentPermanent(r.Context(), r.PathValue("id")); err != nil {
		s.itemError(w, r, "deleting attachment", err)
		return
	}
	http.Redirect(w, r, "/trash", http.StatusSeeOther)
}

// handleEmptyTrash permanently deletes everything in the trash (X-8).
func (s *Server) handleEmptyTrash(w http.ResponseWriter, r *http.Request) {
	if !s.postCSRF(w, r) {
		return
	}
	if err := s.items.EmptyTrash(r.Context()); err != nil {
		s.serverError(w, r, "emptying trash", err)
		return
	}
	http.Redirect(w, r, "/trash", http.StatusSeeOther)
}

// postCSRF parses a form and validates its CSRF token for a state-changing
// action. It reports false after writing a response.
func (s *Server) postCSRF(w http.ResponseWriter, r *http.Request) bool {
	sess, ok := sessionFromContext(r.Context())
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return false
	}
	if !s.parseForm(w, r) {
		return false
	}
	return s.checkCSRF(w, r, sess)
}

// attachmentToView converts a domain attachment for rendering.
func attachmentToView(a domain.Attachment) attachmentView {
	view := attachmentView{
		ID:        a.ID,
		Kind:      string(a.Kind),
		Label:     a.Label,
		URL:       a.URL,
		FileName:  a.FileName,
		SizeBytes: a.SizeBytes,
		Trashed:   a.State != domain.StateActive,
	}
	if a.TrashedAt != nil {
		view.Deleted = a.TrashedAt.Format("2 Jan 2006")
	}
	return view
}

// duplicateGroupView is one duplicate report group (R-2, R-4).
type duplicateGroupView struct {
	Kind    string
	Key     string
	Entries []duplicateEntryView
}

type duplicateEntryView struct {
	Kind        string
	Label       string
	URL         string
	FileName    string
	ItemID      string
	ItemTitle   string
	ItemTrashed bool
	Trashed     bool
}

// handleDuplicates renders the duplicate report (R-1..R-6).
func (s *Server) handleDuplicates(w http.ResponseWriter, r *http.Request) {
	groups, err := s.items.Duplicates(r.Context())
	if err != nil {
		s.serverError(w, r, "building duplicate report", err)
		return
	}
	data, err := s.itemPageData(r.Context(), r)
	if err != nil {
		s.serverError(w, r, "loading tags", err)
		return
	}

	views := make([]duplicateGroupView, 0, len(groups))
	for _, group := range groups {
		view := duplicateGroupView{Kind: string(group.Kind), Key: group.Key}
		for _, entry := range group.Entries {
			view.Entries = append(view.Entries, duplicateEntryView{
				Kind:        string(entry.Attachment.Kind),
				Label:       entry.Attachment.Label,
				URL:         entry.Attachment.URL,
				FileName:    entry.Attachment.FileName,
				ItemID:      entry.ItemID,
				ItemTitle:   entry.ItemTitle,
				ItemTrashed: entry.ItemTrashed,
				Trashed:     entry.ItemTrashed || entry.AttachmentTrashed,
			})
		}
		views = append(views, view)
	}

	data["Title"] = "Duplicates"
	data["Nav"] = "duplicates"
	data["Groups"] = views
	data["Empty"] = len(views) == 0
	s.render(w, http.StatusOK, "duplicates", data)
}
