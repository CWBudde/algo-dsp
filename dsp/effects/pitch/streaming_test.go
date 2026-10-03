package pitch

import (
	"fmt"
	"math"
	"testing"
)

type sampleStream interface {
	ProcessInPlace([]float64) error
	Reset()
	Latency() int
}

func newTestStream(t *testing.T, kind string, semitones float64) sampleStream {
	t.Helper()

	if kind == "time" {
		fx, err := NewPitchShifter(48000)
		if err != nil {
			t.Fatal(err)
		}

		if err = fx.SetPitchSemitones(semitones); err != nil {
			t.Fatal(err)
		}

		stream, err := NewStreamingPitchShifter(fx)
		if err != nil {
			t.Fatal(err)
		}

		return stream
	}

	fx, err := NewSpectralPitchShifter(48000)
	if err != nil {
		t.Fatal(err)
	}

	if err = fx.SetPitchSemitones(semitones); err != nil {
		t.Fatal(err)
	}

	stream, err := NewStreamingSpectralPitchShifter(fx)
	if err != nil {
		t.Fatal(err)
	}

	return stream
}

func toneProjection(samples []float64, frequency float64) float64 {
	step := 2 * math.Pi * frequency / 48000
	cs, sn := math.Cos(step), math.Sin(step)
	real, imag, c, s := 0.0, 0.0, 1.0, 0.0

	for i, x := range samples {
		window := 0.5 - 0.5*math.Cos(2*math.Pi*float64(i)/float64(len(samples)))
		real += x * window * c
		imag -= x * window * s
		c, s = c*cs-s*sn, s*cs+c*sn
	}

	return math.Hypot(real, imag)
}

func TestStreamingPitchIndependentFrequencyAndLatency(t *testing.T) {
	for _, kind := range []string{"time", "spectral"} {
		for _, semitones := range []float64{-12, 12} {
			t.Run(fmt.Sprintf("%s/%g", kind, semitones), func(t *testing.T) {
				stream := newTestStream(t, kind, semitones)
				// Three seconds crosses the bounded input and synthesis histories;
				// inspect the late output so startup-only correctness cannot pass.
				const frames = 144000

				input := make([]float64, frames+stream.Latency()+4096)
				for i := 0; i < frames; i++ {
					input[i] = 0.3 * math.Sin(2*math.Pi*440*float64(i)/48000)
				}

				if err := stream.ProcessInPlace(input); err != nil {
					t.Fatal(err)
				}

				for _, x := range input[:stream.Latency()] {
					if x != 0 {
						t.Fatal("nonzero startup before declared latency")
					}
				}

				for i, x := range input {
					if math.IsNaN(x) || math.IsInf(x, 0) {
						t.Fatalf("non-finite sample at %d", i)
					}
				}

				probe := input[stream.Latency()+frames-32768 : stream.Latency()+frames-8192]
				expected := 440 * math.Exp2(semitones/12)
				peak, peakFrequency := 0.0, 0.0

				for hz := expected - 4; hz <= expected+4; hz += 0.25 {
					magnitude := toneProjection(probe, hz)
					if magnitude > peak {
						peak = magnitude
						peakFrequency = hz
					}
				}

				if math.Abs(peakFrequency-expected) > 0.5 {
					t.Errorf("dominant frequency%g want%g", peakFrequency, expected)
				}

				if peak < 50 || peak < 5*toneProjection(probe, 440) {
					t.Errorf("shifted tone too weak target%g source%g", peak, toneProjection(probe, 440))
				}
			})
		}
	}
}

func TestStreamingPitchSteadyStateAllocationsAfterHistoryWrap(t *testing.T) {
	for _, kind := range []string{"time", "spectral"} {
		t.Run(kind, func(t *testing.T) {
			stream := newTestStream(t, kind, 12)
			block := make([]float64, 128)
			render := func() {
				for i := range block {
					block[i] = 0.25
				}

				if err := stream.ProcessInPlace(block); err != nil {
					panic(err)
				}
			}
			// Reach real FFT/overlap-search work and wrap every bounded history
			// before measuring, without resetting between measured renders.
			for range 1024 {
				render()
			}

			if allocations := testing.AllocsPerRun(3, func() {
				for range 64 {
					render()
				}
			}); allocations != 0 {
				t.Errorf("steady-state render allocs=%g", allocations)
			}
		})
	}
}

