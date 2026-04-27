# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.1] — 2026-04-27

CI / release pipeline hardening. No runtime or behaviour changes to the CLI or the rendered viewer.

### Changed

- **Release workflow now uploads to a draft release.** The `softprops/action-gh-release` step is configured with `draft: true`, making it compatible with GitHub's immutable releases setting — the maintainer creates a draft release manually, the workflow attaches the built `.tar.gz` binaries to that draft on tag push, and the maintainer publishes when ready.
- **macOS release runners pinned to current images.** `macos-13` → `macos-26-intel` for `darwin/amd64`; `macos-latest` → `macos-26` for `darwin/arm64` (reproducibility — no longer floats with the `macos-latest` alias).

### Security

- **Least-privilege `GITHUB_TOKEN` on both workflows.** Workflow-level default is now `contents: read`; `release.yml`'s `release` job is the only place granted `contents: write`. The build matrix can no longer write to the repo.

## [0.1.0] — 2026-04-27

First public release. `pdf-to-webpage` converts a PDF presentation into a self-contained, full-width slide viewer webpage as a single Go binary with no runtime dependencies.

### Added

#### Core conversion pipeline
- PDF rasterisation via [gen2brain/go-fitz](https://github.com/gen2brain/go-fitz) (MuPDF, CGO)
- Per-page WebP encoding at quality 100 via [chai2010/webp](https://github.com/chai2010/webp) (libwebp, CGO)
- CatmullRom downscaling (`golang.org/x/image`) for crisp text when shrinking high-DPI renders to the target width
- Static HTML/CSS/JS slide viewer generated from an embedded template

#### CLI flags
- `-o` — output folder (default: derived from PDF basename via slugify)
- `-dpi` — render DPI for PDF rasterisation (default `300`)
- `-width` — max image width in px (default `1920`, `0` disables downscaling)
- `-title` — page title (default: derived from PDF basename via humanise)
- `-header` — HTML file injected above the slide viewer
- `-footer` — HTML file injected below the slide viewer
- `-force` — wipe and overwrite an existing output folder
- `-allow-download` — include the PDF and show the download button (default `true`; set `=false` to omit both)

#### Viewer features
- Dark-themed responsive viewer
- Keyboard navigation: arrow keys, `Home`/`End`
- Grid overview mode (`G` key) with thumbnail navigation
- Fullscreen mode (`F` key)
- Touch / swipe navigation for mobile
- URL hash navigation (`#slide-3`) for direct linking
- Print-friendly CSS
- Optional download button for the source PDF
- Self-contained output — works offline, serves from any static host

#### Output structure
```
output-dir/
├── index.html
├── viewer.css
├── viewer.js
├── slides.pdf      # only when -allow-download=true
└── images/
    ├── slide-001.webp
    └── ...
```

#### Project layout
- `main.go` — flag parsing, orchestration, slugify/humanise helpers
- `internal/render` — PDF → WebP rasterisation
- `internal/site` — HTML generation and asset emission
- `internal/assets` — embedded `index.html.tmpl`, `viewer.css`, `viewer.js`

#### Build & CI
- GitHub Actions CI workflow — vet, build, and test on `ubuntu-latest` and `macos-latest`
- GitHub Actions release workflow — multi-platform binaries on tag push:
  - `linux/amd64`, `linux/arm64` (cross-compiled with `gcc-aarch64-linux-gnu`)
  - `darwin/amd64`, `darwin/arm64`
- Linux release binaries compressed with UPX
- Release artefacts packaged as `.tar.gz` with `README.md` and `LICENSE` and uploaded via `softprops/action-gh-release`
- All third-party GitHub Actions pinned to commit SHAs (with version comments) for supply-chain hardening:
  - `actions/checkout@v6.0.2`
  - `actions/setup-go@v6.4.0`
  - `actions/upload-artifact@v7.0.1`
  - `actions/download-artifact@v8.0.1`
  - `softprops/action-gh-release@v3.0.0`

#### Documentation
- `README.md` — features, install, usage, flags, output structure, dev, dependencies, licence
- `AGENTS.md` — project context for AI coding agents
- `CLAUDE.md` — Claude Code quick-reference (build commands, architecture, known quirks)
- `LICENSE` — GPL-3.0
- `Acknowledgements` section crediting [anantshri/hugo-techie-personal](https://github.com/anantshri/hugo-techie-personal) for the initial idea
- `AI-Assisted Development` section disclosing Cursor IDE + Claude Code use

### Technical notes

- **CGO is mandatory.** `go-fitz` (MuPDF) and `chai2010/webp` (libwebp) both require it; `CGO_ENABLED=0` builds will not work.
- **MuPDF signal-handler workaround.** `os.Setenv("GODEBUG", "asyncpreemptoff=1")` is set in `main()` to disable Go's async goroutine preemption, which conflicts with MuPDF's C signal handlers and can cause crashes. Removing this line will reintroduce intermittent crashes during PDF rendering.
- **macOS linker warning.** Both CGO deps list `-lm` in their `LDFLAGS`, producing a harmless `ld: warning: ignoring duplicate libraries: '-lm'` on Apple toolchains. Documented build/install commands include `-ldflags="-extldflags=-Wl,-no_warn_duplicate_libraries"` to silence it. The release workflow applies the flag automatically on darwin matrix entries.

### Dependencies

| Package | Version |
|---|---|
| `github.com/chai2010/webp` | v1.4.0 |
| `github.com/gen2brain/go-fitz` | v1.24.15 |
| `golang.org/x/image` | v0.38.0 |
| `github.com/ebitengine/purego` (indirect) | v0.10.0 |
| `github.com/jupiterrider/ffi` (indirect) | v0.6.0 |
| `golang.org/x/sys` (indirect) | v0.43.0 |

### Toolchain

- Go 1.25
- Module path: `github.com/anantshri/pdf-to-webpage`

[0.1.1]: https://github.com/anantshri/pdf-to-webpage/releases/tag/v0.1.1
[0.1.0]: https://github.com/anantshri/pdf-to-webpage/releases/tag/v0.1.0
