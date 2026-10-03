package conv

import (
	"math"
	"testing"

	algofft "github.com/cwbudde/algo-fft"
)

// fdlDirectReference deliberately uses scalar time-domain products, not another
// FFT convolver. Sparse late impulses keep a two-second dense IR affordable.
func fdlDirectReference[F algofft.Float](kernel, source []F) []float64 {
	output := make([]float64, len(source)+len(kernel)-1)

	for i, sample := range source {
		if sample == 0 {
			continue
		}

		for j, tap := range kernel {
			output[i+j] += float64(sample) * float64(tap)
		}
	}

	return output
}

func fdlAssertReference[F algofft.Float](t *testing.T, output []F, reference []float64, latency int, tolerance float64) {
	t.Helper()

	for i, sample := range output {
		want := 0.0
		if i >= latency && i-latency < len(reference) {
			want = reference[i-latency]
		}

		actual := float64(sample)
		if math.IsNaN(actual) || math.IsInf(actual, 0) || math.Abs(actual-want) > tolerance {
			t.Fatalf("sample %d: got %.12g want %.12g (tolerance %g)", i, actual, want, tolerance)
		}

		if i < latency && sample != 0 {
			t.Fatalf("startup sample %d is nonzero before declared latency %d", i, latency)
		}
	}
}

func fdlCheckGolden[F algofft.Float, C algofft.Complex](t *testing.T, tolerance float64) {
	t.Helper()

	kernel := []F{1, -0.5, 0.25, 0.125}
	source := []F{0.5, -0.25, 0.125}
	// Hand-computed signed dyadic products, including a cancelling zero sample.
	golden := []float64{0.5, -0.5, 0.375, -0.0625, 0, 0.015625}

	processor, err := NewPartitionedConvolutionT[F, C](kernel, 2, 2)
	if err != nil {
		t.Fatal(err)
	}

	input := make([]F, len(golden)+3*processor.Latency())
	copy(input, source)
	output := make([]F, len(input))

	for offset := range input {
		if err := processor.ProcessBlock(input[offset:offset+1], output[offset:offset+1]); err != nil {
			t.Fatal(err)
		}
	}

	fdlAssertReference(t, output, golden, processor.Latency(), tolerance)
}

func TestPartitionedFDLHandComputedGolden(t *testing.T) {
	t.Run("float64", func(t *testing.T) { fdlCheckGolden[float64, complex128](t, 1e-12) })
	t.Run("float32", func(t *testing.T) { fdlCheckGolden[float32, complex64](t, 2e-7) })
}

