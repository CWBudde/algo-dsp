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
6. Detailed Phase Plan
7. Testing and Validation Strategy
8. Benchmarking and Performance Strategy
9. Dependency and Versioning Policy
10. Release Engineering
11. Migration Plan from `mfw`
12. Risks and Mitigations
13. Initial 90-Day Execution Plan
14. Revision History

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

### Phase 0: Bootstrap & Governance

Objectives:

- Initialize module, lint/test/format pipeline, release automation baseline.
- Define contribution standards and compatibility policy.

Tasks:

- [x] Create module `github.com/cwbudde/algo-dsp`.
- [x] Add `justfile` targets (`test`, `lint`, `format`, `bench`, `ci`).
- [x] Add CI for Go stable + previous stable.
- [x] Define semantic versioning and support window.
- [x] Write `CONTRIBUTING.md` and issue templates.

Exit criteria:

- CI green on all default checks.
- Repo can publish tagged prereleases.

### Phase 1: Numeric Foundations & Core Utilities

Objectives:

- Provide reusable low-level helpers for DSP packages.
- Establish a lightweight buffer type for allocation-friendly processing patterns.

#### 1a. Numeric Helpers

Tasks:

- [x] Add core numeric helpers (clamp, epsilon compare, dB conversions).
- [x] Define shared option/config patterns (functional options base).
- [x] Add deterministic random/test signal helpers in `internal/testutil`.

#### 1b. Buffer Type (`dsp/buffer`)

The legacy `mfw` code passes pointer+size pairs to every processing function. Go slices handle this naturally, but a thin `Buffer` wrapper adds value for real-time hot paths where allocation control matters.

Design:

```go
package buffer

// Buffer wraps a float64 slice with reuse-friendly semantics.
// DSP functions accept raw []float64; Buffer bridges via .Samples().
type Buffer struct { samples []float64 }

func New(length int) *Buffer              // pre-allocate
func FromSlice(s []float64) *Buffer       // wrap existing data (no copy)
func (b *Buffer) Samples() []float64      // underlying slice
func (b *Buffer) Len() int
func (b *Buffer) Grow(n int)              // ensure capacity >= n, keep data
func (b *Buffer) Resize(n int)            // set length to n, reuse capacity
func (b *Buffer) Zero()                   // fill with zeros
func (b *Buffer) Copy() *Buffer           // deep copy

// Pool provides sync.Pool-based buffer reuse for hot paths.
type Pool struct { ... }

func NewPool() *Pool
func (p *Pool) Get(length int) *Buffer
func (p *Pool) Put(b *Buffer)
```

Tasks:

- [x] Implement `dsp/buffer.Buffer` type with `Samples()`, `Resize()`, `Zero()`, `Copy()`.
- [x] Implement `dsp/buffer.Pool` with `sync.Pool`-based reuse.
- [x] Add `ZeroRange(start, end)` helper (mirrors mfw `FillWithZeroes`).
- [x] Ensure all public DSP APIs accept raw `[]float64` — `Buffer` is optional, not required.

Exit criteria:

- Shared helpers adopted by at least two downstream packages.
- Buffer type validated in window function benchmarks (Phase 2).

### Phase 2: Window Functions

Objectives:

- Deliver comprehensive window functions with coefficients, spectral metadata, and advanced features.
- Port the full inventory from `mfw/legacy` (`MFWindowFunctions.pas`, `MFWindowFunctionUtils.pas`).

#### 2a. Architecture

- `Type` enum (iota-based) for all window types.
- Cosine-term windows share a single parametric implementation with coefficient lookup tables.
- Parametric windows (Kaiser, Gauss, Tukey, Lanczos) use functional options.
- `Metadata` struct per type: ENBW, highest sidelobe level, coherent gain, spectrum correction factor.

#### 2b. Window Inventory (from mfw legacy)

**Tier 1 — Essential (implement first):**

| Window             | Family     | Cosine Terms | ENBW (bins) | Sidelobe (dB) |
| ------------------ | ---------- | :----------: | :---------: | :-----------: |
| Rectangular        | Simple     |      —       |    1.000    |     -13.3     |
| Hann               | Cosine     |      1       |    1.441    |     -31.5     |
| Hamming            | Cosine     |      2       |    1.303    |     -42.7     |
| Blackman           | Cosine     |      3       |    1.644    |     -58.1     |
| Blackman-Harris 4T | Cosine     |      4       |    1.899    |     -92.0     |
| FlatTop            | Cosine     |      5       |      —      |       —       |
| Kaiser             | Parametric |      —       |   varies    |    varies     |
| Tukey              | Parametric |      —       |   varies    |    varies     |

**Tier 2 — Extended:**

