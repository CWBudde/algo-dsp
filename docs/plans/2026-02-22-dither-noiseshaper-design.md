# Phase 29: Dither and Noise Shaping — Design Document

Date: 2026-02-22

## Overview

Add quantization support to `algo-dsp` with configurable dither PDFs and noise-shaping
paths. The implementation lives in `dsp/dither/` (runtime) and `dsp/dither/design/`
(coefficient optimizer), ported from the legacy Pascal sources
`DAV_DspDitherNoiseShaper.pas` and `DAV_DspNoiseShapingFilterDesigner.pas`.

## Package Layout

```
dsp/dither/
  doc.go                   Package documentation
  dither.go                DitherType enum and dither noise helpers
  quantizer.go             Quantizer struct: dither + noise shaping + quantization
  options.go               Option functions, config struct, defaults
  presets.go               Predefined FIR coefficient sets from legacy
  shaper.go                NoiseShaper interface
  shaper_fir.go            FIR error-feedback noise shaper (ring buffer)
  shaper_iir.go            IIR shelf-based noise shaper (wraps biquad)
  quantizer_test.go        Unit tests
  presets_test.go          Coefficient/preset validation tests
  shaper_test.go           Noise shaper unit tests
  example_test.go          Runnable examples
  quantizer_bench_test.go  Benchmarks

dsp/dither/design/
  doc.go                   Package documentation
  designer.go              Stochastic ATH-weighted coefficient optimizer
  ath.go                   Absolute threshold of hearing and critical bandwidth models
  designer_test.go         Tests
  example_test.go          Runnable examples
```

## Core Types

### DitherType

```go
type DitherType int

const (
    DitherNone         DitherType = iota // No dither (truncation)
    DitherRectangular                    // Rectangular (uniform) PDF
    DitherTriangular                     // Triangular PDF (TPDF)
    DitherGaussian                       // Exact Gaussian PDF
    DitherFastGaussian                   // Approximated Gaussian PDF
)
```

### NoiseShaper interface

```go
type NoiseShaper interface {
    Shape(input, quantizationError float64) float64
    Reset()
}
```

Two implementations:
- **FIRShaper**: ring-buffer error-feedback filter with configurable coefficients.
- **IIRShelfShaper**: wraps existing `filter/biquad` low-shelf section.

### Quantizer

The main public type combining dither + noise shaping + bit-depth quantization.

```go
func NewQuantizer(sampleRate float64, opts ...Option) (*Quantizer, error)

func (q *Quantizer) ProcessSample(input float64) float64
func (q *Quantizer) ProcessInteger(input float64) int
func (q *Quantizer) ProcessInPlace(buf []float64)
func (q *Quantizer) Reset()
```

### FIR Presets (from legacy)

| Name             | Order | Description                         |
|------------------|-------|-------------------------------------|
| PresetNone       | 0     | No shaping                          |
| PresetEFB        | 1     | Simple error feedback               |
| Preset2SC        | 2     | Simple 2nd-order highpass           |
| Preset2MEC       | 2     | Modified E-weighted, 2nd order      |
| Preset3MEC       | 3     | Modified E-weighted, 3rd order      |
| Preset9MEC       | 9     | Modified E-weighted, 9th order      |
| Preset5IEC       | 5     | Improved E-weighted, 5th order      |
| Preset9IEC       | 9     | Improved E-weighted, 9th order      |
| Preset3FC        | 3     | F-weighted, 3rd order               |
| Preset9FC        | 9     | F-weighted, 9th order (default)     |
| PresetSBM        | 12    | Sony Super Bit Mapping              |
| PresetSBMReduced | 10    | Reduced Super Bit Mapping           |
| PresetSharp14k   | 7     | Sharp 14 kHz rolloff (44.1 kHz)     |
| PresetSharp15k   | 8     | Sharp 15 kHz rolloff (44.1 kHz)     |
| PresetSharp16k   | 9     | Sharp 16 kHz rolloff (44.1 kHz)     |
| PresetExperimental | 9   | Experimental                        |

