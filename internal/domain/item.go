// Package domain holds the entities and pure rules of Trove. It performs no
// I/O and imports nothing from this project.
package domain

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// Field length limits from spec section 8.2.
const (
	MaxTitle       = 300
	MaxDescription = 5000
	MaxNote        = 500000
	MaxLabel       = 100
	MaxTagName     = 50
)

// AttachmentKind distinguishes a link from a copied file (spec 2.2).
type AttachmentKind string

// Attachment kinds.
const (
	KindLink AttachmentKind = "link"
	KindFile AttachmentKind = "file"
)

// AttachmentState is the trash state from spec 2.2.
type AttachmentState string

// Attachment states.
const (
	StateActive              AttachmentState = "active"
	StateTrashedIndividually AttachmentState = "trashed_individually"
	StateTrashedWithItem     AttachmentState = "trashed_with_item"
)

// Item is the single top-level object (spec 2.1).
type Item struct {
	ID          string
	Title       string
	Description string
	Note        string
	Favorite    bool
	Tags        []Tag
	Attachments []Attachment
	// Counts is filled by list queries for row rendering; Attachments is not
	// loaded for lists.
	Counts    AttachmentCounts
	CreatedAt time.Time
	UpdatedAt time.Time
	TrashedAt *time.Time
}

// AttachmentCounts is the per-kind count shown in a list row (spec 4.2).
type AttachmentCounts struct {
	Links int
	Files int
}

// Tag is a flat, free-typed label (spec 2.3).
type Tag struct {
	ID   string
	Name string
}

// Attachment is a link or a copied file owned by one item (spec 2.2).
type Attachment struct {
	ID            string
	ItemID        string
	Kind          AttachmentKind
	Label         string
	URL           string
	NormalizedURL string
	FileName      string
	SizeBytes     int64
	MediaType     string
	SHA256        string
	CreatedAt     time.Time
	State         AttachmentState
	TrashedAt     *time.Time
}

// ValidationError describes an invalid user input. The web layer shows the
// message next to the form.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// NewID returns a 128-bit random identifier, hex encoded.
func NewID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating id: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// CleanTitle trims a title and enforces I-2's non-empty rule. When the title
// is empty and the item has attachments, it falls back to the first
// attachment (I-2); otherwise it returns a validation error.
func CleanTitle(raw string, attachments []Attachment) (string, error) {
	title := strings.TrimSpace(raw)
	if title == "" && len(attachments) > 0 {
		title = DefaultTitle(attachments)
	}
	if title == "" {
		return "", &ValidationError{Field: "title", Message: "Give the item a title, or add an attachment first."}
	}
	if err := checkLength("title", title, MaxTitle); err != nil {
		return "", err
	}
	return title, nil
}

// DefaultTitle implements I-2: the first attachment's URL for a link or file
// name for a file.
func DefaultTitle(attachments []Attachment) string {
	if len(attachments) == 0 {
		return ""
	}
	first := attachments[0]
	switch first.Kind {
	case KindLink:
		return first.URL
	case KindFile:
		return first.FileName
	default:
		return ""
	}
}

// CleanDescription validates the optional description.
func CleanDescription(raw string) (string, error) {
	if err := checkLength("description", raw, MaxDescription); err != nil {
		return "", err
	}
	return raw, nil
}

// CleanNote validates the optional Markdown note.
func CleanNote(raw string) (string, error) {
	if err := checkLength("note", raw, MaxNote); err != nil {
		return "", err
	}
	return raw, nil
}

// CleanLabel validates an optional attachment label.
func CleanLabel(raw string) (string, error) {
	label := strings.TrimSpace(raw)
	if err := checkLength("label", label, MaxLabel); err != nil {
		return "", err
	}
	return label, nil
}

// CleanTagName applies T-3: trim, collapse inner whitespace, 1 to 50
// characters, and no commas.
func CleanTagName(raw string) (string, error) {
	name := strings.Join(strings.Fields(raw), " ")
	if name == "" {
		return "", &ValidationError{Field: "tags", Message: "Tag names cannot be empty."}
	}
	if strings.ContainsRune(name, ',') {
		return "", &ValidationError{Field: "tags", Message: "Tag names cannot contain commas."}
	}
	if utf8.RuneCountInString(name) > MaxTagName {
		return "", &ValidationError{Field: "tags", Message: fmt.Sprintf("Tag names are limited to %d characters.", MaxTagName)}
	}
	for _, r := range name {
		if unicode.IsControl(r) {
			return "", &ValidationError{Field: "tags", Message: "Tag names cannot contain control characters."}
		}
	}
	return name, nil
}

// TagKey returns the case-insensitive comparison key for a tag name (T-4).
func TagKey(name string) string {
	return strings.ToLower(name)
}

// CleanLinkURL validates a link attachment (A-2): an absolute http or https
// URL, stored exactly as entered after trimming surrounding whitespace.
func CleanLinkURL(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", &ValidationError{Field: "url", Message: "Enter a URL."}
	}
	u, err := url.Parse(trimmed)
	if err != nil {
		return "", &ValidationError{Field: "url", Message: "That is not a valid URL."}
	}
	if !u.IsAbs() || u.Host == "" {
		return "", &ValidationError{Field: "url", Message: "Enter an absolute URL, for example https://example.org."}
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", &ValidationError{Field: "url", Message: "Only http and https links are supported."}
	}
	return trimmed, nil
}

// NormalizeURL implements R-2: remove any fragment and one trailing slash
// from the path, leaving everything else exact. It is stored at write time so
// the duplicate report is fast.
func NormalizeURL(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return strings.TrimSpace(raw)
	}
	u.Fragment = ""
	u.RawFragment = ""
	if strings.HasSuffix(u.Path, "/") {
		u.Path = strings.TrimSuffix(u.Path, "/")
		u.RawPath = strings.TrimSuffix(u.RawPath, "/")
	}
	return u.String()
}

func checkLength(field, value string, max int) error {
	if utf8.RuneCountInString(value) > max {
		return &ValidationError{
			Field:   field,
			Message: fmt.Sprintf("This field is limited to %d characters.", max),
		}
	}
	return nil
}
