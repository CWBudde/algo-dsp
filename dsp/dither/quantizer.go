//nolint:funcorder
package dither

import (
	"fmt"
	"math"
	"math/rand/v2"
)

// Quantizer performs bit-depth quantization with optional dither noise
// and noise shaping.
type Quantizer struct {
	sampleRate      float64
	bitDepth        int
	ditherType      DitherType
	ditherAmplitude float64
	limit           bool
	shaper          NoiseShaper
	rng             *rand.Rand

	// derived from bitDepth
	bitMul  float64
	bitDiv  float64
	limitLo int
	limitHi int
}

// NewQuantizer creates a new Quantizer. The default configuration is:
// 16-bit, triangular dither, amplitude 1.0, limiting enabled,
// F-weighted 9th-order FIR noise shaper (Preset9FC).
func NewQuantizer(sampleRate float64, opts ...Option) (*Quantizer, error) {
	if sampleRate <= 0 || math.IsNaN(sampleRate) || math.IsInf(sampleRate, 0) {
		return nil, fmt.Errorf("dither: sample rate must be > 0 and finite: %f", sampleRate)
	}

	cfg := defaultConfig()

	for _, opt := range opts {
		if opt == nil {
			continue
		}

		err := opt(&cfg)
		if err != nil {
			return nil, err
		}
	}

	// Resolve noise shaper based on config precedence.
	var shaper NoiseShaper
	switch {
	case cfg.shaper != nil:
		shaper = cfg.shaper
	case cfg.sharpPreset:
		shaper = NewFIRShaper(SharpPresetForSampleRate(sampleRate))
	case cfg.iirShelfFreq > 0:
		var err error

		shaper, err = NewIIRShelfShaper(cfg.iirShelfFreq, sampleRate)
		if err != nil {
			return nil, err
		}
	default:
		shaper = NewFIRShaper(Preset9FC.Coefficients())
	}

	quant := &Quantizer{
		sampleRate:      sampleRate,
		bitDepth:        cfg.bitDepth,
		ditherType:      cfg.ditherType,
		ditherAmplitude: cfg.ditherAmplitude,
		limit:           cfg.limit,
		shaper:          shaper,
		rng:             cfg.rng,
	}

	if quant.rng == nil {
		quant.rng = rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))
	}

	quant.updateDerived()

	return quant, nil
}

func (q *Quantizer) updateDerived() {
	q.bitMul = math.Exp2(float64(q.bitDepth-1)) - 0.5
	q.bitDiv = 1.0 / q.bitMul
	q.limitLo = -int(math.Round(q.bitMul + 0.5))
	q.limitHi = int(math.Round(q.bitMul - 0.5))
}

// ProcessInteger quantizes the input (expected in [-1, +1]) to an integer
// in the bit-depth range.
func (q *Quantizer) ProcessInteger(input float64) int {
	// 1. Scale to integer range.
	scaled := q.bitMul * input

	// 2. Apply noise shaping (subtracts weighted past errors).
	shaped := q.shaper.Shape(scaled)

	// 3. Add dither and quantize.
	result := q.quantize(shaped)

	// 4. Optional limiting.
	if q.limit {
		result = max(q.limitLo, min(q.limitHi, result))
	}

	// 5. Record quantization error for next iteration.
	q.shaper.RecordError(float64(result) - shaped)

	return result
}

// ProcessSample quantizes the input and returns a normalized float64
// in approximately [-1, +1].
func (q *Quantizer) ProcessSample(input float64) float64 {
	return (float64(q.ProcessInteger(input)) + 0.5) * q.bitDiv
}

// ProcessInPlace quantizes each sample in buf in-place.
func (q *Quantizer) ProcessInPlace(buf []float64) {
	for idx, val := range buf {
		buf[idx] = q.ProcessSample(val)
	}
}

// Reset clears all internal state (noise shaper history).
func (q *Quantizer) Reset() {
	q.shaper.Reset()
}

// quantize adds dither noise per the configured type and rounds to integer.
// The floor operation implements the truncation bias from the legacy algorithm
// (equivalent to round(x - 0.5) with banker's rounding).
func (q *Quantizer) quantize(input float64) int {
	switch q.ditherType {
	case DitherNone:
		return int(math.Floor(input))
	case DitherRectangular:
		noise := q.ditherAmplitude * (q.rng.Float64()*2 - 1)
		return int(math.Floor(input + noise))
	case DitherTriangular:
		noise := q.ditherAmplitude * (q.rng.Float64() - q.rng.Float64())
		return int(math.Floor(input + noise))
	case DitherGaussian:
		noise := q.ditherAmplitude * q.rng.NormFloat64()
		return int(math.Floor(input + noise))
	case DitherFastGaussian:
		noise := q.ditherAmplitude * q.fastGaussian()
		return int(math.Floor(input + noise))
	default:
		return int(math.Floor(input))
	}
}

// fastGaussian approximates a Gaussian distribution by summing uniform draws.
// The central limit theorem gives a reasonable approximation with 6 draws.
func (q *Quantizer) fastGaussian() float64 {
	var sum float64
	for range 6 {
		sum += q.rng.Float64()
	}

	return sum - 3.0 // mean-centered, approximate stddev ~0.5
}

// Getters.

// BitDepth returns the current target bit depth.
func (q *Quantizer) BitDepth() int { return q.bitDepth }

// DitherType returns the current dither noise type.
func (q *Quantizer) DitherType() DitherType { return q.ditherType }

// DitherAmplitude returns the current dither noise amplitude.
func (q *Quantizer) DitherAmplitude() float64 { return q.ditherAmplitude }

// Limit returns whether output limiting is enabled.
func (q *Quantizer) Limit() bool { return q.limit }

// SampleRate returns the configured sample rate.
func (q *Quantizer) SampleRate() float64 { return q.sampleRate }

// Setters.

// SetBitDepth changes the target bit depth (1–32).
func (q *Quantizer) SetBitDepth(bits int) error {
	if bits < minBitDepth || bits > maxBitDepth {
		return fmt.Errorf("dither: bit depth must be in [%d, %d]: %d", minBitDepth, maxBitDepth, bits)
	}

	q.bitDepth = bits
	q.updateDerived()

	return nil
}

// SetDitherType changes the dither noise PDF.
func (q *Quantizer) SetDitherType(dt DitherType) error {
	if !dt.Valid() {
		return fmt.Errorf("dither: invalid dither type: %d", dt)
	}

	q.ditherType = dt

	return nil
}

// SetDitherAmplitude changes the dither noise amplitude.
func (q *Quantizer) SetDitherAmplitude(amp float64) error {
	if amp < 0 || math.IsNaN(amp) || math.IsInf(amp, 0) {
		return fmt.Errorf("dither: amplitude must be >= 0 and finite: %f", amp)
	}

	q.ditherAmplitude = amp

	return nil
}

// SetLimit enables or disables output limiting.
func (q *Quantizer) SetLimit(enabled bool) {
	q.limit = enabled
}
