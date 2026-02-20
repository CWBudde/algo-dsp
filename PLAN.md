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
6. Detailed Phase Plan (Phases 0–16)

7. Appendices

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
Phase 0:  Bootstrap & Governance                     [1 week]   ✅ Complete
Phase 1:  Numeric Foundations & Core Utilities       [2 weeks]  ✅ Complete
Phase 2:  Window Functions                            [2 weeks]  ✅ Complete
Phase 3:  Filter Runtime Primitives                   [3 weeks]  ✅ Complete
Phase 4:  Filter Design Toolkit                       [3 weeks]  ✅ Complete
Phase 5:  Filter Banks and Weighting                  [2 weeks]  ✅ Complete
Phase 6:  Spectrum Utilities                          [2 weeks]  ✅ Complete
Phase 7:  Convolution and Correlation                 [2 weeks]  ✅ Complete
Phase 8:  Resampling                                  [3 weeks]  ✅ Complete
Phase 9:  Signal Generation and Utilities             [2 weeks]  ✅ Complete
Phase 10: Measurement Kernels (THD)                   [3 weeks]  ✅ Complete
Phase 11: Measurement Kernels (Sweep/IR)              [3 weeks]  ✅ Complete
Phase 12: Stats Packages                              [2 weeks]  ✅ Complete
Phase 13: Optimization and SIMD Paths                 [3 weeks]  🔄 In Progress
Phase 14: API Stabilization and v1.0                  [2 weeks]  🔄 In Progress
Phase 15: Advanced Parametric EQ Design               [2 weeks]  ✅ Complete
Phase 16: High-Order Graphic EQ Bands                 [4 weeks]  ✅ Complete
Phase 17: High-Order Shelving Filters                  [2 weeks]  🔄 In Progress
Phase 18: Effects — High-Priority Modulation          [2 weeks]  📋 Planned
Phase 19: Effects — High-Priority Dynamics            [2 weeks]  📋 Planned
Phase 20: Effects — High-Priority Spatial             [1 week]   📋 Planned
Phase 21: Effects — Medium-Priority Waveshaping/Lo-fi [2 weeks]  📋 Planned
Phase 22: Effects — Medium-Priority Modulation        [2 weeks]  📋 Planned
Phase 23: Effects — Medium-Priority Dynamics          [2 weeks]  📋 Planned
Phase 24: Effects — Spatial and Convolution Reverb    [2 weeks]  📋 Planned
Phase 25: Effects — Specialized / Lower-Priority      [4 weeks]  📋 Planned

Total Estimated Duration: ~57 weeks
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

### Phase 13: Optimization and SIMD Paths (In Progress)

Status:

- The core optimization work (SIMD kernels, allocation reduction, and hot-path tuning) is implemented.
- Remaining work is focused on closing the last allocation/perf gaps and making performance regression detection repeatable.

#### 13.1 Remaining TODOs (must-do)

- [ ] Add a benchmark regression guard (non-blocking at first) that flags large performance regressions compared to baselines in `BENCHMARKS.md`.
  - [x] Choose a small, stable benchmark subset that covers the hottest paths.
  - [ ] Define a regression threshold policy (e.g. ns/op and allocs/op) and document how to update baselines.
  - [x] Add a CI-friendly target (e.g. `just bench-ci`) that runs quickly and emits a machine-readable report.
  - [ ] Wire it into CI as advisory output (make it blocking only after v1.0 if desired).
- [x] Remove remaining allocation overhead in spectrum helpers caused by unpacking `[]complex128` into temporary buffers.
  - [x] Add/extend a zero-allocation fast path that operates on separate real/imag slices (or reuses pooled scratch buffers).
  - [x] Wire `dsp/spectrum` to prefer the fast path when inputs allow it.
  - [x] Record before/after numbers in `BENCHMARKS.md`.
- [ ] Re-run the full benchmark suite on at least two representative machines (amd64 AVX2-capable + arm64 NEON) and update `BENCHMARKS.md` (date + Go version).

#### 13.2 Legacy ASM → Go assembly ports

Moved to [`algo-vecmath/PLAN.md`](../algo-vecmath/PLAN.md) §3.

#### 13.3 Exit Criteria

