package reverb

import (
	"fmt"
	"math"
)

const (
	fdnSize = 8

	defaultFDNSampleRate   = 44100.0
	defaultFDNWet          = 0.2
	defaultFDNDry          = 1.0
	defaultFDNRT60Seconds  = 1.8
	defaultFDNDamp         = 0.3
	defaultFDNPreDelaySec  = 0.01
	defaultFDNModDepthSec  = 0.002
	defaultFDNModRateHz    = 0.1
	minFDNDelayBufferSize  = 4
	fdnReferenceSampleRate = 44100.0
)

var fdnDelaySamples = [fdnSize]float64{1537, 1753, 1999, 2251, 2473, 2689, 2851, 3067}

var fdnHadamard = [fdnSize][fdnSize]float64{
	{1, 1, 1, 1, 1, 1, 1, 1},
	{1, -1, 1, -1, 1, -1, 1, -1},
	{1, 1, -1, -1, 1, 1, -1, -1},
	{1, -1, -1, 1, 1, -1, -1, 1},
	{1, 1, 1, 1, -1, -1, -1, -1},
	{1, -1, 1, -1, -1, 1, -1, 1},
	{1, 1, -1, -1, -1, -1, 1, 1},
	{1, -1, -1, 1, -1, 1, 1, -1},
}

// FDNReverb is a mono feedback-delay-network reverb with modulation and damping.
type FDNReverb struct {
	sampleRate      float64
	wet             float64
	dry             float64
	rt60Seconds     float64
	damp            float64
	preDelaySeconds float64
	modDepthSeconds float64
	modRateHz       float64

	lfoPhase float64

	baseDelaySamples [fdnSize]float64
	lineDelayScale   float64
	modDepthSamples  float64
	preDelaySamples  float64

	lines        [fdnSize]fdnDelayLine
	filterState  [fdnSize]float64
	feedbackGain [fdnSize]float64
	preDelayLine fdnDelayLine

	inputGain   float64
	outputGain  float64
	matrixScale float64
}

// NewFDNReverb creates an FDN reverb configured for the provided sample rate.
func NewFDNReverb(sampleRate float64) (*FDNReverb, error) {
	r := &FDNReverb{
		sampleRate:      defaultFDNSampleRate,
		wet:             defaultFDNWet,
		dry:             defaultFDNDry,
		rt60Seconds:     defaultFDNRT60Seconds,
		damp:            defaultFDNDamp,
		preDelaySeconds: defaultFDNPreDelaySec,
		modDepthSeconds: defaultFDNModDepthSec,
		modRateHz:       defaultFDNModRateHz,
	}
	for i := range r.baseDelaySamples {
		r.baseDelaySamples[i] = fdnDelaySamples[i]
	}

	scale := 1 / math.Sqrt(float64(fdnSize))
	r.inputGain = scale
	r.outputGain = scale
	r.matrixScale = scale

	err := r.SetSampleRate(sampleRate)
	if err != nil {
		return nil, err
	}

	return r, nil
}

// SetSampleRate updates sample rate.
func (r *FDNReverb) SetSampleRate(sampleRate float64) error {
	if sampleRate <= 0 || math.IsNaN(sampleRate) || math.IsInf(sampleRate, 0) {
		return fmt.Errorf("fdn reverb sample rate must be > 0: %f", sampleRate)
	}

	r.sampleRate = sampleRate
	r.lineDelayScale = sampleRate / fdnReferenceSampleRate
	r.modDepthSamples = r.modDepthSeconds * r.sampleRate
	r.preDelaySamples = r.preDelaySeconds * r.sampleRate

	return r.reconfigureDelays()
}

// SetWet sets wet gain.
func (r *FDNReverb) SetWet(v float64) error {
	if v < 0 || math.IsNaN(v) || math.IsInf(v, 0) {
		return fmt.Errorf("fdn reverb wet must be >= 0: %f", v)
	}

	r.wet = v

	return nil
}

// SetDry sets dry gain.
func (r *FDNReverb) SetDry(v float64) error {
	if v < 0 || math.IsNaN(v) || math.IsInf(v, 0) {
		return fmt.Errorf("fdn reverb dry must be >= 0: %f", v)
	}

	r.dry = v

	return nil
}

