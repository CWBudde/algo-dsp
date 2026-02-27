package reverb

import (
	"errors"
	"fmt"

	"github.com/cwbudde/algo-dsp/dsp/conv"
)

// ConvolutionReverb applies a room impulse response via partitioned convolution.
// It adds a wet/dry send-effects reverb to a mono signal.
//
// The wet signal is produced by convolving the input with the given impulse
// response using non-uniformly partitioned overlap-add (UPOLA) convolution,
// which provides low-latency processing even for very long impulse responses.
type ConvolutionReverb struct {
	engine  *conv.PartitionedConvolution
	wet     float64
	dry     float64
	latency int
	buf     []float64 // scratch output buffer
}

// NewConvolutionReverb creates a convolution reverb from a mono IR.
// minBlockOrder determines latency: latency = 2^minBlockOrder samples
// (e.g. 6=64 samples, 7=128 samples, 8=256 samples).
// maxBlockOrder caps the maximum partition size; 13 is a good default.
func NewConvolutionReverb(kernel []float64, minBlockOrder int) (*ConvolutionReverb, error) {
	const maxBlockOrder = 13

	if len(kernel) == 0 {
		return nil, errors.New("reverb: empty impulse response kernel")
	}

	engine, err := conv.NewPartitionedConvolution(kernel, minBlockOrder, maxBlockOrder)
	if err != nil {
		return nil, fmt.Errorf("reverb: failed to create convolution engine: %w", err)
	}

	return &ConvolutionReverb{
		engine:  engine,
		wet:     1.0,
		dry:     1.0,
		latency: engine.Latency(),
	}, nil
}

// SetWetDry sets the wet and dry mix levels.
// wet controls the convolution reverb send level.
// dry controls the pass-through level of the original signal.
func (r *ConvolutionReverb) SetWetDry(wet, dry float64) {
	r.wet = wet
	r.dry = dry
}

// ProcessInPlace applies reverb to block in place (mono).
// The output is: block[i] = dry*block[i] + wet*reverb(block[i]).
// The block length may vary between calls.
func (r *ConvolutionReverb) ProcessInPlace(block []float64) error {
	n := len(block)
	if n == 0 {
		return nil
	}

	// Resize scratch buffer if needed.
	if len(r.buf) < n {
		r.buf = make([]float64, n)
	}

	reverbOut := r.buf[:n]

	err := r.engine.ProcessBlock(block, reverbOut)
	if err != nil {
		return fmt.Errorf("reverb: convolution engine: %w", err)
	}

	wet := r.wet
	dry := r.dry

	for i := range n {
		block[i] = dry*block[i] + wet*reverbOut[i]
	}

	return nil
}

// Reset clears convolution state.
func (r *ConvolutionReverb) Reset() {
	r.engine.Reset()
}

// Latency returns the reverb latency in samples.
func (r *ConvolutionReverb) Latency() int {
	return r.latency
}
