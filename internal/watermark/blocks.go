package watermark

import (
	"image"
	"image/color"
	"math"
	"math/rand/v2"
)

// bitCoef is the low/mid-frequency DCT position carrying each bit. Both
// Embed and Extract must use the same coordinate; do not change without
// bumping Algorithm in watermark.go.
const (
	bitCoefU = 2
	bitCoefV = 2
)

// minBlockVariance is the luma-variance threshold (uint8 scale²) below
// which a block is considered too uniform to carry a watermark bit
// reliably under lossy compression. Embed and Extract both classify
// blocks with this threshold so they pick the same subset.
//
// A small fraction of blocks straddle the threshold and may flip
// classification across a WebP roundtrip. The bit-assignment scheme
// (order_index % rawBits) is robust to that drift: a flipped block just
// loses one vote for one bit position, instead of misaligning all
// subsequent positions.
const minBlockVariance = 100.0

// blockSeed is the fixed seed for the deterministic block-order PRNG.
// Embed and Extract must agree on this seed; do not change once any
// watermarked images exist.
const blockSeed uint64 = 0x70646643696d676b

type blockCoord struct{ x, y int }

// blockOrder returns a deterministic shuffled list of all 8x8 block
// origins inside (0,0)–(width,height). The shuffle is seeded by image
// dimensions so any pair of (Embed, Extract) calls on the same-sized
// image produces the same block order.
func blockOrder(width, height int) []blockCoord {
	cols := width / blockSize
	rows := height / blockSize
	coords := make([]blockCoord, 0, cols*rows)
	for r := 0; r < rows; r++ {
		for c := 0; c < cols; c++ {
			coords = append(coords, blockCoord{c * blockSize, r * blockSize})
		}
	}
	rng := rand.New(rand.NewPCG(blockSeed, uint64(width)*1000003+uint64(height)))
	for i := len(coords) - 1; i > 0; i-- {
		j := int(rng.Uint64N(uint64(i + 1)))
		coords[i], coords[j] = coords[j], coords[i]
	}
	return coords
}

// embedAll walks every block in deterministic order and writes
// rawBitsArr[i % rawBits] into block at order index i, but only into
// blocks above the variance threshold. Returns the number of blocks
// written, and the minimum number of votes any single bit received
// (for diagnostics / capacity check).
func embedAll(rgba *image.RGBA, rawBitsArr []int) (written, minVotes int) {
	all := blockOrder(rgba.Bounds().Dx(), rgba.Bounds().Dy())
	votesPerBit := make([]int, len(rawBitsArr))
	for i, c := range all {
		if blockYVariance(rgba, c.x, c.y) < minBlockVariance {
			continue
		}
		bit := rawBitsArr[i%len(rawBitsArr)]
		var y, cb, cr [blockSize][blockSize]float64
		readBlockYCbCr(rgba, c.x, c.y, &y, &cb, &cr)
		coefs := dct2D(&y)
		coefs[bitCoefU][bitCoefV] = quantizeBit(coefs[bitCoefU][bitCoefV], Delta, bit)
		spatial := idct2D(&coefs)
		writeBlockYCbCr(rgba, c.x, c.y, &spatial, &cb, &cr)
		written++
		votesPerBit[i%len(rawBitsArr)]++
	}
	minVotes = -1
	for _, v := range votesPerBit {
		if minVotes == -1 || v < minVotes {
			minVotes = v
		}
	}
	return written, minVotes
}

// readAll walks every block in deterministic order, reads the QIM bit
// from blocks above the variance threshold, and majority-votes per bit
// position. Returns the rawBitsArr-length decoded array.
func readAll(rgba *image.RGBA, nBits int) []int {
	all := blockOrder(rgba.Bounds().Dx(), rgba.Bounds().Dy())
	ones := make([]int, nBits)
	totals := make([]int, nBits)
	for i, c := range all {
		if blockYVariance(rgba, c.x, c.y) < minBlockVariance {
			continue
		}
		var y [blockSize][blockSize]float64
		readBlockY(rgba, c.x, c.y, &y)
		coefs := dct2D(&y)
		bit := extractBit(coefs[bitCoefU][bitCoefV], Delta)
		idx := i % nBits
		totals[idx]++
		if bit == 1 {
			ones[idx]++
		}
	}
	out := make([]int, nBits)
	for i := range out {
		if totals[i] > 0 && ones[i]*2 > totals[i] {
			out[i] = 1
		}
	}
	return out
}

func blockYVariance(rgba *image.RGBA, x0, y0 int) float64 {
	var sum, sumSq float64
	for j := 0; j < blockSize; j++ {
		for i := 0; i < blockSize; i++ {
			off := rgba.PixOffset(x0+i, y0+j)
			y, _, _ := color.RGBToYCbCr(rgba.Pix[off+0], rgba.Pix[off+1], rgba.Pix[off+2])
			v := float64(y)
			sum += v
			sumSq += v * v
		}
	}
	n := float64(blockSize * blockSize)
	mean := sum / n
	return sumSq/n - mean*mean
}

// toRGBA returns img as a freshly allocated *image.RGBA.
func toRGBA(img image.Image) *image.RGBA {
	b := img.Bounds()
	out := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			r, g, bl, a := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
			off := out.PixOffset(x, y)
			out.Pix[off+0] = uint8(r >> 8)
			out.Pix[off+1] = uint8(g >> 8)
			out.Pix[off+2] = uint8(bl >> 8)
			out.Pix[off+3] = uint8(a >> 8)
		}
	}
	return out
}

func readBlockYCbCr(rgba *image.RGBA, x0, y0 int, y, cb, cr *[blockSize][blockSize]float64) {
	for j := 0; j < blockSize; j++ {
		for i := 0; i < blockSize; i++ {
			off := rgba.PixOffset(x0+i, y0+j)
			yy, cbcb, crcr := color.RGBToYCbCr(rgba.Pix[off+0], rgba.Pix[off+1], rgba.Pix[off+2])
			y[j][i] = float64(yy)
			cb[j][i] = float64(cbcb)
			cr[j][i] = float64(crcr)
		}
	}
}

func readBlockY(rgba *image.RGBA, x0, y0 int, y *[blockSize][blockSize]float64) {
	for j := 0; j < blockSize; j++ {
		for i := 0; i < blockSize; i++ {
			off := rgba.PixOffset(x0+i, y0+j)
			yy, _, _ := color.RGBToYCbCr(rgba.Pix[off+0], rgba.Pix[off+1], rgba.Pix[off+2])
			y[j][i] = float64(yy)
		}
	}
}

func writeBlockYCbCr(rgba *image.RGBA, x0, y0 int, y, cb, cr *[blockSize][blockSize]float64) {
	for j := 0; j < blockSize; j++ {
		for i := 0; i < blockSize; i++ {
			r, g, bl := color.YCbCrToRGB(clampU8(y[j][i]), clampU8(cb[j][i]), clampU8(cr[j][i]))
			off := rgba.PixOffset(x0+i, y0+j)
			rgba.Pix[off+0] = r
			rgba.Pix[off+1] = g
			rgba.Pix[off+2] = bl
		}
	}
}

func clampU8(v float64) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(math.Round(v))
}
