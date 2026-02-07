package generic

import (
	"github.com/cwbudde/algo-dsp/dsp/filter/biquad/internal/arch/registry"
	"github.com/cwbudde/algo-vecmath/cpu"
)

func init() {
	registry.Global.Register(registry.OpEntry{
		Name:         "generic",
		SIMDLevel:    cpu.SIMDNone,
		Priority:     0,
		ProcessBlock: processBlock,
	})
}

func processBlock(c registry.Coefficients, d0, d1 float64, buf []float64) (newD0, newD1 float64) {
	b0, b1, b2 := c.B0, c.B1, c.B2
	a1, a2 := c.A1, c.A2

	i := 0
	n := len(buf)
	for ; i+1 < n; i += 2 {
		x0 := buf[i]
		y0 := b0*x0 + d0
		d0n := b1*x0 - a1*y0 + d1
		d1n := b2*x0 - a2*y0

		x1 := buf[i+1]
		y1 := b0*x1 + d0n
		d0 = b1*x1 - a1*y1 + d1n
		d1 = b2*x1 - a2*y1

		buf[i] = y0
		buf[i+1] = y1
	}

	if i < n {
		x := buf[i]
		y := b0*x + d0
		d0 = b1*x - a1*y + d1
		d1 = b2*x - a2*y
		buf[i] = y
	}

	return d0, d1
}