- [ ] No major regressions in allocations/op on the key hot paths.
- [ ] `go test ./...` and `go test -tags purego ./...` pass.
- [ ] `BENCHMARKS.md` updated with current baselines and notable changes.

#### 13.4 Modal oscillator SIMD track (algo-piano dependency)

Goal: add a reusable, SIMD-friendly modal/quadrature oscillator bank in `algo-dsp` to accelerate modal string synthesis workloads (first consumer: `algo-piano`).

- [ ] Add a new high-level package (candidate: `dsp/osc` or `dsp/modal`) with block APIs for damped complex rotators.
  - [ ] `ProcessDampedComplexBlock32(...)` as the primary path (`float32` hot path for realtime synth use).
  - [ ] Optional `ProcessDampedComplexBlock64(...)` for analysis/offline parity.
  - [ ] API should update `(re, im)` state and optionally accumulate output with per-mode gains.
- [ ] Keep a scalar reference implementation as canonical behavior for correctness and fallback.
- [ ] Integrate architecture-dispatched acceleration via `algo-vecmath` kernels where beneficial.
- [ ] Add microbenchmarks that match modal-bank workloads (e.g. 8-32 modes, 1-3 strings, block size 128).
- [ ] Add parity tests vs scalar reference across damped/undamped transitions, varied frequencies, and long-tail decay behavior.
- [ ] Document denormal strategy and verify behavior with long release tails.
- [ ] Add user-facing docs/example showing the oscillator-bank loop in a modal synth context.

Dependencies and sequencing:

- [ ] First land required low-level kernels in `../algo-vecmath` (see `../algo-vecmath/PLAN.md` section 5).
- [ ] Then wire the high-level DSP API in `algo-dsp`.
- [ ] After release, consume in `algo-piano` modal path and compare CPU/alloc metrics against current scalar implementation.

#### 13.5 Extended Exit Criteria

- [ ] Oscillator-bank API exists with scalar fallback and deterministic tests.
- [ ] At least one architecture-accelerated backend shows measurable speedup on representative modal-bank benchmarks.
- [ ] Benchmarks include modal-bank scenarios and are tracked in `BENCHMARKS.md`.

#### 13.6 Concrete issue backlog (modal/SIMD)

These are implementation-ready tickets for the modal/quadrature SIMD track.

- [ ] `DSP-201` — Add `dsp/osc` package skeleton and scalar reference kernels.
  - Scope: package layout, public API stubs, scalar `float32` reference implementation.
  - Acceptance: deterministic unit tests for reference kernels; public docs with usage example.
  - Depends on: none.
- [ ] `DSP-202` — Add block API for damped complex bank update (`float32`).
  - Scope: `ProcessDampedComplexBlock32(...)` API + in-place state update semantics.
  - Acceptance: parity tests vs scalar reference for random vectors and fixed vectors.
  - Depends on: `DSP-201`.
- [ ] `DSP-203` — Add optional `float64` API parity path.
  - Scope: `ProcessDampedComplexBlock64(...)` for offline/analysis use.
  - Acceptance: API docs + cross-precision behavior tests + no regressions in existing benches.
  - Depends on: `DSP-201`.
- [ ] `DSP-204` — Integrate vecmath-dispatched acceleration in hot loop.
  - Scope: fast path using `algo-vecmath` kernels where profitable.
  - Acceptance: measurable speedup on at least one target CPU vs scalar baseline.
  - Depends on: `VEC-301`, `VEC-302`, `DSP-202`.
- [ ] `DSP-205` — Modal-bank benchmark suite.
  - Scope: dedicated benchmarks for 8/16/24/32 modes, block 128/256, 1-3 strings.
  - Acceptance: benchmark outputs recorded in `BENCHMARKS.md` with date + Go version + machine info.
  - Depends on: `DSP-202`.
- [ ] `DSP-206` — Numerical/stability hardening for long tails.
  - Scope: denormal policy docs + long-release stress tests (NaN/Inf/denormal resistance).
  - Acceptance: stress suite passes under default and `-tags=purego` builds.
  - Depends on: `DSP-202`.
- [ ] `DSP-207` — Public integration example for modal synthesis.
  - Scope: runnable example showing oscillator-bank update and summed output render loop.
  - Acceptance: `go test ./...` executes example tests cleanly and docs link from README/package docs.
  - Depends on: `DSP-202`.

