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
6. Detailed Phase Plan (Phases 0–31)
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
Phase 13: Advanced Parametric EQ Design               [2 weeks]  ✅ Complete
Phase 14: High-Order Graphic EQ Bands                 [4 weeks]  ✅ Complete
Phase 15: Effects — High-Priority Modulation          [2 weeks]  📋 Planned
Phase 16: Effects — High-Priority Dynamics            [2 weeks]  📋 Planned
Phase 17: Effects — High-Priority Spatial             [1 week]   ✅ Complete
Phase 18: Effects — Medium-Priority Waveshaping/Lo-fi [2 weeks]  📋 Planned
Phase 19: Effects — Medium-Priority Modulation        [2 weeks]  📋 Planned
Phase 20: Effects — Medium-Priority Dynamics          [2 weeks]  📋 Planned
Phase 21: Effects — Spatial and Convolution Reverb    [2 weeks]  📋 Planned
Phase 22: Effects — Specialized / Lower-Priority      [4 weeks]  📋 Planned
Phase 23: High-Order Shelving Filters                  [2 weeks]  🔄 In Progress
Phase 24: Optimization and SIMD Paths                 [3 weeks]  🔄 In Progress
Phase 25: API Stabilization and v1.0                  [2 weeks]  🔄 In Progress
Phase 26: Nonlinear Moog Ladder Filters               [3 weeks]  📋 Planned
Phase 27: Goertzel Tone Analysis                      [2 weeks]  📋 Planned
Phase 28: Loudness Metering (EBU R128 / BS.1770)      [3 weeks]  📋 Planned
Phase 29: Dither and Noise Shaping                    [3 weeks]  📋 Planned
Phase 30: Polyphase Hilbert / Analytic Signal         [2 weeks]  📋 Planned
Phase 31: Interpolation Kernel Expansion               [2 weeks]  📋 Planned
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

### Phase 15: Effects — High-Priority Modulation (Flanger, Phaser, Tremolo) (Planned)

Rules:

- Algorithm-only; no I/O.
- Constructor + options; `Process` + `ProcessInPlace` + `Reset`.
- Tests + runnable example per effect.

Tasks:

- [x] Flanger
  - Algorithm: short modulated delay (0.1–10 ms) with feedback and wet/dry.
  - API sketch:

    ```go
    func NewFlanger(sampleRate float64, opts ...Option) (*Flanger, error)
    func (f *Flanger) Process(sample float64) float64
    func (f *Flanger) ProcessInPlace(buf []float64) error
    func (f *Flanger) Reset()
    ```

  - [x] Implement core algorithm (short modulated delay, feedback, mix).
  - [x] Implement interpolation tap (reuse chorus approach).
  - [x] Add tests (parameter validation + basic response sanity).
  - [x] Add runnable example.

- [x] Phaser
  - Algorithm: allpass cascade with LFO modulation.
  - [x] Implement allpass cascade (4–12 stages) with LFO modulation.
  - [x] Add tests + example.
- [x] Tremolo
  - Algorithm: amplitude modulation with LFO and optional smoothing.
  - [x] Implement LFO amplitude mod + smoothing.
  - [x] Add tests + example.

Exit criteria:

- [x] `go test -race ./dsp/effects/` passes with new effects.

### Phase 16: Effects — High-Priority Dynamics (Planned)

Tasks:

- [x] De-esser
  - [x] Implement split-band detection and reduction.
  - [x] Add tests + example.
- [x] Dynamics core architecture (feedforward + feedback)
  - [x] Implement reusable dynamics core in `dsp/effects` with explicit detector/gain-computer separation and shared envelope state.
  - [x] Add topology option: `Feedforward` (detect from input/sidechain) and `Feedback` (detect from output/previous gain), based on `legacy/Source/DSP/DAV_DspDynamics.pas`.
  - [x] Implement detector modes:
  - [x] `Peak` detector with attack/release smoothing coefficients.
  - [x] `RMS` detector with configurable RMS window/time and ring-buffer update path.
  - [x] Optional sidechain prefilter path (low-cut/high-cut) for detector-only control signal.
  - [x] Implement gain-computer modes:
  - [x] Hard-knee compression curve (threshold + ratio).
  - [x] Soft-knee compression curve (knee width in dB, smooth transition around threshold).
  - [x] Manual make-up gain and auto make-up gain policies.
  - [x] Implement reset/state management for deterministic streaming behavior (peak/RMS history, feedback previous-abs sample, hold counters where used).
  - [x] Add sample-rate aware coefficient recalculation and strict parameter validation (threshold, ratio, knee, attack/release, RMS time, sidechain cutoff bounds).
