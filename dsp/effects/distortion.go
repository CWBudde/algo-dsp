package effects

import (
	"fmt"
	"math"
)

const (
	defaultDistortionDrive       = 1.0
	defaultDistortionMix         = 1.0
	defaultDistortionOutputLevel = 1.0
	defaultDistortionClipLevel   = 1.0
	defaultDistortionShape       = 0.5
	defaultDistortionBias        = 0.0
	defaultChebyshevOrder        = 3
	defaultChebyshevGainLevel    = 1.0

	minDistortionDrive       = 0.01
	maxDistortionDrive       = 20.0
	minDistortionOutputLevel = 0.0
	maxDistortionOutputLevel = 4.0
	minDistortionClipLevel   = 0.05
	maxDistortionClipLevel   = 1.0
	minDistortionShape       = 0.0
	maxDistortionShape       = 1.0
	minChebyshevOrder        = 1
	maxChebyshevOrder        = 16
	minChebyshevGainLevel    = 0.0
	maxChebyshevGainLevel    = 4.0
	dcBypassPole             = 0.995
)

// DistortionMode selects the transfer function used by Distortion.
type DistortionMode int

const (
	DistortionModeSoftClip DistortionMode = iota
	DistortionModeHardClip
	DistortionModeTanh
	DistortionModeWaveshaper1
	DistortionModeWaveshaper2
	DistortionModeWaveshaper3
	DistortionModeWaveshaper4
	DistortionModeWaveshaper5
	DistortionModeWaveshaper6
	DistortionModeWaveshaper7
	DistortionModeWaveshaper8
	DistortionModeSaturate
	DistortionModeSaturate2
	DistortionModeSoftSat
	DistortionModeChebyshev
)

// DistortionApproxMode selects exact vs polynomial approximation paths.
type DistortionApproxMode int

const (
	DistortionApproxExact DistortionApproxMode = iota
	DistortionApproxPolynomial
)

// ChebyshevHarmonicMode constrains order parity for Chebyshev shaping.
type ChebyshevHarmonicMode int

const (
	ChebyshevHarmonicAll ChebyshevHarmonicMode = iota
	ChebyshevHarmonicOdd
	ChebyshevHarmonicEven
)

// DistortionOption mutates construction-time parameters.
type DistortionOption func(*distortionConfig) error

type distortionConfig struct {
	mode              DistortionMode
	approxMode        DistortionApproxMode
	drive             float64
	mix               float64
	outputLevel       float64
	clipLevel         float64
	shape             float64
	bias              float64
	chebyshevOrder    int
	chebyshevMode     ChebyshevHarmonicMode
	chebyshevInvert   bool
	chebyshevGain     float64
	chebyshevDCBypass bool
	chebyshevWeights  [16]float64
}

func defaultDistortionConfig() distortionConfig {
	return distortionConfig{
		mode:              DistortionModeSoftClip,
		approxMode:        DistortionApproxExact,
		drive:             defaultDistortionDrive,
		mix:               defaultDistortionMix,
		outputLevel:       defaultDistortionOutputLevel,
		clipLevel:         defaultDistortionClipLevel,
		shape:             defaultDistortionShape,
		bias:              defaultDistortionBias,
		chebyshevOrder:    defaultChebyshevOrder,
		chebyshevMode:     ChebyshevHarmonicAll,
		chebyshevInvert:   false,
		chebyshevGain:     defaultChebyshevGainLevel,
		chebyshevDCBypass: false,
	}
}

// WithDistortionMode selects the distortion transfer mode.
func WithDistortionMode(mode DistortionMode) DistortionOption {
	return func(cfg *distortionConfig) error {
		if !validDistortionMode(mode) {
			return fmt.Errorf("distortion mode is invalid: %d", mode)
		}

		cfg.mode = mode

		return validateChebyshevParity(cfg.chebyshevOrder, cfg.chebyshevMode)
	}
}

// WithDistortionApproxMode selects exact or polynomial approximation processing.
func WithDistortionApproxMode(mode DistortionApproxMode) DistortionOption {
	return func(cfg *distortionConfig) error {
		if !validApproxMode(mode) {
			return fmt.Errorf("distortion approximation mode is invalid: %d", mode)
		}

		cfg.approxMode = mode

		return nil
	}
}

