package thd

import "testing"

func BenchmarkCalculateFromMagnitude(b *testing.B) {
	sizes := []int{1024, 4096, 16384}
	for _, fftSize := range sizes {
		b.Run("fft_"+itoa(fftSize), func(b *testing.B) {
			cfg := Config{
				SampleRate:      48000,
				FFTSize:         fftSize,
				FundamentalFreq: 1000,
				RangeLowerFreq:  20,
				RangeUpperFreq:  20000,
				CaptureBins:     0,
			}
			mag := make([]float64, fftSize/2+1)

			fundBin := int(cfg.FundamentalFreq * float64(fftSize) / cfg.SampleRate)
			if fundBin > 0 && fundBin < len(mag) {
				mag[fundBin] = 1.0
			}

			for k := 2; k <= 10; k++ {
				bin := k * fundBin
				if bin >= len(mag) {
					break
				}

				amp := 0.01 / float64(k)
				mag[bin] = amp * amp
			}

			calc := NewCalculator(cfg)

			b.ReportAllocs()
			b.ResetTimer()

			for range b.N {
				_ = calc.CalculateFromMagnitude(mag)
			}
		})
	}
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}

	buf := [20]byte{}

	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}

	return string(buf[i:])
}
