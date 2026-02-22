package webdemo

import (
	"fmt"
	"math"
	"math/cmplx"
	"strings"

	"github.com/cwbudde/algo-dsp/dsp/window"
	algofft "github.com/cwbudde/algo-fft"
)

// SetSpectrum updates analyzer settings used for the EQ graph spectrum.
func (e *Engine) SetSpectrum(p SpectrumParams) error {
	cfg := sanitizeSpectrumParams(p)

	winType, err := spectrumWindowType(cfg.Window)
	if err != nil {
		return err
	}

	win := window.Generate(winType, cfg.FFTSize, window.WithPeriodic())
	if len(win) != cfg.FFTSize {
		return fmt.Errorf("invalid analyzer window size: %d", cfg.FFTSize)
	}

	sum := 0.0
	for _, w := range win {
		sum += w
	}

	plan, err := algofft.NewPlan64(cfg.FFTSize)
	if err != nil {
		return fmt.Errorf("spectrum init fft plan: %w", err)
	}

	hop := int(math.Round(float64(cfg.FFTSize) * (1 - cfg.Overlap)))
	if hop < 1 {
		hop = 1
	}

	e.spectrum = cfg
	e.spectrumWindow = win
	e.spectrumWindowGain = sum / float64(cfg.FFTSize)
	e.spectrumPlan = plan
	e.spectrumFFTSize = cfg.FFTSize
	e.spectrumHopSize = hop
	e.spectrumInput = make([]complex128, cfg.FFTSize)
	e.spectrumOutput = make([]complex128, cfg.FFTSize)
	e.spectrumRing = make([]float64, cfg.FFTSize)
	e.spectrumWrite = 0
	e.spectrumFilled = 0
	e.spectrumSamplesToHop = 0

	e.spectrumDB = make([]float64, cfg.FFTSize/2+1)
	for i := range e.spectrumDB {
		e.spectrumDB[i] = -130
	}

	e.spectrumReady = false

	return nil
}

// SpectrumCurveDB returns a smoothed real-time spectrum in dBFS for freqs.
func (e *Engine) SpectrumCurveDB(freqs []float64) []float64 {
	out := make([]float64, len(freqs))
	if !e.spectrumReady || len(e.spectrumDB) == 0 {
		for i := range out {
			out[i] = -130
		}

		return out
	}

	nyquist := e.sampleRate * 0.5

	lastBin := len(e.spectrumDB) - 1
	if lastBin < 1 {
		for i := range out {
			out[i] = -130
		}

		return out
	}

	binHz := e.sampleRate / float64(e.spectrumFFTSize)

	for i, f := range freqs {
		f = clamp(f, 0, nyquist)

		bin := f / binHz
		if bin <= 0 {
			out[i] = e.spectrumDB[0]
			continue
		}

		if bin >= float64(lastBin) {
			out[i] = e.spectrumDB[lastBin]
			continue
		}

		base := int(bin)
		frac := bin - float64(base)
		d0 := e.spectrumDB[base]
		d1 := e.spectrumDB[base+1]
		out[i] = d0 + frac*(d1-d0)
	}

	return out
}

func (e *Engine) initSpectrumAnalyzer() error {
	return e.SetSpectrum(e.spectrum)
}

func (e *Engine) pushSpectrumSample(x float64) {
	e.spectrumRing[e.spectrumWrite] = x

	e.spectrumWrite++
	if e.spectrumWrite >= e.spectrumFFTSize {
		e.spectrumWrite = 0
	}

	if e.spectrumFilled < e.spectrumFFTSize {
		e.spectrumFilled++
	}

	e.spectrumSamplesToHop++
	if e.spectrumFilled < e.spectrumFFTSize || e.spectrumSamplesToHop < e.spectrumHopSize {
		return
	}

	e.spectrumSamplesToHop = 0
	e.updateSpectrumFrame()
}

func (e *Engine) updateSpectrumFrame() {
	const (
		minDB = -130.0
		eps   = 1e-12
	)

	read := e.spectrumWrite
	for i := 0; i < e.spectrumFFTSize; i++ {
		s := e.spectrumRing[read]
		e.spectrumInput[i] = complex(s*e.spectrumWindow[i], 0)

		read++
		if read >= e.spectrumFFTSize {
			read = 0
		}
	}

	if err := e.spectrumPlan.Forward(e.spectrumOutput, e.spectrumInput); err != nil {
		return
	}

	norm := float64(e.spectrumFFTSize) * math.Max(e.spectrumWindowGain, eps)

	last := len(e.spectrumDB) - 1
	for k := 0; k <= last; k++ {
		mag := cmplx.Abs(e.spectrumOutput[k]) / norm
		if k > 0 && k < last {
			mag *= 2
		}

		valDB := 20 * math.Log10(math.Max(eps, mag))
		if valDB < minDB {
			valDB = minDB
		}

		if !e.spectrumReady {
			e.spectrumDB[k] = valDB
			continue
		}

		smooth := e.spectrum.Smoothing
		e.spectrumDB[k] = smooth*e.spectrumDB[k] + (1-smooth)*valDB
	}

	e.spectrumReady = true
}

func sanitizeSpectrumParams(p SpectrumParams) SpectrumParams {
	cfg := p
	switch cfg.FFTSize {
	case 256, 512, 1024, 2048, 4096, 8192:
	default:
		cfg.FFTSize = 2048
	}

	cfg.Overlap = clamp(cfg.Overlap, 0.25, 0.95)
	cfg.Smoothing = clamp(cfg.Smoothing, 0, 0.95)

	cfg.Window = strings.ToLower(strings.TrimSpace(cfg.Window))
	if cfg.Window == "" {
		cfg.Window = "blackmanharris"
	}

	return cfg
}

func spectrumWindowType(name string) (window.Type, error) {
	switch name {
	case "hann":
		return window.TypeHann, nil
	case "hamming":
		return window.TypeHamming, nil
	case "blackman":
		return window.TypeBlackman, nil
	case "blackmanharris":
		return window.TypeBlackmanHarris4Term, nil
	case "flattop":
		return window.TypeFlatTop, nil
	default:
		return 0, fmt.Errorf("unsupported spectrum window: %s", name)
	}
}