- [ ] Compressor implementations (topology-specific)
  - [x] Implement feedforward compressor variants:
  - [x] Peak feedforward compressor.
  - [x] RMS feedforward compressor.
  - [x] Sidechain-filtered feedforward compressor.
  - [x] Implement feedback compressor variants:
  - [x] Hard-knee feedback compressor.
  - [x] Soft-knee feedback compressor.
  - [x] Include feedback-specific time-constant behavior where attack/release scaling depends on ratio (legacy parity target).
  - [ ] Expose clear API surface (`ProcessSample`, `ProcessInPlace`, stereo/frame processing variant, `Reset`, constructor+options).
- [ ] Legacy parity and characterization for dynamics
  - [x] Build parity-oriented reference tests for feedforward and feedback paths using vectors derived from `legacy/Source/DSP/DAV_DspDynamics.pas`.
  - [ ] Validate characteristic curves (`in -> out` and gain reduction) for threshold/ratio/knee sweeps.
  - [x] Validate temporal behavior on step/burst tests (attack, release, feedback recovery, RMS window response).
  - [x] Add benchmark coverage for hot paths and allocation checks (`allocs/op` near zero for in-place processing).
- [x] Expander
  - [x] Implement downward expander on top of shared dynamics core (feedforward first, optional feedback mode if stable).
  - [x] Add hard-knee + soft-knee variants with range control where appropriate.
  - [x] Add tests + runnable example.
- [x] Multiband compressor
  - [x] Implement crossover + per-band compressors using feedforward core initially.
  - [x] Add optional feedback mode per band once single-band feedback parity is validated.
  - [x] Add recombination gain-normalization checks and phase/latency sanity tests.
  - [x] Add tests + runnable example.

Exit criteria:

- [ ] Feedforward and feedback compressor topologies both implemented and documented.
- [ ] Parity/characterization tests pass for legacy-aligned behavior envelopes.
- [ ] `go test -race ./dsp/effects/` passes with new dynamics processors.

### Phase 17: Effects — High-Priority Spatial (Complete)

Tasks:

- [x] Stereo widener
  - [x] Implement M/S gain controls with safe bounds.
  - [x] Add mono-compatibility tests + example.
- [x] Crosstalk cancellation
  - [x] Implement stereo crosstalk cancellation effect (`dsp/effects`) with constructor/options, `ProcessStereo`, `ProcessInPlace`, and `Reset`.
  - [x] Port the legacy geometric delay model from `legacy/Source/DSP/DAV_DspCrosstalkCancellation.pas` (listener distance, speaker distance, head radius, attenuation, stage count).
  - [x] Implement staged crossfeed cancellation path per channel: delay line + highshelf crosstalk filter + attenuation.
  - [x] Add parameter validation and guard rails (distance constraints, stage bounds, sample-rate updates).
  - [x] Add parity-oriented tests against legacy behavior (delay-time calculation + staged processing sanity) and runnable example.
- [x] Crosstalk simulator (IIR model)
  - [x] Implement stereo crosstalk simulator effect (`dsp/effects`) based on `legacy/Source/DSP/DAV_DspCrosstalkSimulator.pas`.
  - [x] Port configurable model presets (`Handcrafted`, `IRCAM`, `HDPHX`) as cascaded biquad shaping on the crossfeed path.
  - [x] Port delayed crossfeed buffer model with physical-diameter-derived delay (`diameter / speed_of_sound`), polarity toggle, and dry/crossfeed mix mapping.
  - [x] Add parameter validation and sample-rate dependent delay/buffer recalculation.
  - [x] Add parity-oriented tests for preset responses, delay-size calculation, and stereo processing behavior + runnable example.
