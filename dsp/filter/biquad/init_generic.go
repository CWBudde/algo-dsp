//go:build !amd64 && !arm64

package biquad

import (
	_ "github.com/cwbudde/algo-dsp/dsp/filter/biquad/internal/arch/generic"
	_ "github.com/cwbudde/algo-dsp/dsp/filter/biquad/internal/arch/registry"
)
