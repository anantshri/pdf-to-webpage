# Invisible Image Fingerprinting

## Context

`pdf-to-webpage` rasterises a PDF into WebP slides for a self-contained viewer. There is currently no way to identify the source if someone redistributes those images. This change embeds an invisible, robust fingerprint into every rendered slide so a leaked image can be traced back to the build that produced it.

Decisions already made with the user:
- Default to a **per-build random UUID v4**, embedded identically on every slide. `-fingerprint=<string>` overrides; `-no-fingerprint` disables.
- **DCT-domain watermark** (must survive lossy WebP at the existing default quality 100, plus a screenshot+JPEG-recompression attack).
- Manifest written to output dir **and** UUID printed to stdout.
- Companion `-extract <image-path>` mode reads the fingerprint hash back out.

OCR-extracted *text* cannot carry the watermark — that would require modifying the source PDF and is explicitly out of scope. This protects against **image** redistribution.

## Approach

**Technique:** 8x8 block-based DCT on the luminance (Y) channel, encoding bits via Quantization Index Modulation (QIM) on a fixed pair of mid-frequency AC coefficients per block. Blind extraction (no original needed). 8x8 was chosen because both WebP (VP8 transform) and JPEG quantise around 8x8 blocks — mid-frequency coefficients are exactly what they preserve.

**Bit layout (160 bits raw, repeated 5x = 800 bits embedded):**
```
[ sync header   16 bits = 0xACED ]   ← lets Extract detect "no watermark"
[ payload      128 bits = SHA-256(fingerprint)[:16] ]
[ CRC-16        16 bits over payload ] ← rejects corrupt extractions
```
A 1920x1080 slide has ~32k luma blocks — plenty of room. Blocks are selected via PRNG seeded by a fixed package constant (NOT the fingerprint) so extraction is keyless.

**Why hash, not raw string:** bounded payload, no length field, arbitrary-length human-readable fingerprints supported. The manifest maps hash → original.

**Quantisation step Δ ≈ 14** (luma scale). Exposed as an unadvertised `-fingerprint-strength` flag for tuning if visual artifacts surface.

## File-level changes

### New package `internal/watermark/`

| File | Responsibility |
|---|---|
| `watermark.go` | Public API: `Embed(img image.Image, fingerprint string) (*image.RGBA, error)`, `Extract(img image.Image) (hashHex string, ok bool, err error)`, `HashFingerprint(s string) string`. Package-level docstring noting that cropping/rotation/heavy denoise are out of scope. |
| `dct.go` | Hand-rolled separable 8x8 DCT-II / IDCT-II (~80 LoC, float32, precomputed cosine table). No external dep. |
| `qim.go` | `quantizeBit(coef, delta, bit)` and `extractBit(coef, delta) int`. |
| `codec.go` | Sync header (0xACED), CRC-16, repetition encode/decode, majority vote. |
| `blocks.go` | RGB↔YCbCr (BT.601), deterministic block-index selector via `math/rand/v2` with fixed seed, skip blocks whose Y variance < threshold (avoids visible blocking on solid-colour areas). |
| `watermark_test.go` | Tests (see Verification). |
| `testdata/` | Small fixture image(s) for tests. |

Embed only on Y, leave Cb/Cr untouched. Return `*image.RGBA` so `writeWebP` interface is unchanged.

### `main.go`