// WithDistortionDrive sets input drive in [0.01, 20].
func WithDistortionDrive(drive float64) DistortionOption {
	return func(cfg *distortionConfig) error {
		if drive < minDistortionDrive || drive > maxDistortionDrive || math.IsNaN(drive) || math.IsInf(drive, 0) {
			return fmt.Errorf("distortion drive must be in [%g, %g]: %f", minDistortionDrive, maxDistortionDrive, drive)
		}

		cfg.drive = drive

		return nil
	}
}

// WithDistortionMix sets dry/wet mix in [0, 1].
func WithDistortionMix(mix float64) DistortionOption {
	return func(cfg *distortionConfig) error {
		if mix < 0 || mix > 1 || math.IsNaN(mix) || math.IsInf(mix, 0) {
			return fmt.Errorf("distortion mix must be in [0, 1]: %f", mix)
		}

		cfg.mix = mix

		return nil
	}
}

// WithDistortionOutputLevel sets post-shape output level in [0, 4].
func WithDistortionOutputLevel(level float64) DistortionOption {
	return func(cfg *distortionConfig) error {
		if level < minDistortionOutputLevel || level > maxDistortionOutputLevel || math.IsNaN(level) || math.IsInf(level, 0) {
			return fmt.Errorf("distortion output level must be in [%g, %g]: %f",
				minDistortionOutputLevel, maxDistortionOutputLevel, level)
		}

		cfg.outputLevel = level

		return nil
	}
}

// WithDistortionClipLevel sets hard-clip threshold in [0.05, 1.0].
func WithDistortionClipLevel(level float64) DistortionOption {
	return func(cfg *distortionConfig) error {
		if level < minDistortionClipLevel || level > maxDistortionClipLevel || math.IsNaN(level) || math.IsInf(level, 0) {
			return fmt.Errorf("distortion clip level must be in [%g, %g]: %f",
				minDistortionClipLevel, maxDistortionClipLevel, level)
		}

		cfg.clipLevel = level

		return nil
	}
}

// WithDistortionShape sets formula shaper parameter in [0, 1].
func WithDistortionShape(shape float64) DistortionOption {
	return func(cfg *distortionConfig) error {
		if shape < minDistortionShape || shape > maxDistortionShape || math.IsNaN(shape) || math.IsInf(shape, 0) {
			return fmt.Errorf("distortion shape must be in [%g, %g]: %f", minDistortionShape, maxDistortionShape, shape)
		}

		cfg.shape = shape

		return nil
	}
}

// WithDistortionBias sets a pre-drive bias in [-1, 1].
func WithDistortionBias(bias float64) DistortionOption {
	return func(cfg *distortionConfig) error {
		if bias < -1 || bias > 1 || math.IsNaN(bias) || math.IsInf(bias, 0) {
			return fmt.Errorf("distortion bias must be in [-1, 1]: %f", bias)
		}

		cfg.bias = bias

		return nil
	}
}

// WithChebyshevOrder sets Chebyshev order in [1, 16].
func WithChebyshevOrder(order int) DistortionOption {
	return func(cfg *distortionConfig) error {
		if order < minChebyshevOrder || order > maxChebyshevOrder {
			return fmt.Errorf("chebyshev order must be in [%d, %d]: %d", minChebyshevOrder, maxChebyshevOrder, order)
		}

		cfg.chebyshevOrder = order

		return validateChebyshevParity(cfg.chebyshevOrder, cfg.chebyshevMode)
	}
}

// WithChebyshevHarmonicMode constrains Chebyshev order parity.
func WithChebyshevHarmonicMode(mode ChebyshevHarmonicMode) DistortionOption {
	return func(cfg *distortionConfig) error {
		if !validChebyshevHarmonicMode(mode) {
			return fmt.Errorf("chebyshev harmonic mode is invalid: %d", mode)
		}

		cfg.chebyshevMode = mode

		return validateChebyshevParity(cfg.chebyshevOrder, cfg.chebyshevMode)
	}
}

// WithChebyshevInvert toggles polarity inversion for Chebyshev output.
func WithChebyshevInvert(invert bool) DistortionOption {
	return func(cfg *distortionConfig) error {
		cfg.chebyshevInvert = invert
		return nil
	}
}

// WithChebyshevGainLevel sets Chebyshev output gain in [0, 4].
func WithChebyshevGainLevel(gain float64) DistortionOption {
	return func(cfg *distortionConfig) error {
		if gain < minChebyshevGainLevel || gain > maxChebyshevGainLevel || math.IsNaN(gain) || math.IsInf(gain, 0) {
			return fmt.Errorf("chebyshev gain level must be in [%g, %g]: %f",
				minChebyshevGainLevel, maxChebyshevGainLevel, gain)
		}

		cfg.chebyshevGain = gain

		return nil
	}
}

