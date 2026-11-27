package reverb

import "testing"

// BenchmarkDAVFreeverb32ProcessSample renders a 128-sample block through a
// full sixteen-stage stereo network with Freeverb-sized delay lines.
func BenchmarkDAVFreeverb32ProcessSample(b *testing.B) {
	combs := make([][2]int, davFreeverbMaxStages)
	allpasses := make([][2]int, davFreeverbMaxStages)
	allpassFeedback := make([][2]float32, davFreeverbMaxStages)

	for i := range davFreeverbMaxStages {
		combs[i] = [2]int{1116 + 61*i, 1139 + 61*i}
		allpasses[i] = [2]int{225 + 31*i, 248 + 31*i}
		allpassFeedback[i] = [2]float32{0.5, 0.5}
	}

	r, err := NewDAVFreeverb32(combs, allpasses)
	if err != nil {
		b.Fatal(err)
	}

	if err := r.SetGeometry(combs, allpasses); err != nil {
		b.Fatal(err)
	}

	if err := r.SetCombFeedback(0.84, 0.2); err != nil {
		b.Fatal(err)
	}

	if err := r.SetAllpassFeedback(allpassFeedback); err != nil {
		b.Fatal(err)
	}

	const block = 128

	input := make([]float32, block)
	for i := range input {
		input[i] = float32(i%7) * 0.01
	}

	var sink float32

	b.ReportAllocs()
	b.SetBytes(block * 4)
	b.ResetTimer()

	for range b.N {
		for _, sample := range input {
			left, right := r.ProcessSample(sample)
			sink += left + right
		}
	}

	_ = sink
}
