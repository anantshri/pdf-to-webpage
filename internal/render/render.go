// Package render rasterises a PDF into per-page WebP images.
//
// PDF pages are rendered via go-fitz (MuPDF, CGO) at the requested DPI,
// optionally downscaled to a max width using a CatmullRom kernel, and
// encoded as WebP at quality 100 (chai2010/webp, libwebp via CGO).
package render

import (
	"fmt"
	"image"
	"os"
	"path/filepath"

	"github.com/anantshri/pdf-to-webpage/internal/watermark"
	"github.com/chai2010/webp"
	"github.com/gen2brain/go-fitz"
	"golang.org/x/image/draw"
)

// Options controls how the PDF is rasterised.
type Options struct {
	DPI         float64 // render DPI passed to MuPDF (e.g. 300)
	MaxWidth    int     // pages wider than this are downscaled; 0 disables
	Quality     float32 // WebP quality 0-100
	Fingerprint string  // if non-empty, embed an invisible watermark of this string in every page
}

// Result describes the rendered output.
type Result struct {
	PageCount int
	// Width and Height of the first slide in pixels. Used for <img width height>
	// to avoid layout shift. Subsequent slides are usually the same size in
	// slide decks; CSS sets width:100%/height:auto so the attributes are
	// only a hint.
	Width  int
	Height int
}

// Pages opens pdfPath, renders every page to <imagesDir>/slide-NNN.webp, and
// returns the page count plus the dimensions of slide 1.
func Pages(pdfPath, imagesDir string, opt Options) (Result, error) {
	if opt.DPI <= 0 {
		opt.DPI = 300
	}
	if opt.Quality <= 0 || opt.Quality > 100 {
		opt.Quality = 100
	}

	if err := os.MkdirAll(imagesDir, 0o755); err != nil {
		return Result{}, fmt.Errorf("create images dir: %w", err)
	}

	doc, err := fitz.New(pdfPath)
	if err != nil {
		return Result{}, fmt.Errorf("open pdf: %w", err)
	}
	defer doc.Close()

	n := doc.NumPage()
	if n == 0 {
		return Result{}, fmt.Errorf("pdf contains no pages")
	}

	var firstW, firstH int
	for i := 0; i < n; i++ {
		img, err := doc.ImageDPI(i, opt.DPI)
		if err != nil {
			return Result{}, fmt.Errorf("render page %d: %w", i+1, err)
		}

		var out image.Image = downscale(img, opt.MaxWidth)
		if opt.Fingerprint != "" {
			marked, werr := watermark.Embed(out, opt.Fingerprint)
			if werr != nil {
				return Result{}, fmt.Errorf("watermark page %d: %w", i+1, werr)
			}
			out = marked
		}
		if i == 0 {
			b := out.Bounds()
			firstW, firstH = b.Dx(), b.Dy()
		}

		name := filepath.Join(imagesDir, fmt.Sprintf("slide-%03d.webp", i+1))
		if err := writeWebP(out, name, opt.Quality); err != nil {
			return Result{}, fmt.Errorf("encode page %d: %w", i+1, err)
		}
	}

	return Result{PageCount: n, Width: firstW, Height: firstH}, nil
}

// downscale resizes src to maxWidth keeping aspect, using CatmullRom for
// good text rendering. Returns src unchanged if it already fits.
func downscale(src image.Image, maxWidth int) image.Image {
	if maxWidth <= 0 {
		return src
	}
	b := src.Bounds()
	srcW, srcH := b.Dx(), b.Dy()
	if srcW <= maxWidth {
		return src
	}
	dstW := maxWidth
	dstH := (srcH * dstW) / srcW
	dst := image.NewRGBA(image.Rect(0, 0, dstW, dstH))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, b, draw.Over, nil)
	return dst
}

func writeWebP(img image.Image, path string, quality float32) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return webp.Encode(f, img, &webp.Options{Lossless: false, Quality: quality})
}