- [x] Crosstalk simulator (HRTF)
  - [x] Implement HRTF-based stereo crosstalk simulator in `dsp/effects`, informed by `legacy/Source/DSP/DAV_DspCrosstalkSimulatorHRTF.pas`.
  - [x] Provide two modes: simple crossfeed-only convolution and complete direct+crossfeed convolution.
  - [x] Define an HRTF provider interface contract (transport-agnostic) and support impulse-response reload on HRTF/sample-rate changes.
  - [x] Implement convolution routing/mixing for left/right direct and opposite-channel crossfeed paths.
  - [x] Add deterministic tests (routing/parity sanity with fixture IRs), parameter validation, and runnable example.

### Phase 18: Effects — Medium-Priority Waveshaping/Lo-fi (Planned)

Tasks:

- [x] Distortion
  - [x] Implement baseline waveshapers (soft/hard clip, tanh) and expose selectable shaping modes.
  - [x] Port and cover `legacy/Source/DSP/DAV_DspWaveshaper.pas` shaping family:
  - [x] Formula waveshapers (`Waveshaper1..8`, `Saturate`, `Saturate2`, `SoftSat`) with parameter-range validation.
  - [x] Chebyshev harmonic waveshaper core (order/gain-level/invert controls, odd/even constraints where applicable, optional DC bypass behavior).
  - [x] Add fast polynomial/approximation path options where they are numerically close and measurably faster.
  - [x] Add parity-oriented tests against legacy transfer-curve and harmonic-balance behavior + runnable examples.
- [x] Transformer simulation (waveshaping-based)
  - [x] Implement transformer-style saturation effect inspired by `legacy/Source/DSP/DAV_DspTransformerSimulation.pas`.
  - [x] Recreate processing topology: pre-emphasis/damping filters + oversampling path + nonlinear waveshaper + downsampling.
  - [x] Implement configurable high-pass and damping-frequency controls with sample-rate-aware updates.
  - [x] Provide both high-quality nonlinear path and lightweight polynomial approximation path; document tradeoffs.
  - [x] Add anti-aliasing validation (oversampling effectiveness), spectral characterization tests, and runnable example.
- [ ] Bit crusher
  - [ ] Implement bit depth + sample rate reduction.
  - [ ] Add tests + example.

### Phase 19: Effects — Medium-Priority Modulation (Planned)

Tasks:

- [ ] Auto-wah
  - [ ] Implement envelope follower modulating a filter.
  - [ ] Add tests + example.
- [ ] Ring modulator
  - [ ] Implement carrier multiply + mix.
  - [ ] Add tests + example.

### Phase 20: Effects — Medium-Priority Dynamics (Planned)

Tasks:

- [ ] Transient shaper
  - [ ] Implement attack/release split + shaping.
  - [ ] Add tests + example.
- [ ] Lookahead limiter
  - [ ] Implement delay + detector + gain.
  - [ ] Add tests + example.

### Phase 21: Effects — Spatial and Convolution Reverb (Planned)

Tasks:

- [ ] Convolution reverb
  - [ ] Implement partitioned convolution wrapper specialized for reverb usage.
  - [ ] Add tests + example.
- [ ] Haas delay
  - [ ] Implement short stereo delay + constraints.
  - [ ] Add tests + example.

### Phase 22: Effects — Specialized / Lower-Priority (Planned)

Tasks:

- [ ] Spectral freeze
  - [ ] Implement STFT magnitude hold + phase strategy.
  - [ ] Add tests + example.
- [ ] Granular
  - [ ] Implement grain scheduling + overlap-add.
  - [ ] Add tests + example.
- [ ] Dynamic EQ
  - [ ] Implement band filter + detector + gain mapping.
  - [ ] Add tests + example.
- [ ] Stereo panner
  - [ ] Implement equal-power pan law.
  - [ ] Add tests + example.
- [ ] Vocoder
  - [ ] Implement analysis bank + envelopes + carrier shaping.
  - [ ] Add tests + example.
- [ ] Pitch correction
  - [ ] Implement YIN helper + integrate with spectral shifter.
  - [ ] Add tests + example.
- [ ] Noise reduction
  - [ ] Implement profiling + spectral subtraction/Wiener.
  - [ ] Add tests + example.

Exit criteria:

- [ ] New effects meet minimum coverage and include runnable examples.

---