Tracking note:

- `algo-piano` integration ticket group is tracked in `algo-piano/PLAN.md` Phase 12.5.

### Phase 14: API Stabilization and v1.0 (In Progress)

Remaining TODOs:

- [ ] Final benchmark pass (`just bench`) and confirm no major regressions vs `BENCHMARKS.md`.
- [ ] Final CI pass locally (`just ci`) including race (`go test -race ./...`).
- [ ] Confirm `CHANGELOG.md` and `MIGRATION.md` are complete for `v1.0.0`.
- [ ] Tag and publish `v1.0.0` (git tag + release notes).
- [ ] Verify Go module proxy indexing (fresh `go get` / import works via `GOPROXY`).

### Phase 15: Advanced Parametric EQ Design (Orfanidis) (Complete - 2026-02-07)

Goal: Add a higher-fidelity peaking EQ designer (Orfanidis “prescribed Nyquist gain / decramped” family) and a pragmatic higher-order PEQ path via cascades, without changing runtime processing code.

Rationale / fit with repo:

- This is a coefficient _designer_, so it belongs under `dsp/filter/design` rather than under `dsp/filter/biquad` (which is runtime + response).
- It complements the existing RBJ-style `design.Peak(...)` by adding an alternate formulation with explicit DC/Nyquist constraints.
- Higher-order behavior is implemented by returning `[]biquad.Coefficients` and feeding it into existing `biquad.NewChain(...)`.

#### 15.1 Deliverables (implemented)

- Implemented package: `dsp/filter/design/orfanidis`
  - Public API (expert):
    - `func Peaking(G0, G1, G, GB, w0, dw float64) (biquad.Coefficients, error)`
  - Public API (audio-friendly):
    - `func PeakingFromFreqQGain(sampleRate, f0Hz, Q, gainDB float64) (biquad.Coefficients, error)`
  - Higher-order cascade helper:
    - `func PeakingCascade(sampleRate, f0Hz, Q, gainDB float64, sections int) ([]biquad.Coefficients, error)`

Implementation notes:

- Validate inputs aggressively (NaN/Inf, sample rate/frequency bounds, gain constraints) and return typed errors (e.g. `ErrInvalidParams`).
- Keep the package dependency graph minimal: `math`, `errors`, and `dsp/filter/biquad` only.

#### 15.2 Validation and tests

- [x] Unit tests for parameter validation and edge cases:
  - [x] Invalid values (non-positive gains, `w0` not in (0, π), `dw` not in (0, π), `sections <= 0`, `f0 >= Fs/2`, etc.).
  - [x] Typical audio settings across sample rates (44.1k/48k/96k/192k).
- [x] Response sanity tests using existing `biquad.Response` helpers:
  - [x] Check approximate peak behavior at `f0` (magnitude near requested gain within tolerance).
  - [x] Check stability (poles inside unit circle) for representative and "stress" settings.
  - [x] For the convenience wrapper policy (`G0=G1=1`), verify DC and Nyquist magnitude are near unity.
- [x] Cascade behavior:
  - [x] Verify N-section cascade magnitude at `f0` matches (approximately) the target total gain.

#### 15.3 Documentation and examples

- [x] Package docs clarifying:
  - [x] Difference vs `design.Peak(...)` (RBJ) and why Orfanidis is offered.
  - [x] Meaning of `G0/G1/G/GB` and `w0/dw` for the expert API.
  - [x] The default Nyquist policy (`G1=1`) and when a caller should use the expert API instead.
- [x] Runnable example showing cascade -> `biquad.NewChain`.

#### 15.4 Exit criteria

- [x] `go test ./...` and `go test -race ./...` pass.
- [x] New package has runnable examples and doc comments on public identifiers.
- [x] Numerical validation: response checks pass across at least 2 sample rates.
- [x] No new allocations in biquad runtime paths (designer-only code may allocate where unavoidable).

### Phase 16: High-Order Graphic EQ Bands (Orfanidis-style) (Complete - 2026-02-07)

