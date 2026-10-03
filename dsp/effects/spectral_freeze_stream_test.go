package effects

import (
	"fmt"
	"math"
	"testing"
)

func TestStreamingFreezePartitionResetAndUnfrozenReconstruction(t *testing.T) {
	for _, frozen := range []bool{false, true} {
		fx, err := NewSpectralFreeze(48000)
		if err != nil {
			t.Fatal(err)
		}

		fx.SetFrozen(frozen)

		stream, err := NewStreamingSpectralFreeze(fx)
		if err != nil {
			t.Fatal(err)
		}

		const frames = 12001

		source := make([]float64, frames+stream.Latency()+2048)
		for i := 0; i < frames; i++ {
			source[i] = 0.3*math.Sin(float64(i)*0.07) + 0.03*math.Cos(float64(i)*0.137)
		}

		whole := append([]float64(nil), source...)
		if err := stream.ProcessInPlace(whole); err != nil {
			t.Fatal(err)
		}

		if !frozen {
			for i := 1; i < frames; i++ {
				if math.Abs(whole[i+stream.Latency()]-source[i]) > 1e-9 {
					t.Fatalf("reconstruction sample%d got%.12g want%.12g", i, whole[i+stream.Latency()], source[i])
				}
			}
		}

		for _, sizes := range [][]int{{1}, {128}, {512}, {17, 3, 129}} {
			stream.Reset()

			parts := append([]float64(nil), source...)
			for pos, it := 0, 0; pos < len(parts); it++ {
				end := min(pos+sizes[it%len(sizes)], len(parts))
				if err := stream.ProcessInPlace(parts[pos:end]); err != nil {
					t.Fatal(err)
				}

				pos = end
			}

			for i, want := range whole {
				if parts[i] != want {
					t.Fatalf("frozen%v partition%v sample%d", frozen, sizes, i)
				}
			}
		}

		if frozen {
			stream.Reset()

			changed := append([]float64(nil), source...)
			for i := stream.Latency(); i < len(changed); i++ {
				changed[i] = 0.2 * math.Cos(float64(i)*0.431)
			}

			if err := stream.ProcessInPlace(changed); err != nil {
				t.Fatal(err)
			}

			for i, want := range whole {
				if changed[i] != want {
					t.Fatalf("frozen spectrum changed with later input at%d", i)
				}
			}
		}

		block := make([]float64, 128)

		if allocations := testing.AllocsPerRun(3, func() {
			stream.Reset()

			for range 256 {
				for i := range block {
					block[i] = 0.1
				}

				if err := stream.ProcessInPlace(block); err != nil {
					panic(err)
				}
			}
		}); allocations != 0 {
			t.Errorf("reset/render allocs=%g", allocations)
		}
	}
}

func TestStreamingFreezeLongToneMixAndResetCapture(t *testing.T) {
	for _, phase := range []SpectralFreezePhaseMode{SpectralFreezePhaseHold, SpectralFreezePhaseAdvance} {
		t.Run(fmt.Sprint(phase), func(t *testing.T) {
			newStream := func(mix float64) *StreamingSpectralFreeze {
				t.Helper()

				fx, err := NewSpectralFreeze(48000)
				if err != nil {
					t.Fatal(err)
				}

				fx.SetFrozen(true)

				if err := fx.SetPhaseMode(phase); err != nil {
					t.Fatal(err)
				}

				if err := fx.SetMix(mix); err != nil {
					t.Fatal(err)
				}

				stream, err := NewStreamingSpectralFreeze(fx)
				if err != nil {
					t.Fatal(err)
				}

				return stream
			}
			stream := newStream(1)

			const frames = 48000

			source := make([]float64, frames+stream.Latency())
			for i := range source[:frames] {
				frequency := 750.0
				if i >= 4096 {
					frequency = 1500
				}

				source[i] = 0.3 * math.Sin(2*math.Pi*frequency*float64(i)/48000)
			}

			wet := append([]float64(nil), source...)
			if err := stream.ProcessInPlace(wet); err != nil {
				t.Fatal(err)
			}
			// The held first-frame tone must survive repeated ring wraps and
			// reject the later input tone. These frequencies are exact FFT bins.
			probe := wet[stream.Latency()+24000 : stream.Latency()+frames]

			captured, changed := freezeToneMagnitude(probe, 750), freezeToneMagnitude(probe, 1500)
			if captured < 100 || captured < 100*changed {
				t.Errorf("held tone magnitude %g, later input magnitude %g", captured, changed)
			}

			for _, mix := range []float64{0, 0.25, 0.75} {
				mixedStream := newStream(mix)

				mixed := append([]float64(nil), source...)
				for pos := 0; pos < len(mixed); {
					end := min(pos+137, len(mixed))
					if err := mixedStream.ProcessInPlace(mixed[pos:end]); err != nil {
						t.Fatal(err)
					}

					pos = end
				}

				for i, actual := range mixed {
					dry := 0.0
					if i >= stream.Latency() {
						dry = source[i-stream.Latency()]
					}

					want := dry*(1-mix) + wet[i]*mix
					if math.IsNaN(actual) || math.IsInf(actual, 0) || math.Abs(actual-want) > 1e-12 {
						t.Fatalf("mix %g sample %d: got %g want %g", mix, i, actual, want)
					}
				}
			}

			stream.Reset()

			fresh := make([]float64, len(source))
			for i := range fresh[:frames] {
				fresh[i] = 0.3 * math.Sin(2*math.Pi*1500*float64(i)/48000)
			}

			if err := stream.ProcessInPlace(fresh); err != nil {
				t.Fatal(err)
			}

			probe = fresh[stream.Latency()+24000 : stream.Latency()+frames]
			if next, old := freezeToneMagnitude(probe, 1500), freezeToneMagnitude(probe, 750); next < 100 || next < 100*old {
				t.Errorf("reset retained stale capture: new %g old %g", next, old)
			}

			block := make([]float64, 128)

			if allocations := testing.AllocsPerRun(3, func() {
				for range 64 {
					if err := stream.ProcessInPlace(block); err != nil {
						panic(err)
					}
				}
			}); allocations != 0 {
				t.Errorf("steady-state frozen render allocs=%g", allocations)
			}
		})
	}
}

func freezeToneMagnitude(samples []float64, frequency float64) float64 {
	realPart, imaginaryPart := 0.0, 0.0

	for i, sample := range samples {
		phase := 2 * math.Pi * frequency * float64(i) / 48000
		realPart += sample * math.Cos(phase)
		imaginaryPart -= sample * math.Sin(phase)
	}

	return math.Hypot(realPart, imaginaryPart)
}

func TestStreamingFreezeInvalidConstructors(t *testing.T) {
	if _, err := NewStreamingSpectralFreeze(nil); err == nil {
		t.Error("nil accepted")
	}

	if _, err := NewStreamingSpectralFreeze(&SpectralFreeze{}); err == nil {
		t.Error("invalid accepted")
	}
}
