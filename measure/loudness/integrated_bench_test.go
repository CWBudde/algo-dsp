package loudness

import (
	"math"
	"testing"
)

var integratedBenchmarkResult IntegratedResult

func integratedBenchmarkFixture(frames int) [][]float64 {
	planar := make([][]float64, 2)
	for channel := range planar {
		planar[channel] = make([]float64, frames)
		for frame := range planar[channel] {
			phase := float64(frame) / 48000
			planar[channel][frame] = 0.2*math.Sin(2*math.Pi*(1000+float64(channel)*137)*phase) +
				0.025*math.Sin(2*math.Pi*83*phase)
		}
	}

	return planar
}

func integratedBenchmarkFinish(analyzer *IntegratedAnalyzer) (IntegratedResult, error) {
	for {
		done, err := analyzer.FinishStep(256)
		if err != nil {
			return IntegratedResult{}, err
		}

		if done {
			break
		}
	}

	return analyzer.Result()
}

// BenchmarkIntegratedTenMinuteStereo streams 28.8 million frames through a
// reusable 65536-frame fixture. Construction and fixture generation are outside
// the clock; Reset, all processing and bounded finalization are included. The
// same benchmark runs natively and with Go's js/wasm Node runner, without a
// 460.8 MB full source allocation masking analyzer workspace or throughput.
func BenchmarkIntegratedTenMinuteStereo(b *testing.B) {
	const frames = int64(48000 * 600)

	analyzer, err := NewIntegratedAnalyzer(IntegratedConfig{SampleRate: 48000, Channels: 2, MaxFrames: frames})
	if err != nil {
		b.Fatal(err)
	}

	fixture := integratedBenchmarkFixture(MaxIntegratedBlockFrames)
	block := make([][]float64, 2)

	b.SetBytes(frames * 2 * 8)
	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		analyzer.Reset()

		for remaining := frames; remaining > 0; {
			count := int(min(remaining, int64(MaxIntegratedBlockFrames)))
			for channel := range block {
				block[channel] = fixture[channel][:count]
			}

			if err := analyzer.ProcessPlanar(block); err != nil {
				b.Fatal(err)
			}

			remaining -= int64(count)
		}

		integratedBenchmarkResult, err = integratedBenchmarkFinish(analyzer)
		if err != nil {
			b.Fatal(err)
		}

		if integratedBenchmarkResult.Frames != frames {
			b.Fatalf("processed %d frames, want %d", integratedBenchmarkResult.Frames, frames)
		}
	}
}

// BenchmarkIntegrated32TenMinuteStereo exercises the editor's float32 storage
// input directly, using the same bounded streaming workload with no conversion
// buffer in the measured path.
func BenchmarkIntegrated32TenMinuteStereo(b *testing.B) {
	const frames = int64(48000 * 600)

	analyzer, err := NewIntegratedAnalyzer(IntegratedConfig{SampleRate: 48000, Channels: 2, MaxFrames: frames})
	if err != nil {
		b.Fatal(err)
	}

	source := integratedBenchmarkFixture(MaxIntegratedBlockFrames)

	fixture := make([][]float32, 2)
	for channel := range fixture {
		fixture[channel] = make([]float32, len(source[channel]))
		for frame, value := range source[channel] {
			fixture[channel][frame] = float32(value)
		}
	}

	block := make([][]float32, 2)

	b.SetBytes(frames * 2 * 4)
	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		analyzer.Reset()

		for remaining := frames; remaining > 0; {
			count := int(min(remaining, int64(MaxIntegratedBlockFrames)))
			for channel := range block {
				block[channel] = fixture[channel][:count]
			}

			if err := analyzer.ProcessPlanar32(block); err != nil {
				b.Fatal(err)
			}

			remaining -= int64(count)
		}

		integratedBenchmarkResult, err = integratedBenchmarkFinish(analyzer)
		if err != nil {
			b.Fatal(err)
		}

		if integratedBenchmarkResult.Frames != frames {
			b.Fatalf("processed %d frames, want %d", integratedBenchmarkResult.Frames, frames)
		}
	}
}

func BenchmarkIntegratedProcessPlanar(b *testing.B) {
	const maxFrames = int64(48000 * 60)

	analyzer, err := NewIntegratedAnalyzer(IntegratedConfig{SampleRate: 48000, Channels: 2, MaxFrames: maxFrames})
	if err != nil {
		b.Fatal(err)
	}

	block := integratedBenchmarkFixture(1024)
	remaining := maxFrames

	b.SetBytes(1024 * 2 * 8)
	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		if remaining < 1024 {
			analyzer.Reset()

			remaining = maxFrames
		}

		if err := analyzer.ProcessPlanar(block); err != nil {
			b.Fatal(err)
		}

		remaining -= 1024
	}
}

func BenchmarkIntegratedFinishStep(b *testing.B) {
	analyzer, err := NewIntegratedAnalyzer(IntegratedConfig{SampleRate: 48000, Channels: 2, MaxFrames: 48000})
	if err != nil {
		b.Fatal(err)
	}

	input := integratedBenchmarkFixture(48000)
	if err := analyzer.ProcessPlanar(input); err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		// Reuse the immutable captured energies, resetting only finalization.
		// Reprocessing a second of audio outside the timer on every iteration
		// would make benchmark calibration take minutes despite a tiny scan.
		analyzer.finishing, analyzer.finished = false, false
		analyzer.finishIndex, analyzer.relCount, analyzer.relMean = 0, 0, 0

		for {
			done, err := analyzer.FinishStep(1)
			if err != nil {
				b.Fatal(err)
			}

			if done {
				break
			}
		}
	}
}
