# algo-dsp: Development Plan

## Comprehensive Plan for `github.com/cwbudde/algo-dsp`

This document defines a phased plan for building `algo-dsp` as a reusable,
production-quality DSP (Digital Signal Processing) algorithm library in Go.

It is intentionally separated from:

- application concerns (`mfw`) and
- file/container concerns (`wav`).

This plan is **actionable**: every phase contains **checkable tasks and subtasks**.

---

## Table of Contents

1. Project Scope and Goals
2. Repository and Module Boundaries
3. Architecture and Package Layout
4. API Design Principles
5. Phase Overview
6. Detailed Phase Plan (Phases 0–43)
7. Appendices
   - Appendix A: Testing and Validation Strategy
   - Appendix B: Benchmarking and Performance Strategy
   - Appendix C: Dependency and Versioning Policy
   - Appendix D: Release Engineering
   - Appendix E: Migration Plan from `mfw`
   - Appendix F: Risks and Mitigations
   - Appendix G: Initial 90-Day Execution Plan
   - Appendix H: Revision History

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
- Optional algorithmic effects (strictly algorithm-only; no I/O).

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

- No dependency on Wails/React/app-specific DTOs/desktop runtime packages.
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

Phases are strictly numbered (no sub-phases). Completed phases come first in their original
order; remaining work follows in execution order, ending with the v1.0 release. Phases 16, 22,
23, 24, and 30 are scoped to the work actually shipped — their open follow-ups are split out as
separate numbered phases below.

```plain
# Completed
Phase 0:  Bootstrap & Governance                          [1 week]   ✅ Complete
Phase 1:  Numeric Foundations & Core Utilities            [2 weeks]  ✅ Complete
Phase 2:  Window Functions                                 [2 weeks]  ✅ Complete
Phase 3:  Filter Runtime Primitives                        [3 weeks]  ✅ Complete
Phase 4:  Filter Design Toolkit                            [3 weeks]  ✅ Complete
Phase 5:  Filter Banks and Weighting                       [2 weeks]  ✅ Complete
Phase 6:  Spectrum Utilities                               [2 weeks]  ✅ Complete
Phase 7:  Convolution and Correlation                      [2 weeks]  ✅ Complete
Phase 8:  Resampling                                       [3 weeks]  ✅ Complete
Phase 9:  Signal Generation and Utilities                  [2 weeks]  ✅ Complete
Phase 10: Measurement Kernels (THD)                        [3 weeks]  ✅ Complete
Phase 11: Measurement Kernels (Sweep/IR)                   [3 weeks]  ✅ Complete
Phase 12: Stats Packages                                   [2 weeks]  ✅ Complete
Phase 13: Advanced Parametric EQ Design                    [2 weeks]  ✅ Complete
Phase 14: High-Order Graphic EQ Bands                      [4 weeks]  ✅ Complete
Phase 15: Effects — High-Priority Modulation               [2 weeks]  ✅ Complete
Phase 16: Effects — High-Priority Dynamics (core)          [2 weeks]  ✅ Complete  → curve parity: P31
Phase 17: Effects — High-Priority Spatial                  [1 week]   ✅ Complete
Phase 18: Effects — Medium-Priority Waveshaping/Lo-fi      [2 weeks]  ✅ Complete
Phase 19: Effects — Medium-Priority Modulation             [2 weeks]  ✅ Complete
Phase 20: Effects — Medium-Priority Dynamics               [2 weeks]  ✅ Complete
Phase 21: Effects — Spatial and Convolution Reverb         [2 weeks]  ✅ Complete
Phase 22: Effects — Specialized (Spectral Freeze, Granular)[2 weeks]  ✅ Complete  → rest: P33–P37
Phase 23: High-Order Shelving (Butterworth, Chebyshev I/II)[2 weeks] ✅ Complete  → elliptic: P32
Phase 24: Optimization — Spectrum Fast Path & Bench Harness[1 week]   ✅ Complete  → guard/SIMD: P40–P41
Phase 25: Nonlinear Moog Ladder Filters                    [3 weeks]  ✅ Complete
Phase 26: Goertzel Tone Analysis                           [2 weeks]  ✅ Complete
Phase 27: Loudness Metering (EBU R128 / BS.1770)           [3 weeks]  ✅ Complete
Phase 28: Dither and Noise Shaping                         [3 weeks]  ✅ Complete
Phase 29: Polyphase Hilbert / Analytic Signal              [2 weeks]  ✅ Complete
Phase 30: Interpolation Kernels (core)                     [2 weeks]  ✅ Complete  → expansion: P38–P39

# Remaining (execution order)
Phase 31: Dynamics — Static Characteristic-Curve Parity    [0.5 week] ✅ Complete
Phase 32: Elliptic Shelving Designer                       [1 week]   ✅ Complete
Phase 33: Vocoder Finalization                             [0.5 week] ✅ Complete
Phase 34: Stereo Panner                                    [0.5 week] ✅ Complete
Phase 35: Dynamic EQ                                       [1 week]   ✅ Complete
Phase 36: Pitch Correction (YIN)                           [1 week]   ✅ Complete
Phase 37: Noise Reduction                                  [1 week]   📋 Planned
Phase 38: Interpolation Kernel Expansion                   [1 week]   📋 Planned
Phase 39: Interpolation Integration & Validation           [1 week]   📋 Planned
Phase 40: Benchmark Regression Guard                       [1 week]   🔄 In Progress
Phase 41: SIMD Modal Oscillator Bank                       [2 weeks]  📋 Planned
Phase 42: Release Readiness (v1.0)                         [1 week]   📋 Planned
Phase 43: Tag and Publish v1.0                             [0.5 week] 📋 Planned
```

---

## 6. Detailed Phase Plan

Completed phases are summarized as short bullet lists. In-progress and planned phases keep checkable task lists.

### Phase 0: Bootstrap & Governance (Complete)

- Go module + baseline repo structure.
- `justfile` workflow (test/lint/fmt/bench/ci).
- CI for latest + previous Go versions.
- Contribution/governance docs + release/versioning conventions.

### Phase 1: Numeric Foundations & Core Utilities (Complete)

- Numeric helpers + functional options pattern used across packages.
- `dsp/buffer`: `Buffer` + `Pool` for scratch reuse.
- `internal/testutil`: deterministic signals + tolerance helpers.
- Unit tests + docs/examples for the public surface.

### Phase 2: Window Functions (Complete)

- 25+ window types with coefficient generators.
- Window metadata (ENBW/coherent gain/sidelobes/corrections).
- Advanced behaviors (slope modes, inversion, DC removal, Tukey/variants).
- Tests + runnable examples.

### Phase 3: Filter Runtime Primitives (Complete)

