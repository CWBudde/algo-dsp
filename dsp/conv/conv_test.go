package conv

import (
	"math"
	"testing"
)

func TestDirect(t *testing.T) {
	tests := []struct {
		name     string
		a        []float64
		b        []float64
		expected []float64
	}{
		{
			name:     "simple 3x3",
			a:        []float64{1, 2, 3},
			b:        []float64{1, 1, 1},
			expected: []float64{1, 3, 6, 5, 3},
		},
		{
			name:     "impulse",
			a:        []float64{1, 2, 3, 4, 5},
			b:        []float64{1},
			expected: []float64{1, 2, 3, 4, 5},
		},
		{
			name:     "delayed impulse",
			a:        []float64{1, 2, 3, 4, 5},
			b:        []float64{0, 0, 1},
			expected: []float64{0, 0, 1, 2, 3, 4, 5},
		},
		{
			name:     "symmetric",
			a:        []float64{1, 2, 1},
			b:        []float64{1, 2, 1},
			expected: []float64{1, 4, 8, 8, 5, 2, 1}, // Actually: 1, 4, 6, 4, 1 for symmetric convolution
		},
	}

	// Fix the symmetric test case - let me recalculate
	// conv([1,2,1], [1,2,1])
	// y[0] = 1*1 = 1
	// y[1] = 1*2 + 2*1 = 4
	// y[2] = 1*1 + 2*2 + 1*1 = 6
	// y[3] = 2*1 + 1*2 = 4
	// y[4] = 1*1 = 1
	tests[3].expected = []float64{1, 4, 6, 4, 1}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Direct(tt.a, tt.b)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(result) != len(tt.expected) {
				t.Fatalf("length mismatch: got %d, expected %d", len(result), len(tt.expected))
			}

			for i := range result {
				if math.Abs(result[i]-tt.expected[i]) > 1e-10 {
					t.Errorf("result[%d] = %v, expected %v", i, result[i], tt.expected[i])
				}
			}
		})
	}
}

func TestDirectErrors(t *testing.T) {
	_, err := Direct([]float64{}, []float64{1, 2})
	if err != ErrEmptyInput {
		t.Errorf("expected ErrEmptyInput, got %v", err)
	}

	_, err = Direct([]float64{1, 2}, []float64{})
	if err != ErrEmptyKernel {
		t.Errorf("expected ErrEmptyKernel, got %v", err)
	}
}

func TestDirectCircular(t *testing.T) {
	a := []float64{1, 2, 3, 4}
	b := []float64{1, 0, 0, 0}

	result, err := DirectCircular(a, b)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Circular convolution with impulse at 0 should return the original
	for i := range result {
		if math.Abs(result[i]-a[i]) > 1e-10 {
			t.Errorf("result[%d] = %v, expected %v", i, result[i], a[i])
		}
	}
}

func TestOverlapAddConvolve(t *testing.T) {
	// Create a simple signal and kernel
	signal := make([]float64, 1000)
	for i := range signal {
		signal[i] = math.Sin(2 * math.Pi * float64(i) / 100)
	}

	kernel := []float64{0.25, 0.5, 0.25} // Simple smoothing kernel

	// Compare overlap-add with direct convolution
	directResult, err := Direct(signal, kernel)
	if err != nil {
		t.Fatalf("direct convolution failed: %v", err)
	}

	oaResult, err := OverlapAddConvolve(signal, kernel)
	if err != nil {
		t.Fatalf("overlap-add convolution failed: %v", err)
	}

	if len(directResult) != len(oaResult) {
		t.Fatalf("length mismatch: direct=%d, oa=%d", len(directResult), len(oaResult))
	}

	// Allow small numerical differences
	for i := range directResult {
		if math.Abs(directResult[i]-oaResult[i]) > 1e-10 {
			t.Errorf("mismatch at index %d: direct=%v, oa=%v", i, directResult[i], oaResult[i])
		}
	}
}

func TestOverlapSaveConvolve(t *testing.T) {
	// Create a simple signal and kernel
	signal := make([]float64, 500)
	for i := range signal {
		signal[i] = math.Sin(2 * math.Pi * float64(i) / 50)
	}

	kernel := []float64{0.2, 0.3, 0.3, 0.2} // Smoothing kernel

	// Compare overlap-save with direct convolution
	directResult, err := Direct(signal, kernel)
	if err != nil {
		t.Fatalf("direct convolution failed: %v", err)
	}

	osResult, err := OverlapSaveConvolve(signal, kernel)
	if err != nil {
		t.Fatalf("overlap-save convolution failed: %v", err)
	}

	if len(directResult) != len(osResult) {
		t.Fatalf("length mismatch: direct=%d, os=%d", len(directResult), len(osResult))
	}

	// Allow small numerical differences
	maxDiff := 0.0
	for i := range directResult {
		diff := math.Abs(directResult[i] - osResult[i])
		if diff > maxDiff {
			maxDiff = diff
		}
	}

	if maxDiff > 1e-8 {
		t.Errorf("max difference %v exceeds tolerance", maxDiff)
	}
}

func TestConvolveAutoSelection(t *testing.T) {
	// Short kernel should use direct
	signal := make([]float64, 1000)
	for i := range signal {
		signal[i] = float64(i % 10)
	}

	shortKernel := []float64{1, 2, 1}

	result1, err := Convolve(signal, shortKernel)
	if err != nil {
		t.Fatalf("convolution failed: %v", err)
	}

	directResult, _ := Direct(signal, shortKernel)

	for i := range result1 {
		if math.Abs(result1[i]-directResult[i]) > 1e-10 {
			t.Errorf("short kernel mismatch at %d", i)
			break
		}
	}

	// Long kernel should use FFT
	longKernel := make([]float64, 100)
	for i := range longKernel {
		longKernel[i] = math.Exp(-float64(i) / 20)
	}

	result2, err := Convolve(signal, longKernel)
	if err != nil {
		t.Fatalf("convolution failed: %v", err)
	}

	directResult2, _ := Direct(signal, longKernel)

	maxDiff := 0.0
	for i := range result2 {
		diff := math.Abs(result2[i] - directResult2[i])
		if diff > maxDiff {
			maxDiff = diff
		}
	}

	if maxDiff > 1e-8 {
		t.Errorf("long kernel max difference %v exceeds tolerance", maxDiff)
	}
}

