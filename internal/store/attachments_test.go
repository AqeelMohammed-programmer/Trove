package store

import (
	"context"
	"testing"
	"time"

	"Trove/internal/domain"
)

func TestStore_CreateItemWithAttachments_A1_A3_R2(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st := newTestStore(t)

	now := time.Date(2026, 10, 24, 9, 0, 0, 0, time.UTC)
	item := domain.Item{ID: "item-1", Title: "Mixed", CreatedAt: now, UpdatedAt: now}
	attachments := []domain.Attachment{
		{
			ID: "a-link", Kind: domain.KindLink, Label: "source",
			URL: "https://example.org/a#frag", NormalizedURL: "https://example.org/a",
			CreatedAt: now, State: domain.StateActive,
		},
		{
			ID: "a-file", Kind: domain.KindFile, FileName: "paper.pdf",
			SizeBytes: 123, MediaType: "application/pdf", SHA256: "abc123",
			CreatedAt: now, State: domain.StateActive,
		},
	}
	if _, err := st.CreateItem(ctx, item, nil, attachments); err != nil {
		t.Fatalf("CreateItem(): %v", err)
	}

	got, err := st.GetItem(ctx, "item-1")
	if err != nil {
		t.Fatalf("GetItem(): %v", err)
	}
	if len(got.Attachments) != 2 {
		t.Fatalf("attachments = %d, want 2 (A-1)", len(got.Attachments))
	}
	if got.Counts.Links != 1 || got.Counts.Files != 1 {
		t.Errorf("counts = %+v, want 1 link and 1 file (B-5)", got.Counts)
	}
	byID := map[string]domain.Attachment{}
	for _, a := range got.Attachments {
		byID[a.ID] = a
	}
	if byID["a-link"].NormalizedURL != "https://example.org/a" {
		t.Errorf("normalized URL = %q, want the fragment removed (R-2)", byID["a-link"].NormalizedURL)
	}
	if byID["a-file"].SHA256 != "abc123" {
		t.Errorf("sha256 = %q, want abc123", byID["a-file"].SHA256)
	}
}

func TestStore_ListItems_AttachmentKindFilters_B5(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st := newTestStore(t)
	now := time.Now().UTC()

	withLink := domain.Item{ID: "l", Title: "L", CreatedAt: now, UpdatedAt: now}
	withFile := domain.Item{ID: "f", Title: "F", CreatedAt: now, UpdatedAt: now}
	if _, err := st.CreateItem(ctx, withLink, nil, []domain.Attachment{
		{ID: "lnk", Kind: domain.KindLink, URL: "https://example.org", NormalizedURL: "https://example.org", CreatedAt: now, State: domain.StateActive},
	}); err != nil {
		t.Fatalf("CreateItem(link): %v", err)
	}
	if _, err := st.CreateItem(ctx, withFile, nil, []domain.Attachment{
		{ID: "fil", Kind: domain.KindFile, FileName: "a.txt", SizeBytes: 1, SHA256: "d", CreatedAt: now, State: domain.StateActive},
	}); err != nil {
		t.Fatalf("CreateItem(file): %v", err)
	}

	links, err := st.ListItems(ctx, domain.ListFilter{HasLink: true})
	if err != nil {
		t.Fatalf("ListItems(has link): %v", err)
	}
	if links.Total != 1 || links.Items[0].ID != "l" {
		t.Errorf("has-link filter = %+v, want only l", links.Items)
	}

	files, err := st.ListItems(ctx, domain.ListFilter{HasFile: true})
	if err != nil {
		t.Fatalf("ListItems(has file): %v", err)
	}
	if files.Total != 1 || files.Items[0].ID != "f" {
		t.Errorf("has-file filter = %+v, want only f", files.Items)
	}
}

func TestStore_AttachmentByID_A7(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st := newTestStore(t)
	now := time.Now().UTC()

	item := domain.Item{ID: "item", Title: "Item", CreatedAt: now, UpdatedAt: now}
	if _, err := st.CreateItem(ctx, item, nil, []domain.Attachment{
		{ID: "file-1", Kind: domain.KindFile, FileName: "x.bin", SizeBytes: 9, SHA256: "deadbeef", CreatedAt: now, State: domain.StateActive},
	}); err != nil {
		t.Fatalf("CreateItem(): %v", err)
	}

	got, err := st.AttachmentByID(ctx, "file-1")
	if err != nil {
		t.Fatalf("AttachmentByID(): %v", err)
	}
	if got.FileName != "x.bin" || got.ItemID != "item" {
		t.Errorf("AttachmentByID() = %+v, want file-1 on item", got)
	}
	if _, err := st.AttachmentByID(ctx, "missing"); err == nil {
		t.Error("AttachmentByID(missing) error = nil, want an error")
	}
}
