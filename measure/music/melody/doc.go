// Package melody estimates the predominant melodic line of a music signal:
// a frame-wise pitch track with voicing, a 12-bin chroma (pitch-class)
// profile, and a segmented note list.
//
// # Algorithm
//
// [Analyze] works on a mono signal ([Downmix] averages channels). Frame i is
// centred on sample i*hop (default 240 samples, 10 ms at 24 kHz) and the
// frame count is ceil(n/hop). For each frame:
//
//  1. RMS gate: the RMS over centre ± hop must reach the gate (default
//     −50 dBFS). The FFT window is long and smears note edges; the short gate
//     keeps note boundaries sharp. Gated frames are unvoiced with zero chroma.
//  2. Spectrum: periodic Hann window of the FFT size (default 4096), centred
//     framing with zero padding (dsp/stft), magnitude per bin.
//  3. Chroma: the power of each bin inside the analysis band (default
//     100 Hz–5 kHz) is added to the pitch class of its nearest equal-tempered
//     note; the 12 values are normalised to the frame's strongest class.
//     Frames with a band power below 1e-12 are silent.
//  4. Salience: every candidate on a 0.1-semitone grid over the MIDI range
//     (default 52–96, E3–C7) scores the weighted sum of the linearly
//     interpolated magnitudes at its first harmonics (default 8 harmonics
//     weighted 0.8^(h−1), only those below the band's upper edge). The best
//     candidate is the frame's pitch.
//  5. Voicing: the power within ±2 bins of the chosen candidate's harmonics
//     (each bin counted once) divided by the total band power. The frame is
//     voiced if this share reaches the threshold (default 0.3); otherwise its
//     pitch and voicing are 0.
//
// The voiced pitch values are then median-smoothed ([MedianVoiced], default
// radius 20 ms) without bridging unvoiced gaps, and [SegmentNotes] turns the
// track into notes: a note ends after 30 ms unvoiced, when the pitch departs
// from the running note median by more than 0.6 semitone for 30 ms, or at an
// onset (see [WithOnsets]) at least 60 ms into the note; notes shorter than
// 60 ms are dropped; note starts are snapped onto onsets within 40 ms. The
// note pitch is the rounded median of its frames and its strength the mean
// voicing. All durations are options in seconds and are converted to frames
// with round(seconds*frameRate).
//
// The defaults reproduce the melody analysis of the AudioVisualizer project
// (github.com/cwbudde/AudioVisualizer) bit for bit at 24 kHz with a 240-sample
// hop, including the floating-point operation order. They also work at other
// sample rates: the hop sets the frame rate and the durations follow it.
//
// # What the estimate is, and is not
//
// The pitch is a harmonic-sum salience estimate: the single candidate whose
// harmonic comb collects the most magnitude. It is not pYIN, Melodia or a
// learned model, and it does no tracking across frames beyond the median
// filter, so:
//
//   - It is monophonic. In polyphonic music it follows whatever line has the
//     most harmonic energy in the band, which is usually, but not always, the
//     melody; chords may yield their root or a strong upper partial.
//   - Octave errors are possible, mostly towards the fundamental of a
//     subharmonic whose comb also covers the true partials.
//   - Voicing is an energy share, not a probability: a strongly harmonic
//     accompaniment is "voiced" too, and noisy or breathy leads may fall
//     below the threshold.
//   - The default FFT of 4096 samples at 24 kHz resolves about 0.4 semitone at
//     the lowest candidate and blurs fast passages and vibrato over 171 ms.
//
// Chroma is independent of the pitch estimate and is the more robust feature
// for harmony.
package melody
