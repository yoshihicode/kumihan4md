// Package markdown は Markdown の前処理・HTML 変換・目次生成を担当する。
package markdown

import (
	"bytes"
	"fmt"
	"strings"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	meta "github.com/yuin/goldmark-meta"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
)

// Heading は文書中の見出し。
type Heading struct {
	Level    int
	ID, Text string
}

// Document は変換結果。
type Document struct {
	Body     string          // 本文の HTML
	Headings []Heading       // 見出し一覧（目次用）
	Meta     map[string]any  // front matter
	quoted   map[string]bool // front matter で値を引用符で囲んで書いたキー
}

// Convert は Markdown を HTML に変換し、見出し一覧と front matter を返す。
// highlightStyle が "none" の場合はシンタックスハイライトを行わない。
func Convert(src, highlightStyle string) (*Document, error) {
	exts := []goldmark.Extender{extension.GFM, extension.Footnote, extension.CJK, meta.Meta}
	if highlightStyle != "none" {
		exts = append(exts, highlighting.NewHighlighting(
			highlighting.WithStyle(highlightStyle),
			highlighting.WithFormatOptions(chromahtml.WithClasses(true)),
		))
	}
	conv := goldmark.New(
		goldmark.WithExtensions(exts...),
		goldmark.WithParserOptions(parser.WithAutoHeadingID()),
		goldmark.WithRendererOptions(html.WithUnsafe()),
	)

	// AST を経由して変換し、途中で見出しと front matter を取り出す
	b := []byte(src)
	pctx := parser.NewContext()
	root := conv.Parser().Parse(text.NewReader(b), parser.WithContext(pctx))
	fm, err := meta.TryGet(pctx)
	if err != nil {
		return nil, fmt.Errorf("failed to parse front matter: %w", err)
	}

	var body bytes.Buffer
	if err := conv.Renderer().Render(&body, b, root); err != nil {
		return nil, err
	}
	return &Document{
		Body:     body.String(),
		Headings: collectHeadings(root, b),
		Meta:     fm,
		quoted:   quotedMetaKeys(src),
	}, nil
}

// collectHeadings は AST から見出し（レベル・自動生成ID・テキスト）を集める。
func collectHeadings(root ast.Node, src []byte) []Heading {
	var hs []Heading
	_ = ast.Walk(root, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		h, ok := n.(*ast.Heading)
		if !ok {
			return ast.WalkContinue, nil
		}
		id := ""
		if v, ok := h.AttributeString("id"); ok {
			if b, ok := v.([]byte); ok {
				id = string(b)
			}
		}
		hs = append(hs, Heading{Level: h.Level, ID: id, Text: nodeText(h, src)})
		return ast.WalkSkipChildren, nil
	})
	return hs
}

func nodeText(n ast.Node, src []byte) string {
	var b strings.Builder
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		switch t := c.(type) {
		case *ast.Text:
			b.Write(t.Segment.Value(src))
			if t.SoftLineBreak() {
				b.WriteByte(' ')
			}
		case *ast.String:
			b.Write(t.Value)
		default:
			b.WriteString(nodeText(c, src))
		}
	}
	return b.String()
}