// WithChebyshevDCBypass enables simple DC-blocking at Chebyshev output.
func WithChebyshevDCBypass(enabled bool) DistortionOption {
	return func(cfg *distortionConfig) error {
		cfg.chebyshevDCBypass = enabled
		return nil
	}
}

// WithChebyshevWeights sets per-harmonic blend weights w1..wN for the Chebyshev
// waveshaper. The output becomes sum(weights[k]*T_{k+1}(x), k=0..N-1)*gain.
// When all weights are zero (the default), the legacy T_N-only path is used.
// At most 16 weights may be provided; all values must be finite.
func WithChebyshevWeights(weights []float64) DistortionOption {
	return func(cfg *distortionConfig) error {
		if len(weights) > 16 {
			return fmt.Errorf("chebyshev weights length must be <= 16: %d", len(weights))
		}

		for i, w := range weights {
			if !isFinite(w) {
				return fmt.Errorf("chebyshev weight[%d] must be finite: %v", i, w)
			}
		}

		cfg.chebyshevWeights = [16]float64{}
		copy(cfg.chebyshevWeights[:], weights)

		return nil
	}
}

// Distortion is a configurable waveshaper that supports baseline clipping,
// legacy-style formula shapers, and a Chebyshev harmonic core.
type Distortion struct {
	sampleRate float64

	mode        DistortionMode
	approxMode  DistortionApproxMode
	drive       float64
	mix         float64
	outputLevel float64
	clipLevel   float64
	shape       float64
	bias        float64

	chebyshevOrder    int
	chebyshevMode     ChebyshevHarmonicMode
	chebyshevInvert   bool
	chebyshevGain     float64
	chebyshevDCBypass bool
	chebyshevWeights  [16]float64

	dcPrevIn  float64
	dcPrevOut float64
}

// NewDistortion creates a distortion processor with validated options.
func NewDistortion(sampleRate float64, opts ...DistortionOption) (*Distortion, error) {
	if sampleRate <= 0 || math.IsNaN(sampleRate) || math.IsInf(sampleRate, 0) {
		return nil, fmt.Errorf("distortion sample rate must be > 0 and finite: %f", sampleRate)
	}

	cfg := defaultDistortionConfig()

	for _, opt := range opts {
		if opt == nil {
			continue
		}

		err := opt(&cfg)
		if err != nil {
			return nil, err
		}
	}

	d := &Distortion{
		sampleRate:        sampleRate,
		mode:              cfg.mode,
		approxMode:        cfg.approxMode,
		drive:             cfg.drive,
		mix:               cfg.mix,
		outputLevel:       cfg.outputLevel,
		clipLevel:         cfg.clipLevel,
		shape:             cfg.shape,
		bias:              cfg.bias,
		chebyshevOrder:    cfg.chebyshevOrder,
		chebyshevMode:     cfg.chebyshevMode,
		chebyshevInvert:   cfg.chebyshevInvert,
		chebyshevGain:     cfg.chebyshevGain,
		chebyshevDCBypass: cfg.chebyshevDCBypass,
		chebyshevWeights:  cfg.chebyshevWeights,
	}

	err := d.validate()
	if err != nil {
		return nil, err
	}

	return d, nil
}

// SetSampleRate updates sample rate.
func (d *Distortion) SetSampleRate(sampleRate float64) error {
	if sampleRate <= 0 || math.IsNaN(sampleRate) || math.IsInf(sampleRate, 0) {
		return fmt.Errorf("distortion sample rate must be > 0 and finite: %f", sampleRate)
	}

	d.sampleRate = sampleRate

	return nil
}

// SetMode sets the shaping mode.
func (d *Distortion) SetMode(mode DistortionMode) error {
	if !validDistortionMode(mode) {
		return fmt.Errorf("distortion mode is invalid: %d", mode)
	}

	d.mode = mode

	return d.validate()
}

// SetApproxMode sets exact vs polynomial approximation behavior.
func (d *Distortion) SetApproxMode(mode DistortionApproxMode) error {
	if !validApproxMode(mode) {
		return fmt.Errorf("distortion approximation mode is invalid: %d", mode)
	}

	d.approxMode = mode

	return nil
}