- Biquad runtime (DF-II-T) + cascades.
- Frequency response helpers (magnitude/phase/DB).
- FIR direct-form runtime.
- Tests + benchmarks (coverage targets achieved).

### Phase 4: Filter Design Toolkit (Complete)

- RBJ-style biquad designers (LP/HP/BP/Notch/Allpass/Peak/LS/HS).
- Butterworth + Chebyshev (I/II) cascades.
- Multi-sample-rate validation + tests + runnable examples.

### Phase 5: Filter Banks and Weighting (Complete)

- A/B/C/Z weighting filters as biquad chains.
- Octave + fractional-octave bank builders.
- Curve validation + tests + benchmarks.

### Phase 6: Spectrum Utilities (Complete)

- Spectrum extraction helpers (magnitude/phase/power).
- Phase unwrap + group delay.
- 1/N-octave smoothing + interpolation helpers.
- Tests + examples; FFT-backend agnostic.

### Phase 7: Convolution and Correlation (Complete)

- Direct convolution + overlap-add/save FFT strategies.
- Cross/auto-correlation + deconvolution variants.
- Streaming variants.
- Benchmarks + runnable examples.

### Phase 8: Resampling (Complete)

- Polyphase FIR resampler with rational ratio API.
- Anti-alias defaults + quality modes.
- Tests across common ratio matrix + benchmarks.

### Phase 9: Signal Generation and Utilities (Complete)

- Deterministic generators (sine/multisine/noise/impulse/sweeps).
- Utility transforms (normalize/clip/DC removal/envelopes).
- Tests + runnable examples.

### Phase 10: Measurement Kernels (THD) (Complete)

- `measure/thd`: THD/THD+N analysis with auto fundamental detection and harmonic capture.
- Metrics: odd/even, noise, rub&buzz, SINAD.
- Tests + benchmarks + runnable examples.
- Legacy parity within tolerance.

### Phase 11: Measurement Kernels (Sweep/IR) (Complete)

- `measure/sweep`: log + linear sweeps, inverse filters, deconvolution, harmonic IR extraction.
- `measure/ir`: Schroeder integration + RT metrics + clarity/definition/center time + impulse start.
- Tests + runnable examples.

### Phase 12: Stats Packages (Complete)

- `stats/time`: batch + streaming parity (Welford/moments, RMS/DC/peak/range/crest/energy/power/zero-crossings).
- `stats/frequency`: centroid/spread/flatness/rolloff/bandwidth + basic spectrum stats.
- Zero-alloc hot paths; tests + benchmarks; coverage targets achieved.

### Phase 13: Advanced Parametric EQ Design (Orfanidis) (Complete)

- `dsp/filter/design/orfanidis`: Orfanidis-family parametric EQ coefficient design.
- Expert + audio-friendly APIs.
- Higher-order cascade helper producing `[]biquad.Coefficients`.
- Validation + response sanity tests.
- Docs + runnable example.

### Phase 14: High-Order Graphic EQ Bands (Complete)

- `dsp/filter/design/band`: gain-adjustable high-order band designers for graphic EQ.
- Topologies: Butterworth, Chebyshev I, Chebyshev II, Elliptic.
- Designers return SOS (`[]biquad.Coefficients`) to keep runtime unchanged.
- Stability + response conformance tests.
- Docs + runnable example.

---

### Phase 15: Effects — High-Priority Modulation (Complete)

- Flanger (short modulated delay + feedback + interpolated tap), phaser (4–12 stage allpass
  cascade + LFO), tremolo (LFO amplitude mod + smoothing).
- Constructor+options; `Process`/`ProcessInPlace`/`Reset` per effect.
- Tests + runnable examples; `go test -race ./dsp/effects/` passes.

### Phase 16: Effects — High-Priority Dynamics (core) (Complete)

- Shared dynamics core (`dsp/effects/dynamics/core.go`) with feedforward + feedback topologies,
  peak/RMS detectors, optional sidechain prefilter, hard/soft-knee gain computers
  (`GainForLevel`), manual/auto make-up gain, deterministic reset, and sample-rate-aware
  coefficient recalculation + strict validation.
- Compressor variants — feedforward (peak/RMS/sidechain) and feedback (hard/soft knee, ratio-
  dependent time constants); API surface (`ProcessSample`, `ProcessSampleSidechain`,
  `ProcessInPlace`, `Reset`, `ResetMetrics`) in `dsp/effects/dynamics/compressor.go`.
- De-esser, gate, limiter, expander (hard/soft knee + range), multiband compressor (crossover +
  per-band, recombination gain-normalization + phase/latency checks) — each with tests + examples.
- Streaming legacy parity (`legacy_parity_test.go`) + step/burst temporal tests + hot-path
  benchmarks (near-zero allocs in-place).

> Open follow-up: static characteristic-curve parity is split out as **Phase 31**.

### Phase 17: Effects — High-Priority Spatial (Complete)

- Stereo widener (M/S gains with safe bounds + mono-compatibility tests).
- Crosstalk cancellation: geometric delay-model port (delay line + highshelf + attenuation,
  staged) with validation and parity tests.
- Crosstalk simulator (IIR): `Handcrafted`/`IRCAM`/`HDPHX` presets as cascaded biquad shaping,
  diameter-derived delayed crossfeed, polarity + dry/crossfeed mix.
- Crosstalk simulator (HRTF): transport-agnostic HRTF-provider interface, crossfeed-only and
  full direct+crossfeed convolution modes, IR reload on HRTF/sample-rate change.
- Validation + runnable examples per effect.

### Phase 18: Effects — Medium-Priority Waveshaping/Lo-fi (Complete)

- Distortion: soft/hard/tanh shapers + legacy waveshaper family (`Waveshaper1..8`, `Saturate*`,
  `SoftSat`) + Chebyshev harmonic core; fast polynomial paths; transfer-curve/harmonic parity tests.
- Transformer simulation: pre-emphasis/damping + oversampling + nonlinear waveshaper +
  downsampling, with HQ and approximation paths and anti-aliasing validation.
- Bit crusher: bit-depth + sample-rate reduction (quantize + sample-and-hold).
- Tests + runnable examples.

### Phase 19: Effects — Medium-Priority Modulation (Complete)

- Auto-wah (envelope follower modulating a filter) and ring modulator (carrier multiply + mix).
- Tests + runnable examples.
- Live web demo: <https://cwbudde.github.io/algo-dsp/>.

### Phase 20: Effects — Medium-Priority Dynamics (Complete)

- Transient shaper (attack/release split + shaping) and lookahead limiter (delay + detector + gain).
- Tests + runnable examples.

### Phase 21: Effects — Spatial and Convolution Reverb (Complete)

