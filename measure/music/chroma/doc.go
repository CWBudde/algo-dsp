// Package chroma computes per-frame 12-bin chroma (pitch-class) features
// from a constant-Q transform ([github.com/cwbudde/algo-dsp/dsp/cqt]), and
// converts them into the inputs of the harmony and motif analyses.
//
//	c, frameRate, err := chroma.Compute(x, sampleRate)
//
// returns one row per pitch class, indexed by pitch.PitchClass (C, C#, …,
// B), and the frame rate.
//
// # Algorithm
//
// [New] derives a constant-Q transform whose bins sit on the tuned semitone
// grid: with k = binsPerOctave/12 bins per semitone ([WithBinsPerOctave],
// default 36, so k = 3) the bin centres are the MIDI notes j/k relative to
// the tuning reference ([WithReferenceHz], A4 = 440 Hz by default), so the
// middle bin of every semitone lies on the note and its neighbours 1/k
// semitone either side. The transform covers exactly the grid bins inside
// the folded band ([WithFrequencyRange], to within 1e-6 semitone). The
// default band runs from C1 − 50 cents ([DefaultMinHz], 31.77 Hz) to
// B7 + 50 cents ([DefaultMaxHz], 4066.84 Hz), which at A4 = 440 Hz is seven
// octaves of 252 bins from C1 − 1/3 to B7 + 1/3 semitone. The derived
// options are, in this order,
//
//	cqt.WithBinsPerOctave(binsPerOctave), cqt.WithFMin(fmin), cqt.WithBins(bins),
//	cqt.WithHopLength(DefaultHop), cqt.WithNormalization(cqt.NormalizationWrap),
//	cqt.WithBasisNorm(cqt.NormL1), cqt.WithCenter(cqt.PadZero)
//
// followed by the options of [WithCQT], which override them, and finally
// cqt.WithOutput(cqt.OutputMagnitude), which is forced. L1 kernels with the
// wrap normalization read a sinusoid of amplitude A at a bin centre as
// magnitude A at every frequency. dsp/cqt's default librosa normalization
// would weight each bin by the square root of its kernel length, so low
// bins would dominate the fold. dsp/cqt's early downsampling is left on and
// its output factor (nnAudio's) is divided out, so the scale does not
// depend on the sample rate either.
//
// Each frame is folded bin by bin: the power (squared magnitude) of every
// bin whose centre frequency f lies in the band is added to the pitch class
// of round(pitch.FrequencyToMIDI(f, reference)) mod 12. A bin within
// 1e-6 semitone of the midpoint between two semitones (every other bin at an
// even k, such as 24 bins per octave) adds half its power to each
// neighbour, which keeps the result independent of floating-point rounding.
// Power rather than magnitude is summed, as in melody.Result.Chroma, the
// input harmony.Windows was designed for. A frame whose in-band power is
// below 1e-12 is silent and all zero. Otherwise it is normalized by
// [WithNormalization]: by its strongest class ([NormMax], the default, as
// melody.Result.Chroma), its sum ([NormL1]), its Euclidean norm ([NormL2]),
// or not at all ([NormNone]). Unnormalized, a sinusoid of amplitude A on a
// note reads about 1.5·A² in its class at 36 bins per octave: its bin
// reads A and the two neighbours, which fold into the same class, A/2 each
// (1.5 bins is the Hann window's equivalent noise bandwidth).
//
// # Frames
//
// Frame i is centred on input sample i·hop, at time i/[Chromagram.FrameRate]
// seconds, frame rate being sampleRate/hop with the hop given to the
// transform (default [DefaultHop], 512). A signal of n samples gives
// n/hop + 1 frames (integer division; [Chromagram.FrameCount], dsp/cqt's
// torch.stft(center=True) convention). Samples outside the signal are zero.
// The low bins' kernels are long (about 1.6 s at the default 31.8 Hz), so
// frames near the signal edges and note changes are smeared accordingly.
//
// dsp/cqt requires the hop, after early downsampling, to be divisible by
// 2^(octaves−1) (cqt.ErrHop). The default seven octaves need a multiple of
// 64; a hop of 240 (melody's default at 24 kHz) allows five octaves, and
// only with cqt.WithEarlyDownsampling(false), because dsp/cqt (like nnAudio)
// limits its early downsampling factor by the next power of two of the hop
// rather than by the powers of two the hop contains. The band must also lie
// below the Nyquist frequency (cqt.ErrNyquist): the default band's top bin
// (4027.9 Hz) needs a sample rate of at least 8.06 kHz. [New] reports both as errors wrapping
// [ErrInvalidOption] and the dsp/cqt sentinel.
//
// # Consumers
//
//   - harmony.Windows takes the chroma rows and the frame rate directly,
//     together with the per-frame RMS of the same signal (see the example).
//   - [Profile] averages a frame range per class: the input of
//     harmony.EstimateKey.
//   - [BeatSync] averages the frames of each beat of a rhythm.Grid: the input
//     of motif.FindChromaMotifs. It assigns frames to beats by the rule
//     harmony.Windows uses for its spans. The transposition search stays in
//     motif.
//
// # Relation to melody
//
// melody.Result.Chroma folds the bins of a 4096-point STFT between 100 Hz
// and 5 kHz, max-normalized per frame and gated by the frame RMS. This
// package folds constant-Q bins instead: every semitone gets the same number
// of bins and the same relative resolution, so bass notes are resolved as
// well as treble notes, and the band reaches down to C1. It has no RMS gate
// beyond the silence threshold.
//
// # Concurrency
//
// A [Chromagram] reuses the transform's scratch buffers and is not safe for
// concurrent use; [Chromagram.Clone] returns an independent copy that shares
// the immutable kernels and fold tables. [Compute], [Profile] and
// [BeatSync] are safe for concurrent use.
package chroma
