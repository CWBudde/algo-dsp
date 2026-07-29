package pitch

import "testing"

// benchPitchCorrector measures a full correction pass over one second of
// harmonically rich material. Unlike the detector and tracker this path
// allocates: PitchProcessor.Process returns a new slice by contract.
func benchPitchCorrector(b *testing.B, shifter PitchProcessor, blockSize int) {
	b.Helper()

	c, err := NewPitchCorrector(yinTestSampleRate,
		WithCorrectionShifter(shifter),
		WithCorrectionBlockSize(blockSize))
	if err != nil {
		b.Fatalf("NewPitchCorrector: %v", err)
	}

	buf := yinSawtooth(452, yinTestSampleRate, 0.4, int(yinTestSampleRate))

	b.ReportAllocs()
	b.SetBytes(int64(len(buf) * 8))
	b.ResetTimer()

	for range b.N {
		c.ProcessInPlace(buf)
	}
}

func BenchmarkPitchCorrectorSpectral(b *testing.B) {
	shifter, err := NewSpectralPitchShifter(yinTestSampleRate)
	if err != nil {
		b.Fatalf("NewSpectralPitchShifter: %v", err)
	}

	benchPitchCorrector(b, shifter, defaultCorrectionBlockSize)
}

func BenchmarkPitchCorrectorWSOLA(b *testing.B) {
	shifter, err := NewPitchShifter(yinTestSampleRate)
	if err != nil {
		b.Fatalf("NewPitchShifter: %v", err)
	}

	if err := shifter.SetSequence(30); err != nil {
		b.Fatalf("SetSequence: %v", err)
	}

	benchPitchCorrector(b, shifter, 8192)
}
