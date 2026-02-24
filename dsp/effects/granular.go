package effects

import (
	"fmt"
	"math"
	"math/rand"
)

const (
	defaultGranularGrainSeconds = 0.08
	defaultGranularOverlap      = 0.5
	defaultGranularMix          = 1.0
	defaultGranularPitch        = 1.0
	defaultGranularSpray        = 0.1
	defaultGranularBaseDelaySec = 0.08
	defaultGranularSeed         = 1

	minGranularGrainSeconds = 0.005
	maxGranularGrainSeconds = 0.5
	minGranularPitch        = 0.25
	maxGranularPitch        = 4.0
	maxGranularDelaySeconds = 2.0
	maxGranularVoices       = 64
)

type granularGrain struct {
	active bool
	pos    float64
	age    int
	dur    int
}

// Granular is a mono granular texture processor using overlap-add grain
// scheduling from a short circular history buffer.
//
// Each grain reads from the input history using linear interpolation,
// applies a Hann envelope, and is mixed with dry signal according to Mix.
//
// This processor is real-time safe (no per-sample allocations) and not
// thread-safe.
type Granular struct {
	sampleRate       float64
	grainSeconds     float64
	overlap          float64
	mix              float64
	pitch            float64
	spray            float64
	baseDelaySeconds float64
	seed             int64

	ring  []float64
	write int

	grainSamples     int
	spawnInterval    int
	baseDelaySamples int
	spraySamples     int
	nextSpawn        int

	grains [maxGranularVoices]granularGrain
	rng    *rand.Rand
}

// NewGranular creates a granular processor with practical defaults.
func NewGranular(sampleRate float64) (*Granular, error) {
	if sampleRate <= 0 || math.IsNaN(sampleRate) || math.IsInf(sampleRate, 0) {
		return nil, fmt.Errorf("granular sample rate must be > 0: %f", sampleRate)
	}

	granular := &Granular{
		sampleRate:       sampleRate,
		grainSeconds:     defaultGranularGrainSeconds,
		overlap:          defaultGranularOverlap,
		mix:              defaultGranularMix,
		pitch:            defaultGranularPitch,
		spray:            defaultGranularSpray,
		baseDelaySeconds: defaultGranularBaseDelaySec,
		seed:             defaultGranularSeed,
		rng:              rand.New(rand.NewSource(defaultGranularSeed)),
	}

	if err := granular.reconfigureState(); err != nil {
		return nil, err
	}

	return granular, nil
}

// SampleRate returns sample rate in Hz.
func (g *Granular) SampleRate() float64 { return g.sampleRate }

// GrainSeconds returns grain duration in seconds.
func (g *Granular) GrainSeconds() float64 { return g.grainSeconds }

// Overlap returns normalized grain overlap in [0, 0.95].
func (g *Granular) Overlap() float64 { return g.overlap }

// Mix returns wet/dry mix in [0, 1].
func (g *Granular) Mix() float64 { return g.mix }

// Pitch returns grain playback ratio in [0.25, 4].
func (g *Granular) Pitch() float64 { return g.pitch }

// Spray returns random start-position spread in [0, 1].
func (g *Granular) Spray() float64 { return g.spray }

// BaseDelay returns the read delay in seconds from current write position.
func (g *Granular) BaseDelay() float64 { return g.baseDelaySeconds }

// SetSampleRate updates sample-rate metadata and reconfigures internal state.
func (g *Granular) SetSampleRate(sampleRate float64) error {
	if sampleRate <= 0 || math.IsNaN(sampleRate) || math.IsInf(sampleRate, 0) {
		return fmt.Errorf("granular sample rate must be > 0: %f", sampleRate)
	}

	g.sampleRate = sampleRate

	return g.reconfigureState()
}

// SetGrainSeconds sets grain duration in [0.005, 0.5] seconds.
func (g *Granular) SetGrainSeconds(seconds float64) error {
	if seconds < minGranularGrainSeconds || seconds > maxGranularGrainSeconds ||
		math.IsNaN(seconds) || math.IsInf(seconds, 0) {
		return fmt.Errorf("granular grain seconds must be in [%f, %f]: %f",
			minGranularGrainSeconds, maxGranularGrainSeconds, seconds)
	}

	g.grainSeconds = seconds

	return g.reconfigureState()
}

// SetOverlap sets normalized grain overlap in [0, 0.95].
func (g *Granular) SetOverlap(overlap float64) error {
	if overlap < 0 || overlap > 0.95 || math.IsNaN(overlap) || math.IsInf(overlap, 0) {
		return fmt.Errorf("granular overlap must be in [0, 0.95]: %f", overlap)
	}

	g.overlap = overlap
	g.updateDerivedParams()

	return nil
}

// SetMix sets wet/dry mix in [0, 1].
func (g *Granular) SetMix(mix float64) error {
	if mix < 0 || mix > 1 || math.IsNaN(mix) || math.IsInf(mix, 0) {
		return fmt.Errorf("granular mix must be in [0, 1]: %f", mix)
	}

	g.mix = mix

	return nil
}

// SetPitch sets grain playback ratio in [0.25, 4].
func (g *Granular) SetPitch(pitch float64) error {
	if pitch < minGranularPitch || pitch > maxGranularPitch || math.IsNaN(pitch) || math.IsInf(pitch, 0) {
		return fmt.Errorf("granular pitch must be in [%f, %f]: %f", minGranularPitch, maxGranularPitch, pitch)
	}

	g.pitch = pitch

	return nil
}

