package domain

import (
	"encoding/hex"
	"errors"
	"strings"
	"testing"
)

func TestCleanTitle_RequiresTitleWithoutAttachments_I2(t *testing.T) {
	t.Parallel()

	if _, err := CleanTitle("   ", nil); err == nil {
		t.Fatal("CleanTitle(blank, no attachments) error = nil, want a validation error")
	}
	var validation *ValidationError
	_, err := CleanTitle("", nil)
	if !asValidation(err, &validation) {
		t.Fatalf("CleanTitle() error = %v, want *ValidationError", err)
	}
}

func TestCleanTitle_DefaultsToFirstAttachment_I2(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		raw         string
		attachments []Attachment
		want        string
	}{
		{
			name:        "link URL",
			raw:         "  ",
			attachments: []Attachment{{Kind: KindLink, URL: "https://example.org/a"}},
			want:        "https://example.org/a",
		},
		{
			name:        "file name",
			raw:         "",
			attachments: []Attachment{{Kind: KindFile, FileName: "paper.pdf"}},
			want:        "paper.pdf",
		},
		{
			name:        "first of several wins",
			raw:         "",
			attachments: []Attachment{{Kind: KindFile, FileName: "first.txt"}, {Kind: KindLink, URL: "https://example.org"}},
			want:        "first.txt",
		},
		{
			name:        "explicit title is kept",
			raw:         "  My title  ",
			attachments: []Attachment{{Kind: KindFile, FileName: "paper.pdf"}},
			want:        "My title",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := CleanTitle(tt.raw, tt.attachments)
			if err != nil {
				t.Fatalf("CleanTitle(): %v", err)
			}
			if got != tt.want {
				t.Fatalf("CleanTitle() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCleanTitle_EnforcesLimit(t *testing.T) {
	t.Parallel()

	long := strings.Repeat("a", MaxTitle+1)
	if _, err := CleanTitle(long, nil); err == nil {
		t.Fatal("CleanTitle(too long) error = nil, want a validation error")
	}
}

func TestCleanTagName_T3(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		raw     string
		want    string
		wantErr bool
	}{
		{name: "trims", raw: "  reading  ", want: "reading"},
		{name: "collapses inner whitespace", raw: "to   read\tlater", want: "to read later"},
		{name: "empty after trim", raw: "   ", wantErr: true},
		{name: "comma is rejected", raw: "a,b", wantErr: true},
		{name: "exactly 50 runes", raw: strings.Repeat("ä", 50), want: strings.Repeat("ä", 50)},
		{name: "51 runes rejected", raw: strings.Repeat("ä", 51), wantErr: true},
		{name: "control character rejected", raw: "a\x00b", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := CleanTagName(tt.raw)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("CleanTagName(%q) error = nil, want a validation error", tt.raw)
				}
				return
			}
			if err != nil {
				t.Fatalf("CleanTagName(%q): %v", tt.raw, err)
			}
			if got != tt.want {
				t.Fatalf("CleanTagName(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestTagKey_IsCaseInsensitive_T4(t *testing.T) {
	t.Parallel()

	for _, pair := range [][2]string{{"Reading", "reading"}, {"RUST", "rust"}, {"Ärger", "ärger"}} {
		if TagKey(pair[0]) != TagKey(pair[1]) {
			t.Errorf("TagKey(%q) != TagKey(%q); T-4 requires case-insensitive uniqueness", pair[0], pair[1])
		}
	}
}

func TestCleanLinkURL_A2(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		raw     string
		want    string
		wantErr bool
	}{
		{name: "https", raw: "https://example.org/a?b=c", want: "https://example.org/a?b=c"},
		{name: "http", raw: "http://example.org", want: "http://example.org"},
		{name: "trims surrounding space", raw: "  https://example.org/x  ", want: "https://example.org/x"},
		{name: "empty", raw: "   ", wantErr: true},
		{name: "relative", raw: "/path/only", wantErr: true},
		{name: "javascript scheme", raw: "javascript:alert(1)", wantErr: true},
		{name: "mailto scheme", raw: "mailto:x@example.org", wantErr: true},
		{name: "ftp scheme", raw: "ftp://example.org/file", wantErr: true},
		{name: "no host", raw: "https://", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := CleanLinkURL(tt.raw)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("CleanLinkURL(%q) error = nil, want a validation error", tt.raw)
				}
				return
			}
			if err != nil {
				t.Fatalf("CleanLinkURL(%q): %v", tt.raw, err)
			}
			if got != tt.want {
				t.Fatalf("CleanLinkURL(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestNormalizeURL_R2(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "fragment removed", raw: "https://example.org/a#section", want: "https://example.org/a"},
		{name: "one trailing slash removed", raw: "https://example.org/a/", want: "https://example.org/a"},
		{name: "fragment and slash removed", raw: "https://example.org/a/#x", want: "https://example.org/a"},
		{name: "query kept exactly", raw: "https://example.org/a?x=1#f", want: "https://example.org/a?x=1"},
		{name: "root path kept", raw: "https://example.org/", want: "https://example.org"},
		{name: "case is significant", raw: "https://example.org/AB", want: "https://example.org/AB"},
		{name: "scheme and host kept", raw: "http://Example.org/a/", want: "http://Example.org/a"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := NormalizeURL(tt.raw); got != tt.want {
				t.Fatalf("NormalizeURL(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestNormalizeURL_FragmentOnly(t *testing.T) {
	t.Parallel()

	// Two URLs that differ only by fragment must normalize to the same value.
	a := NormalizeURL("https://example.org/thing#one")
	b := NormalizeURL("https://example.org/thing#two")
	if a != b {
		t.Fatalf("fragments are significant in normalization: %q != %q", a, b)
	}
}

func TestNewID_IsHex128Bit(t *testing.T) {
	t.Parallel()

	seen := make(map[string]struct{})
	for i := 0; i < 100; i++ {
		id, err := NewID()
		if err != nil {
			t.Fatalf("NewID(): %v", err)
		}
		if len(id) != 32 {
			t.Fatalf("NewID() length = %d, want 32", len(id))
		}
		if _, err := hex.DecodeString(id); err != nil {
			t.Fatalf("NewID() = %q, not hex: %v", id, err)
		}
		if _, dup := seen[id]; dup {
			t.Fatalf("NewID() returned a duplicate: %q", id)
		}
		seen[id] = struct{}{}
	}
}

func TestCleanDescription_EnforcesLimit(t *testing.T) {
	t.Parallel()

	if _, err := CleanDescription(strings.Repeat("x", MaxDescription+1)); err == nil {
		t.Fatal("CleanDescription(too long) error = nil, want a validation error")
	}
}

func TestCleanNote_EnforcesLimit(t *testing.T) {
	t.Parallel()

	if _, err := CleanNote(strings.Repeat("x", MaxNote+1)); err == nil {
		t.Fatal("CleanNote(too long) error = nil, want a validation error")
	}
}

// asValidation is a local errors.As for *ValidationError.
func asValidation(err error, target **ValidationError) bool {
	return errors.As(err, target)
}
