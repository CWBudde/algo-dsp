package graphiceq

import (
	"errors"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
	"github.com/cwbudde/algo-dsp/dsp/window"
)

var (
	// ErrNoBands is returned when a design receives no bands.
	ErrNoBands = errors.New("graphiceq: no bands")
	// ErrInvalidSampleRate is returned for a non-positive sample rate.
	ErrInvalidSampleRate = errors.New("graphiceq: sample rate must be positive")
	// ErrInvalidBand is returned when a band centre is outside (0, Nyquist),
	// the centres are not strictly ascending, or a gain is not finite.
	ErrInvalidBand = errors.New("graphiceq: invalid band")
	// ErrInvalidSplit is returned when the number of bands realised as IIR
	// sections is negative or exceeds the number of bands.
	ErrInvalidSplit = errors.New("graphiceq: invalid IIR band count")
	// ErrInvalidLength is returned when the FIR length is negative or even, or
	// when the design grid cannot hold it.
	ErrInvalidLength = errors.New("graphiceq: invalid filter length")
	// ErrInvalidQ is returned for a negative section Q.
	ErrInvalidQ = errors.New("graphiceq: Q must not be negative")
	// ErrInvalidWindowAlpha is returned for a negative window alpha.
	ErrInvalidWindowAlpha = errors.New(
		"graphiceq: window alpha must not be negative",
	)
)

// Band is one equaliser band.
type Band struct {
	// Frequency is the band centre in Hz. It must lie strictly between zero
	// and the Nyquist frequency.
	Frequency float64

	// GainDB is the requested gain at the centre.
	GainDB float64
}

// Config configures [Design].
type Config struct {
	// SampleRate is the sample rate in Hz.
	SampleRate float64

	// Bands lists the equaliser bands in strictly ascending centre frequency.
	Bands []Band

	// IIRBands is how many of the lowest bands are realised as biquad
	// sections. Zero produces the all-FIR reference design. It must stay below
	// len(Bands): the shelf cascade only encodes gain differences, so the FIR
	// is what supplies the broadband level they are measured against.
	IIRBands int

	// Length is the number of FIR taps and must be odd, because the FIR part
	// is linear phase. Zero uses [DefaultLength], which derives the length
	// from the lowest band the FIR still has to shape.
	Length int

	// Q is the quality factor of every shelving section. Zero uses one, which
	// tracks the log-linear target between two octave centres more closely
	// than a maximally flat shelf does. Negative values are rejected.
	Q float64

	// FFTSize controls the dense design grid. Zero selects a power of two at
	// least eight times the FIR length. It must be even and at least as long
	// as the FIR.
	FFTSize int

	// Window is the truncation window applied to the FIR taps. The zero value
	// is rectangular; a tapered window trades passband accuracy for less
	// truncation ripple.
	Window window.Type

	// WindowAlpha supplies the alpha or beta parameter for parametric
	// windows. Zero uses the window package default. Negative values are
	// rejected.
	WindowAlpha float64
}

// Result contains a designed hybrid equaliser.
type Result struct {
	// Sections is the IIR cascade realising the lowest IIRBands bands. It is
	// nil for the all-FIR design and can be fed to
	// [biquad.NewChain] unchanged.
	Sections []biquad.Coefficients

	// Taps is the linear-phase FIR realising the remaining bands, including
	// the broadband level the shelf cascade is measured against.
	Taps []float64

	// Latency is the group delay of the FIR part in samples, (len(Taps)-1)/2.
	// The IIR part contributes no latency, which is the point of the split.
	Latency int

	// Metrics compares the combined response with the requested band gains.
	Metrics Metrics
}

// Metrics describes how closely a design follows the requested gain curve.
//
// All errors are dB differences against the target curve, which interpolates
// the requested band gains linearly in log frequency and stays flat outside the
// outermost band. They are measured over [Bands[0].Frequency/2, Nyquist], since
// below that the target carries no information the design was asked to follow.
type Metrics struct {
	// RMSErrorDB and MaxErrorDB summarise the dB error over the measurement
	// range.
	RMSErrorDB float64
	MaxErrorDB float64

	// BandErrorDB is the dB error at each requested band centre, in the order
	// the bands were given.
	BandErrorDB []float64
}
