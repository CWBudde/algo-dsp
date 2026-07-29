// Package pitch provides reusable non-I/O pitch analysis and pitch-shifting
// processors.
//
// Shifting:
//   - PitchShifter: Time-domain WSOLA-style pitch shifter.
//   - SpectralPitchShifter: Frequency-domain phase-vocoder pitch shifter.
//   - PitchProcessor: Shared interface for interchangeable shifters.
//
// Analysis and correction:
//   - YINDetector: YIN fundamental frequency estimation for a single frame.
//   - PitchTracker: Streaming wrapper adding buffering, hop scheduling and
//     median smoothing around the detector.
//   - PitchCorrector: Auto-tune style correction combining a tracker with a
//     shifter to snap a signal to a scale or a fixed target.
//
// Musical helpers:
//   - Scale, PitchClass and the Scale* constructors describe the note grid
//     that correction snaps to.
//   - FrequencyToMIDI, MIDIToFrequency, SemitonesToRatio, RatioToSemitones and
//     CentsBetween convert between frequency, note and interval.
//
// Correction is expressed as a frequency ratio, so the frequency shifter in
// the modulation package is not an alternative to the shifters here: it
// translates every partial by a constant number of hertz, which destroys the
// harmonic relationship between them.
package pitch
