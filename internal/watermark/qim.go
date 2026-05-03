package watermark

import "math"

// quantizeBit forces round(coef/delta) to have the requested parity
// (0 = even, 1 = odd) and returns the new coefficient.
func quantizeBit(coef, delta float64, bit int) float64 {
	q := int(math.Round(coef / delta))
	if (q & 1) != bit {
		residual := coef - float64(q)*delta
		if residual >= 0 {
			q++
		} else {
			q--
		}
	}
	return float64(q) * delta
}

// extractBit returns the parity of round(coef/delta), i.e. the embedded bit.
func extractBit(coef, delta float64) int {
	return int(math.Round(coef/delta)) & 1
}
