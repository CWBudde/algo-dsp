package rhythm_test

import (
	"testing"

	"github.com/cwbudde/algo-dsp/measure/music/rhythm"
)

func BenchmarkEstimateTempo(b *testing.B) {
	x := rhythm.Novelty(pulseTrack().Flux, rhythm.DefaultNoveltyRadius)

	b.ReportAllocs()

	for b.Loop() {
		_, err := rhythm.EstimateTempo(x, timing, rhythm.WithPrior(120))
		if err != nil {
			b.Fatal(err)
		}
	}
}
