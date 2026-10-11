package chroma

import "testing"

// BenchmarkProcess measures five seconds of a chord at 22.05 kHz with the
// default configuration (252 bins, hop 512).
func BenchmarkProcess(b *testing.B) {
	const sr = 22050

	x := synth(sr, 5, DefaultReferenceHz, event{0, 5, []float64{48, 60, 64, 67}})

	c, err := New(sr)
	if err != nil {
		b.Fatal(err)
	}

	_, err = c.Process(x) // grow the scratch buffers
	if err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.SetBytes(int64(8 * len(x)))

	for b.Loop() {
		_, err = c.Process(x)
		if err != nil {
			b.Fatal(err)
		}
	}
}
