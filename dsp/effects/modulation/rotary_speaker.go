package modulation

import (
	"fmt"
	"math"
)

const (
	defaultRotarySampleRate = 44100.0
	minRotarySampleRate     = 8000.0
	minRotaryMix            = 0.0
	maxRotaryMix            = 1.0
	minRotaryStereoWidth    = 0.0
	maxRotaryStereoWidth    = 1.0
	minRotaryCrossoverHz    = 50.0
	maxRotaryCrossoverHz    = 12000.0
	minRotarySpeedHz        = 0.0
	maxRotarySpeedHz        = 20.0
	minRotaryTauSeconds     = 0.001
	maxRotaryTauSeconds     = 20.0
	minRotaryRadiusMeters   = 0.001
	maxRotaryRadiusMeters   = 2.0
	minRotaryMicSpacing     = 0.0
	maxRotaryMicSpacing     = 5.0
	defaultRotarySoundSpeed = 343.0
)

// SpeedMode selects between the slow (chorale) and fast (tremolo) rotation speeds.
type SpeedMode int

const (
	// SpeedModeChorale is the slow Leslie rotation speed.
	SpeedModeChorale SpeedMode = iota
	// SpeedModeTremolo is the fast Leslie rotation speed.
	SpeedModeTremolo
)

// rotorState tracks the angular position and velocity of a single rotor cabinet
// (horn or drum).
type rotorState struct {
	angle           float64
	omega           float64
	choraleHz       float64
	tremoloHz       float64
	targetHz        float64
	accelTauSeconds float64
	decelTauSeconds float64
}

func (r *rotorState) reset() {
	r.angle = 0
	r.omega = 0
	r.targetHz = r.choraleHz
}

// advance steps the rotor forward by one sample, ramping omega toward targetHz
// using a first-order lag with separate acceleration/deceleration time constants.
func (r *rotorState) advance(sampleRate float64) {
	targetOmega := 2.0 * math.Pi * r.targetHz

	tau := r.accelTauSeconds
	if math.Abs(targetOmega) < math.Abs(r.omega) {
		tau = r.decelTauSeconds
	}

	if tau < minRotaryTauSeconds {
		tau = minRotaryTauSeconds
	}

	alpha := 1.0 - math.Exp(-1.0/(tau*sampleRate))
	r.omega += alpha * (targetOmega - r.omega)
	r.angle += r.omega / sampleRate

	r.angle = math.Mod(r.angle, 2.0*math.Pi)
	if r.angle < 0 {
		r.angle += 2.0 * math.Pi
	}
}

// onePoleLP is a single-pole IIR low-pass filter.
type onePoleLP struct {
	a  float64
	z1 float64
}

func (f *onePoleLP) setCutoff(sampleRate, cutoffHz float64) {
	if cutoffHz <= 0 {
		f.a = 0
		return
	}

	f.a = math.Exp(-2.0 * math.Pi * cutoffHz / sampleRate)
}

func (f *onePoleLP) reset() {
	f.z1 = 0
}

func (f *onePoleLP) processSample(x float64) float64 {
	y := (1.0-f.a)*x + f.a*f.z1
	f.z1 = y

	return y
}

// onePoleHP is a single-pole IIR high-pass filter derived from the low-pass complement.
type onePoleHP struct {
	lp onePoleLP
}

func (f *onePoleHP) setCutoff(sampleRate, cutoffHz float64) {
	f.lp.setCutoff(sampleRate, cutoffHz)
}

func (f *onePoleHP) reset() {
	f.lp.reset()
}

func (f *onePoleHP) processSample(x float64) float64 {
	return x - f.lp.processSample(x)
}

// RotarySpeaker is a Leslie-style rotary speaker effect. It accepts mono input
// and produces stereo output. The cabinet is modelled with two independently
// spinning rotors: a high-frequency horn and a low-frequency drum rotor. Both
// rotors contribute Doppler pitch modulation, amplitude modulation, and stereo
// panning based on physical geometry.
//
// Two speed modes are available:
//   - SpeedModeChorale: slow rotation (~0.8 Hz horn, ~0.6 Hz drum)
//   - SpeedModeTremolo: fast rotation (~6.5 Hz horn, ~5.0 Hz drum)
//
// Acceleration and deceleration between modes use independent first-order time
// constants that mimic the mechanical inertia of a real Leslie cabinet.
type RotarySpeaker struct {
	sampleRate float64

	mix         float64
	stereoWidth float64
	drive       float64
	mode        SpeedMode

	crossoverHz      float64
	soundSpeedMeters float64

	hornRadiusMeters float64
	drumRadiusMeters float64
	micSpacingMeters float64

	horn rotorState
	drum rotorState

	lowLP  onePoleLP
	highHP onePoleHP

	lowDelay  []float64
	highDelay []float64
	lowWrite  int
	highWrite int

	maxDelaySamples int

	hornReflectionGain float64
	drumReflectionGain float64
}

