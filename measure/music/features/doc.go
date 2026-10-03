// Package features extracts frame-wise music-analysis features from
// multichannel audio: RMS and peak level, spectral centroid, stereo width,
// positive spectral flux, per-band RMS envelopes and an optional
// log-frequency spectrogram. It also provides the envelope normalizer,
// percentile and silence finder used to turn those features into control
// curves.
//
// # Framing
//
// [Extract] uses a centred short-time Fourier transform ([stft.New] with a
// periodic Hann window and zero padding): frame i is centred on sample
// i*Hop, the first frame is half zero-padded, and a signal of n samples
// yields ceil(n/Hop) frames. [Timing] converts between frame indices and
// seconds.
//
// # Feature definitions
//
// For each frame the power spectra of all channels are averaged (power, not
// signals, so anti-phase content is not cancelled):
//
//	P[k] = Σ_c |X_c[k]|² / C
//
// From P the extractor derives
//
//   - Centroid: spectral centroid of sqrt(P) in Hz (stats/frequency.Centroid);
//   - Flux: Σ_k max(0, log1p(sqrt(P[k])) - log1p(sqrt(P_prev[k]))), zero for
//     the first frame;
//   - Bands[b]: sqrt(Σ g_k·P[k]/(N·Σw²)) over the bins with
//     edge[b] <= f_k < edge[b+1], so a full-scale sine reads ≈ its RMS
//     amplitude (one-sided power scaling). g_k is 2, except 1 for DC and,
//     for an even N, the Nyquist bin, which have no negative-frequency
//     partner; a band over all bins reads the window-weighted frame RMS.
//
// RMS and Peak pool all channels over the samples [i*Hop-Hop, i*Hop+Hop).
// Width is side/(mid+side) with mid = (L+R)²/4 and side = (L-R)²/4 summed
// over the same samples; it is computed for exactly two channels and zero
// otherwise. Centroid and Flux are forced to zero in frames whose RMS is
// below [Config.Gate].
//
// # Log-frequency spectrogram
//
// [WithLogSpectrogram] adds a frame-major spectrogram on a logarithmic
// frequency grid, in dBFS with the same one-sided scaling as the bands. FFT
// bins are mapped onto the log grid with an energy-preserving overlap
// weighting (see [LogScale]), so no log bin is empty even where log bins are
// narrower than the FFT bin spacing.
//
// The extractor is deterministic: the same input and configuration always
// produce bit-identical output.
package features
