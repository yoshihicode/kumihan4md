package markdown

import (
	"fmt"
	"slices"
	"strings"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/styles"
)

// ValidateStyle はハイライトスタイル名を検証する（"none" は無効化を表す）。
func ValidateStyle(name string) error {
	if name == "none" || slices.Contains(styles.Names(), name) {
		return nil
	}
	return fmt.Errorf("unknown highlight style: %q (available: %s)", name, strings.Join(styles.Names(), ", "))
}

// HighlightCSS はハイライト用の CSS を生成する。"none" の場合は空文字を返す。
func HighlightCSS(name string) (string, error) {
	if name == "none" {
		return "", nil
	}
	var b strings.Builder
	f := chromahtml.New(chromahtml.WithClasses(true))
	if err := f.WriteCSS(&b, styles.Get(name)); err != nil {
		return "", err
	}
	return b.String(), nil
}
