package dynamics

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/filter/biquad"
	"github.com/cwbudde/algo-dsp/dsp/filter/design"
)

const (
	// Default dynamic EQ band parameters
	defaultEQBandQ           = 1.0
	defaultEQBandRatio       = 2.0
	defaultEQBandKneeDB      = 6.0
	defaultEQBandAttackMs    = 10.0
	defaultEQBandReleaseMs   = 100.0
	defaultEQBandRangeDB     = 12.0
	defaultEQBandRMSWindowMs = 30.0
	defaultEQUpdateInterval  = 32

	// Parameter validation ranges
	minEQBandQ            = 0.05
	maxEQBandQ            = 100.0
	maxEQBandStaticGainDB = 48.0
	minEQBandRangeDB      = 0.0
	maxEQBandRangeDB      = 48.0
	maxDynamicEQBands     = 16
	maxEQUpdateInterval   = 4096

	// eqMinDetectorLevel is the magnitude below which the detector is treated as
	// silent, avoiding infinities in the upward-below level mirror.
	eqMinDetectorLevel = 1e-12

	// eqGainEpsilonDB is the gain change below which coefficients are not redesigned.
	eqGainEpsilonDB = 1e-9
)

// EQBandType selects the filter shape of a dynamic EQ band.
type EQBandType int

const (
	// EQBandPeak is a peaking (bell) band centred on the band frequency.
	EQBandPeak EQBandType = iota
	// EQBandLowShelf is a low shelf hinged at the band frequency.
	EQBandLowShelf
	// EQBandHighShelf is a high shelf hinged at the band frequency.
	EQBandHighShelf
)

// EQBandMode selects how detector level is mapped to band gain.
type EQBandMode int

const (
	// EQBandModeStatic applies the static gain only; the band behaves as a
	// plain parametric EQ section.
	EQBandModeStatic EQBandMode = iota
	// EQBandModeDownward cuts the band as the detector level rises above the
	// threshold (compressive), using the shared soft-knee gain computer.
	EQBandModeDownward
	// EQBandModeUpward boosts the band as the detector level rises above the
	// threshold; the mirror image of EQBandModeDownward.
	EQBandModeUpward
	// EQBandModeUpwardBelow boosts the band as the detector level falls below
	// the threshold (upward expansion).
	EQBandModeUpwardBelow
)

// EQDetectorSource selects what the band detector listens to.
type EQDetectorSource int

const (
	// EQDetectorBandpass filters the sidechain with a bandpass centred on the
	// band frequency, so the band only reacts to energy in its own region.
	EQDetectorBandpass EQDetectorSource = iota
	// EQDetectorWideband feeds the unfiltered sidechain to the detector.
	EQDetectorWideband
)

// EQBandConfig describes one dynamic EQ band.
//
// Zero values mean "use the default" for every numeric field except
// FrequencyHz (required) and StaticGainDB, ThresholdDB, SidechainLowCutHz and
// SidechainHighCutHz, where zero is itself a meaningful setting. KneeDB and
// DetectorMode are pointers so that a zero knee (hard knee) can be
// distinguished from "keep default"; use [Float64Ptr] for the former.
type EQBandConfig struct {
	Type         EQBandType // Filter shape (default: peaking)
	FrequencyHz  float64    // Centre/hinge frequency in Hz, required, in (0, sampleRate/2)
	Q            float64    // Filter Q (0 = default 1.0)
	StaticGainDB float64    // Always-applied gain offset in dB

	Mode        EQBandMode // Dynamic behaviour (default: static)
	ThresholdDB float64    // Detector threshold in dB (0 means 0 dBFS)
	Ratio       float64    // Dynamics ratio (0 = default 2.0)
	KneeDB      *float64   // Soft-knee width in dB (nil = default, ptr to 0 = hard knee)
	AttackMs    float64    // Detector attack time in ms (0 = default)
	ReleaseMs   float64    // Detector release time in ms (0 = default)
	RangeDB     float64    // Maximum absolute dynamic gain in dB (0 = default 12)

	DetectorSource     EQDetectorSource // Detector input (default: band-filtered)
	DetectorQ          float64          // Detector bandpass Q (0 = follow Q)
	DetectorMode       *DetectorMode    // Peak or RMS detection (nil = peak)
	RMSWindowMs        float64          // RMS window in ms (0 = default)
	SidechainLowCutHz  float64          // Detector-only low cut in Hz (0 disables)
	SidechainHighCutHz float64          // Detector-only high cut in Hz (0 disables)
}

