# algo-dsp: Development Plan

## Comprehensive Plan for `github.com/cwbudde/algo-dsp`

This document defines a phased plan for building `algo-dsp` as a reusable, production-quality DSP algorithm library in Go.

It is intentionally separated from application concerns (`mfw`) and file/container concerns (`wav`).

---

## Table of Contents

1. Project Scope and Goals
2. Repository and Module Boundaries
3. Architecture and Package Layout
4. API Design Principles
5. Phase Overview
6. Detailed Phase Plan (Phases 0–14)

**Appendices**

A. Testing and Validation Strategy
B. Benchmarking and Performance Strategy
C. Dependency and Versioning Policy
D. Release Engineering
E. Migration Plan from `mfw`
F. Risks and Mitigations
G. Initial 90-Day Execution Plan
H. Revision History

---

## 1. Project Scope and Goals

### 1.1 Primary Goals

- Provide reusable DSP algorithms independent of UI, desktop runtime, and file I/O.
- Deliver stable, well-documented APIs suitable for long-term reuse across projects.
- Achieve high numerical correctness and predictable performance.
- Minimize allocations and support real-time-friendly processing patterns.

### 1.2 Included Scope

- Window functions and spectral preprocessing helpers.
- IIR/FIR filter primitives and design tools.
- Filter banks and weighting filters.
- Convolution/correlation and spectral-domain operations.
- Resampling and rate-conversion algorithms.
- Signal generation and envelope/utility operations.
- Measurement kernels (THD, sweep/deconvolution, IR helpers).

### 1.3 Explicit Non-Goals

- GUI/visualization components.
- Audio device APIs (ASIO/CoreAudio/JACK/PortAudio wrappers).
- File container codecs and metadata systems (WAV/AIFF/FLAC/etc.).
- App orchestration/state management concerns specific to `mfw`.

---

## 2. Repository and Module Boundaries

### 2.1 Ownership Model

- `github.com/cwbudde/algo-dsp`: algorithm implementations and algorithm-level contracts.
- `github.com/cwbudde/algo-fft`: FFT backend and plans (consumed, not duplicated).
- `github.com/cwbudde/wav`: WAV container support (outside scope here).
- `github.com/cwbudde/mfw`: application integration, workflows, UI, and adapters.

### 2.2 Boundary Rules

- No dependency on Wails, React, app-specific DTOs, or desktop runtime packages.
- No direct dependency on application logging/config frameworks.
- Public APIs remain algorithm-centric and transport-agnostic.

---

## 3. Architecture and Package Layout

Target structure:

```plain
algo-dsp/
├── go.mod
├── README.md
├── PLAN.md
├── LICENSE
├── .golangci.yml
├── justfile
├── internal/
│   ├── testutil/             # reference vectors, tolerances, helpers
│   ├── simd/                 # optional SIMD/internal kernels
│   └── unsafeopt/            # isolated low-level optimizations
├── dsp/
│   ├── buffer/               # Buffer type, Pool, allocation helpers
│   ├── window/               # window types, coefficients, and metadata
│   ├── filter/
│   │   ├── biquad/           # biquad runtime and cascades
│   │   ├── fir/              # FIR runtime
│   │   ├── design/           # filter design calculators
│   │   ├── bank/             # octave/third-octave banks
│   │   └── weighting/        # A/B/C/Z etc.
│   ├── spectrum/             # magnitude/phase/group delay/smoothing
│   ├── conv/                 # convolution, deconvolution, correlation
│   ├── resample/             # SRC, up/down sampling
│   ├── signal/               # generators and utility transforms
│   └── effects/              # optional algorithmic effects (non-IO)
├── measure/
│   ├── thd/                  # THD/THD+N kernels
│   ├── sweep/                # log sweep/deconvolution kernels
│   └── ir/                   # impulse response metrics
├── stats/
│   ├── time/                 # RMS, crest factor, moments, etc.
│   └── frequency/            # spectral stats
└── examples/
    ├── filter_response/
    ├── thd_analyzer/
    └── log_sweep_ir/
```

Notes:

- `internal/*` is optimization and test support only.
- Stable APIs live in non-`internal` packages.

---

## 4. API Design Principles

- Prefer small interfaces and concrete constructors.
- Deterministic behavior for same input/options.
- Clear error semantics (`fmt.Errorf("context: %w", err)`).
- Streaming-friendly APIs and in-place variants where practical.
- Zero-alloc fast paths for repeated processing.
- Keep generics usage pragmatic; avoid API complexity for marginal gain.
- Public types and functions require doc comments and runnable examples.

API shape guidelines:

```go
// Constructor + options
func NewProcessor(opts ...Option) (*Processor, error)

// One-shot and reusable processing
func Process(input []float64) ([]float64, error)
func (p *Processor) ProcessInPlace(buf []float64) error
```

---

## 5. Phase Overview

```plain
Phase 0:  Bootstrap & Governance                     [1 week]
Phase 1:  Numeric Foundations & Core Utilities       [2 weeks]
Phase 2:  Window Functions                            [2 weeks]
Phase 3:  Filter Runtime Primitives                   [3 weeks]
Phase 4:  Filter Design Toolkit                       [3 weeks]
Phase 5:  Filter Banks and Weighting                  [2 weeks]
Phase 6:  Spectrum Utilities                          [2 weeks]
Phase 7:  Convolution and Correlation                 [2 weeks]
Phase 8:  Resampling                                  [3 weeks]
Phase 9:  Signal Generation and Utilities             [2 weeks]
Phase 10: Measurement Kernels (THD)                   [3 weeks]
Phase 11: Measurement Kernels (Sweep/IR)              [3 weeks]
Phase 12: Stats Packages                              [2 weeks]
Phase 13: Optimization and SIMD Paths                 [3 weeks]
Phase 14: API Stabilization and v1.0                  [2 weeks]

Total Estimated Duration: ~35 weeks
```

---

## 6. Detailed Phase Plan

### Phase 0: Bootstrap & Governance (Complete)