- Reverb suite in `dsp/effects/reverb`: `ConvolutionReverb` (FFT-backed partitioned-convolution
  kernel `dsp/conv/partitioned.go`), `FDNReverb` (feedback delay network), and `Reverb`/Freeverb.
- `HaasDelay` (`dsp/effects/spatial/haas.go`): short precedence delay reusing the package's
  `monoDelay` ring buffer, with `ProcessStereo`/in-place/interleaved APIs.
- Tests + runnable examples + benchmarks.

### Phase 22: Effects — Specialized (Spectral Freeze, Granular) (Complete)

Recovered onto `main` from the orphaned release lineage (see Appendix H):

- Spectral freeze (`dsp/effects/spectral_freeze.go`): overlap-add STFT with magnitude hold and
  `PhaseHold`/`PhaseAdvance` strategies; configurable frame/hop/window/mix. Tests + example.
- Granular (`dsp/effects/granular.go`): real-time-safe grain scheduler with Hann-windowed
  overlap-add and grain-size/overlap/pitch/spray controls. Tests + example.

> Open follow-ups (independent effects, each its own phase): vocoder finalization **Phase 33**,
> stereo panner **Phase 34**, dynamic EQ **Phase 35**, pitch correction **Phase 36**, noise
> reduction **Phase 37**.

### Phase 23: High-Order Shelving Filters (Butterworth, Chebyshev I/II) (Complete)

High-order low/high-shelf designers in `dsp/filter/design/shelving/` returning SOS, with the
signature `XxxLow/HighShelf(sampleRate, freqHz, gainDB float64, order int) ([]biquad.Coefficients, error)`
(`order >= 1`; odd orders add a first-order section; `gainDB == 0` → passthrough; frequency-bound
and NaN/Inf validation).

- Butterworth (`butterworth.go`), Chebyshev I (`chebyshev1.go`), and Chebyshev II (`chebyshev2.go`,
  Orfanidis framework) designers + tests (endpoint anchors, monotonicity, grid sweeps,
  DC/Nyquist ± stopband ripple). The earlier Chebyshev II shape bug is fixed.

> Open follow-up: elliptic shelving is split out as **Phase 32**.

### Phase 24: Optimization — Spectrum Fast Path & Benchmark Harness (Complete)

- Zero-alloc fast path in spectrum helpers (removed temporary-unpacking allocations); spectrum
  code wired to prefer it; before/after recorded in `BENCHMARKS.md`.
- Stable hot-path benchmark subset + a CI-friendly `just bench-ci` report target.
- `algo-vecmath v0.1.0` SIMD primitives (ADD/MUL/SCALE/ADDMUL/MAXABS, with generic/AVX2/SSE2/NEON
  paths) integrated and benchmarked (2.6–4.1× generic→SIMD; see `BENCHMARKS.md`).

> Open follow-ups: benchmark regression guard **Phase 40**; SIMD modal oscillator bank **Phase 41**.
> The v1.0 release work is **Phases 42–43**.

### Phase 25: Nonlinear Moog Ladder Filters (Complete)

- `dsp/filter/moog`: `New(sampleRate, ...Option)` with six variants via `WithVariant`
  (`Classic`, `ClassicLightweight`, `ImprovedClassic`, `ImprovedClassicLightweight`,
  `Huovilainen`, `ZDF`) and the full option/setter surface (cutoff, resonance, drive, in/out
  gain + normalization, thermal voltage, oversampling, Newton iterations), with strict
  validation/numeric guard rails and mono + stereo/frame helpers.
