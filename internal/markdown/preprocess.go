package markdown

import (
	"html"
	"regexp"
	"strings"
)

// TOCPlaceholder は目次の挿入位置を示す仮の要素。
const TOCPlaceholder = `<div class="toc-placeholder"></div>`

var (
	// 単独行の改ページマーカー: <!-- pagebreak --> / \pagebreak / \newpage
	pageBreakRe = regexp.MustCompile(`(?i)^\s*(<!--\s*pagebreak\s*-->|\\pagebreak|\\newpage)\s*$`)
	// 単独行の目次マーカー: <!-- toc --> / [[toc]]
	tocMarkerRe = regexp.MustCompile(`(?i)^\s*(<!--\s*toc\s*-->|\[\[toc\]\])\s*$`)
)

// Preprocessed は前処理の結果。
type Preprocessed struct {
	Markdown   string
	HasTOC     bool // 目次マーカーがあった
	HasMermaid bool // Mermaid の図があった
}

// Preprocess はコードブロック外の改ページ・目次マーカーを HTML 要素に置き換え、
// ```mermaid のコードブロックを <pre class="mermaid"> に置き換える（PDF 化の直前に図に変換する）。
func Preprocess(src string) Preprocessed {
	var b strings.Builder
	var res Preprocessed
	inFence, inMermaid, fence := false, false, ""

	for _, line := range strings.Split(src, "\n") {
		t := strings.TrimSpace(line)
		switch {
		case inMermaid:
			if strings.HasPrefix(t, fence) {
				inMermaid = false
				b.WriteString("</pre>\n\n")
			} else {
				b.WriteString(html.EscapeString(line) + "\n")
			}
			continue
		case inFence:
			if strings.HasPrefix(t, fence) {
				inFence = false
			}
		case strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~"):
			fence = t[:3]
			if strings.EqualFold(strings.TrimSpace(strings.TrimLeft(t, fence[:1])), "mermaid") {
				// HTML ブロック（<pre> から </pre> まで）として出力する
				inMermaid, res.HasMermaid = true, true
				b.WriteString("\n<pre class=\"mermaid\">\n")
				continue
			}
			inFence = true
		case pageBreakRe.MatchString(line):
			// 前後に空行を入れ、HTMLブロックが後続の段落を巻き込まないようにする
			b.WriteString("\n<div class=\"page-break\"></div>\n\n")
			continue
		case !res.HasTOC && tocMarkerRe.MatchString(line):
			res.HasTOC = true
			b.WriteString("\n" + TOCPlaceholder + "\n\n")
			continue
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	if inMermaid { // 閉じていない mermaid ブロック
		b.WriteString("</pre>\n")
	}
	res.Markdown = b.String()
	return res
}
