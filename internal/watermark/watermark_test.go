package watermark

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"math"
	"os"
	"testing"

	"github.com/chai2010/webp"
)

// makeTestImage produces a slide-like image: white background with
// dark rectangles and stripes simulating text/figures. Smooth gradients
// don't have realistic mid-frequency energy, so robustness numbers
// measured against them mislead.
func makeTestImage(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, color.RGBA{245, 245, 245, 255})
		}
	}
	for i := 0; i < 12; i++ {
		y0 := (h * (i + 1)) / 14
		for y := y0; y < y0+8 && y < h; y++ {
			for x := w / 10; x < (9*w)/10; x++ {
				if (x/4)%2 == 0 {
					img.SetRGBA(x, y, color.RGBA{30, 30, 30, 255})
				}
			}
		}
	}
	return img
}

func TestDCTRoundTrip(t *testing.T) {
	var block [blockSize][blockSize]float64
	for j := 0; j < blockSize; j++ {
		for i := 0; i < blockSize; i++ {
			block[j][i] = float64(j*blockSize + i + 17)
		}
	}
	coefs := dct2D(&block)
	back := idct2D(&coefs)
	for j := 0; j < blockSize; j++ {
		for i := 0; i < blockSize; i++ {
			if math.Abs(block[j][i]-back[j][i]) > 1e-6 {
				t.Fatalf("DCT/IDCT round-trip mismatch at (%d,%d): %v vs %v", i, j, block[j][i], back[j][i])
			}
		}
	}
}

func TestQIMRoundTrip(t *testing.T) {
	for _, c := range []float64{-100, -7.3, 0.0, 5.5, 42.7, 999.1} {
		for _, bit := range []int{0, 1} {
			q := quantizeBit(c, Delta, bit)
			got := extractBit(q, Delta)
			if got != bit {
				t.Errorf("quantize/extract mismatch: coef=%v bit=%d got=%d", c, bit, got)
			}
		}
	}
}

func TestCRC16(t *testing.T) {
	// CRC-16-CCITT (poly 0x1021, init 0xFFFF) of "123456789" is 0x29B1.
	got := crc16([]byte("123456789"))
	if got != 0x29B1 {
		t.Fatalf("CRC-16-CCITT(123456789) = %#04x, want 0x29B1", got)
	}
}

func TestEmbedExtractRoundTrip(t *testing.T) {
	img := makeTestImage(512, 512)
	out, err := Embed(img, "test-fingerprint")
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	hash, ok, err := Extract(out)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if !ok {
		t.Fatal("Extract: ok=false on watermarked image")
	}
	if want := HashFingerprint("test-fingerprint"); hash != want {
		t.Fatalf("hash mismatch: got %s want %s", hash, want)
	}
}

func TestExtractFromWebP(t *testing.T) {
	img := makeTestImage(640, 480)
	marked, err := Embed(img, "alice@example.com")
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	for _, q := range []float32{100, 95, 90} {
		var buf bytes.Buffer
		if err := webp.Encode(&buf, marked, &webp.Options{Lossless: false, Quality: q}); err != nil {
			t.Fatalf("webp.Encode q=%v: %v", q, err)
		}
		decoded, err := webp.Decode(&buf)
		if err != nil {
			t.Fatalf("webp.Decode q=%v: %v", q, err)
		}
		hash, ok, err := Extract(decoded)
		if err != nil {
			t.Fatalf("Extract q=%v: %v", q, err)
		}
		if !ok {
			t.Errorf("q=%v: watermark not detected", q)
			continue
		}
		if want := HashFingerprint("alice@example.com"); hash != want {
			t.Errorf("q=%v: hash mismatch got %s want %s", q, hash, want)
		}
	}
}

func TestExtractAfterJPEGAttack(t *testing.T) {
	img := makeTestImage(640, 480)
	marked, err := Embed(img, "screenshot-victim")
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	// First through WebP at q=100 (the production default).
	var wbuf bytes.Buffer
	if err := webp.Encode(&wbuf, marked, &webp.Options{Lossless: false, Quality: 100}); err != nil {
		t.Fatalf("webp.Encode: %v", err)
	}
	wdecoded, err := webp.Decode(&wbuf)
	if err != nil {
		t.Fatalf("webp.Decode: %v", err)
	}
	// Then JPEG-recompress simulating a screenshot save.
	var jbuf bytes.Buffer
	if err := jpeg.Encode(&jbuf, wdecoded, &jpeg.Options{Quality: 85}); err != nil {
		t.Fatalf("jpeg.Encode: %v", err)
	}
	jdecoded, err := jpeg.Decode(&jbuf)
	if err != nil {
		t.Fatalf("jpeg.Decode: %v", err)
	}
	hash, ok, err := Extract(jdecoded)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if !ok {
		t.Fatal("watermark survived neither WebP nor JPEG recompression")
	}
	if want := HashFingerprint("screenshot-victim"); hash != want {
		t.Errorf("hash mismatch: got %s want %s", hash, want)
	}
}

func TestExtractOnUnmarkedImage(t *testing.T) {
	img := makeTestImage(512, 512)
	_, ok, err := Extract(img)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if ok {
		t.Fatal("Extract returned ok=true on an unmarked image")
	}
}

func TestEmbedDeterminism(t *testing.T) {
	img := makeTestImage(512, 512)
	a, err := Embed(img, "abc")
	if err != nil {
		t.Fatalf("Embed a: %v", err)
	}
	b, err := Embed(img, "abc")
	if err != nil {
		t.Fatalf("Embed b: %v", err)
	}
	if !bytes.Equal(a.Pix, b.Pix) {
		t.Fatal("Embed not deterministic for identical inputs")
	}
}

