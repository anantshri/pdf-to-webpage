# AGENTS.md — Project context for AI coding agents

## Overview

`pdf-to-webpage` is a Go CLI tool that converts PDF presentations into self-contained slide viewer webpages. It renders PDF pages as WebP images and generates an HTML/CSS/JS viewer.

## Build

```bash
CGO_ENABLED=1 go build -o pdf-to-webpage .
```

CGO is required — do not attempt `CGO_ENABLED=0` builds.

## Module info

- **Module:** `pdf-to-webpage`
- **Go version:** 1.24

## Dependencies

| Package | Role |
|---------|------|
| `github.com/gen2brain/go-fitz` | PDF rendering via MuPDF (CGO, bundles static `.a` libs per platform) |
| `github.com/chai2010/webp` | WebP image encoding via libwebp (CGO, bundles C source) |
| `golang.org/x/image` | CatmullRom downscaling kernel |

## Package layout

```
internal/
├── assets/     # Embedded HTML template, CSS, JS (embed.FS)
├── render/     # PDF → WebP rasterisation (go-fitz + chai2010/webp)
└── site/       # HTML generation and file output
```

- `render.Pages(pdfPath, imagesDir, Options) → Result` — renders all pages
- `site.Write(outDir, PageData) → error` — generates index.html, copies CSS/JS
- `site.CopyPDF(src, outDir, name) → error` — copies the source PDF

## CLI flags

`-o`, `-dpi`, `-width`, `-title`, `-header`, `-footer`, `-force`, `-allow-download` — see `main.go` flag definitions.

`-allow-download` (default true) gates both the PDF copy in `outDir` and the download button in the rendered template via `PageData.AllowDownload`.

## Output

A directory containing `index.html`, `viewer.css`, `viewer.js`, `slides.pdf`, and `images/slide-NNN.webp`.

## Known quirks

- `os.Setenv("GODEBUG", "asyncpreemptoff=1")` in `main()` — disables async goroutine preemption to prevent crashes when MuPDF's C signal handlers conflict with Go's runtime. Do not remove.
