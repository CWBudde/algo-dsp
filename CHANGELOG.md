# Changelog

All notable changes to this project are documented in this file.

## [Unreleased]

### Added

- `biquad.Identity()` returns the pass-through second-order section (`B0 = 1`, all other coefficients zero, `H(z) = 1`), and `biquad.Coefficients.IsZero()` reports whether every coefficient is zero. `Identity()` is the failure return of the single-section designers in `dsp/filter/design` (see Changed); `IsZero` lets callers detect an accidentally zero-valued — that is, muting — `Coefficients` before installing it in a chain.
- `biquad.Section.FlushDenormals()` and `biquad.Chain.FlushDenormals()` flush delay-line state whose magnitude has decayed below 1e-30 to exact zero, matching the threshold of `core.FlushDenormals`. Go enables neither FTZ nor DAZ, so the residual tail of a decaying signal leaves denormal state that drags the audio callback onto the slow denormal microcode path. Both are allocation-free (pinned by `testing.AllocsPerRun` and by benchmarks) and intended to be called once per block from a real-time callback; the previously available route, `Chain.State()`, allocates a `[][2]float64` and was unusable there.
- `shelving.EllipticLowShelf` / `shelving.EllipticHighShelf` (Phase 32): high-order elliptic (Cauer) shelving designers, equiripple on both sides of the transition, for `order >= 1` including odd orders. The reference-side ripple is the `stopbandDB` argument; the shelf-side ripple is fixed at 0.05 dB, matching `band.EllipticBand`.
- Runnable examples and benchmarks for `dsp/filter/design/shelving`, which previously had neither.
- Runnable examples for `effects.Vocoder` (Phase 33): default construction, `ProcessBlock` envelope transfer, the Bark band layout with a synthesis-Q override, and multirate analysis via `WithDownsampling`. The vocoder was the last effect in `dsp/effects` without examples; coverage of its exported options, getters and setters is now complete.
- Benchmark regression guard (Phase 40): `cmd/benchguard` compares `go test -bench` output against the checked-in `benchmarks/baseline.json`. Allocation counts gate exactly and `B/op` gets +10% headroom, because both are deterministic and machine-independent; `ns/op` is reported with a +50% bound but does not gate unless `-enforce-timing` is passed and the baseline came from the current machine — repeat runs with no code change moved benchmarks by 43% on an idle machine and up to 7x under load, so no timing threshold is both loose enough to survive that and tight enough to be useful. Repeated `-count=N` observations collapse to their minimum. New `just bench-guard` and `just bench-baseline` recipes, a broadened `just bench-ci` (6 packages / 20 benchmarks, with a `count` parameter), and an advisory `Benchmark Guard` CI job. Tooling only — no library API change.
- Web demo: the elliptic family is now selectable for the low- and high-shelf EQ node types, wired to the new shelving designers. The node's shape control acts as the reference-side ripple bound, as it already does for Chebyshev shelves.
- `pitch.YINDetector`, `pitch.PitchTracker` and `pitch.PitchCorrector` (Phase 36): YIN fundamental frequency estimation (difference function, cumulative mean normalization, parabolic interpolation) with a zero-allocation `Detect`; a streaming tracker adding hop scheduling, median smoothing and an unvoiced hold; and auto-tune style correction that drives any `PitchProcessor` to snap a signal to a scale or a fixed target, with correction amount, a clamped maximum, a confidence gate and a retune glide. The corrector queues its input, so the block timing is independent of the caller's buffer size; its output lags the input by one block plus the seam crossfade, as reported by `Latency`.
- `pitch.Scale`, `pitch.PitchClass` and the note-conversion helpers `FrequencyToMIDI`, `MIDIToFrequency`, `SemitonesToRatio`, `RatioToSemitones` and `CentsBetween`.
- `dynamics.DynamicEQ` (Phase 35): a series chain of parametric EQ bands whose gain is driven by a per-band detector. Peaking and shelving shapes, static/downward/upward/upward-below modes, band-filtered or external sidechain detection, control-rate coefficient updates (`SetUpdateInterval`), per-band metering, and `BandStaticCurve`.
- New public package `dsp/interp` with reusable cubic Hermite interpolation (`Hermite4`) and a configurable `LagrangeInterpolator`.
- New public package `dsp/delay` with reusable circular delay-line primitives, including integer and fractional-delay reads.
- Added `core.FlushDenormals` for denormal-safe hot loops.

- Phase 25 API stabilization artifacts: `API_REVIEW.md`, `MIGRATION.md`, and `BENCHMARKS.md`.
- Runnable examples for previously uncovered major public packages:
  - `dsp/buffer`
  - `dsp/core`
  - `dsp/signal`
  - `stats/time`
  - `stats/frequency`