Goal: Implement gain-adjustable, high-order band filters suitable for graphic EQ bands (fixed center frequencies, per-band gain changes), using Orfanidis-style formulations. Support Butterworth, Chebyshev Type I, Chebyshev Type II, and Elliptic topologies.

This phase is explicitly **not** about UI/stateful application wiring. It provides algorithmic designers that return SOS (`[]biquad.Coefficients`) consumable by `dsp/filter/biquad.Chain`.

Rationale / fit with repo:

- `dsp/filter/design` already holds coefficient designers; this work belongs there.
- `dsp/filter/bank` already defines fractional-octave grids; we can reuse its band edge computation as a frequency _spec provider_.
- Runtime processing stays in `dsp/filter/biquad` (no new processing kernels required).

#### 16.1 Scope and terminology

- A **band** is defined by center frequency `f0` and bandwidth `fb` (Hz) or equivalently by edge frequencies `fl`, `fh`.
- A **band filter** is a gain-adjustable, high-order IIR that boosts/cuts primarily within the band while remaining near-unity outside (as used in classic graphic EQ designs).
- Designers return cascaded second-order sections (SOS) as `[]biquad.Coefficients`.

#### 16.2 Package layout (implemented)

- [x] `dsp/filter/design/band`
  - Contains Orfanidis-style high-order _band_ designers and helpers.
  - Depends only on `math`, `errors`, `dsp/filter/biquad`, and internal helpers.

Design note: keep "grid/band spec" types separate from coefficient design so callers can use IEC 61260 grids (`bank`) or custom grids.

#### 16.3 APIs: expert vs audio-friendly

Expose two layers, similar to Phase 15:

- **Expert API** (digital specs): takes rad/sample quantities and explicit gain constraints.
  - Inputs mirror common Orfanidis-style formulations:
    - `w0` center rad/sample
    - `wb` bandwidth rad/sample
    - `G0` baseline gain (typically unity)
    - `G` gain at band center
    - `Gb` gain at band edges (bandwidth definition)
    - plus topology-specific ripple/attenuation parameters where required.

- **Audio-friendly API**: takes `sampleRate`, `f0Hz`, `bandwidthHz` (or `fl/fh`), and `gainDB`.
  - Implements a _policy_ that chooses a default `Gb` from `gainDB` (band-edge convention).
  - Keeps the API practical for “graphic EQ band knob” use.

#### 16.4 Topology deliverables (complete)

For each topology, implement:

- [x] Butterworth band designer
- [x] Chebyshev Type I band designer
- [x] Chebyshev Type II band designer
- [x] Elliptic band designer

For each designer, provide:

- [x] A coefficient generator returning `[]biquad.Coefficients` with deterministic section ordering.
- [x] A small helper for the default **band-edge gain policy** (the "Gb choice"), so audio-friendly wrappers are consistent and testable.

API sketch (names can be refined during implementation review):

- `func ButterworthBand(sampleRate, f0Hz, bandwidthHz, gainDB float64, order int) ([]biquad.Coefficients, error)`
- `func Chebyshev1Band(sampleRate, f0Hz, bandwidthHz, gainDB float64, order int, rippleDB float64) ([]biquad.Coefficients, error)`
- `func Chebyshev2Band(sampleRate, f0Hz, bandwidthHz, gainDB float64, order int, stopbandRippleDB float64) ([]biquad.Coefficients, error)`
- `func EllipticBand(sampleRate, f0Hz, bandwidthHz, gainDB float64, order int, passRippleDB, stopAttenDB float64) ([]biquad.Coefficients, error)`

Constraints:

- Enforce `order >= 4` and even order for these high-order band designs.
- Validate frequency bounds: `0 < fl < f0 < fh < Fs/2`.
- Ensure stable poles (inside unit circle) for all sections.

#### 16.5 Optional: precomputation/caching helpers (deferred)

The C++ example precomputes a filter per gain step. In Go, keep this optional and transport-agnostic:

- [ ] Provide a small helper that precomputes `map[int][]biquad.Coefficients` for integer dB steps.
- [ ] Keep it purely in-memory and avoid adding any file I/O.

Note: Deferred as not critical for initial implementation. Coefficient generation is fast enough for real-time updates.

#### 16.6 Validation and tests (complete)