### Phase 23: High-Order Shelving Filters (Holters/Zölzer + Orfanidis) (In Progress)

Goal:

- High-order low-shelf and high-shelf filter designers returning SOS.

Design constraints:

- `order >= 1` supported; odd orders produce a first-order section.
- `gainDB == 0` should yield a passthrough.
- Validate frequency bounds (`0 < f < Fs/2`) and numeric sanity (NaN/Inf).

Implementation status (snapshot):

- Butterworth: complete + tests.
- Chebyshev I: complete + tests.
- Chebyshev II: implemented but **shape is wrong** (tests cover it).

Chebyshev II bug (summary):

- Symptom: nearly flat gain instead of shelving transition.
- DC correction makes DC right, but Nyquist stays near shelf gain.
- Likely cause: band-EQ oriented A/B parameterization does not map under direct lowpass bilinear transform.

Tasks:

- [x] Butterworth shelving designers + tests.
- [x] Chebyshev I shelving designers + tests.
- [ ] Fix Chebyshev II shelving shape bug.
  - [x] Implement Chebyshev II sections computation (current).
  - [x] Add tests (currently 8 failing due to shape).
  - [ ] Derive correct lowpass prototype poles/zeros for Chebyshev II shelving and map via bilinear.
  - [ ] Validate DC/Nyquist and stopband ripple conformance.
- [ ] Implement elliptic shelving after Chebyshev II is correct.
  - [ ] Reuse elliptic math machinery already present in `band/elliptic.go`.
  - [ ] Add stability + response tests.

Exit criteria:

- [ ] All shelving topologies produce correct shelf shape.
- [ ] All shelving tests pass.

### Phase 24: Optimization and SIMD Paths (In Progress)

Status:

- Core optimization work exists; remaining work is making performance regression detection repeatable.

Tasks (must-do):

- [x] Remove avoidable allocations in spectrum helpers caused by temporary unpacking.
  - [x] Add/extend a zero-alloc fast path.
  - [x] Wire spectrum code to prefer the fast path.
  - [x] Record before/after numbers in `BENCHMARKS.md`.
- [ ] Add a benchmark regression guard (advisory at first).
  - [x] Choose a stable, small benchmark subset covering hot paths.
  - [ ] Define regression thresholds (`ns/op`, `allocs/op`) and baseline update workflow.
  - [x] Add a CI-friendly target (e.g. `just bench-ci`) emitting a report.
  - [ ] Wire it into CI as advisory output.
- [ ] Re-run full benchmarks on at least 2 machines and update `BENCHMARKS.md`.

Optional SIMD track (modal oscillator bank; depends on `algo-vecmath`):

- [ ] Add `dsp/osc` (or `dsp/modal`) package skeleton with scalar reference.
  - [ ] Define block APIs for damped complex rotators (primary `float32`).
  - [ ] Add parity tests vs scalar reference.
  - [ ] Add microbenchmarks for modal workloads.
  - [ ] Document denormal strategy.

Exit criteria:

- [ ] Key hot paths show no major regressions in allocations/op.
- [ ] `go test ./...` and `go test -tags purego ./...` pass.
- [ ] `BENCHMARKS.md` baselines updated with date + Go version + machine info.

### Phase 25: API Stabilization and v1.0 (In Progress)

Tasks:

- [ ] Run full benchmark pass and confirm no major regressions vs baselines.
- [ ] Run full CI locally (`just ci`) including race (`go test -race ./...`).
- [ ] Confirm `CHANGELOG.md` and `MIGRATION.md` are complete for `v1.0.0`.
- [ ] Complete `API_REVIEW.md` checklist.
- [ ] Tag and publish `v1.0.0` (tag + release notes).
- [ ] Verify module proxy indexing (`go get` via `GOPROXY`).

Exit criteria:

- [ ] `v1.0.0` tag exists and release notes are published.

### Phase 26: Nonlinear Moog Ladder Filters (Planned)

Goal:

- Add production-quality nonlinear Moog ladder filter implementations in `dsp/filter/moog` that are at least on par with `legacy/Source/DSP/DAV_DspFilterMoog.pas`, with an optional path that exceeds the cited Huovilainen-method quality/performance envelope.

Tasks:

