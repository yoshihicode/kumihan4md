// kumihan4md: Markdown を改ページ制御付きで PDF に組版する CLI（コマンド名: kumi）
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/yoshihicode/kumihan4md/internal/app"
	"github.com/yoshihicode/kumihan4md/internal/envcheck"
)

// GoReleaser のビルド時に -ldflags "-X main.Version=... -X main.Revision=..." で埋め込まれる
var (
	Version  = "dev"
	Revision = ""
)

func main() {
	var o app.Options
	var check, showVersion bool

	flag.BoolVar(&showVersion, "version", false, "print version and exit")

	flag.BoolVar(&check, "check", false, "check the runtime environment (Chrome, Japanese fonts) and exit")
	flag.StringVar(&o.Output, "o", "", "output PDF path (default: input file name with .pdf)")
	flag.StringVar(&o.CSSPath, "css", "", "additional CSS file (loaded after the default CSS)")
	flag.StringVar(&o.PaperSize, "size", "A4", `paper size (e.g. "A4", "A4 landscape", "B5", "Letter")`)
	flag.StringVar(&o.Title, "title", "", "document title (default: front matter title, then the first h1)")
	flag.BoolVar(&o.H1Break, "h1-break", true, "insert a page break before every h1 except the first")
	flag.IntVar(&o.KeepLines, "keep-lines", 0, "move a heading to the next page if fewer than N lines of space remain below it (0 to disable)")
	flag.StringVar(&o.KeepLevels, "keep-levels", "2,3", "heading levels affected by -keep-lines (comma-separated, 1-6)")
	flag.BoolVar(&o.PageNumbers, "page-number", true, "show page numbers in the footer")
	flag.StringVar(&o.HighlightStyle, "hl-style", "github", `syntax highlighting style (chroma style name, "none" to disable)`)
	flag.BoolVar(&o.TOC, "toc", false, "generate a table of contents (at <!-- toc --> if present, otherwise at the top)")
	flag.IntVar(&o.TOCDepth, "toc-depth", 3, "maximum heading level included in the TOC (1-6)")
	flag.StringVar(&o.TOCTitle, "toc-title", "目次", "TOC heading text")
	flag.BoolVar(&o.Cover, "cover", false, "generate a cover page from the front matter")
	flag.StringVar(&o.CoverLabels, "cover-labels", "", `cover labels as key=value pairs (keys: version, date, author; e.g. "version=版,date=日付,author=作成者")`)
	flag.BoolVar(&o.Outline, "outline", true, "generate PDF bookmarks (outline) from headings")
	flag.StringVar(&o.HTMLOut, "html", "", "save the intermediate HTML to the given path (for debugging)")
	flag.StringVar(&o.Chrome, "chrome", "", "path to the Chrome/Chromium executable (default: auto-detect)")
	flag.DurationVar(&o.Timeout, "timeout", 60*time.Second, "conversion timeout")

	flag.Usage = func() {
		w := flag.CommandLine.Output()
		fmt.Fprintf(w, "Usage: kumi [options] input.md\n\nOptions:\n")
		flag.PrintDefaults()
		fmt.Fprintln(w)
		envcheck.Print(w, o.Chrome)
	}
	flag.Parse()

	if showVersion {
		if Revision != "" {
			fmt.Printf("kumi %s (%s)\n", Version, Revision)
		} else {
			fmt.Printf("kumi %s\n", Version)
		}
		return
	}
	if check {
		if envcheck.Print(os.Stdout, o.Chrome) {
			os.Exit(0)
		}
		os.Exit(1)
	}
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}
	o.Input = flag.Arg(0)

	if err := app.Run(o); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
