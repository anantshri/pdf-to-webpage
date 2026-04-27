// pdf-to-webpage — turn a PDF into a self-contained, full-width slide
// viewer webpage. Single binary, no runtime dependencies.
//
//	pdf-to-webpage [-o OUT] [-dpi 300] [-width 1920] [-title "Title"]
//	               [-header header.html] [-footer footer.html] [-force]
//	               <slides.pdf>
package main

import (
	"flag"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"pdf-to-webpage/internal/render"
	"pdf-to-webpage/internal/site"
)

const pdfDownloadName = "slides.pdf"

func main() {
	os.Setenv("GODEBUG", "asyncpreemptoff=1")
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		outDir        string
		dpi           float64
		maxWidth      int
		title         string
		headerFile    string
		footerFile    string
		force         bool
		allowDownload bool
	)
	flag.StringVar(&outDir, "o", "", "output folder (default: derived from PDF basename)")
	flag.Float64Var(&dpi, "dpi", 300, "render DPI for PDF rasterisation")
	flag.IntVar(&maxWidth, "width", 1920, "max image width in px (0 to disable downscaling)")
	flag.StringVar(&title, "title", "", "page title (default: derived from PDF basename)")
	flag.StringVar(&headerFile, "header", "", "HTML file injected above the slide viewer")
	flag.StringVar(&footerFile, "footer", "", "HTML file injected below the slide viewer")
	flag.BoolVar(&force, "force", false, "wipe and overwrite an existing output folder")
	flag.BoolVar(&allowDownload, "allow-download", true, "include the PDF in output and show the download button")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: %s [flags] <slides.pdf>\n\n", filepath.Base(os.Args[0]))
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() != 1 {
		flag.Usage()
		return fmt.Errorf("expected exactly one PDF argument")
	}
	pdfPath := flag.Arg(0)
	if _, err := os.Stat(pdfPath); err != nil {
		return fmt.Errorf("pdf: %w", err)
	}

	base := strings.TrimSuffix(filepath.Base(pdfPath), filepath.Ext(pdfPath))
	if outDir == "" {
		outDir = slugify(base)
	}
	if title == "" {
		title = humanise(base)
	}

	if err := prepareOutDir(outDir, force); err != nil {
		return err
	}

	header, err := readOptional(headerFile)
	if err != nil {
		return fmt.Errorf("header: %w", err)
	}
	footer, err := readOptional(footerFile)
	if err != nil {
		return fmt.Errorf("footer: %w", err)
	}

	imagesDir := filepath.Join(outDir, "images")
	fmt.Printf("[pdf-to-webpage] rasterising %s @ %.0f dpi → %s\n", pdfPath, dpi, imagesDir)
	res, err := render.Pages(pdfPath, imagesDir, render.Options{
		DPI:      dpi,
		MaxWidth: maxWidth,
		Quality:  100,
	})
	if err != nil {
		return err
	}
	fmt.Printf("[pdf-to-webpage] rendered %d slides (first slide: %dx%d)\n", res.PageCount, res.Width, res.Height)

	slides := make([]site.Slide, res.PageCount)
	for i := 0; i < res.PageCount; i++ {
		slides[i] = site.Slide{
			Index:  i + 1,
			Src:    fmt.Sprintf("images/slide-%03d.webp", i+1),
			Width:  res.Width,
			Height: res.Height,
		}
	}

	pdfName := ""
	if allowDownload {
		pdfName = pdfDownloadName
	}
	if err := site.Write(outDir, site.PageData{
		Title:         title,
		PageCount:     res.PageCount,
		PDFName:       pdfName,
		AllowDownload: allowDownload,
		Slides:        slides,
		Header:        template.HTML(header),
		Footer:        template.HTML(footer),
	}); err != nil {
		return err
	}

	if allowDownload {
		if err := site.CopyPDF(pdfPath, outDir, pdfDownloadName); err != nil {
			return err
		}
	}

	fmt.Printf("[pdf-to-webpage] done → %s/\n", outDir)
	fmt.Printf("    serve with: cd %s && python3 -m http.server\n", outDir)
	return nil
}

func prepareOutDir(dir string, force bool) error {
	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return os.MkdirAll(dir, 0o755)
		}
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s exists but is not a directory", dir)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		return nil
	}
	if !force {
		return fmt.Errorf("%s is not empty (use -force to overwrite)", dir)
	}
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	return os.MkdirAll(dir, 0o755)
}

func readOptional(path string) ([]byte, error) {
	if path == "" {
		return nil, nil
	}
	return os.ReadFile(path)
}

// slugify lowercases, replaces whitespace and underscores with hyphens,
// and collapses runs of hyphens.
func slugify(s string) string {
	var b strings.Builder
	prevHyphen := false
	for _, r := range s {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(unicode.ToLower(r))
			prevHyphen = false
		case r == '-' || r == '_' || unicode.IsSpace(r):
			if !prevHyphen && b.Len() > 0 {
				b.WriteByte('-')
				prevHyphen = true
			}
		}
	}
	out := strings.TrimRight(b.String(), "-")
	if out == "" {
		out = "slides"
	}
	return out
}

// humanise turns "my-talk_2026" into "My Talk 2026" for the page title.
func humanise(s string) string {
	replaced := strings.Map(func(r rune) rune {
		if r == '-' || r == '_' {
			return ' '
		}
		return r
	}, s)
	fields := strings.Fields(replaced)
	for i, w := range fields {
		if w == "" {
			continue
		}
		runes := []rune(w)
		runes[0] = unicode.ToUpper(runes[0])
		fields[i] = string(runes)
	}
	if len(fields) == 0 {
		return "Slides"
	}
	return strings.Join(fields, " ")
}
