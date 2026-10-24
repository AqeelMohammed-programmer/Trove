package web

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"Trove/internal/domain"
	"Trove/internal/service"
)

// descSnippetLength is how much of the description a list row shows.
const descSnippetLength = 140

// sortOption is one entry of the sort dropdown (B-2).
type sortOption struct {
	Value    string
	Label    string
	Selected bool
}

// tagChip is a tag link or a removable active-filter chip.
type tagChip struct {
	Name string
	URL  string
}

// sidebarTag is one entry of the sidebar tag list, with its active item count
// (T-6, spec 4.1).
type sidebarTag struct {
	Name  string
	URL   string
	Count int
}

// favoriteView feeds the favorite toggle partial. It works as a plain form
// without JavaScript and as an HTMX request when JavaScript is available.
type favoriteView struct {
	ID       string
	Favorite bool
	CSRF     string
	Next     string
}

// itemRow is a list row (spec 4.2).
type itemRow struct {
	ID         string
	Title      string
	Snippet    string
	Tags       []tagChip
	Favorite   favoriteView
	Links      int
	Files      int
	Updated    string
	UpdatedISO string
}

// pagination describes the current page and its neighbours (B-7).
type pagination struct {
	Page     int
	PageSize int
	Total    int
	From     int
	To       int
	HasPrev  bool
	HasNext  bool
	PrevURL  string
	NextURL  string
}

// itemFormView feeds the create and edit form (spec 4.4).
type itemFormView struct {
	Action      string
	Heading     string
	TitleValue  string
	DescValue   string
	NoteValue   string
	TagsValue   string
	Favorite    bool
	CancelURL   string
	SubmitLabel string
	AllTags     []domain.TagCount
	// Attachments are the item's current active attachments, shown read-only
	// on the edit form (M5 adds removal).
	Attachments []domain.Attachment
}

// itemPageData starts from the common page data and adds what the sidebar
// needs. Auth pages do not call this.
func (s *Server) itemPageData(ctx context.Context, r *http.Request) (map[string]any, error) {
	data := pageData(r)
	tags, err := s.items.Tags(ctx)
	if err != nil {
		return nil, err
	}
	chips := make([]sidebarTag, 0, len(tags))
	for _, tag := range tags {
		chips = append(chips, sidebarTag{
			Name:  tag.Name,
			URL:   tagFilterURL(r.URL.Query(), tag.Name),
			Count: tag.Count,
		})
	}
	data["SidebarTags"] = chips
	data["AllTags"] = tags
	data["Nav"] = "all"
	return data, nil
}

// handleList renders the browse view with filters, sorting and paging
// (B-1..B-7).
func (s *Server) handleList(w http.ResponseWriter, r *http.Request) {
	filter := parseListFilter(r)

	result, err := s.items.List(r.Context(), filter)
	if err != nil {
		s.serverError(w, r, "listing items", err)
		return
	}
	data, err := s.itemPageData(r.Context(), r)
	if err != nil {
		s.serverError(w, r, "loading tags", err)
		return
	}

	rows := make([]itemRow, 0, len(result.Items))
	for _, item := range result.Items {
		rows = append(rows, s.itemRow(r, item))
	}

	title := "All items"
	if filter.FavoritesOnly {
		title = "Favorites"
		data["Nav"] = "favorites"
	}
	data["Title"] = title
	data["Rows"] = rows
	data["Total"] = result.Total
	data["Empty"] = result.Total == 0
	data["SortOptions"] = sortOptions(filter.Sort)
	data["FavoritesOnly"] = filter.FavoritesOnly
	data["HasLink"] = filter.HasLink
	data["HasFile"] = filter.HasFile
	data["SelectedTags"] = selectedTagChips(r.URL.Query(), filter.Tags)
	data["Page"] = paginationFor(filter, result.Total)
	data["ClearURL"] = "/"
	if r.URL.Query().Get("changed") == "1" {
		data["Notice"] = "Password changed. Other sessions were signed out."
	}

	s.render(w, http.StatusOK, "list", data)
}

// handleNewItem shows the empty add form (I-1).
func (s *Server) handleNewItem(w http.ResponseWriter, r *http.Request) {
	data, err := s.itemPageData(r.Context(), r)
	if err != nil {
		s.serverError(w, r, "loading tags", err)
		return
	}
	data["Title"] = "Add item"
	data["Form"] = itemFormView{
		Action:      "/items",
		Heading:     "Add item",
		CancelURL:   "/",
		SubmitLabel: "Save item",
		AllTags:     tagsFromData(data),
	}
	s.render(w, http.StatusOK, "item-form", data)
}

