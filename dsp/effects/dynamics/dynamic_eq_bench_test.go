package dynamics

import (
	"fmt"
	"testing"
)

func benchDynamicEQ(b *testing.B, updateInterval int) {
	b.Helper()

	eq, err := NewDynamicEQWithConfig(eqTestSampleRate, []EQBandConfig{
		{FrequencyHz: 120, Type: EQBandLowShelf, Mode: EQBandModeUpwardBelow, ThresholdDB: -30, Ratio: 2},
		{FrequencyHz: 2500, Q: 2, Mode: EQBandModeDownward, ThresholdDB: -24, Ratio: 4},
		{FrequencyHz: 8000, Type: EQBandHighShelf, Mode: EQBandModeDownward, ThresholdDB: -24, Ratio: 3},
	})
	if err != nil {
		b.Fatalf("NewDynamicEQWithConfig: %v", err)
	}

	if err := eq.SetUpdateInterval(updateInterval); err != nil {
		b.Fatalf("SetUpdateInterval: %v", err)
	}

	buf := sineBuffer(1000, 0.5, 512, eqTestSampleRate)

	b.ReportAllocs()
	b.SetBytes(int64(len(buf) * 8))
	b.ResetTimer()

	for range b.N {
		eq.ProcessInPlace(buf)
	}
}

// BenchmarkDynamicEQProcessInPlace measures the three-band hot path at the
// default control rate.
func BenchmarkDynamicEQProcessInPlace(b *testing.B) {
	benchDynamicEQ(b, defaultEQUpdateInterval)
}

// BenchmarkDynamicEQUpdateInterval documents the cost of the coefficient
// redesign rate: interval 1 redesigns every band on every sample.
func BenchmarkDynamicEQUpdateInterval(b *testing.B) {
	for _, interval := range []int{1, 8, 32, 128} {
		b.Run(fmt.Sprintf("interval=%d", interval), func(b *testing.B) {
			benchDynamicEQ(b, interval)
		})
	}
}

func BenchmarkDynamicEQProcessSample(b *testing.B) {
	eq, err := NewDynamicEQWithConfig(eqTestSampleRate, []EQBandConfig{
		{FrequencyHz: 2500, Q: 2, Mode: EQBandModeDownward, ThresholdDB: -24, Ratio: 4},
	})
	if err != nil {
		b.Fatalf("NewDynamicEQWithConfig: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()

	var sink float64
	for i := range b.N {
		sink = eq.ProcessSample(float64(i%2)*0.5 - 0.25)
	}

	_ = sink
}
