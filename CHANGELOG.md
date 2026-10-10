# Changelog

All notable changes to this project are documented in this file.

## [Unreleased]

### Added

- New package `dsp/cqt`: a multi-rate constant-Q transform compatible with
  nnAudio `CQT2010v2`, the front end of spotify/basic-pitch. A single set of
  top-octave kernels is applied per octave as a strided convolution while the
  signal is low-passed and decimated by 2, with nnAudio's early downsampling,
  basis norms (L1, L2, none), reflect/constant padding, `librosa`,
  `convolutional` and `wrap` normalization, and magnitude or complex output.
  `New(sampleRate, opts...)`, `Process`, zero-allocation `ProcessInto` and
  `ProcessInto32`, `NumFrames`, `Frequencies`, `Lengths`, `Kernels`, `Lowpass`
  and `Clone`. Output is frame-major (frames × bins). Golden tests against
  nnAudio 0.3.4 cover eight configurations. basic-pitch's configuration
  (22050 Hz, hop 256, 27.5 Hz, 36 bins per octave, 309 bins) matches nnAudio
  as shipped (float32) to within 1e-5 of each bin's peak. Unlike nnAudio, which
  ignores its `window` argument, `WithWindow` is honoured. Hop lengths the
  octaves cannot share are rejected instead of failing inside the transform.
- `design.Firwin2(numtaps, freq, gain, opts...)`: frequency-sampling FIR
  design with `scipy.signal.firwin2` parity (default `nfreqs`, Hamming window,
  `WithNFreqs`, `WithSampleRate`, `WithWindow`, `WithoutWindow`). The
  antisymmetric types III and IV are not supported yet.
- `window.Type.Valid` reports whether a value is one of the declared window
  types, and `window.Type.String` returns its display name: `Info(t).Name`
  where metadata exists, names such as "Lawrey 5T" or "Albrecht 4T" for the
  types without metadata, and "Type(N)" for unknown values. `window.Generate`
  now documents that an unknown type is treated as rectangular (all ones).

### Changed

- `stft.WithWindow` now rejects unknown window types with `ErrInvalidWindow`
  (previously they silently produced a rectangular window).

## [v0.12.4] - 2026-10-07

- Dynamic EQ defaults to three peak bands at 120 Hz, 1 kHz and 8 kHz, with
  logarithmic span compression for lower sample rates. Explicit saved settings
  remain authoritative; sparse runtime graphs use the same catalogue defaults.
- Expose each dynamic EQ band's actual steady-state gain computer through
  `DynamicEQ.BandCurve` and effectchain `Transfer`'s `responseBand` selection.
  Curves include static gain, all four dynamics modes, soft knee and range
  limiting without advancing audio processing state.

## [v0.12.3] - 2026-10-07

### Added

- Effectchain multiband compressor supports 2–4 bands, a third crossover and
  independent Attack, Release, Knee, Makeup and Auto makeup per band when
  `perBand` is enabled. Shared settings remain the default for existing graphs
  and presets; the two-band high range keeps using the original `mid` parameters.
  `responseBand` selects the actual band gain computer for `Chain.Transfer`
  inspection. Workspace estimates reserve storage for all four bands.

## [v0.12.2] - 2026-10-07

### Added

- Effectchain compressor, gate and multiband compressor expose `topology`
  (`feedforward` / `feedback`) and apply it to the dynamics processors, including
  every multiband compressor band. Expander retains its existing topology control.
  Sparse graphs and factory presets default to feedforward. Regression tests
  cover catalogue availability, actual processing differences and restoring
  feedforward on runtime reconfiguration.

## [v0.12.1] - 2026-10-07

### Added

- Standard effectchain filters now accept and honour orders through 20 for
  Butterworth and Chebyshev I/II pass, shelf and peak designs. Bessel
  remains limited to its supported orders 1–10 and elliptic retains its cap of
  12 (order 20 fails numerical impulse/response agreement at the fixed bounds).
  RBJ and Moog behaviour is unchanged.
  Peak order still denotes the prototype order (twice that many digital poles).
  Regression tests verify actual cascade sizes, stable poles, finite response
  and processing, and agreement between impulse processing and response inspection.
- Single-band effectchain compressor exposes `autoMakeup` (default off), using
  the dynamics core's automatic makeup gain in audio processing and transfer
  inspection. Manual makeup remains stored and is restored when auto gain is
  turned off; sparse existing graphs retain manual-gain behaviour.

## [v0.12.0] - 2026-10-07

### Added

- `design.ParametricBand` and `band.ButterworthPeak` support actual digital band
  orders; parametric EQ exposes per-band orders 2, 4, 6, 8, 10 and 12. The default
  of two preserves the original RBJ coefficients and existing sparse presets.
  Higher orders sharpen Butterworth pass, shelf and peak transitions without
  multiplying the requested gain. Peak Q controls bandwidth; higher-order pass
  and shelf Q is fixed. Shelf midpoints and peak center gains remain anchored.