// SetRT60 sets decay time to -60 dB in seconds.
func (r *FDNReverb) SetRT60(seconds float64) error {
	if seconds <= 0 || math.IsNaN(seconds) || math.IsInf(seconds, 0) {
		return fmt.Errorf("fdn reverb RT60 must be > 0: %f", seconds)
	}

	r.rt60Seconds = seconds
	r.updateFeedbackGains()

	return nil
}

// SetDamp sets feedback damping in [0,1].
func (r *FDNReverb) SetDamp(v float64) error {
	if v < 0 || v > 1 || math.IsNaN(v) || math.IsInf(v, 0) {
		return fmt.Errorf("fdn reverb damp must be in [0,1]: %f", v)
	}

	r.damp = v

	return nil
}

// SetPreDelay sets pre-delay time in seconds.
func (r *FDNReverb) SetPreDelay(seconds float64) error {
	if seconds < 0 || math.IsNaN(seconds) || math.IsInf(seconds, 0) {
		return fmt.Errorf("fdn reverb pre-delay must be >= 0: %f", seconds)
	}

	r.preDelaySeconds = seconds
	r.preDelaySamples = r.preDelaySeconds * r.sampleRate

	return r.reconfigureDelays()
}

// SetModDepth sets delay modulation depth in seconds.
func (r *FDNReverb) SetModDepth(seconds float64) error {
	if seconds < 0 || math.IsNaN(seconds) || math.IsInf(seconds, 0) {
		return fmt.Errorf("fdn reverb mod depth must be >= 0: %f", seconds)
	}

	r.modDepthSeconds = seconds
	r.modDepthSamples = r.modDepthSeconds * r.sampleRate

	return r.reconfigureDelays()
}

// SetModRate sets modulation rate in Hz.
func (r *FDNReverb) SetModRate(hz float64) error {
	if hz < 0 || math.IsNaN(hz) || math.IsInf(hz, 0) {
		return fmt.Errorf("fdn reverb mod rate must be >= 0: %f", hz)
	}

	r.modRateHz = hz

	return nil
}

// Reset clears all delay/filter state.
func (r *FDNReverb) Reset() {
	for i := range r.lines {
		r.lines[i].reset()
		r.filterState[i] = 0
	}

	r.preDelayLine.reset()
	r.lfoPhase = 0
}

// ProcessSample processes one sample.
func (r *FDNReverb) ProcessSample(input float64) float64 {
	in := input
	if r.preDelaySamples > 0 {
		r.preDelayLine.writeSample(input)
		in = r.preDelayLine.sampleFractionalDelay(r.preDelaySamples)
	}

	var delays [fdnSize]float64
	for i := range fdnSize {
		phaseOffset := (2 * math.Pi * float64(i)) / float64(fdnSize)
		mod := 0.5 * (1 + math.Sin(r.lfoPhase+phaseOffset))
		delay := r.baseDelaySamples[i]*r.lineDelayScale + r.modDepthSamples*mod
		delays[i] = r.lines[i].sampleFractionalDelay(delay)
	}

	r.lfoPhase += 2 * math.Pi * r.modRateHz / r.sampleRate
	if r.lfoPhase >= 2*math.Pi {
		r.lfoPhase -= 2 * math.Pi
	}

	for i := range fdnSize {
		feedback := 0.0
		for j := range fdnSize {
			feedback += fdnHadamard[i][j] * delays[j]
		}

		feedback *= r.matrixScale
		filtered := feedback*(1-r.damp) + r.filterState[i]*r.damp
		r.filterState[i] = filtered
		writeSample := in*r.inputGain + filtered*r.feedbackGain[i]
		r.lines[i].writeSample(writeSample)
	}

	out := 0.0
	for i := range fdnSize {
		out += delays[i]
	}

	out *= r.outputGain

	return input*r.dry + out*r.wet
}

// ProcessInPlace applies reverb to buf in place.
func (r *FDNReverb) ProcessInPlace(buf []float64) {
	for i := range buf {
		buf[i] = r.ProcessSample(buf[i])
	}
}

