// Package pdf はヘッドレス Chrome（chromedp）で HTML を PDF に印刷する。
package pdf

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/chromedp/chromedp"

	"github.com/yoshihicode/kumihan4md/internal/mermaid"
)

// Options は PDF 出力の設定。
type Options struct {
	ChromePath string
	Outline    bool
	Mermaid    bool // Mermaid の図を描画する（図がある文書だけ mermaid.js を読み込む）
}

// RenderHTML は HTML を baseDir に一時ファイルとして保存し、PDF に変換する。
// 一時ファイルを baseDir に置くのは、相対パスの画像を正しく読み込むため。
func RenderHTML(ctx context.Context, html []byte, baseDir string, o Options) ([]byte, error) {
	tmp, err := os.CreateTemp(baseDir, ".kumi-*.html")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(html); err != nil {
		tmp.Close()
		return nil, err
	}
	if err := tmp.Close(); err != nil {
		return nil, err
	}
	return Render(ctx, tmp.Name(), o)
}

// waitAssetsJS はフォントと画像の読み込み完了を待つ（未完了のまま印刷すると改ページ位置がずれる）。
const waitAssetsJS = `document.fonts.ready
	.then(() => Promise.all(Array.from(document.images).map(img =>
		img.complete ? null : new Promise(r => { img.onload = img.onerror = r; }))))
	.then(() => true)`

// Render は HTML ファイルを Chrome で開いて PDF に印刷する（chromedp v0.24 以降の API）。
func Render(ctx context.Context, htmlPath string, o Options) ([]byte, error) {
	allocOpts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("allow-file-access-from-files", true),
	)
	if o.ChromePath != "" {
		allocOpts = append(allocOpts, chromedp.ExecPath(o.ChromePath))
	}
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(ctx, allocOpts...)
	defer cancelAlloc()
	cctx, cancelCtx := chromedp.NewContext(allocCtx)
	defer cancelCtx()

	if err := chromedp.Do(cctx,
		chromedp.Navigate(fileURL(htmlPath)),
		chromedp.WaitReady("body"),
	); err != nil {
		return nil, err
	}
	if _, err := chromedp.Run(cctx, chromedp.Evaluate[bool](waitAssetsJS, chromedp.EvalAwaitPromise)); err != nil {
		return nil, err
	}
	if o.Mermaid {
		if err := renderMermaid(cctx); err != nil {
			return nil, fmt.Errorf("mermaid: %w", err)
		}
	}

	// ページ番号は CSS の @page マージンボックスで描くため、
	// Chrome のヘッダー/フッター機能は使わない
	opts := []chromedp.PDFOption{
		chromedp.PDFPrintBackground(),
		chromedp.PDFPreferCSSPageSize(), // @page の size / margin を優先
	}
	if o.Outline {
		opts = append(opts, chromedp.PDFOutlineAndTagged()) // しおり＋タグ付きPDF
	}
	return chromedp.Run(cctx, chromedp.PrintToPDF(opts...))
}

// mermaidRunJS は <pre class="mermaid"> を SVG の図に変換し、描画の完了を待つ。
const mermaidRunJS = `mermaid.initialize({
	startOnLoad: false,
	securityLevel: "strict",
	theme: "default",
	fontFamily: '"Noto Sans JP", "Noto Sans CJK JP", sans-serif',
});
mermaid.run({ querySelector: "pre.mermaid" })
	.catch((e) => { throw new Error((e && (e.message || e.str)) || String(e)); }) // エラー内容を文字列にする
	.then(() => document.fonts.ready)
	.then(() => true)`

// renderMermaid は埋め込んだ mermaid.js をページに読み込み、図を描画する。
func renderMermaid(ctx context.Context) error {
	src, err := mermaid.Script()
	if err != nil {
		return err
	}
	// スクリプトの最後の式の値（オブジェクト）を返さないよう、末尾に true を付ける
	if _, err := chromedp.Run(ctx, chromedp.Evaluate[bool](src+"\n;true")); err != nil {
		return fmt.Errorf("failed to load mermaid.js: %w", err)
	}
	if _, err := chromedp.Run(ctx, chromedp.Evaluate[bool](mermaidRunJS, chromedp.EvalAwaitPromise)); err != nil {
		return fmt.Errorf("failed to render diagram: %w", err)
	}
	return nil
}

func fileURL(path string) string {
	p := filepath.ToSlash(path)
	if !strings.HasPrefix(p, "/") { // Windows: C:/... → /C:/...
		p = "/" + p
	}
	return (&url.URL{Scheme: "file", Path: p}).String()
}