// EQBandMetrics holds metering information for a single dynamic EQ band.
type EQBandMetrics struct {
	InputPeak     float64 // Maximum band input magnitude since last reset
	OutputPeak    float64 // Maximum band output magnitude since last reset
	CurrentGainDB float64 // Most recent dynamic gain contribution in dB
	MinGainDB     float64 // Most negative dynamic gain since last reset
	MaxGainDB     float64 // Most positive dynamic gain since last reset
}

// DynamicEQMetrics holds per-band metering information.
type DynamicEQMetrics struct {
	Bands []EQBandMetrics // Per-band metrics, in band order
}

// DynamicEQ is a series chain of parametric EQ bands whose gains are driven by
// signal level. Each band pairs a biquad section (peaking or shelving) with its
// own detector and soft-knee gain computer from the shared dynamics core, so a
// band can cut when its region gets loud (de-essing, harshness taming), boost
// when it gets loud, or boost when it falls quiet (upward expansion).
//
// By default a band's detector listens to a bandpass of the sidechain centred
// on the band frequency, so bands react only to energy in their own region.
// [EQDetectorWideband] switches a band to full-band detection, and
// [DynamicEQ.ProcessSampleSidechain] supplies an external sidechain.
//
// Unlike [MultibandCompressor], the program path is not crossover-split: all
// bands run in series on the full-band signal, exactly as a parametric EQ does.
//
// Band coefficients are redesigned at control rate (see
// [DynamicEQ.SetUpdateInterval], default every 32 samples) using the canonical
// designers in dsp/filter/design. Coefficients are overwritten in place, so
// filter state is preserved across updates.
//
// This processor is mono, real-time safe, and not thread-safe.
type DynamicEQ struct {
	sampleRate     float64
	updateInterval int
	bands          []*eqBand
}

// NewDynamicEQ creates a dynamic EQ with no bands. Use [DynamicEQ.AddBand] to
// append bands.
func NewDynamicEQ(sampleRate float64) (*DynamicEQ, error) {
	if err := validateSampleRate(sampleRate); err != nil {
		return nil, fmt.Errorf("dynamic eq %w", err)
	}

	return &DynamicEQ{
		sampleRate:     sampleRate,
		updateInterval: defaultEQUpdateInterval,
	}, nil
}

// NewDynamicEQWithConfig creates a dynamic EQ with the supplied bands, applied
// in order from the input to the output of the chain.
func NewDynamicEQWithConfig(sampleRate float64, configs []EQBandConfig) (*DynamicEQ, error) {
	eq, err := NewDynamicEQ(sampleRate)
	if err != nil {
		return nil, err
	}

	for i, cfg := range configs {
		if _, err := eq.AddBand(cfg); err != nil {
			return nil, fmt.Errorf("dynamic eq: band %d: %w", i, err)
		}
	}

	return eq, nil
}

// AddBand appends a band to the chain and returns its index.
func (eq *DynamicEQ) AddBand(cfg EQBandConfig) (int, error) {
	if len(eq.bands) >= maxDynamicEQBands {
		return 0, fmt.Errorf("dynamic eq: at most %d bands are supported", maxDynamicEQBands)
	}

	b, err := newEQBand(cfg, eq.sampleRate)
	if err != nil {
		return 0, err
	}

	eq.bands = append(eq.bands, b)

	return len(eq.bands) - 1, nil
}

// NumBands returns the number of bands in the chain.
func (eq *DynamicEQ) NumBands() int { return len(eq.bands) }

// SampleRate returns the sample rate in Hz.
func (eq *DynamicEQ) SampleRate() float64 { return eq.sampleRate }

