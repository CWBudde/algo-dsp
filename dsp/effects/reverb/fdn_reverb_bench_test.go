package reverb

import "testing"

// The block the benchmarks render, in samples. 128 is a Web Audio render
// quantum, which is the case the inner loop was rewritten for.
const fdnBenchBlock = 128

func benchmarkFDN(b *testing.B, process func(r *FDNReverb, buf []float64)) {
	b.Helper()

	r, err := NewFDNReverb(48000)
	if err != nil {
		b.Fatalf("NewFDNReverb: %v", err)
	}

	buf := make([]float64, fdnBenchBlock)
	buf[0] = 1

	b.ReportAllocs()
	b.SetBytes(int64(len(buf) * 8))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		process(r, buf)
	}
}

func BenchmarkFDNReverbProcessBlock(b *testing.B) {
	benchmarkFDN(b, func(r *FDNReverb, buf []float64) {
		r.ProcessInPlace(buf)
	})
}

// BenchmarkFDNReverbProcessBlockReference renders the same block through the
// pre-rewrite implementation, so the speedup is a number this suite produces
// rather than one a commit message asserts.
func BenchmarkFDNReverbProcessBlockReference(b *testing.B) {
	phase := 0.0

	benchmarkFDN(b, func(r *FDNReverb, buf []float64) {
		for i := range buf {
			buf[i] = referenceProcessSample(r, &phase, buf[i])
		}
	})
}

func BenchmarkFDNHadamardInPlace(b *testing.B) {
	v := [fdnSize]float64{1, 2, 3, 4, 5, 6, 7, 8}

	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		hadamardInPlace(&v)
	}
}

func BenchmarkFDNHadamardMatrix(b *testing.B) {
	in := [fdnSize]float64{1, 2, 3, 4, 5, 6, 7, 8}

	b.ReportAllocs()

	for n := 0; n < b.N; n++ {
		var out [fdnSize]float64
		for i := 0; i < fdnSize; i++ {
			for j := 0; j < fdnSize; j++ {
				out[i] += fdnHadamard[i][j] * in[j]
			}
		}

		in = out
	}
}