### Changed

- **Breaking** — an undesignable filter is now transparent instead of silent. The `dsp/filter/design` functions that return a single `biquad.Coefficients` and have no way to report an error return `biquad.Identity()` — a pass-through section — when the parameters cannot be honoured (non-positive or non-finite sample rate, frequency outside the open interval `(0, Nyquist)`, or a degenerate `a0`). They previously returned the zero `biquad.Coefficients{}`, which has `B0 = 0` and therefore outputs identical silence: a single out-of-range band muted an entire cascade, which is how a downstream plugin was silenced by requesting a 20 kHz cutoff at a 32 kHz sample rate. Affected: `design.Lowpass`, `Highpass`, `Bandpass`, `Notch`, `Allpass`, `Peak`, `LowShelf`, `HighShelf`, `pass.LowpassRBJ`, `pass.HighpassRBJ`, and — through their per-section designers — the cascades `design.ButterworthLP` / `ButterworthHP` and `pass.ButterworthLP` / `ButterworthHP`. `design.PeakRaw` also returns `Identity()` alongside its `ErrInvalidPeakParams`, so a caller that ignores the error passes audio rather than muting it. Permitted under the `v0.x` pre-release policy. Callers that need to know whether a design succeeded should validate the parameters themselves or use the `design/band` and `design/shelving` designers, which return an explicit error. The cascade designers that fail with a `nil` slice (`BesselLP`/`BesselHP`, `Chebyshev1*`, `Chebyshev2*`, `Elliptic*`, `LinkwitzRiley*`) are unchanged: an empty cascade cannot be mistaken for a mute. The `dsp/filter/design/pass` designers reject non-finite frequencies and sample rates explicitly rather than relying on range comparisons, which are all false against `NaN`; a `NaN` sample rate previously produced `NaN` coefficients and an infinite one a muting section. `design.PeakCascade` keeps reporting `ErrInvalidPeakParams` when the RBJ normalization degenerates — for instance at a gain negative enough to underflow `math.Pow` — instead of returning transparent sections with a `nil` error.
- **Breaking** — `shelving.Chebyshev2LowShelf` / `Chebyshev2HighShelf` are reimplemented as genuine Chebyshev Type II designs and produce different coefficients. They previously delegated to a Butterworth shelf designed at `gainDB - stopbandDB`, which has no equiripple stopband at all. The rebuilt designers are equiripple in the flat region (bounded by `stopbandDB`) and monotonic across the shelf, which now reaches `gainDB` exactly instead of `gainDB - sign(gainDB)*stopbandDB`. Permitted under the `v0.x` pre-release policy.
- **Breaking** — `shelving.Chebyshev2*Shelf` now interprets `freqHz` with the package-wide cutoff convention `|H(freqHz)|² = (G² + 1)/2` shared with the Butterworth and Chebyshev I designers, so the transition band sits at a different place than before. A side effect is that boost and cut are no longer exact reciprocals through the transition; that asymmetry is inherent to this cutoff convention and already applied to the Butterworth and Chebyshev I families (Holters & Zölzer §2.3).
- `PitchShifter` and `SpectralPitchShifter` now use the shared `SemitonesToRatio` / `RatioToSemitones` helpers instead of inlined semitone formulas. Behaviour is unchanged.
- `design.Peak` no longer allocates when called without `PeakOption`s, making runtime coefficient redesign allocation-free.
- Benchmark code in `measure/ir` and `measure/sweep` now handles returned errors to satisfy release lint gates.
- Public implementation comments were cleaned to remove open work-item markers in Phase 25-facing code.

### Fixed

- Removed the dead `chebyshev2Sections` helper from `dsp/filter/design/shelving/lowshelf.go`. It carried empirical damping constants (`3.65`, `16.499`, `0.2`) that compensated for a lost frequency scaling in its σ/R² reparametrization; the correct Orfanidis prototype now lives in `internal/orfanidis`. The unused `invertSections` helper went with it.
- Removed unused helper in `measure/ir/ir_test.go` flagged by lint.
- Applied formatting fixes in IR/sweep package files.

## [v0.1.0] - 2026-02-07

### Added

- Initial reusable DSP package scaffolding across:
  - `dsp/window`, `dsp/conv`, `dsp/resample`, `dsp/spectrum`, `dsp/signal`
  - `dsp/filter/{biquad,fir,design,bank,weighting}`
  - `measure/{thd,sweep,ir}`
  - `stats/{time,frequency}`
- Core utilities in `dsp/core` and buffer utilities in `dsp/buffer`.
- Test and benchmark coverage across algorithm packages.
