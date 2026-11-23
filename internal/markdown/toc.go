package markdown

import (
	"fmt"
	"html/template"
	"strings"
)

// BuildTOC は見出し一覧から目次の HTML を作る。
// 入れ子にせず平らなリストにし、レベル差は toc-d0〜toc-d5 のクラスで字下げする。
func BuildTOC(hs []Heading, depth int, title string) string {
	minLevel := 7
	for _, h := range hs {
		if h.Level <= depth && h.Level < minLevel {
			minLevel = h.Level
		}
	}
	var b strings.Builder
	b.WriteString(`<nav class="toc"><div class="toc-title">`)
	b.WriteString(template.HTMLEscapeString(title))
	b.WriteString(`</div><ol>`)
	for _, h := range hs {
		if h.Level > depth || h.ID == "" {
			continue
		}
		fmt.Fprintf(&b, `<li class="toc-d%d"><a href="#%s">%s</a></li>`,
			h.Level-minLevel, template.HTMLEscapeString(h.ID), template.HTMLEscapeString(h.Text))
	}
	b.WriteString(`</ol></nav>`)
	return b.String()
}

// InsertTOC は目次を挿入する。マーカーの位置があればそこに、なければ本文の先頭に入れる。
func InsertTOC(body, toc string) string {
	if strings.Contains(body, TOCPlaceholder) {
		return strings.Replace(body, TOCPlaceholder, toc, 1)
	}
	return toc + body
}
