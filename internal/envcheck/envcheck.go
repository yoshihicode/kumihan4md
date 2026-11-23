// Package envcheck は実行に必要な外部依存（Chrome・日本語フォント）を確認する。
package envcheck

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type checkStatus int

const (
	statusOK checkStatus = iota
	statusWarn
	statusNG
)

func (s checkStatus) label() string {
	return [...]string{"[OK]  ", "[WARN]", "[NG]  "}[s]
}

type checkResult struct {
	name, detail, hint string
	status             checkStatus
}

// Print は実行に必要な外部依存（Chrome・日本語フォント）を確認して表示する。
// 必須項目がすべて揃っていれば true を返す。
func Print(w io.Writer, explicitChrome string) bool {
	results := []checkResult{checkChrome(explicitChrome), checkJapaneseFont()}

	fmt.Fprintln(w, "Environment check:")
	allOK := true
	for _, r := range results {
		fmt.Fprintf(w, "  %s %s: %s\n", r.status.label(), r.name, r.detail)
		if r.hint != "" {
			for _, line := range strings.Split(r.hint, "\n") {
				fmt.Fprintf(w, "         → %s\n", line)
			}
		}
		if r.status == statusNG {
			allOK = false
		}
	}

	return allOK
}

// ---- Chrome ----

func chromeCandidates() []string {
	switch runtime.GOOS {
	case "darwin":
		return []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
			"google-chrome", "chromium",
		}
	case "windows":
		var c []string
		for _, env := range []string{"ProgramFiles", "ProgramFiles(x86)", "LocalAppData"} {
			if d := os.Getenv(env); d != "" {
				c = append(c, filepath.Join(d, `Google\Chrome\Application\chrome.exe`))
			}
		}
		return append(c, "chrome.exe", "chrome")
	default:
		return []string{
			"google-chrome", "google-chrome-stable",
			"chromium", "chromium-browser",
			"headless-shell", "headless_shell",
		}
	}
}

// FindChrome は使用する Chrome のパスを返す。-chrome 指定があればそれを優先する。
func FindChrome(explicit string) (string, error) {
	if explicit != "" {
		if p, err := exec.LookPath(explicit); err == nil {
			return p, nil
		}
		return "", fmt.Errorf("Chrome specified by -chrome not found: %s", explicit)
	}
	for _, c := range chromeCandidates() {
		if p, err := exec.LookPath(c); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("Chrome/Chromium not found")
}

func checkChrome(explicit string) checkResult {
	r := checkResult{name: "Chrome/Chromium (required)"}
	p, err := FindChrome(explicit)
	if err != nil {
		r.status = statusNG
		r.detail = err.Error()
		r.hint = chromeInstallHint()
		return r
	}
	r.detail = p
	if v := chromeVersion(p); v != "" {
		r.detail += " (" + v + ")"
		if m := majorVersion(v); m > 0 && m < minChromeMajor {
			r.status = statusWarn
			r.hint = fmt.Sprintf("Chrome %d or later is required for page numbers; please update Chrome", minChromeMajor)
		}
	}
	// snap 版 Chromium は WSL で起動が遅い・失敗しやすい
	if real, err := filepath.EvalSymlinks(p); err == nil && strings.HasPrefix(real, "/snap/") {
		r.status = statusWarn
		r.hint = "This is the snap version of Chromium. On WSL, the .deb version of Google Chrome is recommended.\n" + chromeInstallHint()
	}
	return r
}

// minChromeMajor はページ番号の描画（CSS の @page マージンボックス）に必要な Chrome のメジャーバージョン。
const minChromeMajor = 131

// majorVersion は "Google Chrome 131.0.6778.85" のような文字列からメジャーバージョンを取り出す（失敗時は 0）。
func majorVersion(v string) int {
	m := regexp.MustCompile(`(\d+)\.\d+\.\d+`).FindStringSubmatch(v)
	if m == nil {
		return 0
	}
	n, _ := strconv.Atoi(m[1])
	return n
}

func chromeVersion(path string) string {
	if runtime.GOOS == "windows" { // Windows の chrome.exe は --version を出力しない
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, path, "--version").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func chromeInstallHint() string {
	switch runtime.GOOS {
	case "linux":
		return "wget https://dl.google.com/linux/direct/google-chrome-stable_current_amd64.deb\n" +
			"sudo apt install -y ./google-chrome-stable_current_amd64.deb"
	default:
		return "install Google Chrome, or specify its path with -chrome"
	}
}

// ---- 日本語フォント ----

func checkJapaneseFont() checkResult {
	r := checkResult{name: "Japanese fonts (required)"}
	if runtime.GOOS != "linux" {
		r.detail = "using the OS built-in Japanese fonts"
		return r
	}
	if _, err := exec.LookPath("fc-list"); err != nil {
		r.status = statusNG
		r.detail = "cannot check because fc-list is not installed"
		r.hint = "sudo apt install -y fontconfig fonts-noto-cjk"
		return r
	}
	out, err := exec.Command("fc-list", ":lang=ja", "family").Output()
	if err != nil {
		r.status = statusNG
		r.detail = "failed to run fc-list: " + err.Error()
		return r
	}
	families := strings.Split(strings.TrimSpace(string(out)), "\n")
	if strings.TrimSpace(string(out)) == "" {
		r.status = statusNG
		r.detail = "not found (text will be rendered as empty boxes)"
		r.hint = "sudo apt install -y fonts-noto-cjk"
		return r
	}
	s := string(out)
	if strings.Contains(s, "Noto Sans CJK JP") || strings.Contains(s, "Noto Sans JP") {
		r.detail = fmt.Sprintf("Noto Sans CJK JP found (%d Japanese font families)", len(families))
		return r
	}
	r.status = statusWarn
	r.detail = fmt.Sprintf("%d found (recommended Noto Sans CJK JP is missing)", len(families))
	r.hint = "sudo apt install -y fonts-noto-cjk"
	return r
}