Around the existing flag block (lines 32–55):
- Add `-fingerprint string`, `-no-fingerprint bool`, `-extract string` flags.
- After `flag.Parse()` (line 55), short-circuit `-extract` mode: open file via `image.Decode` (register WebP/PNG/JPEG decoders), call `watermark.Extract`, print result, return.
- Resolve effective fingerprint: empty if `-no-fingerprint`; user-supplied if `-fingerprint=...`; otherwise generate UUID v4 from `crypto/rand` (~30 LoC, no `google/uuid` dep — matches project's minimal-deps ethos).
- Pass the resolved string into `render.Options{ Fingerprint: ... }`.
- After `render.Pages` succeeds, write `fingerprint.json` to `outDir` and print `[pdf-to-webpage] fingerprint: <uuid> (hash: <hex>)`.

### `internal/render/render.go`

- Add `Fingerprint string` to `Options` (line 20-24).
- In the page loop, between `downscale` (line 69) and `writeWebP` (line 76):
  ```go
  if opt.Fingerprint != "" {
      out, err = watermark.Embed(out, opt.Fingerprint)
      if err != nil {
          return Result{}, fmt.Errorf("watermark page %d: %w", i+1, err)
      }
  }
  ```
- `out` becomes `*image.RGBA`; `writeWebP` already accepts `image.Image`.

### Manifest format (`<outDir>/fingerprint.json`, mode 0o644)

```json
{
  "version": 1,
  "fingerprint": "550e8400-e29b-41d4-a716-446655440000",
  "hash": "a3f5...e7b1",
  "algorithm": "dct8-qim-v1",
  "delta": 14,
  "created": "2026-04-29T12:34:56Z",
  "slide_count": 32,
  "source_pdf": "talk.pdf"
}
```
`algorithm` lets future extractors handle multiple variants.

## Reuse / dependencies

- `crypto/sha256`, `crypto/rand`, `encoding/json`, `image`, `image/color` — all stdlib.
- WebP decode in `-extract` mode: existing `github.com/chai2010/webp` already imported.
- **No new dependencies.** DCT, UUID, CRC-16 all hand-rolled (each <60 LoC).

## Verification

Tests live in `internal/watermark/watermark_test.go`:

1. **Round-trip in-memory:** Embed into synthetic gradient → Extract → assert hash matches.
2. **Through WebP (the production path):** Embed → `webp.Encode` at q ∈ {100, 75, 50} → decode → Extract. Assert match at q ≥ 60.
3. **Screenshot+JPEG attack:** Embed → WebP encode → decode → JPEG encode at q=75 → decode → Extract. Assert match.
4. **Negative test:** Extract on an unmarked render returns `ok=false` (sync-header miss).
5. **Visual regression:** PSNR ≥ 40 dB before/after embed on a real fixture.
6. **Determinism:** Same fingerprint + same image → identical bytes.

End-to-end manual check:
```sh
CGO_ENABLED=1 go build -ldflags="-extldflags=-Wl,-no_warn_duplicate_libraries" -o pdf-to-webpage .
./pdf-to-webpage -fingerprint "test-recipient-alice" sample.pdf
./pdf-to-webpage -extract sample/images/slide-001.webp
# → prints hash matching fingerprint.json
```

`go vet ./...` must pass.

## Risks & open items

- **Solid-colour title slides:** near-zero AC coefficients ⇒ QIM may produce visible blocking. Mitigated by skipping low-variance blocks; the 5x repetition still yields a quorum from the body of the slide. Test with a deliberately empty white slide.
- **WebP at default q=100** is mildly lossy but very forgiving — robustness pressure is lower than the worst case. If the user ever lowers default quality, Δ may need bumping; the algorithm version tag in the manifest covers future migrations.
- **`fingerprint.json` is sensitive** (identifies recipients) — append `fingerprint.json` to the repo `.gitignore` so generated output dirs containing it are never accidentally committed.
- **Adversarial removal** (cropping, rotation, heavy denoise, AI upscaling) is out of scope and will be documented in the package comment.

## Critical files

- `/Users/ion1/WORK/research/slide-maker/pdf-to-webpage/main.go` (modify)
- `/Users/ion1/WORK/research/slide-maker/pdf-to-webpage/internal/render/render.go` (modify)
- `/Users/ion1/WORK/research/slide-maker/pdf-to-webpage/internal/watermark/*.go` (new package, 6 files)
- `/Users/ion1/WORK/research/slide-maker/pdf-to-webpage/.gitignore` (append `fingerprint.json`)
- `/Users/ion1/WORK/research/slide-maker/pdf-to-webpage/AGENTS.md` and `CLAUDE.md` (brief mention of new flags + manifest, optional)