- [x] Parameter validation tests for all topologies.
- [x] Stability tests (poles inside unit circle) across representative settings and stress settings (near Nyquist, wide bands, large boosts/cuts).
- [x] Frequency response conformance tests:
  - [x] Center gain close to requested gain (within tolerance).
  - [x] Band-edge magnitude close to the chosen `Gb` policy.
  - [x] Outside-band behavior is near-unity (define a pragmatic tolerance and frequency points).
- [x] Cross-sample-rate tests at 44.1k/48k/96k/192k.

#### 16.7 Documentation and examples (complete)

- [x] Package docs clarifying:
  - [x] What these "band filters" are (graphic EQ building blocks) vs Phase 15 parametric peaking EQ.
  - [x] How bandwidth is defined and how `Gb` is chosen.
  - [x] Which parameters should be tuned per topology (ripple/attenuation for Chebyshev/Elliptic).
- [x] Runnable example:
  - Build a 10-band or 1/3-octave grid and generate a full EQ chain by cascading band filters.
  - Show updating gains by regenerating coefficients (no stateful UI logic).

#### 16.8 Exit criteria

- [x] `go test ./...` and `go test -race ./...` pass.
- [x] All new public identifiers have doc comments and runnable examples.
- [x] Deterministic outputs for deterministic inputs (tests lock this down).
- [x] No changes to biquad runtime APIs required.

### Phase 17: High-Order Shelving Filters (Holters/Zölzer + Orfanidis) (In Progress)

Goal: Implement high-order shelving filter designers (low-shelf and high-shelf) supporting
Butterworth, Chebyshev Type I, Chebyshev Type II, and Elliptic topologies.
Returns cascaded second-order sections (`[]biquad.Coefficients`).

Rationale / fit with repo:

- Shelving filters are a fundamental EQ building block alongside the band filters from Phase 16.
- The package `dsp/filter/design/shelving` provides coefficient designers that complement the
  existing `design.LowShelf` / `design.HighShelf` (RBJ-style 2nd order) with higher-order variants.
- Uses the Holters & Zölzer decomposition (Section 2.1 of "Parametric Higher-Order Shelving Filters")
  for Butterworth and Chebyshev I, and the Orfanidis framework for Chebyshev II.

#### 17.1 Package: `dsp/filter/design/shelving`

**Data types (internal):**

- `poleParams{sigma, r2}` — analog prototype pole parameters per conjugate pair.
- `sosParams{den, num poleParams}` — independent numerator/denominator analog parameters for a single SOS.
- `fosParams{denSigma, numSigma}` — first-order section parameters (odd-order real pole).

**Core building blocks (internal):**

- `bilinearSOS(K, sosParams)` — bilinear transform from independent num/den analog parameters to digital biquad.
- `bilinearFOS(K, fosParams)` — bilinear transform for first-order section.
- `lowShelfSOS(K, P, poleParams)` — SOS where num = P·den scaling (Butterworth, Chebyshev I).
- `lowShelfFOS(K, P, sigma)` — FOS with P-scaling.
- `butterworthPoles(M)` — unit-circle pole placement for Butterworth.
- `chebyshev1Poles(M, rippleDB)` — elliptical pole placement for Chebyshev I.
- `lowShelfSections(K, P, pairs, realSigma)` — assembles cascade from pole parameters.
- `negateOddPowers(sections)` — converts low-shelf to high-shelf via H_HS(z) = H_LS(−z).

#### 17.2 Public API

```go
// Butterworth shelving (Holters & Zölzer)
func ButterworthLowShelf(sampleRate, freqHz, gainDB float64, order int) ([]biquad.Coefficients, error)
func ButterworthHighShelf(sampleRate, freqHz, gainDB float64, order int) ([]biquad.Coefficients, error)

// Chebyshev Type I shelving (Holters & Zölzer)
func Chebyshev1LowShelf(sampleRate, freqHz, gainDB, rippleDB float64, order int) ([]biquad.Coefficients, error)
func Chebyshev1HighShelf(sampleRate, freqHz, gainDB, rippleDB float64, order int) ([]biquad.Coefficients, error)

// Chebyshev Type II shelving (Orfanidis framework)
func Chebyshev2LowShelf(sampleRate, freqHz, gainDB, rippleDB float64, order int) ([]biquad.Coefficients, error)
func Chebyshev2HighShelf(sampleRate, freqHz, gainDB, rippleDB float64, order int) ([]biquad.Coefficients, error)
```