- `ButterworthPeak` handles odd prototype orders analytically, so total digital
  orders 2, 6 and 10 have the correct number of sections as well as 4, 8 and 12.
  Existing `ButterworthBand` prototype-order behavior remains unchanged.
- Regression coverage sweeps gain/frequency/sample-rate/order combinations,
  bounded gain, sharper slopes, exact legacy coefficients, impulse/response
  agreement, zero-allocation processing and effectchain inspection/processing.

## [v0.11.1] - 2026-10-07

### Added

- Parametric EQ supports highpass/lowpass band types in processing and response
  inspection. Its catalogue/default preset now has six bands: highpass at 30 Hz,
  low shelf at 100 Hz, peaks at 350 Hz and 1.2 kHz, high shelf at 4 kHz, and
  lowpass at 14 kHz. Low sample rates compress the logarithmic spacing. Pass
  bands use Butterworth Q; gain remains meaningful only for peaks and shelves.
- Sparse graphs retain their legacy four-peak runtime fallbacks; explicit saved
  graphs and the Vocal presence preset retain their original band types. Dynamic
  EQ defaults and supported types are unchanged.

## [v0.11.0] - 2026-10-07

### Added

- `design.GraphicEQ` designs complementary Butterworth shelf transitions at
  geometric band boundaries. Equal neighboring gains have no extra transition;
  equal gains across all bands are exactly flat. The first/last controls extend
  to DC/Nyquist, and order controls the transition steepness.

### Fixed

- `effectchain` graphic EQ no longer stacks independent boosted band filters:
  ten +12 dB controls previously produced nearly +24 dB overlap peaks. Actual
  processing and response inspection now share the complementary design.
  Upper bands that coincide after low-sample-rate clamping use their mean gain.
- Regression sweeps cover all advertised orders and sample rates, adjacent
  boost/cut plateaus, centered monotonic reciprocal transitions, actual impulse
  versus inspected response, zero-allocation processing and invalid geometry.

## [v0.10.2] - 2026-10-05

### Added

- `resample.NewStreamPlan` preflights exact integer-rate polyphase filters and
  conservative workspace budgets without allocating coefficients. Its mono
  streams share FIR coefficients across clones, remove integer output delay,
  flush finite tails to exact duration, and preserve continuous indefinite loops
  using bounded allocation-free processing. `FrameCount` and `FramePosition`
  centralize overflow-checked duration and coordinate conversion.
- `signal.AddInto32` and `signal.AverageInto32` mix float32 storage directly
  without float64 scratch or intermediate rounding between contributions.

## [v0.10.1] - 2026-10-04

### Added

- `loudness.BS1770ChannelWeights` centralizes five/six-channel surround/LFE
  weighting and preserves source channel identity for packed subsets.
- `restoration.NewNoiseCapture` owns bounded noise-profile STFT framing. Complete
  windows and reflection of short sources avoid the zero-padding power bias at
  capture boundaries; one-frame profiles are valid for reduction.

## [0.10.0] - 2026-10-04

### Added

- `dsp/effects/restoration`: averaged noise-profile capture, decision-directed
  Wiener filtering, spectral subtraction and profile gates, bounded inverse-STFT
  rectangle/polygon editing, autoregressive short-gap healing, automatic click
  removal, declipping and 50/60 Hz harmonic notch combs.
- `PitchShifter.TimeStretch` exposes its existing WSOLA stage without resampling;
  `pitch.NewStretchStream` adds bounded, cancellable processing with shared stereo
  alignment and exact rounded output duration.
- Native/race and actual WASM reference tests for selective spectral editing,
  click healing, >=15 dB stationary-noise reduction, residual-noise variation and
  tonal-spike proxies, wanted-signal preservation, hum rejection, interpolation,
  duration/pitch/stereo invariants and invalid-input atomicity.

### Notes

- Restoration profile/gate behaviour derives from the legacy Delphi FFT Effects
  examples. Capture corrects the legacy DC-bin typo; normalized STFT overlap-add
  replaces the old hand-built filter/time-domain crossfade. See
  `docs/restoration.md` for scope, limits and reproducible acceptance evidence.

## [v0.9.0] - 2026-10-04

### Added

- Bounded, allocation-free streaming loudness measurement: 400 ms momentary,
  3 s short-term, gated integrated loudness, loudness range, sample-positioned
  maxima and an inexpensive cached reading accessor. Legacy meter APIs remain
  unchanged. Programme history is reserved at construction.
- Four-times oversampled true-peak measurement with the published BS.1770 FIR,
  explicit tail flushing and independent per-channel peaks.