// SetDrive sets input drive in [0.01, 20].
func (d *Distortion) SetDrive(drive float64) error {
	if drive < minDistortionDrive || drive > maxDistortionDrive || math.IsNaN(drive) || math.IsInf(drive, 0) {
		return fmt.Errorf("distortion drive must be in [%g, %g]: %f", minDistortionDrive, maxDistortionDrive, drive)
	}

	d.drive = drive

	return nil
}

// SetMix sets dry/wet mix in [0, 1].
func (d *Distortion) SetMix(mix float64) error {
	if mix < 0 || mix > 1 || math.IsNaN(mix) || math.IsInf(mix, 0) {
		return fmt.Errorf("distortion mix must be in [0, 1]: %f", mix)
	}

	d.mix = mix

	return nil
}

// SetOutputLevel sets post-shape output level in [0, 4].
func (d *Distortion) SetOutputLevel(level float64) error {
	if level < minDistortionOutputLevel || level > maxDistortionOutputLevel || math.IsNaN(level) || math.IsInf(level, 0) {
		return fmt.Errorf("distortion output level must be in [%g, %g]: %f",
			minDistortionOutputLevel, maxDistortionOutputLevel, level)
	}

	d.outputLevel = level

	return nil
}

// SetClipLevel sets hard clip threshold in [0.05, 1.0].
func (d *Distortion) SetClipLevel(level float64) error {
	if level < minDistortionClipLevel || level > maxDistortionClipLevel || math.IsNaN(level) || math.IsInf(level, 0) {
		return fmt.Errorf("distortion clip level must be in [%g, %g]: %f",
			minDistortionClipLevel, maxDistortionClipLevel, level)
	}

	d.clipLevel = level

	return nil
}

// SetShape sets formula shaper parameter in [0, 1].
func (d *Distortion) SetShape(shape float64) error {
	if shape < minDistortionShape || shape > maxDistortionShape || math.IsNaN(shape) || math.IsInf(shape, 0) {
		return fmt.Errorf("distortion shape must be in [%g, %g]: %f", minDistortionShape, maxDistortionShape, shape)
	}

	d.shape = shape

	return nil
}

// SetBias sets pre-drive bias in [-1, 1].
func (d *Distortion) SetBias(bias float64) error {
	if bias < -1 || bias > 1 || math.IsNaN(bias) || math.IsInf(bias, 0) {
		return fmt.Errorf("distortion bias must be in [-1, 1]: %f", bias)
	}

	d.bias = bias

	return nil
}

// SetChebyshevOrder sets Chebyshev order in [1, 16].
func (d *Distortion) SetChebyshevOrder(order int) error {
	if order < minChebyshevOrder || order > maxChebyshevOrder {
		return fmt.Errorf("chebyshev order must be in [%d, %d]: %d", minChebyshevOrder, maxChebyshevOrder, order)
	}

	d.chebyshevOrder = order

	return d.validate()
}

// SetChebyshevHarmonicMode updates Chebyshev odd/even parity constraints.
func (d *Distortion) SetChebyshevHarmonicMode(mode ChebyshevHarmonicMode) error {
	if !validChebyshevHarmonicMode(mode) {
		return fmt.Errorf("chebyshev harmonic mode is invalid: %d", mode)
	}

	d.chebyshevMode = mode

	return d.validate()
}

// SetChebyshevInvert sets polarity inversion on Chebyshev output.
func (d *Distortion) SetChebyshevInvert(invert bool) {
	d.chebyshevInvert = invert
}

// SetChebyshevGainLevel sets Chebyshev output gain in [0, 4].
func (d *Distortion) SetChebyshevGainLevel(gain float64) error {
	if gain < minChebyshevGainLevel || gain > maxChebyshevGainLevel || math.IsNaN(gain) || math.IsInf(gain, 0) {
		return fmt.Errorf("chebyshev gain level must be in [%g, %g]: %f",
			minChebyshevGainLevel, maxChebyshevGainLevel, gain)
	}

	d.chebyshevGain = gain

	return nil
}

// SetChebyshevDCBypass toggles DC blocking for Chebyshev output.
func (d *Distortion) SetChebyshevDCBypass(enabled bool) {
	d.chebyshevDCBypass = enabled
}

