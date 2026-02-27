//go:build amd64 && !purego

package biquad

import (
	_ "github.com/cwbudde/algo-dsp/dsp/filter/biquad/internal/arch/amd64/avx2" // register AVX2 backend
	_ "github.com/cwbudde/algo-dsp/dsp/filter/biquad/internal/arch/amd64/sse2" // register SSE2 backend
	_ "github.com/cwbudde/algo-dsp/dsp/filter/biquad/internal/arch/generic"    // register generic backend
	_ "github.com/cwbudde/algo-dsp/dsp/filter/biquad/internal/arch/registry"   // initialize backend registry
)
