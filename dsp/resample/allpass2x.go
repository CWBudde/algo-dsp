package resample

import (
	"fmt"

	"github.com/cwbudde/algo-dsp/dsp/filter/hilbert"
)

// AllpassUpsampler2x32 is a stateful 2× polyphase IIR interpolator for float32
// samples. Use one instance per channel. Processing allocates no memory.
type AllpassUpsampler2x32 struct {
	stages allpassStages32
}

// NewAllpassUpsampler2x32 designs an interpolator with the given total number
// of allpass sections and normalized transition bandwidth (between 0 and 0.5).
func NewAllpassUpsampler2x32(count int, transition float64) (*AllpassUpsampler2x32, error) {
	stages, err := designAllpassStages32(count, transition)
	if err != nil {
		return nil, err
	}

	return &AllpassUpsampler2x32{stages: stages}, nil
}

// ProcessSample consumes one input sample and returns two consecutive output
// samples in time order.
func (u *AllpassUpsampler2x32) ProcessSample(input float32) (first, second float32) {
	return u.stages.process(input, 0), u.stages.process(input, 1)
}

// Reset clears the filter history.
func (u *AllpassUpsampler2x32) Reset() {
	u.stages.reset()
}

// AllpassDownsampler2x32 is a stateful 2× polyphase IIR decimator for float32
// samples. Use one instance per channel. Processing allocates no memory.
type AllpassDownsampler2x32 struct {
	stages allpassStages32
}

// NewAllpassDownsampler2x32 designs a decimator with the given total number of
// allpass sections and normalized transition bandwidth (between 0 and 0.5).
func NewAllpassDownsampler2x32(count int, transition float64) (*AllpassDownsampler2x32, error) {
	stages, err := designAllpassStages32(count, transition)
	if err != nil {
		return nil, err
	}

	return &AllpassDownsampler2x32{stages: stages}, nil
}

// ProcessSample consumes two consecutive input samples in time order and
// returns one output sample.
func (d *AllpassDownsampler2x32) ProcessSample(first, second float32) float32 {
	even := d.stages.process(second, 0)
	odd := d.stages.process(first, 1)

	return float32((float64(even) + float64(odd)) * 0.5)
}

// Reset clears the filter history.
func (d *AllpassDownsampler2x32) Reset() {
	d.stages.reset()
}

type allpassStage32 struct {
	coefficient    float32
	previousInput  float32
	previousOutput float32
}

// The sections of the two paths alternate in coefficient order.
type allpassStages32 []allpassStage32

func designAllpassStages32(count int, transition float64) (allpassStages32, error) {
	coefficients, err := hilbert.DesignCoefficients(count, transition)
	if err != nil {
		return nil, fmt.Errorf("resample: allpass design: %w", err)
	}

	stages := make(allpassStages32, len(coefficients))
	for i, coefficient := range coefficients {
		rounded := float32(coefficient)
		if !(-1 < rounded && rounded < 1) {
			return nil, fmt.Errorf("resample: allpass coefficient %d is unstable at float32 precision: %g", i, rounded)
		}

		stages[i].coefficient = rounded
	}

	return stages, nil
}

func (stages allpassStages32) process(input float32, phase int) float32 {
	for i := phase; i < len(stages); i += 2 {
		stage := &stages[i]
		// Round only the section output, which feeds the next section and the delay.
		output := float32(float64(stage.previousInput) +
			float64(stage.coefficient)*(float64(input)-float64(stage.previousOutput)))
		stage.previousInput = input
		stage.previousOutput = output
		input = output
	}

	return input
}

func (stages allpassStages32) reset() {
	for i := range stages {
		stages[i].previousInput = 0
		stages[i].previousOutput = 0
	}
}
