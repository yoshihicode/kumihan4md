package markdown

import (
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

// Preprocess はコードブロック外の改ページ・目次マーカーを HTML 要素に置き換える。
// 目次マーカーがあった場合は true を返す。
func Preprocess(src string) (string, bool) {
	var b strings.Builder
	inFence, fence, hasTOC := false, "", false
	for _, line := range strings.Split(src, "\n") {
		t := strings.TrimSpace(line)
		switch {
		case inFence:
			if strings.HasPrefix(t, fence) {
				inFence = false
			}
		case strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~"):
			inFence, fence = true, t[:3]
		case pageBreakRe.MatchString(line):
			// 前後に空行を入れ、HTMLブロックが後続の段落を巻き込まないようにする
			b.WriteString("\n<div class=\"page-break\"></div>\n\n")
			continue
		case !hasTOC && tocMarkerRe.MatchString(line):
			hasTOC = true
			b.WriteString("\n" + TOCPlaceholder + "\n\n")
			continue
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String(), hasTOC
}
