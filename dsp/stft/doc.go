// Package stft provides a short-time Fourier transform (STFT) and its inverse
// (ISTFT) over real signals, built on the real FFT plans of algo-fft and the
// windows of dsp/window.
//
// # Precision
//
// [Transform] is generic over the sample precision. [New] returns a float64
// transform ([STFT]); [New32] returns a float32 transform ([STFT32]) with the
// same options and framing.
//
// # Framing
//
// For a signal x of length n, hop size hop and FFT size nfft, frame i covers
// the nfft samples x[s+k], k = 0..nfft-1, with
//
//	s = i*hop - nfft/2   for PadZero and PadReflect (centred framing, the default)
//	s = i*hop            for PadNone
//
// so a centred frame i is centred on sample i*hop. Samples outside [0, n) are
// zero for PadZero and mirrored without repeating the edge sample for
// PadReflect (x[-j] = x[j], x[n-1+j] = x[n-1-j], as numpy's and torch's
// "reflect" mode). PadReflect needs n > nfft/2.
//
// Each frame is multiplied by the analysis window sample by sample
// (x[s+k]*w[k]; padded zeros are written as 0 without a multiply) and then
// transformed with a forward real FFT, giving nfft/2+1 bins. No scaling is
// applied unless [WithNormalized] is set, in which case the spectrum is
// scaled by 1/sqrt(nfft) (torch.stft(normalized=True)).
//
// [Transform.FrameCount] returns ceil(n/hop) for centred framing, which is the
// count used by the AudioVisualizer analysis. torch.stft(center=True) returns
// n/hop+1 (integer division) frames, which is one more frame when hop divides
// n; [Transform.FrameInto] accepts that extra trailing frame as well. With
// PadNone the count is 1+(n-nfft)/hop for n >= nfft and 0 otherwise. The
// constant-Q transform in dsp/cqt follows torch's n/hop+1 convention instead
// (see its Framing section).
//
// # Inverse
//
// [Transform.Inverse] overlap-adds the windowed inverse FFT of each frame and
// divides each output sample by the sum of the squared windows that overlap
// it (window-sum-square normalization). This reconstructs the input exactly
// for any window/hop pair with a non-vanishing window sum, not only for COLA
// pairs. Output samples whose window sum is below 1e-10 cannot be recovered
// and make Inverse return [ErrWindowSumZero]; with PadNone this includes the
// first sample whenever the window starts with a zero (periodic Hann).
//
// # Concurrency
//
// A Transform owns scratch buffers, so its methods must not be called
// concurrently. Use [Transform.Clone] to obtain an independent Transform per
// goroutine; clones share the immutable window and FFT plan.
package stft
