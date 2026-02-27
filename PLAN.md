# algo-dsp: Development Plan

## Comprehensive Plan for `github.com/cwbudde/algo-dsp`

This document defines a phased plan for building `algo-dsp` as a reusable,
Phase 29: Dither and Noise Shaping [3 weeks] ✅ Complete

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
6. Detailed Phase Plan (Phases 0–35)
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
Phase 15: Effects — High-Priority Modulation          [2 weeks]  ✅ Complete
Phase 16: Effects — High-Priority Dynamics            [2 weeks]  📋 Planned
Phase 17: Effects — High-Priority Spatial             [1 week]   ✅ Complete
Phase 18: Effects — Medium-Priority Waveshaping/Lo-fi [2 weeks]  ✅ Complete
Phase 19: Effects — Medium-Priority Modulation        [2 weeks]  ✅ Complete
Phase 20: Effects — Medium-Priority Dynamics          [2 weeks]  ✅ Complete
Phase 21: Effects — Spatial and Convolution Reverb    [2 weeks]  📋 Planned
Phase 22: Effects — Specialized / Lower-Priority      [4 weeks]  📋 Planned
Phase 23: High-Order Shelving Filters                  [2 weeks]  🔄 In Progress
Phase 24: Optimization and SIMD Paths                 [3 weeks]  🔄 In Progress
Phase 25: API Stabilization and v1.0                  [2 weeks]  🔄 In Progress
Phase 26: Nonlinear Moog Ladder Filters               [3 weeks]  ✅ Complete
Phase 27: Goertzel Tone Analysis                      [2 weeks]  ✅ Complete
Phase 28: Loudness Metering (EBU R128 / BS.1770)      [3 weeks]  ✅ Complete
Phase 29: Dither and Noise Shaping                    [3 weeks]  ✅ Complete
Phase 30: Polyphase Hilbert / Analytic Signal         [2 weeks]  📋 Planned
Phase 31: Vocoder                                      [3 weeks]  🔄 In Progress
Phase 32: Interpolation Kernel Expansion               [2 weeks]  📋 Planned
Phase 35: API Stabilization and v1.0                   [2 weeks]  🔄 In Progress
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

### Phase 15: Effects — High-Priority Modulation (Flanger, Phaser, Tremolo) (Complete)

- Modulation effects implemented with consistent constructor+options, sample + in-place processing, and deterministic `Reset` behavior.
- Flanger: short LFO-modulated delay (~0.1–10 ms) with feedback and wet/dry mix; interpolated tap.
- Phaser: LFO-modulated allpass cascade with configurable stage count (4–12).
- Tremolo: LFO amplitude modulation with optional smoothing.
- Tests + runnable examples for each; `go test -race ./dsp/effects/...` passes.

### Phase 16: Effects — High-Priority Spatial (Complete)

- Stereo widener: M/S gain controls with safe bounds; mono-compatibility tests + example.
- Crosstalk cancellation: staged geometric delay model + per-stage crossfeed filtering/attenuation; strict parameter validation; parity-oriented tests vs legacy + example.
- Crosstalk simulator (IIR): preset-based crossfeed shaping (`Handcrafted`, `IRCAM`, `HDPHX`) + delayed crossfeed model; validation + parity-oriented tests + example.
- Crosstalk simulator (HRTF): transport-agnostic HRTF provider contract; crossfeed-only vs full direct+crossfeed convolution routing; deterministic fixture IR tests + example.
- `go test -race ./dsp/effects/...` passes.

### Phase 17: Effects — Medium-Priority Waveshaping/Lo-fi (Distortion, Transformer, Bit Crusher) (Complete)

- Distortion: baseline waveshapers (soft/hard clip, tanh) plus legacy-derived shaping families and Chebyshev harmonic mode; includes parity-oriented tests and runnable examples.
- Transformer simulation: pre-emphasis/damping + oversampling + nonlinear shaping + downsampling; HQ and lightweight approximation modes; spectral/anti-alias characterization tests + examples.
- Bit crusher: bit-depth + sample-rate reduction; tests + runnable example.
- `go test -race ./dsp/effects/...` passes.

