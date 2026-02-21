package pitch

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/interp"
)

const (
	defaultPitchShifterRatio = 1.0
	// Music-tuned defaults: longer sequence window ensures several beat cycles
	// fit within the autocorrelation window, improving segment selection quality
	// for polyphonic material. Matches SoundTouch's music preset (82/10/28 ms).
	defaultPitchShifterSequenceMs = 82.0
	defaultPitchShifterOverlapMs  = 10.0
	defaultPitchShifterSearchMs   = 28.0

	minPitchShifterRatio = 0.25
	maxPitchShifterRatio = 4.0

	minPitchShifterSequenceMs = 20.0
	maxPitchShifterSequenceMs = 120.0
	minPitchShifterOverlapMs  = 4.0
	maxPitchShifterOverlapMs  = 60.0
	minPitchShifterSearchMs   = 2.0
	maxPitchShifterSearchMs   = 40.0

	pitchShifterIdentityEps = 1e-9
	pitchShifterTiny        = 1e-12
)

// PitchShifter performs time-domain pitch shifting using a WSOLA-style
// stretch stage followed by high-quality fractional resampling.
//
// Pitch ratio:
//   - 1.0 = unchanged
//   - 2.0 = one octave up
//   - 0.5 = one octave down
//
// This processor is mono and block-based.
type PitchShifter struct {
	sampleRate float64
	pitchRatio float64

	sequenceMs float64
	overlapMs  float64
	searchMs   float64

	sequenceLen int
	overlapLen  int
	searchLen   int
	stepOut     int

	fadeIn  []float64
	fadeOut []float64
}

// NewPitchShifter constructs a time-domain pitch shifter with tuned defaults.
func NewPitchShifter(sampleRate float64) (*PitchShifter, error) {
	if !isFinitePositive(sampleRate) {
		return nil, fmt.Errorf("pitch shifter sample rate must be positive and finite: %f", sampleRate)
	}
	p := &PitchShifter{
		sampleRate: sampleRate,
		pitchRatio: defaultPitchShifterRatio,
		sequenceMs: defaultPitchShifterSequenceMs,
		overlapMs:  defaultPitchShifterOverlapMs,
		searchMs:   defaultPitchShifterSearchMs,
	}
	if err := p.rebuild(); err != nil {
		return nil, err
	}
	return p, nil
}

// SampleRate returns the current sample rate in Hz.
func (p *PitchShifter) SampleRate() float64 { return p.sampleRate }

// PitchRatio returns the pitch ratio.
func (p *PitchShifter) PitchRatio() float64 { return p.pitchRatio }

// PitchSemitones returns the current pitch shift in semitones.
func (p *PitchShifter) PitchSemitones() float64 { return 12.0 * math.Log2(p.pitchRatio) }

// Sequence returns sequence length in milliseconds.
func (p *PitchShifter) Sequence() float64 { return p.sequenceMs }

// Overlap returns overlap length in milliseconds.
func (p *PitchShifter) Overlap() float64 { return p.overlapMs }

// Search returns seek window radius in milliseconds.
func (p *PitchShifter) Search() float64 { return p.searchMs }

// SetSampleRate updates the sample rate and recalculates internal windows.
func (p *PitchShifter) SetSampleRate(sampleRate float64) error {
	if !isFinitePositive(sampleRate) {
		return fmt.Errorf("pitch shifter sample rate must be positive and finite: %f", sampleRate)
	}
	old := p.sampleRate
	p.sampleRate = sampleRate
	if err := p.rebuild(); err != nil {
		p.sampleRate = old
		_ = p.rebuild()
		return err
	}
	return nil
}

// SetPitchRatio updates the pitch shift ratio.
func (p *PitchShifter) SetPitchRatio(ratio float64) error {
	if !isFinitePositive(ratio) || ratio < minPitchShifterRatio || ratio > maxPitchShifterRatio {
		return fmt.Errorf("pitch shifter ratio must be in [%f, %f]: %f",
			minPitchShifterRatio, maxPitchShifterRatio, ratio)
	}
	p.pitchRatio = ratio
	return nil
}

// SetPitchSemitones updates pitch shift in semitones.
func (p *PitchShifter) SetPitchSemitones(semitones float64) error {
	if math.IsNaN(semitones) || math.IsInf(semitones, 0) {
		return fmt.Errorf("pitch shifter semitones must be finite: %f", semitones)
	}
	ratio := math.Pow(2, semitones/12.0)
	if err := p.SetPitchRatio(ratio); err != nil {
		return fmt.Errorf("pitch shifter semitones out of range: %w", err)
	}
	return nil
}

