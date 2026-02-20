# algo-dsp: Development Plan (Compact)

This is the living roadmap for `github.com/cwbudde/algo-dsp`.

- **Purpose**: reusable DSP algorithms in Go (no app/UI/IO concerns).
- **Audience**: contributors and future-me doing extractions from `mfw`.
- **Last updated**: 2026-02-20

---

## Guardrails (non-negotiable)

- No UI/audio-device/file-container dependencies (Wails/CoreAudio/JACK/WAV/etc.).
- Algorithm-centric, transport-agnostic APIs (streaming-friendly + deterministic).
- Prefer zero-allocation hot paths (but keep readable scalar reference first).
- Public APIs: doc comments + runnable examples.
- Errors: wrap with context (`fmt.Errorf("context: %w", err)`).

---

## What to do next (bitesize, ordered)

These are ordered by “unblocks v1.0 + current failing tests” first.

### M0 — v1.0 blockers (ship quality)

**T-001 (S)** Fix Chebyshev Type II shelving shape

- Where: `dsp/filter/design/shelving` (`Chebyshev2LowShelf`, `Chebyshev2HighShelf`)
- Current state: compiles; tests exist; shelf shape is wrong (nearly flat).
- Acceptance:
  - All Chebyshev II shelving tests pass (currently 8 failing in that suite).
  - Low-shelf: DC ≈ target gain, Nyquist ≈ 0 dB; High-shelf: inverse.

**T-002 (M)** Implement elliptic shelving (after T-001)

- Where: `dsp/filter/design/shelving`
- Acceptance:
  - New `EllipticLowShelf/HighShelf` designers returning SOS.
  - Stability + response conformance tests (DC/Nyquist/cutoff region).

**T-003 (S)** Benchmark regression guard policy + docs

- Where: `BENCHMARKS.md`, `justfile` (target like `just bench-ci` if not already present)
- Acceptance:
  - Document baseline update workflow and regression thresholds.
  - Produce CI-friendly, short benchmark subset output (machine-readable is a plus).

**T-004 (S)** Refresh baselines on at least 2 machines

- Acceptance:
  - Update `BENCHMARKS.md` with date, Go version, and at least amd64 + arm64 (if available).

**T-005 (S)** v1.0 release checklist

- Where: `API_REVIEW.md`, `CHANGELOG.md`, `MIGRATION.md`
- Acceptance:
  - `just ci` / `go test -race ./...` clean.
  - Tag + release notes ready for `v1.0.0`.

### M1 — Post-v1 performance & SIMD (optional / can be parked)

**T-101 (M)** Advisory perf regression gate in CI

- Acceptance: CI emits advisory warnings on large regressions vs baselines; non-blocking by default.

**T-102 (L)** Modal / quadrature oscillator bank (depends on `algo-vecmath`)

- Goal: reusable SIMD-friendly damped complex rotator bank (first consumer: `algo-piano`).
- Acceptance:
  - New package (candidate `dsp/osc` or `dsp/modal`) with scalar reference + tests.
  - At least one accelerated backend shows measurable speedup.

### M2 — Effects roadmap (planned; keep algorithm-only)

Policy for each new effect:

- Constructor + options (`NewX(sampleRate, opts...)`).
- One-shot and/or in-place processing (`Process`, `ProcessInPlace`) + `Reset()`.
- Table-driven tests + one runnable example.

**T-201 (S)** Flanger

- Short modulated delay + feedback + mix (reuse delay + interpolation approach from chorus).

**T-202 (S)** Phaser

- 4–12 stage allpass cascade with LFO-modulated center frequency.

**T-203 (XS)** Tremolo

- LFO amplitude modulation + smoothing.

**T-211 (M)** De-esser

- Split-band detector + gain reduction on sibilance band.

**T-212 (M)** Expander

- Downward expander based on existing gate/compressor math and envelope tracking.

**T-213 (L)** Multiband compressor

- Crossover bank + per-band compressor + recombine.

**T-221 (S)** Stereo widener

- Mid/Side gain control with safe mono compatibility tests.

**T-231 (S)** Distortion