| Window              | Family      | Notes                                            |
| ------------------- | ----------- | ------------------------------------------------ |
| Triangle / Bartlett | Simple      | Bartlett variant shifts by half-sample           |
| Cosine              | Simple      | `sin(0.5 * pi * x)`, ENBW 1.189, sidelobe -23 dB |
| Welch               | Simple      | Parabolic: `1 - (1 - x)^2`                       |
| Lanczos             | Parametric  | sinc-based, `alpha` parameter (default 1)        |
| Gauss               | Parametric  | `exp(-ln2 * ((x-1)*alpha)^2)`, `alpha` parameter |
| Exact Blackman      | Cosine (3T) | -68.2 dB sidelobe                                |
| Blackman-Harris 3T  | Cosine (3T) | -70.9 dB sidelobe                                |
| Blackman-Nuttall    | Cosine (4T) | -98.2 dB sidelobe                                |
| Nuttall CTD         | Cosine (4T) | continuous 1st derivative                        |
| Nuttall CFD         | Cosine (4T) | continuous 1st derivative variant                |

**Tier 3 — Specialized:**

| Window           | Family                | Notes                          |
| ---------------- | --------------------- | ------------------------------ |
| Albrecht 2T–11T  | Cosine (configurable) | Configurable 2–11 cosine terms |
| Lawrey 5T        | Cosine (5T)           | 5-term optimized               |
| Lawrey 6T        | Cosine (6T)           | 6-term optimized               |
| Burgess Opt 59dB | Cosine (3T)           | Optimized for -59 dB sidelobe  |
| Burgess Opt 71dB | Cosine (3T)           | Optimized for -71 dB sidelobe  |
| FreeCosine       | Cosine (user)         | User-defined coefficient array |

#### 2c. API Surface

```go
package window

// Type identifies a window function.
type Type int

// Slope controls which edge(s) of the window are tapered.
type Slope int
const (
    SlopeSymmetric Slope = iota  // both edges (default)
    SlopeLeft                     // taper left edge only
    SlopeRight                    // taper right edge only
)

// Metadata holds spectral properties of a window type.
type Metadata struct {
    Name                string
    ENBW                float64 // equivalent noise bandwidth (bins)
    HighestSidelobe     float64 // dB
    CoherentGain        float64 // spectrum correction factor
    CoherentGainSquared float64 // squared spectrum correction factor
}

// Generate returns window coefficients of the given length.
func Generate(t Type, length int, opts ...Option) []float64

// Apply multiplies buf in-place by the window. Zero-alloc for standard windows.
func Apply(t Type, buf []float64, opts ...Option)

// Info returns spectral metadata for a window type.
func Info(t Type) Metadata

// Options for parametric windows and advanced features.
func WithAlpha(v float64) Option     // Kaiser beta, Gauss sigma, Tukey ratio, Lanczos alpha
func WithPeriodic() Option           // DFT-periodic (asymmetric)
func WithSlope(s Slope) Option       // left / symmetric / right edge tapering
func WithDCRemoval() Option          // subtract mean after windowing
func WithInvert() Option             // invert: 1 - w[n]
func WithBartlett() Option           // half-sample shift (Triangle only)
func WithCustomCoeffs(c []float64) Option  // user-defined cosine-term coefficients
```

#### 2d. Advanced Features (ported from mfw legacy)

These features exist in `MFWindowFunctions.pas` and are included from day one:

- **Window slope modes** (`TWindowSlope`): controls which edge(s) get tapered — `SlopeLeft`, `SlopeSymmetric`, `SlopeRight`.
- **DC removal** (`FZeroDC`): subtract mean after windowing.
- **Inversion** (`FInvert`): flip window vertically (`1 - w[n]`).
- **Bartlett variant** (`FBartlett`): half-sample shift for Triangle window.
- **Tukey percentage** (`FTukey`): edge taper ratio (0.0 = rectangular, 1.0 = Hann).
- **Correction factors**: spectrum correction factor (`FSpkCorFak`) and squared variant (`FSpkCorFakSq`) computed during coefficient generation and stored in `Metadata`.

#### 2e. Implementation Strategy

The cosine-term window family (Hann through Albrecht) shares a single engine:

1. **Coefficient tables** — ported from `MFWindowFunctionUtils.pas` (lines 22–145) as package-level `var` or `const` arrays.
2. **Horner evaluation** — evaluate cosine sum using nested multiplication (mirrors legacy `TMFWindowFunctionCosineTerm` approach).
3. **Parametric windows** — Kaiser uses modified Bessel I0, Gauss uses `exp(-ln2 * ...)`, Tukey is piecewise cosine/flat.
4. **Simple windows** — Rectangle (no-op), Triangle, Cosine, Welch each have direct formulas.

#### 2f. Task Breakdown