// SetSequence updates sequence length in milliseconds.
func (p *PitchShifter) SetSequence(ms float64) error {
	if ms < minPitchShifterSequenceMs || ms > maxPitchShifterSequenceMs ||
		math.IsNaN(ms) || math.IsInf(ms, 0) {
		return fmt.Errorf("pitch shifter sequence must be in [%f, %f] ms: %f",
			minPitchShifterSequenceMs, maxPitchShifterSequenceMs, ms)
	}
	old := p.sequenceMs
	p.sequenceMs = ms
	if err := p.rebuild(); err != nil {
		p.sequenceMs = old
		_ = p.rebuild()
		return err
	}
	return nil
}

// SetOverlap updates overlap length in milliseconds.
func (p *PitchShifter) SetOverlap(ms float64) error {
	if ms < minPitchShifterOverlapMs || ms > maxPitchShifterOverlapMs ||
		math.IsNaN(ms) || math.IsInf(ms, 0) {
		return fmt.Errorf("pitch shifter overlap must be in [%f, %f] ms: %f",
			minPitchShifterOverlapMs, maxPitchShifterOverlapMs, ms)
	}
	old := p.overlapMs
	p.overlapMs = ms
	if err := p.rebuild(); err != nil {
		p.overlapMs = old
		_ = p.rebuild()
		return err
	}
	return nil
}

// SetSearch updates seek window radius in milliseconds.
func (p *PitchShifter) SetSearch(ms float64) error {
	if ms < minPitchShifterSearchMs || ms > maxPitchShifterSearchMs ||
		math.IsNaN(ms) || math.IsInf(ms, 0) {
		return fmt.Errorf("pitch shifter search must be in [%f, %f] ms: %f",
			minPitchShifterSearchMs, maxPitchShifterSearchMs, ms)
	}
	old := p.searchMs
	p.searchMs = ms
	if err := p.rebuild(); err != nil {
		p.searchMs = old
		_ = p.rebuild()
		return err
	}
	return nil
}

// Reset clears processor state.
//
// PitchShifter is currently stateless, so Reset is a no-op.
func (p *PitchShifter) Reset() {}

// Process pitch-shifts input and returns a new output block with equal length.
func (p *PitchShifter) Process(input []float64) []float64 {
	if len(input) == 0 {
		return nil
	}
	if math.Abs(p.pitchRatio-1) <= pitchShifterIdentityEps {
		out := make([]float64, len(input))
		copy(out, input)
		return out
	}

	stretched := p.timeStretch(input)
	return pitchResampleHermite(stretched, len(input))
}

// ProcessInPlace applies pitch shifting to buf in place.
func (p *PitchShifter) ProcessInPlace(buf []float64) {
	if len(buf) == 0 {
		return
	}
	out := p.Process(buf)
	copy(buf, out)
}

func (p *PitchShifter) rebuild() error {
	if !isFinitePositive(p.sampleRate) {
		return fmt.Errorf("pitch shifter sample rate must be positive and finite: %f", p.sampleRate)
	}
	if p.overlapMs >= p.sequenceMs {
		return fmt.Errorf("pitch shifter overlap must be smaller than sequence: overlap=%f sequence=%f",
			p.overlapMs, p.sequenceMs)
	}

	p.sequenceLen = int(math.Round(p.sequenceMs * 0.001 * p.sampleRate))
	if p.sequenceLen < 32 {
		p.sequenceLen = 32
	}
	p.overlapLen = int(math.Round(p.overlapMs * 0.001 * p.sampleRate))
	if p.overlapLen < 8 {
		p.overlapLen = 8
	}
	if p.overlapLen >= p.sequenceLen {
		return fmt.Errorf("pitch shifter overlap too large for sequence: overlap=%d sequence=%d",
			p.overlapLen, p.sequenceLen)
	}
	p.stepOut = p.sequenceLen - p.overlapLen
	if p.stepOut < 4 {
		return fmt.Errorf("pitch shifter output hop too small: %d", p.stepOut)
	}

	p.searchLen = int(math.Round(p.searchMs * 0.001 * p.sampleRate))
	if p.searchLen < 1 {
		p.searchLen = 1
	}

	p.fadeIn = make([]float64, p.overlapLen)
	p.fadeOut = make([]float64, p.overlapLen)
	if p.overlapLen == 1 {
		p.fadeIn[0] = 1
		p.fadeOut[0] = 0
		return nil
	}
	for i := range p.overlapLen {
		t := float64(i) / float64(p.overlapLen-1)
		in := 0.5 - 0.5*math.Cos(math.Pi*t)
		p.fadeIn[i] = in
		p.fadeOut[i] = 1 - in
	}
	return nil
}

