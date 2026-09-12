# Changelog

All notable changes to this project are documented in this file.

## [Unreleased]

### Changed

- Phase 41c, first half: six hand-rolled loops now call the `algo-vecmath` kernels that were already available and simply unused. `AddBlockInPlace` takes the partitioned-convolution overlap-add (`dsp/conv/partitioned.go`), the streaming overlap-add tail merge (`dsp/conv/streaming_overlap_add.go`), the multiband band sum (`dsp/effects/dynamics/multiband.go`) and the parent-edge mix (`dsp/effectchain/chain_process.go`); `ScaleBlock` takes that mix's output scaling; and `ScaleBlockInPlace` takes the log-sweep inverse-filter normalization (`measure/sweep/sweep.go`). The two zeroing loops in the mix became `clear`.

  **Every one of these is bit-identical on amd64 and arm64, by construction.** They are element-wise adds and element-wise scalar multiplies -- there is no multiply-add to fuse and no reassociation, so the hazard behind the `d2ec9ef` AXPY change (bit-identical on amd64, an ulp apart on arm64) cannot recur here.

  Measured on amd64/AVX2 (Ryzen 5 4600H), before vs after on the same benchmarks: `BenchmarkPartitionedConvolution`, `BenchmarkMultibandProcessInPlace` and `BenchmarkLogSweepInverseFilter` show **no significant change** (p >= 0.33) -- in each the substituted loop is a small fraction of what the benchmark measures, so the gain is real but below the noise floor of the enclosing function. Allocation counts are unchanged everywhere, and the touched paths are pinned allocation-free by new `testing.AllocsPerRun` assertions.

- `spectrum.GoertzelBank.ProcessBlock` advances four bins per pass over the sample buffer instead of one. The recurrence is serial within a bin but the bins are independent, so a group of four reads the buffer once rather than four times and gives the processor four independent dependency chains to overlap -- the single-bin loop is latency-bound on the multiply-add, not throughput-bound. Bins past the last full group of four fall through to the existing per-bin path. **3.8x for four bins** (13.6 -> 3.5 us over a 1024-sample block) and **4.2x for the eight-bin DTMF case** (27.1 -> 6.4 us); a bank of one, two or three bins is unchanged. Output is **bit-identical per bin** -- `TestGoertzelBankProcessBlockBitExact` compares with `==`, not a tolerance, across bin counts 1-9 and block lengths 0-1024. The recurrence itself was factored into one `goertzelStep` helper shared by the single-bin, bank and four-wide paths so no two of them can diverge in how the compiler contracts the multiply-add.

- The vecmath call sites above are guarded by benchmarked length thresholds (`mixSIMDThreshold`, `addBlockSIMDThreshold`, `bandSumSIMDThreshold`, all 64), in the same style as the existing `conv.simdThreshold`. This is not caution: a dispatched vecmath call carries roughly 110 ns of fixed cost on this machine regardless of length, so the four-parent mix loses 0.72x at a 16-sample block before turning over to 3.0x at 512. Short buffers are not hypothetical here -- a partitioned convolver built with a low `minBlockOrder` has a `partSize` of a handful of samples.

- `vecmath.MaxAbs` was **not** adopted, in `stats/time.Peak` or in `measure/ir`'s impulse-onset search, although it is 3.0x and 5.6x faster there respectively. It is unsafe for both: on AVX2 a NaN anywhere in the slice can **discard the true maximum and return a smaller finite value**. `MaxAbs([999, NaN, 0.5])` returns `0.5` on AVX2 and `999` on the pure-Go kernel. That is not a difference in NaN policy -- it is a silently wrong *finite* result, on some CPUs only, and no cheap check detects it, because an `IsNaN` test on the result does not fire. In `findImpulseStart` a peak under-reported by that factor leaves the threshold orders of magnitude too low, so the quiet run before the impulse clears it and the reported onset is far too early, corrupting every metric derived from it. Detecting NaN up front costs a full extra pass, which is the entire speedup, so both sites keep their scalar loops and `TestPeakNaNIsDeterministic` / `TestFindImpulseStartNaNIsDeterministic` fail on an AVX2 machine if anyone re-adopts `MaxAbs`. Raised by review on #25. This looks like an `algo-vecmath` defect rather than a documentation gap and is worth fixing there.