### Phase 18: Effects — Medium-Priority Modulation (Complete)

- Auto-wah: envelope follower modulating a filter; tests + runnable example.
- Ring modulator: carrier multiply + wet/dry mix; tests + runnable example.
- `go test -race ./dsp/effects/...` passes.

### Phase 19: Effects — Medium-Priority Dynamics (Complete)

- Transient shaper: attack/release split + shaping controls; tests + runnable example.
- Lookahead limiter: delayed program path with detector + gain stage; tests + runnable example.
- `go test -race ./dsp/effects/...` passes.

### Phase 20: Nonlinear Moog Ladder Filters (Complete)

- Production-quality nonlinear Moog ladder filters in `dsp/filter/moog` with strict parameter validation and deterministic streaming behavior.
- Algorithm variants: legacy-faithful classic/improved and fast approximation modes, plus higher-quality Huovilainen-style and ZDF/TPT (Newton-iterated) topologies.
- Optional anti-alias strategy for nonlinear drive; documented quality/CPU tradeoffs.
- Parity-oriented tests vs `legacy/Source/DSP/DAV_DspFilterMoog.pas`, plus tuning/response grids, modulation stability, and nonlinear characterization tests.
- Benchmarks captured in `BENCHMARKS.md`; `go test -race ./dsp/filter/moog` passes.

### Phase 21: Goertzel Tone Analysis (Complete)

- Goertzel-based single-bin and batched multi-bin tone analysis utilities in `dsp/spectrum`, with strict input validation.
- Supports streaming (per-sample) and one-shot block workflows; exposes power/magnitude/dB outputs with safe floors.
- Parity tests vs `legacy/Source/DSP/DAV_DspGoertzel.pas` plus correctness checks against DFT/FFT references and edge-case coverage.
- Microbenchmarks + runnable examples (including DTMF-style detection); `go test -race ./dsp/spectrum` passes.

### Phase 22: Loudness Metering (EBU R128 / BS.1770) (Complete)

- Standards-aligned loudness metering (EBU R128 / ITU-R BS.1770) with streaming-friendly APIs.
- K-weighting prefilter chain per sample rate; momentary (400 ms), short-term (3 s), and integrated loudness with gating.
- Mono and stereo processors with deterministic state management; exposes LUFS metrics and peak/hold counters, plus optional allocation-free callbacks.
- Parity/conformance tests (legacy + known vectors), sample-rate matrix coverage, long-run stability tests, and benchmarks; `go test -race ./measure/...` passes.

### Phase 23: Dither and Noise Shaping (Complete)

- Quantization support with configurable dither PDFs and FIR/IIR noise-shaping processors, inspired by the legacy DAV implementations.
- Dither modes include none/rectangular/triangular/gaussian/fast-gaussian with deterministic RNG injection for reproducible tests.
- FIR error-feedback shapers with preset coefficient families and sample-rate-aware preset selection; optional lightweight IIR shelf shaping.
- Noise-shaping filter designer utility (psychoacoustic weighting objective) with deterministic mode, guardrails, and exportable coefficient outputs.
- Spectral validation tests (null/error-spectrum) and legacy parity checks, plus benchmarks for runtime and designer; `go test -race ./dsp/...` passes.

### Phase 24: Effects — High-Priority Dynamics (Planned)

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

### Phase 25: Effects — Spatial and Convolution Reverb (Planned)

Tasks:

- [x] Convolution reverb
  - [x] Implement partitioned convolution wrapper specialized for reverb usage.
  - [x] Add tests + example.
- [ ] Haas delay
  - [ ] Implement short stereo delay + constraints.
  - [ ] Add tests + example.

### Phase 26: Effects — Specialized / Lower-Priority (Planned)

Tasks:

- [x] Spectral freeze
  - [x] Implement STFT magnitude hold + phase strategy.
  - [x] Add tests + example.