- [x] Define `Type` enum with all window types, `Slope` type, `Metadata` struct, `Option` pattern.
- [x] Implement cosine-term engine: shared coefficient lookup + Horner evaluation.
- [x] Port coefficient tables from `MFWindowFunctionUtils.pas` lines 22–145 as package constants.
- [x] Implement Tier 1 windows: Rectangular, Hann, Hamming, Blackman, Blackman-Harris 4T, FlatTop, Kaiser (with Bessel I0), Tukey.
- [x] Implement `Generate` and `Apply` with option handling.
- [x] Implement `Info()` returning ENBW, sidelobe, coherent gain, correction factors per type.
- [x] Implement advanced features: `WithSlope`, `WithDCRemoval`, `WithInvert`, `WithBartlett`.
- [x] Implement Tier 2 windows: Triangle/Bartlett, Cosine, Welch, Lanczos, Gauss, Exact Blackman, BH-3T, Blackman-Nuttall, Nuttall CTD, Nuttall CFD.
- [x] Implement Tier 3 windows: Albrecht (2T–11T), Lawrey 5T/6T, Burgess Opt 59dB/71dB, FreeCosine (`WithCustomCoeffs`).
- [ ] Golden vector tests against mfw outputs and/or NumPy/SciPy `scipy.signal.windows` references.
- [x] Benchmarks for `Generate` and `Apply` across sizes (256, 1024, 4096, 16384) with allocs/op tracking.
- [x] Runnable examples in package documentation.

#### 2g. Exit Criteria

- All Tier 1 + Tier 2 windows implemented and tested.
- All advanced features (slope modes, DC removal, inversion, Bartlett) implemented and tested.
- Golden vectors validated (at minimum: Hann, Hamming, Blackman-Harris 4T, Kaiser, FlatTop).
- Coverage >= 90% in `dsp/window`.
- Benchmarks present for `Generate` and `Apply`.
- `Info()` returns correct ENBW, sidelobe, and correction factors for all implemented types.
- Tier 3 windows implemented (may have lighter test coverage initially).

### Phase 3: Filter Runtime Primitives

Objectives:

- Build runtime processing blocks for IIR (biquad) and FIR filters.
- Port processing topology from `mfw/legacy/Source/MFFilter.pas` (2641 lines).
- Provide frequency-response evaluation helpers for runtime verification.

Phase 3 covers **runtime only** — coefficient design (Butterworth, Chebyshev, parametric EQ, etc.) lives in Phase 4 (`dsp/filter/design`).

#### 3.1 Legacy Architecture Reference

The legacy codebase implements filters through a deep class hierarchy rooted at `TMFDSPFilter`. The Go port uses flat, composition-based design instead.

```
TMFDSPFilter (abstract base: gain, sample rate, abstract ProcessSample)
├── TMFDSPFrequencyFilter (adds frequency, W0, sinW0)
│   └── TMFDSPOrderFilter (adds order)
│       ├── TMFDSPBandwidthFilter (adds bandwidth, alpha)
│       │   └── TMFDSPBiquadIIRFilter (single 2nd-order section)
│       │       ├── TMFDSPGainFilter, TMFDSPPeakFilter
│       │       ├── TMFDSPLowShelfFilter, TMFDSPHighShelfFilter
│       │       ├── TMFDSPHighcutFilter (LP), TMFDSPLowcutFilter (HP)
│       │       ├── TMFDSPBandpass, TMFDSPNotch, TMFDSPAllpass
│       │       └── TMFDSPShapeFilter
│       ├── TMFDSPButterworthFilter (cascaded SOS, order 1–64)
│       │   ├── TMFDSPButterworthLP / HP
│       │   └── TMFDSPCriticalLP / HP
│       └── TMFDSPChebyshevFilter (cascaded SOS with ripple)
│           ├── TMFDSPChebyshev1LP / HP
│           └── TMFDSPChebyshev2LP / HP
└── TMFDSPFreeFilter (arbitrary-order IIR, dynamic arrays)
```

Key implementation details:

- **Biquad topology**: Direct Form II Transposed (MFFilter.pas:737–743):
  `y = b0*x + d0; d0 = b1*x - a1*y + d1; d1 = b2*x - a2*y`
- **Coefficient naming**: Legacy `FNominator[0..2]` = b0/b1/b2, `FDenominator[1..2]` = a1/a2 (a0 normalized to 1)
- **Cascaded layout** (Butterworth/Chebyshev): interleaved `FAB[0..127]` with 4 doubles/section (b0,b1,a1,a2), `FState[0..63]` with 2 doubles/section, odd-order final first-order section
- **Frequency response**: closed-form `MagnitudeSquared`, `Phase`, `Complex` — no FFT required (MFFilter.pas:694–717)
- **State management**: push/pop state stack for non-destructive preview (MFFilter.pas:719–798)
- **Impulse response**: feed impulse through ProcessSample with state save/restore (MFFilter.pas:620–639)