// UpdateInterval returns the coefficient update interval in samples.
func (eq *DynamicEQ) UpdateInterval() int { return eq.updateInterval }

// SetSampleRate changes the sample rate and rebuilds every band.
func (eq *DynamicEQ) SetSampleRate(sampleRate float64) error {
	if err := validateSampleRate(sampleRate); err != nil {
		return fmt.Errorf("dynamic eq %w", err)
	}

	for i, b := range eq.bands {
		if err := b.setSampleRate(sampleRate); err != nil {
			return fmt.Errorf("dynamic eq: band %d: %w", i, err)
		}
	}

	eq.sampleRate = sampleRate

	return nil
}

// SetUpdateInterval sets how often band coefficients are redesigned, in
// samples. One redesigns on every sample (most accurate, most expensive).
func (eq *DynamicEQ) SetUpdateInterval(samples int) error {
	if samples < 1 || samples > maxEQUpdateInterval {
		return fmt.Errorf("dynamic eq: update interval must be in [1, %d]: %d", maxEQUpdateInterval, samples)
	}

	eq.updateInterval = samples
	for _, b := range eq.bands {
		b.counter = 0
	}

	return nil
}

// BandConfig returns the effective (defaults resolved) configuration of a band.
func (eq *DynamicEQ) BandConfig(band int) (EQBandConfig, error) {
	if err := eq.checkBand(band); err != nil {
		return EQBandConfig{}, err
	}

	return eq.bands[band].config(), nil
}

// BandCoefficients returns the biquad coefficients currently applied to a band,
// for frequency-response inspection via [biquad.Coefficients.MagnitudeDB].
func (eq *DynamicEQ) BandCoefficients(band int) (biquad.Coefficients, error) {
	if err := eq.checkBand(band); err != nil {
		return biquad.Coefficients{}, err
	}

	return eq.bands[band].sect.Coefficients, nil
}

// BandGainDB returns the current dynamic gain contribution of a band in dB,
// excluding its static gain offset.
func (eq *DynamicEQ) BandGainDB(band int) (float64, error) {
	if err := eq.checkBand(band); err != nil {
		return 0, err
	}

	return eq.bands[band].currentGainDB, nil
}

// SetBandConfig replaces the configuration of a band, preserving filter state.
func (eq *DynamicEQ) SetBandConfig(band int, cfg EQBandConfig) error {
	if err := eq.checkBand(band); err != nil {
		return err
	}

	b, err := newEQBand(cfg, eq.sampleRate)
	if err != nil {
		return err
	}

	eq.bands[band] = b

	return nil
}

// SetBandFrequency sets the centre/hinge frequency of a band in Hz.
func (eq *DynamicEQ) SetBandFrequency(band int, hz float64) error {
	return eq.updateBand(band, func(p *eqBandParams) { p.FrequencyHz = hz })
}

// SetBandQ sets the filter Q of a band.
func (eq *DynamicEQ) SetBandQ(band int, q float64) error {
	return eq.updateBand(band, func(p *eqBandParams) { p.Q = q })
}

// SetBandStaticGain sets the always-applied gain offset of a band in dB.
func (eq *DynamicEQ) SetBandStaticGain(band int, dB float64) error {
	return eq.updateBand(band, func(p *eqBandParams) { p.StaticGainDB = dB })
}

// SetBandType sets the filter shape of a band.
func (eq *DynamicEQ) SetBandType(band int, t EQBandType) error {
	return eq.updateBand(band, func(p *eqBandParams) { p.Type = t })
}

// SetBandMode sets how detector level maps to band gain.
func (eq *DynamicEQ) SetBandMode(band int, mode EQBandMode) error {
	return eq.updateBand(band, func(p *eqBandParams) { p.Mode = mode })
}

// SetBandRange sets the maximum absolute dynamic gain of a band in dB.
func (eq *DynamicEQ) SetBandRange(band int, dB float64) error {
	return eq.updateBand(band, func(p *eqBandParams) { p.RangeDB = dB })
}

// SetBandDetectorSource selects band-filtered or wideband detection.
func (eq *DynamicEQ) SetBandDetectorSource(band int, src EQDetectorSource) error {
	return eq.updateBand(band, func(p *eqBandParams) { p.DetectorSource = src })
}

