// Package app は Markdown → HTML → PDF の変換処理全体をまとめる。
package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yoshihicode/kumihan4md/internal/envcheck"
	"github.com/yoshihicode/kumihan4md/internal/markdown"
	"github.com/yoshihicode/kumihan4md/internal/page"
	"github.com/yoshihicode/kumihan4md/internal/pdf"
)

// Options はコマンドラインから受け取る設定。
type Options struct {
	Input, Output, CSSPath, PaperSize, Title, HTMLOut, Chrome string
	HighlightStyle, TOCTitle, KeepLevels, BreakLevels         string
	CoverLabels                                               string // "version=版,date=日付,author=作成者" 形式
	PageNumbers, TOC, Cover, Outline                          bool
	TOCDepth, KeepLines, CodeKeepLines                        int
	Timeout                                                   time.Duration
}

// Run は Options に従って Markdown を PDF に変換する。
func Run(o Options) error {
	if err := validate(o); err != nil {
		return err
	}

	// Chrome を先に解決しておく（見つからない場合は変換前に失敗させる）
	chrome, err := envcheck.FindChrome(o.Chrome)
	if err != nil {
		return fmt.Errorf("%w\n  run 'kumi -check' to check the runtime environment", err)
	}

	src, err := os.ReadFile(o.Input)
	if err != nil {
		return err
	}
	if o.Output == "" {
		o.Output = strings.TrimSuffix(o.Input, filepath.Ext(o.Input)) + ".pdf"
	}

	// Markdown → HTML
	pre := markdown.Preprocess(strings.ReplaceAll(string(src), "\r\n", "\n"))
	doc, err := markdown.Convert(pre.Markdown, o.HighlightStyle)
	if err != nil {
		return err
	}

	body := doc.Body
	if o.TOC || pre.HasTOC || markdown.MetaBool(doc.Meta, "toc") {
		body = markdown.InsertTOC(body, markdown.BuildTOC(doc.Headings, o.TOCDepth, o.TOCTitle))
	}

	title := resolveTitle(o, doc)

	var cover *page.Cover
	if o.Cover || markdown.MetaBool(doc.Meta, "cover") {
		labels, err := resolveCoverLabels(o.CoverLabels, markdown.MetaStringMap(doc.Meta, "cover_labels"))
		if err != nil {
			return err
		}
		cover = &page.Cover{
			Labels:       labels,
			Title:        title,
			Subtitle:     markdown.MetaString(doc.Meta, "subtitle"),
			Author:       markdown.MetaString(doc.Meta, "author"),
			Organization: markdown.MetaString(doc.Meta, "organization"),
			Date:         doc.Date("date"),
			Version:      markdown.MetaString(doc.Meta, "version"),
		}
	}

	hlCSS, err := markdown.HighlightCSS(o.HighlightStyle)
	if err != nil {
		return err
	}
	var extraCSS string
	if o.CSSPath != "" {
		b, err := os.ReadFile(o.CSSPath)
		if err != nil {
			return err
		}
		extraCSS = string(b)
	}

	// HTML 文書の組み立て
	html, err := page.Build(page.Options{
		Title:        title,
		Body:         body,
		Cover:        cover,
		HighlightCSS: hlCSS,
		ExtraCSS:     extraCSS,
		PaperSize:    o.PaperSize,
		PageNumbers:  o.PageNumbers,
		BreakLevels:  o.BreakLevels,
		KeepLines:    o.KeepLines,
		KeepLevels:   o.KeepLevels,
		CodeKeep:     o.CodeKeepLines,
	})
	if err != nil {
		return err
	}
	if o.HTMLOut != "" {
		if err := os.WriteFile(o.HTMLOut, html, 0o644); err != nil {
			return err
		}
	}

	// HTML → PDF（相対パスの画像を解決するため、Markdown のあるフォルダを基準にする）
	absIn, err := filepath.Abs(o.Input)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), o.Timeout)
	defer cancel()
	out, err := pdf.RenderHTML(ctx, html, filepath.Dir(absIn), pdf.Options{
		ChromePath: chrome,
		Outline:    o.Outline,
		Mermaid:    pre.HasMermaid,
	})
	if err != nil {
		return fmt.Errorf("failed to generate PDF: %w", err)
	}
	if err := os.WriteFile(o.Output, out, 0o644); err != nil {
		return err
	}
	fmt.Println("Output:", o.Output)
	return nil
}

func validate(o Options) error {
	if !page.ValidPaperSize(o.PaperSize) {
		return fmt.Errorf("invalid paper size: %q", o.PaperSize)
	}
	if o.TOCDepth < 1 || o.TOCDepth > 6 {
		return fmt.Errorf("-toc-depth must be between 1 and 6")
	}
	if o.CodeKeepLines < 0 {
		return fmt.Errorf("-code-keep-lines must be 0 or greater")
	}
	if o.KeepLines < 0 || o.KeepLines > 20 {
		return fmt.Errorf("-keep-lines must be between 0 and 20")
	}
	if _, err := page.ParseLevels(o.KeepLevels); err != nil {
		return fmt.Errorf("-keep-levels: %w", err)
	}
	if _, err := page.ParseLevels(o.BreakLevels); err != nil {
		return fmt.Errorf("-break-levels: %w", err)
	}
	if _, err := parseLabelFlag(o.CoverLabels); err != nil {
		return err
	}
	return markdown.ValidateStyle(o.HighlightStyle)
}

// resolveCoverLabels は表紙の項目名を -cover-labels > front matter の cover_labels > 既定値（英語）の順で決める。
func resolveCoverLabels(flagValue string, fm map[string]string) (page.CoverLabels, error) {
	labels := page.DefaultCoverLabels
	for k, v := range fm {
		if err := labels.Set(k, v); err != nil {
			return labels, fmt.Errorf("front matter cover_labels: %w", err)
		}
	}
	fromFlag, err := parseLabelFlag(flagValue)
	if err != nil {
		return labels, err
	}
	for k, v := range fromFlag {
		if err := labels.Set(k, v); err != nil {
			return labels, fmt.Errorf("-cover-labels: %w", err)
		}
	}
	return labels, nil
}

// parseLabelFlag は "version=版,date=日付" 形式の文字列をマップにする。
func parseLabelFlag(s string) (map[string]string, error) {
	out := map[string]string{}
	if strings.TrimSpace(s) == "" {
		return out, nil
	}
	for _, pair := range strings.Split(s, ",") {
		k, v, ok := strings.Cut(pair, "=")
		k, v = strings.TrimSpace(k), strings.TrimSpace(v)
		if !ok || k == "" {
			return nil, fmt.Errorf("invalid -cover-labels: %q (use key=value pairs, e.g. \"version=版,date=日付,author=作成者\")", s)
		}
		if err := (&page.CoverLabels{}).Set(k, v); err != nil {
			return nil, fmt.Errorf("-cover-labels: %w", err)
		}
		out[k] = v
	}
	return out, nil
}

// resolveTitle はタイトルを -title > front matter > 最初の h1 > ファイル名 の順で決める。
func resolveTitle(o Options, doc *markdown.Document) string {
	if o.Title != "" {
		return o.Title
	}
	if t := markdown.MetaString(doc.Meta, "title"); t != "" {
		return t
	}
	for _, h := range doc.Headings {
		if h.Level == 1 {
			return h.Text
		}
	}
	return filepath.Base(o.Input)
}
