# CLAUDE.md

For full project context, see [AGENTS.md](AGENTS.md).

## Quick reference

- **Build:** `CGO_ENABLED=1 go build -ldflags="-extldflags=-Wl,-no_warn_duplicate_libraries" -o pdf-to-webpage .` (the `-ldflags` arg silences a harmless macOS linker warning about duplicate `-lm`; safe to drop on Linux)
- **Vet:** `go vet ./...`
- **CGO is mandatory** — go-fitz (MuPDF) and chai2010/webp (libwebp) both require it
- Go 1.24, module name `github.com/anantshri/pdf-to-webpage`

## Architecture

`main.go` → parses flags → `render.Pages()` (PDF → WebP via MuPDF) → `site.Write()` (HTML from embedded template) → `site.CopyPDF()`

Three internal packages: `assets` (embedded FS), `render` (rasterisation), `site` (HTML output).

## Important

- The `GODEBUG=asyncpreemptoff=1` line in `main()` is a required workaround for a CGO signal handler conflict — do not remove it
- No tests exist yet; `go test ./...` will be a no-op