// SetChebyshevWeights sets per-harmonic blend weights w1..wN for the Chebyshev
// waveshaper. At most 16 weights may be provided; all values must be finite.
// All 16 internal weight slots are zeroed before the provided values are copied,
// so passing a shorter slice effectively clears the remaining slots.
// When all weights are zero the legacy T_N-only path is used automatically.
func (d *Distortion) SetChebyshevWeights(weights []float64) error {
	if len(weights) > 16 {
		return fmt.Errorf("chebyshev weights length must be <= 16: %d", len(weights))
	}

	for i, w := range weights {
		if !isFinite(w) {
			return fmt.Errorf("chebyshev weight[%d] must be finite: %v", i, w)
		}
	}

	d.chebyshevWeights = [16]float64{}
	copy(d.chebyshevWeights[:], weights)

	return nil
}

// Reset clears internal state.
func (d *Distortion) Reset() {
	d.dcPrevIn = 0
	d.dcPrevOut = 0
}

// ProcessSample applies distortion to one sample.
func (d *Distortion) ProcessSample(input float64) float64 {
	dry := input
	x := (input + d.bias) * d.drive

	wet := d.shapeSample(x)
	wet *= d.outputLevel

	if d.mode == DistortionModeChebyshev && d.chebyshevDCBypass {
		wet = d.dcBypass(wet)
	}

	if !isFinite(wet) {
		wet = 0
	}

	return dry*(1-d.mix) + wet*d.mix
}

// ProcessInPlace applies distortion to buf in place.
func (d *Distortion) ProcessInPlace(buf []float64) {
	for i := range buf {
		buf[i] = d.ProcessSample(buf[i])
	}
}

// SampleRate returns sample rate in Hz.
func (d *Distortion) SampleRate() float64 { return d.sampleRate }

// Mode returns the active shaping mode.
func (d *Distortion) Mode() DistortionMode { return d.mode }

// Drive returns pre-shape drive.
func (d *Distortion) Drive() float64 { return d.drive }

// Mix returns dry/wet mix in [0,1].
func (d *Distortion) Mix() float64 { return d.mix }

// OutputLevel returns post-shape output level.
func (d *Distortion) OutputLevel() float64 { return d.outputLevel }

// ClipLevel returns hard-clip threshold.
func (d *Distortion) ClipLevel() float64 { return d.clipLevel }

// Shape returns formula shaper parameter.
func (d *Distortion) Shape() float64 { return d.shape }

// Bias returns pre-drive bias.
func (d *Distortion) Bias() float64 { return d.bias }

// ChebyshevOrder returns Chebyshev order.
func (d *Distortion) ChebyshevOrder() int { return d.chebyshevOrder }

func (d *Distortion) validate() error {
	err := validateChebyshevParity(d.chebyshevOrder, d.chebyshevMode)
	if err != nil {
		return err
	}

	if !validDistortionMode(d.mode) {
		return fmt.Errorf("distortion mode is invalid: %d", d.mode)
	}

	if !validApproxMode(d.approxMode) {
		return fmt.Errorf("distortion approximation mode is invalid: %d", d.approxMode)
	}

	return nil
}

//nolint:cyclop
func (d *Distortion) shapeSample(x float64) float64 {
	switch d.mode {
	case DistortionModeSoftClip:
		return d.softClip(x)
	case DistortionModeHardClip:
		return d.hardClip(x)
	case DistortionModeTanh:
		return d.tanhShape(x)
	case DistortionModeWaveshaper1:
		return clampUnitDist(x / (1 + d.shape*math.Abs(x)))
	case DistortionModeWaveshaper2:
		return clampUnitDist(((1 + d.shape) * x) / (1 + d.shape*math.Abs(x)))
	case DistortionModeWaveshaper3:
		return clampUnitDist(x - d.shape*x*x*x/3)
	case DistortionModeWaveshaper4:
		return clampUnitDist((3 * x) / (2 + math.Abs(2*x)))
	case DistortionModeWaveshaper5:
		scale := 1 + 4*d.shape
		return clampUnitDist(math.Atan(x*scale) / math.Atan(scale))
	case DistortionModeWaveshaper6:
		return clampUnitDist((1 + d.shape) * x / (1 + d.shape*x*x))
	case DistortionModeWaveshaper7:
		return d.tanhShape(x * (1 + 6*d.shape))
	case DistortionModeWaveshaper8:
		a := 1 + 6*d.shape
		return clampUnitDist(math.Copysign(1-math.Exp(-math.Abs(x)*a), x))
	case DistortionModeSaturate:
		return clampUnitDist(x / (1 + math.Abs(x)))
	case DistortionModeSaturate2:
		return d.softClip(x * (1 + 2*d.shape))
	case DistortionModeSoftSat:
		return d.softSat(x)
	case DistortionModeChebyshev:
		return d.chebyshevShape(x)
	default:
		return d.softClip(x)
	}
}

