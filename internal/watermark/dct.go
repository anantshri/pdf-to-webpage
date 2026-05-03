package watermark

import "math"

const blockSize = 8

var (
	dctCos   [blockSize][blockSize]float64 // dctCos[n][k] = cos(π(2n+1)k / 16)
	dctAlpha [blockSize]float64
)

func init() {
	for n := 0; n < blockSize; n++ {
		for k := 0; k < blockSize; k++ {
			dctCos[n][k] = math.Cos(math.Pi * float64(2*n+1) * float64(k) / float64(2*blockSize))
		}
	}
	dctAlpha[0] = math.Sqrt(1.0 / float64(blockSize))
	for k := 1; k < blockSize; k++ {
		dctAlpha[k] = math.Sqrt(2.0 / float64(blockSize))
	}
}

// dct2D performs separable 2D DCT-II on an 8x8 block.
// Output[u][v] is the (u-th vertical, v-th horizontal) frequency.
func dct2D(block *[blockSize][blockSize]float64) [blockSize][blockSize]float64 {
	var rows [blockSize][blockSize]float64
	for r := 0; r < blockSize; r++ {
		for k := 0; k < blockSize; k++ {
			sum := 0.0
			for c := 0; c < blockSize; c++ {
				sum += block[r][c] * dctCos[c][k]
			}
			rows[r][k] = dctAlpha[k] * sum
		}
	}
	var out [blockSize][blockSize]float64
	for v := 0; v < blockSize; v++ {
		for u := 0; u < blockSize; u++ {
			sum := 0.0
			for r := 0; r < blockSize; r++ {
				sum += rows[r][v] * dctCos[r][u]
			}
			out[u][v] = dctAlpha[u] * sum
		}
	}
	return out
}

// idct2D performs separable 2D IDCT-II on an 8x8 frequency block.
func idct2D(block *[blockSize][blockSize]float64) [blockSize][blockSize]float64 {
	var rows [blockSize][blockSize]float64
	for v := 0; v < blockSize; v++ {
		for r := 0; r < blockSize; r++ {
			sum := 0.0
			for u := 0; u < blockSize; u++ {
				sum += dctAlpha[u] * block[u][v] * dctCos[r][u]
			}
			rows[r][v] = sum
		}
	}
	var out [blockSize][blockSize]float64
	for r := 0; r < blockSize; r++ {
		for c := 0; c < blockSize; c++ {
			sum := 0.0
			for k := 0; k < blockSize; k++ {
				sum += dctAlpha[k] * rows[r][k] * dctCos[c][k]
			}
			out[r][c] = sum
		}
	}
	return out
}