- `VariantZDF`: zero-delay-feedback TPT with Newton-Raphson (Zavalishin / D'Angelo–Välimäki)
  for accurate high-cutoff tuning and self-oscillation; oversampling (2/4/8×) with anti-alias
  filtering + Huovilainen half-sample feedback compensation on the other variants.
- Legacy parity (classic/improved/lightweight), tuning/frequency-response grids, nonlinear
  drive/self-oscillation + rapid-modulation stability tests, examples, benchmarks; all `-race`.
- Adopted from the release lineage during reconciliation (see Appendix H).

### Phase 26: Goertzel Tone Analysis (Complete)

- `dsp/spectrum/goertzel.go`: stateful single-bin `Goertzel` + batched `GoertzelBank`
  (DTMF/pilot-tone) with legacy recurrence + power-formula parity, outputs (power/magnitude/
  dB-floored/complex/normalized), and strict validation.
- One-shot `ProcessBlock`/`AnalyzeBlock` + zero-alloc streaming `ProcessSample`; DFT-reference
  and off-bin correctness tests, edge cases, examples, benchmarks (0 allocs/op).
- Fused from the two lineages' implementations into one canonical file (see Appendix H).

### Phase 27: Loudness Metering (EBU R128 / BS.1770) (Complete)

- `measure/loudness` `Meter`: K-weighting prefilter, 400 ms momentary + 3 s short-term windows,
  integrated loudness with absolute/relative gating, mono/stereo (`WithChannels`), true-peak
  tracking, streaming APIs (`Reset`, `Start/StopIntegration`, `ProcessSample/Block`,
  `Momentary`, `ShortTerm`, `Integrated`, `Peaks`).
- R128/BS.1770 conformance + parity + sample-rate-matrix + long-run stability tests, examples,
  benchmarks; depends only on `dsp/core`, `dsp/filter/biquad`, `dsp/filter/design`.
- Recovered from the release lineage (see Appendix H).
- Deferred: allocation-free callback/event-hook API; explicit loudness-range (LRA) metric.

### Phase 28: Dither and Noise Shaping (Complete)

- `dsp/dither`: dither PDFs (none/rectangular/triangular/gaussian/fast-gaussian) with injectable
  RNG, a `Quantizer` (int/float modes + optional limiting), a `NoiseShaper` interface with FIR
  error-feedback + IIR low-shelf implementations, and legacy coefficient presets
  (E/F/IE/ME/SBM/sharp, with sample-rate-aware "sharp" selection).
- `dsp/dither/design`: ATH/critical-band models + a stochastic ATH-weighted coefficient
  optimizer with order/runtime guardrails and cancellation.
- Null/error-spectrum + preset-parity tests, examples, benchmarks; all `-race`.
- Recovered from the release lineage (see Appendix H).

### Phase 29: Polyphase Hilbert / Analytic Signal (Complete)

- `dsp/filter/hilbert`: 64-bit and 32-bit two-path polyphase/allpass quadrature (A/B)
  processors with reusable state, count-specialized fast paths + generic fallback, analytic-
  envelope helper, coefficient designer + presets, `ProcessSample/Block`, `Reset/ClearBuffers`.
- Phase-quadrature, amplitude-matching, image-rejection, and legacy-parity tests, examples,
  benchmarks; all `-race`.
- Recovered from tags `v0.5.0`/`v0.5.1`; paired with `Chain.Gain`/`SetGain` on
  `dsp/filter/biquad/chain.go`. Bundled frequency-shifter effect deferred (see Appendix H).

### Phase 30: Interpolation Kernels (core) (Complete)

- `dsp/interp` (`interp.go`): `Linear2`, `Hermite4`, `Lagrange4`, `Lanczos6`/`LanczosN`,
  Blackman-windowed `SincInterp`, and a first-order allpass tick, selected via an `interp.Mode`
  enum.
- `dsp/delay/line.go`: `Line` ring buffer with `Write`/`Read`/`ReadFractional(delay)` and
  `WithMode`/`WithSincN` options; `dsp/resample` polyphase FIR resampler (Kaiser-designed,
  Fast/Balanced/Best quality) consumes fractional-phase interpolation internally.
- Tests + docs. Both `dsp/interp` and `dsp/delay` are listed as new public packages in
  `CHANGELOG.md`.

> Open follow-ups: kernel expansion **Phase 38**; integration & validation **Phase 39**.

---

The phases below are the remaining roadmap, in execution order. Each is intentionally small and
ships with tests + a runnable example unless noted.

### Phase 31: Dynamics — Static Characteristic-Curve Parity (Complete)

Validated the steady-state transfer behavior of the Phase 16 dynamics suite against
`legacy/Source/DSP/DAV_DspDynamics.pas` (`CharacteristicCurve`/`CharacteristicCurve_dB`); previously
`legacy_parity_test.go` only covered streaming simulation, not static curves.

- [x] Added a static curve builder (`dsp/effects/dynamics/curve.go`): `StaticCurve(p, min, max,
step)` returning `[]CurvePoint{InputDB, OutputDB, GainReductionDB}` over a `StaticCurveProcessor`
      interface satisfied by Compressor/Expander/Gate via their `CalculateOutputLevel` (gain-computer
      path), plus `MultibandCompressor.BandStaticCurve` for per-band curves. No streaming/detector state
      is touched (verified by a non-mutation test).
- [x] Validated `in → out` and gain-reduction curves across threshold/ratio/knee sweeps: compressor
      and expander match the legacy hard-knee transfer law exactly (1e-9), soft-knee rejoins the legacy
      asymptote outside the knee and stays monotonic/non-amplifying within it.

Exit criteria:

- [x] Characteristic-curve parity tests pass; `go test -race ./dsp/effects/dynamics` passes; lint clean.

### Phase 32: Elliptic Shelving Designer (Complete)

Added the missing elliptic topology to `dsp/filter/design/shelving/` and, in the process,
replaced the Chebyshev II designer, which was a Butterworth shelf in disguise.

- [x] New `internal/orfanidis`: the Orfanidis (JAES 53(11), 2005) analog prototypes
      (`EllipticPrototype`, `Chebyshev2Prototype`) plus `LowpassBLT`/`BandpassBLT` and an
      `EdgeOmega` bisection that places the band edge at an arbitrary level. Extracted from
      `band/elliptic.go`, which now consumes it; a parity test pins the extracted Chebyshev II
      prototype to the shipped band closed form at 1e-12.
- [x] `EllipticLowShelf`/`EllipticHighShelf(sampleRate, freqHz, gainDB, stopbandDB, order)` for
      `order >= 1`, odd orders included. `stopbandDB` bounds the flat region; the shelf-side
      ripple is fixed at 0.05 dB as in `band.EllipticBand`.
- [x] All four families now share the cutoff convention `|H(f_c)|² = (G²+1)/2` of
      Holters & Zölzer eq. (5), verified to 1e-6 dB across an order/gain/frequency grid.
- [x] Equiripple conformance tests (envelope never exceeded once entered, exactly M-1
      flat-band extrema), endpoint anchors, stability grids, examples and benchmarks — the
      package had neither examples nor benchmarks before.
- [x] Removed the dead `chebyshev2Sections` with its empirical damping constants
      (3.65 / 16.499 / 0.2), which compensated for a frequency scaling lost in its σ/R²
      reparametrization, plus the now-unused `invertSections`.

Exit criteria:

- [x] Elliptic shelf shape validated; `go test -race ./dsp/filter/design/shelving` passes;
      lint clean; shelving coverage 93.6%, `internal/orfanidis` 91.9%.

> Note: `Chebyshev2*Shelf` coefficients and cutoff semantics changed as a result. See
> CHANGELOG.md; boost/cut is no longer exactly reciprocal, matching the Butterworth family.

### Phase 33: Vocoder Finalization (Complete)

`dsp/effects/vocoder.go` implements the analysis/synthesis vocoder
(`NewVocoder(sampleRate, opts...)`, `ProcessSample`/`ProcessBlock(modulator, carrier, output)`,
`WithBandLayout` selecting `BandLayoutThirdOctave`/`BandLayoutBark`, attack/release/Q/level
options, and optional per-band multirate analysis via `WithDownsampling`) with `vocoder_test.go`.

- [x] Added the missing runnable examples (`vocoder_example_test.go`): defaults, `ProcessBlock`
      envelope transfer, the Bark layout with a synthesis-Q override, and multirate downsampling.
- [x] Rounded out coverage — every exported option, getter and setter on the vocoder is now at
      100%: the `SynthesisQ` getter, `WithVocoderSynthLevel` validation, nil-option and
      option-error paths in `NewVocoder`, the "no usable bands" error for both layouts, and
      `SetDownsampling`'s disable path (state cleared, `DownsampleFactors() == nil`, full-rate
      processing still finite) plus its Bark recompute branch.

> The residual uncovered statements in `vocoder.go` are defensive branches unreachable through the
> public API (option validation forbids non-positive attack/release, `WithBandLayout` rejects
> unknown layouts, and band selection filters `f < 0.9*nyquist` before `cpgBandpass` sees it).

Exit criteria:

- [x] Examples build and their `// Output:` blocks match; `go test -race ./dsp/effects` passes.

### Phase 34: Stereo Panner (Complete)

`StereoPanner` (`dsp/effects/spatial/stereo_panner.go`) follows the `HaasDelay` API shape
(constructor + options, shared `validate*` helpers, `ProcessStereo`/in-place/interleaved,
`Reset`, getters + `Set*`).

- [x] Selectable pan law: `PanLawEqualPower` (−3 dB centre, `gL²+gR²=1`, default),
      `PanLawCompromise` (−4.5 dB, geometric mean) and `PanLawLinear` (−6 dB, `gL+gR=1`).
- [x] Two modes: mono-pan (`ProcessMono`, `ProcessMonoToStereo`, `ProcessMonoToInterleaved`)
      applying the raw law gains, and attenuate-only stereo balance (`ProcessStereo` and friends)
      where the centre is unity pass-through and off-centre only ever fades the far channel.
