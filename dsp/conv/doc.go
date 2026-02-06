// Package conv provides convolution, correlation, and deconvolution routines.
//
// The package offers multiple convolution strategies optimized for different use cases:
//
//   - Direct convolution: Simple O(N*M) time-domain convolution, best for very short kernels (< 64 samples)
//   - Overlap-add (OLA): FFT-based block convolution, efficient for long signals with medium kernels
//   - Overlap-save (OLS): Alternative FFT-based block convolution with different memory characteristics
//
// The package also provides correlation functions for signal matching and alignment,
// and deconvolution with regularization for inverse filtering.
//
// # Usage
//
// For one-shot convolution, use the simple functions:
//
//	result, err := conv.Convolve(signal, kernel)  // Auto-selects best algorithm
//	result, err := conv.Direct(signal, kernel)    // Force direct convolution
//	result, err := conv.Correlate(a, b)           // Cross-correlation
//
// For repeated convolution with the same kernel, create a reusable convolver:
//
//	c, err := conv.NewOverlapAdd(kernel, blockSize)
//	result, err := c.Process(signal)
//
// # Algorithm Selection
//
// The [Convolve] function automatically selects the best algorithm based on kernel size:
//   - Kernel length < 64: Direct convolution
//   - Kernel length >= 64: FFT-based overlap-add
//
// These thresholds were determined empirically through benchmarking on typical hardware.
// The crossover point is approximately 64-128 samples for a 4096-sample signal.
//
// # Correlation
//
// Cross-correlation computes how similar two signals are as a function of displacement:
//
//	corr, err := conv.Correlate(signal, template)
//	peakIdx, peakVal := conv.FindPeak(corr)
//	lag := conv.LagFromIndex(peakIdx, len(template))
//
// Auto-correlation is useful for detecting periodicity:
//
//	acf, err := conv.AutoCorrelateNormalized(signal)
//
// # Deconvolution
//
// Deconvolution recovers an estimate of the original signal from a convolved result.
// This is an ill-posed problem that requires regularization:
//
//	opts := conv.DefaultDeconvOptions()
//	opts.Epsilon = 1e-3  // Regularization strength
//	recovered, err := conv.Deconvolve(convolved, kernel, opts)
//
// Available methods:
//   - DeconvNaive: Simple spectral division (sensitive to noise)
//   - DeconvRegularized: Adds epsilon to prevent division by small values
//   - DeconvWiener: Optimal in MSE sense when noise statistics are known
//
// # Performance
//
// Benchmark results for convolution of 4096-sample signal (typical laptop):
//
//	Kernel 8:    Direct ~64μs, FFT ~330μs (use direct)
//	Kernel 32:   Direct ~180μs, FFT ~430μs (use direct)
//	Kernel 64:   Direct ~360μs, FFT ~430μs (crossover region)
//	Kernel 128:  Direct ~650μs, FFT ~450μs (use FFT)
//	Kernel 256:  Direct ~1.4ms, FFT ~430μs (use FFT)
//	Kernel 512:  Direct ~3.2ms, FFT ~860μs (use FFT)
//
// For repeated convolution with the same kernel, pre-create an [OverlapAdd] or
// [OverlapSave] convolver to avoid repeated FFT plan creation.
package conv