- Stateful mono float32/float64 STFT analysis and normalized overlap-add inverse
  streams with bounded EOF steps, all existing padding/window options and reset.
- Cooperative YIN analysis jobs preserve the existing detector's exact search,
  interpolation and result while bounding each processing step.
- Streaming time statistics, contiguous clipping runs, phase correlation and
  chronological mid/side points; rolling/accumulated power averaging, calibrated
  one-sided FFT dBFS and allocation-free fractional-octave smoothing.
- Opt-in conformance tests for all 66 applicable published EBU test sequences,
  including programme loudness/LRA, shifted maxima and true peak. See
  `measure/loudness/CONFORMANCE.md` for provenance and fixture instructions.

## [v0.8.4] - 2026-10-03

### Added

- `reverb.NewConvolutionReverbWithMaxBlockOrder` offers an explicit maximum
  partition hop for hosts balancing transform bursts against average CPU use.
  The original constructor retains its 8192-sample maximum partition.

### Changed

- Effect-chain convolution caps partition hops at 1024 samples to reduce
  worst render bursts. It retains the complete impulse response, 128-sample
  latency and allocation-free processing. The generic convolution and reverb
  defaults remain unchanged. Different FFT partitioning can change results
  at roundoff level within the existing float32/float64 tolerances.
- Effect-chain cold workspace estimates follow the actual partition cap.

## [v0.8.3] - 2026-10-03

### Changed

- Streaming WSOLA overlap searches reuse owned contiguous reference and
  candidate windows. Each input ring sample is copied once per search rather
  than checked and indexed for every correlation term. The full search radius,
  ascending candidate order and sequential correlation arithmetic are
  unchanged, preserving exact output, reset and partition behavior.
- `effectchain.EstimateWorkspace` includes both bounded search scratch buffers
  before constructing time-pitch runtimes. Processing and reset remain
  allocation-free.

## [v0.8.2] - 2026-10-03

### Added

- `effectchain.WithSourceChannelMap` preserves physical stereo IR sides when
  hosts pack selected source channels. Right-only selections use the right IR;
  nonadjacent source channels retain their original parity. Maps are owned,
  validated and must cover prepared channels; omitting the option preserves
  existing packed indexing.

### Changed

- Multi-partition convolution stages retain a frequency-domain input delay
  line and combine matching past input/IR spectra before one inverse FFT.
  The full IR, FFT sizes, partition layout and technical latency remain the
  same, while large stages no longer execute a separate inverse transform
  for every IR partition in one render quantum. Single-partition stages keep
  their existing path. Floating-point sums can differ at roundoff level.
- `effectchain.EstimateWorkspace` includes the retained frequency histories
  in its conservative cold allocation budget before graph construction.

## [v0.8.1] - 2026-10-03

### Added

- `effectchain.DefaultDescriptors` exposes all 51 registered effects with
  sample-rate-aware parameter bounds, explicit units/enumerations and owned
  factory presets. New factories cover parametric/graphic/dynamic EQ,
  A/C weighting, auto-wah, frequency shifting, panning, Haas and crosstalk.
  Default filter construction supports Butterworth, Bessel, Chebyshev I/II,
  elliptic and RBJ designs without application-supplied builders.
- `Chain.Prepare`, `PreparePlanar`, `ProcessPlanar` and `ResetProcessing`
  provide reusable mono/stereo graph rendering with no built-in processing
  allocations. Stereo effects preserve both channels. `Response` evaluates
  actual filter/EQ coefficients, including current dynamic EQ coefficients;
  `Transfer` evaluates actual static dynamics gain computers.
- Public streaming WSOLA/Hermite and phase-vocoder pitch processors and
  spectral freeze preserve their sample clocks across arbitrary partitions.
  `Chain.Latency` reports technical lookahead and aligns parallel graph paths;
  callers can compensate startup and flush selected-duration output.
  Checked processing propagates runtime failures rather than silently using
  partially processed audio.
- `EstimateWorkspace` bounds cold graph/runtime/preparation allocations,
  including maximum block geometry, convolution IR lengths and technical
  branch delays, before constructors allocate DSP storage.
- `Chain.TryUpdateGraph` stages zero-latency parameter changes atomically while
  preserving unchanged processors, including expensive convolution spectra
  and tails. Convolution wet-only updates retain all histories; topology, IR
  or buffered-latency changes explicitly request full replacement.

### Fixed

- Convolution graph nodes reject missing, nonfinite, unequal-length or
  sample-rate-mismatched IRs and preserve corresponding stereo IR channels.
  Their optional aligned dry path follows the actual engine latency.