// SetBandDetectorQ sets the Q of the band's detector bandpass.
func (eq *DynamicEQ) SetBandDetectorQ(band int, q float64) error {
	return eq.updateBand(band, func(p *eqBandParams) { p.DetectorQ = q })
}

// SetBandThreshold sets the detector threshold of a band in dB.
func (eq *DynamicEQ) SetBandThreshold(band int, dB float64) error {
	return eq.updateBandCore(band, func(b *eqBand) error {
		if err := b.core.SetThreshold(dB); err != nil {
			return err
		}

		b.params.ThresholdDB = dB

		return nil
	})
}

// SetBandRatio sets the dynamics ratio of a band.
func (eq *DynamicEQ) SetBandRatio(band int, ratio float64) error {
	return eq.updateBandCore(band, func(b *eqBand) error {
		if err := b.core.SetRatio(ratio); err != nil {
			return err
		}

		b.params.Ratio = ratio

		return nil
	})
}

// SetBandKnee sets the soft-knee width of a band in dB.
func (eq *DynamicEQ) SetBandKnee(band int, kneeDB float64) error {
	return eq.updateBandCore(band, func(b *eqBand) error {
		if err := b.core.SetKnee(kneeDB); err != nil {
			return err
		}

		b.params.KneeDB = kneeDB

		return nil
	})
}

// SetBandAttack sets the detector attack time of a band in ms.
func (eq *DynamicEQ) SetBandAttack(band int, ms float64) error {
	return eq.updateBandCore(band, func(b *eqBand) error {
		if err := b.core.SetAttack(ms); err != nil {
			return err
		}

		b.params.AttackMs = ms

		return nil
	})
}

// SetBandRelease sets the detector release time of a band in ms.
func (eq *DynamicEQ) SetBandRelease(band int, ms float64) error {
	return eq.updateBandCore(band, func(b *eqBand) error {
		if err := b.core.SetRelease(ms); err != nil {
			return err
		}

		b.params.ReleaseMs = ms

		return nil
	})
}

// SetBandDetectorMode selects peak or RMS detection for a band.
func (eq *DynamicEQ) SetBandDetectorMode(band int, mode DetectorMode) error {
	return eq.updateBandCore(band, func(b *eqBand) error {
		if err := b.core.SetDetectorMode(mode); err != nil {
			return err
		}

		b.params.DetectorMode = mode

		return nil
	})
}

// SetBandRMSWindow sets the RMS detector window of a band in ms.
func (eq *DynamicEQ) SetBandRMSWindow(band int, ms float64) error {
	return eq.updateBandCore(band, func(b *eqBand) error {
		if err := b.core.SetRMSWindow(ms); err != nil {
			return err
		}

		b.params.RMSWindowMs = ms

		return nil
	})
}

// SetBandSidechainLowCut sets the detector-only low-cut frequency of a band in
// Hz (0 disables).
func (eq *DynamicEQ) SetBandSidechainLowCut(band int, hz float64) error {
	return eq.updateBandCore(band, func(b *eqBand) error {
		if err := b.core.SetSidechainLowCut(hz); err != nil {
			return err
		}

		b.params.SidechainLowCutHz = hz

		return nil
	})
}

// SetBandSidechainHighCut sets the detector-only high-cut frequency of a band
// in Hz (0 disables).
func (eq *DynamicEQ) SetBandSidechainHighCut(band int, hz float64) error {
	return eq.updateBandCore(band, func(b *eqBand) error {
		if err := b.core.SetSidechainHighCut(hz); err != nil {
			return err
		}

		b.params.SidechainHighCutHz = hz

		return nil
	})
}

// ProcessSample filters one sample, using the input as its own sidechain.
func (eq *DynamicEQ) ProcessSample(input float64) float64 {
	return eq.ProcessSampleSidechain(input, input)
}

// ProcessSampleSidechain filters one sample while driving every band detector
// from an external sidechain signal.
func (eq *DynamicEQ) ProcessSampleSidechain(input, sidechain float64) float64 {
	out := input
	for _, b := range eq.bands {
		out = b.process(out, sidechain, eq.updateInterval)
	}

	return out
}