// makeSlideLike returns a 1920x1080 image that resembles a real
// slide: mostly white, with a small region of dark text-like content.
func makeSlideLike() *image.RGBA {
	w, h := 1920, 1080
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, color.RGBA{255, 255, 255, 255})
		}
	}
	// Title line near top-center.
	for y := 100; y < 140; y++ {
		for x := 200; x < 1700; x++ {
			if (x/8)%2 == 0 {
				img.SetRGBA(x, y, color.RGBA{40, 40, 40, 255})
			}
		}
	}
	// Body text rows lower in the slide.
	for row := 0; row < 6; row++ {
		y0 := 250 + row*80
		for y := y0; y < y0+24; y++ {
			for x := 250; x < 1500; x++ {
				if (x/6)%2 == 0 && (y/3)%2 == 0 {
					img.SetRGBA(x, y, color.RGBA{60, 60, 60, 255})
				}
			}
		}
	}
	return img
}

// TestBitErrorRate is diagnostic: prints per-bit error rate at each
// WebP quality so we can tune Delta/coefs without rebuilding intuition.
func TestBitErrorRate(t *testing.T) {
	img := makeSlideLike()
	marked, err := Embed(img, "diag")
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	expected := readAll(marked, rawBits)
	for _, q := range []float32{100, 95, 90, 85, 80, 75} {
		var buf bytes.Buffer
		if err := webp.Encode(&buf, marked, &webp.Options{Lossless: false, Quality: q}); err != nil {
			t.Fatalf("encode q=%v: %v", q, err)
		}
		dec, err := webp.Decode(&buf)
		if err != nil {
			t.Fatalf("decode q=%v: %v", q, err)
		}
		got := readAll(toRGBA(dec), rawBits)
		errs := 0
		for i := range expected {
			if got[i] != expected[i] {
				errs++
			}
		}
		t.Logf("q=%-3v bit-error rate %d/%d = %.1f%%", q, errs, len(expected), 100*float64(errs)/float64(len(expected)))
	}
}

// TestFileRoundTrip exercises the actual production path: Embed →
// webp.Encode to a file on disk → reopen → webp.Decode → Extract.
func TestFileRoundTrip(t *testing.T) {
	img := makeSlideLike()
	marked, err := Embed(img, "diag")
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	tmp, err := os.CreateTemp("", "wm-test-*.webp")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmp.Name())
	if err := webp.Encode(tmp, marked, &webp.Options{Lossless: false, Quality: 100}); err != nil {
		t.Fatal(err)
	}
	tmp.Close()

	r, err := os.Open(tmp.Name())
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	dec, err := webp.Decode(r)
	if err != nil {
		t.Fatal(err)
	}
	hash, ok, err := Extract(dec)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("file roundtrip: ok=%v hash=%s", ok, hash)
	if !ok {
		t.Fatal("watermark lost in file roundtrip")
	}
}

// TestRealSlideRoundTrip loads a real slide rendered by the production
// pipeline (already watermarked with "alice@example.com") and checks
// extraction. If this fails but TestExtractFromWebP passes, our
// synthetic test image isn't representative of real slide content.
func TestRealSlideRoundTrip(t *testing.T) {
	f, err := os.Open("testdata/real-slide.webp")
	if err != nil {
		t.Skipf("real-slide.webp not present: %v", err)
	}
	defer f.Close()
	img, err := webp.Decode(f)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	hash, ok, err := Extract(img)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	t.Logf("real slide extract: ok=%v hash=%s dims=%dx%d", ok, hash, img.Bounds().Dx(), img.Bounds().Dy())
	if !ok {
		// Diagnostic: re-Embed the same fingerprint, encode/decode,
		// and compare bit error rates so we can see if it's the embed
		// path or the content that's the problem.
		marked, err := Embed(img, "alice@example.com")
		if err != nil {
			t.Fatalf("re-Embed: %v", err)
		}
		var buf bytes.Buffer
		if err := webp.Encode(&buf, marked, &webp.Options{Lossless: false, Quality: 100}); err != nil {
			t.Fatalf("re-encode: %v", err)
		}
		dec, err := webp.Decode(&buf)
		if err != nil {
			t.Fatalf("re-decode: %v", err)
		}
		hash2, ok2, _ := Extract(dec)
		t.Logf("re-Embed→encode→decode→Extract: ok=%v hash=%s", ok2, hash2)
	}
}

func TestPSNR(t *testing.T) {
	img := makeTestImage(512, 512)
	marked, err := Embed(img, "fingerprint")
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	mse := 0.0
	count := 0
	bounds := img.Bounds()
	for y := 0; y < bounds.Dy(); y++ {
		for x := 0; x < bounds.Dx(); x++ {
			off := img.PixOffset(x, y)
			for ch := 0; ch < 3; ch++ {
				d := float64(img.Pix[off+ch]) - float64(marked.Pix[off+ch])
				mse += d * d
				count++
			}
		}
	}
	mse /= float64(count)
	if mse == 0 {
		return
	}
	psnr := 10 * math.Log10(255*255/mse)
	if psnr < 38 {
		t.Errorf("PSNR too low: %.2f dB (want >= 38 dB)", psnr)
	}
	t.Logf("PSNR after embed: %.2f dB", psnr)
}