- [ ] Core architecture and API
  - [ ] Define Moog ladder processor API with constructor+options, sample/block processing, `Reset`, and explicit state type.
  - [ ] Support mono first; add stereo/frame helper API consistent with existing DSP package style.
  - [ ] Expose core controls: cutoff, resonance, drive/input gain, output gain/normalization, thermal-voltage-style shaping control (or equivalent musically-meaningful parameterization).
  - [ ] Define strict parameter validation and numeric guard rails (NaN/Inf handling, cutoff bounds `< Fs/2`, resonance safety limits).
- [ ] Legacy-faithful implementations (parity track)
  - [ ] Implement classic 4-stage nonlinear ladder variant matching Pascal structure (per-stage `tanh` nonlinearity and resonant feedback path).
  - [ ] Implement “improved classic” variant from legacy behavior and verify coefficient/update behavior parity.
  - [ ] Implement fast-approximation variant(s) for `tanh` equivalent to legacy lightweight mode, guarded behind clear option/strategy flags.
  - [ ] Reproduce legacy reset/state behavior and gain scaling semantics where practical.
- [ ] Paper-or-better implementation track
  - [ ] Implement Huovilainen-style nonlinear ladder reference path (as cited in the Pascal unit header) with documented discretization choices.
  - [ ] Evaluate and optionally implement a higher-accuracy path (e.g., zero-delay/newton refinement or equivalent) when it measurably improves tuning/resonance behavior at high cutoff/resonance.
  - [ ] Add optional anti-alias strategy for nonlinear drive path (e.g., oversampling mode) with documented CPU/quality tradeoffs.
  - [ ] Ensure the “high quality” path meets or exceeds reference behavior in tuning, self-oscillation onset consistency, and modulation robustness.
- [ ] Validation, parity, and characterization
  - [ ] Add parity-oriented tests against vectors derived from `legacy/Source/DSP/DAV_DspFilterMoog.pas` (classic + improved + lightweight modes).
  - [ ] Add frequency-response/tuning tests across sample rates and cutoff/resonance grids.
  - [ ] Add nonlinear behavior tests (drive sweep, harmonic growth trends, saturation symmetry, self-oscillation sanity).
  - [ ] Add stability tests under rapid modulation (cutoff/resonance automation) and extreme parameter bounds.
  - [ ] Add deterministic benchmark suite for scalar and fast modes; track `ns/op`, `allocs/op`, and quality deltas.
- [ ] Documentation and examples
  - [ ] Document algorithm variants and tradeoffs (faithful/fast/high-quality) with clear recommendation defaults.
  - [ ] Add runnable examples: subtractive synth-style sweep, resonance emphasis, and driven saturation comparison.

Exit criteria:

- [ ] Legacy-faithful Moog ladder variants pass parity-oriented tests.
- [ ] At least one high-quality variant demonstrates equal or better measured behavior than the reference paper/legacy baseline in documented metrics.
- [ ] `go test -race ./dsp/filter/moog` passes and benchmarks are recorded in `BENCHMARKS.md`.

### Phase 27: Goertzel Tone Analysis (Planned)

Goal:

- Add Goertzel-based single/multi-tone analysis utilities to `dsp/spectrum` with legacy parity for `legacy/Source/DSP/DAV_DspGoertzel.pas` and production-ready APIs for streaming and block workflows.

Tasks:

- [ ] Core Goertzel implementation
  - [ ] Implement a stateful single-bin Goertzel analyzer (frequency, sample rate, reset, per-sample update).
  - [ ] Port legacy recurrence and coefficient model (`2*cos(2*pi*f/fs)`) and parity-check power formula.
  - [ ] Expose outputs: power, magnitude, and dB variants with safe floor handling.
  - [ ] Add strict input validation (frequency bounds, sample rate sanity, NaN/Inf behavior).
- [ ] API and processing modes
  - [ ] Provide one-shot block API and reusable streaming API with zero-alloc hot path.
  - [ ] Support batched multi-bin processing (shared input block across many target frequencies) for DTMF/pilot-tone style detection.
  - [ ] Define reset/window semantics clearly (continuous accumulation vs block-finalized metrics).
