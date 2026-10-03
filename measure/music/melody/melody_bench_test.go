package melody

import "testing"

func BenchmarkAnalyze(b *testing.B) {
	x := sequenceSignal(testRate)[:2*int(testRate)]
	onsets := []float64{0.2, 0.55, 0.8, 1.2, 1.7}

	b.ReportAllocs()
	b.SetBytes(int64(len(x) * 8))

	for b.Loop() {
		_, err := Analyze(x, testRate, WithOnsets(onsets))
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSegmentNotes(b *testing.B) {
	res, err := Analyze(sequenceSignal(testRate), testRate)
	if err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()

	for b.Loop() {
		_, err := SegmentNotes(res.Pitch, res.Voicing, res.FrameRate)
		if err != nil {
			b.Fatal(err)
		}
	}
}
