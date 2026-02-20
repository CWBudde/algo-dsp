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
6. Detailed Phase Plan (Phases 0–25)
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
Phase 17: Effects — High-Priority Spatial             [1 week]   📋 Planned
Phase 18: Effects — Medium-Priority Waveshaping/Lo-fi [2 weeks]  📋 Planned
Phase 19: Effects — Medium-Priority Modulation        [2 weeks]  📋 Planned
Phase 20: Effects — Medium-Priority Dynamics          [2 weeks]  📋 Planned
Phase 21: Effects — Spatial and Convolution Reverb    [2 weeks]  📋 Planned
Phase 22: Effects — Specialized / Lower-Priority      [4 weeks]  📋 Planned
Phase 23: High-Order Shelving Filters                  [2 weeks]  🔄 In Progress
Phase 24: Optimization and SIMD Paths                 [3 weeks]  🔄 In Progress
Phase 25: API Stabilization and v1.0                  [2 weeks]  🔄 In Progress
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

- [ ] Flanger
  - Algorithm: short modulated delay (0.1–10 ms) with feedback and wet/dry.
  - API sketch:

    ```go
    func NewFlanger(sampleRate float64, opts ...Option) (*Flanger, error)
    func (f *Flanger) Process(sample float64) float64
    func (f *Flanger) ProcessInPlace(buf []float64) error
    func (f *Flanger) Reset()
    ```

  - [ ] Implement core algorithm (short modulated delay, feedback, mix).
  - [ ] Implement interpolation tap (reuse chorus approach).
  - [ ] Add tests (parameter validation + basic response sanity).
  - [ ] Add runnable example.

- [ ] Phaser
  - Algorithm: allpass cascade with LFO modulation.
  - [ ] Implement allpass cascade (4–12 stages) with LFO modulation.
  - [ ] Add tests + example.
- [ ] Tremolo
  - Algorithm: amplitude modulation with LFO and optional smoothing.
  - [ ] Implement LFO amplitude mod + smoothing.
  - [ ] Add tests + example.

Exit criteria:

- [ ] `go test -race ./dsp/effects/` passes with new effects.

### Phase 16: Effects — High-Priority Dynamics (Planned)

Tasks:

- [ ] De-esser
  - [ ] Implement split-band detection and reduction.
  - [ ] Add tests + example.
- [ ] Expander
  - [ ] Implement downward expander (envelope + gain computer).
  - [ ] Add tests + example.
- [ ] Multiband compressor
  - [ ] Implement crossover + per-band compressor + recombine.
  - [ ] Add tests + example.

### Phase 17: Effects — High-Priority Spatial (Planned)

Tasks:

- [ ] Stereo widener
  - [ ] Implement M/S gain controls with safe bounds.
  - [ ] Add mono-compatibility tests + example.

### Phase 18: Effects — Medium-Priority Waveshaping/Lo-fi (Planned)

Tasks:

- [ ] Distortion
  - [ ] Implement waveshapers (soft/hard clip, tanh).
  - [ ] Add tests + example.
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
