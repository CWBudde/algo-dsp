package melody

import (
	"errors"
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/effects/pitch"
	"github.com/cwbudde/algo-dsp/dsp/stft"
)

// Sentinel errors returned by this package. Errors are wrapped with context,
// so test for them with errors.Is.
var (
	// ErrEmptyInput reports an empty signal or channel list.
	ErrEmptyInput = errors.New("melody: empty input")
	// ErrInvalidSampleRate reports a sample rate that is not a positive,
	// finite number.
	ErrInvalidSampleRate = errors.New("melody: invalid sample rate")
	// ErrInvalidFrameRate reports a frame rate that is not a positive,
	// finite number.
	ErrInvalidFrameRate = errors.New("melody: invalid frame rate")
	// ErrNilOption reports a nil [Option].
	ErrNilOption = errors.New("melody: nil option")
	// ErrInvalidOption reports an option value outside its valid range.
	ErrInvalidOption = errors.New("melody: invalid option")
	// ErrInvalidArgument reports an invalid argument, such as an invalid
	// grid passed to [Clean].
	ErrInvalidArgument = errors.New("melody: invalid argument")
	// ErrLengthMismatch reports input slices whose lengths must agree but
	// do not.
	ErrLengthMismatch = errors.New("melody: length mismatch")
)

// silentPower is the band power below which a frame counts as silent even
// when it passes the RMS gate.
const silentPower = 1e-12

// Note is one segmented melody note.
type Note struct {
	// Start and End are the note boundaries in seconds.
	Start, End float64
	// MIDI is the rounded median pitch of the note's voiced frames.
	MIDI int
	// Strength is the mean voicing of the note's voiced frames, in (0, 1].
	Strength float64
}

// Result is the frame-wise melody analysis of a signal. Frame i is centred on
// time i/FrameRate.
type Result struct {
	// Pitch is the predominant pitch per frame as fractional MIDI note
	// number, median-smoothed over voiced frames; 0 marks an unvoiced frame.
	Pitch []float64
	// Voicing is the share of band power explained by the chosen harmonic
	// series, in [0, 1]; 0 for frames below the voicing threshold.
	Voicing []float64
	// Chroma holds one row per pitch class, indexed by [pitch.PitchClass]
	// (C, C#, ..., B). Each frame is normalised so that its strongest class
	// is 1; silent and gated frames are all zero.
	Chroma [12][]float64
	// Notes is the segmented note list in time order. It is never nil.
	Notes []Note
	// FrameRate is the number of frames per second, sampleRate/hop.
	FrameRate float64
}

// Downmix averages equally long channels into one mono signal:
// mono[i] = sum over channels c of channels[c][i]/len(channels), accumulated
// in channel order. A single channel is returned as a copy.
func Downmix(channels [][]float64) ([]float64, error) {
	if len(channels) == 0 || len(channels[0]) == 0 {
		return nil, fmt.Errorf("%w: no samples to downmix", ErrEmptyInput)
	}

	length := len(channels[0])
	for c, ch := range channels {
		if len(ch) != length {
			return nil, fmt.Errorf("%w: channel %d has %d samples, channel 0 has %d", ErrLengthMismatch, c, len(ch), length)
		}
	}

	n := float64(len(channels))
	mono := make([]float64, length)

	for _, ch := range channels {
		for i, v := range ch {
			mono[i] += v / n
		}
	}

	return mono, nil
}

// Analyze tracks the predominant pitch, voicing and chroma of a mono signal
// frame by frame and segments the pitch track into notes. See the package
// documentation for the algorithm and its limits.
//
// Analyze is deterministic and safe for concurrent use; it allocates its
// scratch buffers per call.
func Analyze(mono []float64, sampleRate float64, opts ...Option) (*Result, error) {
	cfg, err := newConfig(opts)
	if err != nil {
		return nil, err
	}

	if len(mono) == 0 {
		return nil, fmt.Errorf("%w: no samples to analyze", ErrEmptyInput)
	}

	if !finite(sampleRate) || sampleRate <= 0 {
		return nil, fmt.Errorf("%w: %g", ErrInvalidSampleRate, sampleRate)
	}

	a, err := newAnalyzer(&cfg, sampleRate)
	if err != nil {
		return nil, err
	}

	count := a.frameCount(len(mono))
	res := &Result{
		Pitch:     make([]float64, count),
		Voicing:   make([]float64, count),
		FrameRate: sampleRate / float64(cfg.hop),
	}

	for c := range res.Chroma {
		res.Chroma[c] = make([]float64, count)
	}

	if a.cq != nil {
		err := a.cq.process(mono)
		if err != nil {
			return nil, err
		}
	}

	gate := math.Pow(10, cfg.gateDB/20)

	for frame := range count {
		if frameRMS(mono, frame*cfg.hop, cfg.hop) < gate {
			continue
		}

		err := a.frame(res, mono, frame)
		if err != nil {
			return nil, err
		}
	}

	radius := int(math.Round(cfg.smoothing * res.FrameRate))
	res.Pitch = MedianVoiced(res.Pitch, radius)
	res.Notes = segmentNotes(res.Pitch, res.Voicing, 1/res.FrameRate, cfg.onsets, newSegmentParams(&cfg.notes, res.FrameRate))

	return res, nil
}