// SetSpray sets random start-position spread in [0, 1].
func (g *Granular) SetSpray(spray float64) error {
	if spray < 0 || spray > 1 || math.IsNaN(spray) || math.IsInf(spray, 0) {
		return fmt.Errorf("granular spray must be in [0, 1]: %f", spray)
	}

	g.spray = spray
	g.updateDerivedParams()

	return nil
}

// SetBaseDelay sets the read delay in [0, 2] seconds.
func (g *Granular) SetBaseDelay(seconds float64) error {
	if seconds < 0 || seconds > maxGranularDelaySeconds || math.IsNaN(seconds) || math.IsInf(seconds, 0) {
		return fmt.Errorf("granular base delay must be in [0, %f]: %f", maxGranularDelaySeconds, seconds)
	}

	g.baseDelaySeconds = seconds
	g.updateDerivedParams()

	return nil
}

// SetRandomSeed sets RNG seed for deterministic grain spray.
func (g *Granular) SetRandomSeed(seed int64) {
	g.seed = seed
	g.rng.Seed(seed)
	g.Reset()
}

// Reset clears ring/grain state and rewinds random state.
func (g *Granular) Reset() {
	for i := range g.ring {
		g.ring[i] = 0
	}

	g.write = 0
	g.nextSpawn = 0

	for i := range g.grains {
		g.grains[i] = granularGrain{}
	}

	g.rng.Seed(g.seed)
}

// ProcessSample processes one sample through the granular engine.
func (g *Granular) ProcessSample(input float64) float64 {
	g.ring[g.write] = input

	g.write++
	if g.write >= len(g.ring) {
		g.write = 0
	}

	if g.nextSpawn <= 0 {
		g.spawnGrain()
		g.nextSpawn = g.spawnInterval
	} else {
		g.nextSpawn--
	}

	wet := 0.0
	norm := 0.0

	for i := range g.grains {
		grain := &g.grains[i]
		if !grain.active {
			continue
		}

		if grain.age >= grain.dur {
			grain.active = false
			continue
		}

		env := hannEnv(grain.age, grain.dur)
		s := g.readLinear(grain.pos)
		wet += s * env
		norm += env

		grain.pos += g.pitch
		for grain.pos >= float64(len(g.ring)) {
			grain.pos -= float64(len(g.ring))
		}

		for grain.pos < 0 {
			grain.pos += float64(len(g.ring))
		}

		grain.age++
		if grain.age >= grain.dur {
			grain.active = false
		}
	}

	if norm > 1e-12 {
		wet /= norm
	}

	return input*(1-g.mix) + wet*g.mix
}

// ProcessInPlace applies granular processing to buf in place.
func (g *Granular) ProcessInPlace(buf []float64) {
	for i := range buf {
		buf[i] = g.ProcessSample(buf[i])
	}
}

func (g *Granular) reconfigureState() error {
	if g.sampleRate <= 0 || math.IsNaN(g.sampleRate) || math.IsInf(g.sampleRate, 0) {
		return fmt.Errorf("granular sample rate must be > 0: %f", g.sampleRate)
	}

	if g.grainSeconds < minGranularGrainSeconds || g.grainSeconds > maxGranularGrainSeconds {
		return fmt.Errorf("granular grain seconds must be in [%f, %f]: %f",
			minGranularGrainSeconds, maxGranularGrainSeconds, g.grainSeconds)
	}

	ringLen := int(math.Ceil(maxGranularDelaySeconds*g.sampleRate)) + int(math.Ceil(g.grainSeconds*g.sampleRate)) + 4
	if ringLen < 128 {
		ringLen = 128
	}

	g.ring = make([]float64, ringLen)
	g.updateDerivedParams()
	g.Reset()

	return nil
}

func (g *Granular) updateDerivedParams() {
	g.grainSamples = int(math.Round(g.grainSeconds * g.sampleRate))
	if g.grainSamples < 2 {
		g.grainSamples = 2
	}

	interval := int(math.Round(float64(g.grainSamples) * (1 - g.overlap)))
	if interval < 1 {
		interval = 1
	}

	g.spawnInterval = interval

	g.baseDelaySamples = int(math.Round(g.baseDelaySeconds * g.sampleRate))
	if g.baseDelaySamples < 0 {
		g.baseDelaySamples = 0
	}

	g.spraySamples = int(math.Round(float64(g.grainSamples) * g.spray))
}

func (g *Granular) spawnGrain() {
	slot := -1

	for i := range g.grains {
		if !g.grains[i].active {
			slot = i
			break
		}
	}

	if slot < 0 {
		return
	}

	offset := g.baseDelaySamples
	if g.spraySamples > 0 {
		jitter := int(math.Round((g.rng.Float64()*2 - 1) * float64(g.spraySamples)))
		offset += jitter
	}

	maxOffset := len(g.ring) - 2

	if offset < 0 {
		offset = 0
	}

	if offset > maxOffset {
		offset = maxOffset
	}

	start := g.write - offset
	for start < 0 {
		start += len(g.ring)
	}

	for start >= len(g.ring) {
		start -= len(g.ring)
	}

	g.grains[slot] = granularGrain{
		active: true,
		pos:    float64(start),
		age:    0,
		dur:    g.grainSamples,
	}
}

func (g *Granular) readLinear(pos float64) float64 {
	i0 := int(pos)
	frac := pos - float64(i0)

	i1 := i0 + 1
	if i1 >= len(g.ring) {
		i1 = 0
	}

	v0 := g.ring[i0]
	v1 := g.ring[i1]

	return v0 + (v1-v0)*frac
}

func hannEnv(age, dur int) float64 {
	if dur <= 1 {
		return 1
	}

	phase := 2 * math.Pi * float64(age) / float64(dur-1)

	return 0.5 * (1 - math.Cos(phase))
}
