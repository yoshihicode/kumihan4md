// Package mermaid は、バイナリに埋め込んだ mermaid.js（gzip 圧縮）を提供する。
//
// 埋め込んでいるファイルの更新手順:
//
//	curl -sL https://registry.npmjs.org/mermaid/-/mermaid-<version>.tgz | tar xz
//	gzip -9 -c package/dist/mermaid.min.js > internal/mermaid/assets/mermaid.min.js.gz
//	cp package/LICENSE internal/mermaid/assets/LICENSE-mermaid
//
// 更新したら Version も書き換えること。
package mermaid

import (
	"bytes"
	"compress/gzip"
	_ "embed"
	"io"
	"sync"
)

// Version は埋め込んでいる mermaid のバージョン。
const Version = "12.3.0"

//go:embed assets/mermaid.min.js.gz
var compressed []byte

var (
	once    sync.Once
	script  string
	loadErr error
)

// Script は mermaid.min.js の中身を返す。展開は初回の呼び出し時だけ行う。
func Script() (string, error) {
	once.Do(func() {
		r, err := gzip.NewReader(bytes.NewReader(compressed))
		if err != nil {
			loadErr = err
			return
		}
		defer r.Close()
		b, err := io.ReadAll(r)
		if err != nil {
			loadErr = err
			return
		}
		script = string(b)
	})
	return script, loadErr
}