- [ ] Numerical and behavioral validation
  - [ ] Add parity tests against vectors derived from `legacy/Source/DSP/DAV_DspGoertzel.pas`.
  - [ ] Add correctness tests versus DFT/FFT reference for on-bin and off-bin tones.
  - [ ] Add edge-case tests (near-DC, near-Nyquist, silence, clipping-level amplitudes, very short blocks).
  - [ ] Add detection-oriented tests (frequency discrimination and leakage behavior with/without windowing).
- [ ] Performance and documentation
  - [ ] Add microbenchmarks for single-bin and multi-bin workloads; track `ns/op` and allocations.
  - [ ] Add runnable examples for tone detection (single target and DTMF-style dual-tone case).
  - [ ] Document algorithm limits and recommended block sizes/windowing for robust detection.

Exit criteria:

- [ ] Legacy-parity single-bin behavior verified within tolerance.
- [ ] Multi-bin Goertzel API available and benchmarked.
- [ ] `go test -race ./dsp/spectrum` passes with Goertzel additions.

### Phase 28: Loudness Metering (EBU R128 / BS.1770) (Planned)

Goal:

- Add standards-aligned loudness metering (momentary, short-term, integrated, loudness range, true peak track) with streaming APIs and parity checks against `legacy/Source/DSP/DAV_DspR128.pas`.

Tasks:

- [ ] R128 core implementation
  - [ ] Implement K-weighting prefilter chain (high-shelf + high-pass/RLB stage) per sample rate.
  - [ ] Implement 400 ms momentary and 3 s short-term integration windows with overlap updates.
  - [ ] Implement integrated loudness gating workflow (absolute gate and relative gate).
  - [ ] Implement mono and stereo processors with shared core.
- [ ] API and metrics surface
  - [ ] Expose streaming meter state with `Reset`, `StartIntegration`, `StopIntegration`, and block/sample update APIs.
  - [ ] Expose metrics: `LUFS-M`, `LUFS-S`, integrated LUFS, peak/hold, sample counters.
  - [ ] Add optional callbacks/event hooks for periodic loudness/peak updates without allocations.
- [ ] Validation and conformance
  - [ ] Add parity-oriented tests vs `DAV_DspR128.pas` behavior envelope.
  - [ ] Add conformance tests against known R128/BS.1770 vectors and gating edge cases.
  - [ ] Add sample-rate matrix tests and long-run numerical-stability tests.
  - [ ] Add benchmarks for streaming/batch paths and allocation checks.

Exit criteria:

- [ ] R128 meter outputs for momentary/short/integrated are validated against references within tolerance.
- [ ] Mono and stereo APIs documented with runnable examples.
- [ ] `go test -race ./measure/...` passes with loudness additions.

### Phase 29: Dither and Noise Shaping (Planned)

Goal:

- Add quantization support with configurable dither PDFs and FIR/IIR noise-shaping paths, including predefined shaper sets and design tooling inspired by `legacy/Source/DSP/DAV_DspDitherNoiseShaper.pas` and `legacy/Source/DSP/DAV_DspNoiseShapingFilterDesigner.pas`.

Tasks:

- [ ] Dither core
  - [ ] Implement bit-depth quantizer core with int/float output modes and optional output limiting.
  - [ ] Implement dither modes: none, rectangular/equal, triangular, gaussian, fast-gaussian.
  - [ ] Implement deterministic RNG injection for reproducible testing.
- [ ] Noise-shaping processors
  - [ ] Implement FIR error-feedback noise shaper with configurable coefficients and history ring buffer.
  - [ ] Port predefined coefficient families/presets from legacy (E/F/IE/ME/SBM/sharp variants).
  - [ ] Implement sample-rate-aware “sharp” preset selection logic.
  - [ ] Implement optional IIR shelf-based shaping variant for lightweight mode.
- [ ] Noise-shaping filter design tooling
  - [ ] Add a filter-designer utility package for psychoacoustically weighted noise-shaper coefficient search (ATH/critical-band weighting based objective).
  - [ ] Provide deterministic search mode and exportable coefficient outputs for embedding in runtime presets.
  - [ ] Add guardrails on order/loop-count/runtime and cancellation support for long searches.
