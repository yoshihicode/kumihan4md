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
	Title, Subtitle, Author, Organization, Date, Version, Classification string
	Labels                                                               CoverLabels
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

// classificationCSS は機密区分（社外秘など）を各ページのヘッダー右上に描く（@page マージンボックス）。
// %s には cssString でエスケープ済みの文字列が入る。表紙は余白が 0 のため描かない。
const classificationCSS = `
@page {
  @top-right {
    content: %s;
    font-family: "Noto Sans JP", "Noto Sans CJK JP", sans-serif;
    font-size: 9pt;
    font-weight: 700;
    color: #c62828;
  }
}
@page cover {
  @top-right {
    content: none;
  }
}
`

// cssString は s を CSS の文字列リテラル（"..."）にする。
// ASCII の英数字・空白と非 ASCII 文字以外はすべて 16 進エスケープし、
// 引用符や "</style>" で CSS・HTML が壊れないようにする。
func cssString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == ' ', r > 0x7f:
			b.WriteRune(r)
		default:
			fmt.Fprintf(&b, "\\%06x", r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// Options は HTML 文書の組み立て設定。
type Options struct {
	Title          string
	Body           string // 本文の HTML
	Cover          *Cover // nil なら表紙なし
	HighlightCSS   string
	ExtraCSS       string // -css で指定された追加 CSS
	PaperSize      string
	PageNumbers    bool
	Classification string // 機密区分（社外秘など）。空でなければ各ページのヘッダーに表示する
	BreakLevels    string // "1,2" 形式。この見出しレベルの前で改ページする（空なら改ページしない）
	KeepLines      int
	KeepLevels     string // "2,3" 形式
	CodeKeep       int    // この行数以下のコードブロックはページで分割しない（0 なら常に分割可）
}

// ValidPaperSize は用紙サイズの文字列を検証する。CSS に直接埋め込むため英数字・空白・ピリオドのみ許可する。
func ValidPaperSize(s string) bool {
	return sizeRe.MatchString(s)
}

// ParseLevels は "2,3" のような見出しレベル指定を検証し、正規表現の文字クラス用に "23" を返す。
// 空文字（空白のみを含む）は「対象なし」として "" を返す。
func ParseLevels(s string) (string, error) {
	var b strings.Builder
	if strings.TrimSpace(s) == "" {
		return "", nil
	}
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if len(p) != 1 || p[0] < '1' || p[0] > '6' {
			return "", fmt.Errorf("invalid heading levels: %q (use comma-separated levels 1-6, e.g. \"2,3\")", s)
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
		return nil, fmt.Errorf("embedded template.txt is broken; check internal/page/assets/template.txt (an editor may have reformatted it) and rebuild")
	}
	body := o.Body
	// 改ページの判定は、-keep-lines で見出しを包む前に行う
	levels, err := ParseLevels(o.BreakLevels)
	if err != nil {
		return nil, err
	}
	body = markBreaks(body, levels)
	body = markLeadIns(body)
	if o.CodeKeep > 0 {
		body = markShortCode(body, o.CodeKeep)
	}
	if o.KeepLines > 0 {
		if body, err = wrapHeadings(body, o.KeepLevels); err != nil {
			return nil, err
		}
	}

	// CSS: ハイライト → 既定 → ページ番号 → 機密区分 → -keep-lines → 追加 → 用紙サイズ の順（後勝ち）
	var css strings.Builder
	css.WriteString(o.HighlightCSS)
	css.WriteString(defaultCSS)
	if o.PageNumbers {
		css.WriteString(pageNumberCSS)
	}
	if c := strings.TrimSpace(o.Classification); c != "" {
		fmt.Fprintf(&css, classificationCSS, cssString(c))
	}
	if o.KeepLines > 0 {
		fmt.Fprintf(&css, "\n:root { --keep-lines: %d; }\n", o.KeepLines)
	}
	if o.ExtraCSS != "" {
		css.WriteString("\n" + o.ExtraCSS)
	}
	fmt.Fprintf(&css, "\n@page { size: %s; }\n", o.PaperSize)

	tpl, err := template.New("page").Parse(pageTemplate)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	err = tpl.Execute(&out, map[string]any{
		"Title":     o.Title,
		"CSS":       template.CSS(css.String()),
		"BodyClass": "",
		"Cover":     o.Cover,
		"Body":      template.HTML(body),
	})
	return out.Bytes(), err
}

var (
	headingTagRe = regexp.MustCompile(`<h([1-6])((?:\s[^>]*)?)>`)
	// 改ページの判定で「内容なし」とみなすもの: 手動改ページ・目次（それぞれ自前で改ページする）
	pageBreakDiv = `<div class="page-break"></div>`
	tocNavRe     = regexp.MustCompile(`(?s)<nav class="toc">.*?</nav>`)
)

// markBreaks は levels（"12" 形式）に含まれるレベルの見出しに break-before クラスを付ける。
// 次の場合は、不自然な改ページになるため付けない。
//   - 文書の最初の見出し（前に本文がない）: 先頭に白紙ページができるのを防ぐ
//   - 見出しの直後（間に本文がない）: 「# タイトル」や本文のない見出しだけのページを防ぐ
//   - 手動改ページ・目次の直後: 改ページが二重になるのを防ぐ
func markBreaks(body, levels string) string {
	if levels == "" {
		return body
	}
	var b strings.Builder
	prevEnd, prevLevel := 0, 0 // 直前の見出しの終了位置とレベル（0 は見出しなし）
	for _, loc := range headingTagRe.FindAllStringSubmatchIndex(body, -1) {
		start, end := loc[0], loc[1]
		level := int(body[loc[2]] - '0')
		attrs := body[loc[4]:loc[5]]
		between := strings.TrimSpace(body[prevEnd:start])

		brk := strings.ContainsRune(levels, rune('0'+level))
		switch {
		case !brk:
		case between == "" && prevLevel > 0:
			brk = false // 見出しの直後（「# タイトル」や本文のない見出しだけのページを作らない）
		case strings.HasSuffix(between, pageBreakDiv), strings.HasSuffix(between, "</nav>"):
			brk = false // 手動改ページ・目次の直後
		case prevLevel == 0 && strings.TrimSpace(tocNavRe.ReplaceAllString(strings.ReplaceAll(between, pageBreakDiv, ""), "")) == "":
			brk = false // 文書の最初の見出し
		}

		b.WriteString(body[prevEnd:start])
		if brk {
			b.WriteString(addClass("<h"+string(body[loc[2]])+attrs+">", "break-before"))
		} else {
			b.WriteString(body[start:end])
		}
		// 見出しの終了タグの直後までを「見出し」として扱う
		closeTag := "</h" + string(body[loc[2]]) + ">"
		if i := strings.Index(body[end:], closeTag); i >= 0 {
			b.WriteString(body[end : end+i+len(closeTag)])
			prevEnd = end + i + len(closeTag)
		} else {
			prevEnd = end
		}
		prevLevel = level
	}
	b.WriteString(body[prevEnd:])
	return b.String()
}

// addClass は開始タグに class を追加する（class 属性がなければ作る）。
func addClass(tag, class string) string {
	if i := strings.Index(tag, `class="`); i >= 0 {
		i += len(`class="`)
		return tag[:i] + class + " " + tag[i:]
	}
	return strings.TrimSuffix(tag, ">") + ` class="` + class + `">`
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

var paraRe = regexp.MustCompile(`(?s)<p>(.*?)</p>`)

// markLeadIns は太字だけの段落（**図の名前** のような小見出し）に lead-in クラスを付ける。
// CSS で直後の要素（図・表・リストなど）との間の改ページを避け、小見出しだけがページ末尾に残るのを防ぐ。
// 「**注意**：本文」のように太字の後に文字が続く段落は対象外（CSS だけでは区別できないため Go 側で判定する）。
func markLeadIns(body string) string {
	return paraRe.ReplaceAllStringFunc(body, func(m string) string {
		inner := strings.TrimSpace(m[len("<p>") : len(m)-len("</p>")])
		if !strings.HasPrefix(inner, "<strong>") || !strings.HasSuffix(inner, "</strong>") ||
			strings.Count(inner, "<strong>") != 1 {
			return m
		}
		return `<p class="lead-in">` + m[len("<p>"):]
	})
}

// wrapHeadings は対象レベルの見出しを、N 行分の余白を確保する要素と一緒に囲む（-keep-lines）。
func wrapHeadings(body, levels string) (string, error) {
	lv, err := ParseLevels(levels)
	if err != nil {
		return "", err
	}
	if lv == "" {
		return body, nil
	}
	re := regexp.MustCompile(fmt.Sprintf(`(?s)<h([%[1]s])(?:\s[^>]*)?>.*?</h[%[1]s]>`, lv))
	return re.ReplaceAllString(body,
		`<div class="keep-with-next">$0<div class="keep-spacer"></div></div>`), nil
}