func (d *Distortion) hardClip(x float64) float64 {
	if x > d.clipLevel {
		x = d.clipLevel
	}

	if x < -d.clipLevel {
		x = -d.clipLevel
	}

	return x / d.clipLevel
}

func (d *Distortion) softClip(x float64) float64 {
	ax := math.Abs(x)
	if ax < 1 {
		return 1.5 * (x - (x*x*x)/3)
	}

	return math.Copysign(1, x)
}

func (d *Distortion) tanhShape(x float64) float64 {
	if d.approxMode == DistortionApproxPolynomial {
		return fastTanhApprox(x)
	}

	return math.Tanh(x)
}

func (d *Distortion) softSat(x float64) float64 {
	if d.approxMode == DistortionApproxPolynomial {
		return clampUnitDist(x * (27 + x*x) / (27 + 9*x*x))
	}

	return clampUnitDist((2 / math.Pi) * math.Atan((math.Pi/2)*x))
}

func (d *Distortion) chebyshevShape(x float64) float64 {
	x = clamp(x, -1, 1)

	// Determine whether per-harmonic weights are active.
	hasWeights := false

	for k := range d.chebyshevOrder {
		if d.chebyshevWeights[k] != 0 {
			hasWeights = true
			break
		}
	}

	// T_0=1, T_1=x; recurrence T_n = 2x·T_{n-1} − T_{n-2}
	t0 := 1.0 // T_0
	t1 := x   // T_1

	weightedSum := 0.0
	if hasWeights {
		weightedSum = d.chebyshevWeights[0] * t1 // T_1 contribution
	}

	tn := t1
	for n := 2; n <= d.chebyshevOrder; n++ {
		tn = 2*x*t1 - t0
		if hasWeights {
			weightedSum += d.chebyshevWeights[n-1] * tn
		}

		t0, t1 = t1, tn
	}

	var out float64
	if hasWeights {
		out = weightedSum * d.chebyshevGain
	} else {
		out = tn * d.chebyshevGain
	}

	if d.chebyshevInvert {
		out = -out
	}

	return clampUnitDist(out)
}

func (d *Distortion) dcBypass(x float64) float64 {
	y := x - d.dcPrevIn + dcBypassPole*d.dcPrevOut
	d.dcPrevIn = x
	d.dcPrevOut = y

	return y
}

func validDistortionMode(mode DistortionMode) bool {
	return mode >= DistortionModeSoftClip && mode <= DistortionModeChebyshev
}

func validApproxMode(mode DistortionApproxMode) bool {
	return mode == DistortionApproxExact || mode == DistortionApproxPolynomial
}

func validChebyshevHarmonicMode(mode ChebyshevHarmonicMode) bool {
	return mode >= ChebyshevHarmonicAll && mode <= ChebyshevHarmonicEven
}

func validateChebyshevParity(order int, mode ChebyshevHarmonicMode) error {
	switch mode {
	case ChebyshevHarmonicOdd:
		if order%2 == 0 {
			return fmt.Errorf("chebyshev odd harmonic mode requires odd order: %d", order)
		}
	case ChebyshevHarmonicEven:
		if order%2 != 0 {
			return fmt.Errorf("chebyshev even harmonic mode requires even order: %d", order)
		}
	case ChebyshevHarmonicAll:
		return nil
	default:
		return fmt.Errorf("chebyshev harmonic mode is invalid: %d", mode)
	}

	return nil
}

func fastTanhApprox(x float64) float64 {
	if x > 3 {
		return 1
	}

	if x < -3 {
		return -1
	}

	x2 := x * x

	return clampUnitDist(x * (27 + x2) / (27 + 9*x2))
}

func clamp(x, lo, hi float64) float64 {
	if x < lo {
		return lo
	}

	if x > hi {
		return hi
	}

	return x
}

func clampUnitDist(x float64) float64 {
	return clamp(x, -1, 1)
}

func isFinite(x float64) bool {
	return !math.IsNaN(x) && !math.IsInf(x, 0)
}