// handleCreateItem validates and stores a new item (I-1, I-2).
func (s *Server) handleCreateItem(w http.ResponseWriter, r *http.Request) {
	sess, ok := sessionFromContext(r.Context())
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	in, ok := s.parseItemInput(w, r)
	if !ok {
		return
	}
	defer in.Close()
	if !s.checkCSRF(w, r, sess) {
		return
	}

	item, err := s.items.Create(r.Context(), in.input)
	if err != nil {
		s.renderItemFormError(w, r, err, itemFormView{
			Action:      "/items",
			Heading:     "Add item",
			TitleValue:  in.input.Title,
			DescValue:   in.input.Description,
			NoteValue:   in.input.Note,
			TagsValue:   strings.Join(in.input.Tags, ", "),
			Favorite:    in.input.Favorite,
			CancelURL:   "/",
			SubmitLabel: "Save item",
		})
		return
	}
	http.Redirect(w, r, "/items/"+item.ID, http.StatusSeeOther)
}

// handleShowItem renders one item with its Markdown note (spec 4.3).
func (s *Server) handleShowItem(w http.ResponseWriter, r *http.Request) {
	item, err := s.items.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		s.itemError(w, r, "loading item", err)
		return
	}

	noteHTML, err := s.markdown.Render(item.Note)
	if err != nil {
		s.serverError(w, r, "rendering note", err)
		return
	}

	data, dataErr := s.itemPageData(r.Context(), r)
	if dataErr != nil {
		s.serverError(w, r, "loading tags", dataErr)
		return
	}
	data["Title"] = item.Title
	data["Item"] = item
	data["NoteHTML"] = noteHTML
	data["Tags"] = detailTagChips(item.Tags)
	data["Favorite"] = s.favoriteView(r, item)
	data["Created"] = item.CreatedAt.Format("2 Jan 2006")
	data["Updated"] = item.UpdatedAt.Format("2 Jan 2006")
	if item.TrashedAt != nil {
		data["Trashed"] = item.TrashedAt.Format("2 Jan 2006")
	}
	s.render(w, http.StatusOK, "item", data)
}

// handleEditItem shows the edit form for an existing item (I-6).
func (s *Server) handleEditItem(w http.ResponseWriter, r *http.Request) {
	item, err := s.items.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		s.itemError(w, r, "loading item", err)
		return
	}
	if item.TrashedAt != nil {
		http.NotFound(w, r)
		return
	}

	data, dataErr := s.itemPageData(r.Context(), r)
	if dataErr != nil {
		s.serverError(w, r, "loading tags", dataErr)
		return
	}
	data["Title"] = "Edit " + item.Title
	data["Form"] = itemFormView{
		Action:      "/items/" + item.ID,
		Heading:     "Edit item",
		TitleValue:  item.Title,
		DescValue:   item.Description,
		NoteValue:   item.Note,
		TagsValue:   strings.Join(tagNames(item.Tags), ", "),
		Favorite:    item.Favorite,
		CancelURL:   "/items/" + item.ID,
		SubmitLabel: "Save changes",
		AllTags:     tagsFromData(data),
		Attachments: item.Attachments,
	}
	s.render(w, http.StatusOK, "item-form", data)
}

// handleUpdateItem saves edits to an existing item (I-5, I-6).
func (s *Server) handleUpdateItem(w http.ResponseWriter, r *http.Request) {
	sess, ok := sessionFromContext(r.Context())
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	id := r.PathValue("id")
	in, ok := s.parseItemInput(w, r)
	if !ok {
		return
	}
	defer in.Close()
	if !s.checkCSRF(w, r, sess) {
		return
	}

	item, err := s.items.Update(r.Context(), id, in.input)
	if err != nil {
		existing, getErr := s.items.Get(r.Context(), id)
		heading := "Edit item"
		if getErr == nil {
			heading = "Edit " + existing.Title
		}
		s.renderItemFormError(w, r, err, itemFormView{
			Action:      "/items/" + id,
			Heading:     heading,
			TitleValue:  in.input.Title,
			DescValue:   in.input.Description,
			NoteValue:   in.input.Note,
			TagsValue:   strings.Join(in.input.Tags, ", "),
			Favorite:    in.input.Favorite,
			CancelURL:   "/items/" + id,
			SubmitLabel: "Save changes",
		})
		return
	}
	http.Redirect(w, r, "/items/"+item.ID, http.StatusSeeOther)
}

