package resample

import (
	"fmt"
	"math"
	"testing"
)

func TestResampleInteriorPreservesSumOrder(t *testing.T) {
	t.Parallel()

	for _, ratio := range [][2]int{{1, 1}, {3, 2}, {147, 160}, {160, 147}} {
		for _, taps := range []int{1, 3, 7, 16, 32, 64} {
			t.Run(fmt.Sprintf("%d_%d/taps%d", ratio[0], ratio[1], taps), func(t *testing.T) {
				t.Parallel()

				r := mustResampler(t, ratio[0], ratio[1], WithTapsPerPhase(taps))
				reference := newLegacyStream(r)

				for _, size := range []int{1, 3, 127, 1024, 4096, 17, 8192} {
					input := make([]float64, size)
					for i := range input {
						// Cancellation, denormals and different exponents expose
						// an accidental regrouping of the dot-product additions.
						switch i % 5 {
						case 0:
							input[i] = math.Ldexp(math.Sin(float64(i)), 100)
						case 1:
							input[i] = math.SmallestNonzeroFloat64
						default:
							input[i] = math.Cos(float64(i))
						}
					}

					checkOutputBits(t, r.Process(input), reference.process(input))
				}
			})
		}
	}
}

// BenchmarkTenMinuteStereoDownsample measures the exact 48 kHz -> 44.1 kHz
// profiles used by the editor, including state updates but excluding fixture,
// candidate storage and UI. It streams fixed 65536-frame source blocks and
// never materializes the full source. Each stereo channel has independent state.
func BenchmarkTenMinuteStereoDownsample(b *testing.B) {
	const frames = 48000 * 600

	for _, quality := range []Quality{QualityFast, QualityBalanced, QualityBest} {
		b.Run([]string{"fast", "balanced", "best"}[quality], func(b *testing.B) {
			// The editor preserves prototype support at the destination rate
			// by scaling profile taps by ceil(down/up) for downsampling.
			left, err := NewRational(147, 160, WithQuality(quality), WithTapsPerPhase(QualityProfile(quality).TapsPerPhase*2))
			if err != nil {
				b.Fatal(err)
			}

			right := left.Clone()
			input := sine(1000, 48000, 65536)
			dst := make([]float64, left.PredictOutputLen(len(input)))

			b.ReportAllocs()
			b.SetBytes(frames * 2 * 8)
			b.ResetTimer()

			for range b.N {
				left.Reset()
				right.Reset()

				for offset := 0; offset < frames; offset += len(input) {
					block := input[:min(len(input), frames-offset)]
					if _, err := left.ProcessInto(dst, block); err != nil {
						b.Fatal(err)
					}

					if _, err := right.ProcessInto(dst, block); err != nil {
						b.Fatal(err)
					}
				}
			}
		})
	}
}
