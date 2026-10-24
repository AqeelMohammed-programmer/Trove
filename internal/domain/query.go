package domain

// SortOrder is the list ordering chosen by the user (B-2).
type SortOrder string

// Supported sort orders.
const (
	SortNewest    SortOrder = "newest"
	SortUpdated   SortOrder = "updated"
	SortTitle     SortOrder = "title"
	SortRelevance SortOrder = "relevance"
)

// IsValid reports whether the sort order is one the UI offers.
func (s SortOrder) IsValid() bool {
	switch s {
	case SortNewest, SortUpdated, SortTitle, SortRelevance:
		return true
	default:
		return false
	}
}

// ListFilter is the combined browse/search query (B-3, S-5). All filters
// combine with AND; Tags requires every listed tag (B-4).
type ListFilter struct {
	Search        string
	Tags          []string
	FavoritesOnly bool
	HasLink       bool
	HasFile       bool
	Sort          SortOrder
	Page          int
	// PageSize is filled by the service; zero means the default.
	PageSize int
}

// ListResult is one page of items plus the total match count (B-7).
type ListResult struct {
	Items    []Item
	Total    int
	Page     int
	PageSize int
}

// TagCount is a tag with the number of active items using it (M-1, T-6).
type TagCount struct {
	Tag
	Count int
}

// TrashAttachment is an individually trashed attachment shown with its owning
// item in the trash view (X-3).
type TrashAttachment struct {
	Attachment
	// ItemTitle is the title of the owning item.
	ItemTitle string
}

// TrashList is everything the trash view shows (X-3).
type TrashList struct {
	// Items are trashed items, each with all of its attachments.
	Items []Item
	// Attachments are individually trashed attachments of active items.
	Attachments []TrashAttachment
}

// DuplicateMatch is another attachment that matches one of an item's
// attachments by exact URL or SHA-256 (D-1, D-2, D-4).
type DuplicateMatch struct {
	Attachment Attachment
	// ItemID and ItemTitle identify the owning item.
	ItemID    string
	ItemTitle string
	// ItemTrashed and AttachmentTrashed mark trashed entries (D-5, R-3).
	ItemTrashed       bool
	AttachmentTrashed bool
}

// DuplicateEntry is one attachment in a duplicate report group (R-2, R-4).
type DuplicateEntry struct {
	Attachment        Attachment
	ItemID            string
	ItemTitle         string
	ItemTrashed       bool
	AttachmentTrashed bool
}

// DuplicateGroup is a set of two or more matching attachments (R-2).
type DuplicateGroup struct {
	Kind    AttachmentKind
	Key     string
	Entries []DuplicateEntry
}