// NewRotarySpeaker creates a RotarySpeaker with tuned musical defaults.
func NewRotarySpeaker() (*RotarySpeaker, error) {
	r := &RotarySpeaker{
		sampleRate:         defaultRotarySampleRate,
		mix:                1.0,
		stereoWidth:        1.0,
		drive:              1.0,
		mode:               SpeedModeChorale,
		crossoverHz:        800.0,
		soundSpeedMeters:   defaultRotarySoundSpeed,
		hornRadiusMeters:   0.18,
		drumRadiusMeters:   0.14,
		micSpacingMeters:   0.30,
		hornReflectionGain: 0.20,
		drumReflectionGain: 0.10,
		maxDelaySamples:    256,
	}
	r.horn = rotorState{
		choraleHz:       0.8,
		tremoloHz:       6.5,
		targetHz:        0.8,
		accelTauSeconds: 0.35,
		decelTauSeconds: 1.20,
	}

	r.drum = rotorState{
		choraleHz:       0.6,
		tremoloHz:       5.0,
		targetHz:        0.6,
		accelTauSeconds: 0.80,
		decelTauSeconds: 1.80,
	}

	err := r.reconfigure()
	if err != nil {
		return nil, err
	}

	r.Reset()

	return r, nil
}

// Reset clears all delay lines, filter states, and rotor positions. Two
// subsequent calls with the same input will produce identical output.
func (r *RotarySpeaker) Reset() {
	r.horn.reset()
	r.drum.reset()
	r.setModeTargets()

	r.lowLP.reset()
	r.highHP.reset()

	for i := range r.lowDelay {
		r.lowDelay[i] = 0
	}

	for i := range r.highDelay {
		r.highDelay[i] = 0
	}

	r.lowWrite = 0
	r.highWrite = 0
}

// SetSampleRate updates the sample rate and reconfigures internal filters and
// delay lines.
func (r *RotarySpeaker) SetSampleRate(sampleRate float64) error {
	if sampleRate < minRotarySampleRate || math.IsNaN(sampleRate) || math.IsInf(sampleRate, 0) {
		return fmt.Errorf("rotary speaker sample rate must be >= %g Hz: %f", minRotarySampleRate, sampleRate)
	}

	r.sampleRate = sampleRate

	return r.reconfigure()
}

// SetMix sets the wet/dry ratio in [0, 1].
func (r *RotarySpeaker) SetMix(mix float64) error {
	if mix < minRotaryMix || mix > maxRotaryMix || math.IsNaN(mix) || math.IsInf(mix, 0) {
		return fmt.Errorf("rotary speaker mix must be in [%g, %g]: %f", minRotaryMix, maxRotaryMix, mix)
	}

	r.mix = mix

	return nil
}

// SetStereoWidth sets the stereo spread in [0, 1]. At 0 both output channels
// are identical; at 1 the full mic-spacing geometry is used.
func (r *RotarySpeaker) SetStereoWidth(width float64) error {
	if width < minRotaryStereoWidth || width > maxRotaryStereoWidth || math.IsNaN(width) || math.IsInf(width, 0) {
		return fmt.Errorf("rotary speaker stereo width must be in [%g, %g]: %f", minRotaryStereoWidth, maxRotaryStereoWidth, width)
	}

	r.stereoWidth = width

	return nil
}

// SetDrive sets the pre-distortion drive gain. Values > 1 push the tanh soft
// clipper harder.
func (r *RotarySpeaker) SetDrive(drive float64) error {
	if drive <= 0 || math.IsNaN(drive) || math.IsInf(drive, 0) {
		return fmt.Errorf("rotary speaker drive must be > 0 and finite: %f", drive)
	}

	r.drive = drive

	return nil
}

// SetSpeedMode switches between SpeedModeChorale and SpeedModeTremolo. Rotor
// velocity changes gradually according to the acceleration/deceleration time
// constants.
func (r *RotarySpeaker) SetSpeedMode(mode SpeedMode) error {
	if mode != SpeedModeChorale && mode != SpeedModeTremolo {
		return fmt.Errorf("rotary speaker: invalid speed mode %d", int(mode))
	}

	r.mode = mode
	r.setModeTargets()

	return nil
}

