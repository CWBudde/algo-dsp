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
6. Detailed Phase Plan (Phases 0–46)
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
Phase 27: Loudness Metering (EBU R128 / BS.1770)           [3 weeks]  ◐ Integrated complete; live compliance pending
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
Phase 41b: Web Demo — Purpose, Hardening, Coverage         [2 weeks]  🔄 In Progress
Phase 41c: SIMD Adoption in Existing Hot Paths             [1 week]   🔄 In Progress
Phase 42: Release Readiness (v1.0)                         [1 week]   📋 Planned
Phase 43: Tag and Publish v1.0                             [0.5 week] 📋 Planned

# Post-v1.0
Phase 44: Music Analysis & Source Separation (Demucs port)  [6-8 weeks] 🔄 In Progress
Phase 45: Music Structure & Harmony (story layer port)      [3-4 weeks] ✅ Complete
Phase 46: Constant-Q Transform (basic-pitch front end)      [1-2 weeks] 🔄 In Progress
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

### Phase 27: Loudness Metering (Integrated complete; live compliance pending)

- [x] Legacy `measure/loudness.Meter`, recovered from the release lineage (Appendix H),
      retains approximate K-weighting, zero-padded startup integration, 400 ms momentary
      and 3 s short-term readings for compatibility. Existing tests characterize that
      implementation, not full EBU conformance. `Peaks` tracks sample peaks, not true peaks.
- [x] Additive `IntegratedAnalyzer` in `integrated.go` / `k_weighting.go` uses the published
      BS.1770-5 48 kHz coefficients and inverse-bilinear rate mapping, complete 400 ms windows
      with 100 ms hops, absolute/relative gates, explicit copied channel weights and sample
      peaks. Planar float32/float64 blocks and incremental finalization are bounded and
      allocation-free; configuration caps workspace at 64 MiB. Invalid input is atomically
      rejected; arithmetic overflow is terminal until reusable `Reset`.
- [x] `normalize.go`: `PlanNormalization` and fresh-output `NormalizeLoudness` apply one
      linked input-derived gain through tagged `algo-vecmath`, without source mutation,
      clipping, layout inference or a false unconditional post-gain LUFS guarantee.
- [x] `target.go` / `target_finish.go`: additive `TargetAnalyzer` retains positive
      below-gate windows and cooperatively solves re-gated target intervals, including
      tied energies and multiple solutions. Cached channel-major filters and four
      positive hop sums remove full-window rescans; original analyzer outputs stay
      unchanged. `target_measurement.go` provides incremental measurement-only
      finalization for actual rounded candidate verification, and progressive finite
      `SamplePeak`. No float32 accuracy certificate is claimed: plans require a fresh
      output measurement. `dsp/signal/peak_normalization.go` derives unclamped linked
      peak gains, including silence and subnormal positive sources. Independent DF1 /
      static/rate/float32/interval tests, atomic ownership/error/reset/allocation tests,
      examples and complete ten-minute scan/finalization benchmarks accompany them.
- [x] Streamed EBU Tech 3341 integrated cases 1–6, independent DF1/gate static golden,
      rate/chunk/EOF/weight tests (including accumulated fractional-frame endpoints),
      safety/reset/ownership/allocation regressions, runnable examples and ten-minute
      stereo benchmarks. Native CI, full race suite, native/WASM vet, 12 browser demo
      tests and actual Node/V8 WASM loudness tests pass; package coverage is 95.8%.
      Analyzer-only ten-minute timing is 0.45–0.56 s native / 1.25–1.30 s WASM,
      with 0 B/op and 0 allocs/op; this is not an editor end-to-end acceptance claim.
- [ ] Standards-compliant live momentary/short-term metering, oversampled true peak,
      loudness range (LRA), and callback/event-hook API remain unimplemented. The integrated
      analyzer alone does not establish full EBU Mode conformance.

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

### Phase 37: Noise Reduction — Complete (2026-10-04)

- [x] `dsp/effects/restoration` captures mean bin powers and applies Wiener,
      subtraction or profile-gate gains through the public bounded STFT APIs.
      `SpectralProcessor.Step` supports rectangle/polygon attenuation, removal and
      short-gap autoregressive healing; click/clip repair and harmonic hum removal
      are reusable DSP primitives. Legacy lineage and intentional fixes are in
      `docs/restoration.md`.
- [x] `restoration_test.go` and `validation_test.go` cover stationary-noise
      suppression >=15 dB, wanted-tone preservation, residual-power variation and
      isolated-line musical-noise proxies, selective-bin editing, click residual
      below -80 dBFS, hum rejection, interpolation and rejection without mutation.
      Examples and zero-allocation reducer/hum benchmarks accompany the API.
- [x] `pitch/time_stretch.go` exposes the private WSOLA stage and adds a bounded
      stereo-coherent stream; tests cover exact duration, retained pitch,
      ratio-one identity and stereo phase. Consumed by algo-audio-editor Phase 8.

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

### Phase 41c: SIMD Adoption in Existing Hot Paths (In Progress)

Distinct from Phase 41, which adds a _new_ modal oscillator package. This phase vectorizes
loops that already exist and already run in production paths. It came out of the 2026-08-15
`algo-vecmath` arm64 round, which rewrote the NEON backend (nine of ten kernels had been
2x-unrolled _scalar_ loops living in a package called `neon`) and lifted `DotProduct` by
3.0x-3.9x and `Sum` by 2.1x-2.5x on an Apple M5. `dsp/filter/fir` and `stats/*` inherited
that for free; the items below are what a survey of this repo found still on the table.

The first entry is the substantial one; the rest are progressively cheaper.