- [x] Granular
  - [x] Implement grain scheduling + overlap-add.
  - [x] Add tests + example.
- [ ] Dynamic EQ
  - [ ] Implement band filter + detector + gain mapping.
  - [ ] Add tests + example.
- [ ] Stereo panner
  - [ ] Implement equal-power pan law.
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

### Phase 27: High-Order Shelving Filters (Holters/Zölzer + Orfanidis) (In Progress)

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

### Phase 28: Optimization and SIMD Paths (In Progress)

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

### Phase 29: Polyphase Hilbert / Analytic Signal (Planned)

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

### Phase 30: Interpolation Kernel Expansion (Planned)

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

### Phase 31: Vocoder (Planned)

Goal:

- Implement a channel vocoder that applies the spectral envelope of a modulator signal (typically speech) to a carrier signal (typically a synthesizer or noise), producing the classic "talking synthesizer" effect.
- Provide multiple band-layout strategies (1/3-octave ISO and Bark scale) with configurable filter order, attack/release envelopes, and dry/wet mixing.
- Based on the legacy implementation in `legacy/Source/DSP/DAV_DspVocoder.pas`, which contains three complete vocoder variants: a simple third-octave bandpass vocoder, a Bark-scale LP/HP subtractive bank vocoder, and a mixed-design vocoder with Chebyshev analysis and bandpass synthesis.

---

#### Architecture Overview

A vocoder works in three stages:

1. **Analysis bank** — the modulator signal is decomposed into N frequency bands. Each band produces a slowly-varying amplitude envelope (the "spectral shape" of the voice or modulator at that frequency region).
2. **Envelope follower** — for each band, a peak or RMS detector with configurable attack and release times tracks the band's instantaneous energy level. This is the "spectral envelope" that gives the vocoder its characteristic sound.
3. **Synthesis bank** — the carrier signal is fed through N matching bandpass (or LP/HP) filters. Each band's output is amplitude-modulated by the analysis envelope extracted in step 2. The modulated bands are summed to form the vocoded output.

A final output stage blends:

- **Vocoded mix** (`VocoderLevel`) — the summed carrier-reshaped output
- **Dry carrier** (`SynthLevel`) — the unprocessed carrier (adds brightness/presence)
- **Dry modulator** (`InputLevel`) — the unprocessed modulator (optional intelligibility aid)

---

#### Band Layout Strategies

Two layouts are required, matching the legacy variants:

**1/3-Octave ISO Bank (32 bands)**

Center frequencies follow the ISO 1/3-octave series from 16 Hz to 20 kHz:

```
16, 20, 25, 31, 40, 50, 63, 80, 100, 125, 160, 200, 250, 315, 400, 500,
630, 800, 1000, 1250, 1600, 2000, 2500, 3150, 4000, 5000, 6300, 8000,
10000, 12500, 16000, 20000
```

Each band uses a pair of 2nd-order bandpass filters (analysis and synthesis), both centered on the same ISO frequency. The synthesis bandwidth is independently configurable (Q factor), defaulting to 0.707.

The simple variant (`SimpleVocoder`) runs both analysis and synthesis entirely through bandpass filters. This is computationally straightforward but has lower frequency selectivity. Filter type: basic biquad bandpass (existing `dsp/filter/biquad` package).

**Bark Scale Bank (24 bands)**

Center frequencies follow the critical-band (Bark) scale, which approximates the perceptual frequency resolution of the human ear:

```
100, 200, 300, 400, 510, 630, 770, 920, 1080, 1270, 1480, 1720, 2000,
2320, 2700, 3150, 3700, 4400, 5300, 6400, 7700, 9500, 12000, 15500
```

Analysis and synthesis use **subtractive band splitting**: a cascade of LP/HP filter pairs at each band boundary frequency, implemented with Chebyshev Type I filters (order configurable, default 4). The band signal at each stage is the HP output; the LP output is passed to the next (lower) stage. This is more computationally efficient than running a full bandpass per band.

