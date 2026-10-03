package loudness

import "testing"

var (
	targetBenchmarkResult            TargetResult
	targetMeasurementBenchmarkResult IntegratedResult
)

// BenchmarkTarget32TenMinuteStereo streams the same varied 65536-frame fixture
// as the integrated analyzer benchmark. It includes Reset, complete analysis,
// incremental sorting and exact-target planning, with all construction and
// fixture generation outside the clock. This same test runs under native or
// Go's WASM runner, and never materializes a full ten-minute input array.
func BenchmarkTarget32TenMinuteStereo(b *testing.B) {
	benchmarkTarget32TenMinuteStereo(b, false)
}

// BenchmarkTargetMeasurement32TenMinuteStereo exercises actual candidate
// measurement with the same bounded fixture and optimized scan, without the
// target solver's sorting. Setup is untimed; Reset/process/finish are timed.
func BenchmarkTargetMeasurement32TenMinuteStereo(b *testing.B) {
	benchmarkTarget32TenMinuteStereo(b, true)
}

func benchmarkTarget32TenMinuteStereo(b *testing.B, measurementOnly bool) {
	b.Helper()

	const frames = int64(48000 * 600)

	a, err := NewTargetAnalyzer(IntegratedConfig{SampleRate: 48000, Channels: 2, MaxFrames: frames}, -23)
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
		a.Reset()

		for remaining := frames; remaining > 0; {
			count := int(min(remaining, int64(MaxIntegratedBlockFrames)))
			for channel := range block {
				block[channel] = fixture[channel][:count]
			}

			if err := a.ProcessPlanar32(block); err != nil {
				b.Fatal(err)
			}

			remaining -= int64(count)
		}

		for {
			var done bool
			if measurementOnly {
				done, err = a.FinishMeasurementStep(4096)
			} else {
				done, err = a.FinishStep(4096)
			}

			if err != nil {
				b.Fatal(err)
			}

			if done {
				break
			}
		}

		if measurementOnly {
			targetMeasurementBenchmarkResult, err = a.MeasurementResult()
			if err != nil || targetMeasurementBenchmarkResult.Frames != frames {
				b.Fatalf("incorrect measured result %+v error=%v", targetMeasurementBenchmarkResult, err)
			}
		} else {
			targetBenchmarkResult, err = a.Result()
			if err != nil || targetBenchmarkResult.Frames != frames {
				b.Fatalf("incorrect target result %+v error=%v", targetBenchmarkResult, err)
			}
		}
	}
}

func BenchmarkTargetMeasurementFinishStep(b *testing.B) {
	a, err := NewTargetAnalyzer(IntegratedConfig{SampleRate: 48000, Channels: 2, MaxFrames: 48000}, -23)
	if err != nil {
		b.Fatal(err)
	}

	if err := a.ProcessPlanar(integratedBenchmarkFixture(48000)); err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		// Reuse immutable source energies; repeated untimed source rescans can
		// otherwise make calibration take minutes despite a tiny gate scan.
		a.phase = targetInput
		for {
			done, err := a.FinishMeasurementStep(1)
			if err != nil {
				b.Fatal(err)
			}

			if done {
				break
			}
		}
	}
}

func BenchmarkTargetProcessPlanar32(b *testing.B) {
	const maxFrames = int64(48000 * 60)

	a, err := NewTargetAnalyzer(IntegratedConfig{SampleRate: 48000, Channels: 2, MaxFrames: maxFrames}, -23)
	if err != nil {
		b.Fatal(err)
	}

	source := integratedBenchmarkFixture(1024)

	block := make([][]float32, 2)
	for channel := range block {
		block[channel] = make([]float32, 1024)
		for frame, value := range source[channel] {
			block[channel][frame] = float32(value)
		}
	}

	remaining := maxFrames

	b.SetBytes(1024 * 2 * 4)
	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		if remaining < 1024 {
			a.Reset()

			remaining = maxFrames
		}

		if err := a.ProcessPlanar32(block); err != nil {
			b.Fatal(err)
		}

		remaining -= 1024
	}
}