- [x] Optional auto-pan LFO (`WithAutoPanRate`/`WithAutoPanDepth`) using the inline sine
      accumulator pattern of `dsp/effects/modulation/tremolo.go`; disabled by default so the
      static path uses cached, trig-free gains.
- [x] Tests (power/amplitude invariants across the sweep, centre levels, hard-left/right,
      monotonicity, balance never boosts, auto-pan sweep/depth/determinism) + 4 runnable
      examples + benchmarks.

> Not yet wired into `dsp/effectchain` or the web demo. Tracked as demo coverage work in
> Phase 41b, not as a permanent exclusion.

Exit criteria:

- [x] Equal-power sweep holds `gL²+gR²=1` to 1e-12; `go test -race ./dsp/effects/spatial`
      passes; static path is 0 allocs/op.

### Phase 35: Dynamic EQ (Complete)

- [x] Per-band filter + detector + gain mapping in `dsp/effects/dynamics/dynamic_eq.go`: peaking and
      shelving bands run in series on the full-band signal, each pairing a `biquad.Section` with its own
      `dynamicsCore` detector and soft-knee gain computer.
- [x] Four band modes — static offset, downward (cut above threshold), upward (boost above
      threshold) and upward-below (boost below threshold). Upward is the negated downward curve and
      upward-below evaluates the same computer at the threshold-mirrored level, so knee and ratio behave
      identically across all modes.
- [x] Per-band detection from a unity-gain bandpass of the sidechain (default) or wideband, with an
      external sidechain through `ProcessSampleSidechain`/`ProcessInPlaceSidechain`.
- [x] Control-rate coefficient redesign through the canonical `dsp/filter/design` designers
      (`SetUpdateInterval`, default 32 samples), overwriting coefficients in place so filter state
      survives; zero-allocation hot path.
- [x] `BandStaticCurve` reuses the Phase 31 `StaticCurveProcessor` plumbing.
- [x] Tests (static band gain vs detector level, mode directions, range clamp, band selectivity,
      multi-band interaction, determinism/reset, update-interval fidelity, zero-alloc) + benchmarks +
      runnable examples.

Exit criteria:

- [x] `go test -race ./dsp/effects/dynamics` passes; `BenchmarkDynamicEQ*` report 0 allocs/op.

### Phase 36: Pitch Correction (YIN) (Complete)

- [x] `YINDetector` (`dsp/effects/pitch/yin_detector.go`): squared difference function, cumulative
      mean normalized difference (`d'(0) := 1`), absolute threshold followed down to its local
      minimum, and parabolic interpolation. Frame-at-a-time, zero-allocation, with a frame RMS
      silence gate and a `PitchEstimate` that reports 0 Hz when unvoiced so a failed estimate can
      never be mistaken for a real one.
- [x] `PitchTracker` (`dsp/effects/pitch/pitch_tracker.go`): ring buffer + hop scheduling around the
      detector, plus a fixed-array median filter and an unvoiced hold. The median filter is the
      octave-error defence — a single stray frame cannot move the reported pitch.
- [x] `PitchCorrector` (`dsp/effects/pitch/pitch_corrector.go`): composes the tracker with any
      `PitchProcessor` (default `SpectralPitchShifter`). Scale or fixed-target modes, correction
      amount interpolated in the semitone domain, a clamped maximum correction, a confidence gate,
      an unvoiced hold, and a retune glide (`WithCorrectionSpeedMs`). Blocks are processed with a
      short lookahead and crossfaded at the seams. It deliberately does **not** implement
      `PitchProcessor`, since its ratio is derived rather than set.
- [x] Note/scale helpers (`dsp/effects/pitch/note.go`): `Scale` (comparable, bitmask-backed) with
      chromatic/major/natural-minor/harmonic-minor/pentatonic/blues/whole-tone constructors and
      `SnapMIDI` (ties resolve down), plus `FrequencyToMIDI`, `MIDIToFrequency`,
      `SemitonesToRatio`, `RatioToSemitones` and `CentsBetween`. The two shifters now use these
      instead of four inlined copies of the semitone formulas.
- [x] Tests (sine accuracy within 1 cent across a 55–1318 Hz grid at 44.1/48 kHz; sawtooth, square
      and missing-fundamental octave-error robustness; threshold-vs-octave trade-off; noise
      robustness at 20/10/3 dB SNR; unvoiced/silence; determinism; zero-alloc) + benchmarks + 6
      runnable examples.

> The `frequency_shifter` mentioned in the original scope is **not** used: `modulation.FrequencyShifter`
> is a Bode SSB shifter that translates every partial by a constant number of hertz, destroying
> harmonicity. Correction needs a ratio, not an offset. Documented in `dsp/effects/pitch/doc.go`.

> Not yet wired into `dsp/effectchain` or the web demo, matching the Phase 34/35 scoping. Tracked
> as demo coverage work in Phase 41b, not as a permanent exclusion. An FFT-accelerated difference
> function and pYIN are noted in `EFFECTS.md` as future refinements.

Exit criteria:

- [x] `go test -race ./dsp/effects/pitch` passes; `BenchmarkYINDetectorDetect` and
      `BenchmarkPitchTrackerWrite` report 0 allocs/op; detection is within 1 cent on the synthetic
      sine grid with no octave errors on harmonic-rich material.

### Phase 37: Noise Reduction (Planned)

- [ ] Noise-profile capture + spectral subtraction / Wiener filtering over an STFT (reuse the
      spectral-freeze STFT scaffolding).
- [ ] Tests (SNR improvement on profiled noise; musical-noise sanity) + runnable example.

### Phase 38: Interpolation Kernel Expansion (Planned)

Extend `dsp/interp` toward `legacy/Source/DSP/DAV_DspInterpolation.pas`, keeping deterministic,
allocation-free behavior.

- [ ] Add the remaining Hermite family (`Hermite1..3`) and B-spline kernels (4-point/3rd-order,
      6-point/5th-order) with documented formulas and stable edge semantics.
- [ ] Parity tests vs legacy formulas + reference vectors; smoothness/continuity tests across a
      fractional sweep.

### Phase 39: Interpolation Integration & Validation (Planned)

- [ ] Optional complex/interleaved interpolation helpers for spectral/complex pipelines
      (cf. `DAV_DspSpectrumInterpolation.pas`).
- [ ] Unify interpolation-mode selection across `dsp/delay`, `dsp/resample`, and effects call
      sites; expose low-level hot-path helpers where measured.
- [ ] Kernel quality-vs-CPU benchmarks; boundary tests (short buffers, wrap/clamp policies).

Exit criteria:

- [ ] Legacy-equivalent kernels available with tests/docs; callers select strategy explicitly.
- [ ] `go test -race ./dsp/interp ./dsp/delay ./dsp/resample` passes.