// frameRMS returns the RMS of x over [center-radius, center+radius).
func frameRMS(x []float64, center, radius int) float64 {
	energy, n := 0.0, 0.0
	for j := max(0, center-radius); j < min(len(x), center+radius); j++ {
		energy += x[j] * x[j]
		n++
	}

	return math.Sqrt(energy / math.Max(1, n))
}

// candidate is one pitch hypothesis of the salience grid. Its harmonics are
// harmonics[first : first+count].
type candidate struct {
	midi         float64
	first, count int
}

// harmonic is a precomputed linear interpolation of the FFT magnitude
// spectrum at a fractional bin: mag[idx]*(1-frac) + mag[idx+1]*frac, or 0
// when idx+1 is past the last bin.
type harmonic struct {
	weight   float64
	idx      int
	oneMinus float64
	frac     float64
	pastEnd  bool
}

// analyzer holds the per-call spectral state of [Analyze]. Exactly one front
// end is set: st (STFT, the default) or cq (CQT, [WithCQT]). Both fill
// mag[lo..hi] per frame; chroma and voicing are shared, salience
// interpolates per front end.
type analyzer struct {
	cfg        *config
	st         *stft.STFT
	cq         *cqtFront
	binsPerHz  float64
	lo, hi     int
	spec       []complex128
	mag        []float64
	used       []bool
	pitchClass []int
	cands      []candidate
	harmonics  []harmonic
}

func newAnalyzer(cfg *config, sampleRate float64) (*analyzer, error) {
	a := &analyzer{cfg: cfg}

	var err error
	if cfg.cqt != nil {
		err = a.initCQT(sampleRate)
	} else {
		err = a.initSTFT(sampleRate)
	}

	if err != nil {
		return nil, err
	}

	// The candidate grid is accumulated, not multiplied, so its values
	// match a loop that steps midi += 0.1.
	for midi := cfg.minMIDI; midi <= cfg.maxMIDI; midi += pitchStep {
		f0 := pitch.MIDIToFrequency(midi, cfg.referenceHz)
		c := candidate{midi: midi, first: a.harmonicCount()}
		weight := 1.0

		for h := 1; h <= cfg.harmonics && f0*float64(h) < cfg.maxHz; h++ {
			a.addHarmonic(a.binOf(f0*float64(h)), weight)
			weight *= cfg.harmonicDecay
		}

		c.count = a.harmonicCount() - c.first
		a.cands = append(a.cands, c)
	}

	return a, nil
}

// initSTFT sets up the default STFT front end: FFT bins k*sampleRate/fftSize,
// the band [minHz, maxHz] mapped to bins lo..hi (bin 0 excluded).
func (a *analyzer) initSTFT(sampleRate float64) error {
	cfg := a.cfg

	st, err := stft.New(cfg.fftSize, cfg.hop)
	if err != nil {
		return fmt.Errorf("melody: %w", err)
	}

	bins := st.Bins()
	binsPerHz := float64(cfg.fftSize) / sampleRate
	a.st = st
	a.binsPerHz = binsPerHz
	a.lo = max(1, int(cfg.minHz*binsPerHz))
	a.hi = min(bins-1, int(cfg.maxHz*binsPerHz))
	a.spec = make([]complex128, bins)
	a.allocBins(bins)

	for k := 1; k < bins; k++ {
		a.pitchClass[k] = pitchClassOf(float64(k)/binsPerHz, cfg.referenceHz)
	}

	return nil
}

func (a *analyzer) allocBins(bins int) {
	a.mag = make([]float64, bins)
	a.used = make([]bool, bins)
	a.pitchClass = make([]int, bins)
}

// pitchClassOf returns the pitch class (0 = C) of the equal-tempered note
// nearest to hz.
func pitchClassOf(hz, referenceHz float64) int {
	return ((int(math.Round(12*math.Log2(hz/referenceHz)))+69)%12 + 12) % 12
}