- [ ] **`MulComplexBlock` over `[]complex128`** — the strongest remaining candidate, and the
      one that needs a new `algo-vecmath` primitive (tracked there; see that repo's PLAN.md).
      The same `dst[i] = a[i] * b[i]` complex multiply appears at **8 sites**, including both
      streaming convolvers and `dsp/conv/partitioned.go:150` / `:166` — the hottest loop in
      the repo for long-IR convolution and the lowest-latency real-time path. Go compiles a
      complex multiply to 4 multiplies and 2 adds with no vectorization. **No data-layout
      change is needed anywhere**: `[]complex128` is already bit-identical to interleaved
      `[]float64`, and `dsp/conv/streaming.go:104-138` already reinterprets it via
      `unsafe.Slice`. Needs a `complex64` twin for the generic streaming paths.
- [ ] **`SumSquaredDiff` for the YIN difference function** (`dsp/effects/pitch/yin_detector.go:465-475`)
      — roughly 640k FLOPs per frame, the densest scalar loop in the repo. Also needs a new
      `algo-vecmath` primitive.
- [x] **Call primitives that already exist and are simply not used.** Done: `AddBlockInPlace`
      in both `dsp/conv/partitioned.go` overlap-add loops, the `dsp/conv/streaming_overlap_add.go`
      tail merge (a site this list missed), `dsp/effects/dynamics/multiband.go` and
      `dsp/effectchain/chain_process.go`; `ScaleBlock` for that file's output scaling; and
      `ScaleBlockInPlace` in `measure/sweep.LogSweep.InverseFilter`. All bit-identical on amd64
      and arm64 by construction — element-wise ops have nothing to fuse or reassociate. The
      partitioned-conv, multiband and sweep benchmarks show no significant change, because the
      substituted loop is a small fraction of each.

  **`MaxAbs` is excluded and must stay excluded until `algo-vecmath` is fixed.** It is 3.0x
  faster in `stats/time.Peak` and 5.6x in `measure/ir`'s onset search, and unsafe in both: on
  AVX2 a NaN anywhere in the slice can discard the true maximum and return a smaller **finite**
  value — `MaxAbs([999, NaN, 0.5])` returns `0.5` on AVX2 and `999` in pure Go. That is silent
  corruption of the finite result, not a NaN-policy difference, and an `IsNaN` check on the
  result does not catch it, because the result is finite. Detecting NaN up front costs a full
  pass, which is the entire speedup. Both sites keep their scalar loops, and
  `TestPeakNaNIsDeterministic` / `TestFindImpulseStartNaNIsDeterministic` fail on an AVX2 machine
  if anyone re-adopts it. Action for `algo-vecmath`: `MaxAbs` should either skip NaN or propagate
  it, consistently across kernels — returning an arbitrary finite non-maximum is defensible under
  no policy. Found by review on #25.

  Three corrections to this list, for whoever reads it next.

  **One: `dsp/effectchain/chain.go:62` has no scale loop and never did.** Line 62 was `LoadGraph`
  already in `5606165`, the commit that wrote this phase. The only scale-shaped site in the
  package is the one in `chain_process.go`, counted above.

  **Two: these were not one-line substitutions.** `dsp/conv`'s loops live in `partStageT[F, C]`,
  generic over `float32`/`float64`, and `algo-vecmath` is float64-only; they needed a helper
  dispatching on `unsafe.Sizeof`, per the precedent in `dsp/conv/streaming.go`. The `float32`
  instantiations keep the scalar loop.

  **Three: unguarded, these are regressions at small N.** A dispatched vecmath call costs ~110 ns
  on amd64 regardless of length, so the four-parent mix loses 0.72x at a 16-sample block and only
  turns over to 3.0x at 512. Every site is now behind a benchmarked length threshold of 64, in
  the style of `conv.simdThreshold`. Assume the same is true of items 1, 2 and 4 below — and, per
  the `MaxAbs` finding above, never assume a vecmath kernel is correct on non-finite input just
  because it is fast.

- [ ] **Interleaved `Magnitude` / `Power` consuming `[]complex128` directly**, letting
      `dsp/spectrum` delete its deinterleave scratch pool and one whole memory pass.
- [x] **Bin-parallel Goertzel.** Done: `GoertzelBank.ProcessBlock` advances four bins per pass
      over the sample buffer, with bins past the last full group falling through to the per-bin
      path. 3.8x for four bins and 4.2x for the eight-bin DTMF case over a 1024-sample block;
      one to three bins are unchanged. Bit-identical per bin, pinned by a `==` comparison in
      `TestGoertzelBankProcessBlockBitExact`. The win is latency hiding, not width — the
      single-bin loop is bound on the multiply-add dependency chain, so four independent chains
      fill the stalls. The recurrence now lives in one `goertzelStep` helper shared by all three
      paths so they cannot diverge in how the compiler contracts the multiply-add.

> **Confirmed non-starters, so nobody re-surveys them:** biquad/IIR and Hilbert recursions,
> the Moog ladder, envelope followers, dither noise shaping, and phase unwrap are all serial
> by construction. Kahan and Welford accumulation are vectorizable only by changing the
> documented numerics, which is not a trade this library should make silently.

> **Precedent set by the conv change (already landed, `d2ec9ef`).** `conv.DirectTo` moved from
> the two-pass `ScaleBlock` + `AddBlockInPlace` idiom to the single fused
> `vecmath.AddScaledBlockInPlace`, worth 1.26x-2.28x on `BenchmarkDirect` for 32- and 64-tap
> kernels. **That change is bit-identical on amd64 but not on arm64**, where the NEON AXPY
> fuses the multiply-add that the two-pass form rounded twice. Every item above carries the
> same hazard: check it against real fixtures rather than assuming, and state the answer in
> the CHANGELOG either way.

> **Do not size these against the `*Ref` / `*Generic` benchmarks.** Those call test-local
> copies that get inlined while a dispatched call cannot be, so the comparison flatters the
> generic side. Measure against `-tags purego`, and keep an untouched kernel in the same run
> as a control — if it does not read ~1.00x, nothing in that run is trustworthy.

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

### Phase 44: Music Analysis & Source Separation (In Progress, post-v1.0)

Status: Workstreams A, B (plus the B extension) and C landed in `dsp/stft`, `dsp/separate`,
`dsp/resample`, `dsp/core` and `measure/music/*`; D and E's Demucs items remain open.

Ports the two Python steps of the `AudioVisualizer` analysis pipeline
(`github.com/cwbudde/AudioVisualizer`, `scripts/separate.py` and `scripts/plot_analysis.py`) to
Go, so that pipeline no longer needs a Python/PyTorch venv. The Go half of that pipeline
(`internal/audioanalysis`) already consumes `dsp/window`, `dsp/resample`, `stats/frequency` and
`algo-fft`; the reusable parts of it move here as well (Workstream B). Scheduled after v1.0
because nothing in it blocks the release, and because Workstream D is by far the largest single
item in this plan.

**What the two scripts actually do — read this before scoping anything:**

- `plot_analysis.py` does **no signal analysis at all**. It loads the precomputed
  `analysis/features.json` and draws a six-panel matplotlib PNG: per track (mix, drums, bass,
  other, vocals), the RMS curve and five band envelopes in dBFS (`20·log10(max(x, 1e-6))`,
  y-range −85…0), shaded silence intervals and cue lines; then a 64-bin log-frequency
  spectrogram of the mix (25 Hz–12 kHz, `magma`, −75…−10 dB). Drawing a figure is a §1.3
  non-goal ("GUI/visualization components"), so **the renderer does not move into this
  library**. What moves here are the data products it plots, all of which are currently computed
  in `AudioVisualizer/internal/audioanalysis/features.go`.
- `separate.py` runs **Demucs v4 (`htdemucs`, the Hybrid Transformer Demucs)** through PyTorch
  to split a stereo mix into `drums`, `bass`, `other`, `vocals`, and records provenance in
  `analysis/separation.json`. Porting it means writing a neural-network inference engine, not a
  DSP algorithm. Workstream C adds a classical (weight-free) separation baseline, and
  Workstream D is the actual Demucs port.

#### Decision gate (resolve before starting Workstream D)

- [ ] **Where the neural inference lives.** §1.2/§2.2 define this library as algorithm-only with
      minimal dependencies. A Demucs port adds a tensor/layer runtime (conv, attention, norms,
      GEMM) with a 42M-parameter model, which is a different kind of code from anything else
      here. Options: (a) `dsp/separate/htdemucs` in this module, with weights supplied by the
      caller as an `io.Reader` (never embedded, never fetched); (b) a separate module (e.g.
      `algo-demucs`) that depends on `algo-dsp` for STFT, resampling and windows. **Lean: (b)**,
      with Workstreams A–C landing here either way, since they are what (b) would import.
      Record the decision here and in §2.1.
- [ ] **Weights licence.** The Demucs code is MIT. Before anything ships beyond local use,
      confirm the licence terms of the pretrained `htdemucs` checkpoint; this plan assumes users
      download it themselves and the repo never redistributes it.

#### Workstream A: STFT / ISTFT primitive (`dsp/stft`)

Needed by B, C and D, and also wanted by Phase 37 (noise reduction) and already open-coded in
`dsp/effects/spectral_freeze.go` and `dsp/effects/pitch/pitch_shift_spectral.go`.

- [x] Frame-wise forward STFT over a whole buffer: window from
      `dsp/window`, configurable `nfft`/hop, `center` framing with selectable padding
      (`zero` as `audioanalysis` uses today, `reflect` as `torch.stft` uses in Demucs), and
      optional orthonormal scaling (`torch.stft(normalized=True)` divides by `sqrt(nfft)`).
      Done as `stft.New`/`New32` with `WithWindow`, `WithCenter(PadNone|PadZero|PadReflect)`,
      `WithNormalized`; `FrameInto` is the zero-alloc per-frame building block.
- [ ] A separate streaming STFT processor (push samples, pull frames).
- [x] ISTFT with window-sum-squared normalization (not a COLA assumption), so non-COLA
      window/hop pairs still reconstruct; report an error on hops where the window sum has zeros.
- [x] Both `float64` and `float32` (`algo-fft` plans are generic; D runs in `float32`).
- [x] Tests: perfect reconstruction (Hann periodic, hop = nfft/4, error < 1e-12 for float64),
      non-COLA pairs, Parseval for unitary scaling, zero-alloc `FrameInto` benchmark.
- [ ] Golden vectors exported from `torch.stft`/`torch.istft` for the exact Demucs settings
      (`nfft=4096`, `hop=1024`, Hann, reflect, normalized); zero-alloc streaming benchmark.
- [ ] Migrate `SpectralFreeze` and the spectral pitch shifter onto it, keeping their output
      bit-identical (or document the difference).

#### Workstream B: Music-analysis features (`measure/music/{features,onset,rhythm,align}`)

Lift the generic parts of `AudioVisualizer/internal/audioanalysis` here. Current parameters
used there, which become defaults rather than constants: analysis rate 24 kHz, `FFTSize = 2048`,
`Hop = 240` (10 ms, frame `i` centred at `i·hop`, first frame zero-padded), band edges
`{25, 140, 400, 2000, 6000, 12000}` Hz.

Name decision: one `measure/music` tree with a package per concern, so callers import only
what they use. Every package carries a parity test against a verbatim copy of the app code and
reproduces it bit for bit with the default configuration.

- [x] **Per-frame feature extractor** producing RMS and peak (over `center ± hop`, all
      channels pooled), spectral centroid (via `stats/frequency.Centroid`), stereo width
      `side/(mid+side)` with `mid=(L+R)²/4`, `side=(L−R)²/4`, and positive spectral flux on
      `log1p(|X|)`. Channel spectra are combined by **averaging power**, not by summing the
      signals, so out-of-phase content is not cancelled. Centroid and flux are forced to 0 below
      an RMS gate (1e-4 today), and the gate is an option.
- [x] **Band envelopes**: RMS amplitude per band with one-sided power scaling
      `2·|X|²/(N·Σw²)`, so a full-scale sine in a band reads ≈ its RMS. Bins are assigned by
      `edge[b] ≤ f < edge[b+1]`.
- [x] **Log-frequency spectrogram** (frame-major, `bins` log-spaced between `fmin`/`fmax`, in
      dBFS). **Known defect to fix during the port, not carry over:** today each FFT bin is
      assigned to `floor(log(f/25)/log(12000/25)·64)`. At 24 kHz / 2048 points the FFT bin
      spacing is 11.7 Hz, while the lowest log bins are only ~2.5 Hz wide, so the bottom of the
      spectrogram has empty bins (drawn as −120 dB stripes) and others with exactly one FFT bin.
      Use triangular or energy-preserving interpolation of FFT bins onto the log grid (as in a
      constant-Q or mel filterbank) and test that a swept sine produces no empty rows.
      Also export the frequency of each log bin, so a renderer can label its axis without
      re-deriving the mapping (`plot_analysis.py` re-implements it for its tick labels).
- [x] **dB helper** with an explicit floor (`20·log10(max(x, floor))`, floor 1e-6 → −120 dB);
      check whether `dsp/core` already has one before adding it.
- [x] **Envelope normalizer** (`Normalize` today): scale to the 95th percentile of
      above-gate values, clamp to [0, 1], then a one-pole attack/release smoother with
      `α = 1 − exp(−hop/(fs·τ))`. Check whether `dsp/effects/dynamics` already has the smoother
      before writing a new one.
- [x] **Onset detector** (`DetectOnsets` today): adaptive spectral-novelty peak picking (local
      maximum, ≥ 12% of the 95th-percentile flux, ≥ 1.35× the ±25-frame mean, RMS gate), then
      the onset time is refined to the largest causal 5 ms RMS rise within ±50 ms, and events
      closer than 75 ms are de-duplicated, keeping the stronger one. Document that events are
      observations, not instrument labels.
- [x] **Tempo / beat grid** (`EstimateRhythm` today): novelty = flux minus a ±30-frame moving
      mean, half-wave rectified; tempo score = normalized autocorrelation at lags
      `{1, 2, 4, 8}` × beat period with linear fractional-lag interpolation; broad scan
      60–180 BPM in 0.5 BPM steps, with candidates closer than 2 BPM merged (top 6 kept); optional
      caller-supplied prior refined ±3 BPM in 0.01 steps; beat phase fitted in 1 ms steps to the
      positive changes of the lowest band (kick-like energy). The prior must remain an
      explicit input; the "105 BPM" and "4/4" in the app are song-specific and stay there.
- [x] **Silence finder**: all channels below a threshold (−45 dBFS today) for at least a
      minimum duration (150 ms).
- [x] **Alignment check** (`CheckAlignment` today, generalized): correlation of a reference
      against the sum of N parts over ±lag, best lag and residual RMS. This becomes the main
      acceptance metric of Workstream D (see its exit criteria).
- [x] Stays in `AudioVisualizer`: WAV loading (§1.3), `features.json` schema and quantization,
      `PixelParadeCues` (hand-authored, song-specific), the Markdown/HTML report and the PNG
      renderer. `ResampleAligned` (tail flush + fractional group-delay compensation around
      `resample.NewForRates`) is generic: move it into `dsp/resample` as an option such as
      `WithZeroDelay`, with a test that an impulse at `t` lands at `t·out/in` within 0.01 sample.
      Done as `Resampler.ProcessAligned` / `resample.ResampleAligned` (a whole-buffer call, since
      delay removal needs the tail and cannot stream); the dB helper is `core.LinearToDBFloor`.
- [x] Tests: synthetic fixtures with known answers (click train at a known BPM, gated sines per
      band, a silent gap of known length, ±90° stereo pair for the width); a regression fixture
      that reproduces the current `AudioVisualizer` `features.json` for `PixelParade.wav` within
      quantization (except the log-spectrogram rows changed by the defect fix above); runnable
      examples.
- [ ] `examples/analysis_overview`: computes the features for a generated test signal and writes
      a label-free spectrogram and envelope heatmap PNG with the standard library's `image/png`
      (no font or plotting dependency in the root module). The labelled six-panel figure is
      `AudioVisualizer`'s job.

#### Workstream B extension: melody, drum kinds, downbeat

Added after the PixelParade visualizer rework, which needed the lead melody and drum roles:

- [x] **Predominant pitch and chroma** (`measure/music/melody.Analyze`): harmonic-sum salience on
      a 0.1-semitone grid (MIDI 52–96, 8 harmonics weighted 0.8^(h−1)), voicing as the power
      share of the chosen harmonic series, −50 dBFS gate, voiced-only median smoothing, and a
      12-bin chroma (C…B, `pitch.PitchClass` order) per frame. Frame durations are options in
      seconds, so other rates/hops work.
- [x] **Note segmentation** (`melody.SegmentNotes`), usable on any pitch track (also YIN):
      minimum length, gap, pitch-jump split and re-attack at onsets; starts snap to onsets
      within 40 ms.
- [x] **Drum-hit kinds** (`onset.ClassifyDrums`): kick / snare / hat from band power shares
      over the first 30 ms; heuristic labels for the default 5-band layout, rules replaceable.
- [x] **Downbeat** (`rhythm.Downbeat`): bar phase from weighted accents (kicks, bass onsets)
      near beats.
- [ ] pYIN/Melodia-style probabilistic pitch tracking. (Key/chord estimation on top of chroma
      moved to Phase 45.)

#### Workstream C: Classical source separation baseline (`dsp/separate`)

Weight-free, small, deterministic, and useful whenever the model is not available. It also gives
Workstream D a sanity baseline to beat.

- [x] **HPSS** (Fitzgerald 2010): median filter across time (harmonic) and across frequency
      (percussive) on the STFT magnitude, with soft Wiener masks of power `p` (Driedger et al.
      2014 margin variant optional). Outputs harmonic + percussive (+ residual), and the outputs
      sum back to the input.
- [x] **Mid/side centre extraction** as a cheap "vocals-ish / centre" split; documented as
      heuristic.
- [x] **Generic soft-mask / Wiener application helper** shared with D's optional Wiener
      post-filter and with Phase 37.
- [x] Tests: sum of outputs reconstructs the input (< −100 dB); a sine + click mixture separates
      with > 20 dB isolation; zero-alloc streaming path where feasible. Measured: reconstruction
      ≈ −310 dB, isolation 25–35 dB (tone 10 dB above the clicks; with equal energy, clicks leak
      ≈ −18.5 dB into the harmonic output at nfft 2048, a floor set by the bins the tone
      occupies). `SeparateInto`, `MedianFilter.Filter` and `SoftMasks` run without allocations;
      a block-streaming HPSS (needs `harmonicKernel/2` frames of lookahead) is left open.
- [ ] Golden-vector parity against librosa's `hpss` (edge mode and margin formula follow it).

#### Workstream D: Demucs v4 (`htdemucs`) inference port

All numbers below were read from the actual checkpoint used by `AudioVisualizer`
(`955717e8-8726e21a.th`, SHA-256 `8726e21a993978c7ba086d3872e7608d7d5bfca646ca4aca459ffda844faa8b4`,
84 MB) and from `demucs==4.1.0`'s `apply.py`/`api.py`/`htdemucs.py`.

**Checkpoint facts:**

- The `.th` file is a **pickled torch dict** (`klass`, `args`, `kwargs`, `state`,
  `training_args`, `metrics`), not safetensors. Go must not unpickle it.
- `state` has **533 tensors, 41,984,456 parameters, all `float16`**. PyTorch upcasts them to
  `float32` on `load_state_dict`, so inference runs in **float32 with fp16-rounded weights**.
  The `--float32` flag in `separate.py` affects only the output WAV encoding.
- `kwargs`: `sources=[drums, bass, other, vocals]`, `audio_channels=2`, `samplerate=44100`,
  `segment=Fraction(39,5)` (7.8 s), `channels=48`, `growth=2`, `nfft=4096` (hop = nfft/4 = 1024),
  `cac=True` (complex-as-channels), `depth=4`, `rewrite=True` (1×1 conv + GLU),
  `kernel_size=8`, `stride=4`, `time_stride=2`, `context=1`, `context_enc=0`, `norm_starts=4`,
  `norm_groups=4`, `dconv_mode=3` (DConv residual branch in encoder and decoder),
  `dconv_depth=2`, `dconv_comp=8`, `freq_emb=0.2`, `emb_scale=10`, `emb_smooth=True`,
  `multi_freqs=[]`, `bottom_channels=512`, `t_layers=5`, `t_heads=8`, `t_hidden_scale=4.0`,
  `t_emb="sin"`, `t_max_period=10000`, `t_weight_pos_embed=1.0`, `t_norm_in=True`,
  `t_norm_first=True`, `t_norm_out=True`, `t_layer_scale=True`, `t_gelu=True`,
  `t_cross_first=False`, sparse attention off, `wiener_iters=0`, `rescale=0.1`.
  Training-only settings (`t_dropout=0.02`, `t_cape_augment`, `t_sin_random_shift`) have no
  effect at inference and must be ignored, not implemented.

**Pipeline exactly as `separate.py` runs it** (`-n htdemucs -d cpu --shifts 1 --float32
--clip-mode none`, seed 42, 4 threads):

1. Load the mix (48 kHz stereo for `PixelParade.wav`) and resample it to 44.1 kHz with
   `julius` (sinc, 48000:44100 = 160:147). `algo-dsp`'s resampler is not `julius`, so its output
   will differ slightly; parity tests must feed the **Python-resampled 44.1 kHz input** to
   isolate model parity from resampler differences.
2. Normalize: `ref = mean over channels`; `x = (x − ref.mean()) / (ref.std() + 1e-8)`.
   `torch.std` is the **unbiased (n−1)** estimator. The inverse is applied to every output stem.
3. **Shifts** (`--shifts 1` still shifts): pad the mix by `max_shift = 0.5·44100 = 22050` on
   both sides, draw `offset = random.randint(0, 22050)` from **Python's `random` module**
   (seeded 42; Mersenne Twister), run on `padded[offset : offset + length + max_shift − offset]`,
   and keep `out[..., max_shift − offset:]`. The Go API takes the offsets explicitly (a slice,
   one per shift), so the caller controls determinism without emulating Python's RNG. Also
   **fix in `AudioVisualizer`**: `separation.json` does not currently record the drawn offset,
   so the reference run cannot be reproduced bit-exactly from its own metadata. Record it.
4. **Split**: `segment_length = int(44100·7.8) = 343980` samples, `overlap = 0.25` →
   `stride = 257985`, offsets `0, stride, 2·stride, …`; each chunk is weighted by a triangle
   (`1…L/2, L−L/2…1`, normalized to max 1, `transition_power = 1`), and the summed outputs are
   divided by the summed weights. The last chunk is shorter and zero-padded; the model pads each
   chunk to the training length (`use_train_segment`) and crops its output back.
5. **Model forward** (per chunk, batch 1):
   - Spectral branch input: reflect-padded STFT (`pad = 3·hop/2`, `le = ceil(len/hop)`,
     `normalized=True`, `center=True`, Hann), drop the Nyquist bin (2048 bins remain) and
     crop frames to `[2 : 2 + le]`; with CaC, real and imaginary parts become channels
     (2 audio × 2 = 4 input channels). Normalize by the spectrogram's mean/std, and the
     time branch by its own mean/std; both are undone at the output.
   - 4-level encoder in each branch (channels 48 → 96 → 192 → 384): spectral `Conv2d`
     kernel `[8,1]` stride `[4,1]` over frequency, temporal `Conv1d` kernel 8 stride 4; each
     followed by GELU, a `DConv` residual (dilated conv → GroupNorm(1) → GELU → 1×1 conv →
     GroupNorm(1) → GLU → LayerScale, depth 2, compress 8), and the 1×1 `rewrite` conv + GLU.
     `ScaledEmbedding` frequency embedding (scale 10, smooth init, weight 0.2) is added after the
     first spectral encoder. `norm_starts=4` means no GroupNorm in the 4 main layers (check this
     against the state-dict keys, not the docs).
   - Bottleneck: the spectral branch's remaining (frequency × time) grid is treated as one
     sequence, both branches are projected 384 → 512 (`bottom_channels`), and go through the **cross-domain transformer**:
     5 layers, d_model 512, 8 heads, FFN 2048, pre-norm, LayerScale, GELU. Layers alternate
     self-attention (even) and cross-attention between the branches (odd). Positional encodings:
     2-D sinusoidal for the spectral branch, 1-D sinusoidal for the time branch, both
     `max_period` 10000 with weight 1.0, plus input LayerNorm and output GroupNorm-style norms.
     Projected back 512 → 384.
   - Decoders mirror the encoders with skip connections (`ConvTranspose`), output 4 sources ×
     CaC channels on the spectral side and 4 sources × 2 channels on the time side.
   - Output: iSTFT of the spectral estimate (inverse padding/cropping of step 5a) **plus** the
     time-branch estimate, per source.
6. Denormalize and write float32 stems (`--clip-mode none`: no clipping, no rescale). In
   `AudioVisualizer` the stems come out at 44.1 kHz and are later resampled to 24 kHz for
   analysis; the mix analysis uses 48 kHz → 24 kHz. Workstream B's alignment check is what
   proves the two paths share t = 0.

**Implementation tasks:**

- [ ] **Weight conversion** (one-time, offline, Python): `scripts/htdemucs_export.py` loads the
      `.th`, writes `htdemucs.safetensors` (keep fp16; 84 MB) plus `htdemucs.json` (the
      `kwargs` above, source order, source checkpoint SHA-256). Keep it out of the module
      proper; it is a tool, like the legacy Pascal references.
- [ ] **safetensors reader** in Go (8-byte LE header length + JSON header + raw little-endian
      data; fp16 → fp32 conversion at load), validating tensor names/shapes against a
      checked-in manifest of all 533 keys so a different checkpoint fails loudly instead of
      producing garbage.
- [ ] **Minimal tensor/layer runtime**, float32, NCHW, no autograd: `Conv1d`, `Conv2d` (kernel
      along one axis only, which simplifies it), `ConvTranspose1d/2d`, `Linear`, `GroupNorm`,
      `LayerNorm`, `GELU` (exact erf form, as PyTorch's default), `GLU`, `LayerScale`,
      multi-head attention (softmax computed in float32 with max subtraction), sinusoidal 1-D/2-D
      embeddings, `ScaledEmbedding`. Each layer gets a golden-tensor test.
- [ ] **GEMM is the hot path** (convs as im2col + GEMM, attention, FFN). `algo-vecmath` is
      float64-only today (see Phase 41c), so a cache-blocked `float32` GEMM is needed: either
      a new `algo-vecmath` primitive (preferred; tracked in that repo's PLAN.md) or a local
      blocked pure-Go kernel as a scalar reference first. Same rule as Phase 41c: a scalar
      reference path plus parity tests before any SIMD.
- [ ] **Parallelism**: chunks are independent, so run them on a worker pool and accumulate
      results **in offset order**, so output is bit-identical regardless of worker count.
      Expose `WithWorkers(n)`.
- [ ] **Golden-tensor harness** (`scripts/htdemucs_golden.py`): run the PyTorch model on a
      short fixed input with forward hooks and dump every module's input and output to `.npy`/
      safetensors, so Go parity can be checked layer by layer. Without this, debugging a 533-
      tensor port means comparing only the final output, which is hopeless.
- [ ] **API sketch** (final shape decided at the gate):
      `htdemucs.Load(r io.Reader) (*Model, error)`;
      `(*Model).Separate(ctx, mix [][]float32, opts...) (map[string][][]float32, error)` with
      `WithShiftOffsets([]int)`, `WithOverlap(0.25)`, `WithSegment(seconds)`,
      `WithWorkers(n)`, progress callback; input must already be 44.1 kHz stereo (resampling
      stays the caller's choice, via `dsp/resample`). `context.Context` for cancellation, since
      a run takes minutes.
- [ ] **Provenance struct** returned with the result, so callers can write their own
      `separation.json`: model id, checkpoint SHA-256, offsets used, segment/overlap, workers,
      elapsed time, and the Go/`algo-dsp` version. The library does not write files.

**Performance reference:** PyTorch CPU, 4 threads, took **126.7 s for 86.12 s of audio**
(≈ 1.47× real time) on the dev machine. First target: ≤ 3× that with the scalar GEMM and
the worker pool; then close the gap with SIMD. Track in `BENCHMARKS.md` as a per-chunk
benchmark (one 7.8 s chunk), never a full-song benchmark in CI.

**Risks:**

- fp32 summation order differs from PyTorch/MKL, so bit-exactness is impossible; parity is
  statistical (tolerances below). Accumulating attention/FFN dot products in float64 is an
  option if the tolerance is missed.
- Memory: activations of a 7.8 s chunk at 48–512 channels are hundreds of MB if every layer's
  output is kept; free skip tensors as soon as the decoder consumes them and reuse buffers.
- Upstream Demucs is archived and unmaintained; the port pins one checkpoint and does not try
  to track other variants (`htdemucs_ft`, `htdemucs_6s`, `mdx`). Adding `htdemucs_ft` later is
  "four models plus a bag average", which the API should not preclude.

#### Workstream E: Hand-back to `AudioVisualizer`

- [ ] Replace `scripts/separate.py` with a Go `cmd/separate` on top of D, and
      `internal/audioanalysis` with calls to B. Delete the Python venv requirement
      (`scripts/requirements-stems.txt`); keep the Python export/golden scripts for
      re-verification only.
- [ ] Move the six-panel overview figure from matplotlib to Go in `AudioVisualizer`
      (renderer of its choice), consuming B's log-bin frequencies for axis labels. Bands colours
      and dB ranges as in `plot_analysis.py` (bands cyan/pink/violet/yellow/white, −85…0 dBFS
      envelopes, −75…−10 dB spectrogram).
- [ ] Bump `separation.json` with `implementation: "go"` and the shift offset, so old Python
      and new Go stems can be told apart.

Exit criteria:

- [ ] A/B: `go test -race` passes for the new packages; STFT round-trip and torch golden
      vectors pass; features reproduce the existing `PixelParade` `features.json` within
      quantization (spectrogram rows excepted, as documented).
- [x] C: HPSS reconstruction < −100 dB and isolation > 20 dB on the synthetic fixture.
- [ ] D: every layer matches the PyTorch golden tensors to a relative error ≤ 1e-4; full-song
      stems for `PixelParade.wav` (same 44.1 kHz input, same shift offset) match the PyTorch
      stems at a residual of ≤ −80 dBFS RMS per stem; the stem sum still passes
      `AudioVisualizer`'s alignment gate (correlation ≥ 0.95, |lag| ≤ 0.5 ms).
- [ ] D: output is bit-identical across `WithWorkers(1)` and `WithWorkers(8)`; runtime within
      the performance target above.
- [ ] E: `AudioVisualizer` regenerates `analysis/` without Python.

### Phase 45: Music Structure & Harmony (Complete, post-v1.0)

Status: Workstreams A–F landed (`measure/music/{rhythm,melody,harmony,structure,motif,align,features}`,
`examples/structure_overview`, the `github.com/cwbudde/midi` module and the `AudioVisualizer`
hand-back). Only the optional librosa/madmom chord comparison is open. Depends on Phase 44
Workstream B (`measure/music/{features,melody,rhythm}`).

Ports the second analysis layer of `AudioVisualizer` (`github.com/cwbudde/AudioVisualizer`,
`internal/story` behind `cmd/story`, about 2,600 LOC with tests) that was written for the
PixelParade v3 music video. Phase 44 B turns audio into frame features, onsets, a beat grid and
melody notes. This layer turns those into musical facts: a key, a chord chart, phrase
structure with A/A′/B labels, and recurring motifs (leitmotifs) with their transposed returns.
None of it exists here yet. PLAN.md's Phase 44 B extension lists "key/chord estimation on top
of chroma" as open, and that item moves into this phase.

Same rules as Phase 44 B:

- One package per concern.
- Song-specific values (105 BPM, G major, the Demucs stem names, the hand-set cues) stay in the
  app and become explicit inputs.
- Each package carries a `reference_test.go` with a verbatim copy of the app code and a
  `TestXParity`, which must reproduce it bit for bit with the default options.

The app's parameter structs are positional literals today, for example
`CleanParams{52, 0.35, 50, 1, 19, 2, 16, 2, 4, 2, 2}`. They become functional options with
those values as documented defaults.

#### Decision gate (resolve before starting Workstream F)

- [x] **Where the MIDI file writer lives.** `AudioVisualizer/internal/smf` (208 LOC,
      dependency-free) is a format-1 Standard MIDI File writer and reader: PPQ 480, tempo, time
      and key signature, markers, text, explicit note-offs, no running status, and a
      deterministic order for simultaneous events (meta, note-off, other, note-on).
      It was used to export the analysis as `PixelParade.mid` for checking in a DAW.
      §1.3 excludes file containers, so it does not belong in this module.
      **Lean: a separate `github.com/cwbudde/midi` module**, like `wav`.
      This library returns the note, chord and marker data, and the caller encodes it.
      **Decided: `github.com/cwbudde/midi` (package `smf`)**, API-compatible with the app's
      `internal/smf` and byte-identical on its output; `Track.Note` now clamps a negative
      note-on before placing the note-off, so a note before tick 0 no longer hangs.
- [x] **Viterbi placement.** Chord smoothing is a small Viterbi over a constant switch
      penalty. **Lean: keep it unexported in `harmony`** until a second user appears
      (e.g. pYIN from Phase 44 B); only then promote it to `stats/hmm`.
      **Decided: unexported in `harmony/viterbi.go`**, allocation-free on a reused `Chorder`.

#### Workstream A: Grid and note cleanup (`measure/music/rhythm`, `measure/music/melody`)

From `internal/story/grid.go`, `notes.go` and `internal/audioanalysis/melody.go`.

- [x] **`rhythm.Grid`**: a constant-tempo 16th/beat/bar grid built from plain values (BPM,
      beat origin, downbeat index, beats per bar, subdivisions per beat), including a pickup
      bar before the first downbeat. Methods map time to slot, slot to time, bar, bar start,
      beat start and bar position. It complements `BeatGrid` (beat times only), and its input
      is `BeatGrid`/`Downbeat` output, not the app's `Rhythm` type.
- [x] **`melody.Clean(notes, grid, opts...)`** returns cleaned notes plus the dropped raw notes
      with a reason: - drop notes below a floor MIDI note (the tracker's lowest pitch is mostly artefacts) or
      below a minimum strength; - quantise starts to 16th slots, flag notes more than 50 ms off the grid, and keep the
      stronger note per slot (monophonic); - octave correction in three steps: a bar-periodic vote (same slot one 16-slot period
      apart, at least 2 votes), then the distance to the median of nearby bars, then
      folding lone octave spikes; - split short repeated runs into an arpeggio voice (min run 4, max gap 2 slots, max note
      2 slots).

      Defaults are the app's `LeadCleanParams`. The bass preset differs only in the floor
                      (MIDI 28).

- [x] **`melody.BassPreset() []Option`**: FFT 8192, MIDI 28–60, 30–1200 Hz, 6 harmonics, minimum
      note 0.10 s, onset snap 0.06 s (for use with bass-stem onsets). Currently
      `audioanalysis.BassMelodyOptions` in the app.
- [x] Tests: - an arpeggio with planted octave errors comes back corrected, and genuine leaps
      (G5 G4 F#5) survive; - a pickup bar and a downbeat offset map correctly; - quantisation is stable for notes jittered by ±30 ms; - the bass preset tracks a synthetic 41–55 Hz bass line within 10 cents.

#### Workstream B: Key and chords (`measure/music/harmony`)

From `internal/story/harmony.go` and the window pooling in `story.go`.

- [x] **`EstimateKey(chroma [12]float64, opts...) (Key, error)`**: correlates the chroma with
      the Krumhansl–Kessler major/minor profiles over all 24 rotations. If the relative major
      and minor keys are within a tie margin (0.05), optional tonic evidence (typically bass
      pitch-class weight, `WithTonicEvidence`) decides, and `Key.Ambiguous` is set. - `Key` reports tonic (`pitch.PitchClass`), mode, correlation, runner-up and margin. - Methods: `Sharps()` and `Diatonic(pc)`, built on `pitch.Scale`. - Profile set is an option, so Temperley or Albrecht–Shanahan profiles can be added
      later without an API change.
- [x] **`Windows(chroma, frameRate, spans, opts...)`**: pools frame chroma (Phase 44's
      `melody.Result.Chroma`) over grid spans, RMS-weighted, into per-window evidence
      (chroma, optional bass pitch-class weight, level in dBFS).
- [x] **`Chords(windows, key, opts...) ([]Chord, error)`**: - scores 12 roots × templates (maj, min, 7, maj7, m7; the template set is an option)
      against each window's chroma; - adds a bass-root bonus (0.2), half credit for a bass on another chord tone
      (inversions, 0.5), a seventh penalty (0.03) and an in-key bonus (0.05); - outputs `N` when the window is below the gate (−50 dBFS) or flat
      (max/mean < 1.5); - Viterbi smoothing with a switch penalty (0.10), then merges equal neighbours.

      `Chord` carries root, quality, bass (slash chords), symbol, score, margin and voicing.

- [x] Tests: - every one of the 24 keys is recovered from a synthetic I–IV–V–I cadence; - the relative-key tie is broken by bass evidence and flagged ambiguous; - a I–vi–IV–V progression with one first-inversion chord yields the right symbols,
      including the slash; - a B7/D♯ window is not read as D♯m (the regression the 0.2 bass bonus fixes); - silence gives `N`; one noisy window does not cause a chord flip.
- [ ] Optional: golden comparison against `librosa`/`madmom` chord output on a public-domain
      clip.

#### Workstream C: Structure (`measure/music/structure`)

From `internal/story/structure.go`. Feature-agnostic: callers pass rows (beats or bars × dims).

- [x] **`ZScore(rows)`**: standardises each dimension in place; constant dimensions become 0.
      **`Blocks`**: L2-normalises then weights feature groups (e.g. chroma, bass chroma, band
      dB, stem dB, drum grid), so callers can mix heterogeneous features. The app weights are
      1, 0.7, 0.5, 0.5, 0.5.
- [x] **`SelfSimilarity(rows)` / `SelfSimilarityInto(dst, rows)`**: cosine SSM.
- [x] **`FooteNovelty(ssm, halfWidth)`**: Gaussian-tapered checkerboard kernel (half-width 8
      beats in the app).
- [x] **`Peaks(novelty, radius, sigma)`**: boundaries are local maxima within ±radius (4) above
      mean + σ·sd (σ = 1). Optional reference marks (`WithMarks`) report the nearest mark
      and its offset for each boundary. This is how the app compares detected boundaries
      with its hand-set cues; the cues themselves stay in the app.
- [x] **`Label(ssm, unit, same, variant)`**: phrase labelling into A, A′, B, … Phrases are
      `unit` rows long (4 bars). Similarity ≥ `same` (0.6) reuses a label, ≥ `variant` (0.35)
      marks a variant, otherwise a new letter. Returns labels and per-phrase similarity.
- [x] Tests: - a synthetic A B A B feature sequence gives boundaries at the joins and labels A B A B; - a perturbed third phrase becomes A′; - a constant input gives no boundaries and no NaN.
- [x] `examples/structure_overview`: writes the SSM and novelty curve as a label-free heat-map
      PNG with `image/png`, replacing the app's `ssmpng.go`. Same pattern as
      `examples/analysis_overview`.

#### Workstream D: Motifs (`measure/music/motif`)

From `internal/story/motif.go` (613 LOC), the most valuable piece of the layer. Inputs are
cleaned notes plus a `rhythm.Grid`, or beat-level chroma.

- [x] **`FindNoteMotifs(notes, grid, opts...)`**, two passes: 1. bar and half-bar windows (16 and 8 slots, at least 4 notes) compared by Dice overlap
      on (slot position, pitch class) under the best transposition, with a small
      transposition penalty; 2. transposition-invariant n-grams of 8, 6 and 4 notes (span ≤ 32 slots, gaps ≤ 4)
      compared by interval and inter-onset-ratio distance (weights 0.7 and 0.3,
      octave-tolerant).

      Greedy clustering keeps occurrences non-overlapping. Rotations of the same figure and
                      shorter motifs mostly covered by longer ones are suppressed. Matches need similarity
                      ≥ 0.8 and at least 3 occurrences.

- [x] **`FindChromaMotifs(chroma, grid, opts...)`**: fallback for material without reliable
      notes. Beat-chroma windows of 8 and 4 beats are grouped under the optimal transposition
      index (similarity ≥ 0.85).
- [x] **`Corroborate(motif, chromaMotifs, share)`** marks note motifs that the chroma pass
      confirms. **`Rank(motifs, opts...)`** scores salience from count, sections spanned,
      prominence (energy), distinctiveness, span and confirmation. It flags ostinatos (a
      figure covering ≥ 0.6 of its span) and picks the top N leitmotifs with an overlap limit
      and a per-source cap.
- [x] Types: `Motif` (ID, role theme/ostinato, notes or chroma pattern, salience terms) and
      `Occurrence{Start, End, Bar, Slot, Transposition, Similarity, Variant, NoteIndices}`.
- [x] Tests: - a planted 8-note motif returns transposed (+5), varied (one note changed) and with an
      octave error, and all three are found with the right transpositions; - a bar-long repeating arpeggio is found once and flagged as an ostinato, not as 16
      rotations; - the chroma fallback finds the planted motif when the notes are removed; - the result is deterministic under input order.
- [x] Benchmarks: the n-gram pass on 2,000 notes; allocations bounded and reported in
      `BENCHMARKS.md`.

#### Workstream E: Small generalisations (`measure/music/align`, `measure/music/features`)

- [x] **`align.Lag(ref, x, sampleRate, opts...) (LagResult, error)`**: lag, normalised
      correlation and gain in dB of one signal against a reference. Coarse search with a
      stride over ±max lag, then a fine search around the best coarse lag.
      `AudioVisualizer/cmd/verifyrender` open-codes this (stride 16 over ±20 ms, then ±16
      samples) to check a rendered video's soundtrack against the source WAV. `Check` covers
      the N-part sum case but reports no gain. Reuse `dsp/conv` for the correlation where it
      pays off.
- [x] **`features.Activity(levels map[string][]float64, spans, opts...)`**: per-span activity of
      named tracks. A track is active in a span when its mean level is within X dB
      (−12 dB in the app) of its own 95th-percentile frame level. Generalised from
      `internal/story/roles.go`, which hardcodes the four Demucs stem names. Role names
      ("rhythm", "root", "theme") stay in the app.
- [x] Tests: a known integer and fractional lag and gain are recovered; a silent span is
      inactive; a quiet but present track is active relative to its own level.

#### Workstream F: Hand-back to `AudioVisualizer`

- [x] `internal/story` keeps only the pipeline, the `story.json` schema, the Markdown report
      and the PixelParade parameters, and calls A–E.
- [x] `cmd/verifyrender` uses `align.Lag`.
- [x] `internal/smf` moves to the module chosen at the gate.
- [x] `bun run story` regenerates `analysis/story.json`, `public/analysis/story.json`,
      `story.md` and `PixelParade.mid` byte-identically. Any intended difference is
      documented here, like the log-spectrogram fix in Phase 44 B.

> Outcome notes: every package reproduces the app bit for bit with default options
> (`motif/song_test.go` reproduces PixelParade's 20 motifs from a real-song fixture), and the
> app's `story.json`, `story.md`, `PixelParade.mid` and SSM PNGs regenerate byte-identically
> (only the provenance strings change). API differences from the plan sketch: `Windows` takes
> the RMS track positionally; `Corroborate` takes the grid and options; `structure` functions
> return errors; `align.LagChannels` was added because the app sums channels inside the lag
> loop. Review hardening rejects non-finite input, caps motif sizes and makes `Rank`
> idempotent. Open follow-ups (API polish, not blocking): one span type across `harmony`,
> `motif` and `features`; consistent error-variable names; options instead of positional
> tuning parameters in `structure`; per-input drop reasons from `melody.Clean`; an exported
> per-span level helper in `features`.

Exit criteria:

- [x] A–D: `go test -race` and `-tags purego` pass for the new and extended packages; each
      `TestXParity` reproduces the `AudioVisualizer` code bit for bit with default options;
      `SelfSimilarityInto`, `FooteNovelty` and the chord Viterbi report 0 allocs/op on
      reused buffers; every exported identifier has a runnable example.
- [x] B: all 24 synthetic keys correct; chord fixture symbols exact.
- [x] D: the planted motif fixture finds every occurrence with the correct transposition and
      no rotations.
- [x] E: `verifyrender` reports the same lag, correlation and gain as before on the
      PixelParade v3 master.
- [x] F: `AudioVisualizer` story outputs are regenerated byte-identically, and
      `internal/story` contains no signal-analysis code.

### Phase 46: Constant-Q Transform (In Progress, post-v1.0)

Status: in progress. 46.1 is merged (#32) but not released. Consumer: `github.com/cwbudde/algo-transcribe`, a pure-Go port of Spotify's
basic-pitch, whose front end is an nnAudio `CQT2010v2` constant-Q transform (see that repo's
`docs/port-spec.md` §3.1 for the exact configuration and derived numbers). The CQT and the
`firwin2` design it needs are generic DSP and belong here; the model-specific parts (normalized
log, harmonic stacking, the CNN) stay in algo-transcribe.

46.1 landed close to nnAudio and apart from the rest of the library: it was tested against a Python
project, wrote its own padding, decimation and strided convolution, and had no consumer here.
46.2–46.5 reshape it before the first release. They rebuild it on shared primitives, make it a
generic CQT with basic-pitch and nnAudio presets, replace the Python golden data with Go-only tests,
and feed it into the music analysis packages. Run them in three batches: 46.2, then 46.3, then
46.4 and 46.5 in parallel. Tag v0.13.0 after 46.5 so algo-transcribe can depend on it; a
bounded-work stepper for per-step time budgets follows as a later item.

- [x] 46.1 `cqt` — Add package `dsp/cqt`, a multi-rate constant-Q transform compatible with nnAudio `CQT2010v2` (as ported in spotify/basic-pitch `basic_pitch/layers/nnaudio.py`, which is the reference to read line by line) and librosa normalization. Structure: one complex kernel set for the top octave placed in an `n_fft` frame (kernel length `ceil(Q*sr/f)` with `Q = filter_scale/(2^(1/bins_per_octave)-1)`, periodic window times complex exponential, divided by the length, then basis-normalized), applied per octave as a strided VALID convolution with stride equal to the current hop after padding by `n_fft/2`, with the hop halving per octave and the signal low-passed and decimated by 2 between octaves; octaves concatenated lowest-first and cropped to `n_bins`. Options: sample rate, hop length, fmin, n_bins, bins per octave, filter scale, window (via `dsp/window`, periodic), basis norm (L1, L2, none), padding (reflect, constant), early downsampling (implemented as nnAudio does, even though basic-pitch's configuration makes it a no-op), normalization (`librosa` multiplies by sqrt of the kernel length, `convolutional`, `wrap`), and output (magnitude, complex). API: `New(sampleRate float64, opts ...Option) (*Transform, error)`, `Process(x []float64)`, zero-allocation `ProcessInto` with `float32` and `float64` variants, `NumFrames(n int)`, `Frequencies()`, `Lengths()`, `Kernels()` and `Lowpass()` for inspection and cross-checks, and `Clone()`. Also add a public `design.Firwin2(numtaps int, freq, gain []float64, opts ...Option) ([]float64, error)` in `dsp/filter/design` with `scipy.signal.firwin2` parity (default `nfreqs`, Hamming window by default, window option; reject antisymmetric types for now) and use it for the CQT's anti-alias low-pass `firwin2(256, [0, 0.5/1.001, 0.5*1.001, 1], [1, 1, 0, 0])`. Tests: golden vectors from a pinned `uv` project in `scripts/fixtures/cqt/` (nnAudio, scipy, librosa, torch pinned exactly; fixtures committed, Go tests never run Python) covering the firwin2 cases, the kernels and full transforms for several configurations including basic-pitch's (22050 Hz, hop 256, fmin 27.5, 36 bins per octave, 309 bins, filter scale 1, L1, periodic Hann, reflect, librosa normalization) on a deterministic chirp-plus-noise signal of 43844 samples, matching to ≤1e-5 relative; table-driven option validation; zero allocations in `ProcessInto`; benchmarks for the basic-pitch configuration; runnable `Example`s; a CHANGELOG entry under Unreleased.

> Outcome notes (46.1): `dsp/cqt` and `design.Firwin2` are in place. The golden vectors come
> from `scripts/fixtures/cqt` (uv; nnAudio 0.3.4, torch 2.14.1, scipy 1.18.1, librosa 1.0.0,
> numpy 2.5.3; regenerates byte-identically). They cover 17 firwin2 cases and 8 CQT
> configurations:
>
> - basic-pitch's configuration, at full length and on a short signal;
> - nnAudio's defaults;
> - early downsampling by 8, both with and without it enabled;
> - constant padding with L2 kernels, convolutional normalization and complex output;
> - Hamming kernels with no basis norm and wrap normalization;
> - Blackman kernels.
>
> Each configuration is run twice: through nnAudio's own code in float64, and through nnAudio
> as shipped in float32. Errors are measured per bin, relative to that bin's peak:
>
> - **Float64 reference:** the Go transform matches within 1.3e-13 (limit 1e-12).
> - **Float32 reference:** basic-pitch's configuration matches within 1.3e-6 (limit 1e-5). That
>   difference is entirely nnAudio's own float32 rounding.
> - **Firwin2:** matches scipy within 2.2e-16.
>
> On one basic-pitch window, `ProcessInto` and `ProcessInto32` make 0 allocs and take about
> 6 ms on a loaded M5 Pro. Coverage is 99% for `dsp/cqt` and 100% for `firwin2.go`.
>
> Discoveries:
>
> 1. nnAudio's `CQT2010v2`, and basic-pitch's port of it, never forward `window` to
>    `create_cqt_kernels`, so they are always Hann. Here `WithWindow` is honoured. The fixtures
>    for other windows rebuild the kernels with nnAudio's own builder.
> 2. When an octave is no longer than `n_fft/2` samples, torch's reflection pad raises and
>    nnAudio silently zero-pads that octave instead. This is mirrored and has its own fixture.
> 3. `scipy.get_window` returns `[1]` for length 1, but `dsp/window` evaluates the window at its
>    edge, so Hamming(1) gives 0.08. Firwin2 and the kernels special-case this.
> 4. `dsp/window`'s Kaiser window uses the Abramowitz–Stegun polynomial for I0, with about
>    1e-7 relative accuracy. A Kaiser-windowed firwin2 therefore matches scipy only to
>    1.5e-10. That is a known limitation of `dsp/window`, left unchanged here.
> 5. `librosa.cqt` differs from nnAudio by 32% (relative Frobenius norm), because it is a
>    different algorithm. Its level still agrees: the median ratio is 1.001, and that ratio is
>    what the test checks.
>
> Deliberate deviations from nnAudio:
>
> - Computation is float64 throughout.
> - Output is frame-major (frames × bins).
> - `New` rejects hops that are not divisible by 2^(octaves−1) after early downsampling. For
>   those hops nnAudio's octaves disagree on the frame count and `torch.cat` fails.
> - `New` rejects single-sample kernel frames.
>
> Firwin2 rejects the antisymmetric types III and IV, as planned.

- [x] 46.2 `shared-primitives` — Rebuild `dsp/cqt` on shared, public building blocks instead of private copies, without changing its output. Add `window.Type.Valid() bool` and `window.Type.String() string` in `dsp/window` (String may reuse `Info(t).Name`), and use `Valid` in `cqt.WithWindow` and `design.WithWindow` (replacing their copied range checks) and in `stft.WithWindow`, which accepts unknown types today; document that `window.Generate` treats unknown types as rectangular. Add an exported numpy/torch-"reflect" padding helper to `dsp/core` (edge sample not repeated; decide the exact signature, for example `core.PadReflect(dst, x []float64, left, right int) error`, with a clear error when the pad is not shorter than the input) and use it in `dsp/stft` (replacing its private per-frame mirroring where that keeps it zero-allocation; otherwise document why) and in `dsp/cqt`; note in its doc comment that scipy's "reflect" in `dsp/separate` is numpy's "symmetric" and deliberately different. Move `overlaps` from `dsp/cqt/process.go` to `core.Overlaps(a, b []float64) bool`. Add a public strided correlation to `dsp/conv`, for example `CorrelateStridedInto(dst, x, h []float64, start, stride int) error` computing `dst[i] = sum_k h[k]*x[start+i*stride+k]` with samples outside `x` taken as zero (so negative `start` is implicit zero padding), zero-allocation, built on `vecmath.DotProduct` over the in-range span. Rebuild cqt's `decimate` on it (`start = -(len(h)-1)/2`, stride = factor) and its kernel convolution where that is not slower; keep a fused real+imaginary inner loop only if a benchmark shows the primitive more than 5% slower, and say so in a comment. Replace cqt's `grow` with `core.EnsureLen`. Constraints: every existing `dsp/cqt` and `dsp/filter/design` test passes unchanged (including the golden tests), `ProcessInto`/`ProcessInto32` stay at 0 allocs/op, and the basic-pitch benchmark stays within 5% (record before/after in the PR body). Tests for every new primitive: table-driven against a naive loop, edge cases (empty, stride larger than kernel, start far outside the signal, aliasing), 0 allocs; `Example`s for `PadReflect` and `CorrelateStridedInto`; CHANGELOG entries under Unreleased.

> Outcome notes (46.2): `dsp/cqt` now pads with `core.PadReflect` and decimates with
> `conv.CorrelateStridedInto`; `overlaps` became `core.Overlaps` and `grow` became
> `core.EnsureLen`. Its output is bit-identical to 46.1: the SHA-256 of `Process` matches `main`
> for four configurations (basic-pitch, constant padding with complex output, nnAudio defaults
> with early downsampling, a three-octave window starting at 220 Hz) at 43844 and 3000 samples,
> the latter running into the zero-pad fallback. Every `dsp/cqt` and `dsp/filter/design` test
> passes unchanged.
>
> - **New API:** `window.Type.Valid`/`String` (`String` reuses `Info(t).Name`; the Lawrey,
>   Burgess and Albrecht types, which have no metadata, get names like "Albrecht 4T", and
>   unknown values give "Type(N)"). `core.PadReflect(dst, x, left, right) error` returns
>   `ErrPadTooLong`, `ErrNegativePad`, `ErrShortBuffer` or `ErrOverlap`; it does not pad in
>   place. `core.Overlaps`.
>   `conv.CorrelateStridedInto(dst, x, h, start, stride) error` returns `ErrInvalidStride`, `ErrEmptyKernel` or `ErrAliasing`; an empty `x` gives
>   zeros. Its in-range span is computed in `uint`, so `start = math.MinInt` cannot overflow.
> - **Benchmark** (basic-pitch window, M5 Pro, 10 interleaved runs of the old and new test
>   binaries): `ProcessInto` 3.82 → 3.79 ms (p=0.44), `ProcessInto32` 3.86 → 3.82 ms
>   (p=0.53), 0 allocs/op before and after.
> - **Coverage:** `dsp/core` 94.8%, `dsp/conv` 92.9%, `dsp/cqt` 98.9%.
>
> Deliberate residual gaps:
>
> 1. The cqt kernel correlation keeps its fused real+imaginary loop (`dot2`). Rebuilt on
>    `CorrelateStridedInto` (one call each for the real and imaginary row of every filter,
>    scattered into the frame-major output), the basic-pitch benchmark was 8–14% slower.
>    Two `vecmath.DotProduct` calls per window were 10% slower. Both are over the 5% budget;
>    the comment on `dot2` records this.
> 2. `dsp/stft` keeps its per-frame mirroring. `FrameInto` must stay O(nfft) and
>    zero-allocation for a generic `F`, and the stream reads from a ring buffer, so padding the
>    whole signal with `core.PadReflect` does not fit either path. Comments on `fillFrame` and
>    `emitFrame` say so.
>
> Behaviour change: `stft.WithWindow` now rejects unknown window types with
> `ErrInvalidWindow`; it used to fall back to a rectangular window silently. The
> `cqt`/`design` `WithWindow` errors now name the type ("unknown window type Type(99)").

- [x] 46.3 `generic-cqt` — Make `dsp/cqt` a generic constant-Q transform with reference presets, and align its API with `dsp/stft`. Generic defaults: every kernel sits at exactly the frequency `Frequencies()` reports, including when the bin count is not a whole number of octaves (build the full top-octave kernel set and crop, instead of nnAudio's shifted partial octave). Presets, following `melody.BassPreset`: `cqt.NNAudio() []Option` reproduces nnAudio `CQT2010v2` exactly, including its partial-octave kernel placement and zero-padding of octaves too short to reflect (a private option may carry each quirk); `cqt.BasicPitch() []Option` is `NNAudio()` plus basic-pitch's configuration (hop 256, fmin 27.5 Hz, 36 bins per octave, 309 bins, filter scale 1, L1 basis norm, periodic Hann, reflect padding, librosa normalization, magnitude output). Later options override a preset. Align naming with `dsp/stft` where the concepts match: `FrameCount`/`Bins` instead of `NumFrames`/`NumBins`, the same padding option name and enum style (check whether `stft.Padding` itself can be shared; if not, document the difference), and the same float32 approach as `stft` if that is reasonable for a float64-internal transform (otherwise keep `ProcessInto32` and say why). Document the frame-count convention next to stft's. Update `doc.go` (replace "Differences from nnAudio" with a section on presets and what each quirk does), all `Example`s (at least one generic and one `BasicPitch()`), and the CHANGELOG entry from 46.1 so it describes the released API. The golden tests run through the presets and must stay at their current tolerances; add tests that generic kernels are centred on `Frequencies()` for partial octaves and that each preset's quirk is active only in that preset. The API is unreleased, so rename freely; no deprecated aliases.

> Outcome notes (46.3): `dsp/cqt` is a generic CQT with the presets `NNAudio()` and
> `BasicPitch()` (plus `NNAudioHopLength`… and `BasicPitchSampleRate`… constants, following
> `melody.BassPreset`). `NNAudio()` lists every nnAudio default explicitly and adds two
> private quirk options; `BasicPitch()` is `NNAudio()` plus basic-pitch's values. Through the
> presets the output is bit-identical to 46.1/46.2: the SHA-256 of `Process` matches `main`
> for basic-pitch, nnAudio's defaults, constant padding with complex output, and a 5-bin
> partial octave, at 43844 and 3000 samples. The golden tests run through the presets at
> unchanged tolerances (worst float64 error still 1.3e-13, float32 1.3e-6); new tests run
> `basic_pitch*` through `BasicPitch()` alone and `nnaudio_defaults` through `NNAudio()` alone.
>
> - **API alignment with `dsp/stft`:** `NumFrames`→`FrameCount`, `NumBins`→`Bins`,
>   `WithPadding`→`WithCenter`, `PadConstant`→`PadZero`, new `Padding()` getter.
>   `cqt.Padding` has stft's names and numeric values (`PadZero` = 1, `PadReflect` = 2) but
>   is not shared: the multi-rate transform needs centred frames in every octave, so stft's
>   `PadNone` has no counterpart. `ProcessInto32` stays: the transform is float64
>   throughout, so stft's generic `New`/`New32` would only change the I/O type and double
>   the type surface. The frame count stays torch's `n/hop+1`; the docs of both packages now
>   contrast it with stft's `ceil(n/hop)`.
> - **Generic defaults:** as nnAudio's, except zero padding (stft's default, librosa ≥0.10)
>   and no quirks. Generic `PadReflect` rejects signals whose lowest octave is not longer
>   than nfft/2 (`FrameCount` 0, `ErrSignalTooShort`), like stft's `PadReflect`; the
>   `NNAudio()` quirk zero pads those octaves instead.
> - **Tests:** every bin's measured kernel frequency equals `Frequencies()` to 7.7e-16 for
>   3/12, 20/36, 30/12, 309/36, 84/12 and 72/36 bins; tones at `Frequencies()` peak at their
>   own bin. Under `NNAudio()` the shifted placement is pinned (3/12: 0.75 octaves low;
>   20/36: a tone at bin b < 4 peaks at b+16). Both quirks are off in the generic transform,
>   on in both presets. They also stay on when every public option is given after the preset.
>   That is the documented behaviour: later options override parameters, not quirks. The
>   reflect threshold is pinned exactly (basic-pitch: 33023 samples → 0 frames, 33024 →
>   130). Coverage 99.1%.
> - **Benchmark** (basic-pitch window, M5 Pro, 6 interleaved runs against `main`):
>   `ProcessInto` 3.63 → 3.70 ms, `ProcessInto32` 3.65 → 3.68 ms, within noise; 0 allocs/op.
>
> Discovery: nnAudio's kernel shift affects only configurations with fewer bins than one
> octave. When the bin count is not a whole number of octaves but at least one, nnAudio's
> top octave covers the highest `binsPerOctave` bins and the cropped partial octave is the
> lowest one, so every kernel is already at its bin (basic-pitch's 309/36 included). The
> generic placement therefore differs from nnAudio only for bins < binsPerOctave. For
> whole-octave configurations it differs only by last-bit rounding of the kernel frequencies
> (generic takes them from the bin formula, nnAudio steps up from `fmax_t/2^(1-1/bpo)`):
> 3.5e-14 in the kernel samples and 6.2e-14 in basic-pitch's output. The presets keep
> nnAudio's formula.
>
> Residual gap: the 46.2 `### Changed` CHANGELOG bullet still describes `dsp/cqt`'s
> internal move to the shared primitives, a change between two unreleased states. It is left
> for the release edit.

- [x] 46.4 `go-only-tests` — Replace the Python-generated golden data of `dsp/cqt` and `design.Firwin2` with Go-only tests, and delete `scripts/fixtures/cqt/`, `dsp/cqt/testdata/` and `dsp/filter/design/testdata/firwin2.json.gz`. Python is not run again: values that stay pinned are extracted once from the committed fixtures before they are deleted, and each pinned block carries a provenance comment (tool, version and call, for example scipy 1.18.1 `firwin2(6, [0, 0.5, 1], [0, 1, 0])` or nnAudio 0.3.4 float64), as `dsp/filter/weighting` does for IEC 61672. Firwin2: keep scipy's behaviour pinned with inline tap values for `type2_ramp`, `one_tap`, `two_taps`, `nfreqs_odd_len`, `step_repeated_freq`, `window_none`, `type1_multiband` and the 256-tap nnAudio low-pass (store symmetric taps once); check the broad cases against an independent Go reference (inverse real FFT via algo-fft, or the closed-form cosine series) to 1e-14; keep the existing property and validation tests. CQT: generate the test signal in Go (chirp, two tones, seeded noise; deterministic) and use two references of different strength. The exact oracle is a naive multirate reference written in the test with plain loops and none of the production primitives (`conv`, `vecmath`, `core` padding): it reproduces every stage (padding, the same `Firwin2` low-pass and decimation by 2 per octave, direct per-octave kernel correlation at the decimated rate, normalization) and must match `ProcessInto` to 1e-12 relative. A full-rate direct CQT (same kernels, no decimation) is not an oracle, because the multirate path also carries the low-pass passband ripple and residual aliasing in the lower octaves; use it only for property checks on pure tones placed well inside the passband (same peak bin, magnitude within a bound derived from the low-pass passband ripple and stated in the test comment), never as an elementwise comparison; pin about 50 nnAudio values per preset (`NNAudio()`, `BasicPitch()`) from the fixtures, covering kernels, the low-pass, a few output bins per configuration, the partial-octave placement and the short-signal fallback; keep the property tests (sine peak bin, linearity, scaling, zero allocations, NaN propagation). Remove the librosa level test or replace it with a Go-checked property of the same intent. Update the package docs, README, AGENTS.md and PLAN.md wherever they mention the fixtures, and record in the outcome notes what precision was given up (the 1e-13 match against nnAudio across eight configurations; bit-level basic-pitch parity is checked downstream in algo-transcribe against basic-pitch's own CQT output). Exit check: `git ls-files | grep -E '\.py$|uv\.lock|pyproject'` lists only `scripts/extract_irs.py`, and `dsp/cqt` coverage stays at about 99%.

> Outcome notes (46.4): `dsp/cqt` and `design.Firwin2` are tested in Go only. `scripts/fixtures/cqt/`,
> `dsp/cqt/testdata/` and `dsp/filter/design/testdata/firwin2.json.gz` are deleted, and
> `git ls-files | grep -E '\.py$|uv\.lock|pyproject'` lists only `scripts/extract_irs.py`.
> The pinned values were extracted once from the committed fixtures with throwaway Go programs
> outside the repo; no Python ran.
>
> - **Firwin2:** the eight required cases pin scipy 1.18.1's taps inline, symmetric filters
>   stored once, each block with its call as provenance. They match to 1.1e-16 on the stored
>   half and 5.1e-15 mirrored: scipy's taps come from an FFT and are not bitwise symmetric,
>   by up to 5.2e-15 (`type1_multiband`). All 17 former cases are also checked against an
>   independent reference, an inverse real FFT via algo-fft with its own grid,
>   repeated-frequency and window handling. That reference matched scipy to 2.2e-16
>   (Kaiser 1.5e-10, the `dsp/window` I0 limit from 46.1) before the fixture was deleted, and
>   matches `Firwin2` to 2.2e-16. Coverage: `firwin2.go` 100%.
> - **Test signal:** generated in Go (`testSignal`: the same chirp, 440/55 Hz tones and
>   noise recipe, PCG-seeded, float32-rounded). The eight 46.1 configurations live on as the
>   option table `refConfigs()`.
> - **Exact oracle:** a naive multi-rate CQT in `oracle_test.go`, plain loops, no `conv`,
>   `vecmath` or `core`. It covers early downsampling, reflect, zero and the `NNAudio()`
>   zero-pad fallback, the `Firwin2` low-pass (bit-equal to `Lowpass()`/`EarlyLowpass()`),
>   2:1 decimation, per-octave correlation, normalization and cropping. `ProcessInto` matches
>   it to 3.1e-14 per bin (limit 1e-12) in the eight reference and five generic
>   configurations; `ProcessInto32` matches the oracle rounded to float32 to 1.5e-16.
>   Repeating the edge sample in the reflect padding or dropping the imaginary negation fails
>   it with errors of 0.3–2.5.
> - **Full-rate direct CQT:** property checks only, on pure tones at bin centres in four
>   octaves including the lowest. Both paths peak at the tone's bin. The magnitude ratio lies
>   within (1±δ)^o times the kernels' negative-frequency image terms, δ being the measured
>   passband ripple of `Lowpass()` up to the highest normalized tone frequency (basic-pitch:
>   δ = 4.0e-4, worst |ratio−1| 4.7e-4 against a bound of 3.2e-3).
> - **Level:** `TestNormalizationLevel` replaces the librosa test. A centred tone of
>   amplitude A gives F·A/2 with convolutional normalization (F the early-downsampling
>   factor), within the same ripple bound; wrap gives exactly twice that; librosa gives
>   `sqrt(Lengths()[b])` times it to 4.5e-16. New `TestScaling`.
> - **Pinned nnAudio values** (`nnaudio_test.go`, nnAudio 0.3.4 float64 unless marked
>   float32):
>   - `BasicPitch()`: about 75 values — 21 setup integers, 12 frequencies and lengths,
>     20 kernel samples, 10 low-pass taps, 6 frame-0 magnitudes in float64 and 6 in float32.
>   - `NNAudio()`: about 115 — 35 setup integers, 26 frequencies and lengths, 32 kernel
>     samples, 10 low-pass taps, 8 frame-0 magnitudes, and 3 complex frame-0 bins of
>     `const_l2_conv_complex`, which anchor the imaginary sign, zero padding and early
>     downsampling by 2.
>
>   Output values depend on the Python signal, so the first 393 samples of it are pinned.
>   Frame 0 of the top-octave bins depends on nothing else: the test computes each bin's
>   span from `Kernels()` (through the early low-pass where there is one) and requires
>   bit-identical values with two different tails. Measured errors: kernels 3.6e-15 of the
>   peak sample, low-pass 4.6e-17, outputs 7.3e-16 of the dot-product rounding scale,
>   float32 outputs 2.2e-8.
>
> - **Coverage and timing:** `dsp/cqt` 99.1%, `dsp/filter/design` 95.5% (unchanged).
>   `go test -race ./dsp/cqt` takes about 5.5 s, up from 2.8 s, mostly from the full-rate
>   tones on 163840-sample signals.
>
> Precision given up: 46.1 compared every frame and bin of eight configurations with nnAudio
> float64 (1.3e-13) and basic-pitch's with nnAudio float32 (1.3e-6). Now only the pinned
> values are compared with nnAudio, and the oracle checks the algorithm but not nnAudio's
> conventions. Not pinned against nnAudio:
>
> - outputs of `blackman`, `early_ds8_complex` and `no_early_ds`, whose cheapest bins need
>   1449, 537 and 410 input samples;
> - lower octaves, whose values depend on most of the signal;
> - the zero-padded octaves of `basic_pitch_short`, which depend on all 4000 samples (its
>   frame count, nnAudio's fallback warning and its unaffected top octave are pinned);
> - nnAudio's single-partial-octave kernel shift, which no fixture had.
>
> Bit-level basic-pitch parity is checked downstream in algo-transcribe against
> basic-pitch's own CQT output.
>
> Discoveries:
>
> 1. scipy's `firwin2` taps are not exactly symmetric (up to 5.2e-15), so storing half of
>    them costs that much against scipy.
> 2. With early downsampling the level scales with the factor F: nnAudio multiplies by F
>    while the kernels are L1-normalized at the decimated rate, and `Lengths()` (hence the
>    librosa factor) is computed at that rate too. This is pinned as is.
> 3. The 2:1 low-pass ripples by 2.5e-4 even at 0.18 of Nyquist. That ripple, not aliasing,
>    limits how closely the multi-rate and full-rate transforms agree.
>
> Neither README nor AGENTS.md mentioned the fixtures, so both are unchanged.

- [x] 46.5 `cqt-music` — Feed the CQT into the music analysis packages. Add package `measure/music/chroma`: per-frame 12-bin chroma from a `dsp/cqt` transform whose bins per octave are a multiple of 12, folding bins to pitch classes with `pitch.FrequencyToMIDI` (`dsp/effects/pitch/note.go`), with options for the tuning reference (A4 Hz, default 440), the bin range (minimum/maximum frequency), the CQT configuration and the per-frame normalization (max, L1, L2, none); return `[12][]float64` plus the frame rate, which `harmony.Windows` takes directly. The other consumers need a different shape, so add the conversions next to it: an aggregate over a frame range (for example `chroma.Profile(c [12][]float64, start, end int) [12]float64`, the mean per pitch class) that feeds `harmony.EstimateKey`, and beat pooling (for example `chroma.BeatSync(c [12][]float64, frameRate float64, grid rhythm.Grid, beats int) [][12]float64`, the mean of the frames inside each beat from `grid.Origin()` in steps of `grid.BeatSeconds()`, zero for beats without frames) that feeds `motif.FindChromaMotifs`, whose transposition search stays inside motif. In `measure/music/melody`, add an option that takes salience and chroma from the CQT instead of the STFT (harmonic summation on the CQT grid); the defaults stay unchanged so the AudioVisualizer bit-parity tests keep passing untouched. Optionally add a CQT log-spectrogram to `measure/music/features` if it fits the existing `LogSpectrogram` shape cleanly; otherwise note why not. Tests: pitch classes of synthetic chords (triads in several keys and octaves), key estimation of synthetic C-major and A-minor cadences through chroma → `Profile` → `harmony.EstimateKey`, a repeated synthetic chord phrase found by `motif.FindChromaMotifs` through `BeatSync` (including a transposed repeat), `BeatSync` edge cases (frames straddling a beat boundary, beats before the first or after the last frame), tuning-offset handling, and on synthetic glides and low notes the CQT melody option is at least as accurate as the STFT default (record both errors); `Example`s; CHANGELOG entries under Unreleased.

> Outcome notes (46.5): new package `measure/music/chroma` and the option `melody.WithCQT`.
> Coverage: chroma 100.0%, melody 99.0%; harmony, motif and melody's AudioVisualizer parity
> tests pass unchanged.
>
> - **Chroma:** `Compute(x, sampleRate, opts...)` returns `[12][]float64` and the frame rate;
>   `New`/`Process`/`Clone` reuse the transform. The CQT is derived from the options and placed
>   on the tuned semitone grid: 36 bins per octave (multiples of 12 only), A4 440 Hz, band
>   C1 − 50 cents to B7 + 50 cents, which at 440 Hz is 252 bins from C1 − 1/3 to B7 + 1/3
>   semitone. The other settings are hop 512, L1 kernels with `NormalizationWrap` and zero
>   padding; `WithCQT` options override them, and the output is always magnitude. With wrap
>   normalization a sine of amplitude A reads ≈ A in its bin. librosa's normalization would
>   tilt the bins by sqrt(kernel length).
>
>   Power is folded with `pitch.FrequencyToMIDI`. Bins exactly halfway between two semitones
>   (24 or 48 bins per octave) split evenly. `NormNone` gives ≈ 1.51·A² per sine, because the
>   two neighbour bins at ≈ A/2 fold into the same class. Chroma divides the early-downsampling
>   factor back out, which `dsp/cqt` multiplies into wrap/convolutional output as nnAudio does,
>   so values do not depend on the sample rate.
>
>   `Profile` clips the frame range and returns zeros when it is empty. `BeatSync` uses
>   harmony.Windows' frame rule `ceil(start·fr) ≤ i < ceil(end·fr)`; a cross-test checks that.
>   A frame exactly on a boundary goes to the later beat.
>
> - **Chroma results:**
>   - Triads (C3, F♯4 minor, B♭2, a spread E minor, a D major inversion): the weakest chord
>     tone is ≥ 17× the strongest other class.
>   - Cadences through `Profile` → `EstimateKey`: C I–IV–V–I gives C major (r 0.970 vs 0.632);
>     A minor i–iv–V–i gives A minor (r 0.854 vs A major 0.799, a margin of only 0.055 over
>     the 0.05 tie margin).
>   - Chord phrase (C–Am–F–G at 120 BPM, repeated, then +2 semitones): through `BeatSync`,
>     `FindChromaMotifs` with default options finds one 8-beat motif with occurrences +0, +0
>     and +2, similarity 1.000. Its default minimum is three occurrences, so the phrase needs
>     an exact repeat besides the transposed one.
>   - Tuning: an A major triad 60 cents flat folds to G♯/C/D♯ at 440 Hz and to A/C♯/E with
>     `WithReferenceHz`; +20 cents is still correct at 440 Hz.
> - **Melody:** `WithCQT(binsPerOctave, cqtOpts...)` takes salience, voicing and chroma from one
>   CQT of the whole signal. Frames (ceil(n/hop)), candidates, gate, smoothing and notes are
>   unchanged; the STFT path keeps its operation order. The CQT starts one bin below the lowest
>   candidate and runs up to maxHz. Early downsampling is off (it fails the hop check for
>   `BassPreset` at hop 240). The hop must be divisible by 2^(octaves−1): hop 240 allows 5
>   octaves, which fits the default and bass ranges at 36 bins per octave.
>
>   Three changes were forced by measurements:
>   - Salience interpolates with Lanczos-3, not linearly. Linear interpolation peaks at bin
>     centres, and since harmonics are nearly equal-tempered they all snap to the grid
>     together: 0.059 semitone mean error on a slow glide against the STFT's 0.034.
>   - `WithCQT` sets the voicing threshold to 0.6 (`DefaultCQTVoicingThreshold`). With ±2
>     CQT bins, white noise voiced 182 of 200 frames at 0.3 (the STFT: 0).
>   - The recommended resolution is `DefaultCQTBinsPerOctave` = 36. 48 and 60 smear fast
>     glides, and 12 needs 6 octaves for the default band.
>
>   Mean / p95 error in semitones, STFT default → CQT 36, all voiced, all notes correct:
>
>   | Case                     | STFT       | CQT 36     |
>   | ------------------------ | ---------- | ---------- |
>   | Glide A3→A4, 2 s         | 0.034/0.08 | 0.024/0.04 |
>   | Glide A3→A4, 0.5 s       | 0.063/0.22 | 0.029/0.08 |
>   | Glide E4→E5, 0.5 s       | 0.055/0.24 | 0.029/0.08 |
>   | Low notes E2–C3          | 0.033/0.10 | 0.000/0.00 |
>   | Low notes E2–C3, detuned | 0.025/0.04 | 0.025/0.04 |
>   | Bass E1–E2, `BassPreset` | 0.000/0.00 | 0.000/0.00 |
>   | Bass E1–E2, detuned      | 0.045/0.08 | 0.025/0.04 |
>
>   The detuned low-note case ties at the 0.1-semitone candidate floor. The test asserts
>   CQT ≤ STFT with no slack. CQT mode costs about 3.8× the STFT (16.3 vs 4.2 ms per 2 s).
>
> - **Features:** no CQT log-spectrogram. `LogSpectrogram` carries a `*LogScale`, an
>   energy-preserving FFT-bin mapping, and dBFS values calibrated by Parseval on the features
>   frame grid (ceil(n/hop) frames). A CQT has none of those: n/hop+1 frames, no FFT bins to
>   map, no Parseval calibration. It also cannot cover the AudioVisualizer range (64 bins,
>   25 Hz–12 kHz, 8.9 octaves) at the default hop 240, which allows 5 octaves. Users who want
>   one call `dsp/cqt` directly.
>
> Discovery (not fixed, `dsp/cqt`): the early-downsampling factor is capped by `nextpow2(hop)`
> as in nnAudio, not by the factors of two the hop contains. At 24 kHz with hop 240 and 5
> octaves it picks factor 2, leaving hop 120, which is not divisible by 16 → `ErrHop`, although
> the same transform works with `WithEarlyDownsampling(false)`. Both consumers document the
> workaround (melody turns early downsampling off). Counting the hop's factors of two would
> fix it without changing power-of-two hops; it is left for a separate change because the
> presets must keep nnAudio's choice.

Exit criteria:

- [x] `go test -race ./dsp/cqt ./dsp/filter/design` passes; the basic-pitch configuration matches
      the nnAudio golden vectors to ≤1e-5 relative; `ProcessInto` reports 0 allocs/op.
- [x] 46.2: `dsp/cqt` uses only public primitives for padding, decimation and strided
      correlation; its existing tests pass unchanged at 0 allocs/op. (The kernel correlation
      keeps a fused loop, as 46.2 allows: the primitive was more than 5% slower.)
- [x] 46.3: generic defaults place kernels at `Frequencies()`; `BasicPitch()` and `NNAudio()`
      reproduce the 46.1 output to the 46.1 tolerances.
- [x] 46.4: no Python project or generated fixture remains for `dsp/cqt` and `design.Firwin2`.
- [x] 46.5: CQT chroma drives `harmony` key estimation on synthetic cadences and `motif` chroma
      motifs through beat pooling; melody's defaults are unchanged.
- [ ] v0.13.0 is tagged with `just tag-release` and algo-transcribe consumes it.

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
| 0.19    | 2026-10-03 | Claude  | Added Phase 44 (post-v1.0): port of `AudioVisualizer`'s `separate.py` (Demucs v4 `htdemucs` inference) and `plot_analysis.py` (feature data products only; rendering stays a non-goal). Split into STFT/ISTFT primitive, music-analysis features, classical HPSS baseline, the Demucs port with checkpoint and pipeline facts read from the real checkpoint, and hand-back; with a decision gate on whether neural inference belongs in this module.                                                                                                                                                                                                                                                                                                                                                                                                                                                                   |
| 0.20    | 2026-10-03 | Claude  | Phase 44 Workstreams A, B and C implemented: `dsp/stft` (float64/float32 STFT with zero/reflect/no padding, unitary scaling, WSS-normalized ISTFT), `measure/music/{features,onset,rhythm,align}` (bit-identical to `AudioVisualizer`, log spectrogram defect fixed with energy-preserving bin overlap), the B extension `measure/music/melody` plus drum kinds and downbeat, `dsp/separate` (HPSS, soft masks, mid/side and centre extraction), `resample.ResampleAligned` and `core.LinearToDBFloor`. Open: streaming STFT processor, torch/librosa golden vectors, SpectralFreeze migration, `examples/analysis_overview`, Workstream D.                                                                                                                                                                                                                                                                            |
| 0.21    | 2026-10-03 | Claude  | Added Phase 45 (post-v1.0): port of `AudioVisualizer`'s `internal/story` layer — `rhythm.Grid` and `melody.Clean` note cleanup, a bass melody preset, `measure/music/harmony` (Krumhansl–Kessler key, template chords with Viterbi smoothing), `measure/music/structure` (SSM, Foote novelty, A/A′/B labels), `measure/music/motif` (transposition-invariant note and chroma motifs, salience ranking), `align.Lag` and `features.Activity`. Decision gate: the SMF writer goes to a separate `midi` module (§1.3). Phase 44's key/chord item moved here.                                                                                                                                                                                                                                                                                                                                                              |
| 0.22    | 2026-10-03 | Claude  | Completed Phase 45: `rhythm.Grid`, `melody.Clean`/`BassPreset`, `measure/music/harmony` (key, windows, chords), `measure/music/structure` (SSM, novelty, peaks, labels), `measure/music/motif` (note and chroma motifs, salience, leitmotifs), `align.Lag`/`LagChannels` and `features.Activity`, all bit-identical to AudioVisualizer; `examples/structure_overview`; SMF writer moved to `github.com/cwbudde/midi`; review hardening against non-finite and oversized input. Optional librosa/madmom comparison open.                                                                                                                                                                                                                                                                                                                                                                                                |
| 0.23    | 2026-10-10 | Claude  | Added Phase 46 (post-v1.0): `dsp/cqt`, an nnAudio `CQT2010v2`-compatible multi-rate constant-Q transform, and `design.Firwin2`, for the basic-pitch port in `algo-transcribe`.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                         |
| 0.24    | 2026-10-10 | Claude  | Phase 46 reshape before the first release: added 46.2 (shared primitives), 46.3 (generic CQT with `NNAudio()`/`BasicPitch()` presets, stft-aligned API), 46.4 (Go-only tests, Python fixtures removed) and 46.5 (CQT chroma and melody option for `measure/music`). v0.13.0 is tagged after 46.5.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                      |

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
