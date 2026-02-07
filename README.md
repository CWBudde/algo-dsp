# algo-dsp

Reusable DSP (digital signal processing) algorithms for Go.

## Status

- Current focus: Phase 14 API stabilization (`PLAN.md`).
- Module path: `github.com/cwbudde/algo-dsp`

## Quick Start

```bash
go get github.com/cwbudde/algo-dsp@latest
```

```go
package main

import (
	"fmt"

	"github.com/cwbudde/algo-dsp/dsp/window"
)

func main() {
	w, err := window.Generate(window.Hann, 8)
	if err != nil {
		panic(err)
	}
	fmt.Println(w)
}
```

## Package Overview

- `dsp/window`: analysis windows and metadata
- `dsp/filter/biquad`: IIR biquad sections and chains
- `dsp/filter/fir`: FIR runtime filtering
- `dsp/filter/design`: filter design utilities
- `dsp/filter/bank`: octave and fractional-octave bank helpers
- `dsp/filter/weighting`: A/B/C/Z weighting filters
- `dsp/conv`: convolution, overlap-add/save, deconvolution
- `dsp/resample`: sample-rate conversion helpers
- `dsp/spectrum`: magnitude/phase/group delay/smoothing utilities
- `dsp/signal`: deterministic signal generation and shaping helpers
- `measure/thd`: THD and THD+N estimators
- `measure/sweep`: linear/log sweep generation and deconvolution
- `measure/ir`: impulse response metrics (RT60/EDT/C50/C80/D50)
- `stats/time`: time-domain signal statistics
- `stats/frequency`: frequency-domain statistics

## Project Docs

- Roadmap: `PLAN.md`
- API review notes: `API_REVIEW.md`
- Changelog: `CHANGELOG.md`
- Migration guide: `MIGRATION.md`
- Benchmarks: `BENCHMARKS.md`
- Contributing guide: `CONTRIBUTING.md`

## Development

Requirements:

- Go 1.25+
- `just` (optional)

Common commands:

- `just fmt`
- `just test`
- `just test-race`
- `just lint`
- `just bench`
- `just ci`

## Scope Boundaries

- This repository is algorithm-centric only.
- UI/runtime/audio-device/file-codec integrations are intentionally out of scope.