- Delay reset restores the configured initial time, mono widener histories
  persist across blocks, and multiband compression reuses prepared crossover
  buffers. Independent numerical tests cover long pitch/freeze histories,
  every catalogue default/preset, partition/reset parity, true stereo routing
  and zero allocations after FFT/WSOLA startup.
- Butterworth graphic-EQ pole construction uses analytic quadratic factors,
  preserving stable low-frequency bands at high sample rates where generic
  quartic root finding lost precision.

- `measure/thd`: aggregate metrics (THD+N, Noise, SINAD, OddHD, EvenHD,
  RubNBuzz) are now computed by power summation (root-sum-square) instead of
  summing linear magnitudes. The old math inflated wideband terms by roughly
  sqrt(bin count): a pure sine with a -84 dB white noise floor reported
  SINAD ~30 dB low at realistic FFT sizes.
- `measure/thd`: THD is now the fundamental-referenced THD_F
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

## [v0.8.0] - 2026-10-03

### Added

- `dsp/stft`: short-time Fourier transform (`New`, `New32`) built on algo-fft's real
  plans. Centred framing with zero or reflect padding (`PadZero` reproduces frame `i`
  centred at `i·hop`, `PadReflect` matches `torch.stft(center=True)`) or unpadded
  framing, optional unitary scaling (`WithNormalized`), custom windows, a zero-alloc
  `FrameInto`, and an `Inverse` normalized by the window sum of squares, so non-COLA
  window/hop pairs reconstruct; it reports `ErrWindowSumZero` instead of dividing by
  zero. Reconstruction error is below 1e-12 for float64.
- `measure/music/features`, `onset`, `rhythm`, `align`: music-analysis features lifted
  from the AudioVisualizer pipeline (Phase 44 Workstream B): frame RMS/peak/centroid/
  width/flux and band envelopes, an energy-preserving log-frequency spectrogram with
  exported bin frequencies (fixes the empty low rows of the original mapping), envelope
  normalizer, silence finder, spectral-flux onsets with attack refinement and heuristic
  drum kinds, tempo estimation with an optional prior, beat phase/grid, downbeat, and a
  mix-versus-parts alignment check. With default settings each reproduces the original
  code bit for bit; parity tests embed the reference implementation.
- `measure/music/melody`: predominant pitch by harmonic-sum salience, voicing, 12-bin
  chroma and note segmentation with onset snapping (`Analyze`, `SegmentNotes`,
  `MedianVoiced`, `Downmix`).
- `dsp/separate`: HPSS (Fitzgerald 2010) with soft masks of power `p` or the margin
  variant and a residual, outputs summing to the input (≈ −310 dB); a zero-alloc
  sliding `MedianFilter`, N-source `SoftMasks`, `ApplyMask`, `MidSide`/`LeftRight` and a
  heuristic `CentreExtractor`.
- `resample.Resampler.ProcessAligned` and `resample.ResampleAligned`: whole-buffer
  resampling with the FIR group delay removed (output sample `j` corresponds to input
  time `j·down/up`). It runs on a clone, so the receiver's streaming state is untouched.
- `core.LinearToDBFloor` and `core.LinearPowerToDBFloor`: dB conversions with an explicit
  floor (1e-6 → −120 dB); NaN stays NaN.
- `rhythm.Grid` (`NewGrid`, `WithBeats`, `WithBeatsPerBar`, `WithSubdivisions`): an immutable
  16th/beat/bar grid with a pickup bar, built from `BeatGrid`/`Downbeat` output.
- `melody.Clean`: grid quantisation, monophonic per slot, octave-error correction and an
  arpeggio voice for tracked notes; `melody.BassPreset` and `BassCleanOptions` for bass lines.
- `measure/music/harmony`: Krumhansl–Kessler key estimation with a tonic-evidence tie-break
  (`EstimateKey`), RMS-weighted chroma windows (`Windows`), and template chord recognition with
  bass, inversion, seventh and in-key terms plus Viterbi smoothing (`Chords`, allocation-free
  `Chorder`).
- `measure/music/structure`: z-scored weighted feature `Blocks`, cosine `SelfSimilarity`,
  Gaussian-checkerboard `FooteNovelty`, `Peaks` with reference marks, and A/A′/B phrase `Label`;
  `examples/structure_overview` draws the SSM and novelty curve.
- `measure/music/motif`: transposition-invariant motif discovery on notes (`FindNoteMotifs`)
  and beat chroma (`FindChromaMotifs`), `Corroborate`, salience `Score` and leitmotif `Rank`.
- `align.Lag` and `align.LagChannels`: coarse-then-fine lag search with correlation, gain and a
  parabolic sub-sample estimate. `features.Activity`: per-span activity of named tracks
  relative to their own 95th-percentile level.

### Fixed

- `align.Check` no longer overflows or scans for a long time with a huge `WithMaxLag`.

## [v0.7.11] - 2026-10-03