// ProcessInPlace filters buf in place, using it as its own sidechain.
func (eq *DynamicEQ) ProcessInPlace(buf []float64) {
	for i := range buf {
		buf[i] = eq.ProcessSample(buf[i])
	}
}

// ProcessInPlaceSidechain filters program in place while driving the band
// detectors from sidechain. Both slices must have the same length.
func (eq *DynamicEQ) ProcessInPlaceSidechain(program, sidechain []float64) error {
	if len(program) != len(sidechain) {
		return fmt.Errorf("dynamic eq: program and sidechain must have equal length, got %d and %d",
			len(program), len(sidechain))
	}

	for i := range program {
		program[i] = eq.ProcessSampleSidechain(program[i], sidechain[i])
	}

	return nil
}

// BandStaticCurve samples the steady-state characteristic curve of one band,
// i.e. the band's centre-frequency gain as a function of detector level. It is
// shorthand for [StaticCurve] over that band and does not touch detector state.
func (eq *DynamicEQ) BandStaticCurve(band int, minInputDB, maxInputDB, stepDB float64) ([]CurvePoint, error) {
	if err := eq.checkBand(band); err != nil {
		return nil, err
	}

	return StaticCurve(eq.bands[band], minInputDB, maxInputDB, stepDB)
}

// Reset clears all filter and detector state without reallocating, returning
// the processor to its freshly constructed condition.
func (eq *DynamicEQ) Reset() {
	for _, b := range eq.bands {
		b.reset()
	}
}

// GetMetrics returns per-band metering values.
func (eq *DynamicEQ) GetMetrics() DynamicEQMetrics {
	bands := make([]EQBandMetrics, len(eq.bands))
	for i, b := range eq.bands {
		bands[i] = b.metrics
	}

	return DynamicEQMetrics{Bands: bands}
}

// ResetMetrics clears metering state for all bands.
func (eq *DynamicEQ) ResetMetrics() {
	for _, b := range eq.bands {
		b.metrics = EQBandMetrics{}
	}
}

func (eq *DynamicEQ) checkBand(band int) error {
	if band < 0 || band >= len(eq.bands) {
		return fmt.Errorf("dynamic eq: band index %d out of range [0, %d)", band, len(eq.bands))
	}

	return nil
}

// updateBand mutates the filter-shaping parameters of a band and rebuilds it,
// leaving the detector core and its envelope untouched.
func (eq *DynamicEQ) updateBand(band int, mutate func(*eqBandParams)) error {
	if err := eq.checkBand(band); err != nil {
		return err
	}

	b := eq.bands[band]

	params := b.params
	mutate(&params)

	if err := validateEQBandShape(params, eq.sampleRate); err != nil {
		return err
	}

	b.params = params
	b.rebuildFilters()

	return nil
}

// updateBandCore mutates detector/gain-computer parameters through the shared
// dynamics core, which owns their validation.
func (eq *DynamicEQ) updateBandCore(band int, mutate func(*eqBand) error) error {
	if err := eq.checkBand(band); err != nil {
		return err
	}

	if err := mutate(eq.bands[band]); err != nil {
		return fmt.Errorf("dynamic eq: band %d: %w", band, err)
	}

	return nil
}

// eqBandParams is the resolved (defaults applied) form of [EQBandConfig].
type eqBandParams struct {
	Type         EQBandType
	FrequencyHz  float64
	Q            float64
	StaticGainDB float64

	Mode        EQBandMode
	ThresholdDB float64
	Ratio       float64
	KneeDB      float64
	AttackMs    float64
	ReleaseMs   float64
	RangeDB     float64

	DetectorSource     EQDetectorSource
	DetectorQ          float64
	DetectorMode       DetectorMode
	RMSWindowMs        float64
	SidechainLowCutHz  float64
	SidechainHighCutHz float64
}