// SetHornSpeed sets the chorale and tremolo rotation frequencies (Hz) for the
// high-frequency horn rotor.
func (r *RotarySpeaker) SetHornSpeed(choraleHz, tremoloHz float64) error {
	if choraleHz < minRotarySpeedHz || choraleHz > maxRotarySpeedHz {
		return fmt.Errorf("horn chorale speed must be in [%g, %g] Hz: %f", minRotarySpeedHz, maxRotarySpeedHz, choraleHz)
	}

	if tremoloHz < minRotarySpeedHz || tremoloHz > maxRotarySpeedHz {
		return fmt.Errorf("horn tremolo speed must be in [%g, %g] Hz: %f", minRotarySpeedHz, maxRotarySpeedHz, tremoloHz)
	}

	r.horn.choraleHz = choraleHz
	r.horn.tremoloHz = tremoloHz
	r.setModeTargets()

	return nil
}

// SetDrumSpeed sets the chorale and tremolo rotation frequencies (Hz) for the
// low-frequency drum rotor.
func (r *RotarySpeaker) SetDrumSpeed(choraleHz, tremoloHz float64) error {
	if choraleHz < minRotarySpeedHz || choraleHz > maxRotarySpeedHz {
		return fmt.Errorf("drum chorale speed must be in [%g, %g] Hz: %f", minRotarySpeedHz, maxRotarySpeedHz, choraleHz)
	}

	if tremoloHz < minRotarySpeedHz || tremoloHz > maxRotarySpeedHz {
		return fmt.Errorf("drum tremolo speed must be in [%g, %g] Hz: %f", minRotarySpeedHz, maxRotarySpeedHz, tremoloHz)
	}

	r.drum.choraleHz = choraleHz
	r.drum.tremoloHz = tremoloHz
	r.setModeTargets()

	return nil
}

// SetHornTimeConstants sets the first-order acceleration (accel) and
// deceleration (decel) time constants in seconds for the horn rotor.
func (r *RotarySpeaker) SetHornTimeConstants(accelTauSeconds, decelTauSeconds float64) error {
	if accelTauSeconds < minRotaryTauSeconds || accelTauSeconds > maxRotaryTauSeconds {
		return fmt.Errorf("horn accel tau must be in [%g, %g] s: %f", minRotaryTauSeconds, maxRotaryTauSeconds, accelTauSeconds)
	}

	if decelTauSeconds < minRotaryTauSeconds || decelTauSeconds > maxRotaryTauSeconds {
		return fmt.Errorf("horn decel tau must be in [%g, %g] s: %f", minRotaryTauSeconds, maxRotaryTauSeconds, decelTauSeconds)
	}

	r.horn.accelTauSeconds = accelTauSeconds
	r.horn.decelTauSeconds = decelTauSeconds

	return nil
}

// SetDrumTimeConstants sets the first-order acceleration (accel) and
// deceleration (decel) time constants in seconds for the drum rotor.
func (r *RotarySpeaker) SetDrumTimeConstants(accelTauSeconds, decelTauSeconds float64) error {
	if accelTauSeconds < minRotaryTauSeconds || accelTauSeconds > maxRotaryTauSeconds {
		return fmt.Errorf("drum accel tau must be in [%g, %g] s: %f", minRotaryTauSeconds, maxRotaryTauSeconds, accelTauSeconds)
	}

	if decelTauSeconds < minRotaryTauSeconds || decelTauSeconds > maxRotaryTauSeconds {
		return fmt.Errorf("drum decel tau must be in [%g, %g] s: %f", minRotaryTauSeconds, maxRotaryTauSeconds, decelTauSeconds)
	}

	r.drum.accelTauSeconds = accelTauSeconds
	r.drum.decelTauSeconds = decelTauSeconds

	return nil
}

// SetCrossoverHz sets the crossover frequency (Hz) between the drum (low) and
// horn (high) signal paths.
func (r *RotarySpeaker) SetCrossoverHz(crossoverHz float64) error {
	if crossoverHz < minRotaryCrossoverHz || crossoverHz > maxRotaryCrossoverHz {
		return fmt.Errorf("rotary speaker crossover must be in [%g, %g] Hz: %f", minRotaryCrossoverHz, maxRotaryCrossoverHz, crossoverHz)
	}

	r.crossoverHz = crossoverHz

	return r.reconfigure()
}

