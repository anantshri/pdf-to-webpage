// Package site emits the static slide-viewer site.
//
// All output paths are relative; the resulting folder works at any URL
// prefix and from a file:// URL.
package site

import (
	"fmt"
	"html/template"
	"io"
	"os"
	"path/filepath"

	"github.com/anantshri/pdf-to-webpage/internal/assets"
)

// Slide is one entry passed to the HTML template.
type Slide struct {
	Index  int    // 1-based
	Src    string // relative URL, e.g. "images/slide-001.webp"
	Width  int
	Height int
}

// PageData is rendered into index.html.tmpl.
type PageData struct {
	Title         string
	PageCount     int
	PDFName       string // e.g. "slides.pdf"; empty when AllowDownload is false
	AllowDownload bool   // when true, render the download button
	Slides        []Slide
	Header        template.HTML // raw HTML, optional
	Footer        template.HTML // raw HTML, optional
}

// Write emits index.html, viewer.css, and viewer.js into outDir.
// Caller is responsible for the images/ folder and the PDF copy.
func Write(outDir string, data PageData) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("create out dir: %w", err)
	}

	tmplBytes, err := assets.FS.ReadFile("index.html.tmpl")
	if err != nil {
		return fmt.Errorf("read template: %w", err)
	}
	tmpl, err := template.New("index.html").Parse(string(tmplBytes))
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}
	indexPath := filepath.Join(outDir, "index.html")
	out, err := os.Create(indexPath)
	if err != nil {
		return fmt.Errorf("create index.html: %w", err)
	}
	if err := tmpl.Execute(out, data); err != nil {
		out.Close()
		return fmt.Errorf("execute template: %w", err)
	}
	if err := out.Close(); err != nil {
		return fmt.Errorf("close index.html: %w", err)
	}

	for _, name := range []string{"viewer.css", "viewer.js"} {
		if err := copyEmbedded(name, filepath.Join(outDir, name)); err != nil {
			return err
		}
	}
	return nil
}

func copyEmbedded(srcInFS, dstPath string) error {
	in, err := assets.FS.Open(srcInFS)
	if err != nil {
		return fmt.Errorf("open embedded %s: %w", srcInFS, err)
	}
	defer in.Close()
	out, err := os.Create(dstPath)
	if err != nil {
		return fmt.Errorf("create %s: %w", dstPath, err)
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("copy to %s: %w", dstPath, err)
	}
	return nil
}

// CopyPDF copies the source PDF to <outDir>/<dstName>.
func CopyPDF(srcPDF, outDir, dstName string) error {
	in, err := os.Open(srcPDF)
	if err != nil {
		return fmt.Errorf("open pdf: %w", err)
	}
	defer in.Close()
	out, err := os.Create(filepath.Join(outDir, dstName))
	if err != nil {
		return fmt.Errorf("create pdf copy: %w", err)
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("copy pdf: %w", err)
	}
	return nil
}