// eqBand is one dynamic EQ band: a biquad on the program path, a detector
// bandpass on the sidechain path, and a shared dynamics core in between.
type eqBand struct {
	params     eqBandParams
	sampleRate float64

	core *dynamicsCore
	sect biquad.Section
	det  biquad.Section

	currentGainDB float64
	appliedGainDB float64
	counter       int

	metrics EQBandMetrics
}

func newEQBand(cfg EQBandConfig, sampleRate float64) (*eqBand, error) {
	params := resolveEQBandConfig(cfg)
	if err := validateEQBandShape(params, sampleRate); err != nil {
		return nil, err
	}

	core, err := newDynamicsCore(dynamicsCoreConfig{
		sampleRate:         sampleRate,
		topology:           DynamicsTopologyFeedforward,
		detectorMode:       params.DetectorMode,
		thresholdDB:        params.ThresholdDB,
		ratio:              params.Ratio,
		kneeDB:             params.KneeDB,
		attackMs:           params.AttackMs,
		releaseMs:          params.ReleaseMs,
		rmsWindowMs:        params.RMSWindowMs,
		autoMakeup:         false,
		sidechainLowCutHz:  params.SidechainLowCutHz,
		sidechainHighCutHz: params.SidechainHighCutHz,
	})
	if err != nil {
		return nil, fmt.Errorf("dynamic eq band core init: %w", err)
	}

	b := &eqBand{params: params, sampleRate: sampleRate, core: core, appliedGainDB: math.NaN()}
	b.rebuildFilters()

	return b, nil
}

func resolveEQBandConfig(cfg EQBandConfig) eqBandParams {
	params := eqBandParams{
		Type:               cfg.Type,
		FrequencyHz:        cfg.FrequencyHz,
		Q:                  orDefault(cfg.Q, defaultEQBandQ),
		StaticGainDB:       cfg.StaticGainDB,
		Mode:               cfg.Mode,
		ThresholdDB:        cfg.ThresholdDB,
		Ratio:              orDefault(cfg.Ratio, defaultEQBandRatio),
		KneeDB:             defaultEQBandKneeDB,
		AttackMs:           orDefault(cfg.AttackMs, defaultEQBandAttackMs),
		ReleaseMs:          orDefault(cfg.ReleaseMs, defaultEQBandReleaseMs),
		RangeDB:            orDefault(cfg.RangeDB, defaultEQBandRangeDB),
		DetectorSource:     cfg.DetectorSource,
		DetectorMode:       DetectorModePeak,
		RMSWindowMs:        orDefault(cfg.RMSWindowMs, defaultEQBandRMSWindowMs),
		SidechainLowCutHz:  cfg.SidechainLowCutHz,
		SidechainHighCutHz: cfg.SidechainHighCutHz,
	}

	params.DetectorQ = orDefault(cfg.DetectorQ, params.Q)

	if cfg.KneeDB != nil {
		params.KneeDB = *cfg.KneeDB
	}

	if cfg.DetectorMode != nil {
		params.DetectorMode = *cfg.DetectorMode
	}

	return params
}

func validateEQBandShape(p eqBandParams, sampleRate float64) error {
	if p.Type != EQBandPeak && p.Type != EQBandLowShelf && p.Type != EQBandHighShelf {
		return fmt.Errorf("dynamic eq: invalid band type: %d", p.Type)
	}

	if p.Mode < EQBandModeStatic || p.Mode > EQBandModeUpwardBelow {
		return fmt.Errorf("dynamic eq: invalid band mode: %d", p.Mode)
	}

	if p.DetectorSource != EQDetectorBandpass && p.DetectorSource != EQDetectorWideband {
		return fmt.Errorf("dynamic eq: invalid detector source: %d", p.DetectorSource)
	}

	nyquist := sampleRate / 2
	if !isFinite(p.FrequencyHz) || p.FrequencyHz <= 0 || p.FrequencyHz >= nyquist {
		return fmt.Errorf("dynamic eq: band frequency must be in (0, %g): %f", nyquist, p.FrequencyHz)
	}

	if !isFinite(p.Q) || p.Q < minEQBandQ || p.Q > maxEQBandQ {
		return fmt.Errorf("dynamic eq: band q must be in [%g, %g]: %f", minEQBandQ, maxEQBandQ, p.Q)
	}

	if !isFinite(p.DetectorQ) || p.DetectorQ < minEQBandQ || p.DetectorQ > maxEQBandQ {
		return fmt.Errorf("dynamic eq: detector q must be in [%g, %g]: %f", minEQBandQ, maxEQBandQ, p.DetectorQ)
	}

	if !isFinite(p.StaticGainDB) || math.Abs(p.StaticGainDB) > maxEQBandStaticGainDB {
		return fmt.Errorf("dynamic eq: static gain must be in [%g, %g]: %f",
			-maxEQBandStaticGainDB, maxEQBandStaticGainDB, p.StaticGainDB)
	}

	if !isFinite(p.RangeDB) || p.RangeDB < minEQBandRangeDB || p.RangeDB > maxEQBandRangeDB {
		return fmt.Errorf("dynamic eq: band range must be in [%g, %g]: %f",
			minEQBandRangeDB, maxEQBandRangeDB, p.RangeDB)
	}

	return nil
}

