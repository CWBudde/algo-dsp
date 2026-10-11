// Package cqt provides a multi-rate constant-Q transform (CQT) of real
// signals: bins spaced geometrically at fmin*2^(b/binsPerOctave), each
// analysed by a windowed complex exponential whose length is inversely
// proportional to its frequency.
//
// Without options [New] gives seven octaves of semitone bins from C1:
//
//	tr, err := cqt.New(sampleRate)
//
// The presets [NNAudio] and [BasicPitch] reproduce nnAudio's CQT2010v2
// (nnAudio 0.3.4) and the front end of spotify/basic-pitch, which ported it:
//
//	tr, err := cqt.New(cqt.BasicPitchSampleRate, cqt.BasicPitch()...)
//
// turns basic-pitch's 43844-sample windows into 172 frames of 309 bins.
//
// # Algorithm
//
// The bins are fmin*2^(b/binsPerOctave), b = 0..bins-1 ([Transform.Frequencies]),
// grouped into ceil(bins/binsPerOctave) octaves. Only the top octave has its
// own kernels; every lower octave reuses them on a signal decimated by 2 per
// octave. The quality factor is Q = filterScale/(2^(1/binsPerOctave)-1).
//
//  1. The top octave's kernels analyse the highest min(bins, binsPerOctave)
//     bins. Kernel k (frequency f_k) is a periodic window of length
//     l_k = ceil(Q*fs/f_k) times exp(i*2*pi*f_k*m/fs), divided by l_k and
//     normalized ([WithBasisNorm]); m runs from floor(-l_k/2), and fs is the
//     sample rate after early downsampling (step 2). All kernels
//     are centred in a frame of nfft samples, the next power of two of the
//     longest kernel ([Transform.NFFT]).
//  2. If early downsampling applies ([WithEarlyDownsampling]), the input is
//     first low-pass filtered and decimated by a power of two, and the hop and
//     sample rate fs are divided by the same factor.
//  3. The top octave is analysed: the signal is padded by nfft/2 samples on
//     each side ([WithCenter]) and correlated with every kernel at stride hop
//     (torch conv1d, so the kernels are not flipped). The imaginary part is
//     the negated correlation with the imaginary kernel, as in nnAudio.
//  4. For every lower octave the signal is filtered with a 256-tap firwin2
//     half-band low-pass, decimated by 2, and analysed with the same kernels
//     and half the hop, so kernel k of octave o analyses f_k/2^o, which is
//     again a bin frequency.
//  5. The octaves are stacked lowest first, the lowest octave's kernels below
//     fmin are dropped, and every bin is multiplied by the early downsampling
//     factor and by the [Normalization] scale (sqrt(l_b) for
//     NormalizationLibrosa, l_b being [Transform.Lengths]).
//
// Every kernel is therefore centred on the frequency Frequencies() reports.
// With fewer bins than one octave the single octave starts at fmin: the
// result equals building a full octave of kernels from fmin and dropping
// the ones above the top bin (which are not built, as they may lie above the
// Nyquist frequency). The anti-alias filters are those of nnAudio's
// create_lowpass_filter, designed with [design.Firwin2].
//
// # Output layout
//
// Output is frame-major. For [OutputMagnitude], value dst[frame*Bins()+bin]
// is the magnitude of bin bin (lowest frequency first) in frame frame. For
// [OutputComplex], dst[2*(frame*Bins()+bin)] and the element after it hold
// the real and imaginary part. nnAudio returns (bins, frames[, 2]); the
// transpose suits frame-wise consumers such as basic-pitch's network input.
//
// # Framing
//
// Frames are always centred: frame i of every octave is centred (to within
// the decimation filters' half-sample offsets) on input sample i*hop, hop
// being the value given to [WithHopLength]. Samples outside the signal are
// zero ([PadZero], the default, as in dsp/stft and in librosa's CQT since
// 0.10) or mirrored without repeating the edge sample ([PadReflect],
// nnAudio's default). Reflection needs every octave to be longer than nfft/2 samples,
// just as dsp/stft's PadReflect needs n > nfft/2; since the octaves shrink,
// the lowest one decides, and a shorter signal gives [ErrSignalTooShort].
// Under either padding, a signal too short to be decimated down to the
// lowest octave gives ErrSignalTooShort too.
//
// A signal of n samples gives L/Hop()+1 frames (integer division), L being
// its length after early downsampling, so n/hop+1 frames without it; see
// [Transform.FrameCount]. This is torch.stft(center=True)'s convention,
// which nnAudio inherits. dsp/stft's FrameCount returns ceil(n/hop) for
// centred framing instead: one frame fewer when hop divides n, the same
// count otherwise.
//
// [Padding] has the names and numeric values of dsp/stft's Padding
// (PadZero = 1, PadReflect = 2) and [WithCenter] is named after
// stft.WithCenter, but the type is not shared: the multi-rate transform
// needs centred frames in every octave, so stft's PadNone (uncentred frames)
// has no counterpart here.
//
// # Precision
//
// Kernels, filters and all arithmetic are float64. [Transform.ProcessInto32]
// takes and returns float32 but converts the input to float64 and rounds
// only the result. dsp/stft is generic over its precision instead (New and
// New32 returning Transform[F, C]); here a type parameter would only change
// the I/O type while doubling the constructor and type surface, so a single
// Transform serves both precisions and shares its kernels and scratch.
//
// # Presets
//
// [NNAudio] returns the options that reproduce nnAudio's CQT2010v2 with its
// defaults (hop 512, fmin 32.70 Hz, 84 bins, 12 per octave, reflect padding,
// the other parameters as the generic defaults). [BasicPitch] is NNAudio()
// plus basic-pitch's configuration (hop 256, fmin 27.5 Hz, 309 bins, 36 per
// octave); use it with [BasicPitchSampleRate]. Options given after a preset
// override its parameters, but not the two nnAudio quirks the presets turn
// on, which the generic transform does not have:
//
//   - Partial-octave kernel placement. For bins < binsPerOctave nnAudio
//     builds only bins kernels but still starts them a full octave minus one
//     bin below the top bin, at fmin*2^(bins/binsPerOctave-1). Kernel k is
//     then centred on fmin*2^((bins+k)/binsPerOctave-1),
//     (binsPerOctave-bins)/binsPerOctave octaves below [Transform.Frequencies],
//     which still reports the nominal frequencies, as nnAudio does. The
//     generic transform starts the kernels at fmin. Configurations with at
//     least one full octave, including both presets' own, are not affected.
//   - Zero padding of short octaves. When an octave's signal is not longer
//     than nfft/2 samples, torch's reflection pad raises, and nnAudio catches
//     that and zero pads the octave instead. The presets do the same; the
//     generic transform rejects such signals under PadReflect with
//     [ErrSignalTooShort] ([Transform.FrameCount] returns 0).
//
// Even under the presets, this package deliberately differs from nnAudio in
// these points:
//
//   - The window is honoured. CQT2010v2 accepts a window argument but never
//     forwards it to create_cqt_kernels, so nnAudio and basic-pitch always use
//     Hann. [WithWindow] selects the window for real; with the default Hann
//     window the result matches nnAudio.
//   - The hop is checked. [New] requires the effective hop (after early
//     downsampling) to be divisible by 2^(octaves-1) and returns [ErrHop]
//     otherwise. nnAudio halves the hop with integer division per octave;
//     for other hops the octaves give different frame counts and nnAudio
//     fails when it concatenates them, or (for a hop decimated to 0) when it
//     runs the convolution.
//   - Arithmetic is float64 throughout, including kernels and filters, which
//     nnAudio keeps in float32/complex64. Against nnAudio run in float64 the
//     results agree to about 1e-13 relative to each bin's peak magnitude;
//     against nnAudio as shipped (float32) the difference is about 1.3e-6, all
//     of it nnAudio's float32 rounding.
//   - A kernel of length 1 uses the window value 1, as scipy's get_window
//     (which nnAudio calls) does, rather than dsp/window's value at the
//     window edge. Configurations whose longest kernel has a single sample,
//     so that nfft is 1, are rejected with [ErrInvalidOption].
//   - The output is frame-major (see Output layout).
//
// NaN or Inf samples propagate into the frames that cover them.
//
// # Performance and concurrency
//
// The kernels are stored by their non-zero support and evaluated as direct
// dot products, the real and imaginary parts in one fused pass. Octaves are
// reflect-padded with [core.PadReflect] and decimated with
// [conv.CorrelateStridedInto]. [Transform.ProcessInto] and
// [Transform.ProcessInto32] grow their scratch buffers to the input length
// once and do not allocate after that. A Transform is not safe for
// concurrent use; [Transform.Clone] returns an independent Transform that
// shares the immutable kernels and filters.
package cqt