Key design:

- `order >= 1` (unlike band filters which require even order >= 4).
  Odd orders produce an additional first-order section.
- `rippleDB > 0` for Chebyshev types (controls transition ripple for Type I, stopband ripple for Type II).
- `gainDB == 0` returns a single passthrough section.
- Low-shelf uses `K = tan(π·f/fs)`, high-shelf uses `K = 1/tan(π·f/fs)` plus `negateOddPowers`.

#### 17.3 Implementation status

**Butterworth (Complete):**

- [x] `butterworthPoles` — unit-circle pole placement.
- [x] `ButterworthLowShelf` / `ButterworthHighShelf` — validated across orders 1–12, gains ±30 dB.
- [x] Tests: parameter validation, zero gain, section count, DC/Nyquist gain accuracy,
      cutoff gain (Eq. 5: |H|² = (g²+1)/2), pole stability, boost/cut inversion,
      order sweep, frequency sweep, extreme gains, monotonicity, paper design example.

**Chebyshev Type I (Complete):**

- [x] `chebyshev1Poles` — elliptical pole placement with ripple parameter.
- [x] `Chebyshev1LowShelf` / `Chebyshev1HighShelf` — validated across orders 1–12.
- [x] Tests: parameter validation, zero gain, section count, DC/Nyquist accuracy,
      pole stability, order sweep, steeper-than-Butterworth comparison, extreme gains,
      various ripple values (0.1–3.0 dB), frequency sweep.

**Chebyshev Type II (In Progress — filter shape bug):**

- [x] `chebyshev2Sections` — Orfanidis A/B parameter computation and bilinear transform.
- [x] `Chebyshev2LowShelf` / `Chebyshev2HighShelf` — API implemented, compiles, runs.
- [x] DC gain correction — post-hoc scaling of first section's numerator to achieve target DC gain.
- [x] Tests written (22 test functions covering all categories, 51 passing, 8 failing).
- [ ] **BUG: filter does not produce a proper shelf shape.**

#### 17.4 TODO: Fix Chebyshev Type II shelving filter shape

**Problem:**

The current `chebyshev2Sections` implementation produces nearly flat gain across all frequencies
instead of transitioning from shelf gain (at DC for low-shelf) to ~0 dB (at Nyquist for low-shelf).
The DC gain correction makes DC correct, but the Nyquist gain is approximately `gainDB − rippleDB`
(e.g. +11.5 dB for a +12 dB shelf with 0.5 dB ripple) instead of ~0 dB.

Example: `Chebyshev2LowShelf(48000, 1000, 12, 0.5, 4)` produces:

```plain
    1 Hz: +12.00 dB    (correct — shelf gain)
  500 Hz: +11.90 dB    (wrong — should be transitioning)
 1000 Hz: +12.00 dB    (wrong — should be near cutoff gain)
 5000 Hz: +11.56 dB    (wrong — should be near 0 dB)
23999 Hz: +11.90 dB    (wrong — should be ~0 dB)
```

_Root cause analysis:_

The Orfanidis A/B parameters were adapted directly from the band EQ case (`chebyshev2BandRad`
in `band.go`). In the band EQ case, these parameters work correctly because the bandpass bilinear
transform `s → (z² − 2·cos(w0)·z + 1) / (z² − 1)` embeds frequency warping that creates the
correct shape. For the shelving case, we use a direct lowpass bilinear transform `s → (z−1)/(z+1)·(1/K)`
which maps the analog frequency axis differently.

The key insight is that the Orfanidis Chebyshev II formulation for _band_ EQ places zeros on the
imaginary axis to create notches at specific frequencies, and the bandpass transform maps these
notches to the correct digital frequencies. When the same A/B parameters are used with a direct
lowpass bilinear transform, the zeros end up at wrong frequencies and don't create the expected
shelf-to-flat transition.

_Possible approaches:_