### Added

- `dither.WithPCMQuantization` opts into conventional signed PCM scaling by
  2^(bitDepth-1), nearest rounding with halfway values away from zero, and
  normalized `ProcessSample` output without the legacy half-LSB offset.
  Direct integer PCM codes therefore avoid the legacy floor convention's
  half-LSB silent-dither bias. Defaults, noise distributions and RNG draws
  remain unchanged for existing callers.
  Limited PCM saturates finite source input before scaling and records only
  quantization error before output clipping, excluding saturation distortion
  from noise-shaper feedback so over-range input cannot cause windup.

### Fixed

- Limited quantization clips floating-point integer codes before conversion to
  `int`, preventing finite over-range samples or dither from overflowing and
  reversing sample polarity. Signed endpoints are constructed safely on
  32-bit platforms, including PCM32. Scalar-reference tests cover every dither
  distribution, full scale, noise-shaping histories, extreme input, partitions,
  unbiased silent TPDF statistics and zero processing allocations.

## [v0.7.10] - 2026-10-03

### Added

- `TargetAnalyzer.ProcessCertifiedPlanar32` accepts a caller-proven finite,
  exact float32 sample peak to omit repeated sample classification. It still
  processes every actual sample through the same K-weighting filters, windows
  and loudness gates; the certificate does not predict loudness or replace an
  independent stored-output measurement. Ordinary `ProcessPlanar32` remains
  fully validating, and state/shape/frame/certificate validation stays atomic.
- Independent scalar-reference bit-parity tests cover fused and custom-weight
  layouts, mixed precisions, fractional rates, extreme/subnormal samples,
  arbitrary partitions and input ownership. Invalid certificate and state
  tests verify rejection before mutation; certified ten-minute stereo
  benchmarks include actual analysis/finalization and allocate nothing.

## [v0.7.9] - 2026-10-03

### Added

- `fade.EnvelopeInto64`, `ApplyEnvelopeInto32` and `CrossfadeEnvelopeInto32`
  share unrounded float64 envelope gains across linked channels. They preserve
  the original float32 output rounding while avoiding repeated curve evaluation;
  callers supply bounded reusable scratch and processing allocates nothing.

### Changed

- Float32 fades hoist shape dispatch, logarithmic constants and whole-block
  position conversion out of sample loops. Positions through 2^53 use exact
  float64 integer addition; larger positions retain the original formula.
- Continuous generators dispatch once per block and avoid floating-point
  positions for noise/silence. SplitMix64 uniform conversion splits its 53 bits
  into exact 32-bit binary fractions, retaining the original random sequence.
- `MeanAccumulator.AddFloat32` classifies nonfinite float32 encodings directly,
  retaining compensated addition order and atomic invalid-input rejection.
- Rational resampling caches phase advances, keeps streaming counters in locals,
  and splits the current-block interior from startup/history handling. Unrolled
  interior loops preserve filter coefficients, profile taps and sequential
  summation order without additional coefficient storage or processing allocations.

### Validation

- Independent original-formula bit-parity tests cover fade directions/shapes,
  generator streams and large-position boundaries, fixed seeded random vectors,
  and resampling cancellation/exponent stress across ratios and custom taps.
- A ten-minute stereo 48 kHz to 44.1 kHz benchmark uses the editor's unchanged
  Fast/Balanced/Best profile tap scaling. Component benchmarks exclude candidate
  storage, UI and editor acceptance timing; no full-editor timing pass is claimed.

## [v0.7.8] - 2026-10-03

### Added

- `dsp/fade` offers allocation-free float32 fades and crossfades with linear,
  equal-power, logarithmic and smooth S-curve envelopes. Explicit whole-fade
  positions preserve endpoints and sample parity across arbitrary block sizes.
- `dsp/signal.MeanAccumulator` measures a compensated full-range float32 mean
  across blocks. `SubtractMeanInto32` applies that measured value without
  recomputing block-local means; `ScaleInto32` multiplies directly from float32
  storage using float64 gain and one final float32 rounding.
- `dsp/signal.StreamGenerator` produces continuous float32 silence, sine,
  seeded white/pink noise and linear/logarithmic sweeps into caller-owned
  blocks. It retains global sample position and deterministic SplitMix64/pink
  state; block overruns leave output and state unchanged. Existing one-shot
  generators and their random sequences remain unchanged.

### Validation

- Independent analytic and fixed random golden vectors, exact block partition
  parity, aliasing, endpoint and invalid-argument tests; successful processing
  paths are checked for zero allocations and benchmarked with `-benchmem`.
- Extreme logarithmic sweeps spanning subnormal frequencies stay finite.

## [v0.7.7] - 2026-10-03

### Changed

