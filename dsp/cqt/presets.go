package cqt

import "github.com/cwbudde/algo-dsp/dsp/window"

// Parameters of nnAudio CQT2010v2's defaults, set by [NNAudio].
const (
	// NNAudioHopLength is nnAudio's default hop size.
	NNAudioHopLength = 512
	// NNAudioFMin is nnAudio's default lowest bin frequency in Hz (C1).
	NNAudioFMin = 32.70
	// NNAudioBins is nnAudio's default number of bins (seven octaves).
	NNAudioBins = 84
	// NNAudioBinsPerOctave is nnAudio's default number of bins per octave.
	NNAudioBinsPerOctave = 12
)

// Parameters of spotify/basic-pitch's CQT front end, set by [BasicPitch].
const (
	// BasicPitchSampleRate is the sample rate basic-pitch resamples its
	// input to. Pass it to [New]; a preset cannot set the sample rate.
	BasicPitchSampleRate = 22050
	// BasicPitchHopLength is basic-pitch's hop size (FFT_HOP).
	BasicPitchHopLength = 256
	// BasicPitchFMin is the frequency of basic-pitch's lowest bin in Hz
	// (A0, MIDI note 21).
	BasicPitchFMin = 27.5
	// BasicPitchBinsPerOctave is basic-pitch's resolution, three bins per
	// semitone.
	BasicPitchBinsPerOctave = 36
	// BasicPitchBins is basic-pitch's number of bins: 88 semitones plus the
	// headroom of its harmonic stacking, 8.58 octaves.
	BasicPitchBins = 309
)

// NNAudio returns the options that make [New] reproduce nnAudio's CQT2010v2
// (nnAudio 0.3.4) with its defaults: hop [NNAudioHopLength], fmin
// [NNAudioFMin], [NNAudioBins] bins with [NNAudioBinsPerOctave] per octave,
// filter scale 1, periodic Hann kernels with L1 basis norm, reflect padding
// ([PadReflect]), early downsampling, librosa normalization and magnitude
// output. Options given after the preset override it.
//
// It also turns on two nnAudio quirks that the generic transform does not
// have:
//
//   - Partial-octave kernel placement. With fewer bins than one octave
//     (bins < binsPerOctave) nnAudio builds only bins kernels but starts
//     them a full octave minus one bin below the top bin, so kernel k is
//     centred on fmin*2^((bins+k)/binsPerOctave-1), (binsPerOctave-bins)/
//     binsPerOctave octaves below [Transform.Frequencies]. Configurations
//     with at least one full octave are not affected: there the cropped
//     partial octave is the lowest one and every kernel sits at its bin.
//   - Zero padding of short octaves. When an octave's signal is not longer
//     than nfft/2 samples, torch's reflection pad raises and nnAudio zero
//     pads that octave instead. The generic transform rejects such signals
//     with [ErrSignalTooShort] under [PadReflect].
//
// nnAudio ignores its window argument and always uses Hann; [WithWindow]
// given after the preset is still honoured.
func NNAudio() []Option {
	return []Option{
		WithHopLength(NNAudioHopLength),
		WithFMin(NNAudioFMin),
		WithBins(NNAudioBins),
		WithBinsPerOctave(NNAudioBinsPerOctave),
		WithFilterScale(1),
		WithWindow(window.TypeHann),
		WithBasisNorm(NormL1),
		WithCenter(PadReflect),
		WithEarlyDownsampling(true),
		WithNormalization(NormalizationLibrosa),
		WithOutput(OutputMagnitude),
		withNNAudioKernelPlacement(),
		withNNAudioShortOctaves(),
	}
}

// BasicPitch returns [NNAudio] plus the configuration of spotify/basic-pitch's
// CQT front end: hop [BasicPitchHopLength], fmin [BasicPitchFMin],
// [BasicPitchBins] bins with [BasicPitchBinsPerOctave] per octave, filter
// scale 1, L1 basis norm, periodic Hann kernels, reflect padding, librosa
// normalization and magnitude output. Use it with [BasicPitchSampleRate]:
//
//	cqt.New(cqt.BasicPitchSampleRate, cqt.BasicPitch()...)
//
// which turns basic-pitch's 43844-sample windows into 172 frames of 309
// bins. Options given after the preset override it.
func BasicPitch() []Option {
	return append(NNAudio(),
		WithHopLength(BasicPitchHopLength),
		WithFMin(BasicPitchFMin),
		WithBins(BasicPitchBins),
		WithBinsPerOctave(BasicPitchBinsPerOctave),
		WithFilterScale(1),
		WithBasisNorm(NormL1),
		WithWindow(window.TypeHann),
		WithCenter(PadReflect),
		WithNormalization(NormalizationLibrosa),
		WithOutput(OutputMagnitude),
	)
}

// withNNAudioKernelPlacement derives the top-octave kernel frequencies as
// nnAudio does (from fmax_t/2^(1-1/binsPerOctave)) instead of from the bin
// frequencies, including nnAudio's shift of a single partial octave; see
// [NNAudio].
func withNNAudioKernelPlacement() Option {
	return func(cfg *config) error {
		cfg.nnAudioKernelPlacement = true

		return nil
	}
}

// withNNAudioShortOctaves zero pads octaves too short to reflect instead of
// rejecting the signal; see [NNAudio].
func withNNAudioShortOctaves() Option {
	return func(cfg *config) error {
		cfg.nnAudioShortOctaves = true

		return nil
	}
}