1. _Derive proper Chebyshev II poles and zeros for the lowpass prototype_ — analogous to how
   `chebyshev1Poles` computes poles on an ellipse, compute Chebyshev II poles _and_ transmission
   zeros for the lowpass prototype. The key difference from Chebyshev I is that Type II has zeros
   at `s = ±j/cos(θ_m)` in the analog prototype. These zeros need to be placed so they map to
   the correct digital frequencies under the lowpass bilinear transform.

2. _Use the Holters/Zölzer approach for Chebyshev II_ — the paper's decomposition works for
   any all-pole prototype. Chebyshev II is _not_ all-pole (it has finite transmission zeros),
   so the simple P-scaling (`σ_n = P·σ_d`) doesn't apply. However, the `sosParams` infrastructure
   already supports independent numerator/denominator parameters — the challenge is computing
   the correct analog prototype zeros.

3. _Reference: AES paper on shelving filters_ — see `dsp/filter/design/AESShelving.pdf` for
   the Orfanidis shelving filter formulation that may contain the proper Chebyshev II lowpass
   prototype derivation.

**Failing tests (8 of 22 Chebyshev II tests, all trace to the same root cause):**

- `TestChebyshev2LowShelf_NyquistGain` — Nyquist ≈ +11.9 dB instead of ~0 dB
- `TestChebyshev2HighShelf_DCGain` — DC ≈ +11.9 dB instead of ~0 dB
- `TestChebyshev2LowShelf_VariousOrders` — Nyquist check fails for all orders
- `TestChebyshev2LowShelf_StopbandRipple` — stopband is at shelf gain (~11.5 dB), not ~0 dB
- `TestChebyshev2HighShelf_StopbandRipple` — stopband is at shelf gain, not ~0 dB
- `TestChebyshev2LowShelf_VariousRipple` — Nyquist check fails for all ripple values (0.1-3.0 dB)
- `TestChebyshev2LowShelf_MonotonicShelfRegion` — non-monotonic at 380 Hz due to wrong shape
- `TestChebyshev2_FlatStopband` — stopband deviation = 11.5 dB (exceeds 0.5 dB ripple bound)

**Tests passing (51 of 59 total including Butterworth and Chebyshev I):**

- Butterworth (all tests pass): parameter validation, zero gain, section count, DC/Nyquist gain accuracy,
  cutoff gain, pole stability, boost/cut inversion, order sweep (1-12), frequency sweep, extreme gains,
  monotonicity, paper design example.
- Chebyshev Type I (all tests pass): parameter validation, zero gain, section count, DC/Nyquist accuracy,
  pole stability, order sweep, steeper-than-Butterworth comparison, extreme gains, various ripple values,
  frequency sweep.
- Chebyshev Type II (14 of 22 pass): parameter validation, zero gain, section count, DC gain (low-shelf),
  Nyquist gain (high-shelf), pole stability, extreme gains, frequency sweep (DC-only), boost/cut inversion.

#### 17.5 Future: Elliptic shelving filters

- [ ] Implement elliptic shelving filters once Chebyshev II is working.
      The elliptic case also has transmission zeros, so it will face similar challenges
      to Chebyshev II. The elliptic function machinery already exists in `band/elliptic.go`.

#### 17.6 Exit criteria

- [ ] All shelving filter topologies produce correct shelf shape.
- [ ] All tests pass (currently 51/59 total: Butterworth ✓, Chebyshev I ✓, Chebyshev II 14/22).
- [x] `go test ./dsp/filter/design/shelving/ -race` passes (for implemented Butterworth/Chebyshev I).
- [x] Doc comments on all public functions.

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