- `measure/loudness.TargetAnalyzer` float32 preflight classifies nonfinite
  encodings and tracks the signless sample peak with integer comparisons,
  converting the block peak once. Shape, frame-limit and finite-input rejection
  remain atomic, including zero-weight channels and mixed float32/float64 calls.
- Hop aggregation checks finite sequential sums at complete or partial segment
  boundaries instead of every frame, retaining the same addition order,
  nearest-sample endpoints and terminal overflow/reset behavior.
- Unit-weight mono/stereo scans fuse both K-weighting stages with hop
  accumulation, eliminating energy-scratch initialization and extra passes.
  Larger/custom-weight layouts retain the generic prepared path. Successful
  filter states, window energies, target plans and actual measurements are
  checked bit-for-bit against that independent retained path across rates,
  partitions and extreme finite inputs; streaming remains allocation-free.
- Published `IntegratedAnalyzer`, `Meter`, normalization plans and the
  requirement to actually verify stored float32 loudness output are unchanged.
  These source optimizations do not claim a passed editor browser timing gate.

### Validation

- Full native CI/race tests, native/WASM vet, all 12 browser demo checks, and
  actual Node/V8 WASM loudness tests pass; loudness coverage is 96.4%.
- Serial three-iteration ten-minute 48 kHz stereo benchmarks measure target
  analysis / fresh-candidate measurement at 176.855 / 172.652 ms natively and
  264.457 / 254.440 ms under Node/V8 WASM, with 0 B/op and 0 allocs/op.
  Reset and bounded finalization are included; constructor/fixture setup,
  scaling, storage, editor UI and commit are excluded. These host-sensitive
  component timings are not full-editor acceptance results.

## [v0.7.6] - 2026-10-03

### Added

- `measure/loudness.TargetAnalyzer` plans a linked gain with both loudness gates
  recalculated after scaling. It retains positive complete-window energies,
  including originally below-gate material, and incrementally enumerates
  consistent absolute-gate intervals instead of assuming gated LUFS is monotonic.
  `TargetResult` distinguishes a finite source measurement from undefined
  below-gate LUFS, and predicts the target to 0.01 LU in the energy domain.
- Float32 quantization can change strict gate membership, so the plan explicitly
  requires stored-output verification. `FinishMeasurementStep` / `MeasurementResult`
  provide bounded measurement-only finalization of a fresh candidate scan using
  the same fast analyzer. Measurement and target finalization are mutually
  exclusive until `Reset`; `SamplePeak` exposes finite progressive input telemetry.
- Channel-major K-weighting and four positive hop sums avoid rescanning the full
  400 ms energy window. Preflight rejection remains atomic, workspace remains
  capped at 64 MiB, and successful streaming / finalization allocate nothing.
  Existing `IntegratedAnalyzer`, `Meter` and input-derived normalization APIs
  retain their implementations and behavior.
- `dsp/signal.PlanPeakNormalization` derives one linked gain for a finite sample
  peak and dBFS target without an application UI clamp. Zero peak is an identity;
  subnormal sources are supported and unrepresentable gain/peak values rejected.
- Runnable examples, independent direct-form-I / gating and static goldens,
  rate/partition/weight/float32 parity, exhaustive small gate-interval oracles,
  ownership/state/reset/overflow/allocation tests and ten-minute WASM benchmarks
  accompany the new APIs. Predictions and sample peaks do not claim true-peak
  support or peak limiting.

### Validation

- Native CI, full race tests, native/WASM vet, all 12 browser demo checks, and
  actual Node/V8 WASM loudness/signal tests pass. Loudness package coverage is
  96.1%; the new peak planner has 100% statement coverage.
- Ten-minute 48 kHz stereo target analysis measures 0.49–0.52 s under Node/V8
  WASM, and fresh-candidate measurement 0.49 s, with 0 B/op and 0 allocs/op.
  These include reset and bounded finalization but exclude constructor/fixture
  setup, scaling, output storage, UI and commit. They are not editor browser
  acceptance results; the full workflow must be timed separately.

## [v0.7.5] - 2026-10-03

### Added

- `measure/loudness.IntegratedAnalyzer` provides bounded planar float64/float32
  integrated loudness analysis. It uses the published BS.1770-5 48 kHz
  K-weighting coefficients (inverse-bilinear mapped at other supported rates),
  complete 400 ms windows at nominal 100 ms intervals, and absolute -70 LUFS
  / relative -10 LU gates. Sample-rounded accumulated timing avoids cadence
  drift at rates such as 8005 and 11025 Hz. Channel power weights are explicit
  and copied; nil defaults to unit weights, never an inferred surround layout.
