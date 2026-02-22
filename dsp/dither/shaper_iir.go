package dither

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
	"github.com/cwbudde/algo-dsp/dsp/filter/design"
)

const (
	iirShelfDefaultGainDB = -5.0
	iirShelfDefaultQ      = 0.707 // 1/sqrt(2), Butterworth slope
)

// IIRShelfShaper implements noise shaping using a biquad low-shelf filter
// applied to the quantization error signal. This provides a lightweight
// alternative to FIR noise shaping with less precise spectral control
// but lower CPU cost.
type IIRShelfShaper struct {
	filter    *biquad.Section
	lastError float64
}

// NewIIRShelfShaper creates an IIR shelf noise shaper with the given corner
// frequency and sample rate. The shelf applies -5 dB of low-frequency
// de-emphasis to the error signal, pushing quantization noise above the
// shelf frequency where human hearing is less sensitive.
func NewIIRShelfShaper(freq, sampleRate float64) (*IIRShelfShaper, error) {
	if freq <= 0 || math.IsNaN(freq) || math.IsInf(freq, 0) {
		return nil, fmt.Errorf("dither: IIR shelf frequency must be > 0 and finite: %f", freq)
	}
	if sampleRate <= 0 || math.IsNaN(sampleRate) || math.IsInf(sampleRate, 0) {
		return nil, fmt.Errorf("dither: IIR shelf sample rate must be > 0 and finite: %f", sampleRate)
	}
	coeffs := design.LowShelf(freq, iirShelfDefaultGainDB, iirShelfDefaultQ, sampleRate)
	return &IIRShelfShaper{
		filter: biquad.NewSection(coeffs),
	}, nil
}

// Shape applies the IIR shelf filter to the previous error and subtracts
// it from the input.
func (s *IIRShelfShaper) Shape(input float64) float64 {
	return input - s.filter.ProcessSample(s.lastError)
}

// RecordError stores the quantization error for the next Shape call.
func (s *IIRShelfShaper) RecordError(quantizationError float64) {
	s.lastError = quantizationError
}

// Reset clears the biquad filter state and stored error.
func (s *IIRShelfShaper) Reset() {
	s.filter.Reset()
	s.lastError = 0
}
