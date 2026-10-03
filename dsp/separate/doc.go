// Package separate provides classical, weight-free source separation:
// harmonic/percussive separation (HPSS), a heuristic stereo centre
// extractor, and generic soft-mask helpers shared with other mask-based
// separators and noise reducers.
//
// Everything here is deterministic and needs no trained model. It is a
// baseline for, not a substitute for, neural separators such as Demucs.
//
// # Spectrogram layout
//
// Spectrograms are frame-major, spec[frame][bin], as returned by
// dsp/stft's Forward. The spectrogram-level methods accept any rectangular
// spectrogram; the signal-level methods (SeparateSignal) transform with a
// configurable [github.com/cwbudde/algo-dsp/dsp/stft.STFT] and invert to
// the input length.
//
// # HPSS
//
// [HPSS] implements Fitzgerald (2010), "Harmonic/percussive separation
// using median filtering": the STFT magnitude is median filtered across
// time to enhance steady (harmonic) components and across frequency to
// enhance broadband (percussive) components, and soft masks of power p are
// built from the two. [WithMargin] adds the margin variant of Driedger,
// Müller and Disch (2014), "Extending harmonic-percussive separation of
// audio signals", in the soft form librosa uses (binary with an infinite
// power), with the remainder going to a residual output. The outputs always
// sum back to the input.
//
// # Median filtering and edges
//
// [MedianFilter] is an allocation-free sliding median over a sorted window.
// Edges are handled by "reflect" padding including the edge sample
// (d c b a | a b c d | d c b a), the default of scipy.ndimage.median_filter
// that librosa's HPSS uses, so a steady tone keeps its level up to the first
// and last frame and a spectral peak at DC or Nyquist is not halved as zero
// padding would.
//
// # Masks
//
// [SoftMasks] computes generalized Wiener masks from N magnitude estimates,
// [ApplyMask] applies a real mask to a complex spectrogram, and
// [Magnitudes] extracts magnitudes. Bins where all estimates are zero are
// split evenly, so masks always sum to one and mask-based outputs always
// add up to the mixture.
//
// # Centre extraction
//
// [MidSide] and [LeftRight] convert between left/right and mid/side.
// [CentreExtractor] is a heuristic STFT-domain centre extractor: bins that
// are nearly identical in level and phase in both channels go to the centre
// output, the rest to the side outputs. It finds what is panned to the
// centre (often vocals, bass, kick and snare), which is a "vocals-ish"
// split at best, and its outputs reconstruct the input channels.
//
// # Concurrency
//
// [HPSS], [CentreExtractor] and [MedianFilter] hold scratch buffers and must
// not be used concurrently; [HPSS.Clone] and [CentreExtractor.Clone] return
// independent copies. The package-level functions are safe for concurrent
// use on distinct data.
package separate
