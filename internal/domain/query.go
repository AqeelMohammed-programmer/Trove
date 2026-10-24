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
