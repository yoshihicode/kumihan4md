package markdown

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// DateLayout は引用符なしで書かれた日付の表示形式（例: November 20, 2026）。
const DateLayout = "January 2, 2006"

// quotedKeyRe は front matter のトップレベルで、値が引用符で始まる行に一致する。
var quotedKeyRe = regexp.MustCompile(`^([A-Za-z0-9_-]+):\s*["']`)

// quotedMetaKeys は front matter の元のテキストから、値を引用符で囲んで書いたキーを集める。
// YAML のライブラリは date: 2026-11-20 と date: "2026-11-20" をどちらも同じ文字列で返すため、
// 引用符の有無は元のテキストで判定する。
func quotedMetaKeys(src string) map[string]bool {
	keys := map[string]bool{}
	lines := strings.Split(src, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return keys
	}
	for _, line := range lines[1:] {
		if t := strings.TrimSpace(line); t == "---" || t == "..." {
			break
		}
		if m := quotedKeyRe.FindStringSubmatch(line); m != nil {
			keys[m[1]] = true
		}
	}
	return keys
}

// MetaString は front matter の値を文字列で返す。
func MetaString(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	return fmt.Sprint(v)
}

// Date は front matter の日付を表示用の文字列で返す。
// 引用符で囲んで書いた場合は書いたとおりに、囲まずに YYYY-MM-DD で書いた場合は
// DateLayout の形式（例: November 20, 2026）にする。それ以外の書き方はそのまま返す。
func (d *Document) Date(key string) string {
	v, ok := d.Meta[key]
	if !ok || v == nil {
		return ""
	}
	if t, ok := v.(time.Time); ok {
		return t.Format(DateLayout)
	}
	s := fmt.Sprint(v)
	if d.quoted[key] {
		return s
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t.Format(DateLayout)
	}
	return s
}

// MetaStringMap は front matter の入れ子のマップ（例: cover_labels）を文字列のマップで返す。
// YAML のライブラリは入れ子のマップを map[interface{}]interface{} で返すため、両方の型に対応する。
func MetaStringMap(m map[string]any, key string) map[string]string {
	out := map[string]string{}
	switch v := m[key].(type) {
	case map[string]any:
		for k, x := range v {
			out[k] = fmt.Sprint(x)
		}
	case map[any]any:
		for k, x := range v {
			out[fmt.Sprint(k)] = fmt.Sprint(x)
		}
	}
	return out
}

// MetaBool は front matter の値を真偽値で返す（未指定や型違いは false）。
func MetaBool(m map[string]any, key string) bool {
	b, _ := m[key].(bool)
	return b
}