- [ ] Validation and quality
  - [ ] Add null/error-spectrum tests validating expected in-band noise reduction behavior.
  - [ ] Add parity checks against legacy presets for representative sample rates.
  - [ ] Add benchmarks for per-sample quantization path and preset designer runtime.

Exit criteria:

- [ ] Dither + FIR noise-shaper runtime paths are stable and documented.
- [ ] Preset and designed shapers are validated by spectral tests.
- [ ] `go test -race ./dsp/...` passes for new quantization/noise-shaping packages.

### Phase 30: Polyphase Hilbert / Analytic Signal (Planned)

Goal:

- Add a production-quality polyphase half-pi Hilbert transformer for analytic signal and envelope extraction workflows, based on `legacy/Source/DSP/DAV_DspPolyphaseHilbert.pas` (HIIR-style allpass/polyphase approach).

Tasks:

- [ ] Polyphase Hilbert core
  - [ ] Implement 32-bit and 64-bit processor variants with reusable state.
  - [ ] Implement two-path polyphase/allpass structure producing quadrature outputs (A/B) with half-sample alignment handling.
  - [ ] Implement coefficient-count-specialized fast paths for small orders and generic fallback path.
- [ ] API and processing modes
  - [ ] Expose `ProcessSample`, `ProcessBlock`, `Reset/ClearBuffers`, and coefficient-configuration APIs.
  - [ ] Provide envelope helper derived from analytic signal magnitude.
  - [ ] Define coefficient source/validation contracts and numeric safety checks.
- [ ] Validation and characterization
  - [ ] Add phase-quadrature tests (near 90° target across passband).
  - [ ] Add amplitude-matching and image-rejection tests for analytic signal generation.
  - [ ] Add parity-oriented checks against legacy outputs for selected coefficient sets.
  - [ ] Add benchmarks and allocation checks for block/sample APIs.

Exit criteria:

- [ ] Hilbert processor achieves documented quadrature and image-rejection targets.
- [ ] Streaming and block APIs pass race/tests with no unexpected allocations.
- [ ] Runnable examples for quadrature and envelope extraction are included.

### Phase 31: Interpolation Kernel Expansion (Planned)

Goal:

- Extend interpolation coverage beyond current primitives to include legacy-equivalent kernels and pointer/stride-oriented helpers from `legacy/Source/DSP/DAV_DspInterpolation.pas`, while preserving deterministic and allocation-free behavior.

Tasks:

- [ ] Kernel set expansion
  - [ ] Add/verify Hermite variants (`Hermite1..4` style family) with clearly documented formulas.
  - [ ] Add cubic and B-spline kernels (4-point/3rd-order and 6-point/5th-order variants) with stable edge semantics.
  - [ ] Add optional complex/interleaved interpolation helpers for DSP spectral/complex pipelines.
- [ ] API ergonomics and performance
  - [ ] Provide pointer-free safe Go APIs plus optional low-level hot-path helpers for delay/resampler internals.
  - [ ] Unify interpolation mode selection across delay/resample/effects call sites where beneficial.
  - [ ] Add benchmark coverage comparing kernels for quality-vs-CPU tradeoffs.
- [ ] Validation
  - [ ] Add parity-oriented tests versus legacy formulas and known reference vectors.
  - [ ] Add smoothness/continuity tests (value and derivative trends across fractional sweep).
  - [ ] Add boundary-condition tests for short buffers and wrap/clamp policies.

Exit criteria:

- [ ] Legacy-equivalent interpolation kernels are available with tests and docs.
- [ ] Callers can select interpolation strategy explicitly with clear tradeoffs.
- [ ] `go test -race ./dsp/interp ./dsp/delay ./dsp/resample` passes.

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

| Version | Date       | Author  | Changes                                                       |
| ------- | ---------- | ------- | ------------------------------------------------------------- |
| 0.1     | 2026-02-06 | Codex   | Initial comprehensive plan                                    |
| 0.2     | 2026-02-06 | Claude  | Expanded early phases + migration notes                       |
| 0.3     | 2026-02-08 | Claude  | Added shelving filter design phase + known Chebyshev II bug   |
| 0.4     | 2026-02-20 | Copilot | Restored detailed plan + added checkable tasks for all phases |

---

This plan is a living document and should be updated after each phase completion and major architectural decision.
