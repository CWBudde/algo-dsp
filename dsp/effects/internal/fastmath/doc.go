// Package fastmath provides fast approximations for mathematical functions
// commonly used in audio DSP processing.
//
// These approximations trade a small amount of accuracy for significant
// performance improvements, making them suitable for real-time audio
// processing where speed is critical.
//
// # Accuracy Characteristics
//
// FastLog2: ~10x faster than math.Log2, <0.5% relative error for x ∈ [0.001, 100]
//
// FastPower2: ~5x faster than math.Pow(2, x), <0.1% relative error for x ∈ [-10, 10]
//
// FastSqrt: ~2-3x faster than math.Sqrt, <0.01% relative error for x ∈ [0, 1000]
//
// # Usage
//
// These functions are designed for use in performance-critical audio processing
// loops where the small accuracy trade-off is acceptable. For applications
// requiring IEEE 754 precision, use the standard library math package instead.
package fastmath