func fdlCheckPartitions[F algofft.Float, C algofft.Complex](t *testing.T, kernelLength, sourceLength, minimumOrder, maximumOrder int, tolerance float64) {
	t.Helper()

	kernel := make([]F, kernelLength)
	for i := range kernel {
		kernel[i] = F(0.02 * math.Cos(float64(i)*0.017) / (1 + float64(i)*0.001))
	}

	// Excite every IR sample with a dense beginning, then exercise delayed
	// input spectra after the FDL has wrapped. The final tap is never zero.
	kernel[0] = 0.5
	kernel[len(kernel)-1] = -0.125
	source := make([]F, sourceLength)

	for i := range min(257, sourceLength) {
		source[i] = F(0.1*math.Sin(float64(i)*0.31) + 0.03*math.Cos(float64(i)*0.071))
	}

	source[sourceLength/2] += 0.75
	source[sourceLength-9] -= 0.5
	reference := fdlDirectReference(kernel, source)

	processor, err := NewPartitionedConvolutionT[F, C](kernel, minimumOrder, maximumOrder)
	if err != nil {
		t.Fatal(err)
	}

	input := make([]F, len(reference)+3*processor.Latency())
	copy(input, source)

	for _, partitions := range [][]int{{len(input)}, {128}, {17, 509, 1, 1000}} {
		processor.Reset()

		output := make([]F, len(input))

		for offset, iteration := 0, 0; offset < len(input); iteration++ {
			end := min(offset+partitions[iteration%len(partitions)], len(input))
			if err := processor.ProcessBlock(input[offset:end], output[offset:end]); err != nil {
				t.Fatal(err)
			}

			offset = end
		}

		fdlAssertReference(t, output, reference, processor.Latency(), tolerance)
	}

	// Aliased input/output has the same public contract as separate buffers.
	processor.Reset()

	aliased := append([]F(nil), input...)

	for offset := 0; offset < len(aliased); {
		end := min(offset+131, len(aliased))
		if err := processor.ProcessBlock(aliased[offset:end], aliased[offset:end]); err != nil {
			t.Fatal(err)
		}

		offset = end
	}

	fdlAssertReference(t, aliased, reference, processor.Latency(), tolerance)

	// Excite a fresh stream and reset while its IR tail is still pending. Reset
	// must clear input FFT history and output while retaining the prepared IR.
	processor.Reset()

	warmFrames := min(257, sourceLength)

	if err := processor.ProcessBlock(input[:warmFrames], aliased[:warmFrames]); err != nil {
		t.Fatal(err)
	}

	processor.Reset()
	clear(aliased)

	if err := processor.ProcessBlock(aliased, aliased); err != nil {
		t.Fatal(err)
	}

	for i, sample := range aliased {
		if sample != 0 {
			t.Fatalf("reset retained sample %d: %g", i, sample)
		}
	}

	processor.Reset()

	if err := processor.ProcessBlock(input, aliased); err != nil {
		t.Fatal(err)
	}

	fdlAssertReference(t, aliased, reference, processor.Latency(), tolerance)
}

func TestPartitionedFDLDirectOraclePartitionResetAndLongTail(t *testing.T) {
	cases := []struct {
		name                                 string
		kernelLength, sourceLength, min, max int
	}{
		{"single-partial", 31, 37, 7, 13},
		{"uniform-history-wrap", 1537, 12001, 6, 6},
		{"nonuniform-final-partial", 4099, 20011, 6, 9},
		{"two-second-dense-ir-late-impulses", 96000, 192769, 7, 13},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Run("float64", func(t *testing.T) {
				fdlCheckPartitions[float64, complex128](t, tc.kernelLength, tc.sourceLength, tc.min, tc.max, 1e-9)
			})
			t.Run("float32", func(t *testing.T) {
				fdlCheckPartitions[float32, complex64](t, tc.kernelLength, tc.sourceLength, tc.min, tc.max, 2e-4)
			})
		})
	}
}

func fdlCheckPreparedAllocations[F algofft.Float, C algofft.Complex](t *testing.T) {
	t.Helper()

	kernel := make([]F, 96000)
	for i := range kernel {
		kernel[i] = F(0.01 * math.Exp(-float64(i)/12000))
	}

	processor, err := NewPartitionedConvolutionT[F, C](kernel, 7, 13)
	if err != nil {
		t.Fatal(err)
	}

	input, output := make([]F, 128), make([]F, 128)
	for i := range input {
		input[i] = F(0.1 * math.Sin(float64(i)*0.13))
	}

	render := func(blocks int) {
		for range blocks {
			if err := processor.ProcessBlock(input, output); err != nil {
				panic(err)
			}
		}
	}

	// More than five seconds wraps the longest spectral history repeatedly and
	// reaches every modulo-scheduled FFT before measuring steady processing.
	render(2048)

	if allocations := testing.AllocsPerRun(3, func() { render(256) }); allocations != 0 {
		t.Errorf("prepared steady render allocations: %g", allocations)
	}

	if allocations := testing.AllocsPerRun(3, func() {
		processor.Reset()
		render(128)
	}); allocations != 0 {
		t.Errorf("prepared reset and render allocations: %g", allocations)
	}
}

func TestPartitionedFDLPreparedZeroAllocations(t *testing.T) {
	t.Run("float64", func(t *testing.T) { fdlCheckPreparedAllocations[float64, complex128](t) })
	t.Run("float32", func(t *testing.T) { fdlCheckPreparedAllocations[float32, complex64](t) })
}
