//go:build amd64 && !purego

package biquad

import (
	_ "github.com/cwbudde/algo-dsp/dsp/filter/biquad/internal/arch/amd64/avx2"
	_ "github.com/cwbudde/algo-dsp/dsp/filter/biquad/internal/arch/amd64/sse2"
	_ "github.com/cwbudde/algo-dsp/dsp/filter/biquad/internal/arch/generic"
	_ "github.com/cwbudde/algo-dsp/dsp/filter/biquad/internal/arch/registry"
)