- Waveshaping (tanh/soft clip/hard clip) + oversampling option if needed (algorithm-only).

**T-232 (S)** Bit crusher

- Bit depth + sample-rate reduction.

**T-241 (M)** Auto-wah

- Envelope follower modulating a bandpass/peaking filter.

**T-242 (M)** Ring modulator

- Multiply by carrier oscillator + optional mix.

**T-251 (M)** Transient shaper

- Attack/release envelope split + gain shaping.

**T-252 (M)** Lookahead limiter

- Delay line + peak detector + gain computer.

**T-261 (L)** Convolution reverb

- Partitioned convolution + wet/dry; no file I/O (impulse is caller-provided).

**T-262 (S)** Haas delay

- Short stereo delay with crossfeed parameters + safety limits.

**T-271 (L)** Spectral freeze

- STFT magnitude hold + phase strategy.

**T-272 (L)** Granular

- Grain windowing + overlap-add (no IO; caller provides samples).

**T-273 (L)** Dynamic EQ

- Band filter + level detector + gain mapping.

**T-274 (M)** Stereo panner

- Equal-power pan law + optional stereo width.

**T-275 (L)** Vocoder

- Analysis filter bank + envelope followers + carrier shaping.

**T-276 (L)** Pitch correction

- YIN pitch detection + existing spectral shifter.

**T-277 (L)** Noise reduction

- Noise profile + spectral subtraction / Wiener.

---

## Current status snapshot

- **In progress**: high-order shelving filters (Chebyshev II bug), perf guard + v1.0 stabilization.
- **Planned**: effects backlog.

---

## Completed work (condensed)

Completed work is intentionally summarized here; details live in code, tests, and docs.

| Area                     | Summary                                                             |
| ------------------------ | ------------------------------------------------------------------- |
| Bootstrap / repo hygiene | go module + justfile + CI + contributing docs                       |
| Core utilities           | numeric helpers, options, deterministic test helpers                |
| Buffers                  | `dsp/buffer` Buffer + Pool (real-time friendly)                     |
| Windows                  | 25+ window functions + metadata (ENBW, sidelobes, etc.)             |
| Filter runtime           | biquad DF-II-T sections + chain + response; FIR direct-form runtime |
| Filter design            | RBJ designers + Butterworth/Chebyshev cascades                      |
| Weighting & banks        | A/B/C/Z weighting + octave/fractional-octave banks                  |
| Spectrum utils           | magnitude/phase/group delay + smoothing                             |
| Convolution              | direct + overlap-add/save; correlation; deconvolution               |
| Resampling               | polyphase FIR resampler with quality modes                          |
| Signal utilities         | generators + normalize/clip/DC removal/envelopes                    |
| Measurement              | THD/THD+N, sweeps + IR analysis                                     |
| Stats                    | time + frequency stats (streaming + one-shot), zero-alloc           |
| EQ design (advanced)     | Orfanidis peaking + high-order graphic EQ band designers            |

---

## Appendices (still relevant, kept short)

### A — Testing & validation

- Use table-driven tests and edge-case coverage.
- Prefer golden vectors + property checks for invariants.
- Target coverage: project ≥ 85%, core packages ≥ 90% (pragmatically).

### B — Benchmarking

- Microbenchmarks for hot paths + scenario benches.
- Track `ns/op`, `B/op`, `allocs/op`.
- Keep baselines in `BENCHMARKS.md` and document update workflow.

### C — Dependencies & versioning

- Keep deps minimal; `algo-fft` only via narrow integration.
- Support latest Go stable + previous stable.
- Use semantic versioning; document breaking changes.

### D — Release engineering

- Conventional commits; tag-driven releases.
- Release gates: lint + tests + race + benchmark sanity + docs/examples.

### E — Migration from `mfw` (high-level)

Extraction order (done historically, still the preferred pattern):

1. Windows → filter runtime/design → banks/weighting
2. Spectrum/conv/resample
3. Measurement + stats

Mechanics:

- Move code with tests; then switch imports; then remove duplication.

---

## Revision history

This document was previously a fully expanded multi-thousand line plan. It was intentionally
condensed to keep the "next actionable work" visible.
