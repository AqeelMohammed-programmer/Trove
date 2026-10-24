package web

import (
	"bytes"
	"fmt"
	"html/template"

	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// Markdown renders notes for display. It is the only place in the application
// that turns user text into HTML, and it always sanitizes the result (V-2).
type Markdown struct {
	engine goldmark.Markdown
	policy *bluemonday.Policy
}

// NewMarkdown builds the note renderer.
//
// Raw HTML is never passed through: goldmark's default renderer omits it
// (V-2), and bluemonday sanitizes whatever survives parsing. Images are turned
// into ordinary links by imageToLink so nothing is fetched or displayed inline
// (V-3), and bluemonday marks every link with target="_blank" (V-4).
func NewMarkdown() *Markdown {
	engine := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithParserOptions(
			parser.WithASTTransformers(util.Prioritized(&imageTransformer{}, 100)),
		),
	)

	policy := bluemonday.UGCPolicy()
	policy.AllowElements("table", "thead", "tbody", "tr", "th", "td")
	policy.AllowAttrs("colspan", "rowspan").OnElements("td", "th")
	policy.RequireNoFollowOnLinks(false)
	policy.RequireNoFollowOnFullyQualifiedLinks(false)
	policy.AddTargetBlankToFullyQualifiedLinks(true)

	return &Markdown{engine: engine, policy: policy}
}

// Render returns sanitized HTML for a Markdown note.
func (m *Markdown) Render(source string) (template.HTML, error) {
	var buf bytes.Buffer
	if err := m.engine.Convert([]byte(source), &buf); err != nil {
		return "", fmt.Errorf("rendering markdown: %w", err)
	}
	// #nosec G203 -- the HTML is produced by goldmark and sanitized by
	// bluemonday before being marked safe in a template.
	return template.HTML(m.policy.SanitizeBytes(buf.Bytes())), nil //nolint:gosec
}

// nodeText collects the literal text of a node's descendants.
func nodeText(n ast.Node, source []byte) []byte {
	var out []byte
	_ = ast.Walk(n, func(child ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch v := child.(type) {
		case *ast.Text:
			out = append(out, v.Segment.Value(source)...)
		case *ast.String:
			out = append(out, v.Value...)
		}
		return ast.WalkContinue, nil
	})
	return out
}

// imageTransformer rewrites Markdown images into links, implementing V-3.
type imageTransformer struct{}

// Transform collects images first and replaces them after the walk, so the
// walker never follows a node that has already moved.
func (t *imageTransformer) Transform(doc *ast.Document, reader text.Reader, _ parser.Context) {
	var images []*ast.Image
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering {
			if img, ok := n.(*ast.Image); ok {
				images = append(images, img)
			}
		}
		return ast.WalkContinue, nil
	})

	for _, img := range images {
		parent := img.Parent()
		if parent == nil {
			continue
		}
		link := ast.NewLink()
		link.Destination = img.Destination
		link.Title = img.Title
		alt := nodeText(img, reader.Source())
		if len(alt) == 0 {
			alt = img.Destination
		}
		link.AppendChild(link, ast.NewString(alt))
		parent.ReplaceChild(parent, img, link)
	}
}
