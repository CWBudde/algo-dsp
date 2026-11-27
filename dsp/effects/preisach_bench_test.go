package effects

import (
	"strconv"
	"testing"
)

// BenchmarkPreisachProcessSample measures the per-sample cost across
// resolutions. Consecutive inputs always quantize to different steps inside
// the grid, so every call updates the rows instead of returning the cached
// output.
func BenchmarkPreisachProcessSample(b *testing.B) {
	for _, resolution := range []int{16, 128, 1024, 8192} {
		b.Run(strconv.Itoa(resolution), func(b *testing.B) {
			p, err := NewPreisach(resolution)
			if err != nil {
				b.Fatal(err)
			}

			inputs := preisachBenchInputs(resolution, 1024)

			b.ReportAllocs()
			b.ResetTimer()

			var out float32
			for i := range b.N {
				out = p.ProcessSample(inputs[i%len(inputs)])
			}

			_ = out
		})
	}
}

// preisachBenchInputs returns n inputs whose quantized steps lie strictly
// inside (-resolution, resolution) and differ from their predecessor,
// including across the wrap-around. The first step is zero.
func preisachBenchInputs(resolution, n int) []float32 {
	span := uint32(2*resolution - 3) // steps -(resolution-2) .. resolution-2
	state := uint32(1)
	inputs := make([]float32, n)
	last := 0

	for i := 1; i < n; i++ {
		state = state*1664525 + 1013904223
		step := int(state>>8%span) - (resolution - 2)

		if step == last {
			step = -step
		}

		if step == last || (i == n-1 && step == 0) {
			step = 1
			if last == 1 {
				step = 2
			}
		}

		inputs[i] = float32(step) / float32(resolution)
		last = step
	}

	return inputs
}
