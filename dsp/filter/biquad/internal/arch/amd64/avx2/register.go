//go:build amd64 && !purego

package avx2

import (
	"github.com/cwbudde/algo-dsp/dsp/filter/biquad/internal/arch/registry"
	"github.com/cwbudde/algo-vecmath/cpu"
)

func init() {
	registry.Global.Register(registry.OpEntry{
		Name:         "avx2",
		SIMDLevel:    cpu.SIMDAVX2,
		Priority:     20,
		ProcessBlock: processBlock,
	})
}

// processBlock is a 4x-unrolled scalar kernel selected for AVX2-capable CPUs.
// TODO: replace with explicit AVX2 asm kernel.
func processBlock(c registry.Coefficients, d0, d1 float64, buf []float64) (newD0, newD1 float64) {
	b0, b1, b2 := c.B0, c.B1, c.B2
	a1, a2 := c.A1, c.A2

	i := 0
	n := len(buf)
	for ; i+3 < n; i += 4 {
		x0 := buf[i]
		y0 := b0*x0 + d0
		d0n0 := b1*x0 - a1*y0 + d1
		d1n0 := b2*x0 - a2*y0

		x1 := buf[i+1]
		y1 := b0*x1 + d0n0
		d0n1 := b1*x1 - a1*y1 + d1n0
		d1n1 := b2*x1 - a2*y1

		x2 := buf[i+2]
		y2 := b0*x2 + d0n1
		d0n2 := b1*x2 - a1*y2 + d1n1
		d1n2 := b2*x2 - a2*y2

		x3 := buf[i+3]
		y3 := b0*x3 + d0n2
		d0 = b1*x3 - a1*y3 + d1n2
		d1 = b2*x3 - a2*y3

		buf[i] = y0
		buf[i+1] = y1
		buf[i+2] = y2
		buf[i+3] = y3
	}

	for ; i < n; i++ {
		x := buf[i]
		y := b0*x + d0
		d0 = b1*x - a1*y + d1
		d1 = b2*x - a2*y
		buf[i] = y
	}

	return d0, d1
}