- Constructor validation bounds workspace to 64 MiB and caller blocks to 65536
  frames before allocation or state changes. Successful processing and bounded
  `FinishStep` finalization allocate nothing; `Reset` reuses reserved storage.
  Short/below-gate input produces errors, nonfinite samples are atomically
  rejected, and finite arithmetic overflow is terminal until reset. Combined
  positive-energy window rebasing avoids false loud residuals after huge transients.
- `PlanNormalization` and `NormalizeLoudness` apply one linked, input-derived
  gain, using tagged `algo-vecmath` for fresh-output scaling. They do not mutate
  input, limit peaks or clamp gain to a UI range. Gain/peak overflow and total
  underflow are rejected. Post-gain LUFS is not unconditionally guaranteed:
  absolute-gate membership may change, so callers requiring that guarantee
  must remeasure. Sample peaks include zero-weight channels and are not true peaks.
- Runnable examples, streamed mathematical EBU Tech 3341 integrated cases 1–6,
  an independent direct-form-I filter/gating oracle and static golden, rate and
  partition parity, EOF timing, ownership, overflow/reset and allocation tests,
  and bounded ten-minute stereo analysis benchmarks accompany these APIs.

### Documentation

- The existing `Meter` retains its legacy approximate filters and startup
  integration for compatibility. Corrected prior roadmap claims: its `Peaks`
  reports sample peaks, and its tests do not establish full EBU conformance.
  Standards-compliant live metering, LRA and oversampled true peak remain pending.

### Validation

- Full native CI and race suite pass, including all 12 browser demo checks,
  lint and native/WASM vet. The integrated/normalization tests also pass under
  Node/V8 WASM; loudness package statement coverage is 95.8%.
- Ten-minute 48 kHz stereo analysis, including reset and bounded finalization
  but excluding constructor/fixture setup, measures 0.45–0.56 s natively and
  1.25–1.30 s under Node/V8 WASM on an i7-1255U. Both precisions and the block /
  finalization microbenchmarks report 0 B/op and 0 allocs/op. These are analyzer
  timings, not an editor end-to-end performance or browser acceptance claim.

## [v0.7.4] - 2026-10-03

### Added

- `stats/time.NearestZeroCrossing` locates the nearest exact zero or finite
  strict-sign crossing in an inclusive bounded sample window for float32 or
  float64 signals, with earlier-index tie breaking. EOF targets are supported;
  empty/invalid inputs and absent candidates return `found=false`. NaN and
  infinity gaps are not bridged. The helper does not allocate or mutate samples,
  and radius clipping remains safe at 32-bit and 64-bit integer limits.
- Regression tests cover both precisions, signed zeros, subnormal and extreme
  samples, window boundaries, EOF, nonfinite values, exhaustive window parity,
  named sample types, mutation/NaN-payload preservation, and zero allocations.
  Runnable examples and zero-allocation benchmarks accompany the API.

### Validation

- Full native tests and race tests, lint, native/WASM vet, and actual Node/V8
  WASM statistics tests pass. The new helper has 100% statement coverage; the
  package has 98.9%. Float32/float64 benchmarks report 0 B/op and 0 allocs/op.

## [v0.7.3] - 2026-10-03

### Added

- `resample.Resampler.ProcessInto` converts complete mono blocks into a caller's
  destination without allocations. Short destinations and unrepresentable
  output lengths are rejected atomically with `ErrShortDst` / `ErrOutputTooLarge`.
- `GroupDelayInput` and `GroupDelayOutput` report fractional FIR delay in their
  respective sample-frame units. Callers explicitly feed zeros to flush tails.
- `Resampler.Clone` creates a reset stream sharing immutable coefficients and
  owning independent history, avoiding repeated filter design for each channel.

### Changed

- `PredictOutputLen` computes the exact next-block output length in constant
  time, without overflowing at 32-bit input limits. Lengths that cannot fit in
  `int` saturate at the platform maximum and are rejected by `ProcessInto`.
- Streaming positions are relative to the current block, so WASM streams no
  longer overflow after 2^31 input frames. `Process` retains the previous FIR
  coefficients and arithmetic, and now allocates only its returned output.
- Regression tests cover bitwise legacy/chunk parity across ten ratios and all
  quality modes, atomic rejection, independent clones, group delay/tail flushing,
  zero allocations, and the WASM stream-counter boundary. Extreme 384 kHz to
  8 kHz quality tests validate explicitly scaled taps per phase; default filter
  design remains unchanged for compatibility.

### Validation

- Full native tests, race tests, lint, and native/WASM vet pass; the resampler
  suite also runs under Node/V8 with `GOOS=js GOARCH=wasm`. Package coverage is
  94%, and `ProcessInto` benchmarks report 0 B/op and 0 allocs/op for all quality
  modes. At 384 kHz to 8 kHz, scaling taps per phase by 48 measures 74/89/106 dB
  rejection at 6 kHz for Fast/Balanced/Best; regression floors are 55/75/90 dB.

