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
	engine      *conv.PartitionedConvolution
	wet         float64
	dry         float64
	latency     int
	buf         []float64 // scratch output buffer
	dryHistory  []float64
	dryPosition int
	alignedDry  bool
}

// NewConvolutionReverb creates a convolution reverb from a mono IR.
// minBlockOrder determines latency: latency = 2^minBlockOrder samples
// (e.g. 6=64 samples, 7=128 samples, 8=256 samples).
// The maximum partition size is 8192 samples (block order 13).
func NewConvolutionReverb(kernel []float64, minBlockOrder int) (*ConvolutionReverb, error) {
	return NewConvolutionReverbWithMaxBlockOrder(kernel, minBlockOrder, 13)
}

// NewConvolutionReverbWithMaxBlockOrder creates a convolution reverb with an
// explicit maximum partition size of 2^maxBlockOrder samples. Smaller maximum
// partitions reduce transform bursts at the cost of more frequent processing;
// the complete IR and latency of 2^minBlockOrder samples are preserved.
func NewConvolutionReverbWithMaxBlockOrder(kernel []float64, minBlockOrder, maxBlockOrder int) (*ConvolutionReverb, error) {
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

// SetLatencyAlignedDry delays the dry path by the convolution engine's technical
// latency. This permits a host to compensate startup latency without shifting
// the dry signal relative to the impulse response. The default is false to
// preserve the traditional send-reverb API. Enabling reserves bounded storage.
func (r *ConvolutionReverb) SetLatencyAlignedDry(enabled bool) {
	if enabled && len(r.dryHistory) != r.latency {
		r.dryHistory = make([]float64, r.latency)
		r.dryPosition = 0
	}

	r.alignedDry = enabled
}

// Prepare reserves scratch space for blocks up to maxFrames without advancing
// convolution history. Later ProcessInPlace calls within that size allocate no
// scratch space. Existing processing state is preserved.
func (r *ConvolutionReverb) Prepare(maxFrames int) error {
	if maxFrames < 1 {
		return errors.New("reverb: invalid maximum block size")
	}

	if len(r.buf) < maxFrames {
		r.buf = make([]float64, maxFrames)
	}

	return nil
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
		drySample := block[i]
		if r.alignedDry {
			drySample = r.dryHistory[r.dryPosition]
			r.dryHistory[r.dryPosition] = block[i]

			r.dryPosition++
			if r.dryPosition == len(r.dryHistory) {
				r.dryPosition = 0
			}
		}

		block[i] = dry*drySample + wet*reverbOut[i]
	}

	return nil
}

// Reset clears convolution state.
func (r *ConvolutionReverb) Reset() {
	r.engine.Reset()
	clear(r.dryHistory)
	r.dryPosition = 0
}

// Latency returns the reverb latency in samples.
func (r *ConvolutionReverb) Latency() int {
	return r.latency
}