#### 3.2 Legacy → Go Mapping

| Legacy (Pascal)                            | Go (algo-dsp)                          |
| ------------------------------------------ | -------------------------------------- |
| `TMFDSPBiquadIIRFilter.ProcessSample`      | `biquad.Section.ProcessSample`         |
| `TMFDSPBiquadIIRFilter.FNominator[0..2]`   | `biquad.Coefficients.B0, B1, B2`      |
| `TMFDSPBiquadIIRFilter.FDenominator[1..2]` | `biquad.Coefficients.A1, A2`          |
| `TMFDSPBiquadIIRFilter.FState[0..1]`       | `biquad.Section.d0, d1`               |
| `TMFDSPButterworthFilter.FAB[0..127]`      | `biquad.Chain.sections[i].Coefficients`|
| `TMFDSPButterworthFilter.FState[0..63]`    | `biquad.Chain.sections[i].d0/d1`      |
| `TMFDSPButterworthLP.ProcessSample`        | `biquad.Chain.ProcessSample`          |
| `TMFDSPBiquadIIRFilter.MagnitudeSquared`   | `biquad.Coefficients.MagnitudeSquared`|
| `TMFDSPBiquadIIRFilter.Phase`              | `biquad.Coefficients.Phase`           |
| `TMFDSPBiquadIIRFilter.Complex`            | `biquad.Coefficients.Response`        |
| `TMFDSPBiquadIIRFilter.GetIR`              | `biquad.Section.ImpulseResponse`      |
| `TMFDSPBiquadIIRFilter.PushStates/Pop`     | `biquad.Section.State/SetState`       |
| `TMFDSPFreeFilter.ProcessSample`           | (higher-order IIR via Chain, or later)|

#### 3a. Biquad Section (`dsp/filter/biquad`)

```go
package biquad

// Coefficients holds transfer function coefficients for a single
// second-order section. a0 is normalized to 1.
type Coefficients struct {
    B0, B1, B2 float64 // feedforward (numerator)
    A1, A2     float64 // feedback (denominator)
}

// Section is a single biquad with coefficients and DF-II-T state.
type Section struct {
    Coefficients
    d0, d1 float64 // delay line
}

func NewSection(c Coefficients) *Section
func (s *Section) ProcessSample(x float64) float64
func (s *Section) ProcessBlock(buf []float64)
func (s *Section) ProcessBlockTo(dst, src []float64)
func (s *Section) Reset()
func (s *Section) State() [2]float64
func (s *Section) SetState(state [2]float64)
```

#### 3b. Cascaded Chain (`dsp/filter/biquad`)

```go
// Chain cascades biquad sections in series for higher-order filters.
type Chain struct {
    sections []Section
    gain     float64
}

type ChainOption func(*chainConfig)
func WithGain(g float64) ChainOption

func NewChain(coeffs []Coefficients, opts ...ChainOption) *Chain
func (c *Chain) ProcessSample(x float64) float64
func (c *Chain) ProcessBlock(buf []float64)
func (c *Chain) Reset()
func (c *Chain) Order() int
func (c *Chain) NumSections() int
func (c *Chain) Section(i int) *Section
func (c *Chain) State() [][2]float64
func (c *Chain) SetState(states [][2]float64)
```

#### 3c. Frequency Response (`dsp/filter/biquad`)

```go
func (c *Coefficients) Response(freqHz, sampleRate float64) complex128
func (c *Coefficients) MagnitudeSquared(freqHz, sampleRate float64) float64
func (c *Coefficients) MagnitudeDB(freqHz, sampleRate float64) float64
func (c *Coefficients) Phase(freqHz, sampleRate float64) float64

func (c *Chain) Response(freqHz, sampleRate float64) complex128
func (c *Chain) MagnitudeDB(freqHz, sampleRate float64) float64

func (s *Section) ImpulseResponse(n int) []float64
func (c *Chain) ImpulseResponse(n int) []float64
```

#### 3d. FIR Runtime (`dsp/filter/fir`)

Direct-form FIR filter with circular-buffer delay line. Suitable for short filters (order < ~256). Partitioned/FFT convolution for long FIR is Phase 7 (`dsp/conv`).

```go
package fir

type Filter struct {
    coeffs []float64
    delay  []float64
    pos    int
}

func New(coeffs []float64) *Filter
func (f *Filter) ProcessSample(x float64) float64
func (f *Filter) ProcessBlock(buf []float64)
func (f *Filter) ProcessBlockTo(dst, src []float64)
func (f *Filter) Reset()
func (f *Filter) Order() int
func (f *Filter) Coefficients() []float64
func (f *Filter) Response(freqHz, sampleRate float64) complex128
func (f *Filter) MagnitudeDB(freqHz, sampleRate float64) float64
```

