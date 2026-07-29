# Effects Catalog

This document lists the audio effects currently implemented in `algo-dsp` and
candidates for future addition.

## Implemented Effects (`dsp/effects/`)

All effects listed below are production-ready with tests, examples, and
zero-allocation hot paths suitable for real-time use. Every effect supports
both single-sample (`Process`) and buffer-based (`ProcessInPlace`) processing.

### Dynamics

| Effect               | File                   | Description                                                                                                                                                                                       |
| -------------------- | ---------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Compressor**       | `compressor.go`        | Soft-knee feed-forward compressor with log2-domain gain calculation. Configurable threshold, ratio, knee width, attack/release, and auto-makeup gain.                                             |
| **Gate**             | `gate.go`              | Soft-knee noise gate with hold time to prevent chattering. Configurable threshold, expansion ratio, knee width, attack/hold/release, and range.                                                   |
| **Limiter**          | `limiter.go`           | Peak limiter (compressor preset with 100:1 ratio, 0.1 ms attack, hard knee).                                                                                                                      |
| **LookaheadLimiter** | `lookahead_limiter.go` | Limiter with delayed program path and optional sidechain detector input for true lookahead peak control.                                                                                          |
| **TransientShaper**  | `transient_shaper.go`  | Attack/release envelope-split transient shaper with independent attack and sustain emphasis/attenuation controls.                                                                                 |
| **DynamicEQ**        | `dynamic_eq.go`        | Parametric EQ bands whose gain follows a per-band detector. Peaking/shelving bands, downward/upward/upward-below dynamics, band-filtered or external sidechain, control-rate coefficient updates. |

### Time-Based

| Effect         | File            | Description                                                                                                                            |
| -------------- | --------------- | -------------------------------------------------------------------------------------------------------------------------------------- |
| **Delay**      | `delay.go`      | Feedback delay line with configurable delay time (1-2000 ms), feedback, and dry/wet mix. Circular buffer implementation.               |
| **Reverb**     | `reverb.go`     | Schroeder/Freeverb-style algorithmic reverb. 8 comb filters + 4 allpass filters with room size and damping controls.                   |
| **FDN Reverb** | `fdn_reverb.go` | Feedback delay network reverb. 8 delay lines mixed via Hadamard matrix, with RT60-based decay, pre-delay, damping, and LFO modulation. |

### Modulation

| Effect             | File                | Description                                                                                                                                    |
| ------------------ | ------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------- |
| **AutoWah**        | `auto_wah.go`       | Envelope-following band-pass modulation. Configurable center-frequency range, Q, attack/release detector times, sensitivity, and dry/wet mix.  |
| **Chorus**         | `chorus.go`         | Multi-voice modulated delay for ensemble/thickening effects. Hermite interpolation, configurable speed, depth, base delay, and voice count.    |
| **Flanger**        | `flanger.go`        | Short modulated delay (0.1-10 ms) with feedback for classic jet/sweep sound. Configurable speed, depth, feedback, and base delay.              |
| **Phaser**         | `phaser.go`         | Cascaded allpass filters with LFO-modulated center frequencies. Configurable stages, speed, depth, feedback, and mix.                          |
| **Ring Modulator** | `ring_modulator.go` | Multiplication of input with a sine-wave carrier oscillator, producing sum and difference frequencies. Configurable carrier frequency and mix. |
| **Tremolo**        | `tremolo.go`        | LFO amplitude modulation with optional smoothing. Configurable rate, depth, smoothing time, and mix.                                           |

### Pitch

| Effect                   | File                      | Description                                                                                                                                   |
| ------------------------ | ------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------- |
| **PitchShifter**         | `pitch_shifter.go`        | Time-domain WSOLA-style pitch shifter. Ratio range 0.25-4.0 with configurable sequence length, overlap, and search window.                    |
| **SpectralPitchShifter** | `pitch_shift_spectral.go` | Frequency-domain phase-vocoder pitch shifter. STFT time-stretch + resample with configurable frame size and hop.                              |
| **YINDetector**          | `yin_detector.go`         | YIN fundamental frequency estimation for one frame: difference function, cumulative mean normalization, parabolic interpolation. Zero-alloc.  |
| **PitchTracker**         | `pitch_tracker.go`        | Streaming wrapper around the detector: ring buffer, hop scheduling, median smoothing and an unvoiced hold. Zero-alloc.                        |
| **PitchCorrector**       | `pitch_corrector.go`      | Auto-tune style correction: tracker plus shifter, snapping to a scale or a fixed target with amount, clamp, confidence gate and retune glide. |

Both pitch shifters implement the `PitchProcessor` interface
(`pitch_processor.go`) for interchangeable use. `PitchCorrector` deliberately
does not: its ratio is derived from detection rather than set by the caller.

