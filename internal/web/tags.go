package web

import (
	"encoding/json"
	"net/http"
	"strings"

	"Trove/internal/domain"
)

// tagRow is one tag on the management screen (M-1).
type tagRow struct {
	ID    string
	Name  string
	Count int
}

// handleTags renders the tag management screen (M-1). The default order is
// alphabetical; ?sort=count orders by usage.
func (s *Server) handleTags(w http.ResponseWriter, r *http.Request) {
	order := r.URL.Query().Get("sort")
	if order != "count" {
		order = "name"
	}
	s.renderTags(w, r, order, http.StatusOK, "")
}

// renderTags loads and renders the management screen with an optional error.
func (s *Server) renderTags(w http.ResponseWriter, r *http.Request, order string, status int, message string) {
	tags, err := s.items.TagsForManagement(r.Context(), order)
	if err != nil {
		s.serverError(w, r, "listing tags", err)
		return
	}
	data, err := s.itemPageData(r.Context(), r)
	if err != nil {
		s.serverError(w, r, "loading tags", err)
		return
	}

	rows := make([]tagRow, 0, len(tags))
	for _, tag := range tags {
		rows = append(rows, tagRow{ID: tag.ID, Name: tag.Name, Count: tag.Count})
	}

	data["Title"] = "Tags"
	data["Nav"] = "tags"
	data["Tags"] = rows
	data["CountSort"] = order == "count"
	data["Empty"] = len(rows) == 0
	if message != "" {
		data["Error"] = message
	}
	s.render(w, status, "tags", data)
}

// handleRenameTag renames a tag everywhere (M-2).
func (s *Server) handleRenameTag(w http.ResponseWriter, r *http.Request) {
	if !s.postCSRF(w, r) {
		return
	}
	if err := s.items.RenameTag(r.Context(), r.PathValue("id"), r.PostFormValue("name")); err != nil {
		if isValidationError(err) {
			s.renderTags(w, r, "name", http.StatusBadRequest, validationMessage(err))
			return
		}
		s.itemError(w, r, "renaming tag", err)
		return
	}
	http.Redirect(w, r, "/tags", http.StatusSeeOther)
}

// handleMergeTag merges the tag in the path into the target chosen in the form
// (M-3).
func (s *Server) handleMergeTag(w http.ResponseWriter, r *http.Request) {
	if !s.postCSRF(w, r) {
		return
	}
	if err := s.items.MergeTags(r.Context(), r.PathValue("id"), r.PostFormValue("target")); err != nil {
		if isValidationError(err) {
			s.renderTags(w, r, "name", http.StatusBadRequest, validationMessage(err))
			return
		}
		s.itemError(w, r, "merging tags", err)
		return
	}
	http.Redirect(w, r, "/tags", http.StatusSeeOther)
}

// tagSuggestResponse is the JSON body of the tag autocomplete endpoint (T-2).
type tagSuggestResponse struct {
	Tags []string `json:"tags"`
}

// handleTagSuggest returns existing tag names that start with ?q, for the
// autocomplete on the item form (T-2). Matching and the active-only rule live
// in the service, so the browser never reimplements them. An empty query
// returns no suggestions rather than the whole list.
func (s *Server) handleTagSuggest(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	// A prefix longer than the longest allowed tag name cannot match one.
	if runes := []rune(query); len(runes) > domain.MaxTagName {
		query = string(runes[:domain.MaxTagName])
	}

	names := []string{}
	if query != "" {
		var err error
		names, err = s.items.SuggestTags(r.Context(), query)
		if err != nil {
			s.serverError(w, r, "suggesting tags", err)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(tagSuggestResponse{Tags: names}); err != nil {
		s.logger.WarnContext(r.Context(), "writing tag suggestions", "error", err)
	}
}

// handleDeleteTag removes a tag from every item (M-4). The screen has already
// shown how many items are affected and asked for confirmation.
func (s *Server) handleDeleteTag(w http.ResponseWriter, r *http.Request) {
	if !s.postCSRF(w, r) {
		return
	}
	if _, err := s.items.DeleteTag(r.Context(), r.PathValue("id")); err != nil {
		s.itemError(w, r, "deleting tag", err)
		return
	}
	http.Redirect(w, r, "/tags", http.StatusSeeOther)
}