### Phase 40: Benchmark Regression Guard (In Progress)

Builds on the Phase 24 harness (`just bench-ci`, `BENCHMARKS.md`).

- [x] Define regression thresholds (`ns/op`, `allocs/op`) + a baseline-update workflow.
      `cmd/benchguard` (logic in `internal/benchguard`) parses `go test -bench` output and
      compares it against the checked-in `benchmarks/baseline.json`: `allocs/op` exact and
      `B/op` +10% gate; `ns/op` +50% is reported but does **not** gate unless
      `-enforce-timing` is passed and the baseline is from the current machine. The
      justfile gained `bench-guard` and `bench-baseline`, and `bench-ci` grew from 3 to 6
      packages (20 benchmarks, ~35 s at `-count=1`) with a `count` parameter.
- [x] Wire the guard into CI as advisory output (`.github/workflows/test-bench.yml`,
      `continue-on-error`, comparison table written to the job summary and the raw output
      uploaded as an artifact). It drives `just bench-ci` so the benchmark set cannot
      drift between CI and local runs.
- [ ] Re-run full benchmarks on ≥2 machines; refresh `BENCHMARKS.md`.
      **Blocked on hardware**: only `linux/amd64` (i7-1255U) was available, so the
      baseline and the `BENCHMARKS.md` prose numbers come from that one machine. The
      format is already multi-machine-ready — a baseline records `goVersion`/`goos`/
      `goarch`/`cpu`, and a mismatch downgrades timing to advisory automatically.

> Treating timing as non-gating is a measured conclusion, not caution. With no code
> change at all: three repeat runs on an idle machine at `-count=1` moved the
> sub-microsecond spectrum benchmarks by up to 43%; a `-count=3` run against a
> `-count=5` baseline on the _same_ machine still put five benchmarks past a 50% bound
> (a sustained sweep throttles the CPU); and under real desktop load individual entries
> moved 7x. In that last run the guard correctly reported **no regressions**, because
> every allocation column held steady. No `ns/op` threshold is both loose enough to
> survive that and tight enough to be useful, so allocations — deterministic and
> machine-independent — are what gate. `benchguard` also keeps the minimum across
> `-count=N` repeats to suppress what noise it can.
>
> Consequence for the baseline: its `ns/op` figures were captured on a loaded machine
> and are provisional (see the note in `BENCHMARKS.md`); its `B/op` / `allocs/op`
> columns are load-independent and correct.

Exit criteria:

- [x] Hot paths show no major allocations/op regressions (`just bench-guard` is clean
      against the recorded baseline).
- [x] `go test ./...` and `go test -tags purego ./...` pass.
- [ ] `BENCHMARKS.md` baselines updated (date + Go version + machine). Guard thresholds,
      the update workflow and the measured noise floor are documented; the ≥2-machine
      refresh of the prose numbers is still outstanding.

### Phase 41: SIMD Modal Oscillator Bank (Planned)

Optional; uses the already-present `algo-vecmath v0.1.0` dependency.

- [ ] `dsp/osc` (or `dsp/modal`) package skeleton with a scalar reference.
- [ ] Block APIs for damped complex rotators (primary `float32`).
- [ ] Parity tests vs the scalar reference + modal-workload microbenchmarks.
- [ ] Document the denormal strategy (cf. `core.FlushDenormals`).

### Phase 41b: Web Demo — Purpose, Hardening, Coverage (In Progress)

The web demo (`web/` + `internal/webdemo/`) has absorbed ~42% of the repo's commits without ever
having a phase, an owner, or exit criteria. This phase gives it one.

**Purpose statement.** The web demo is the **showcase and integration test for the public API**.
Every package under `dsp/`, `measure/`, and `stats/` either appears in the demo or is explicitly
listed here as out of scope, with a reason. It is app-layer code and stays out of the library, but
it is no longer optional: if a phase adds a user-visible capability, wiring it into the demo is part
of that phase's work, not a separate favour.

**Baseline coverage at the start of this phase — 13 of ~31 packages.** Exercised: `dsp/effectchain`,
`dsp/effects` (+ `dynamics`, `modulation`, `pitch`, `reverb`, `spatial`), `dsp/filter/biquad`,
`dsp/filter/design` (+ `band`, `shelving`), `dsp/window`. Not exercised: `dsp/conv`, `dsp/dither`,
`dsp/filter/bank`, `dsp/filter/fir`, `dsp/filter/hilbert`, `dsp/filter/weighting`, `dsp/resample`,
`dsp/signal`, `dsp/spectrum`, and **all of `measure/*` and `stats/*`**.

- [x] Stage 0: this entry; retire the blanket "not wired into the web demo" de-scoping notes.
- [x] Stage 1: link the library from the demo (repo, pkg.go.dev, roadmap, license); rewrite the
      stale `web/README.md`; `recover()` at the JS boundary; validate persisted settings against
      their declared types; a visible error/status region plus `<noscript>` and a guarded init
      sequence; debounce the per-pointermove save; skip redundant graph reloads in `SetEffects`;
      drop the duplicate `web/irs.irlib`; add `-ldflags="-s -w" -trimpath` to the WASM build.
- [x] Stage 2: PR-time `GOOS=js GOARCH=wasm` vet+build (`.github/workflows/test-web.yml`),
      prettier/ESLint over `*.js,*.html,*.css` via `treefmt` and `package.json`, and an 11-case
      Playwright smoke suite (`web/test/demo.spec.js`). `just web-check` runs all of it and is
      wired into `just ci`.
- [x] Stage 3 (partial): single `js.CopyBytesToJS` per audio block against persistent buffers;
      theme tokens cached instead of re-read from the CSSOM ~12x per frame; the analyser redraw
      loop now runs only while audio is playing and the tab is visible (was 24 fps forever);
      `ResizeObserver` instead of a forced layout per chain-canvas frame; device-pixel-ratio
      handling for the dynamics and Chebyshev plots; the step highlight now mirrors the Go
      engine's clock instead of running a second `setInterval` clock that drifted.
- [ ] Stage 3 (blocked): `AudioWorklet` instead of `ScriptProcessorNode`.

> **Why the `AudioWorklet` migration is blocked, not merely pending.** Both routes are closed on
> the current deployment:
>
> 1. _Go runtime inside `AudioWorkletGlobalScope`._ `wasm_exec.js` requires `crypto`,
>    `performance`, `TextEncoder`/`TextDecoder`, and `setTimeout` — the last backs
>    `runtime.scheduleTimeoutEvent`, which is how the Go scheduler runs timers.
>    `AudioWorkletGlobalScope` provides none of them.
> 2. _Worker + `SharedArrayBuffer` ring buffer with a thin worklet._ `SharedArrayBuffer` requires
>    cross-origin isolation, i.e. COOP/COEP response headers. GitHub Pages cannot set custom
>    headers, and the deployed site sends neither (verified).
>
> The viable path is a `coi-serviceworker`-style service worker that synthesises COOP/COEP,
> then route 2. That is a deployment change with its own failure modes and should be decided
> deliberately rather than folded into a hardening pass. Until then the demo keeps
> `ScriptProcessorNode`; the main-thread pressure that made it audible (a synchronous
> `localStorage` write and a full graph rebuild per pointermove, plus a permanent 24 fps redraw
> loop) has been removed.