// SampleRate returns sample rate in Hz.
func (r *FDNReverb) SampleRate() float64 { return r.sampleRate }

// Wet returns wet gain.
func (r *FDNReverb) Wet() float64 { return r.wet }

// Dry returns dry gain.
func (r *FDNReverb) Dry() float64 { return r.dry }

// RT60 returns decay time to -60 dB in seconds.
func (r *FDNReverb) RT60() float64 { return r.rt60Seconds }

// Damp returns damping amount in [0,1].
func (r *FDNReverb) Damp() float64 { return r.damp }

// PreDelay returns pre-delay time in seconds.
func (r *FDNReverb) PreDelay() float64 { return r.preDelaySeconds }

// ModDepth returns modulation depth in seconds.
func (r *FDNReverb) ModDepth() float64 { return r.modDepthSeconds }

// ModRate returns modulation rate in Hz.
func (r *FDNReverb) ModRate() float64 { return r.modRateHz }

func (r *FDNReverb) reconfigureDelays() error {
	if r.sampleRate <= 0 {
		return fmt.Errorf("fdn reverb sample rate must be > 0: %f", r.sampleRate)
	}

	if r.modDepthSeconds < 0 {
		return fmt.Errorf("fdn reverb mod depth must be >= 0: %f", r.modDepthSeconds)
	}

	if r.preDelaySeconds < 0 {
		return fmt.Errorf("fdn reverb pre-delay must be >= 0: %f", r.preDelaySeconds)
	}

	for i := range fdnSize {
		maxDelay := max(int(math.Ceil(r.baseDelaySamples[i]*r.lineDelayScale+r.modDepthSamples))+3, minFDNDelayBufferSize)

		r.lines[i].resize(maxDelay)
		r.filterState[i] = 0
	}

	preDelayMax := max(int(math.Ceil(r.preDelaySamples))+3, minFDNDelayBufferSize)

	r.preDelayLine.resize(preDelayMax)

	r.updateFeedbackGains()

	return nil
}

func (r *FDNReverb) updateFeedbackGains() {
	if r.sampleRate <= 0 || r.rt60Seconds <= 0 {
		return
	}

	for i := range fdnSize {
		delaySeconds := (r.baseDelaySamples[i] * r.lineDelayScale) / r.sampleRate
		r.feedbackGain[i] = math.Pow(10, -3*delaySeconds/r.rt60Seconds)
	}
}

type fdnDelayLine struct {
	buffer   []float64
	writePos int
	maxDelay int
}

func (d *fdnDelayLine) resize(maxDelay int) {
	if maxDelay < minFDNDelayBufferSize {
		maxDelay = minFDNDelayBufferSize
	}

	if maxDelay == len(d.buffer) {
		return
	}

	d.buffer = make([]float64, maxDelay)
	d.writePos = 0
	d.maxDelay = maxDelay - 3
}

func (d *fdnDelayLine) reset() {
	for i := range d.buffer {
		d.buffer[i] = 0
	}

	d.writePos = 0
}

func (d *fdnDelayLine) writeSample(x float64) {
	if len(d.buffer) == 0 {
		return
	}

	d.buffer[d.writePos] = x

	d.writePos++
	if d.writePos >= len(d.buffer) {
		d.writePos = 0
	}
}

func (d *fdnDelayLine) sampleFractionalDelay(delay float64) float64 {
	if len(d.buffer) == 0 {
		return 0
	}

	if delay < 0 {
		delay = 0
	}

	maxDelay := float64(d.maxDelay)
	if delay > maxDelay {
		delay = maxDelay
	}

	p := int(math.Floor(delay))
	t := delay - float64(p)

	xm1 := d.sampleDelayInt(maxInt(0, p-1))
	x0 := d.sampleDelayInt(p)
	x1 := d.sampleDelayInt(p + 1)
	x2 := d.sampleDelayInt(p + 2)

	return hermite4(t, xm1, x0, x1, x2)
}

func (d *fdnDelayLine) sampleDelayInt(delay int) float64 {
	if delay < 0 || delay >= len(d.buffer) {
		return 0
	}

	idx := d.writePos - 1 - delay
	if idx < 0 {
		idx += len(d.buffer)
	}

	return d.buffer[idx]
}
