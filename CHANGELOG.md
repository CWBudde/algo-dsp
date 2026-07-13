# Changelog

All notable changes to this project are documented in this file.

## [Unreleased]

### Fixed (measurement math)

- `measure/thd`: aggregate metrics (THD+N, Noise, SINAD, OddHD, EvenHD,
  RubNBuzz) are now computed by power summation (root-sum-square) instead of
  summing linear magnitudes. The old math inflated wideband terms by roughly
  sqrt(bin count): a pure sine with a -84 dB white noise floor reported
  SINAD ~30 dB low at realistic FFT sizes.
- `measure/thd`: THD now follows the IEEE THD_R definition
  (sqrt(sum of harmonic powers)/fundamental). Two harmonics of 1% each
  report 1.414%, where the previous linear sum reported 2%. The
  per-harmonic `Result.Harmonics` amplitude ratios are unchanged.
- `measure/thd`: `AnalyzeSignal` no longer panics (index out of range) when
  `len(signal) > Config.FFTSize`; the input is truncated to the FFT size and
  the window is applied to the analyzed segment.
- `measure/sweep`: the log-sweep inverse filter used an inverted amplitude
  envelope (proportional to 1/f instead of f), tilting deconvolved spectra
  by about -12 dB/octave; its closed-form normalization also left the
  identity-system peak well below unity. The envelope now follows Farina's
  method and the filter is normalized exactly, so identity round trips yield
  a unit impulse at sample len(inverse)-1 with an in-band flat spectrum.
- `measure/sweep`: the linear-sweep inverse filter truncated a circular
  spectral inverse to the sweep length, discarding most of the filter energy
  (self-deconvolution peaked at the wrong index with amplitude ~0.006). It
  is now the energy-normalized matched filter (time-reversed sweep), which
  is exact for the linear chirp's flat in-band spectrum.

### Added

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

- Benchmark code in `measure/ir` and `measure/sweep` now handles returned errors to satisfy release lint gates.
- Public implementation comments were cleaned to remove open work-item markers in Phase 25-facing code.

### Fixed

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