- Not covered, deliberately: `algo-vecmath` v0.1.3 is float64-only, so `PartitionedConvolution32`, `StreamingOverlapAdd32` and the other `float32` instantiations keep their scalar loops. The generic helper dispatches on `unsafe.Sizeof` rather than boxing through `any()`, so that branch resolves at compile time per instantiation and the float32 stencils are byte-for-byte unchanged.

## [v0.7.1] - 2026-09-08

### Changed

- Requires `algo-approx` v0.2.0, up from v0.1.0. That release removes `FastSqrt`, `FastInvSqrt` and `FastRecip`, none of which algo-dsp ever referenced -- the only call sites here are `approx.FastLog` and `approx.FastExp` in the `fastmath`-tagged `dynamics` helpers, both unchanged. It also adds a public batch API and NEON kernels for batch exp and fused tanh/log(cosh), neither adopted here yet.

- `reverb.FDNReverb.ProcessSample` no longer calls `math.Sin` and no longer multiplies the feedback matrix out. The modulator is one unit-magnitude complex rotor advanced per sample, with each delay line's fixed phase offset applied by angle addition, which removes eight sine calls per sample; the Hadamard mixing runs as a three-stage fast Walsh-Hadamard butterfly, 24 add/subtracts against 64 multiplies and 56 adds. **2.2x faster** on a 128-sample block (34.8 -> 16.0 us, amd64, best of three), and the mixing alone is 3.3x (66.4 -> 20.4 ns). The gain is larger where `math.Sin` has no hardware instruction behind it: measured in a browser under `GOARCH=wasm`, a stereo pair of these went from 89-179% of realtime to 45-55% in Firefox and from 9-13% to 4.5-5% in Chromium. Output is unchanged to within 4.1e-11 over ten seconds at 48 kHz -- roughly three decimal digits below a 24-bit LSB -- the difference being rounding: a rotor against a sine, and pairwise against left-to-right summation. `TestFDNReverbMatchesReference` keeps the previous implementation and checks against it.

## [v0.7.0] - 2026-08-15

### Changed

- Requires `algo-fft` v0.8.0, up from v0.7.3. The only breaking change in that range is the removal of the `KernelEightStep` kernel strategy constant, which duplicated `KernelSixStep` — algo-dsp never referenced it, so no call site changed. The upgrade also brings the additive v0.8.0 surface (`ConvolveReal64`, the reusable `Convolver`/`Correlator`/`RealConvolver` types, `CurrentCPUIdentifier`, and per-plan algorithm reporting via `Plan.ForwardAlgorithm`/`InverseAlgorithm`), none of which is adopted here yet.
- `conv.DirectTo` accumulates through the single fused `vecmath.AddScaledBlockInPlace` (AXPY) kernel instead of scaling the kernel into a scratch buffer with `ScaleBlock` and adding that buffer in with `AddBlockInPlace`. The two-pass form did an extra write and read of the whole kernel on each of the `n` outer iterations; the scratch buffer and its `sync.Pool` are gone entirely. Measured 1.26x-2.28x on `BenchmarkDirect` for kernels of 32 and 64 taps (amd64); kernels below the 16-tap SIMD threshold take the scalar path and are unaffected. Results are bit-identical on amd64. On arm64 they differ by up to an ulp, because the NEON AXPY kernel fuses the multiply-add where the two-pass form rounded twice.
- Requires `algo-vecmath` v0.1.3, up from v0.1.0. That release fixes an out-of-bounds write in the arm64 kernels — reachable from any length-1 slice, and a reproducible segfault in `ScaleBlock` on Apple Silicon — so this is a correctness-relevant bump for arm64 builds, not only a performance one. It also carries a rewrite of the NEON backend that lifts `DotProduct` by 3.0x-3.9x and `Sum` by 2.1x-2.5x, which `filter/fir` and the statistics packages inherit for free.

## [v0.6.0] - 2026-08-08

Note: v0.2.0 through v0.5.1 were tagged without their own headings, so the entries
below cover everything accumulated since v0.1.0, not only what is new in v0.6.0.

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