Deferred to a later phase (direction agreed, not in scope here): collapsing the four parallel
effect-parameter tables by generating the JS side from the Go structs; ES modules; promoting
`internal/webdemo/eq.go` into `dsp/filter/design` and `spectrum.go` into `dsp/spectrum`; deleting
the duplicated `effects_chain_configure.go` and the legacy serial chain; surfacing `measure/*` and
`stats/*`.

Exit criteria:

- [x] A corrupted `algo-dsp-settings` entry in `localStorage` cannot prevent the demo from starting.
- [x] A failed WASM load produces a visible message rather than an inert UI.
- [x] `web/wasm` is compiled by PR CI; `.js`/`.html`/`.css` are format-checked by `just ci`.
- [x] The analyser redraw loop is idle when no audio is playing.
- [ ] No audio dropouts while dragging a canvas slider (DevTools Performance trace) — the known
      causes are fixed, but this has not been confirmed on a real trace.
- [ ] Stereo output. Deferred: the engine is mono end to end (`Render([]float32)`, and
      `dsp/effectchain` processes a mono block), and the widener/rotary nodes deliberately fold
      down to mono. Widening the output channel count alone would change nothing audible; real
      stereo means threading it through `dsp/effectchain`, which is library work and needs its
      own phase.

### Phase 42: Release Readiness (v1.0) (Planned)

- [ ] Full benchmark pass; confirm no major regressions vs baselines.
- [ ] Full local CI (`just ci`) including race (`go test -race ./...`).
- [ ] Finalize `CHANGELOG.md` and the placeholder `MIGRATION.md`; create the missing
      `API_REVIEW.md` and complete its checklist for `v1.0.0`.

### Phase 43: Tag and Publish v1.0 (Planned)

- [ ] Tag and publish `v1.0.0` (tag + release notes), advancing from the current `v0.5.1`.
- [ ] Verify module-proxy indexing (`go get` via `GOPROXY`).

Exit criteria:

- [ ] `v1.0.0` tag exists and release notes are published.

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
- Gate regressions with benchmark trend checks in CI (non-blocking initially, blocking by v1.0 if desired).

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

1. Windows
2. Filter runtime + design + weighting/banks
3. Spectrum/conv/resample helpers
4. Measurement kernels + stats

### E.2 Migration Mechanics

- Keep APIs adapter-friendly during extraction.
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

