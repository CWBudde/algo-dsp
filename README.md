# algo-dsp

Production-quality DSP (Digital Signal Processing) algorithms for Go. Algorithm-centric and transport-agnostic -- no UI, audio device, or file format dependencies.

**Module**: `github.com/cwbudde/algo-dsp`

Try an interactive DSP demo live in your browser: [https://cwbudde.github.io/algo-dsp/](https://cwbudde.github.io/algo-dsp/)

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

## Packages

### DSP Core (`dsp/`)

| Package                | Description                                                                                                                                                                                                            |
| ---------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `dsp/window`           | 46 window types (Hann, Hamming, Blackman, Kaiser, Flat-Top, Tukey, Gaussian, Albrecht, Lawrey, Burgess, and more) with slope modes, DC removal, inversion, and spectral metadata (ENBW, sidelobe level, coherent gain) |
| `dsp/filter/biquad`    | Biquad IIR filter with Direct Form II Transposed topology. Single-sample and block processing, cascadeable chains, state save/restore. SIMD-accelerated (AVX2, SSE2, NEON)                                             |
| `dsp/filter/fir`       | FIR filter runtime with circular-buffer delay line                                                                                                                                                                     |
| `dsp/filter/design`    | Biquad coefficient designers (Lowpass, Highpass, Bandpass, Notch, Allpass, Peak, LowShelf, HighShelf) plus Butterworth and Chebyshev Type I/II cascade design                                                          |
| `dsp/filter/bank`      | Octave and fractional-octave filter bank builders with standard center frequencies                                                                                                                                     |
| `dsp/filter/weighting` | A/B/C/Z frequency weighting filters (IEC 61672 validated) as pre-designed biquad chains                                                                                                                                |
| `dsp/spectrum`         | Magnitude, phase, power extraction from complex FFT output. Phase unwrapping, group delay, 1/N-octave smoothing                                                                                                        |
| `dsp/conv`             | Direct, overlap-add, and overlap-save convolution. Cross/auto-correlation. Deconvolution (naive, regularized, Wiener). Auto-selects strategy based on kernel size                                                      |
| `dsp/resample`         | Polyphase FIR resampler with rational ratio API. Three quality modes (Fast/Balanced/Best) with configurable anti-aliasing                                                                                              |
| `dsp/signal`           | Signal generators (sine, multisine, white/pink noise, impulse, linear/log sweep) and utilities (normalize, clip, DC removal, envelope)                                                                                 |
| `dsp/effects`          | Algorithmic audio effects: soft-knee compressor (log₂-domain, zero-alloc), time-domain pitch shifter (WSOLA), frequency-domain pitch shifter (phase vocoder), chorus (modulated delay), feedback delay, reverb (Schroeder/Freeverb). Optional fast-math mode (`-tags=fastmath`) for 26% speedup |
| `dsp/buffer`           | Buffer type with pool-based reuse for real-time hot paths                                                                                                                                                              |
| `dsp/core`             | Numeric helpers (clamp, epsilon compare, dB conversions)                                                                                                                                                               |

### Measurement (`measure/`)

| Package         | Description                                                                                                                                           |
| --------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------- |
| `measure/thd`   | THD and THD+N analysis with auto fundamental detection, odd/even harmonic separation, rub-and-buzz detection, SINAD, and configurable frequency range |
| `measure/sweep` | Log and linear sweep generation with inverse filter calculation, FFT-based deconvolution, and harmonic IR extraction                                  |
| `measure/ir`    | Impulse response metrics: RT60, EDT, T20, T30, C50, C80, D50, D80, center time, Schroeder backward integration                                        |

### Statistics (`stats/`)

| Package           | Description                                                                                                                                                                     |
| ----------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `stats/time`      | Single-pass time-domain statistics (DC, RMS, peak, crest factor, energy, power, zero crossings, variance, skewness, kurtosis) with streaming incremental mode. Zero allocations |
| `stats/frequency` | Spectral descriptors: centroid, spread, flatness (Wiener entropy), rolloff, 3dB bandwidth. Works with magnitude or complex spectrum input. Zero allocations                     |

## Features

- **Zero-allocation fast paths** for real-time and streaming use cases
- **SIMD acceleration** on amd64 (AVX2/SSE2) and arm64 (NEON) with automatic runtime detection and pure-Go scalar fallbacks (`-tags=purego`)
- **Deterministic processing** -- same input + options = same output
- **Streaming-friendly APIs** with both one-shot and in-place processing variants
- **Comprehensive test coverage** (90%+ on core packages) with golden-vector validation against trusted references

## Development

Requirements: Go 1.25+, `just` (optional)

```bash
just test       # Run all tests
just test-race  # Run tests with race detector
just lint       # Run golangci-lint
just fmt        # Format code
just bench      # Run benchmarks
just ci         # Run all CI checks
```

Or without `just`:

```bash
go test ./...
go test -race ./...
go test -bench=. -benchmem ./...
```

## Scope Boundaries

This library is algorithm-centric and transport-agnostic:

- **No** UI/visualization frameworks
- **No** audio device APIs (ASIO, CoreAudio, JACK, PortAudio)
- **No** file format codecs (WAV, AIFF, FLAC)
- **No** application logging or configuration frameworks

Related repositories: [`algo-fft`](https://github.com/cwbudde/algo-fft) (FFT backend), [`wav`](https://github.com/cwbudde/wav) (WAV support), [`mfw`](https://github.com/cwbudde/mfw) (application)

## Project Docs

- [PLAN.md](PLAN.md) -- development roadmap
- [CHANGELOG.md](CHANGELOG.md) -- release notes
- [BENCHMARKS.md](BENCHMARKS.md) -- performance baselines
- [API_REVIEW.md](API_REVIEW.md) -- API review notes
- [MIGRATION.md](MIGRATION.md) -- migration guide from mfw
- [CONTRIBUTING.md](CONTRIBUTING.md) -- contribution guidelines
- [Web demo](web/) -- GitHub Pages step sequencer with DSP running in Go/WASM

## License

See [LICENSE](LICENSE).