## [v0.7.2] - 2026-10-02

### Added

- `stats/time.Summary` computes minimum, maximum, and float64 energy directly
  from float32 or float64 signals in one allocation-free pass. It avoids the
  higher-order moments and decibel conversions in `Calculate`, and needs no
  float32-to-float64 scratch buffer. Extrema and energy follow `Calculate`'s
  sample order and floating-point semantics, including signed zero, NaN, and
  infinity. Empty input returns zero-valued `SummaryStats`. Runnable examples,
  numerical parity tests, allocation checks, and benchmarks cover both widths.

### Changed

- Phase 41c, first half: six hand-rolled loops now call the `algo-vecmath` kernels that were already available and simply unused. `AddBlockInPlace` takes the partitioned-convolution overlap-add (`dsp/conv/partitioned.go`), the streaming overlap-add tail merge (`dsp/conv/streaming_overlap_add.go`), the multiband band sum (`dsp/effects/dynamics/multiband.go`) and the parent-edge mix (`dsp/effectchain/chain_process.go`); `ScaleBlock` takes that mix's output scaling; and `ScaleBlockInPlace` takes the log-sweep inverse-filter normalization (`measure/sweep/sweep.go`). The two zeroing loops in the mix became `clear`.

  **Every one of these is bit-identical on amd64 and arm64, by construction.** They are element-wise adds and element-wise scalar multiplies -- there is no multiply-add to fuse and no reassociation, so the hazard behind the `d2ec9ef` AXPY change (bit-identical on amd64, an ulp apart on arm64) cannot recur here.

  Measured on amd64/AVX2 (Ryzen 5 4600H), before vs after on the same benchmarks: `BenchmarkPartitionedConvolution`, `BenchmarkMultibandProcessInPlace` and `BenchmarkLogSweepInverseFilter` show **no significant change** (p >= 0.33) -- in each the substituted loop is a small fraction of what the benchmark measures, so the gain is real but below the noise floor of the enclosing function. Allocation counts are unchanged everywhere, and the touched paths are pinned allocation-free by new `testing.AllocsPerRun` assertions.

- `spectrum.GoertzelBank.ProcessBlock` advances four bins per pass over the sample buffer instead of one. The recurrence is serial within a bin but the bins are independent, so a group of four reads the buffer once rather than four times and gives the processor four independent dependency chains to overlap -- the single-bin loop is latency-bound on the multiply-add, not throughput-bound. Bins past the last full group of four fall through to the existing per-bin path. **3.8x for four bins** (13.6 -> 3.5 us over a 1024-sample block) and **4.2x for the eight-bin DTMF case** (27.1 -> 6.4 us); a bank of one, two or three bins is unchanged. Output is **bit-identical per bin** -- `TestGoertzelBankProcessBlockBitExact` compares with `==`, not a tolerance, across bin counts 1-9 and block lengths 0-1024. The recurrence itself was factored into one `goertzelStep` helper shared by the single-bin, bank and four-wide paths so no two of them can diverge in how the compiler contracts the multiply-add.

- The vecmath call sites above are guarded by benchmarked length thresholds (`mixSIMDThreshold`, `addBlockSIMDThreshold`, `bandSumSIMDThreshold`, all 64), in the same style as the existing `conv.simdThreshold`. This is not caution: a dispatched vecmath call carries roughly 110 ns of fixed cost on this machine regardless of length, so the four-parent mix loses 0.72x at a 16-sample block before turning over to 3.0x at 512. Short buffers are not hypothetical here -- a partitioned convolver built with a low `minBlockOrder` has a `partSize` of a handful of samples.

- `vecmath.MaxAbs` was **not** adopted, in `stats/time.Peak` or in `measure/ir`'s impulse-onset search, although it is 3.0x and 5.6x faster there respectively. It is unsafe for both: on AVX2 a NaN anywhere in the slice can **discard the true maximum and return a smaller finite value**. `MaxAbs([999, NaN, 0.5])` returns `0.5` on AVX2 and `999` on the pure-Go kernel. That is not a difference in NaN policy -- it is a silently wrong _finite_ result, on some CPUs only, and no cheap check detects it, because an `IsNaN` test on the result does not fire. In `findImpulseStart` a peak under-reported by that factor leaves the threshold orders of magnitude too low, so the quiet run before the impulse clears it and the reported onset is far too early, corrupting every metric derived from it. Detecting NaN up front costs a full extra pass, which is the entire speedup, so both sites keep their scalar loops and `TestPeakNaNIsDeterministic` / `TestFindImpulseStartNaNIsDeterministic` fail on an AVX2 machine if anyone re-adopts `MaxAbs`. Raised by review on #25. This looks like an `algo-vecmath` defect rather than a documentation gap and is worth fixing there.

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