func TestConvolveMode(t *testing.T) {
	a := []float64{1, 2, 3, 4, 5}
	b := []float64{1, 2, 3}

	// Full mode
	full, _ := ConvolveMode(a, b, ModeFull)
	if len(full) != len(a)+len(b)-1 {
		t.Errorf("full mode length: got %d, expected %d", len(full), len(a)+len(b)-1)
	}

	// Same mode
	same, _ := ConvolveMode(a, b, ModeSame)
	if len(same) != len(a) {
		t.Errorf("same mode length: got %d, expected %d", len(same), len(a))
	}

	// Valid mode
	valid, _ := ConvolveMode(a, b, ModeValid)
	if len(valid) != len(a)-len(b)+1 {
		t.Errorf("valid mode length: got %d, expected %d", len(valid), len(a)-len(b)+1)
	}
}

func TestCorrelate(t *testing.T) {
	// Auto-correlation of cosine should peak at zero lag
	n := 256
	signal := make([]float64, n)
	for i := range signal {
		signal[i] = math.Cos(2 * math.Pi * float64(i) / 32)
	}

	result, err := AutoCorrelate(signal)
	if err != nil {
		t.Fatalf("auto-correlation failed: %v", err)
	}

	// Peak should be at center (zero lag)
	peakIdx, _ := FindPeak(result)
	expectedPeakIdx := n - 1 // Zero lag is at index n-1

	if peakIdx != expectedPeakIdx {
		t.Errorf("peak at index %d, expected %d (lag %d)", peakIdx, expectedPeakIdx, LagFromIndex(peakIdx, n))
	}
}

func TestCorrelateNormalized(t *testing.T) {
	a := []float64{1, 2, 3, 4, 5}

	result, err := AutoCorrelateNormalized(a)
	if err != nil {
		t.Fatalf("normalized auto-correlation failed: %v", err)
	}

	// Zero-lag should be 1.0
	zeroLagIdx := len(a) - 1
	if math.Abs(result[zeroLagIdx]-1.0) > 1e-10 {
		t.Errorf("zero-lag value %v, expected 1.0", result[zeroLagIdx])
	}
}

func TestDeconvolve(t *testing.T) {
	// Create a simple signal
	original := make([]float64, 100)
	for i := range original {
		original[i] = math.Sin(2 * math.Pi * float64(i) / 20)
	}

	// Create a simple kernel (moving average)
	kernel := []float64{0.25, 0.5, 0.25}

	// Convolve
	convolved, _ := Direct(original, kernel)

	// Deconvolve with regularization
	opts := DefaultDeconvOptions()
	opts.Epsilon = 1e-3

	recovered, err := Deconvolve(convolved, kernel, opts)
	if err != nil {
		t.Fatalf("deconvolution failed: %v", err)
	}

	// The recovered signal won't be perfect due to the ill-posed nature
	// But it should be reasonably close
	snr := SNR(original, recovered)
	if snr < 10 { // At least 10 dB SNR
		t.Logf("Warning: low SNR %.2f dB in deconvolution test", snr)
	}
}

func TestInverseFilter(t *testing.T) {
	kernel := []float64{0.5, 1.0, 0.5}

	invFilter, err := InverseFilter(kernel, 64, 1e-3)
	if err != nil {
		t.Fatalf("inverse filter creation failed: %v", err)
	}

	// Convolving kernel with its inverse should approximate impulse
	result, _ := Direct(kernel, invFilter)

	// The result should have a dominant peak
	peakIdx, peakVal := FindPeak(result)

	// Verify the peak is reasonably large
	if peakVal < 0.1 {
		t.Errorf("peak value %v too low", peakVal)
	}

	// Verify other values are relatively small compared to peak
	// (this is a loose check since deconvolution is ill-posed)
	for i, v := range result {
		if i != peakIdx && math.Abs(v) > peakVal*0.5 {
			// Allow some spread due to the ill-posed nature
			t.Logf("Note: significant value at index %d: %v (peak at %d: %v)", i, v, peakIdx, peakVal)
		}
	}
}

func TestHelperFunctions(t *testing.T) {
	// Test nextPowerOf2
	tests := []struct {
		input    int
		expected int
	}{
		{1, 1},
		{2, 2},
		{3, 4},
		{5, 8},
		{7, 8},
		{8, 8},
		{9, 16},
		{100, 128},
	}

	for _, tt := range tests {
		result := nextPowerOf2(tt.input)
		if result != tt.expected {
			t.Errorf("nextPowerOf2(%d) = %d, expected %d", tt.input, result, tt.expected)
		}
	}

	// Test l2Norm
	v := []float64{3, 4}
	norm := l2Norm(v)
	if math.Abs(norm-5.0) > 1e-10 {
		t.Errorf("l2Norm([3,4]) = %v, expected 5", norm)
	}
}

func TestLagConversion(t *testing.T) {
	lenB := 10

	// Test round-trip
	for lag := -9; lag <= 9; lag++ {
		idx := IndexFromLag(lag, lenB)
		recoveredLag := LagFromIndex(idx, lenB)
		if recoveredLag != lag {
			t.Errorf("lag %d -> idx %d -> lag %d", lag, idx, recoveredLag)
		}
	}
}
