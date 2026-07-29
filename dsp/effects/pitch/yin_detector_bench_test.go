package pitch

import (
	"fmt"
	"testing"
)

// benchYINDetect measures one full analysis frame. The lower frequency bound
// is the dominant cost driver: it sets the longest lag, and the direct
// difference function is quadratic in that lag.
func benchYINDetect(b *testing.B, minHz float64) {
	b.Helper()

	d, err := NewYINDetector(yinTestSampleRate, WithYINFrequencyRange(minHz, 1600))
	if err != nil {
		b.Fatalf("NewYINDetector: %v", err)
	}

	frame := yinSawtooth(220, yinTestSampleRate, 0.5, d.FrameSize())

	b.ReportAllocs()
	b.SetBytes(int64(len(frame) * 8))
	b.ResetTimer()

	for range b.N {
		if _, err := d.Detect(frame); err != nil {
			b.Fatalf("Detect: %v", err)
		}
	}
}

func BenchmarkYINDetectorDetect(b *testing.B) {
	for _, minHz := range []float64{60, 80, 200} {
		b.Run(fmt.Sprintf("minHz=%g", minHz), func(b *testing.B) {
			benchYINDetect(b, minHz)
		})
	}
}

func BenchmarkPitchTrackerWrite(b *testing.B) {
	t, err := NewPitchTracker(yinTestSampleRate)
	if err != nil {
		b.Fatalf("NewPitchTracker: %v", err)
	}

	block := yinSawtooth(220, yinTestSampleRate, 0.5, 256)

	b.ReportAllocs()
	b.SetBytes(int64(len(block) * 8))
	b.ResetTimer()

	for range b.N {
		t.Write(block)
	}
}
