package resample

import "math"

// alignedPadInput is the minimum number of zero input frames appended beyond
// the input-domain group delay so the interpolation in ProcessAligned always
// has a right-hand neighbour for its last output sample.
const alignedPadInput = 4

// ProcessAligned resamples a whole finite signal with the FIR group delay
// removed. Output sample j corresponds to input time j*down/up (see Ratio), so
// an event at input frame t appears at output frame t*up/down, and the output
// length is round(len(input)*up/down).
//
// The causal filter tail is flushed with zeros, then the output is shifted by
// GroupDelayOutput. The integer part of that delay is an index offset; the
// fractional part is removed by linear interpolation between neighbouring
// filter outputs. That interpolation is a slight extra low-pass, well below the
// conversion's anti-aliasing cutoff at the default quality settings.
//
// Because compensation needs look-ahead, ProcessAligned is a whole-buffer
// operation, not a streaming one. It runs on a fresh Clone and therefore
// neither reads nor changes the receiver's streaming state: calls are
// independent of earlier Process or ProcessInto calls, leave later ones
// unaffected, and may run concurrently on the same Resampler.
//
// Empty input returns an empty slice. ErrOutputTooLarge is returned when the
// output length cannot fit in int.
func (r *Resampler) ProcessAligned(input []float64) ([]float64, error) {
	scaled := float64(len(input)) * float64(r.up) / float64(r.down)
	if scaled >= float64(math.MaxInt) {
		return nil, ErrOutputTooLarge
	}

	output := make([]float64, int(math.Round(scaled)))
	if len(output) == 0 {
		return output, nil
	}

	delayOutput := r.GroupDelayOutput()
	// The last output reads raw[int(len(output)-1+delayOutput)+1]; size the
	// zero flush so that index always exists, but never below the delay-based
	// padding, which keeps results reproducible across padding policies.
	need := int(float64(len(output)-1)+delayOutput) + 2
	pad := int(math.Ceil(r.GroupDelayInput())) + alignedPadInput
	pad = maxInt(pad, int(math.Ceil(float64(need)*float64(r.down)/float64(r.up)))-len(input)+1)

	total := len(input) + pad
	if total < len(input) {
		return nil, ErrOutputTooLarge
	}

	stream := r.Clone()

	rawLen, fits := stream.outputLen(total)
	if !fits {
		return nil, ErrOutputTooLarge
	}

	raw := make([]float64, rawLen)

	written, err := stream.ProcessInto(raw, input)
	if err != nil {
		return nil, err
	}

	flushed, err := stream.ProcessInto(raw[written:], make([]float64, pad))
	if err != nil {
		return nil, err
	}

	raw = raw[:written+flushed]

	for i := range output {
		pos := float64(i) + delayOutput
		j := int(pos)

		u := pos - float64(j)
		if j+1 < len(raw) {
			// Keep this exact expression: callers rely on bit-identical
			// output across releases (see TestProcessAlignedReferenceParity).
			output[i] = raw[j]*(1-u) + raw[j+1]*u
		}
	}

	return output, nil
}

// ResampleAligned converts a whole signal from inRate to outRate with the
// filter group delay removed, so both signals share the same t=0 origin: an
// event at input time t seconds appears at output time t seconds.
//
// The rate ratio is approximated as in NewForRates and the result follows
// Resampler.ProcessAligned. When inRate equals outRate exactly, the input is
// returned as an unfiltered copy. Invalid rates return ErrInvalidRate.
func ResampleAligned(input []float64, inRate, outRate float64, opts ...Option) ([]float64, error) {
	if inRate <= 0 || outRate <= 0 || math.IsNaN(inRate) || math.IsNaN(outRate) ||
		math.IsInf(inRate, 0) || math.IsInf(outRate, 0) {
		return nil, ErrInvalidRate
	}

	if inRate == outRate {
		return append([]float64{}, input...), nil
	}

	r, err := NewForRates(inRate, outRate, opts...)
	if err != nil {
		return nil, err
	}

	return r.ProcessAligned(input)
}
