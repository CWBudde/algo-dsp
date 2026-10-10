// Package cqt provides a multi-rate constant-Q transform (CQT) of real
// signals that reproduces nnAudio's CQT2010v2 (nnAudio 0.3.4), the CQT that
// spotify/basic-pitch ported to TensorFlow for its front end.
//
// The basic-pitch configuration is
//
//	cqt.New(22050, cqt.WithHopLength(256), cqt.WithFMin(27.5),
//		cqt.WithBins(309), cqt.WithBinsPerOctave(36))
//
// which turns basic-pitch's 43844-sample windows into 172 frames of 309 bins.
// Without options [New] uses nnAudio's defaults.
//
// # Algorithm
//
// The bins are fmin*2^(b/binsPerOctave), b = 0..bins-1, grouped into
// ceil(bins/binsPerOctave) octaves; only the top octave has its own kernels,
// with quality factor Q = filterScale/(2^(1/binsPerOctave)-1):
//
//  1. Kernel k of the top octave (frequency f_k) is a periodic window of
//     length l_k = ceil(Q*fs/f_k) times exp(i*2*pi*f_k*m/fs), divided by l_k
//     and normalized ([WithBasisNorm]); m runs from floor(-l_k/2). All
//     kernels are centred in a frame of nfft samples, the next power of two
//     of the longest kernel.
//  2. If early downsampling applies ([WithEarlyDownsampling]), the input is
//     first low-pass filtered and decimated by a power of two, and the hop and
//     sample rate are divided by the same factor.
//  3. The top octave is analysed: the signal is padded by nfft/2 samples on
//     each side ([WithPadding]) and correlated with every kernel at stride hop
//     (torch conv1d, so the kernels are not flipped). The imaginary part is
//     the negated correlation with the imaginary kernel, as in nnAudio.
//  4. For every lower octave the signal is filtered with a 256-tap firwin2
//     half-band low-pass, decimated by 2, and analysed with the same kernels
//     and half the hop.
//  5. The octaves are stacked lowest first, the surplus bins of the lowest
//     octave are dropped, and every bin is multiplied by the early
//     downsampling factor and by the [Normalization] scale (sqrt(l_b) for
//     NormalizationLibrosa).
//
// The anti-alias filters are those of nnAudio's create_lowpass_filter,
// designed with [design.Firwin2].
//
// # Output layout
//
// Output is frame-major. For [OutputMagnitude], value dst[frame*NumBins()+bin]
// is the magnitude of bin bin (lowest frequency first) in frame frame. For
// [OutputComplex], dst[2*(frame*NumBins()+bin)] and the element after it hold
// the real and imaginary part. nnAudio returns (bins, frames[, 2]); the
// transpose suits frame-wise consumers such as basic-pitch's network input.
//
// Frame i of every octave is centred (to within the decimation filters'
// half-sample offsets) on input sample i*hop, hop being the value given to
// [WithHopLength]. A signal of n samples gives L/Hop()+1 frames, L being its
// length after early downsampling; see [Transform.NumFrames].
//
// # Differences from nnAudio
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
//     of it nnAudio's float32 rounding. [Transform.ProcessInto32] takes and
//     returns float32 but still computes in float64.
//   - A kernel of length 1 uses the window value 1, as scipy's get_window
//     does; configurations whose longest kernel has a single sample are
//     rejected.
//
// nnAudio's handling of short signals is mirrored: when an octave's signal is
// not longer than nfft/2 samples, torch's reflection pad raises, nnAudio
// catches that and zero pads the octave instead, and so does this package.
// A signal too short to be decimated down to the lowest octave yields
// [ErrSignalTooShort]. NaN or Inf samples propagate into the frames that
// cover them.
//
// # Performance and concurrency
//
// The kernels are stored by their non-zero support and evaluated as direct
// dot products. [Transform.ProcessInto] and [Transform.ProcessInto32] grow
// their scratch buffers to the input length once and do not allocate after
// that. A Transform is not safe for concurrent use; [Transform.Clone] returns
// an independent Transform that shares the immutable kernels and filters.
package cqt
