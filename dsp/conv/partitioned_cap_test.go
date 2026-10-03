package conv

import (
	"fmt"
	"math"
	"slices"
	"testing"

	algofft "github.com/cwbudde/algo-fft"
)

func capDirectFixture[F algofft.Float]() ([]F, []F, []float64) {
	// Every sample of a two-second IR participates in an independent scalar
	// oracle. Sparse source impulses bound the cost without shortening the IR.
	kernel := make([]F, 96000)
	for i := range kernel {
		kernel[i] = F(.013 * math.Cos(float64(i)*.019) / (1 + float64(i)*.0003))
	}

	kernel[0], kernel[65536], kernel[len(kernel)-1] = .5, .25, -.125
	source := make([]F, 96019)
	source[0], source[127], source[8191], source[48013], source[len(source)-1] = 1, -.5, .25, .125, -.75

	return kernel, source, fdlDirectReference(kernel, source)
}

func capRender[F algofft.Float, C algofft.Complex](t *testing.T, processor *PartitionedConvolutionT[F, C], input, output []F, partitions []int) {
	t.Helper()

	for offset, block := 0, 0; offset < len(input); block++ {
		end := min(offset+partitions[block%len(partitions)], len(input))
		if err := processor.ProcessBlock(input[offset:end], output[offset:end]); err != nil {
			t.Fatal(err)
		}

		offset = end
	}
}

func capCheckDirectTail[F algofft.Float, C algofft.Complex](t *testing.T, maximumOrder int, tolerance float64) {
	t.Helper()

	kernel, source, reference := capDirectFixture[F]()

	processor, err := NewPartitionedConvolutionT[F, C](kernel, 7, maximumOrder)
	if err != nil {
		t.Fatal(err)
	}

	if processor.Latency() != 128 {
		t.Fatalf("maximum order %d changed latency to %d", maximumOrder, processor.Latency())
	}

	if reference[len(reference)-1] != .09375 {
		t.Fatalf("independent final late-IR product %g want .09375", reference[len(reference)-1])
	}

	input := make([]F, len(reference)+processor.Latency()+3*(1<<maximumOrder))
	copy(input, source)

	for _, partitions := range [][]int{{128}, {1, 17, 509, 3, 1000}} {
		for _, alias := range []bool{false, true} {
			processor.Reset()

			in, output := input, make([]F, len(input))
			if alias {
				output = slices.Clone(input)
				in = output
			}

			capRender(t, processor, in, output, partitions)
			fdlAssertReference(t, output, reference, 128, tolerance)
		}
	}

	// Reset while a nonzero two-second tail is active, including a partly filled
	// large-hop stage; subsequent silence must be exactly silent for the full IR.
	processor.Reset()

	warm := make([]F, 1437)
	capRender(t, processor, input[:len(warm)], warm, []int{128, 17})
	processor.Reset()

	zeros := make([]F, len(input))
	capRender(t, processor, zeros, zeros, []int{128, 31, 1})

	for i, sample := range zeros {
		if sample != 0 {
			t.Fatalf("maximum order %d retained active tail at frame %d: %g", maximumOrder, i, sample)
		}
	}

	// The same prepared IR remains usable after reset; verify all late products
	// again rather than accepting a reset that accidentally discards the IR.
	processor.Reset()
	capRender(t, processor, input, zeros, []int{128})
	fdlAssertReference(t, zeros, reference, 128, tolerance)
}

func TestPartitionedCapIndependentDenseIRCompleteTail(t *testing.T) {
	for _, maximumOrder := range []int{10, 11, 13} {
		t.Run(fmt.Sprintf("maximum-order-%d", maximumOrder), func(t *testing.T) {
			t.Run("float64", func(t *testing.T) {
				capCheckDirectTail[float64, complex128](t, maximumOrder, 1e-9)
			})
			t.Run("float32", func(t *testing.T) {
				capCheckDirectTail[float32, complex64](t, maximumOrder, 2e-4)
			})
		})
	}
}

func capCheckPreparedAllocations[F algofft.Float, C algofft.Complex](t *testing.T, maximumOrder int) {
	t.Helper()

	kernel, _, _ := capDirectFixture[F]()

	processor, err := NewPartitionedConvolutionT[F, C](kernel, 7, maximumOrder)
	if err != nil {
		t.Fatal(err)
	}

	input, output := make([]F, 128), make([]F, 128)
	for i := range input {
		input[i] = F(.1*math.Sin(float64(i)*.13) + .03*math.Cos(float64(i)*.051))
	}

	render := func(blocks int) {
		for range blocks {
			if err := processor.ProcessBlock(input, output); err != nil {
				panic(err)
			}
		}
	}
	// Cross the full two-second IR history before measuring every scheduled FFT.
	render(1024)

	if allocations := testing.AllocsPerRun(3, func() { render(128) }); allocations != 0 {
		t.Fatalf("maximum order %d steady allocations %g", maximumOrder, allocations)
	}

	if allocations := testing.AllocsPerRun(3, func() {
		processor.Reset()
		render(128)
	}); allocations != 0 {
		t.Fatalf("maximum order %d reset/render allocations %g", maximumOrder, allocations)
	}
}

func TestPartitionedCapPreparedZeroAllocations(t *testing.T) {
	for _, maximumOrder := range []int{10, 11, 13} {
		t.Run(fmt.Sprintf("maximum-order-%d", maximumOrder), func(t *testing.T) {
			t.Run("float64", func(t *testing.T) {
				capCheckPreparedAllocations[float64, complex128](t, maximumOrder)
			})
			t.Run("float32", func(t *testing.T) {
				capCheckPreparedAllocations[float32, complex64](t, maximumOrder)
			})
		})
	}
}
