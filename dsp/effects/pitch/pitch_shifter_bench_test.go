package pitch

import "testing"

func BenchmarkPitchShifterProcessInPlace1024(b *testing.B) {
	p, _ := NewPitchShifter(48000)
	_ = p.SetPitchSemitones(7)

	buf := make([]float64, 1024)
	for i := range buf {
		buf[i] = 0.25
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.ProcessInPlace(buf)
	}
}

func BenchmarkPitchShifterProcessInPlace4096(b *testing.B) {
	p, _ := NewPitchShifter(48000)
	_ = p.SetPitchSemitones(-7)

	buf := make([]float64, 4096)
	for i := range buf {
		buf[i] = 0.25
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.ProcessInPlace(buf)
	}
}
