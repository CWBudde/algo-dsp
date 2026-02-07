//go:build arm64 && !purego

package neon

import (
	"github.com/cwbudde/algo-dsp/dsp/filter/biquad/internal/arch/registry"
	"github.com/cwbudde/algo-dsp/internal/cpu"
)

func init() {
	registry.Global.Register(registry.OpEntry{
		Name:         "neon",
		SIMDLevel:    cpu.SIMDNEON,
		Priority:     15,
		ProcessBlock: processBlock,
	})
}

func processBlock(c registry.Coefficients, d0, d1 float64, buf []float64) (newD0, newD1 float64) {
	if len(buf) == 0 {
		return d0, d1
	}
	return processBlockNEON(buf, c.B0, c.B1, c.B2, c.A1, c.A2, d0, d1)
}
