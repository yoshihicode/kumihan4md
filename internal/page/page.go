// Package page は本文 HTML に CSS・表紙を組み合わせ、印刷用の HTML 文書を作る。
package page

import (
	"bytes"
	_ "embed"
	"fmt"
	"html/template"
	"regexp"
	"strings"
)

//go:embed assets/style.css
var defaultCSS string

//go:embed assets/template.txt
var pageTemplate string

var sizeRe = regexp.MustCompile(`^[A-Za-z0-9 .]+$`)

// Cover は表紙に表示する情報。
type Cover struct {
	Title, Subtitle, Author, Organization, Date, Version string
	Labels                                               CoverLabels
}

// CoverLabels は表紙の項目名。
type CoverLabels struct {
	Version, Date, Author string
}

// DefaultCoverLabels は項目名の既定値（英語）。
var DefaultCoverLabels = CoverLabels{Version: "Version", Date: "Date", Author: "Author"}

// Set は "version" / "date" / "author" のキーで項目名を上書きする。
func (l *CoverLabels) Set(key, value string) error {
	switch key {
	case "version":
		l.Version = value
	case "date":
		l.Date = value
	case "author":
		l.Author = value
	default:
		return fmt.Errorf("unknown cover label key: %q (use version, date or author)", key)
	}
	return nil
}

// pageNumberCSS はページ下部中央にページ番号を描く（Chrome 131 以降の @page マージンボックス）。
const pageNumberCSS = `
@page {
  @bottom-center {
    content: counter(page);
    font-family: "Noto Sans JP", "Noto Sans CJK JP", sans-serif;
    font-size: 8pt;
    color: #666;
  }
}
`

// Options は HTML 文書の組み立て設定。
type Options struct {
	Title        string
	Body         string // 本文の HTML
	Cover        *Cover // nil なら表紙なし
	HighlightCSS string
	ExtraCSS     string // -css で指定された追加 CSS
	PaperSize    string
	PageNumbers  bool
	H1Break      bool
	KeepLines    int
	KeepLevels   string // "2,3" 形式
	CodeKeep     int    // この行数以下のコードブロックはページで分割しない（0 なら常に分割可）
}

// ValidPaperSize は用紙サイズの文字列を検証する。CSS に直接埋め込むため英数字・空白・ピリオドのみ許可する。
func ValidPaperSize(s string) bool {
	return sizeRe.MatchString(s)
}

// ParseLevels は "2,3" のような見出しレベル指定を検証し、正規表現の文字クラス用に "23" を返す。
func ParseLevels(s string) (string, error) {
	var b strings.Builder
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if len(p) != 1 || p[0] < '1' || p[0] > '6' {
			return "", fmt.Errorf("invalid -keep-levels: %q (use comma-separated levels 1-6, e.g. \"2,3\")", s)
		}
		if !strings.Contains(b.String(), p) {
			b.WriteString(p)
		}
	}
	return b.String(), nil
}

// Build は印刷用の HTML 文書を組み立てる。
func Build(o Options) ([]byte, error) {
	// 埋め込みファイルが空のままビルドされると、スタイルのない PDF が黙って出力されるため、ここで止める
	if !strings.Contains(defaultCSS, ".markdown-body") {
		return nil, fmt.Errorf("embedded style.css is empty or broken (%d bytes); check internal/page/assets/style.css and rebuild", len(defaultCSS))
	}
	if !strings.Contains(pageTemplate, "{{.CSS}}") {
		return nil, fmt.Errorf("embedded template.gohtml is broken; check internal/page/assets/template.gohtml (an editor may have reformatted it) and rebuild")
	}
	body := o.Body
	if o.CodeKeep > 0 {
		body = markShortCode(body, o.CodeKeep)
	}
	if o.KeepLines > 0 {
		var err error
		if body, err = wrapHeadings(body, o.KeepLevels); err != nil {
			return nil, err
		}
	}

	// CSS: ハイライト → 既定 → ページ番号 → -keep-lines → 追加 → 用紙サイズ の順（後勝ち）
	var css strings.Builder
	css.WriteString(o.HighlightCSS)
	css.WriteString(defaultCSS)
	if o.PageNumbers {
		css.WriteString(pageNumberCSS)
	}
	if o.KeepLines > 0 {
		fmt.Fprintf(&css, "\n:root { --keep-lines: %d; }\n", o.KeepLines)
	}
	if o.ExtraCSS != "" {
		css.WriteString("\n" + o.ExtraCSS)
	}
	fmt.Fprintf(&css, "\n@page { size: %s; }\n", o.PaperSize)

	bodyClass := ""
	if o.H1Break {
		bodyClass = "h1-break"
	}

	tpl, err := template.New("page").Parse(pageTemplate)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	err = tpl.Execute(&out, map[string]any{
		"Title":     o.Title,
		"CSS":       template.CSS(css.String()),
		"BodyClass": bodyClass,
		"Cover":     o.Cover,
		"Body":      template.HTML(body),
	})
	return out.Bytes(), err
}

var preRe = regexp.MustCompile(`(?s)<pre([^>]*)>(.*?)</pre>`)

// markShortCode は maxLines 行以下のコードブロックに keep-together クラスを付ける（ページで分割しない）。
// Mermaid の図（pre.mermaid）は CSS で常に分割しないため対象外。
func markShortCode(body string, maxLines int) string {
	return preRe.ReplaceAllStringFunc(body, func(m string) string {
		sub := preRe.FindStringSubmatch(m)
		attrs, content := sub[1], sub[2]
		if strings.Contains(attrs, "mermaid") || strings.Count(content, "\n") > maxLines {
			return m
		}
		if i := strings.Index(attrs, `class="`); i >= 0 {
			attrs = attrs[:i+len(`class="`)] + "keep-together " + attrs[i+len(`class="`):]
		} else {
			attrs += ` class="keep-together"`
		}
		return "<pre" + attrs + ">" + content + "</pre>"
	})
}

// wrapHeadings は対象レベルの見出しを、N 行分の余白を確保する要素と一緒に囲む（-keep-lines）。
func wrapHeadings(body, levels string) (string, error) {
	lv, err := ParseLevels(levels)
	if err != nil {
		return "", err
	}
	re := regexp.MustCompile(fmt.Sprintf(`(?s)<h([%[1]s])(?:\s[^>]*)?>.*?</h[%[1]s]>`, lv))
	return re.ReplaceAllString(body,
		`<div class="keep-with-next">$0<div class="keep-spacer"></div></div>`), nil
}