---

#### Envelope Follower Design

Each band has a peak-detecting envelope follower with asymmetric attack and release:

```
if |x[n]| > env[n-1]:
    env[n] = env[n-1] + (|x[n]| - env[n-1]) * attack_factor
else:
    env[n] = |x[n]| + (env[n-1] - |x[n]|) * release_factor
```

Time constants are computed from attack/release times in milliseconds:

```
attack_factor  = 1 - exp(-1 / (attack_ms  * 0.001 * sample_rate))
release_factor =     exp(-1 / (release_ms * 0.001 * sample_rate))
```

Defaults: attack = 0.5 ms, release = 2 ms. These must recompute when sample rate changes.

---

#### Downsampling / Efficiency (TVocoder variant)

For the LP/HP subtractive bank, lower frequency bands can be processed at reduced sample rates (power-of-2 decimation) since their content is band-limited. The legacy code implements this via a `DownsampleFactor` per band and a shared `FDownSampler` counter, only processing a band when `(downsample_counter % factor) == 0`.

The Go implementation should support this as an optional optimization path, controllable via a `WithDownsampling(enabled bool)` option. The default may disable downsampling for correctness and simplicity in a first pass.

---

#### Tasks

- [x] Core vocoder infrastructure
  - [x] Define `Vocoder` struct with configurable band count, layout, attack, release, and mix levels.
  - [x] Define `BandLayout` enum/type: `ThirdOctaveISO` (32 bands) and `BarkScale` (24 bands).
  - [x] Implement `WithBandLayout`, `WithAttack`, `WithRelease`, `WithInputLevel`, `WithSynthLevel`, `WithVocoderLevel` option functions. (Note: `WithFilterOrder` removed — fixed at 2nd-order CPG bandpass; `WithSynthesisBandwidth` deferred.)
  - [x] Implement `NewVocoder(sampleRate float64, opts ...Option) (*Vocoder, error)`.
  - [x] Implement `Reset()` to clear all filter and envelope state.
  - [x] Implement `SetSampleRate(sr float64)` to recompute all time constants and filter coefficients on the fly.