Musical helpers live in `note.go`: `Scale` and `PitchClass` with the usual
scale constructors, plus `FrequencyToMIDI`, `MIDIToFrequency`,
`SemitonesToRatio`, `RatioToSemitones` and `CentsBetween`.

Possible refinements, neither required for correction to work: an
FFT-accelerated difference function for the detector's hot loop, and pYIN's
probabilistic candidate tracking in place of the median filter.

### Lo-fi

| Effect         | File             | Description                                                                                                                                            |
| -------------- | ---------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------ |
| **BitCrusher** | `bit_crusher.go` | Sample rate and bit-depth reduction for retro/lo-fi aesthetics. Configurable bit depth (1-32, fractional), downsample factor (1-256), and dry/wet mix. |

### Spectral / Psychoacoustic

| Effect           | File               | Description                                                                                                                                               |
| ---------------- | ------------------ | --------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **HarmonicBass** | `harmonic_bass.go` | Psychoacoustic bass enhancer. Crossover filtering with non-linear harmonic generation, configurable frequency/ratio/response/decay, and built-in limiter. |

### Spatial

| Effect            | File                        | Description                                                                                                                                                               |
| ----------------- | --------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **StereoWidener** | `spatial/stereo_widener.go` | Mid/side stereo image widener. Configurable width (0=mono to 4=extra-wide), optional bass mono crossover to keep low frequencies centered when widening.                  |
| **StereoPanner**  | `spatial/stereo_panner.go`  | Pan law placement of a mono source plus attenuate-only stereo balance. Selectable equal-power (-3 dB), compromise (-4.5 dB) or linear (-6 dB) law, optional auto-pan LFO. |

---

## Candidate Effects for Future Addition

The effects below would complement the existing set. They are grouped roughly
by priority and implementation complexity.

### High Priority

These fill common gaps in any effects toolkit and can largely be built from
primitives already in the library (biquad filters, delay lines, LFOs).

| Effect                   | Category | Description                                                              | Building Blocks                                      |
| ------------------------ | -------- | ------------------------------------------------------------------------ | ---------------------------------------------------- |
| **De-esser**             | Dynamics | Frequency-selective compressor targeting sibilance (typically 4-10 kHz). | Biquad bandpass for detection, compressor sidechain  |
| **Expander**             | Dynamics | Downward expander (complement to gate with gentler ratios).              | Gate with low ratio, or compressor with ratio < 1    |
| **Multiband Compressor** | Dynamics | Independent compression per frequency band using crossover filters.      | Crossover (already implemented), compressor per band |

### Medium Priority

Useful effects that require somewhat more involved implementations or
additional DSP building blocks.

| Effect                      | Category    | Description                                                                               | Notes                                            |
| --------------------------- | ----------- | ----------------------------------------------------------------------------------------- | ------------------------------------------------ |
| **Distortion / Saturation** | Waveshaping | Tube, tape, or transistor-style non-linear waveshaping with configurable transfer curves. | Oversampling recommended to control aliasing     |
| **Convolution Reverb**      | Spatial     | IR-based reverb using partitioned convolution.                                            | `conv` package (overlap-add already implemented) |

### Lower Priority / Specialized

Effects for specific use cases or those requiring more complex algorithms.

| Effect              | Category    | Description                                                                              | Notes                                                   |
| ------------------- | ----------- | ---------------------------------------------------------------------------------------- | ------------------------------------------------------- |
| **Vocoder**         | Spectral    | Analysis/synthesis vocoder using filter bank envelope following.                         | Filter bank (already implemented), envelope followers   |
| **Spectral Freeze** | Spectral    | Captures and sustains a single STFT frame indefinitely.                                  | STFT infrastructure (reuse from spectral pitch shifter) |
| **Granular Delay**  | Granular    | Delay with grain-based playback for texture and time-stretching.                         | Grain scheduler, window/envelope per grain              |
| **Noise Reduction** | Restoration | Spectral subtraction or Wiener-filter-based noise reduction with noise profile learning. | STFT, noise estimation, spectral gating                 |
| **Haas Delay**      | Spatial     | Short inter-channel delay (1-30 ms) for precedence-effect stereo widening.               | Per-channel delay line                                  |

---

## Design Guidelines for New Effects

New effects should follow the conventions established by existing
implementations:

1. **Functional options** via `With*` option functions and a `NewXxx(sampleRate float64, opts ...Option)` constructor.
2. **`Process(sample float64) float64`** for single-sample streaming.
3. **`ProcessInPlace(buf []float64) error`** for zero-allocation buffer processing.
4. **`Reset()`** to clear internal state without reallocating.
5. **Metrics** where meaningful (e.g., gain reduction for dynamics processors).
6. **Table-driven tests** with golden vectors and tolerance checks.
7. **Runnable examples** in `example_test.go`.
8. **Benchmarks** for hot paths tracking allocations/op.