func (b *eqBand) config() EQBandConfig {
	knee := b.params.KneeDB
	mode := b.params.DetectorMode

	return EQBandConfig{
		Type:               b.params.Type,
		FrequencyHz:        b.params.FrequencyHz,
		Q:                  b.params.Q,
		StaticGainDB:       b.params.StaticGainDB,
		Mode:               b.params.Mode,
		ThresholdDB:        b.params.ThresholdDB,
		Ratio:              b.params.Ratio,
		KneeDB:             &knee,
		AttackMs:           b.params.AttackMs,
		ReleaseMs:          b.params.ReleaseMs,
		RangeDB:            b.params.RangeDB,
		DetectorSource:     b.params.DetectorSource,
		DetectorQ:          b.params.DetectorQ,
		DetectorMode:       &mode,
		RMSWindowMs:        b.params.RMSWindowMs,
		SidechainLowCutHz:  b.params.SidechainLowCutHz,
		SidechainHighCutHz: b.params.SidechainHighCutHz,
	}
}

func (b *eqBand) setSampleRate(sampleRate float64) error {
	if err := validateEQBandShape(b.params, sampleRate); err != nil {
		return err
	}

	if err := b.core.SetSampleRate(sampleRate); err != nil {
		return err
	}

	b.sampleRate = sampleRate
	b.rebuildFilters()

	return nil
}

// rebuildFilters redesigns the detector bandpass and forces a program-path
// coefficient refresh.
func (b *eqBand) rebuildFilters() {
	b.det.Coefficients = detectorBandpass(b.params.FrequencyHz, b.params.DetectorQ, b.sampleRate)
	b.counter = 0
	b.appliedGainDB = math.NaN()
	b.applyGain(b.params.StaticGainDB + b.currentGainDB)
}

// detectorBandpass returns a unity-peak-gain bandpass. design.Bandpass is the
// constant-skirt-gain form, whose peak gain is Q; scaling the numerator by 1/Q
// yields the constant-0 dB-peak form, so the detected level equals the level of
// the in-band content rather than being amplified by Q.
func detectorBandpass(freq, q, sampleRate float64) biquad.Coefficients {
	c := design.Bandpass(freq, q, sampleRate)

	c.B0 /= q
	c.B1 /= q
	c.B2 /= q

	return c
}

// applyGain redesigns the program-path biquad for the supplied total gain,
// preserving its delay-line state. Coefficients are only recomputed when the
// gain actually moved.
func (b *eqBand) applyGain(totalGainDB float64) {
	// The comparison is false when appliedGainDB is NaN, which is how
	// rebuildFilters forces a redesign after a shape change.
	if math.Abs(totalGainDB-b.appliedGainDB) <= eqGainEpsilonDB {
		return
	}

	switch b.params.Type {
	case EQBandLowShelf:
		b.sect.Coefficients = design.LowShelf(b.params.FrequencyHz, totalGainDB, b.params.Q, b.sampleRate)
	case EQBandHighShelf:
		b.sect.Coefficients = design.HighShelf(b.params.FrequencyHz, totalGainDB, b.params.Q, b.sampleRate)
	case EQBandPeak:
		fallthrough
	default:
		b.sect.Coefficients = design.Peak(b.params.FrequencyHz, totalGainDB, b.params.Q, b.sampleRate)
	}

	b.appliedGainDB = totalGainDB
}