- [x] Analysis filter bank
  - [x] Implement 1/3-octave bandpass analysis bank: 32 CPG (constant-peak-gain) biquad bandpass filters (one per ISO band).
  - [x] Implement Bark-scale analysis bank: 24 CPG bandpass filters at Bark center frequencies with per-band Q. (Note: simplified from subtractive LP/HP Chebyshev — Chebyshev filters aren't complementary, causing gain accumulation.)
  - [x] Wire filter coefficient recomputation on sample rate change.

- [x] Envelope followers
  - [x] Implement per-band peak-follower with asymmetric attack/release as described above.
  - [ ] Provide RMS-mode alternative (optional, `WithEnvelopeMode(mode EnvelopeMode)`) using a leaky integrator on the squared signal.
  - [x] Recompute attack/release factors on sample rate or time-constant change.

- [x] Synthesis filter bank
  - [x] Implement 1/3-octave bandpass synthesis bank: 32 CPG biquad bandpass filters (same ISO center frequencies).
  - [x] Implement Bark-scale synthesis bank: 24 CPG bandpass filters mirroring analysis bank.
  - [x] Implement carrier modulation: filter carrier through synthesis bandpass, then multiply by analysis envelope before summing.

- [x] Output mixing
  - [x] Sum all modulated synthesis band outputs.
  - [x] Apply three-way blend: `output = vocoder_level * vocoded + synth_level * carrier + input_level * modulator`.
  - [x] All level factors are linear (not dB) at the processing layer.

- [x] Processing API
  - [x] Implement `ProcessSample(modulator, carrier float64) float64` for sample-by-sample streaming.
  - [x] Implement `ProcessBlock(modulators, carriers, output []float64) error` for block processing (zero-alloc, in-place output).
  - [ ] Document latency: both filter banks introduce group delay that is inherently asymmetric between IIR filter types; document this clearly.

- [x] Optional downsampling optimization
  - [x] Implement per-band downsample factor computation for both layouts (power-of-2 based on band frequency vs. Nyquist).
  - [x] Implement bitmask-based counter mechanism so lower bands only run analysis on every Nth sample; synthesis always runs at full rate.
  - [x] Guard with `WithDownsampling(true)` option and `SetDownsampling()` setter; default off.

- [x] Tests
  - [x] Unit test: `ProcessSample` with silence modulator produces silent output.
  - [x] Unit test: non-silent output for non-silent modulator+carrier.
  - [x] Unit test: envelope asymmetry (attack vs release time constants).
  - [x] Unit test: band-layout frequencies match expected ISO and Bark tables.
  - [x] Unit test: `SetSampleRate` recomputes coefficients correctly.
  - [x] Unit test: `ProcessBlock` equivalence with `ProcessSample`.
  - [x] Unit test: `Reset` clears state. Setter/getter roundtrip. Setter validation. Dry mix. Output bounded. Bark layout.
  - [x] Benchmark: `ProcessSample` for both band layouts; 0 allocs/op, ~192ns (ThirdOctave), ~212ns (Bark).

- [x] Review follow-up hardening (DSP correctness + parity)
  - [x] Fix analysis downsampling path so per-band analysis frequency response is preserved (do not skip IIR state evolution); implement proper decimation strategy or equivalent full-rate analysis with decimated control updates.
  - [x] Make envelope attack/release timing invariant in milliseconds when downsampling is enabled (factor-aware coefficient derivation and tests).
  - [x] Resolve Bark Q behavior mismatch: either default Bark synthesis Q to per-band Bark-derived values or explicitly split analysis/synthesis Q options and documentation.
  - [x] Align comments/docs with implementation (`barkSynthQ` naming/usage, Bark analysis filter description, downsample factor rule text).
  - [x] Add downsample quality regression tests stronger than broad RMS checks (band-energy error and/or STFT-distance thresholds vs full-rate baseline).
  - [x] Add targeted tests for high-frequency/Nyquist-near bands under varying synthesis Q to validate stable/expected behavior.

- [ ] Example
  - [ ] Add `example_test.go` demonstrating: construct a Bark-scale vocoder, feed a speech-like modulator (sine sweep) and synthesizer-like carrier (sawtooth approximation), run `ProcessBlock`, print output energy.

Exit criteria:

- [x] Both `ThirdOctaveISO` and `BarkScale` layouts produce non-silent output for non-silent inputs.
- [x] Envelope followers have correct asymmetric time constant behavior verified by unit tests.
- [x] `go test -race ./dsp/effects/...` passes with no data races.
- [x] `ProcessBlock` shows zero heap allocations in benchmark.
- [ ] Runnable example compiles and runs without error.

---

### Phase 35: API Stabilization and v1.0 (In Progress)

Tasks:

- [ ] Run full benchmark pass and confirm no major regressions vs baselines.
- [ ] Run full CI locally (`just ci`) including race (`go test -race ./...`).
- [ ] Confirm `CHANGELOG.md` and `MIGRATION.md` are complete for `v1.0.0`.
- [ ] Complete `API_REVIEW.md` checklist.
- [ ] Tag and publish `v1.0.0` (tag + release notes).
- [ ] Verify module proxy indexing (`go get` via `GOPROXY`).

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

| Version | Date       | Author  | Changes                                                       |
| ------- | ---------- | ------- | ------------------------------------------------------------- |
| 0.1     | 2026-02-06 | Codex   | Initial comprehensive plan                                    |
| 0.2     | 2026-02-06 | Claude  | Expanded early phases + migration notes                       |
| 0.3     | 2026-02-08 | Claude  | Added shelving filter design phase + known Chebyshev II bug   |
| 0.4     | 2026-02-20 | Copilot | Restored detailed plan + added checkable tasks for all phases |

---

This plan is a living document and should be updated after each phase completion and major architectural decision.