// SetGeometry sets the physical cabinet geometry used to compute Doppler delay
// and directivity patterns. All values in metres.
func (r *RotarySpeaker) SetGeometry(hornRadiusMeters, drumRadiusMeters, micSpacingMeters float64) error {
	if hornRadiusMeters < minRotaryRadiusMeters || hornRadiusMeters > maxRotaryRadiusMeters {
		return fmt.Errorf("horn radius must be in [%g, %g] m: %f", minRotaryRadiusMeters, maxRotaryRadiusMeters, hornRadiusMeters)
	}

	if drumRadiusMeters < minRotaryRadiusMeters || drumRadiusMeters > maxRotaryRadiusMeters {
		return fmt.Errorf("drum radius must be in [%g, %g] m: %f", minRotaryRadiusMeters, maxRotaryRadiusMeters, drumRadiusMeters)
	}

	if micSpacingMeters < minRotaryMicSpacing || micSpacingMeters > maxRotaryMicSpacing {
		return fmt.Errorf("mic spacing must be in [%g, %g] m: %f", minRotaryMicSpacing, maxRotaryMicSpacing, micSpacingMeters)
	}

	r.hornRadiusMeters = hornRadiusMeters
	r.drumRadiusMeters = drumRadiusMeters
	r.micSpacingMeters = micSpacingMeters

	return r.reconfigure()
}

// SampleRate returns the current sample rate in Hz.
func (r *RotarySpeaker) SampleRate() float64 { return r.sampleRate }

// Mix returns the current wet/dry ratio in [0, 1].
func (r *RotarySpeaker) Mix() float64 { return r.mix }

// StereoWidth returns the current stereo spread in [0, 1].
func (r *RotarySpeaker) StereoWidth() float64 { return r.stereoWidth }

// Drive returns the current pre-distortion gain.
func (r *RotarySpeaker) Drive() float64 { return r.drive }

// ProcessSample processes one mono input sample and returns a stereo pair
// (left, right).
func (r *RotarySpeaker) ProcessSample(x float64) (float64, float64) {
	dryL, dryR := x, x

	x = rotarySoftClip(x * r.drive)

	low := r.lowLP.processSample(x)
	high := r.highHP.processSample(x)

	r.horn.advance(r.sampleRate)
	r.drum.advance(r.sampleRate)

	r.writeLow(low)
	r.writeHigh(high)

	drumL, drumR := r.renderDrum()
	hornL, hornR := r.renderHorn()

	wetL := drumL + hornL
	wetR := drumR + hornR

	outL := dryL*(1.0-r.mix) + wetL*r.mix
	outR := dryR*(1.0-r.mix) + wetR*r.mix

	return outL, outR
}

// ProcessStereoInPlace folds stereo input to mono, processes it through the
// rotary cabinet, and writes stereo output back into the provided slices. The
// shorter slice length is used when lengths differ.
func (r *RotarySpeaker) ProcessStereoInPlace(left, right []float64) {
	n := min(len(left), len(right))
	for i := range n {
		x := 0.5 * (left[i] + right[i])
		l, rr := r.ProcessSample(x)
		left[i] = l
		right[i] = rr
	}
}

func (r *RotarySpeaker) setModeTargets() {
	switch r.mode {
	case SpeedModeChorale:
		r.horn.targetHz = r.horn.choraleHz
		r.drum.targetHz = r.drum.choraleHz
	case SpeedModeTremolo:
		r.horn.targetHz = r.horn.tremoloHz
		r.drum.targetHz = r.drum.tremoloHz
	}
}

func (r *RotarySpeaker) reconfigure() error {
	if r.sampleRate < minRotarySampleRate {
		return fmt.Errorf("rotary speaker sample rate must be >= %g Hz: %f", minRotarySampleRate, r.sampleRate)
	}

	if r.soundSpeedMeters <= 0 {
		return fmt.Errorf("rotary speaker sound speed must be > 0: %f", r.soundSpeedMeters)
	}

	r.lowLP.setCutoff(r.sampleRate, r.crossoverHz)
	r.highHP.setCutoff(r.sampleRate, r.crossoverHz)

	maxPathMeters := 1.5 + math.Max(r.hornRadiusMeters, r.drumRadiusMeters) + 0.5*r.micSpacingMeters
	maxDelaySeconds := maxPathMeters/r.soundSpeedMeters + 0.02
	r.maxDelaySamples = max(int(math.Ceil(maxDelaySeconds*r.sampleRate))+8, 64)
	r.lowDelay = make([]float64, r.maxDelaySamples)
	r.highDelay = make([]float64, r.maxDelaySamples)
	r.lowWrite = 0
	r.highWrite = 0

	return nil
}

func (r *RotarySpeaker) writeLow(x float64) {
	r.lowDelay[r.lowWrite] = x

	r.lowWrite++
	if r.lowWrite >= len(r.lowDelay) {
		r.lowWrite = 0
	}
}

