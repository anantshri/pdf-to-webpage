# pdf-to-webpage

Convert PDF presentations into self-contained, full-width slide viewer webpages. Single binary, no runtime dependencies.

## Features

- Renders each PDF page as WebP at configurable DPI
- CatmullRom downscaling for crisp text at any resolution
- Dark-themed viewer with keyboard navigation (arrow keys, Home/End)
- Grid overview mode (G key) with thumbnail navigation
- Fullscreen mode (F key)
- Touch/swipe support for mobile
- URL hash navigation (`#slide-3`) for direct linking
- Optional header/footer HTML injection
- Print-friendly CSS
- Self-contained output — works offline, serves from any static host

## Prerequisites

- **Go 1.24+**
- **C compiler** (GCC or Clang) — required for CGO (MuPDF and libwebp)

## Installation

Build from source:

```bash
git clone https://github.com/anantshri/pdf-to-webpage.git
cd pdf-to-webpage
CGO_ENABLED=1 go build -ldflags="-extldflags=-Wl,-no_warn_duplicate_libraries" -o pdf-to-webpage .
```

Or install directly:

```bash
CGO_ENABLED=1 go install -ldflags="-extldflags=-Wl,-no_warn_duplicate_libraries" github.com/anantshri/pdf-to-webpage@latest
```

> The `-ldflags` arg silences a harmless macOS linker warning about duplicate `-lm` from CGO deps. On Linux/other platforms you can drop it.

## Usage

```bash
pdf-to-webpage [flags] <slides.pdf>
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-o` | derived from filename | Output directory |
| `-dpi` | 300 | Render DPI for PDF rasterisation |
| `-width` | 1920 | Max image width in px (0 disables downscaling) |
| `-title` | derived from filename | Page title |
| `-header` | | HTML file injected above the slide viewer |
| `-footer` | | HTML file injected below the slide viewer |
| `-force` | false | Overwrite an existing output directory |
| `-allow-download` | true | Include the PDF in output and show the download button (set `=false` to omit both) |

### Example

```bash
pdf-to-webpage -dpi 300 -title "My Talk" presentation.pdf
```

This creates a `presentation/` directory containing the full slide viewer.

## How it works

1. **MuPDF** (via go-fitz) renders each PDF page to a raw image at the requested DPI
2. Images wider than `-width` are downscaled using CatmullRom resampling
3. Each page is encoded as WebP at quality 100 via libwebp
4. An embedded HTML/CSS/JS template generates the viewer page
5. The original PDF is copied alongside for download

## Output structure

```
output-dir/
├── index.html       # Slide viewer page
├── viewer.css       # Styles
├── viewer.js        # Navigation logic
├── slides.pdf       # Original PDF
└── images/
    ├── slide-001.webp
    ├── slide-002.webp
    └── ...
```

Serve with any static file server:

```bash
cd output-dir && python3 -m http.server
```

## Development

```bash
# Build
CGO_ENABLED=1 go build -ldflags="-extldflags=-Wl,-no_warn_duplicate_libraries" -o pdf-to-webpage .

# Static analysis
go vet ./...
```

### Dependencies

| Package | Purpose |
|---------|---------|
| [gen2brain/go-fitz](https://github.com/gen2brain/go-fitz) | PDF rendering via MuPDF (CGO) |
| [chai2010/webp](https://github.com/chai2010/webp) | WebP encoding via libwebp (CGO) |
| [golang.org/x/image](https://pkg.go.dev/golang.org/x/image) | CatmullRom image downscaling |

## Acknowledgements

Hat tip to [anantshri/hugo-techie-personal](https://github.com/anantshri/hugo-techie-personal) for the initial idea on how to approach this.

## 🤖 AI-Assisted Development

This project was developed with the assistance of AI tools, most notably Cursor IDE, Claude Code. These tools helped accelerate development and improve velocity. All AI-generated code has been carefully reviewed and validated through human inspection to ensure it aligns with the project's intended functionality and quality standards.

## License

This project is licensed under [GPL-3.0](LICENSE). The compiled binary statically links [MuPDF](https://mupdf.com/) (AGPL-3.0) via go-fitz.