func (b *eqBand) process(input, sidechain float64, updateInterval int) float64 {
	detectorInput := sidechain
	if b.params.DetectorSource == EQDetectorBandpass {
		detectorInput = b.det.ProcessSample(sidechain)
	}

	// The detector runs every sample; the gain computer and the biquad
	// redesign only run at the control rate, since intermediate gains would be
	// discarded anyway.
	level := b.core.detectorLevel(b.core.detectorSource(0, detectorInput))

	if b.counter == 0 {
		b.currentGainDB = b.gainDBForLevel(level)
		b.applyGain(b.params.StaticGainDB + b.currentGainDB)
	}

	b.counter++
	if b.counter >= updateInterval {
		b.counter = 0
	}

	output := b.sect.ProcessSample(input)
	b.updateMetrics(abs(input), abs(output))

	return output
}

// gainDBForLevel maps a detector level to the band's dynamic gain in dB. It is
// state-free: all three dynamic modes are expressed through the shared
// soft-knee gain computer, so knee and ratio behave exactly as they do for the
// compressor and expander.
func (b *eqBand) gainDBForLevel(level float64) float64 {
	var deltaDB float64

	switch b.params.Mode {
	case EQBandModeStatic:
		return 0

	case EQBandModeDownward:
		deltaDB = ampToDB(b.core.GainForLevel(level))

	case EQBandModeUpward:
		deltaDB = -ampToDB(b.core.GainForLevel(level))

	case EQBandModeUpwardBelow:
		if level <= eqMinDetectorLevel {
			return b.params.RangeDB
		}

		deltaDB = -ampToDB(b.core.GainForLevel(mirrorLevel(level, b.core.ThresholdLog2())))
	}

	return clampDB(deltaDB, b.params.RangeDB)
}

// CalculateOutputLevel reports the steady-state centre-frequency gain applied
// to the supplied input magnitude, satisfying [StaticCurveProcessor]. The
// envelope follower is bypassed, so the call neither reads nor mutates
// detector state.
func (b *eqBand) CalculateOutputLevel(inputMagnitude float64) float64 {
	magnitude := abs(inputMagnitude)

	return magnitude * mathPower10((b.params.StaticGainDB+b.gainDBForLevel(magnitude))/20.0)
}

func (b *eqBand) reset() {
	b.core.Reset()
	b.sect.Reset()
	b.det.Reset()

	b.currentGainDB = 0
	b.counter = 0
	b.metrics = EQBandMetrics{}

	b.applyGain(b.params.StaticGainDB)
}

func (b *eqBand) updateMetrics(inputLevel, outputLevel float64) {
	if inputLevel > b.metrics.InputPeak {
		b.metrics.InputPeak = inputLevel
	}

	if outputLevel > b.metrics.OutputPeak {
		b.metrics.OutputPeak = outputLevel
	}

	b.metrics.CurrentGainDB = b.currentGainDB

	if b.currentGainDB < b.metrics.MinGainDB {
		b.metrics.MinGainDB = b.currentGainDB
	}

	if b.currentGainDB > b.metrics.MaxGainDB {
		b.metrics.MaxGainDB = b.currentGainDB
	}
}

// mirrorLevel reflects a level about the threshold in the log domain, so that
// the compressive gain computer evaluated at the mirrored level yields the
// undershoot curve needed for upward expansion.
func mirrorLevel(level, thresholdLog2 float64) float64 {
	return mathPower2(2*thresholdLog2 - mathLog2(level))
}

func clampDB(v, limitDB float64) float64 {
	if v > limitDB {
		return limitDB
	}

	if v < -limitDB {
		return -limitDB
	}

	return v
}

func orDefault(v, fallback float64) float64 {
	if v == 0 {
		return fallback
	}

	return v
}
