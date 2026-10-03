package onset_test

import (
	"testing"

	"github.com/cwbudde/algo-dsp/measure/music/features"
	"github.com/cwbudde/algo-dsp/measure/music/onset"
)

func BenchmarkDetect(b *testing.B) {
	channels := musicFixture(4)

	f, err := features.Extract(channels, features.DefaultConfig())
	if err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()

	for b.Loop() {
		_, err := onset.Detect(channels, f)
		if err != nil {
			b.Fatal(err)
		}
	}
}
