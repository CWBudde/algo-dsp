package pitch

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/interp"
)

// StreamingPitchShifter performs causal WSOLA time stretching followed by
// fractional Hermite resampling. Analysis positions, overlap searches and output
// coordinates use a persistent sample clock, independently of block boundaries.
// Configuration belongs to the supplied PitchShifter and must remain unchanged.
type StreamingPitchShifter struct {
	processor                               *PitchShifter
	input, stretched                        []float64
	reference, search                       []float64
	inputMask, outputMask                   int64
	count, frames, previousStart, finalized int64
	latency                                 int
}

// NewStreamingPitchShifter reserves bounded input and synthesis history and
// creates a fresh stream. The existing one-shot PitchShifter APIs are unchanged.
func NewStreamingPitchShifter(processor *PitchShifter) (*StreamingPitchShifter, error) {
	if processor == nil {
		return nil, fmt.Errorf("pitch stream: nil processor")
	}

	if !isFinitePositive(processor.sampleRate) || !isFinitePositive(processor.pitchRatio) || processor.pitchRatio < minPitchShifterRatio || processor.pitchRatio > maxPitchShifterRatio || processor.sequenceLen < 1 || processor.overlapLen < 1 || processor.overlapLen >= processor.sequenceLen || processor.stepOut < 1 {
		return nil, fmt.Errorf("pitch stream: invalid processor configuration")
	}

	latency := processor.sequenceLen + processor.searchLen + int(math.Ceil(float64(processor.stepOut)/processor.pitchRatio)) + 8

	inputSize := 1
	for inputSize < 4*(processor.sequenceLen+processor.searchLen)+64 {
		inputSize *= 2
	}

	outputSize := 1
	for outputSize < 4*(latency+processor.sequenceLen+processor.searchLen)+64 {
		outputSize *= 2
	}

	return &StreamingPitchShifter{processor: processor, input: make([]float64, inputSize), stretched: make([]float64, outputSize), reference: make([]float64, processor.overlapLen), search: make([]float64, processor.overlapLen+2*processor.searchLen), inputMask: int64(inputSize - 1), outputMask: int64(outputSize - 1), latency: latency}, nil
}

// Latency returns bounded WSOLA lookahead, or zero for the identity ratio.
func (p *StreamingPitchShifter) Latency() int {
	if math.Abs(p.processor.pitchRatio-1) <= pitchShifterIdentityEps {
		return 0
	}

	return p.latency
}

// Reset rewinds the sample clock and histories without reallocating.
func (p *StreamingPitchShifter) Reset() {
	clear(p.input)
	clear(p.stretched)
	clear(p.reference)
	clear(p.search)
	p.count = 0
	p.frames = 0
	p.previousStart = 0
	p.finalized = 0
}

func (p *StreamingPitchShifter) inputSample(position int64) float64 {
	if position < 0 || position >= p.count {
		return 0
	}

	return p.input[position&p.inputMask]
}

func (p *StreamingPitchShifter) synthesisSample(position int64) float64 {
	if position < 0 {
		return p.stretched[0]
	}

	return p.stretched[position&p.outputMask]
}

func (p *StreamingPitchShifter) makeFrame(start int64) {
	fx := p.processor

	out := p.frames * int64(fx.stepOut)
	for i := 0; i < fx.sequenceLen; i++ {
		x := p.inputSample(start + int64(i))
		if p.frames > 0 && i < fx.overlapLen {
			x = p.inputSample(p.previousStart+int64(fx.stepOut+i))*fx.fadeOut[i] + x*fx.fadeIn[i]
		}

		p.stretched[(out+int64(i))&p.outputMask] = x
	}

	p.previousStart = start
	p.frames++
	p.finalized = out + int64(fx.stepOut)
}

func (p *StreamingPitchShifter) bestOverlap(predicted int64) int64 {
	fx := p.processor
	best := predicted
	score := math.Inf(-1)
	refEnergy := pitchShifterTiny
	reference := p.reference[:fx.overlapLen]

	for i := range reference {
		x := p.inputSample(p.previousStart + int64(fx.stepOut+i))
		reference[i] = x
		refEnergy += x * x
	}

	first := max(predicted-int64(fx.searchLen), 0)
	last := predicted + int64(fx.searchLen)

	if last < first {
		return best
	}

	// Materialize the complete search window once. Correlation retains the
	// original sample and candidate order, but avoids guarded ring accesses
	// for every term of every candidate's dot product and energy sum.
	search := p.search[:int(last-first)+fx.overlapLen]
	for i := range search {
		search[i] = p.inputSample(first + int64(i))
	}

	for candidate := first; candidate <= last; candidate++ {
		dot, energy := 0.0, pitchShifterTiny
		offset := int(candidate - first)
		window := search[offset : offset+len(reference)]

		for i, ref := range reference {
			x := window[i]
			dot += ref * x
			energy += x * x
		}

		correlation := dot / math.Sqrt(refEnergy*energy)
		if correlation > score {
			score = correlation
			best = candidate
		}
	}

	return best
}

// ProcessInPlace consumes and produces the same number of samples without
// allocation. Feed zeros after input to drain Latency and the final overlap.
func (p *StreamingPitchShifter) ProcessInPlace(block []float64) error {
	if p.Latency() == 0 {
		return nil
	}

	fx := p.processor
	for i, x := range block {
		position := p.count
		p.input[position&p.inputMask] = x

		p.count++
		if p.frames == 0 && p.count >= int64(fx.sequenceLen) {
			p.makeFrame(0)
		}

		for p.frames > 0 {
			predicted := int64(math.Round(float64(p.frames) * float64(fx.stepOut) / fx.pitchRatio))
			if p.count < predicted+int64(fx.searchLen+fx.sequenceLen) {
				break
			}

			p.makeFrame(p.bestOverlap(predicted))
		}

		if position < int64(p.latency) {
			block[i] = 0
			continue
		}

		readPosition := float64(position-int64(p.latency)) * fx.pitchRatio

		index := int64(math.Floor(readPosition))
		if index+2 >= p.finalized {
			return fmt.Errorf("pitch stream: insufficient synthesis lookahead")
		}

		block[i] = interp.Hermite4(readPosition-float64(index), p.synthesisSample(index-1), p.synthesisSample(index), p.synthesisSample(index+1), p.synthesisSample(index+2))
	}

	return nil
}