func (r *RotarySpeaker) writeHigh(x float64) {
	r.highDelay[r.highWrite] = x

	r.highWrite++
	if r.highWrite >= len(r.highDelay) {
		r.highWrite = 0
	}
}

func (r *RotarySpeaker) renderHorn() (float64, float64) {
	return r.renderBand(
		r.highDelay,
		r.highWrite,
		r.horn.angle,
		r.hornRadiusMeters,
		0.85, // stronger directivity
		0.20, // second harmonic directivity shaping
		r.hornReflectionGain,
	)
}

func (r *RotarySpeaker) renderDrum() (float64, float64) {
	return r.renderBand(
		r.lowDelay,
		r.lowWrite,
		r.drum.angle,
		r.drumRadiusMeters*0.65, // weaker Doppler than horn
		0.35,                    // gentler directivity
		0.08,
		r.drumReflectionGain,
	)
}

func (r *RotarySpeaker) renderBand(
	delay []float64,
	writeIndex int,
	angle float64,
	radiusMeters float64,
	g1 float64,
	g2 float64,
	reflectionGain float64,
) (float64, float64) {
	micX := 0.5 * r.micSpacingMeters * r.stereoWidth

	leftGain, leftDelay := r.pathGainAndDelay(angle, radiusMeters, -micX, 1.0, g1, g2)
	rightGain, rightDelay := r.pathGainAndDelay(angle, radiusMeters, micX, 1.0, g1, g2)

	left := leftGain * rotarySampleFractionalDelay(delay, writeIndex, leftDelay*r.sampleRate)
	right := rightGain * rotarySampleFractionalDelay(delay, writeIndex, rightDelay*r.sampleRate)

	// Simple reflected contribution from the far side.
	refAngle := angle + math.Pi
	refLeftGain, refLeftDelay := r.pathGainAndDelay(refAngle, radiusMeters*0.6, -micX, 1.4, 0.25*g1, 0.0)
	refRightGain, refRightDelay := r.pathGainAndDelay(refAngle, radiusMeters*0.6, micX, 1.4, 0.25*g1, 0.0)

	left += reflectionGain * refLeftGain * rotarySampleFractionalDelay(delay, writeIndex, refLeftDelay*r.sampleRate+0.75)
	right += reflectionGain * refRightGain * rotarySampleFractionalDelay(delay, writeIndex, refRightDelay*r.sampleRate+0.75)

	return left, right
}

func (r *RotarySpeaker) pathGainAndDelay(
	angle float64,
	radiusMeters float64,
	micX float64,
	micY float64,
	g1 float64,
	g2 float64,
) (float64, float64) {
	srcX := radiusMeters * math.Cos(angle)
	srcY := radiusMeters * math.Sin(angle)

	dx := micX - srcX
	dy := micY - srcY
	distanceMeters := math.Sqrt(dx*dx + dy*dy)

	phi := math.Atan2(srcY, srcX)

	gain := 0.65 + g1*math.Cos(phi) + g2*math.Cos(2.0*phi)
	if gain < 0.05 {
		gain = 0.05
	}

	delaySeconds := distanceMeters / r.soundSpeedMeters

	return gain, delaySeconds
}

// rotarySampleFractionalDelay reads from a circular delay buffer at a
// fractional position using 4-point Hermite interpolation.
func rotarySampleFractionalDelay(delay []float64, writeIndex int, delaySamples float64) float64 {
	if delaySamples < 0 {
		delaySamples = 0
	}

	maxRead := float64(len(delay) - 4)
	if delaySamples > maxRead {
		delaySamples = maxRead
	}

	readPos := float64(writeIndex) - delaySamples
	for readPos < 0 {
		readPos += float64(len(delay))
	}

	for readPos >= float64(len(delay)) {
		readPos -= float64(len(delay))
	}

	i1 := int(math.Floor(readPos))
	frac := readPos - float64(i1)

	i0 := rotaryWrapIndex(i1-1, len(delay))
	i2 := rotaryWrapIndex(i1+1, len(delay))
	i3 := rotaryWrapIndex(i1+2, len(delay))
	i1 = rotaryWrapIndex(i1, len(delay))

	return hermite4(frac, delay[i0], delay[i1], delay[i2], delay[i3])
}

func rotaryWrapIndex(i, n int) int {
	for i < 0 {
		i += n
	}

	for i >= n {
		i -= n
	}

	return i
}

// rotarySoftClip applies tanh saturation to the input signal.
func rotarySoftClip(x float64) float64 {
	return math.Tanh(x)
}
