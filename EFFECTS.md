# Effects Catalog

This document lists the audio effects currently implemented in `algo-dsp` and
candidates for future addition.

## Implemented Effects (`dsp/effects/`)

All effects listed below are production-ready with tests, examples, and
zero-allocation hot paths suitable for real-time use. Every effect supports
both single-sample (`Process`) and buffer-based (`ProcessInPlace`) processing.

### Dynamics

| Effect         | File            | Description                                                                                                                                           |
| -------------- | --------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Compressor** | `compressor.go` | Soft-knee feed-forward compressor with log2-domain gain calculation. Configurable threshold, ratio, knee width, attack/release, and auto-makeup gain. |
| **Gate**       | `gate.go`       | Soft-knee noise gate with hold time to prevent chattering. Configurable threshold, expansion ratio, knee width, attack/hold/release, and range.       |
| **Limiter**    | `limiter.go`    | Peak limiter (compressor preset with 100:1 ratio, 0.1 ms attack, hard knee).                                                                          |

### Time-Based

| Effect         | File            | Description                                                                                                                                 |
| -------------- | --------------- | ------------------------------------------------------------------------------------------------------------------------------------------- |
| **Delay**      | `delay.go`      | Feedback delay line with configurable delay time (1-2000 ms), feedback, and dry/wet mix. Circular buffer implementation.                    |
| **Chorus**     | `chorus.go`     | Multi-voice modulated delay for ensemble/thickening effects. Hermite interpolation, configurable speed, depth, base delay, and voice count. |
| **Reverb**     | `reverb.go`     | Schroeder/Freeverb-style algorithmic reverb. 8 comb filters + 4 allpass filters with room size and damping controls.                        |
| **FDN Reverb** | `fdn_reverb.go` | Feedback delay network reverb. 8 delay lines mixed via Hadamard matrix, with RT60-based decay, pre-delay, damping, and LFO modulation.      |

### Pitch

| Effect                   | File                      | Description                                                                                                                |
| ------------------------ | ------------------------- | -------------------------------------------------------------------------------------------------------------------------- |
| **PitchShifter**         | `pitch_shifter.go`        | Time-domain WSOLA-style pitch shifter. Ratio range 0.25-4.0 with configurable sequence length, overlap, and search window. |
| **SpectralPitchShifter** | `pitch_shift_spectral.go` | Frequency-domain phase-vocoder pitch shifter. STFT time-stretch + resample with configurable frame size and hop.           |

Both pitch shifters implement the `PitchProcessor` interface
(`pitch_processor.go`) for interchangeable use.

### Spectral / Psychoacoustic

| Effect           | File               | Description                                                                                                                                               |
| ---------------- | ------------------ | --------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **HarmonicBass** | `harmonic_bass.go` | Psychoacoustic bass enhancer. Crossover filtering with non-linear harmonic generation, configurable frequency/ratio/response/decay, and built-in limiter. |

---

## Candidate Effects for Future Addition

The effects below would complement the existing set. They are grouped roughly
by priority and implementation complexity.

### High Priority

These fill common gaps in any effects toolkit and can largely be built from
primitives already in the library (biquad filters, delay lines, LFOs).

| Effect                   | Category   | Description                                                               | Building Blocks                                      |
| ------------------------ | ---------- | ------------------------------------------------------------------------- | ---------------------------------------------------- |
| **Flanger**              | Modulation | Short modulated delay (0.1-10 ms) with feedback. Classic jet/sweep sound. | Delay line, LFO (reuse chorus infrastructure)        |
| **Phaser**               | Modulation | Cascaded allpass filters with LFO-modulated center frequencies.           | Biquad allpass sections, LFO                         |
| **Tremolo**              | Modulation | Amplitude modulation via LFO (sine, triangle, square).                    | LFO, gain multiplication                             |
| **De-esser**             | Dynamics   | Frequency-selective compressor targeting sibilance (typically 4-10 kHz).  | Biquad bandpass for detection, compressor sidechain  |
| **Expander**             | Dynamics   | Downward expander (complement to gate with gentler ratios).               | Gate with low ratio, or compressor with ratio < 1    |
| **Multiband Compressor** | Dynamics   | Independent compression per frequency band using crossover filters.       | Crossover (already implemented), compressor per band |
| **Stereo Widener**       | Spatial    | Mid/side processing to widen or narrow the stereo image.                  | M/S encode/decode, per-channel gain                  |

### Medium Priority

Useful effects that require somewhat more involved implementations or
additional DSP building blocks.

| Effect                         | Category    | Description                                                                               | Notes                                            |
| ------------------------------ | ----------- | ----------------------------------------------------------------------------------------- | ------------------------------------------------ |
| **Distortion / Saturation**    | Waveshaping | Tube, tape, or transistor-style non-linear waveshaping with configurable transfer curves. | Oversampling recommended to control aliasing     |
| **Bit Crusher**                | Lo-fi       | Sample rate and bit-depth reduction for retro/lo-fi aesthetics.                           | Quantization + sample-and-hold                   |
| **Auto-Wah / Envelope Filter** | Modulation  | Bandpass filter with cutoff controlled by input envelope.                                 | Envelope follower + biquad bandpass              |
| **Ring Modulator**             | Modulation  | Multiplication of input with a carrier oscillator.                                        | Oscillator, simple multiply                      |
| **Convolution Reverb**         | Spatial     | IR-based reverb using partitioned convolution.                                            | `conv` package (overlap-add already implemented) |
| **Transient Shaper**           | Dynamics    | Independent control of attack and sustain portions of transients.                         | Envelope follower with dual time constants       |
| **Lookahead Limiter**          | Dynamics    | True-peak limiter with lookahead buffer for inter-sample peak detection.                  | Oversampled peak detection, delay compensation   |

### Lower Priority / Specialized

Effects for specific use cases or those requiring more complex algorithms.

| Effect               | Category    | Description                                                                              | Notes                                                   |
| -------------------- | ----------- | ---------------------------------------------------------------------------------------- | ------------------------------------------------------- |
| **Vocoder**          | Spectral    | Analysis/synthesis vocoder using filter bank envelope following.                         | Filter bank (already implemented), envelope followers   |
| **Spectral Freeze**  | Spectral    | Captures and sustains a single STFT frame indefinitely.                                  | STFT infrastructure (reuse from spectral pitch shifter) |
| **Granular Delay**   | Granular    | Delay with grain-based playback for texture and time-stretching.                         | Grain scheduler, window/envelope per grain              |
| **Pitch Correction** | Pitch       | Auto-tune style chromatic or scale-constrained pitch correction.                         | Pitch detection (e.g. YIN/pYIN) + pitch shifter         |
| **Noise Reduction**  | Restoration | Spectral subtraction or Wiener-filter-based noise reduction with noise profile learning. | STFT, noise estimation, spectral gating                 |
| **Dynamic EQ**       | Dynamics    | Parametric EQ bands with gain driven by signal level (sidechain-aware).                  | Biquad design + envelope follower per band              |
| **Stereo Panner**    | Spatial     | Constant-power or linear panning with LFO-driven auto-pan.                               | Trig or table-based pan law, LFO                        |
| **Haas Delay**       | Spatial     | Short inter-channel delay (1-30 ms) for precedence-effect stereo widening.               | Per-channel delay line                                  |

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
