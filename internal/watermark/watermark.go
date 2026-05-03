// Package watermark embeds and extracts an invisible robust fingerprint
// in images via 8x8 block-DCT QIM on the luminance channel.
//
// The fingerprint string is hashed (SHA-256, first 16 bytes = 128 bits).
// A 16-bit sync header and 16-bit CRC are added; the resulting 160 bits
// are embedded by walking every 8x8 block in a deterministic shuffled
// order and writing bit (order_index % 160) into each block above a
// luma-variance threshold. Each bit ends up in many blocks; extraction
// majority-votes per bit position.
//
// This bit-assignment scheme is robust to "boundary" blocks whose
// variance flips above/below threshold across a lossy WebP roundtrip:
// a flipped block costs one vote for one bit, instead of misaligning
// all subsequent bit positions (as a fixed-cap selection would).
//
// Robustness range (measured on slide-like content):
//   - Lossy WebP at q >= 90: reliable round-trip.
//   - Screenshot saved as PNG (lossless) and recompressed JPEG at q >= 75:
//     reliable round-trip.
//   - Lossy WebP at q < 90 or aggressive denoising: not guaranteed.
//
// The pdf-to-webpage build pipeline encodes WebP at quality 100 by default,
// well above the reliability floor. Cropping, rotation, geometric
// transforms, and aggressive denoising are out of scope.
package watermark

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
)

const (
	// Algorithm tag stored in the build manifest.
	Algorithm = "dct8-qim-v2"

	// Delta is the QIM quantisation step on the luma DCT-II scale.
	Delta = 40.0

	syncHeader  uint16 = 0xACED
	payloadBits        = 128
	rawBits            = 16 + payloadBits + 16

	// minVotesPerBit is the minimum number of votes any bit position must
	// receive for extraction to be considered viable. Below this, the
	// image lacks enough textured blocks to reliably carry a watermark
	// (a bit position with 0 votes defaults to 0 at extract, breaking
	// the CRC for any non-zero embedded bit at that position).
	minVotesPerBit = 1
)

// HashFingerprint returns the hex-encoded SHA-256[:16] of s. Extract
// recovers the same hash from a watermarked image.
func HashFingerprint(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:16])
}

// Embed returns a copy of img with the fingerprint hash invisibly encoded
// in its luminance channel. The returned image is *image.RGBA.
func Embed(img image.Image, fingerprint string) (*image.RGBA, error) {
	if fingerprint == "" {
		return nil, errors.New("watermark: empty fingerprint")
	}
	bounds := img.Bounds()
	if bounds.Dx() < blockSize || bounds.Dy() < blockSize {
		return nil, errors.New("watermark: image too small")
	}
	sum := sha256.Sum256([]byte(fingerprint))
	bits := encodeRaw(sum[:16])

	rgba := toRGBA(img)
	_, minVotes := embedAll(rgba, bits)
	if minVotes < minVotesPerBit {
		return nil, fmt.Errorf("watermark: image lacks enough textured blocks (min votes per bit: %d, want >= %d)", minVotes, minVotesPerBit)
	}
	return rgba, nil
}

// Extract recovers the fingerprint hash from img. Returns ok=false if no
// watermark was detected (sync header missing or CRC mismatch).
func Extract(img image.Image) (string, bool, error) {
	bounds := img.Bounds()
	if bounds.Dx() < blockSize || bounds.Dy() < blockSize {
		return "", false, errors.New("watermark: image too small")
	}
	rgba := toRGBA(img)
	bits := readAll(rgba, rawBits)
	payload, ok := decodeRaw(bits)
	if !ok {
		return "", false, nil
	}
	return hex.EncodeToString(payload), true, nil
}
