package web

import (
	"strings"
	"testing"
)

func TestMarkdown_RendersStandardSyntax_V1(t *testing.T) {
	t.Parallel()

	md := NewMarkdown()
	source := "# Heading\n\nSome *emphasis* and **strong**.\n\n- one\n- two\n\n> quote\n\n`code`\n\n| a | b |\n|---|---|\n| 1 | 2 |\n"
	html, err := md.Render(source)
	if err != nil {
		t.Fatalf("Render(): %v", err)
	}
	rendered := string(html)
	for _, want := range []string{"<h1", "<em>", "<strong>", "<ul>", "<blockquote>", "<code>", "<table>"} {
		if !strings.Contains(rendered, want) {
			t.Errorf("rendered note is missing %q:\n%s", want, rendered)
		}
	}
}

func TestMarkdown_RawHTMLNotRendered_V2(t *testing.T) {
	t.Parallel()

	md := NewMarkdown()
	html, err := md.Render("before\n\n<script>alert(1)</script>\n\n<img src=x onerror=alert(1)>\n\nafter")
	if err != nil {
		t.Fatalf("Render(): %v", err)
	}
	rendered := string(html)
	if strings.Contains(rendered, "<script") {
		t.Errorf("script tag survived rendering:\n%s", rendered)
	}
	if strings.Contains(rendered, "onerror") {
		t.Errorf("event handler attribute survived rendering:\n%s", rendered)
	}
	if strings.Contains(rendered, "<img") {
		t.Errorf("raw img tag survived rendering:\n%s", rendered)
	}
}

func TestMarkdown_ImagesBecomeLinks_V3(t *testing.T) {
	t.Parallel()

	md := NewMarkdown()
	html, err := md.Render("![alt text](https://example.org/pic.png)")
	if err != nil {
		t.Fatalf("Render(): %v", err)
	}
	rendered := string(html)
	if strings.Contains(rendered, "<img") {
		t.Errorf("image syntax rendered as an inline image:\n%s", rendered)
	}
	if !strings.Contains(rendered, "<a") || !strings.Contains(rendered, "https://example.org/pic.png") {
		t.Errorf("image syntax did not become a link:\n%s", rendered)
	}
}

func TestMarkdown_LinksOpenInNewTab_V4(t *testing.T) {
	t.Parallel()

	md := NewMarkdown()
	html, err := md.Render("[Example](https://example.org/page)")
	if err != nil {
		t.Fatalf("Render(): %v", err)
	}
	rendered := string(html)
	if !strings.Contains(rendered, `target="_blank"`) {
		t.Errorf("link is missing target=\"_blank\":\n%s", rendered)
	}
	if !strings.Contains(rendered, "noopener") {
		t.Errorf("link is missing rel=noopener:\n%s", rendered)
	}
}

func TestMarkdown_RejectsJavascriptLinks_V2(t *testing.T) {
	t.Parallel()

	md := NewMarkdown()
	html, err := md.Render("[click](javascript:alert(1))")
	if err != nil {
		t.Fatalf("Render(): %v", err)
	}
	if strings.Contains(string(html), "javascript:") {
		t.Errorf("javascript: URL survived sanitizing:\n%s", html)
	}
}