#### 3e. Task Breakdown

**3a. Biquad Section** (Critical):

- [ ] Define `Coefficients` struct and `Section` type.
- [ ] Implement `ProcessSample` — Direct Form II Transposed (port from MFFilter.pas:737–743).
- [ ] Implement `ProcessBlock` and `ProcessBlockTo`.
- [ ] Implement `Reset`, `State`, `SetState`.
- [ ] Table-driven tests: known coefficient sets -> expected output sequences.
- [ ] Property tests: gain=1 passthrough, zero coefficients -> silence.
- [ ] Benchmarks: `ProcessSample` and `ProcessBlock` at 256/1024/4096 samples.

**3b. Cascaded Chain** (Critical):

- [ ] Implement `Chain` with `NewChain`, gain option.
- [ ] Implement `ProcessSample` cascading through sections (port from MFFilter.pas:1374–1395).
- [ ] Implement `ProcessBlock`.
- [ ] Implement `Reset`, `State`/`SetState`, `Order`, `NumSections`, `Section`.
- [ ] Tests: 2nd/4th/6th order cascades with known coefficients.
- [ ] Test odd-order chain (first-order final section).
- [ ] Benchmarks: cascade throughput at various orders (2, 4, 8, 16).

**3c. Frequency Response** (High):

- [ ] Implement `Coefficients.Response` (complex H(z) evaluation).
- [ ] Implement `MagnitudeSquared` closed-form (port from MFFilter.pas:702–708).
- [ ] Implement `MagnitudeDB` and `Phase` (port from MFFilter.pas:694–717).
- [ ] Implement `Chain.Response` and `Chain.MagnitudeDB` (product of sections).
- [ ] Implement `ImpulseResponse` with state save/restore (port from MFFilter.pas:620–639).
- [ ] Tests: verify against known analytical responses (e.g., unit-gain allpass).
- [ ] Tests: verify `MagnitudeSquared` matches `|Response|²` within tolerance.

**3d. FIR Runtime** (Medium):

- [ ] Implement `Filter` with circular-buffer delay line.
- [ ] Implement `ProcessSample` — direct-form convolution.
- [ ] Implement `ProcessBlock`, `ProcessBlockTo`.
- [ ] Implement `Reset`, `Order`, `Coefficients`.
- [ ] Implement `Response` and `MagnitudeDB`.
- [ ] Tests: known FIR (e.g., 3-tap moving average, differentiator).
- [ ] Tests: impulse response matches coefficients.
- [ ] Benchmarks: FIR processing at various tap counts (8, 32, 128, 512).

**3e. Integration & Documentation**:

- [ ] Runnable examples: create biquad, process block, evaluate frequency response.
- [ ] Runnable example: cascaded chain.
- [ ] Ensure `go vet` and `golangci-lint` pass.
- [ ] Coverage >= 90% for `dsp/filter/biquad`, >= 85% for `dsp/filter/fir`.

#### 3f. Exit Criteria

- `biquad.Section.ProcessSample` produces bit-identical output to legacy DF-II-T for same coefficients and input.
- `biquad.Chain.ProcessSample` correctly cascades N sections, matching legacy Butterworth/Chebyshev processing loop structure.
- `MagnitudeSquared`, `Phase`, and `Response` match legacy closed-form formulas within 1e-12 tolerance.
- `ImpulseResponse` uses state save/restore and doesn't modify filter state.
- FIR direct-form produces correct output for known coefficient/input pairs.
- All tests pass with race detector (`go test -race`).
- Benchmarks present for all `ProcessSample`/`ProcessBlock` paths.
- `go vet` and `golangci-lint` clean.
- Coverage >= 90% for `dsp/filter/biquad`.

### Phase 4: Filter Design Toolkit

Objectives:

- Provide coefficient calculators that produce `biquad.Coefficients` (and `[]biquad.Coefficients` for cascaded designs) from frequency/gain/Q specs.
- Port design algorithms from `mfw/legacy/Source/MFFilter.pas` `CalculateCoefficients` methods.

Source: `MFFilter.pas` lines 868–2150 contain all coefficient calculations.

#### 4.1 Legacy Coefficient Design Reference