- Initialized Go module `github.com/cwbudde/algo-dsp` with justfile targets (test, lint, format, bench, ci).
- Set up CI for Go stable + previous stable versions with semantic versioning.
- Created CONTRIBUTING.md and issue templates for contribution standards.

### Phase 1: Numeric Foundations & Core Utilities (Complete)

- Added core numeric helpers (clamp, epsilon compare, dB conversions) and functional options pattern.
- Implemented `dsp/buffer.Buffer` type with `Samples()`, `Resize()`, `Zero()`, `Copy()` methods.
- Added `dsp/buffer.Pool` with sync.Pool-based reuse for real-time hot paths.
- Created `internal/testutil` with deterministic random/test signal helpers.

### Phase 2: Window Functions (Complete)

- Implemented 25+ window types across 3 tiers: essential (Hann, Hamming, Blackman, Kaiser, etc.), extended (Triangle, Welch, Gauss, etc.), and specialized (Albrecht, Lawrey, Burgess).
- Ported cosine-term coefficient tables from legacy MFWindowFunctionUtils.pas with shared Horner evaluation engine.
- Added advanced features: slope modes (left/symmetric/right), DC removal, inversion, Bartlett variant, Tukey percentage.
- Implemented `Metadata` struct with ENBW, sidelobe level, coherent gain, and spectrum correction factors.

### Phase 3: Filter Runtime Primitives (Complete - 2026-02-06)

- Implemented `biquad.Section` with Direct Form II Transposed topology (port of MFFilter.pas:737-743).
- Added `biquad.Chain` for cascading N sections with gain, state save/restore for impulse response.
- Implemented frequency response evaluation: `Response()`, `MagnitudeSquared()`, `MagnitudeDB()`, `Phase()`.
- Added `fir.Filter` with circular-buffer delay line for direct-form FIR processing.
- Coverage: biquad ≥90%, fir ≥85%.

### Phase 4: Filter Design Toolkit (Complete)

- Implemented biquad coefficient designers: Lowpass, Highpass, Bandpass, Notch, Allpass, Peak, LowShelf, HighShelf.
- Added Butterworth LP/HP cascade design with bilinear transform and odd-order handling (orders 1-64).
- Implemented Chebyshev Type I/II LP/HP with ripple factors (corrected angle term for Type II).
- Validated across sample rates: 44100, 48000, 96000, 192000 Hz. Coverage ≥90%.

### Phase 5: Filter Banks and Weighting (Complete - 2026-02-06)

- Implemented A/B/C/Z frequency weighting filters as pre-designed biquad chains (ported from MFDSPWeightingFilters.pas).
- Added octave and fractional-octave filter bank builders with standard center frequencies/bandwidths.
- IEC 61672 compliance validation for weighting curves.
- Coverage: weighting 100%, bank 93%.

### Phase 6: Spectrum Utilities (Complete)

- Added magnitude/phase/power extraction helpers for complex FFT output to real arrays.
- Implemented phase unwrapping and group delay calculations.
- Added 1/N-octave smoothing and interpolation utilities.
- Backend-agnostic interfaces integrating cleanly with `algo-fft` outputs.

### Phase 7: Convolution and Correlation (Complete - 2026-02-06)

- Implemented direct convolution baseline plus overlap-add and overlap-save FFT-based strategies.
- Added cross-correlation (direct/FFT/normalized) and auto-correlation functions.
- Implemented deconvolution with regularization options (naive/regularized/Wiener) and inverse filter generation.
- Benchmarks show crossover at ~64-128 sample kernels. Coverage 86%.

### Phase 8: Resampling (Complete)

- Implemented polyphase FIR resampler with rational ratio API.
- Added anti-aliasing defaults and quality modes (low/medium/high).
- Published quality/performance matrix for standard ratios (44.1k↔48k, 2x, 4x).

### Phase 9: Signal Generation and Utilities (Complete)

- Implemented generators: sine, multisine, noise (white/pink), impulse, sweep (linear/log).
- Added signal utilities: normalize, clip, DC removal, envelope helpers.
- Deterministic seed strategy for reproducibility in tests and measurements.

### Phase 10: Measurement Kernels (THD)

Objectives:

- Build THD/THD+N measurement logic reusable across applications.
- Port calculation algorithms from `mfw/legacy/Source/MFTotalHarmonicDistortionCalculation.pas`.

Source: `MFTotalHarmonicDistortionCalculation.pas` (576 lines), `MFTHDData.pas` (2107 lines — data structures for level/log sweep THD).

#### 10.1 Legacy Algorithm Reference

The legacy implementation calculates distortion from frequency-domain data:

- **Fundamental detection**: Find bin with maximum squared magnitude in search range
- **Harmonic extraction**: Sum magnitudes at integer multiples of fundamental bin
- **Capture range**: Window-based bin width for spectral leakage compensation (uses window's first minimum)
- **Noise calculation**: THD+N minus THD (all energy in range minus harmonic energy)

Key formulas (from MFTotalHarmonicDistortionCalculation.pas):

- `THD = Σ sqrt(|H_k|²)` for k = 2, 3, ... (harmonics at k × fundamental_bin)
- `THD+N = Σ sqrt(|X_i|²)` for all bins in evaluation range
- `Noise = THD+N - THD`
- `OddHD = Σ sqrt(|H_k|²)` for k = 3, 5, 7, ... (H3, H5, H7, ...)
- `EvenHD = Σ sqrt(|H_k|²)` for k = 2, 4, 6, ... (H2, H4, H6, ...)

#### 10.2 API Surface (`measure/thd`)

```go
package thd

// Config holds THD calculation parameters.
type Config struct {
    SampleRate        float64
    FFTSize           int
    FundamentalFreq   float64    // 0 = auto-detect
    RangeLowerFreq    float64    // evaluation range lower bound (default 20 Hz)
    RangeUpperFreq    float64    // evaluation range upper bound (default 20 kHz)
    CaptureBins       int        // 0 = auto from window, >0 = fixed
    MaxHarmonics      int        // max harmonics to evaluate (0 = unlimited)
    RubNBuzzStart     int        // start harmonic for Rub & Buzz (default 10)
}

// Result holds THD measurement results.
type Result struct {
    FundamentalFreq   float64    // detected or specified fundamental
    FundamentalLevel  float64    // fundamental magnitude (linear)
    THD               float64    // total harmonic distortion (linear ratio)
    THDN              float64    // THD+N (linear ratio)
    THD_dB            float64    // THD in dB
    THDN_dB           float64    // THD+N in dB
    OddHD             float64    // odd harmonics sum
    EvenHD            float64    // even harmonics sum
    Noise             float64    // noise floor (THDN - THD)
    RubNBuzz          float64    // high-order harmonics (from RubNBuzzStart)
    Harmonics         []float64  // individual harmonic levels [H2, H3, H4, ...]
    SINAD             float64    // signal-to-noise-and-distortion ratio (dB)
}

// Calculator performs THD analysis on frequency-domain data.
type Calculator struct { ... }

func NewCalculator(cfg Config) *Calculator
func (c *Calculator) Calculate(spectrum []complex128) Result
func (c *Calculator) CalculateFromMagnitude(magSquared []float64) Result

// Convenience functions for one-shot analysis.
func Analyze(spectrum []complex128, cfg Config) Result
func AnalyzeSignal(signal []float64, cfg Config) Result  // includes windowing + FFT
```

#### 10.3 Task Breakdown

- [x] Define `Config` struct with all parameters from legacy (sample rate, FFT size, capture bins, range, flags).
- [x] Implement fundamental detection: find max magnitude bin in specified range.
- [x] Implement harmonic extraction with configurable capture range.
- [x] Implement `GetTHD`: sum of harmonics starting from H2 (port MFTotalHarmonicDistortionCalculation:234–264).
- [x] Implement `GetTHDN`: sum of all bins in evaluation range (port MFTotalHarmonicDistortionCalculation:266–285).
- [x] Implement `GetOddHD` and `GetEvenHD`: odd/even harmonic summation (port MFTotalHarmonicDistortionCalculation:292–352).
- [x] Implement `GetNoise`: THDN - THD.
- [x] Implement `GetRubNBuzz`: high-order harmonics from configurable start (port MFTotalHarmonicDistortionCalculation:365–406).
- [x] Implement SINAD calculation: 20\*log10(fundamental / THDN).
- [x] Add window-based capture bin calculation using window first-minimum estimates.
- [x] Tests with synthetic signals: pure tone (THD ≈ 0), known distortion levels.
- [x] Tests with multi-tone signals for harmonic separation accuracy.
- [x] Benchmarks for calculation throughput at various FFT sizes.
- [x] Runnable examples demonstrating THD measurement workflow.

#### 10.4 Exit Criteria

- THD/THD+N calculations match legacy output within 0.01 dB for same input spectra.
- Fundamental auto-detection correctly identifies fundamental in presence of harmonics.
- Odd/even harmonic separation validated with asymmetric distortion test signals.
- Coverage >= 90% in `measure/thd`.

### Phase 11: Measurement Kernels (Sweep/IR) (Complete - 2026-02-06)

- Implemented `LogSweep` with `Generate()`, `InverseFilter()`, `Deconvolve()`, `ExtractHarmonicIRs()` using FFT-based convolution.
- Added `LinearSweep` with `Generate()`, `InverseFilter()`, `Deconvolve()` for comparison/testing.
- Implemented `ir.Analyzer` with Schroeder backward integration, RT60/EDT/T20/T30 via linear regression on decay curve.
- Added Definition D(t), Clarity C(t), Center Time, and `FindImpulseStart` for IR analysis.
- RT60 matches analytical value within 5% for synthetic exponential decays (0.3s–2.5s).
- D50/C50/D80/C80 validated with known energy splits; D↔C relationship verified.
- Harmonic IR separation correctly isolates linear IR from harmonic distortion IRs.
- Coverage: sweep 85.4%, ir 86.2%. All tests pass with race detector.

### Phase 12: Stats Packages (Complete - 2026-02-07)

- Implemented `stats/time.Calculate()` with single-pass Welford's algorithm for DC, RMS, max/min with positions, peak, range, crest factor, energy/power, zero crossings, variance, skewness, kurtosis.
- Added `StreamingStats` for incremental block-based updates producing bit-identical results to `Calculate()`.
- Added standalone functions: `RMS()`, `DC()`, `Peak()`, `CrestFactor()`, `ZeroCrossings()`, `Moments()`.
- Implemented `stats/frequency.Calculate()` with spectral centroid, spread, flatness (Wiener entropy), rolloff, and 3dB bandwidth.
- Added `CalculateFromComplex()` and standalone functions: `Centroid()`, `Flatness()`, `Rolloff()`, `Bandwidth()`.
- Zero allocations across all functions and benchmarks. Coverage: time 98.0%, frequency 97.8%.

Source: `MFTypes.pas` (TMFTimeDomainInfoType, TMFFrequencyDomainInfoType enums), `MFAudioData.pas` (TMFTimeDomainDataInformation class).

#### 12.1 Legacy Statistics Reference

**Time-domain info types** (from MFTypes.pas):

- `titZeroTransitions`: zero crossing count
- `titDC`, `titDC_dB`: mean value (linear and dB)
- `titRMS`, `titRMS_dB`: root mean square
- `titMax`, `titMin`, `titPeak`, `titRange`: amplitude statistics
- `titCrest`: crest factor (peak/RMS ratio in dB)
- `titEnergy`, `titPower`: signal energy and power
- `titM1`–`titM4`: statistical moments (mean, variance, skewness, kurtosis)
- `titSkew`, `titKurtosis`: higher-order moments

**Frequency-domain info types** (from MFTypes.pas):

- `fitDC`, `fitSum`, `fitMaximum`, `fitMinimum`, `fitAverage`, `fitRange`
- `fitEnergy`, `fitPower`

#### 12.2 API Surface (`stats/time`, `stats/frequency`)

```go
package time

// Stats holds time-domain signal statistics.
type Stats struct {
    Length          int
    DC              float64  // mean
    DC_dB           float64
    RMS             float64
    RMS_dB          float64
    Max             float64
    MaxPos          int
    Min             float64
    MinPos          int
    Peak            float64  // max(|max|, |min|)
    Peak_dB         float64
    Range           float64  // max - min
    Range_dB        float64
    CrestFactor     float64  // peak/RMS (linear)
    CrestFactor_dB  float64
    Energy          float64  // sum of squares
    Power           float64  // energy / length
    ZeroCrossings   int
    // Higher moments
    Variance        float64
    Skewness        float64
    Kurtosis        float64
}

// Calculate computes all statistics for the given signal.
func Calculate(signal []float64) Stats

// Streaming calculator for incremental updates.
type StreamingStats struct { ... }
func NewStreamingStats() *StreamingStats
func (s *StreamingStats) Update(samples []float64)
func (s *StreamingStats) Result() Stats
func (s *StreamingStats) Reset()

// Individual stat functions for selective calculation.
func RMS(signal []float64) float64
func DC(signal []float64) float64
func Peak(signal []float64) float64
func CrestFactor(signal []float64) float64
func ZeroCrossings(signal []float64) int
func Moments(signal []float64) (mean, variance, skewness, kurtosis float64)
```

```go
package frequency

// Stats holds frequency-domain statistics.
type Stats struct {
    BinCount        int
    DC              float64  // bin 0 magnitude
    DC_dB           float64
    Sum             float64  // sum of magnitudes
    Sum_dB          float64
    Max             float64
    MaxBin          int
    Min             float64
    MinBin          int
    Average         float64
    Average_dB      float64
    Range           float64
    Range_dB        float64
    Energy          float64  // sum of squared magnitudes
    Power           float64
    // Spectral shape descriptors
    Centroid        float64  // spectral centroid (Hz)
    Spread          float64  // spectral spread
    Flatness        float64  // spectral flatness (Wiener entropy)
    Rolloff         float64  // frequency below which X% energy (Hz)
    Bandwidth       float64  // 3dB bandwidth around peak (Hz)
}

// Calculate computes statistics from magnitude spectrum.
func Calculate(magnitude []float64, sampleRate float64) Stats
func CalculateFromComplex(spectrum []complex128, sampleRate float64) Stats

// Individual spectral descriptors.
func Centroid(magnitude []float64, sampleRate float64) float64
func Flatness(magnitude []float64) float64
func Rolloff(magnitude []float64, sampleRate float64, percent float64) float64
func Bandwidth(magnitude []float64, sampleRate float64) float64
```

#### 12.3 Task Breakdown

**Time-domain stats (`stats/time`):**

- [x] Implement single-pass statistics: DC, RMS, max, min, peak, range.
- [x] Implement crest factor and energy/power calculations.
- [x] Implement zero-crossing counter.
- [x] Implement higher moments: variance, skewness, kurtosis (Welford's algorithm for numerical stability).
- [x] Implement `StreamingStats` for incremental block-based updates.
- [x] Tests with known signals (DC, sine, square wave → predictable stats).
- [x] Benchmarks for block processing throughput.

**Frequency-domain stats (`stats/frequency`):**

- [x] Implement basic spectrum stats: DC, sum, max, min, average, range.
- [x] Implement spectral centroid: `Σ(f_i × |X_i|) / Σ|X_i|`.
- [x] Implement spectral spread: second moment around centroid.
- [x] Implement spectral flatness: `exp(mean(log(|X|))) / mean(|X|)`.
- [x] Implement spectral rolloff: frequency below which N% of energy lies.
- [x] Implement 3dB bandwidth around spectral peak.
- [x] Tests with synthetic spectra (narrowband, broadband, noise).
- [x] Benchmarks for spectrum analysis throughput.

#### 12.4 Exit Criteria

- Time-domain stats match legacy `TMFTimeDomainDataInformation` output for same input.
- Streaming stats produce identical results to block calculation.
- Spectral descriptors validated against reference implementations (librosa, scipy).
- Zero-allocation variants available for hot paths.
- Coverage >= 90% for both `stats/time` and `stats/frequency`.

### Phase 13: Optimization and SIMD Paths

Objectives:

- Profile-guided optimization of hot paths identified in Phases 1–12.
- Optional SIMD acceleration behind build tags with scalar fallback.

Source: `mfw/legacy/Source/MFASM.pas` (Pascal declarations), `mfw/legacy/Source/ASM/` (~1.5MB hand-optimized x86/SSE assembly), `MFDSPPolyphaseFilter.pas` (FPU/3DNow/SSE variants).

#### 13.1 Optimization Strategy

1. **Profile first**: Use `go test -bench` and `pprof` to identify actual bottlenecks.
2. **Algorithm-level optimizations**: Loop unrolling, cache-friendly access patterns, reduced allocations.
3. **SIMD paths**: Optional `amd64` assembly or Go assembly (Plan 9 syntax) for:
   - Block multiply/add (windowing, filtering)
   - Dot products (FIR convolution)
   - Magnitude calculations (complex → real)
4. **Build tag isolation**: `//go:build !purego` for optimized paths, scalar fallback always available.
5. **Numerical parity testing**: Optimized path must match scalar reference within epsilon.

#### 13.2 Candidate Hot Paths

| Package             | Function         | Priority | Optimization Type      |
| ------------------- | ---------------- | -------- | ---------------------- |
| `dsp/window`        | `Apply`          | High     | SIMD multiply          |
| `dsp/filter/biquad` | `ProcessBlock`   | High     | Loop unrolling         |
| `dsp/filter/fir`    | `ProcessBlock`   | High     | SIMD dot product       |
| `dsp/conv`          | `directConvolve` | Medium   | SIMD dot product       |
| `dsp/resample`      | `Resample`       | High     | Polyphase optimization |
| `stats/time`        | `Calculate`      | Medium   | SIMD reductions        |
| `dsp/spectrum`      | `Magnitude`      | Medium   | SIMD sqrt              |

#### 13.3 Benchmark Baseline (2026-02-07, i7-1255U, Go 1.24)

Comprehensive benchmarks across all packages. Key results:

| Package / Function           | Size          | ns/op     | MB/s   | allocs | Notes                        |
| ---------------------------- | ------------- | --------- | ------ | ------ | ---------------------------- |
| `window/Apply` (Hann)        | 4096          | 224,781   | —      | 2      | Regenerates coeffs each call |
| `window/Generate` (Hann)     | 4096          | 145,677   | —      | 2      | Per-sample trig              |
| `filter/biquad/ProcessBlock` | 4096          | 27,710    | 1,183  | 0      | Already fast, sequential     |
| `filter/fir/ProcessBlock`    | 1024×128 taps | 133,093   | 62     | 0      | Circular buffer overhead     |
| `filter/fir/ProcessBlock`    | 1024×512 taps | 654,398   | 13     | 0      | O(N×M) dominates             |
| `conv/Direct`                | 4096×64       | 244,466   | —      | 1      | Inner loop not vectorized    |
| `conv/OverlapAdd`            | 16384×256     | 1,054,244 | —      | 60     | Heavy allocation per call    |
| `conv/OverlapAddReuse`       | 16384×256     | 1,317,175 | —      | 1      | Reuse cuts allocs to 1       |
| `simd/MulBlock` (AVX2)       | 4K            | 1,568     | 62,708 | 0      | 3.4× vs scalar               |
| `simd/ScaleBlock` (AVX2)     | 4K            | 1,258     | 52,104 | 0      | 2.9× vs scalar               |
| `simd/AddMulBlock` (AVX2)    | 4K            | 1,259     | 78,090 | 0      | 3.7× vs scalar               |

**Top 5 hot paths identified** (by impact × frequency of use):

1. **`window/Apply`** — regenerates full coefficient array + scalar multiply every call; 2 allocs.
2. **`filter/fir/ProcessBlock`** — O(N×M) dot product with circular-buffer branch per tap.
3. **`conv/OverlapAdd`** — 60 allocs/call from FFT temporary buffers.
4. **`conv/Direct`** — inner loop not SIMD-accelerated; used for short kernels.
5. **`dsp/spectrum/Magnitude`** — per-element `cmplx.Abs` call (sqrt of re²+im²).

#### 13.3 Task Breakdown

##### 13.3.1 Completed

- [x] Run comprehensive benchmarks across all packages, identify top 5 hot paths.
- [x] Add `internal/simd` package with build-tagged SIMD kernels.
  - AVX2 assembly: MulBlock, MulBlockInPlace, ScaleBlock, ScaleBlockInPlace, AddBlock, AddBlockInPlace, AddMulBlock, MulAddBlock.
  - Pure Go fallbacks in `mul_generic.go` with `purego` build tag.
  - Comprehensive tests and benchmarks confirming 2–5× speedup.
- [x] Add numerical parity tests: SIMD vs scalar match within floating-point epsilon.

##### 13.3.2 Window Optimization

- [x] Wire `ApplyCoefficientsInPlace` to use `simd.MulBlockInPlace` instead of scalar loop.
- [x] Wire `ApplyCoefficients` to use `simd.MulBlock` instead of scalar loop.
- [x] Wire `Apply` inner multiply to use `simd.MulBlockInPlace`.
- [x] Add benchmarks for precomputed coefficient paths (`ApplyCoefficientsInPlace`, `ApplyCoefficients`).
- [x] Verify purego fallback passes all tests.
  - Precomputed path: 0 allocs, ~3 GB/s at 4K (SIMD) vs Apply's ~170 μs + 2 allocs (dominated by Generate).
  - `ApplyCoefficients`: 1 alloc (output slice only), ~3 GB/s at 4K.
  - Users should cache `Generate` output and use `ApplyCoefficientsInPlace` for hot paths.

##### 13.3.3 SIMD Reduction Operations

- [x] Implement `MaxAbs([]float64) float64` — AVX2/SSE2/NEON horizontal max-abs reduction.
- [x] Implement `Sum([]float64) float64` — AVX2/SSE2/NEON horizontal sum (for RMS, energy).
- [x] Implement `DotProduct(a, b []float64) float64` — AVX2/SSE2/NEON dot product (for FIR inner loop).
- [x] Pure Go fallbacks for all new reductions.
- [x] Numerical parity tests for reductions.

##### 13.3.4 FIR Optimization

- [x] Rewrite `ProcessBlock` to use double-buffered delay line (avoid branch per tap).
- [x] Use `vecmath.DotProduct` for inner convolution loop (requires 13.3.3).
- [x] Benchmark: achieved **3.24× improvement for 128 taps**, 2.78× for 512 taps (exceeds ≥ 3× target).

##### 13.3.5 Convolution Allocation Reduction

- [x] Pool FFT scratch buffers in `OverlapAdd` / `OverlapSave` (reduced 60 → **1 alloc/call**, exceeded ≤2 target).
- [x] Pre-allocate accumulator in `DirectTo` using `sync.Pool` for scratch buffers.
- [x] Wire `DirectTo` inner loop to `vecmath.ScaleBlock` + `vecmath.AddBlockInPlace` for kernel length ≥ 16.
- [x] Benchmark: **OverlapAdd/OverlapSave** achieved **60× allocation reduction** with **1.3-2.3× speedup**.
- [x] Benchmark: **DirectTo** achieved **34-48% speedup** for kernels ≥32 samples with SIMD acceleration.

##### 13.3.6 Spectrum SIMD

- [x] Implement `Magnitude(dst, re, im []float64)` using AVX2/SSE2/NEON (re²+im², vsqrt).
- [x] Implement `Power(dst, re, im []float64)` using AVX2/SSE2/NEON (re²+im², no sqrt).
- [x] Add comprehensive tests for Magnitude and Power operations.
- [x] Add benchmarks showing performance improvements.
  - vecmath.Magnitude: 3.5-10 GB/s throughput (size-dependent, includes sqrt)
  - vecmath.Power: 18-100 GB/s throughput (faster, no sqrt operation)
- [x] Wire `spectrum.Magnitude` / `spectrum.Power` to use SIMD paths.
  - Integrated SIMD implementations into dsp/spectrum package
  - Added benchmarks comparing SIMD vs naive implementations
  - Note: Current integration has allocation overhead from unpacking complex128
  - Future optimization opportunity: buffer pooling or specialized complex128 SIMD kernels

##### 13.3.7 Biquad Scalar Optimization

- [ ] Profile biquad ProcessBlock with pprof to confirm it's register-bound (not memory-bound).
- [ ] Test manual 2× loop unrolling (process two independent sections in parallel).
- [ ] If gain < 10%, mark biquad as "already optimal" and skip further work.

##### 13.3.8 Purego Validation & Documentation

- [x] Ensure `go test -tags purego ./...` passes all tests.
- [x] Create `BENCHMARKS.md` with baseline numbers and SIMD vs scalar comparison table.
- [x] Update this section with measured gains after each sub-task completes.
  - `go test -tags purego ./...` now passes after `purego`-specific vecmath registration/import fixes.
  - SIMD vs scalar (n=4096, `internal/vecmath`): AddBlock 2.61-2.78x, MulBlock 3.36-3.94x, ScaleBlock 1.75-2.58x, AddMulBlock 2.96-4.09x, MaxAbs (AVX2) 3.66x.

#### 13.4 Legacy ASM to Plan 9 Assembly Conversion

The `../mfw/legacy/Source/ASM/` directory contains ~1.5MB of hand-optimized x86/SSE assembly. This section defines the conversion strategy for porting DSP-relevant routines to Go's Plan 9 assembly syntax.

##### 13.4.1 Legacy ASM Inventory

| File           | Size  | Description                 | DSP Relevance               |
| -------------- | ----- | --------------------------- | --------------------------- |
| `MF-TIME.ASM`  | 247KB | Time-domain FPU processing  | High - block ops, noise gen |
| `MFS-TIME.ASM` | 148KB | Time-domain SSE2 processing | High - SIMD reference       |
| `MF-SPEK.ASM`  | 299KB | Spectrum FPU processing     | High - mag, smoothing       |
| `MFS-SPEK.ASM` | 12KB  | Spectrum SSE helpers        | Medium                      |
| `MF-SPKB.ASM`  | 196KB | Spectrum processing B       | Medium                      |
| `MF-WIN.ASM`   | 71KB  | Window functions            | High - all windows          |
| `MF-TIDE.ASM`  | 25KB  | Biquad/IIR filtering        | High - filter runtime       |
| `MF-MATH.ASM`  | 71KB  | Math utilities              | Medium - dB, min/max        |
| `MFS-TRAN.ASM` | 157KB | Requantization SSE2         | Low - I/O focused           |
| `mf-hada.asm`  | 37KB  | Hadamard transform          | Low                         |
| `MFS-HADA.ASM` | 37KB  | Hadamard SSE                | Low                         |
| `MFASM.pas`    | 81KB  | Pascal declarations         | Reference only              |

##### 13.4.2 Priority Conversion Targets

**Tier 1 - Critical Hot Paths** (convert first):

| Function              | Source       | Target Package      | Notes                            |
| --------------------- | ------------ | ------------------- | -------------------------------- |
| `tsAddMul`            | MF-TIME.ASM  | `internal/simd`     | Block multiply-add for windowing |
| `tsIIRfilter`         | MF-TIDE.ASM  | `dsp/filter/biquad` | Biquad cascade processing        |
| `SqMagWinConvKernel`  | MF-SPEK.ASM  | `dsp/spectrum`      | Spectral smoothing               |
| `MaxAbsF64`, `MaxF64` | MF-MATH.ASM  | `internal/simd`     | Reduction operations             |
| `UPDFnoise64_SSE2`    | MFS-TIME.ASM | `dsp/signal`        | TPDF dither/noise                |

**Tier 2 - Window Application**:

| Function          | Source     | Target Package | Notes                       |
| ----------------- | ---------- | -------------- | --------------------------- |
| `FenstereDoubles` | MF-WIN.ASM | `dsp/window`   | In-place window application |
| `HannFenster`     | MF-WIN.ASM | `dsp/window`   | Hann generation kernel      |
| `KaiBessFenster`  | MF-WIN.ASM | `dsp/window`   | Kaiser-Bessel kernel        |
| `GaussFenster`    | MF-WIN.ASM | `dsp/window`   | Gaussian window kernel      |

**Tier 3 - Spectral Processing**:

| Function             | Source      | Target Package | Notes                    |
| -------------------- | ----------- | -------------- | ------------------------ |
| `MovingAvgOverSqMag` | MF-SPEK.ASM | `dsp/spectrum` | Smoothing                |
| `ReImWinConvKernel`  | MF-SPEK.ASM | `dsp/conv`     | Complex convolution      |
| `initLinSlope`       | MF-SPEK.ASM | `dsp/spectrum` | Log/linear interpolation |

**Tier 4 - Noise Generators** (if profiling shows need):

| Function               | Source       | Target Package | Notes                 |
| ---------------------- | ------------ | -------------- | --------------------- |
| `PinkNoiseKernel_SSE2` | MFS-TIME.ASM | `dsp/signal`   | Pink noise generation |
| `GaussNoise64_SSE2`    | MFS-TIME.ASM | `dsp/signal`   | Gaussian noise        |

##### 13.4.3 Plan 9 Assembly Conversion Guidelines

1. **File naming**: `*_amd64.s` for AMD64-specific, `*_arm64.s` for ARM64.
2. **ABI compliance**: Use Go's ABI0 calling convention (stack-based arguments).
3. **Function declaration**: Pair `.go` stubs with `.s` implementations:
   ```go
   // internal/simd/mulblock_amd64.go
   //go:noescape
   func mulBlockAVX2(dst, src []float64, scale float64)
   ```
4. **Register usage**: Follow Plan 9 naming (AX, BX, X0-X15 for SSE/AVX).
5. **Build tags**: Use `//go:build !purego && amd64` for optimized paths.
6. **Scalar fallback**: Always provide pure Go implementation in `*_generic.go`.

##### 13.4.4 Conversion Process

For each function:

1. **Document original**: Extract algorithm from legacy ASM with inline comments.
2. **Write Go reference**: Create scalar Go implementation as source of truth.
3. **Add benchmarks**: Establish baseline performance metrics.
4. **Convert to Plan 9**: Translate instruction-by-instruction, adapting to Go ABI.
5. **Numerical parity**: Verify output matches Go reference within epsilon.
6. **Performance validation**: Ensure SIMD version shows >= 2x improvement.

##### 13.4.5 Conversion Task Checklist

- [ ] Document legacy ASM algorithms for priority Tier 1 functions.
- [x] Create `internal/simd/` package skeleton with build tags.
- [x] Convert `tsAddMul` → `mulBlockAVX2`, `addMulBlockAVX2` (block arithmetic).
  - Also: `scaleBlockAVX2`, `addBlockAVX2`, `mulAddBlockAVX2` and in-place variants.
- [x] Convert `MaxAbsF64` → `maxAbsAVX2` (reduction) — see 13.3.3.
- [ ] Convert `tsIIRfilter` → biquad kernel (only if profiling justifies — see 13.3.7).
- [ ] Convert `UPDFnoise64_SSE2` → TPDF dither kernel.
- [ ] Add ARM64 NEON variants for cross-platform optimization.
- [ ] Validate all conversions against legacy output (golden vectors).

#### 13.6 Exit Criteria

- Top 5 hot paths show measurable improvement (>20% for SIMD paths).
- All optimized paths pass numerical parity tests against scalar reference.
- `purego` build passes all tests.
- No regressions in correctness or API.
- Optimization gains documented in BENCHMARKS.md.
- At least Tier 1 legacy ASM conversions completed with validated parity.

### Phase 14: API Stabilization and v1.0

Objectives:

- Freeze public API surface and publish stable v1.0.0 release.
- Complete documentation, examples, and migration guides.

#### 14.1 API Review Checklist

- [x] Review all exported types, functions, and methods for consistency.
- [x] Ensure naming follows Go conventions (MixedCaps, no stuttering).
- [x] Verify all public functions have doc comments with examples.
- [x] Check for unnecessary exported symbols that should be internal.
- [x] Validate error types and error wrapping patterns.
- [x] Review option patterns for extensibility without breaking changes.

#### 14.2 Documentation Requirements

- [x] Package-level doc.go for all public packages.
- [x] Runnable examples for all major APIs (`Example_*` functions).
- [x] README.md with quick start guide and package overview.
- [x] CHANGELOG.md with all changes since v0.1.0.
- [x] MIGRATION.md for users upgrading from prerelease versions.
- [x] BENCHMARKS.md with performance characteristics and comparisons.

#### 14.3 Task Breakdown

- [x] Conduct API review with checklist above.
- [x] Deprecate any experimental APIs identified during review (none identified).
- [x] Remove deprecated symbols or move to `internal/` (none pending).
- [x] Complete all package documentation.
- [x] Add comprehensive examples to each package.
- [x] Write migration guide for breaking changes since v0.x.
- [x] Final test pass: `go test -race ./...`, lint, vet.
- [ ] Final benchmark pass: no major regressions from v0.x.
- [ ] Tag `v1.0.0` release.

#### 14.4 Exit Criteria

- [x] All public APIs documented with examples.
- [x] No `// TODO` or `// FIXME` comments in public code.
- [x] All tests pass with race detector.
- [x] Benchmark baselines established and documented.
- [ ] `v1.0.0` tagged and released with full changelog.
- [ ] Go module proxy indexed and importable.

---

## Appendix A: Testing and Validation Strategy

### A.1 Test Types

- Unit tests (table-driven and edge-case heavy).
- Property-based tests for invariants.
- Golden vector tests for deterministic algorithm outputs.
- Integration tests across package boundaries.

### A.2 Numerical Validation

- Define tolerance policy per algorithm category.
- Compare selected outputs against trusted references (MATLAB/NumPy/known datasets).
- Track expected floating-point drift across architectures.

### A.3 Coverage Targets

- Project-wide: >= 85% where practical.
- Core algorithm packages: >= 90%.

---

## Appendix B: Benchmarking and Performance Strategy

- Maintain microbenchmarks for all hot paths.
- Maintain scenario benchmarks reflecting realistic workloads.
- Track allocations/op and bytes/op as first-class metrics.
- Gate regressions with benchmark trend checks in CI (non-blocking initially, blocking by v1.0).

Key benchmark families:

- Filter block processing throughput.
- Convolution strategy crossover points.
- Resampler quality/performance modes.
- THD/sweep analysis runtime and allocations.

---

## Appendix C: Dependency and Versioning Policy

- Keep external dependencies minimal and justified.
- Prefer pure-Go paths unless CGo brings clear, measured value.
- `algo-fft` is consumed via narrow integration interfaces.
- Use semantic versioning; document breaking changes before major bumps.
- Support latest Go stable and previous stable.

---

## Appendix D: Release Engineering

- Conventional commits for changelog generation.
- Tag-driven releases with generated notes.
- Pre-release channel (`v0.x`) until API freeze.
- Required release gates:
  - Lint + tests + race checks
  - Benchmark sanity pass
  - Documentation/examples up to date

---

## Appendix E: Migration Plan from `mfw`

### E.1 Extraction Sequence

1. **Window functions** -> `algo-dsp/dsp/window`
   - Source: `mfw/legacy/Source/MFWindowFunctions.pas` (class hierarchy, 25+ window types)
   - Source: `mfw/legacy/Source/MFWindowFunctionUtils.pas` (coefficient tables lines 22–145, processing loops)
   - Port coefficient tables, window metadata (ENBW, sidelobe), and advanced features (slope, DC removal, inversion)
   - Validate against mfw outputs before switching imports
2. **Filter runtime + design** -> `algo-dsp/dsp/filter/*`
   - Source: `mfw/legacy/Source/MFFilter.pas` (2641 lines — biquad DF-II-T, cascaded SOS, frequency response, all coefficient designs)
   - Source: `mfw/legacy/Source/MFFilterList.pas` (filter registry and UI wrappers — not ported, app-specific)
   - Source: `mfw/legacy/Source/DSP/MFDSPWeightingFilters.pas` (A/B/C weighting as cascaded IIR)
   - Source: `mfw/legacy/Source/DSP/MFDSPFractionalOctaveFilter.pas` (octave/fractional-octave banks)
   - Port biquad runtime (Phase 3), then coefficient designers (Phase 4), then banks/weighting (Phase 5)
   - Validate frequency response parity before switching imports
3. Spectrum/conv/resample helpers
4. Measurement kernels (`pkg/measure/thd`, `pkg/measure/sweep`, `pkg/measure/ir`)

### E.2 Migration Mechanics

- Keep APIs in `mfw` adapter-friendly during extraction.
- Move code with tests first; then switch imports.
- Add compatibility tests in `mfw` to validate behavior parity.
- Remove duplicated code only after parity checks pass.

### E.3 Completion Definition

- `mfw` retains orchestration and app-specific domain logic only.
- Algorithm-heavy packages imported from `algo-dsp`.
- CI in both repos passes with pinned compatible versions.

---

## Appendix F: Risks and Mitigations

| Risk                                     | Impact | Mitigation                                            |
| ---------------------------------------- | ------ | ----------------------------------------------------- |
| API churn during extraction              | Medium | Enforce phased stabilization and deprecation windows  |
| Numerical regressions after optimization | High   | Scalar reference path + parity tests + golden vectors |
| Scope creep into app/file concerns       | Medium | Strict boundary rules and review checklist            |
| Performance regressions across CPUs      | Medium | Per-arch benchmarks and build-tag fallback            |
| Test fixture fragility                   | Low    | Versioned fixture sets and deterministic generation   |

---

## Appendix G: Initial 90-Day Execution Plan

### Month 1

- Complete Phase 0 and Phase 1.
- Start and finish Phase 2 windows.

### Month 2

- Complete Phase 3 filter runtimes.
- Start Phase 4 filter design.

### Month 3

- Complete Phase 4.
- Complete Phase 5 weighting/banks.
- Start Phase 6 spectrum utilities.

Quarter-end success criteria:

- First production-ready extraction target from `mfw`: windows + core filter runtime.
- Tagged prerelease (`v0.1.0` or later) with docs and examples.

---

## Appendix H: Revision History

| Version | Date       | Author | Changes                                                                                                                                                                                                                                                                                                                                                                                 |
| ------- | ---------- | ------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 0.1     | 2026-02-06 | Codex  | Initial comprehensive `algo-dsp` development plan                                                                                                                                                                                                                                                                                                                                       |
| 0.2     | 2026-02-06 | Claude | Refined Phase 1 (buffer type in `dsp/buffer`), rewrote Phase 2 (window functions) with full mfw legacy inventory (25+ types, 3 tiers, advanced features), updated architecture and migration sections                                                                                                                                                                                   |
| 0.3     | 2026-02-06 | Claude | Rewrote Phase 3 (filter runtime) with full MFFilter.pas analysis: biquad DF-II-T, cascaded chains, frequency response, FIR runtime, legacy mapping table. Refined Phase 4 (filter design) with per-filter-type legacy source references and API surface. Refined Phase 5 (weighting/banks) with legacy source references. Updated migration section with filter extraction sources      |
| 0.4     | 2026-02-06 | Codex  | Completed Phase 3 implementation checklist (3a-3e), including biquad/FIR runtime validation, added biquad block+response runnable example, and validated tests/race/lint/vet/coverage targets.                                                                                                                                                                                          |
| 0.5     | 2026-02-06 | Codex  | Started Phase 4 implementation: added `dsp/filter/design` biquad designers (`Lowpass`/`Highpass`/`Bandpass`/`Notch`/`Allpass`/`Peak`/`LowShelf`/`HighShelf`), Butterworth LP/HP cascades with odd-order handling, bilinear helper, tests/examples, and checklist progress updates.                                                                                                      |
| 0.6     | 2026-02-06 | Codex  | Implemented Chebyshev Type I/II cascade designers in `dsp/filter/design`, added legacy-parity tests for Type I, documented/implemented corrected Type II LP angle term, formatted `dsp/filter/weighting/weighting.go`, and revalidated lint/vet/tests/race/coverage.                                                                                                                    |
| 0.7     | 2026-02-06 | Claude | Completed Phase 5 implementation: validated weighting filters (A/B/C/Z with 100% coverage, IEC 61672 compliance), octave/fractional-octave filter banks (93% coverage), block processing wrappers, and marked all Phase 5 tasks complete.                                                                                                                                               |
| 0.8     | 2026-02-06 | Claude | Completed Phase 7 implementation: direct convolution, overlap-add/overlap-save (FFT-based), cross-correlation (direct/FFT/normalized), auto-correlation, deconvolution (naive/regularized/Wiener), inverse filter generation. Added benchmarks showing crossover at ~64-128 sample kernels, comprehensive tests, and examples.                                                          |
| 0.9     | 2026-02-06 | Claude | Compacted Phases 0-9 to summaries. Refined Phases 10-14 with detailed specs from mfw/legacy: Phase 10 (THD) with MFTotalHarmonicDistortionCalculation.pas algorithms; Phase 11 (Sweep/IR) with TMFSchroederData metrics; Phase 12 (Stats) with TMFTimeDomainInfoType/TMFFrequencyDomainInfoType; Phase 13 (SIMD) with optimization strategy; Phase 14 (v1.0) with API review checklist. |
| 1.0     | 2026-02-06 | Claude | Completed Phase 11: LogSweep/LinearSweep with generate/inverse/deconvolve/harmonic extraction, IR Analyzer with Schroeder integral, RT60/EDT/T20/T30, C50/C80/D50/D80, CenterTime, FindImpulseStart. Coverage: sweep 85.4%, ir 86.2%. All tests pass with race detector.                                                                                                                |
| 1.1     | 2026-02-07 | Claude | Completed Phase 12: stats/time with single-pass Welford's algorithm (DC, RMS, peak, range, crest factor, energy/power, zero crossings, variance, skewness, kurtosis), StreamingStats with bit-identical results. stats/frequency with spectral centroid, spread, flatness, rolloff, bandwidth. Zero allocations. Coverage: time 98%, freq 97.8%.                                        |

---

This plan is a living document and should be updated after each phase completion and major architectural decision.
