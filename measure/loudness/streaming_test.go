package loudness

import (
	"errors"
	"math"
	"testing"
)

func TestStreamingMeterStereoCalibrationAndLRA(t *testing.T) {
	const rate = 8000

	m, err := NewStreamingMeter(IntegratedConfig{SampleRate: rate, Channels: 2, MaxFrames: 40 * rate})
	if err != nil {
		t.Fatal(err)
	}

	block := make([]float32, 256)

	for offset := 0; offset < 40*rate; offset += 128 {
		level := -20.0
		if offset >= 20*rate {
			level = -30
		}

		amplitude := math.Pow(10, level/20)
		for i := 0; i < 128; i++ {
			x := float32(amplitude * math.Sin(2*math.Pi*1000*float64(offset+i)/rate))
			block[2*i], block[2*i+1] = x, x
		}

		if err := m.ProcessInterleaved32(block); err != nil {
			t.Fatal(err)
		}
	}

	r := m.Snapshot()
	if !r.HasMomentary || !r.HasShortTerm || !r.HasIntegrated || !r.HasLRA || r.LRAStable {
		t.Fatal(r)
	}
	// EBU Tech3342 case1 expects ten LU between these two twenty-second tones.
	if math.Abs(r.LRA-10) > 1 {
		t.Fatalf("LRA%g", r.LRA)
	}

	if math.Abs(r.MaxMomentary-r.MaxShortTerm) > .02 {
		t.Fatalf("M/S constant tone disagree:%+v", r)
	}

	reading := m.Reading()
	if reading != r {
		t.Fatalf("cached reading%+v want%+v", reading, r)
	}

	m.Reset()

	r = m.Reading()
	if r.Frames != 0 || r.HasLRA || r.HasIntegrated {
		t.Fatal(r)
	}
}

func TestStreamingMeterPartitionFormatsAndIntegratedReference(t *testing.T) {
	const frames = 64000

	src := make([]float32, frames)
	x := make([]float64, frames)

	for i := range src {
		src[i] = float32(.08 * math.Sin(2*math.Pi*1000*float64(i)/8000))
		x[i] = float64(src[i])
	}

	config := IntegratedConfig{SampleRate: 8000, Channels: 1, MaxFrames: frames}

	reference, _ := NewIntegratedAnalyzer(config)
	if err := reference.ProcessPlanar([][]float64{x}); err != nil {
		t.Fatal(err)
	}

	for {
		done, err := reference.FinishStep(4096)
		if err != nil {
			t.Fatal(err)
		}

		if done {
			break
		}
	}

	want, err := reference.Result()
	if err != nil {
		t.Fatal(err)
	}

	var baseline StreamingSnapshot

	for _, partition := range []int{1, 17, 128, 65536} {
		for mode := 0; mode < 3; mode++ {
			m, _ := NewStreamingMeter(config)

			for start := 0; start < frames; start += partition {
				end := min(start+partition, frames)

				var err error

				switch mode {
				case 0:
					err = m.ProcessPlanar([][]float64{x[start:end]})
				case 1:
					err = m.ProcessPlanar32([][]float32{src[start:end]})
				case 2:
					err = m.ProcessInterleaved32(src[start:end])
				}

				if err != nil {
					t.Fatal(err)
				}
			}

			got := m.Snapshot()
			if math.Abs(got.Integrated-want.LUFS) > 1e-10 {
				t.Fatalf("I%g reference%g", got.Integrated, want.LUFS)
			}

			if partition == 1 && mode == 0 {
				baseline = got
			} else if got != baseline {
				t.Fatalf("partition%d mode%d: %+v !=%+v", partition, mode, got, baseline)
			}
		}
	}
}

func TestStreamingMeterAtomicValidationAvailabilityAndAllocation(t *testing.T) {
	config := IntegratedConfig{SampleRate: 8000, Channels: 2, MaxFrames: 8000}

	m, _ := NewStreamingMeter(config)
	if err := m.ProcessPlanar([][]float64{{.1}, {.2}}); err != nil {
		t.Fatal(err)
	}

	for _, block := range [][][]float64{{{1}}, {{1}, {1, 2}}, {{1}, {math.NaN()}}, {{1}, {1e200}}} {
		if err := m.ProcessPlanar(block); err == nil {
			t.Fatal("invalid block accepted")
		}

		if m.frames != 1 {
			t.Fatal("invalid input changed state")
		}
	}

	if err := m.ProcessInterleaved32([]float32{1}); err == nil {
		t.Fatal("partial frame accepted")
	}

	if err := m.ProcessPlanar([][]float64{make([]float64, 8000), make([]float64, 8000)}); !errors.Is(err, ErrLimit) {
		t.Fatal(err)
	}

	for _, bad := range []IntegratedConfig{{SampleRate: 0, Channels: 1, MaxFrames: 1}, {SampleRate: 8000, Channels: 0, MaxFrames: 1}, {SampleRate: 8000, Channels: 1, MaxFrames: 1 << 60}, {SampleRate: 8000, Channels: 1, MaxFrames: 1, ChannelWeights: []float64{0}}} {
		if _, err := NewStreamingMeter(bad); err == nil {
			t.Fatal("invalid config accepted")
		}
	}

	block := make([]float32, 256)

	if allocations := testing.AllocsPerRun(3, func() {
		m.Reset()

		for range 60 {
			if err := m.ProcessInterleaved32(block); err != nil {
				panic(err)
			}
		}

		_ = m.Snapshot()
		_ = m.Reading()
	}); allocations != 0 {
		t.Fatalf("allocations%g", allocations)
	}

	r := m.Snapshot()
	if !r.HasMomentary || r.HasShortTerm || r.HasIntegrated || r.HasLRA {
		t.Fatal(r)
	}
}

func BenchmarkStreamingMeterStereo128(b *testing.B) {
	m, _ := NewStreamingMeter(IntegratedConfig{SampleRate: 48000, Channels: 2, MaxFrames: 48000 * 3600})

	block := make([]float32, 256)
	for i := range block {
		block[i] = float32(.1 * math.Sin(float64(i)*.2))
	}

	b.ReportAllocs()
	b.SetBytes(1024)
	b.ResetTimer()

	for range b.N {
		if m.frames+128 > m.maxFrames {
			m.Reset()
		}

		if err := m.ProcessInterleaved32(block); err != nil {
			b.Fatal(err)
		}
	}
}