// binOf returns the fractional bin position of a frequency: hz*fftSize/
// sampleRate for the STFT, binsPerOctave*log2(hz/f_0) for the CQT, f_0 being
// the centre of CQT bin 0.
func (a *analyzer) binOf(hz float64) float64 {
	if a.cq != nil {
		return a.cq.binsPerOctave * math.Log2(hz/a.cq.fmin)
	}

	return hz * a.binsPerHz
}

// frameCount returns the number of analysis frames for n samples:
// ceil(n/hop) for both front ends.
func (a *analyzer) frameCount(n int) int {
	if a.cq != nil {
		return (n + a.cfg.hop - 1) / a.cfg.hop
	}

	return a.st.FrameCount(n)
}

// addHarmonic appends the interpolation of the magnitude at a fractional bin
// to the front end's harmonic list.
func (a *analyzer) addHarmonic(bin, weight float64) {
	if a.cq != nil {
		a.cq.addHarmonic(bin, weight)

		return
	}

	i := int(bin)
	f := bin - float64(i)

	a.harmonics = append(a.harmonics, harmonic{
		weight:   weight,
		idx:      i,
		oneMinus: 1 - f,
		frac:     f,
		pastEnd:  i+1 >= len(a.mag),
	})
}

func (a *analyzer) harmonicCount() int {
	if a.cq != nil {
		return len(a.cq.harmonics)
	}

	return len(a.harmonics)
}

// fill writes the magnitudes of the band bins lo..hi of one frame to a.mag.
// Bins outside the band are never written and stay 0.
func (a *analyzer) fill(mono []float64, frame int) error {
	if a.cq != nil {
		row := a.cq.row(frame)
		copy(a.mag[a.lo:a.hi+1], row[a.lo:a.hi+1])

		return nil
	}

	err := a.st.FrameInto(a.spec, mono, frame)
	if err != nil {
		return fmt.Errorf("melody: frame %d: %w", frame, err)
	}

	for k := a.lo; k <= a.hi; k++ {
		a.mag[k] = math.Hypot(real(a.spec[k]), imag(a.spec[k]))
	}

	return nil
}

// frame analyses one frame that passed the RMS gate.
func (a *analyzer) frame(res *Result, mono []float64, frame int) error {
	err := a.fill(mono, frame)
	if err != nil {
		return err
	}

	mag := a.mag
	total, chroma := 0.0, [12]float64{}

	for k := a.lo; k <= a.hi; k++ {
		p := mag[k] * mag[k]
		total += p
		chroma[a.pitchClass[k]] += p
	}

	if total < silentPower {
		return nil
	}

	strongest := 0.0
	for _, v := range chroma {
		strongest = math.Max(strongest, v)
	}

	for c, v := range chroma {
		res.Chroma[c][frame] = v / strongest
	}

	bestMIDI := a.salience()

	voicing := math.Min(1, a.explained(bestMIDI)/total)
	if voicing >= a.cfg.voicing {
		res.Pitch[frame] = bestMIDI
		res.Voicing[frame] = voicing
	}

	return nil
}

// salience returns the candidate with the largest weighted harmonic sum, or
// 0 if no candidate has a positive sum.
func (a *analyzer) salience() float64 {
	if a.cq != nil {
		return a.cq.salience(a.cands)
	}

	best, bestMIDI := 0.0, 0.0

	for _, c := range a.cands {
		s := 0.0

		for _, h := range a.harmonics[c.first : c.first+c.count] {
			s += h.weight * a.at(h)
		}

		if s > best {
			best, bestMIDI = s, c.midi
		}
	}

	return bestMIDI
}

func (a *analyzer) at(h harmonic) float64 {
	if h.pastEnd {
		return 0
	}

	return a.mag[h.idx]*h.oneMinus + a.mag[h.idx+1]*h.frac
}

// explained returns the power inside the main lobes (±2 bins) of the
// harmonic series of midi, counting each bin once.
func (a *analyzer) explained(midi float64) float64 {
	clear(a.used)

	explained := 0.0
	f0 := pitch.MIDIToFrequency(midi, a.cfg.referenceHz)

	for h := 1; h <= a.cfg.harmonics && f0*float64(h) < a.cfg.maxHz; h++ {
		k := int(math.Round(a.binOf(f0 * float64(h))))
		for j := max(a.lo, k-2); j <= min(a.hi, k+2); j++ {
			if !a.used[j] {
				a.used[j] = true
				explained += a.mag[j] * a.mag[j]
			}
		}
	}

	return explained
}