func (p *PitchShifter) timeStretch(input []float64) []float64 {
	targetLen := int(math.Round(float64(len(input)) * p.pitchRatio))
	if targetLen < 1 {
		targetLen = 1
	}

	nominalInStep := float64(p.stepOut) / p.pitchRatio
	if nominalInStep < 1 {
		nominalInStep = 1
	}

	nFrames := targetLen/p.stepOut + 4
	outCap := nFrames*p.stepOut + p.sequenceLen + 1
	out := make([]float64, outCap)

	for i := 0; i < p.sequenceLen; i++ {
		out[i] = pitchSampleZero(input, i)
	}
	outLen := p.sequenceLen
	prevStart := 0
	nextNominal := nominalInStep
	ref := make([]float64, p.overlapLen)

	for outLen < targetLen+p.sequenceLen {
		refStart := prevStart + p.stepOut
		for i := 0; i < p.overlapLen; i++ {
			ref[i] = pitchSampleZero(input, refStart+i)
		}

		predicted := int(math.Round(nextNominal))
		candStart := p.findBestOverlap(ref, input, predicted)

		outStart := outLen - p.overlapLen
		for i := 0; i < p.overlapLen; i++ {
			yOld := out[outStart+i]
			yNew := pitchSampleZero(input, candStart+i)
			out[outStart+i] = yOld*p.fadeOut[i] + yNew*p.fadeIn[i]
		}
		writePos := outStart + p.overlapLen
		for i := p.overlapLen; i < p.sequenceLen; i++ {
			out[writePos+i-p.overlapLen] = pitchSampleZero(input, candStart+i)
		}

		outLen = outStart + p.sequenceLen
		prevStart = candStart
		nextNominal += nominalInStep

		if prevStart > len(input)+p.sequenceLen && outLen >= targetLen {
			break
		}
	}

	if targetLen <= len(out) {
		return out[:targetLen]
	}
	padded := make([]float64, targetLen)
	copy(padded, out)
	return padded
}

func (p *PitchShifter) findBestOverlap(ref, input []float64, predicted int) int {
	best := predicted
	bestScore := math.Inf(-1)

	searchStart := predicted - p.searchLen
	searchEnd := predicted + p.searchLen

	refEnergy := pitchShifterTiny
	for _, v := range ref {
		refEnergy += v * v
	}

	for cand := searchStart; cand <= searchEnd; cand++ {
		dot := 0.0
		candEnergy := pitchShifterTiny
		for i, rv := range ref {
			cv := pitchSampleZero(input, cand+i)
			dot += rv * cv
			candEnergy += cv * cv
		}
		score := dot / math.Sqrt(refEnergy*candEnergy)
		if score > bestScore {
			bestScore = score
			best = cand
		}
	}

	return best
}

func pitchResampleHermite(input []float64, outLen int) []float64 {
	if outLen <= 0 || len(input) == 0 {
		return nil
	}

	out := make([]float64, outLen)
	if len(input) == 1 {
		for i := range out {
			out[i] = input[0]
		}
		return out
	}
	if outLen == 1 {
		out[0] = input[0]
		return out
	}

	step := float64(len(input)-1) / float64(outLen-1)
	pos := 0.0
	for i := range out {
		out[i] = pitchSampleHermite(input, pos)
		pos += step
	}
	return out
}

func pitchSampleHermite(input []float64, pos float64) float64 {
	idx := int(math.Floor(pos))
	frac := pos - float64(idx)
	xm1 := pitchSampleClamp(input, idx-1)
	x0 := pitchSampleClamp(input, idx)
	x1 := pitchSampleClamp(input, idx+1)
	x2 := pitchSampleClamp(input, idx+2)
	return interp.Hermite4(frac, xm1, x0, x1, x2)
}

func pitchSampleZero(x []float64, idx int) float64 {
	if idx < 0 || idx >= len(x) {
		return 0
	}
	return x[idx]
}

func pitchSampleClamp(x []float64, idx int) float64 {
	if len(x) == 0 {
		return 0
	}
	if idx < 0 {
		return x[0]
	}
	if idx >= len(x) {
		return x[len(x)-1]
	}
	return x[idx]
}

func isFinitePositive(v float64) bool {
	return v > 0 && !math.IsNaN(v) && !math.IsInf(v, 0)
}
