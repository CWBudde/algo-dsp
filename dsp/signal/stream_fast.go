package signal

import "math"

// Dispatch once per block. Silence/noise need no floating-point position, and
// usual positions through 2^53 permit a single int64-to-float conversion.
func (g *StreamGenerator) generateFast32(dst []float32) {
	base, amplitude := float64(g.position), g.cfg.Amplitude
	switch g.cfg.Kind {
	case StreamSilence:
		clear(dst)
	case StreamWhiteNoise:
		for i := range dst {
			dst[i] = float32(amplitude * (g.uniform()*2 - 1))
		}
	case StreamPinkNoise:
		thresholds := [5]float64{0.00198, 0.01478, 0.06378, 0.23378, 0.91578}
		weights := [5]float64{0.23980, 0.18727, 0.16380, 0.194685, 0.214463}

		const weightSum = .23980 + .18727 + .16380 + .194685 + .214463

		for i := range dst {
			u, value := g.uniform(), g.uniform()*2-1
			for band, threshold := range thresholds {
				if u <= threshold {
					g.pink[band] = value * weights[band]
					break
				}
			}

			sum := 0.0
			for _, contribution := range g.pink {
				sum += contribution
			}

			dst[i] = float32(amplitude * (sum / weightSum))
		}
	case StreamSine:
		for i := range dst {
			dst[i] = float32(amplitude * math.Sin(g.step*(base+float64(i))))
		}
	case StreamLinearSweep:
		for i := range dst {
			n := base + float64(i)
			dst[i] = float32(amplitude * math.Sin(g.step*n+math.Pi*g.sweepRate*n*n))
		}
	case StreamLogSweep:
		if g.logK == 0 {
			for i := range dst {
				dst[i] = float32(amplitude * math.Sin(g.step*(base+float64(i))))
			}

			return
		}

		for i := range dst {
			n := base + float64(i)
			if g.logK*n > 500 {
				integral := (math.Exp(g.logStartRate+g.logK*n) - g.cfg.StartHz/g.cfg.SampleRate) / g.logK
				dst[i] = float32(amplitude * math.Sin(2*math.Pi*integral))
			} else {
				dst[i] = float32(amplitude * math.Sin(g.step*math.Expm1(g.logK*n)/g.logK))
			}
		}
	}
}
