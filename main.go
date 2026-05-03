// pdf-to-webpage — turn a PDF into a self-contained, full-width slide
// viewer webpage. Single binary, no runtime dependencies.
//
//	pdf-to-webpage [-o OUT] [-dpi 300] [-width 1920] [-title "Title"]
//	               [-header header.html] [-footer footer.html] [-force]
//	               <slides.pdf>
package main

import (
	"crypto/rand"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	_ "github.com/chai2010/webp"

	"github.com/anantshri/pdf-to-webpage/internal/render"
	"github.com/anantshri/pdf-to-webpage/internal/site"
	"github.com/anantshri/pdf-to-webpage/internal/watermark"
)

const (
	pdfDownloadName = "slides.pdf"
	manifestName    = "fingerprint.json"
)

func main() {
	os.Setenv("GODEBUG", "asyncpreemptoff=1")
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		outDir         string
		dpi            float64
		maxWidth       int
		title          string
		headerFile     string
		footerFile     string
		force          bool
		allowDownload  bool
		fingerprint    string
		noFingerprint  bool
		extractPath    string
	)
	flag.StringVar(&outDir, "o", "", "output folder (default: derived from PDF basename)")
	flag.Float64Var(&dpi, "dpi", 300, "render DPI for PDF rasterisation")
	flag.IntVar(&maxWidth, "width", 1920, "max image width in px (0 to disable downscaling)")
	flag.StringVar(&title, "title", "", "page title (default: derived from PDF basename)")
	flag.StringVar(&headerFile, "header", "", "HTML file injected above the slide viewer")
	flag.StringVar(&footerFile, "footer", "", "HTML file injected below the slide viewer")
	flag.BoolVar(&force, "force", false, "wipe and overwrite an existing output folder")
	flag.BoolVar(&allowDownload, "allow-download", true, "include the PDF in output and show the download button")
	flag.StringVar(&fingerprint, "fingerprint", "", "watermark identifier embedded in every slide (default: random UUID)")
	flag.BoolVar(&noFingerprint, "no-fingerprint", false, "disable invisible watermarking")
	flag.StringVar(&extractPath, "extract", "", "extract the fingerprint hash from a watermarked image and exit")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: %s [flags] <slides.pdf>\n", filepath.Base(os.Args[0]))
		fmt.Fprintf(os.Stderr, "       %s -extract <image>\n\n", filepath.Base(os.Args[0]))
		flag.PrintDefaults()
	}
	flag.Parse()

	if extractPath != "" {
		return runExtract(extractPath)
	}

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

	effectiveFingerprint := ""
	if !noFingerprint {
		if fingerprint != "" {
			effectiveFingerprint = fingerprint
		} else {
			gen, err := newUUID()
			if err != nil {
				return fmt.Errorf("generate fingerprint: %w", err)
			}
			effectiveFingerprint = gen
		}
	}

	imagesDir := filepath.Join(outDir, "images")
	fmt.Printf("[pdf-to-webpage] rasterising %s @ %.0f dpi → %s\n", pdfPath, dpi, imagesDir)
	res, err := render.Pages(pdfPath, imagesDir, render.Options{
		DPI:         dpi,
		MaxWidth:    maxWidth,
		Quality:     100,
		Fingerprint: effectiveFingerprint,
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

	if effectiveFingerprint != "" {
		hash := watermark.HashFingerprint(effectiveFingerprint)
		if err := writeManifest(outDir, effectiveFingerprint, hash, res.PageCount, pdfPath); err != nil {
			return fmt.Errorf("write manifest: %w", err)
		}
		fmt.Printf("[pdf-to-webpage] fingerprint: %s (hash: %s)\n", effectiveFingerprint, hash)
	}

	fmt.Printf("[pdf-to-webpage] done → %s/\n", outDir)
	fmt.Printf("    serve with: cd %s && python3 -m http.server\n", outDir)
	return nil
}

// runExtract reads an image (WebP/PNG/JPEG) and prints the embedded
// fingerprint hash on success, or returns an error if no watermark is
// detected.
func runExtract(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	hash, ok, err := watermark.Extract(img)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("no watermark detected in %s", path)
	}
	fmt.Println(hash)
	return nil
}

// newUUID returns a freshly generated RFC 4122 v4 UUID string.
func newUUID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

type manifest struct {
	Version     int     `json:"version"`
	Fingerprint string  `json:"fingerprint"`
	Hash        string  `json:"hash"`
	Algorithm   string  `json:"algorithm"`
	Delta       float64 `json:"delta"`
	Created     string  `json:"created"`
	SlideCount  int     `json:"slide_count"`
	SourcePDF   string  `json:"source_pdf"`
}

func writeManifest(dir, fingerprint, hash string, slideCount int, sourcePDF string) error {
	m := manifest{
		Version:     1,
		Fingerprint: fingerprint,
		Hash:        hash,
		Algorithm:   watermark.Algorithm,
		Delta:       watermark.Delta,
		Created:     time.Now().UTC().Format(time.RFC3339),
		SlideCount:  slideCount,
		SourcePDF:   filepath.Base(sourcePDF),
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(filepath.Join(dir, manifestName), data, 0o644)
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