Sample-rate-adaptive sharp presets (selected automatically by `WithSharpPreset()`):

| Sample Rate Range | Coefficients     |
|-------------------|------------------|
| < 41000           | Sharp15k @ 40000 |
| 41000–46000       | Sharp15k @ 44100 |
| 46000–55000       | Sharp15k @ 48000 |
| 55000–75100       | Sharp15k @ 64000 |
| >= 75100          | Sharp15k @ 96000 |

## Options

```go
type Option func(*config) error

WithBitDepth(bits int)           // 1–32, default 16
WithDitherType(dt DitherType)    // default DitherTriangular
WithDitherAmplitude(amp float64) // default 1.0
WithLimit(enabled bool)          // default true
WithNoiseShaper(ns NoiseShaper)  // default FIRShaper with Preset9FC
WithFIRPreset(p Preset)          // shorthand for FIR shaper from preset
WithSharpPreset()                // sample-rate-adaptive sharp preset
WithIIRShelf(freq float64)       // shorthand for IIR shelf shaper
WithRNG(rng *rand.Rand)          // deterministic RNG for testing
```

## Processing Algorithm

Matches the legacy approach:

1. **Scale**: `input *= bitMul` where `bitMul = 2^(bitDepth-1) - 0.5`
2. **Noise shaping**: `input = shaper.Shape(input, 0)` (error from previous sample)
3. **Dither + quantize**: add dither noise per DitherType, round to integer
4. **Limit** (optional): clamp to `[-2^(bitDepth-1), 2^(bitDepth-1)-1]`
5. **Store error**: `error = quantized - pre_quantized_input`
6. **Normalize**: return `(quantized + 0.5) / bitMul`

## Differences from Legacy

1. **float64 only** — no separate 32-bit variant.
2. **Deterministic RNG injection** — `WithRNG(*rand.Rand)` for reproducible tests.
3. **NoiseShaper as interface** — FIR and IIR are interchangeable; custom shapers
   can be plugged in.
4. **Context-based cancellation** in the designer (vs. legacy's infinite loop).
5. **Bug fix**: legacy `TDitherNoiseShaper64.AssignTo` checked for 32-bit twice.

## Design Package (`dsp/dither/design`)

Stochastic coefficient optimizer for psychoacoustically-weighted noise shapers.

### ATH Model

Painter & Spanias (1997), modified by Gabriel Bouvigne:

```
ATH(f) = 3.640 * f^(-0.8)
       - 6.800 * exp(-0.6 * (f - 3.4)^2)
       + 6.000 * exp(-0.15 * (f - 8.7)^2)
       + 0.6 * 0.001 * f^4
```

where f is in kHz.

### Critical Bandwidth

Zwicker (1982):

```
CB(f) = 25 + 75 * (1 + 1.4 * (f/1000)^2)^0.69
```

### Optimizer API

```go
func NewDesigner(sampleRate float64, opts ...DesignerOption) (*Designer, error)
func (d *Designer) Run(ctx context.Context) ([]float64, error)
```

Options: `WithOrder(n)`, `WithIterations(n)`, `WithOnProgress(callback)`.

The optimizer randomly perturbs coefficients, evaluates candidates by FFT'ing
the impulse response `[1, c0, c1, ...]` and computing the ATH-weighted peak
magnitude. Keeps the best candidate. Returns the coefficient slice usable
directly with `NewFIRShaper(coeffs)`.

## Testing Strategy

1. Constructor/setter validation (NaN, Inf, zero, out-of-range)
2. Deterministic output with fixed RNG seed
3. Silence preservation (zero in, no dither -> zero out)
4. Reset correctness
5. ProcessSample / ProcessInPlace parity
6. Spectral validation (FFT error signal, verify in-band noise reduction)
7. Preset coefficient parity with legacy constants
8. Stability under clipping (1000+ hot samples, no NaN/Inf)
9. Benchmarks (per-sample cost, zero allocations)
10. Designer convergence test (verify optimizer reduces weighted peak)