// handleFavorite toggles the favorite flag (F-1). It deliberately does not
// touch updated_at (I-5). With HTMX it returns just the new button; without
// JavaScript the form redirects back to the same view.
func (s *Server) handleFavorite(w http.ResponseWriter, r *http.Request) {
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

	id := r.PathValue("id")
	favorite := r.PostFormValue("favorite") == "1"
	if err := s.items.SetFavorite(r.Context(), id, favorite); err != nil {
		s.itemError(w, r, "setting favorite", err)
		return
	}

	next := localPath(r.PostFormValue("next"), "/items/"+id)
	if r.Header.Get("HX-Request") == "true" {
		s.renderFragment(w, "favorite-form", favoriteView{
			ID:       id,
			Favorite: favorite,
			CSRF:     sess.CSRFToken,
			Next:     next,
		})
		return
	}
	http.Redirect(w, r, next, http.StatusSeeOther)
}

// itemRow builds the display model for one list row.
func (s *Server) itemRow(r *http.Request, item domain.Item) itemRow {
	return itemRow{
		ID:         item.ID,
		Title:      item.Title,
		Snippet:    snippet(item.Description, descSnippetLength),
		Tags:       detailTagChips(item.Tags),
		Favorite:   s.favoriteView(r, item),
		Links:      item.Counts.Links,
		Files:      item.Counts.Files,
		Updated:    item.UpdatedAt.Format("2 Jan 2006"),
		UpdatedISO: item.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

func (s *Server) favoriteView(r *http.Request, item domain.Item) favoriteView {
	sess, _ := sessionFromContext(r.Context())
	return favoriteView{
		ID:       item.ID,
		Favorite: item.Favorite,
		CSRF:     sess.CSRFToken,
		Next:     r.URL.RequestURI(),
	}
}

// renderItemFormError shows a validation message next to the form.
func (s *Server) renderItemFormError(w http.ResponseWriter, r *http.Request, err error, form itemFormView) {
	if errors.Is(err, domain.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if !isValidationError(err) {
		s.serverError(w, r, "saving item", err)
		return
	}

	data, dataErr := s.itemPageData(r.Context(), r)
	if dataErr != nil {
		s.serverError(w, r, "loading tags", dataErr)
		return
	}
	form.AllTags = tagsFromData(data)
	data["Title"] = form.Heading
	data["Form"] = form
	data["Error"] = validationMessage(err)
	s.render(w, http.StatusBadRequest, "item-form", data)
}

func (s *Server) itemError(w http.ResponseWriter, r *http.Request, action string, err error) {
	if errors.Is(err, domain.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	s.serverError(w, r, action, err)
}

// parseListFilter reads the URL state. Filter state lives in the query string
// so views can be bookmarked (AGENTS.md section 11).
func parseListFilter(r *http.Request) domain.ListFilter {
	q := r.URL.Query()
	filter := domain.ListFilter{
		Tags:          cleanQueryValues(q["tag"]),
		FavoritesOnly: q.Get("favorites") == "1",
		HasLink:       q.Get("link") == "1",
		HasFile:       q.Get("file") == "1",
		Sort:          domain.SortOrder(q.Get("sort")),
	}
	if !filter.Sort.IsValid() {
		filter.Sort = domain.SortNewest
	}
	if page, err := strconv.Atoi(q.Get("page")); err == nil && page > 0 {
		filter.Page = page
	}
	return filter
}

// sortOptions returns the dropdown entries, marking the active one (B-2).
func sortOptions(current domain.SortOrder) []sortOption {
	return []sortOption{
		{Value: string(domain.SortNewest), Label: "Newest", Selected: current == domain.SortNewest},
		{Value: string(domain.SortUpdated), Label: "Recently edited", Selected: current == domain.SortUpdated},
		{Value: string(domain.SortTitle), Label: "Title", Selected: current == domain.SortTitle},
	}
}

// paginationFor computes the display range and neighbouring page links.
func paginationFor(filter domain.ListFilter, total int) pagination {
	page := filter.Page
	if page < 1 {
		page = 1
	}
	pageSize := filter.PageSize
	if pageSize < 1 {
		pageSize = service.DefaultPageSize
	}
	from := (page-1)*pageSize + 1
	if total == 0 {
		from = 0
	}
	to := page * pageSize
	if to > total {
		to = total
	}
	p := pagination{
		Page:     page,
		PageSize: pageSize,
		Total:    total,
		From:     from,
		To:       to,
		PrevURL:  listURL(filter, page-1),
		NextURL:  listURL(filter, page+1),
	}
	p.HasPrev = page > 1
	p.HasNext = to < total
	return p
}

// listURL rebuilds the current list URL with another page number.
func listURL(filter domain.ListFilter, page int) string {
	q := url.Values{}
	for _, tag := range filter.Tags {
		q.Add("tag", tag)
	}
	if filter.FavoritesOnly {
		q.Set("favorites", "1")
	}
	if filter.HasLink {
		q.Set("link", "1")
	}
	if filter.HasFile {
		q.Set("file", "1")
	}
	if filter.Sort != "" && filter.Sort != domain.SortNewest {
		q.Set("sort", string(filter.Sort))
	}
	if page > 1 {
		q.Set("page", strconv.Itoa(page))
	}
	if encoded := q.Encode(); encoded != "" {
		return "/?" + encoded
	}
	return "/"
}

// tagFilterURL adds one tag to the current filters.
func tagFilterURL(current url.Values, name string) string {
	q := url.Values{}
	for _, existing := range current["tag"] {
		if existing == name {
			continue
		}
		q.Add("tag", existing)
	}
	q.Add("tag", name)
	for _, key := range []string{"favorites", "link", "file", "sort"} {
		if value := current.Get(key); value != "" {
			q.Set(key, value)
		}
	}
	return "/?" + q.Encode()
}

// selectedTagChips renders the active tag filters as removable chips.
func selectedTagChips(current url.Values, tags []string) []tagChip {
	chips := make([]tagChip, 0, len(tags))
	for _, name := range tags {
		q := url.Values{}
		for _, existing := range current["tag"] {
			if existing != name {
				q.Add("tag", existing)
			}
		}
		for _, key := range []string{"favorites", "link", "file", "sort"} {
			if value := current.Get(key); value != "" {
				q.Set(key, value)
			}
		}
		uri := "/"
		if encoded := q.Encode(); encoded != "" {
			uri += "?" + encoded
		}
		chips = append(chips, tagChip{Name: name, URL: uri})
	}
	return chips
}

// detailTagChips links item tags to a filtered list.
func detailTagChips(tags []domain.Tag) []tagChip {
	chips := make([]tagChip, 0, len(tags))
	for _, tag := range tags {
		q := url.Values{}
		q.Add("tag", tag.Name)
		chips = append(chips, tagChip{Name: tag.Name, URL: "/?" + q.Encode()})
	}
	return chips
}

// splitTags splits the comma-separated tag field. Empty pieces are dropped;
// the service trims and validates what remains.
func splitTags(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

// cleanQueryValues trims and drops empty repeated query values.
func cleanQueryValues(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

// tagNames extracts tag names for the edit form.
func tagNames(tags []domain.Tag) []string {
	names := make([]string, 0, len(tags))
	for _, tag := range tags {
		names = append(names, tag.Name)
	}
	return names
}

// snippet collapses whitespace and truncates to max runes (spec 4.2).
func snippet(text string, max int) string {
	fields := strings.Fields(text)
	joined := strings.Join(fields, " ")
	if utf8.RuneCountInString(joined) <= max {
		return joined
	}
	runes := []rune(joined)
	return string(runes[:max]) + "…"
}

// isValidationError reports whether err is a domain validation failure.
func isValidationError(err error) bool {
	var validation *domain.ValidationError
	return errors.As(err, &validation)
}

// validationMessage returns the user-facing part of a validation error.
func validationMessage(err error) string {
	var validation *domain.ValidationError
	if errors.As(err, &validation) {
		return validation.Message
	}
	return "That value is not valid."
}

// localPath accepts only same-site absolute paths, preventing an open redirect
// through the favorite form's `next` field.
func localPath(raw, fallback string) string {
	if raw == "" || !strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, "//") ||
		strings.Contains(raw, "\\") || strings.Contains(raw, "\n") || strings.Contains(raw, "\r") {
		return fallback
	}
	u, err := url.Parse(raw)
	if err != nil || u.IsAbs() || u.Host != "" {
		return fallback
	}
	return u.String()
}

// tagsFromData recovers the sidebar tag list loaded by itemPageData.
func tagsFromData(data map[string]any) []domain.TagCount {
	tags, _ := data["AllTags"].([]domain.TagCount)
	return tags
}
