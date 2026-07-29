# Changelog

All notable changes to this project are documented in this file.

## [Unreleased]

### Added

- `shelving.EllipticLowShelf` / `shelving.EllipticHighShelf` (Phase 32): high-order elliptic (Cauer) shelving designers, equiripple on both sides of the transition, for `order >= 1` including odd orders. The reference-side ripple is the `stopbandDB` argument; the shelf-side ripple is fixed at 0.05 dB, matching `band.EllipticBand`.
- Runnable examples and benchmarks for `dsp/filter/design/shelving`, which previously had neither.
- Web demo: the elliptic family is now selectable for the low- and high-shelf EQ node types, wired to the new shelving designers. The node's shape control acts as the reference-side ripple bound, as it already does for Chebyshev shelves.
- `pitch.YINDetector`, `pitch.PitchTracker` and `pitch.PitchCorrector` (Phase 36): YIN fundamental frequency estimation (difference function, cumulative mean normalization, parabolic interpolation) with a zero-allocation `Detect`; a streaming tracker adding hop scheduling, median smoothing and an unvoiced hold; and auto-tune style correction that drives any `PitchProcessor` to snap a signal to a scale or a fixed target, with correction amount, a clamped maximum, a confidence gate and a retune glide.
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
