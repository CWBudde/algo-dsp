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

// fdnHadamardInput is the vector both mixing benchmarks transform.
//
// Each iteration starts from it again rather than feeding the previous result
// back in. The transform is unnormalised -- H*H = fdnSize*I -- so a vector run
// through it repeatedly grows by a factor of eight every second pass and
// reaches infinity in a few hundred iterations, well inside a benchmark that
// runs for millions. What is timed after that is arithmetic on Inf and NaN,
// which is not the thing being measured.
//
// The copy is one eight-element array assignment and is identical in both
// benchmarks, so it does not tilt the comparison between them.
var fdnHadamardInput = [fdnSize]float64{1, 2, 3, 4, 5, 6, 7, 8}

// fdnHadamardSink keeps the results reachable, so neither loop can be optimised
// away as dead.
var fdnHadamardSink [fdnSize]float64

func BenchmarkFDNHadamardInPlace(b *testing.B) {
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		v := fdnHadamardInput
		hadamardInPlace(&v)
		fdnHadamardSink = v
	}
}

func BenchmarkFDNHadamardMatrix(b *testing.B) {
	b.ReportAllocs()

	for n := 0; n < b.N; n++ {
		in := fdnHadamardInput

		var out [fdnSize]float64

		for i := 0; i < fdnSize; i++ {
			for j := 0; j < fdnSize; j++ {
				out[i] += fdnHadamard[i][j] * in[j]
			}
		}

		fdnHadamardSink = out
	}
}