| Version | Date       | Author  | Changes                                                                                                                                                                                                                                                                                                                                                                                 |
| ------- | ---------- | ------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 0.1     | 2026-02-06 | Codex   | Initial comprehensive `algo-dsp` development plan                                                                                                                                                                                                                                                                                                                                       |
| 0.2     | 2026-02-06 | Claude  | Refined Phase 1 (buffer type in `dsp/buffer`), rewrote Phase 2 (window functions) with full mfw legacy inventory (25+ types, 3 tiers, advanced features), updated architecture and migration sections                                                                                                                                                                                   |
| 0.3     | 2026-02-06 | Claude  | Rewrote Phase 3 (filter runtime) with full MFFilter.pas analysis: biquad DF-II-T, cascaded chains, frequency response, FIR runtime, legacy mapping table. Refined Phase 4 (filter design) with per-filter-type legacy source references and API surface. Refined Phase 5 (weighting/banks) with legacy source references. Updated migration section with filter extraction sources      |
| 0.4     | 2026-02-06 | Codex   | Completed Phase 3 implementation checklist (3a-3e), including biquad/FIR runtime validation, added biquad block+response runnable example, and validated tests/race/lint/vet/coverage targets.                                                                                                                                                                                          |
| 0.5     | 2026-02-06 | Codex   | Started Phase 4 implementation: added `dsp/filter/design` biquad designers (`Lowpass`/`Highpass`/`Bandpass`/`Notch`/`Allpass`/`Peak`/`LowShelf`/`HighShelf`), Butterworth LP/HP cascades with odd-order handling, bilinear helper, tests/examples, and checklist progress updates.                                                                                                      |
| 0.6     | 2026-02-06 | Codex   | Implemented Chebyshev Type I/II cascade designers in `dsp/filter/design`, added legacy-parity tests for Type I, documented/implemented corrected Type II LP angle term, formatted `dsp/filter/weighting/weighting.go`, and revalidated lint/vet/tests/race/coverage.                                                                                                                    |
| 0.7     | 2026-02-06 | Claude  | Completed Phase 5 implementation: validated weighting filters (A/B/C/Z with 100% coverage, IEC 61672 compliance), octave/fractional-octave filter banks (93% coverage), block processing wrappers, and marked all Phase 5 tasks complete.                                                                                                                                               |
| 0.8     | 2026-02-06 | Claude  | Completed Phase 7 implementation: direct convolution, overlap-add/overlap-save (FFT-based), cross-correlation (direct/FFT/normalized), auto-correlation, deconvolution (naive/regularized/Wiener), inverse filter generation. Added benchmarks showing crossover at ~64-128 sample kernels, comprehensive tests, and examples.                                                          |
| 0.9     | 2026-02-06 | Claude  | Compacted Phases 0-9 to summaries. Refined Phases 10-14 with detailed specs from mfw/legacy: Phase 10 (THD) with MFTotalHarmonicDistortionCalculation.pas algorithms; Phase 11 (Sweep/IR) with TMFSchroederData metrics; Phase 12 (Stats) with TMFTimeDomainInfoType/TMFFrequencyDomainInfoType; Phase 13 (SIMD) with optimization strategy; Phase 14 (v1.0) with API review checklist. |
| 1.0     | 2026-02-06 | Claude  | Completed Phase 11: LogSweep/LinearSweep with generate/inverse/deconvolve/harmonic extraction, IR Analyzer with Schroeder integral, RT60/EDT/T20/T30, C50/C80/D50/D80, CenterTime, FindImpulseStart. Coverage: sweep 85.4%, ir 86.2%. All tests pass with race detector.                                                                                                                |
| 1.1     | 2026-02-07 | Claude  | Completed Phase 12: stats/time with single-pass Welford's algorithm (DC, RMS, peak, range, crest factor, energy/power, zero crossings, variance, skewness, kurtosis), StreamingStats with bit-identical results. stats/frequency with spectral centroid, spread, flatness, rolloff, bandwidth. Zero allocations. Coverage: time 98%, freq 97.8%.                                        |
| 1.2     | 2026-02-07 | Copilot | Added Phase 15 plan for Orfanidis peaking EQ coefficient design and pragmatic higher-order PEQ via cascaded sections under `dsp/filter/design/orfanidis`.                                                                                                                                                                                                                               |
| 1.3     | 2026-02-07 | Copilot | Added Phase 16 plan for Orfanidis-style high-order graphic EQ band filters (Butterworth, Chebyshev I/II, Elliptic) with SOS outputs and validation strategy.                                                                                                                                                                                                                            |
| 1.4     | 2026-02-08 | Claude  | Added Phase 17: high-order shelving filters. Butterworth and Chebyshev I complete. Chebyshev II API + tests written (22 tests, 14 passing) with documented bug: Orfanidis band EQ A/B params don't produce correct shelf shape under direct lowpass bilinear transform. Root cause analysis and three possible fix approaches documented.                                               |

---

This plan is a living document and should be updated after each phase completion and major architectural decision.