| Filter Type     | Legacy Class                     | Lines       | Notes                                                     |
| --------------- | -------------------------------- | ----------- | --------------------------------------------------------- |
| Peak (PEQ)      | `TMFDSPPeakFilter`              | 868–878     | Parametric EQ with gain and Q                             |
| Low Shelf       | `TMFDSPLowShelfFilter`          | 882–897     | Shelving with gain, uses `sqrt(gain)*alpha`               |
| High Shelf      | `TMFDSPHighShelfFilter`         | 901–916     | Shelving with gain                                        |
| Lowpass (LP)    | `TMFDSPHighcutFilter`           | 920–931     | Standard biquad LP with Q/bandwidth                       |
| Highpass (HP)   | `TMFDSPLowcutFilter`            | 935–946     | Standard biquad HP with Q/bandwidth                       |
| Bandpass        | `TMFDSPBandpass`                | 950–959     | Constant-skirt-gain bandpass                               |
| Notch           | `TMFDSPNotch`                   | 963–973     | Band-reject filter                                        |
| Allpass         | `TMFDSPAllpass`                 | (similar)   | Phase-shifting filter                                     |
| Gain            | `TMFDSPGainFilter`              | 977–984     | Pure gain, b0=gain²                                       |
| Shape           | `TMFDSPShapeFilter`             | 999–1090    | Parametric with shape control                             |
| Butterworth LP  | `TMFDSPButterworthLP`           | 1277–1339   | Bilinear-transform SOS cascade, `K=tan(W0/2)`, order 1–64|
| Butterworth HP  | `TMFDSPButterworthHP`           | 1452–1513   | HP variant with negated b1                                |
| Critical LP/HP  | `TMFDSPCriticalLP/HP`           | 1613–1750   | First-order only (Butterworth order=1)                    |
| Chebyshev I LP  | `TMFDSPChebyshev1LP`            | 1895–2032   | Ripple-factor SOS cascade                                 |
| Chebyshev I HP  | `TMFDSPChebyshev1HP`            | 2106–2150   | HP variant                                                |
| Chebyshev II LP | `TMFDSPChebyshev2LP`            | (similar)   | Stopband-ripple variant                                   |
| Chebyshev II HP | `TMFDSPChebyshev2HP`            | (similar)   | HP variant                                                |

#### 4a. API Surface (`dsp/filter/design`)

```go
package design

import "github.com/cwbudde/algo-dsp/dsp/filter/biquad"

// Biquad coefficient designers — each returns a single biquad.Coefficients.
func Lowpass(freq, q, sampleRate float64) biquad.Coefficients
func Highpass(freq, q, sampleRate float64) biquad.Coefficients
func Bandpass(freq, q, sampleRate float64) biquad.Coefficients
func Notch(freq, q, sampleRate float64) biquad.Coefficients
func Allpass(freq, q, sampleRate float64) biquad.Coefficients
func Peak(freq, gainDB, q, sampleRate float64) biquad.Coefficients
func LowShelf(freq, gainDB, q, sampleRate float64) biquad.Coefficients
func HighShelf(freq, gainDB, q, sampleRate float64) biquad.Coefficients

// Cascaded coefficient designers — return []biquad.Coefficients for Chain.
func ButterworthLP(freq float64, order int, sampleRate float64) []biquad.Coefficients
func ButterworthHP(freq float64, order int, sampleRate float64) []biquad.Coefficients
func Chebyshev1LP(freq float64, order int, rippleDB, sampleRate float64) []biquad.Coefficients
func Chebyshev1HP(freq float64, order int, rippleDB, sampleRate float64) []biquad.Coefficients
func Chebyshev2LP(freq float64, order int, rippleDB, sampleRate float64) []biquad.Coefficients
func Chebyshev2HP(freq float64, order int, rippleDB, sampleRate float64) []biquad.Coefficients

// Bilinear transform helpers (internal, but exported for advanced use).
func BilinearTransform(sCoeffs [3]float64, sampleRate float64) [3]float64
```

#### 4b. Task Breakdown

- [ ] Implement bilinear transform helper: `K = tan(W0 * 0.5)` and frequency pre-warping.
- [ ] Implement biquad designers: `Lowpass`, `Highpass`, `Bandpass`, `Notch`, `Allpass` (port MFFilter.pas:920–973).
- [ ] Implement `Peak`, `LowShelf`, `HighShelf` (port MFFilter.pas:868–916).
- [ ] Implement `ButterworthLP`/`HP` cascaded SOS design (port MFFilter.pas:1277–1513).
- [ ] Implement `Chebyshev1LP`/`HP` with ripple factors (port MFFilter.pas:1865–2150).
- [ ] Implement `Chebyshev2LP`/`HP` stopband-ripple variant.
- [ ] Handle odd-order Butterworth/Chebyshev (final first-order section).
- [ ] Golden vector tests: design at known freq/SR/order, compare coefficients against legacy output.
- [ ] Integration tests: design -> chain -> frequency response matches expected magnitude curve.
- [ ] Validate across sample rates: 44100, 48000, 96000, 192000 Hz.
- [ ] Runnable examples: design a 4th-order Butterworth LP, plot its response.

#### 4c. Exit Criteria

