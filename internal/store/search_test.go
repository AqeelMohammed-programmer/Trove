package store

import (
	"context"
	"testing"
	"time"

	"Trove/internal/domain"
)

func insertSearchItem(t *testing.T, st *Store, item domain.Item, tags []string, attachments []domain.Attachment) {
	t.Helper()
	if _, err := st.CreateItem(context.Background(), item, tags, attachments); err != nil {
		t.Fatalf("CreateItem(%s): %v", item.ID, err)
	}
}

func newSearchItem(id, title, description, note string) domain.Item {
	now := time.Date(2026, 10, 24, 9, 0, 0, 0, time.UTC)
	return domain.Item{ID: id, Title: title, Description: description, Note: note, CreatedAt: now, UpdatedAt: now}
}

func searchIDs(t *testing.T, st *Store, query string) []string {
	t.Helper()
	result, err := st.ListItems(context.Background(), domain.ListFilter{
		Search: query, Sort: domain.SortRelevance,
	})
	if err != nil {
		t.Fatalf("ListItems(search %q): %v", query, err)
	}
	ids := make([]string, 0, len(result.Items))
	for _, item := range result.Items {
		ids = append(ids, item.ID)
	}
	return ids
}

func TestSearch_CoversTitleDescriptionNoteTags_S1(t *testing.T) {
	t.Parallel()

	st := newTestStore(t)
	insertSearchItem(t, st, newSearchItem("a", "Alpha", "a description", "a note"), []string{"reading"}, nil)

	for _, query := range []string{"Alpha", "description", "note", "reading"} {
		if ids := searchIDs(t, st, query); len(ids) != 1 || ids[0] != "a" {
			t.Errorf("search %q = %v, want [a] (S-1)", query, ids)
		}
	}
}

func TestSearch_ExcludesURLsLabelsAndFileNames_S1(t *testing.T) {
	t.Parallel()

	st := newTestStore(t)
	now := time.Now().UTC()
	item := newSearchItem("a", "Title", "", "")
	attachments := []domain.Attachment{
		{ID: "l", Kind: domain.KindLink, URL: "https://zzzunique.example", NormalizedURL: "https://zzzunique.example", Label: "qqqlabel", CreatedAt: now, State: domain.StateActive},
		{ID: "f", Kind: domain.KindFile, FileName: "wwwname.txt", SizeBytes: 1, SHA256: "d", CreatedAt: now, State: domain.StateActive},
	}
	insertSearchItem(t, st, item, nil, attachments)

	for _, query := range []string{"zzzunique", "qqqlabel", "wwwname"} {
		if ids := searchIDs(t, st, query); len(ids) != 0 {
			t.Errorf("search %q = %v, want none (S-1 excludes it)", query, ids)
		}
	}
}

func TestSearch_PrefixCaseInsensitiveAllWordsRequired_S2(t *testing.T) {
	t.Parallel()

	st := newTestStore(t)
	insertSearchItem(t, st, newSearchItem("a", "Rust programming", "memory safety", ""), nil, nil)

	// Case-insensitive prefix.
	if ids := searchIDs(t, st, "prOgRa"); len(ids) != 1 {
		t.Errorf("case-insensitive prefix search = %v, want one result (S-2)", ids)
	}
	// All words required: one word missing yields nothing.
	if ids := searchIDs(t, st, "rust missing"); len(ids) != 0 {
		t.Errorf("all-words search = %v, want none (S-2)", ids)
	}
	// Both words present matches.
	if ids := searchIDs(t, st, "rust safe"); len(ids) != 1 {
		t.Errorf("multi-word search = %v, want one result (S-2)", ids)
	}
}

func TestSearch_TitleRanksAboveBody_S3(t *testing.T) {
	t.Parallel()

	st := newTestStore(t)
	insertSearchItem(t, st, newSearchItem("body", "Unrelated", "keyword appears here", ""), nil, nil)
	insertSearchItem(t, st, newSearchItem("title", "keyword in title", "", ""), nil, nil)

	ids := searchIDs(t, st, "keyword")
	if len(ids) != 2 {
		t.Fatalf("search = %v, want two results", ids)
	}
	if ids[0] != "title" {
		t.Errorf("ranking = %v, want the title match first (S-3)", ids)
	}
}

func TestSearch_MarkdownSymbolsDoNotAffectMatching_S4(t *testing.T) {
	t.Parallel()

	st := newTestStore(t)
	insertSearchItem(t, st, newSearchItem("a", "T", "", "# Heading\n\n**bold** and `code`"), nil, nil)

	for _, query := range []string{"heading", "bold", "code"} {
		if ids := searchIDs(t, st, query); len(ids) != 1 {
			t.Errorf("search %q = %v, want one result (S-4)", query, ids)
		}
	}
}

func TestSearch_SavedChangeIsImmediatelySearchable_S6(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st := newTestStore(t)
	insertSearchItem(t, st, newSearchItem("a", "Original", "", "oldnote"), nil, nil)

	item, err := st.GetItem(ctx, "a")
	if err != nil {
		t.Fatalf("GetItem(): %v", err)
	}
	item.Title = "Renamed"
	item.Note = "newnote"
	item.Tags = nil
	if _, err := st.UpdateItem(ctx, item, []string{"freshtag"}, nil); err != nil {
		t.Fatalf("UpdateItem(): %v", err)
	}

	if ids := searchIDs(t, st, "Renamed"); len(ids) != 1 {
		t.Errorf("new title not searchable immediately (S-6): %v", ids)
	}
	if ids := searchIDs(t, st, "freshtag"); len(ids) != 1 {
		t.Errorf("new tag not searchable immediately (S-6): %v", ids)
	}
	if ids := searchIDs(t, st, "oldnote"); len(ids) != 0 {
		t.Errorf("removed note still searchable (S-6): %v", ids)
	}
}

func TestSearch_SyntaxInjectionIsInert(t *testing.T) {
	t.Parallel()

	st := newTestStore(t)
	insertSearchItem(t, st, newSearchItem("a", "Alpha", "", ""), nil, nil)

	for _, query := range []string{`"`, `*`, `AND`, `NOT`, `NEAR(`, `alpha OR beta`, `^`, `-alpha`, `a:b`} {
		if _, err := st.ListItems(context.Background(), domain.ListFilter{Search: query}); err != nil {
			t.Errorf("search %q returned an error instead of being treated as text: %v", query, err)
		}
	}
}

func TestBuildMatch(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"":            "",
		"   ":         "",
		"foo":         `"foo"*`,
		"foo bar":     `"foo"* "bar"*`,
		`he"llo`:      `"he""llo"*`,
		"  spaced   ": `"spaced"*`,
	}
	for input, want := range tests {
		if got := buildMatch(input); got != want {
			t.Errorf("buildMatch(%q) = %q, want %q", input, got, want)
		}
	}
}
