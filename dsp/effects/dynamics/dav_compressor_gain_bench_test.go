package dynamics

import "testing"

var davCompressorGainSink float64

func BenchmarkDAVSoftKneeCompressorGain(b *testing.B) {
	curve, err := NewDAVSoftKneeCompressorGain(-10, 2, 4)
	if err != nil {
		b.Fatal(err)
	}

	peaks := make([]float64, 256)
	for i := range peaks {
		peaks[i] = float64(i+1) / float64(len(peaks))
	}

	b.ReportAllocs()

	var sum float64

	for b.Loop() {
		for _, peak := range peaks {
			sum += curve.Gain(peak)
		}
	}

	davCompressorGainSink = sum
}