- All biquad designers produce coefficients matching legacy `CalculateCoefficients` within 1e-12.
- Butterworth/Chebyshev cascades match legacy `FAB` array output for orders 1–16.
- Designed filters → frequency response → magnitude at DC, Nyquist, and cutoff match expected values.
- Coverage >= 90% in `dsp/filter/design`.

### Phase 5: Filter Banks and Weighting

Objectives:

- Add application-oriented filter compositions.

Source: `mfw/legacy/Source/DSP/MFDSPWeightingFilters.pas` (A/B/C weighting as cascaded biquads), `MFDSPFractionalOctaveFilter.pas` (octave/fractional-octave banks).

#### 5a. Weighting Filters (`dsp/filter/weighting`)

```go
package weighting

import "github.com/cwbudde/algo-dsp/dsp/filter/biquad"

type Type int
const (
    TypeA Type = iota
    TypeB
    TypeC
    TypeZ // unity (no weighting)
)

// New returns a biquad.Chain configured for the given weighting curve
// at the specified sample rate.
func New(t Type, sampleRate float64) *biquad.Chain
```

Legacy reference: A-weighting uses 6th-order cascaded biquads, B uses 5th, C uses 4th. Coefficients are fixed per sample rate.

#### 5b. Filter Banks (`dsp/filter/bank`)

```go
package bank

// Octave builds an octave or fractional-octave filter bank.
func Octave(fraction int, sampleRate float64, opts ...Option) *Bank
```

Tasks:

- [ ] Implement A/B/C/Z weighting filters as pre-designed biquad chains.
- [ ] Implement octave/fractional-octave filter bank builders.
- [ ] Add convenience wrappers for block processing across all bands.
- [ ] Compliance-oriented validation tests for weighting curves (IEC 61672).

Exit criteria:

- Weighting filter magnitude responses match IEC 61672 tolerances.
- Octave bank center frequencies and bandwidths match standard definitions.

### Phase 6: Spectrum Utilities

Objectives:

- Provide FFT-adjacent processing independent of FFT implementation.

Tasks:

- [ ] Add magnitude/phase/power extraction helpers (complex FFT output -> real arrays).
- [ ] Add phase unwrapping and group delay calculations.
- [ ] Add smoothing/interpolation utilities (1/N-octave smoothing).
- [ ] Define interfaces that integrate cleanly with `algo-fft` outputs.

Exit criteria:

- No FFT implementation duplication; only integration helpers.
- Smooth integration with `algo-fft` complex output types.

### Phase 7: Convolution and Correlation

Objectives:

- Support linear/circular convolution and correlation workflows.

Tasks:

- [ ] Implement direct convolution baseline.
- [ ] Implement overlap-add and overlap-save strategies (using `algo-fft`).
- [ ] Implement cross-correlation and normalized variants.
- [ ] Add deconvolution with regularization options.
- [ ] Benchmark crossover points: direct vs. OLA vs. OLS by input/kernel size.

Exit criteria:

- Algorithm switches chosen by input size with benchmark-backed thresholds.
- Partitioned convolution can serve as FIR backend for long filters (hook from Phase 3d).

### Phase 8: Resampling

Objectives:

- High-quality sample rate conversion.

Tasks:

- [ ] Implement polyphase FIR resampler.
- [ ] Add rational ratio API and convenience wrappers.
- [ ] Add anti-aliasing defaults and quality modes.
- [ ] Validate passband/stopband performance targets.

Source: `mfw/legacy/Source/DSP/MFDSPPolyphaseFilter.pas` (polyphase with FPU/3DNow/SSE variants).

Exit criteria:

- Published quality/performance matrix for standard ratios (44.1k<->48k, 2x, 4x).

### Phase 9: Signal Generation and Utilities

Objectives:

- Generators and common transforms for tests and measurements.

Tasks:

- [ ] Implement sine/multisine/noise/impulse/sweep generators.
- [ ] Implement normalize, clip, DC removal, envelope helpers.
- [ ] Add deterministic seed strategy for reproducibility.

Exit criteria:

- Generators usable as fixtures in measure package tests.

### Phase 10: Measurement Kernels (THD)

Objectives:

- Build measurement logic reusable across applications.

Tasks:

- [ ] THD/THD+N calculator core.
- [ ] Fundamental detection strategies.
- [ ] Harmonic extraction and odd/even summaries.
- [ ] Noise floor and SINAD utilities.

Exit criteria:

- Accuracy validated with synthetic + recorded reference sets.

### Phase 11: Measurement Kernels (Sweep/IR)

Objectives:

- Log-sweep and impulse-response analysis kernels.

Tasks:

- [ ] Log sweep generation and inverse filter generation.
- [ ] Deconvolution pipeline.
- [ ] Harmonic IR separation.
- [ ] IR metrics (RT60, EDT, C50, C80, D50, center time).