func TestStreamingPitchPartitionResetIdentityShortAndAllocations(t *testing.T) {
	for _, kind := range []string{"time", "spectral"} {
		for _, semi := range []float64{0, -24, -7, 7, 24} {
			t.Run(fmt.Sprintf("%s/%g", kind, semi), func(t *testing.T) {
				stream := newTestStream(t, kind, semi)
				n := stream.Latency() + 12001

				source := make([]float64, n)
				for i := range source {
					source[i] = 0.1*math.Sin(float64(i)*0.053) + 0.05*math.Cos(float64(i)*0.011)
				}

				whole := append([]float64(nil), source...)
				if err := stream.ProcessInPlace(whole); err != nil {
					t.Fatal(err)
				}

				if semi == 0 {
					for i, x := range source {
						if whole[i] != x {
							t.Fatal("identity changed sample")
						}
					}
				}

				for _, sizes := range [][]int{{1}, {128}, {17, 512, 3, 129}} {
					stream.Reset()

					parts := append([]float64(nil), source...)

					for pos, it := 0, 0; pos < n; it++ {
						end := min(pos+sizes[it%len(sizes)], n)
						if err := stream.ProcessInPlace(parts[pos:end]); err != nil {
							t.Fatal(err)
						}

						pos = end
					}

					for i, want := range whole {
						if parts[i] != want {
							t.Fatalf("partition%v sample%d", sizes, i)
						}
					}
				}

				stream.Reset()

				short := make([]float64, stream.Latency()+512+2048)
				for i := 0; i < 512; i++ {
					short[i] = 0.3 * math.Sin(2*math.Pi*440*float64(i)/48000)
				}

				if err := stream.ProcessInPlace(short); err != nil {
					t.Fatal(err)
				}

				energy := 0.0
				for _, x := range short[stream.Latency() : stream.Latency()+512] {
					energy += x * x
				}

				if energy < 1e-6 {
					t.Error("latency-compensated short selection became silence")
				}

				block := make([]float64, 128)

				if allocations := testing.AllocsPerRun(3, func() {
					stream.Reset()

					for range 256 {
						for i := range block {
							block[i] = 0.05
						}

						if err := stream.ProcessInPlace(block); err != nil {
							panic(err)
						}
					}
				}); allocations != 0 {
					t.Errorf("stream reset/render allocs=%g", allocations)
				}
			})
		}
	}
}

func TestStreamingPitchInvalidConstructors(t *testing.T) {
	if _, err := NewStreamingPitchShifter(nil); err == nil {
		t.Error("nil time processor accepted")
	}

	if _, err := NewStreamingPitchShifter(&PitchShifter{}); err == nil {
		t.Error("invalid time processor accepted")
	}

	if _, err := NewStreamingSpectralPitchShifter(nil); err == nil {
		t.Error("nil spectral processor accepted")
	}

	if _, err := NewStreamingSpectralPitchShifter(&SpectralPitchShifter{}); err == nil {
		t.Error("invalid spectral processor accepted")
	}
}

func BenchmarkStreamingPitch(b *testing.B) {
	for _, kind := range []string{"time", "spectral"} {
		b.Run(kind, func(b *testing.B) {
			var stream sampleStream

			if kind == "time" {
				fx, _ := NewPitchShifter(48000)
				_ = fx.SetPitchSemitones(7)
				stream, _ = NewStreamingPitchShifter(fx)
			} else {
				fx, _ := NewSpectralPitchShifter(48000)
				_ = fx.SetPitchSemitones(7)
				stream, _ = NewStreamingSpectralPitchShifter(fx)
			}

			block := make([]float64, 128)
			for range 256 {
				_ = stream.ProcessInPlace(block)
			}

			b.ReportAllocs()
			b.ResetTimer()

			for range b.N {
				_ = stream.ProcessInPlace(block)
			}
		})
	}
}