| Version | Date       | Author  | Changes                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                |
| ------- | ---------- | ------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 0.1     | 2026-02-06 | Codex   | Initial comprehensive plan                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                             |
| 0.2     | 2026-02-06 | Claude  | Expanded early phases + migration notes                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                |
| 0.3     | 2026-02-08 | Claude  | Added shelving filter design phase + known Chebyshev II bug                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            |
| 0.4     | 2026-02-20 | Copilot | Restored detailed plan + added checkable tasks for all phases                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                          |
| 0.5     | 2026-06-21 | Claude  | Status refresh (Phases 15/18 complete, 16/23 progress, Chebyshev II fixed); implemented Phase 27 Goertzel tone analysis                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                |
| 0.6     | 2026-06-21 | Claude  | Implemented Phase 26 legacy-faithful Moog ladder core (`dsp/filter/moog`); paper-or-better track deferred, phase now In Progress                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                       |
| 0.7     | 2026-06-21 | Claude  | Completed Phase 26: added oversampled high-quality Moog path (anti-aliasing + half-sample feedback compensation) and nonlinear characterization tests; phase Complete                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                  |
| 0.8     | 2026-06-21 | Claude  | Ported Phase 29 (dither/noise shaping) and recovered Phase 30 (polyphase Hilbert) onto `main` from the orphaned release lineage; both phases Complete. See history-divergence note below.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                              |
| 0.9     | 2026-06-21 | Claude  | Recovered Phase 28 (EBU R128 loudness) onto `main`; recovered the stranded effects (granular, spectral-freeze, vocoder, rotary speaker, frequency shifter, convolution reverb) + partitioned convolution; adopted the release-line Moog (regaining VariantZDF/Newton); recovered the `dsp/effectchain` subsystem (with Delay/Distortion superset upgrades).                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            |
| 0.10    | 2026-06-21 | Claude  | Status refresh after the recovery: Phase 21 (convolution reverb done, Haas pending) and Phase 22 (spectral-freeze/granular/vocoder done; dynamic-EQ/panner/pitch-correction/noise-reduction pending) moved Planned → In Progress with their done items checked; refreshed the Phase 26 Moog snapshot to describe the adopted six-variant release-line filter (incl. `VariantZDF`); swapped the web demo to the `dsp/effectchain`-driven architecture + IR library (PR #14).                                                                                                                                                                                                                                                                                                                                                                                                                                            |
| 0.11    | 2026-06-21 | Claude  | Completed Phase 21: implemented the `HaasDelay` precedence effect (`dsp/effects/spatial`, reusing `monoDelay`) with tests/example/benchmarks, and added the missing convolution-reverb tests/example; snapshot now credits the full reverb suite (Convolution + FDN + Freeverb) plus Haas. Phase Complete.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                             |
| 0.12    | 2026-06-21 | Claude  | Condensed all completed phases (15, 17–21, 26–30) to compact summaries; split the oversized undone phases into focused sub-phases — Phase 22 → 22.1–22.5 (vocoder finalize, panner, dynamic EQ, YIN pitch correction, noise reduction), Phase 24 → 24.1/24.2 (regression guard / SIMD modal track), Phase 25 → 25.1/25.2 (readiness / release), Phase 31 → 31.1/31.2 (kernels / integration); slimmed Phases 16 & 23 to done-summary + remaining item; refreshed Phase Overview to match.                                                                                                                                                                                                                                                                                                                                                                                                                              |
| 0.13    | 2026-06-21 | Claude  | Reordered into completed-then-remaining and re-applied **strict integer numbering** (no `x.y` sub-phases). Completed phases come first (old 26–31 shifted to 25–30); partial phases 16/22/23/24 are now scoped to shipped work, with their open follow-ups split into standalone phases. Remaining roadmap is Phases 31–43 in execution order, ending with v1.0 (P42–P43). Open phases refined with concrete file paths / API hooks from a codebase audit (interpolation core found already complete → P30; dynamics static-curve path via `GainForLevel`/`CalculateOutputLevel`; elliptic reuse of `internal/ellipticmath`; `algo-vecmath` already a dependency; `API_REVIEW.md` still missing). **Note:** revision entries 0.1–0.12 reference the pre-0.13 phase numbers.                                                                                                                                            |
| 0.14    | 2026-07-29 | Claude  | Completed Phase 34: added `StereoPanner` (`dsp/effects/spatial/stereo_panner.go`) with three selectable pan laws (equal-power/compromise/linear), mono-pan and attenuate-only stereo-balance modes, and an optional auto-pan LFO; tests, 4 runnable examples and benchmarks included. Effect-chain/web-demo wiring deliberately left out of scope.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                     |
| 0.15    | 2026-07-29 | Claude  | Completed Phase 32: extracted the Orfanidis parametric-EQ prototypes into `internal/orfanidis` (shared with `dsp/filter/design/band`), added `Elliptic{Low,High}Shelf` for orders >= 1, and rebuilt `Chebyshev2{Low,High}Shelf` as a genuine equiripple Type II design — the previous version delegated to a Butterworth shelf. All four shelving families now share the `\|H(f_c)\|² = (G²+1)/2` cutoff convention. Dead `chebyshev2Sections` (empirical damping constants) and `invertSections` removed; examples and benchmarks added. Breaking coefficient change for Chebyshev II, see CHANGELOG.md.                                                                                                                                                                                                                                                                                                              |
| 0.16    | 2026-07-29 | Claude  | Completed Phase 36: added the YIN pitch detector (`YINDetector`), the streaming `PitchTracker` with median/hold smoothing, the auto-tune `PitchCorrector`, and the `Scale`/note-conversion helpers — all in `dsp/effects/pitch`; the two existing shifters now share the new semitone conversions. Recorded the decision not to use `modulation.FrequencyShifter` for correction (it breaks harmonicity). Effect-chain/web-demo wiring and an FFT difference function deliberately left out of scope.                                                                                                                                                                                                                                                                                                                                                                                                                  |
| 0.17    | 2026-07-29 | Claude  | Completed Phase 33: added `dsp/effects/vocoder_example_test.go` (defaults, `ProcessBlock` envelope transfer, Bark layout with a synthesis-Q override, multirate downsampling) — the vocoder was the last effect in `dsp/effects` without runnable examples — and closed the reachable coverage gaps so every exported vocoder option/getter/setter is at 100%. Corrected the phase's stale `NewVocoder(sampleRate, bandLayout, opts...)` signature to the real `NewVocoder(sampleRate, opts...)` + `WithBandLayout`, and recorded the `WithDownsampling` multirate feature the phase text had omitted. No API change.                                                                                                                                                                                                                                                                                                  |
| 0.18    | 2026-07-29 | Claude  | Phase 40 partially completed: added `internal/benchguard` + `cmd/benchguard`, a benchmark regression guard that diffs `go test -bench` output against the checked-in `benchmarks/baseline.json` (`allocs/op` exact and `B/op` +10% gate; `ns/op` +50% is reported but non-gating unless `-enforce-timing` is passed on quiet hardware). Broadened `just bench-ci` from 3 to 6 packages (20 benchmarks) with a `count` parameter, added `just bench-guard` / `just bench-baseline`, and wired an advisory `Benchmark Guard` CI job that drives the same justfile recipe. Timing was demoted to non-gating after measurement: repeat runs with no code change moved benchmarks 43% on an idle machine and up to 7x under load, while allocation columns held steady throughout. The remaining item — refreshing `BENCHMARKS.md` from >=2 machines — is blocked on hardware availability, so the phase stays In Progress. |

---

### Repository history note: disjoint lineages

As of 2026-06-21, this repository contains **two unrelated Git histories with no common
ancestor** (the root `initial commit` differs: `main` roots at `7639b95`, the release line at
`b3d2887`, both stamped `2026-02-06 15:23:50`). At some point `main` was re-initialised,
orphaning the original development line.

- **`main`** (root `7639b95`) originally carried only the recent Moog / Goertzel work and
  lacked the v0.2–v0.5 release content.
- **Release lineage** (tags `v0.2.0`–`v0.5.1`, root `b3d2887`) and the `claude/*` branches
  contained the dither, Hilbert, loudness, `effectchain`, and the full v0.2–v0.5 effect set,
  but none of `main`'s recent Moog/Goertzel work.

**Decision:** `main` is the source of truth; release-line work is forward-ported feature by
feature (file-grab / cherry-pick), **not** merged — a cross-history `git merge` is
inappropriate here.

**Recovered onto `main`** (this reconciliation pass): Phase 28 (loudness), Phase 29 (dither),
Phase 30 (Hilbert); the stranded effects (granular, spectral-freeze, vocoder, rotary speaker,
frequency shifter, convolution reverb) + `dsp/conv` partitioned convolution; the release-line
Moog (a functional superset — regained `VariantZDF`/Newton; main's reduced reimplementation was
replaced, no callers broke); and the `dsp/effectchain` subsystem (which required upgrading
`effects.Delay` and `effects.Distortion` to their release-line superset versions).

**Goertzel reconciliation (done):** the two Goertzel implementations (independent reimplementations
on each lineage) were fused into a single canonical `dsp/spectrum/goertzel.go` — `main`'s richer
API (options/configurable `DB` floor, `Complex`, `NormalizedMagnitude`, allocation-free
`GoertzelBank.Powers/Magnitudes(dst)`, pass-through `ProcessSample`, strict validation) as the
base, plus the release line's faster register-hoisted `ProcessBlock` and its one-shot
`AnalyzeBlock` helper.

**Web demo (PR #14):** the demo was swapped from its webdemo-local effect chain to the
`dsp/effectchain`-driven architecture (adapter + configure) and gained the IR library
(`irlib.go` + embedded data) so the recovered convolution reverb is usable. This is app-layer
(`internal/webdemo` + `web/`), not library code, and needs browser validation in review.

**Status:** a full file-level audit (`v0.5.1` vs `main`) shows **no DSP library source remains
stranded** — every `dsp/`, `measure/`, and `stats/` file is on `main`. The only release-line
files not on `main` are app-layer (the webdemo glue handled by PR #14) and tooling. The orphan
`claude/*` branches carry no unique library work (older flat-layout duplicates, preserved in
tags `v0.2.0`–`v0.5.1`) and can be archived.

**Net-new (not recovery):** the still-open Phase 21/22 items — Haas delay, dynamic EQ, stereo
panner, pitch correction (YIN), noise reduction — were never implemented on either lineage and
remain genuine future work.

---

This plan is a living document and should be updated after each phase completion and major architectural decision.