Exit criteria:

- Deterministic outputs for fixed settings and fixtures.

### Phase 12: Stats Packages

Objectives:

- Add reusable time/frequency statistics.

Tasks:

- [ ] Time-domain stats (RMS, crest factor, moments, crossings).
- [ ] Frequency-domain stats (centroid, flatness, bandwidth).
- [ ] Streaming and block-based variants.

Exit criteria:

- Stable APIs and doc examples for all major stats.

### Phase 13: Optimization and SIMD Paths

Objectives:

- Improve hot-path throughput without API churn.

Tasks:

- [ ] Profile-based optimization plan.
- [ ] Add architecture-specific optional kernels behind build tags.
- [ ] Keep scalar fallback as correctness source of truth.
- [ ] Benchmark and verify numerical parity across variants.

Exit criteria:

- Measurable gains on targeted workloads and no correctness regressions.

### Phase 14: API Stabilization and v1.0

Objectives:

- Freeze public API and publish stable release.

Tasks:

- [ ] Deprecate or remove experimental APIs.
- [ ] Complete package docs and examples.
- [ ] Create migration notes for prerelease users.
- [ ] Tag `v1.0.0` once compatibility guarantees are met.

Exit criteria:

- API review completed.
- CI, tests, benchmarks, docs all green.

---

## 7. Testing and Validation Strategy

### 7.1 Test Types

- Unit tests (table-driven and edge-case heavy).
- Property-based tests for invariants.
- Golden vector tests for deterministic algorithm outputs.
- Integration tests across package boundaries.

### 7.2 Numerical Validation

- Define tolerance policy per algorithm category.
- Compare selected outputs against trusted references (MATLAB/NumPy/known datasets).
- Track expected floating-point drift across architectures.

### 7.3 Coverage Targets

- Project-wide: >= 85% where practical.
- Core algorithm packages: >= 90%.

---

## 8. Benchmarking and Performance Strategy

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

## 9. Dependency and Versioning Policy

- Keep external dependencies minimal and justified.
- Prefer pure-Go paths unless CGo brings clear, measured value.
- `algo-fft` is consumed via narrow integration interfaces.
- Use semantic versioning; document breaking changes before major bumps.
- Support latest Go stable and previous stable.

---

## 10. Release Engineering

- Conventional commits for changelog generation.
- Tag-driven releases with generated notes.
- Pre-release channel (`v0.x`) until API freeze.
- Required release gates:
  - Lint + tests + race checks
  - Benchmark sanity pass
  - Documentation/examples up to date

---

## 11. Migration Plan from `mfw`

### 11.1 Extraction Sequence

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

### 11.2 Migration Mechanics

- Keep APIs in `mfw` adapter-friendly during extraction.
- Move code with tests first; then switch imports.
- Add compatibility tests in `mfw` to validate behavior parity.
- Remove duplicated code only after parity checks pass.

### 11.3 Completion Definition

- `mfw` retains orchestration and app-specific domain logic only.
- Algorithm-heavy packages imported from `algo-dsp`.
- CI in both repos passes with pinned compatible versions.

---

## 12. Risks and Mitigations

| Risk                                     | Impact | Mitigation                                            |
| ---------------------------------------- | ------ | ----------------------------------------------------- |
| API churn during extraction              | Medium | Enforce phased stabilization and deprecation windows  |
| Numerical regressions after optimization | High   | Scalar reference path + parity tests + golden vectors |
| Scope creep into app/file concerns       | Medium | Strict boundary rules and review checklist            |
| Performance regressions across CPUs      | Medium | Per-arch benchmarks and build-tag fallback            |
| Test fixture fragility                   | Low    | Versioned fixture sets and deterministic generation   |

---

## 13. Initial 90-Day Execution Plan

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

## 14. Revision History

| Version | Date       | Author | Changes                                                                                                                                                                                               |
| ------- | ---------- | ------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 0.1     | 2026-02-06 | Codex  | Initial comprehensive `algo-dsp` development plan                                                                                                                                                     |
| 0.2     | 2026-02-06 | Claude | Refined Phase 1 (buffer type in `dsp/buffer`), rewrote Phase 2 (window functions) with full mfw legacy inventory (25+ types, 3 tiers, advanced features), updated architecture and migration sections |
| 0.3     | 2026-02-06 | Claude | Rewrote Phase 3 (filter runtime) with full MFFilter.pas analysis: biquad DF-II-T, cascaded chains, frequency response, FIR runtime, legacy mapping table. Refined Phase 4 (filter design) with per-filter-type legacy source references and API surface. Refined Phase 5 (weighting/banks) with legacy source references. Updated migration section with filter extraction sources |

---

This plan is a living document and should be updated after each phase completion and major architectural decision.
