package harmony

import (
	"errors"
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/effects/pitch"
)

// Sentinel errors returned by this package. Errors are wrapped with context,
// so test for them with errors.Is.
var (
	// ErrNilOption reports a nil option.
	ErrNilOption = errors.New("harmony: nil option")
	// ErrInvalidOption reports an option value outside its valid range.
	ErrInvalidOption = errors.New("harmony: invalid option")
	// ErrInvalidInput reports an input value that is out of range, such as a
	// NaN or negative chroma weight, or an invalid [Key].
	ErrInvalidInput = errors.New("harmony: invalid input")
	// ErrLengthMismatch reports input slices whose lengths must agree but do
	// not.
	ErrLengthMismatch = errors.New("harmony: length mismatch")
	// ErrFlatChroma reports a chroma profile whose twelve values are all
	// equal (including all zero). It carries no tonal information, so no key
	// can be estimated from it.
	ErrFlatChroma = errors.New("harmony: flat chroma")
)

// DefaultTieMargin is the default correlation margin within which the best
// key and its relative major or minor count as tied (see [WithTieMargin]).
const DefaultTieMargin = 0.05

// pitchClasses is the number of pitch classes per octave.
const pitchClasses = 12

// Mode is the mode of a key.
type Mode uint8

// The two modes [EstimateKey] distinguishes.
const (
	// Major is the major mode (Ionian scale).
	Major Mode = iota
	// Minor is the minor mode; [Key.Scale] uses the natural minor (Aeolian)
	// scale.
	Minor
)

func (m Mode) valid() bool { return m <= Minor }

// String returns "major", "minor", or "Mode(n)" for an out-of-range value.
func (m Mode) String() string {
	switch m {
	case Major:
		return "major"
	case Minor:
		return "minor"
	default:
		return fmt.Sprintf("Mode(%d)", uint8(m))
	}
}

// ProfileSet selects the pair of major and minor key profiles that
// [EstimateKey] correlates with the chroma.
type ProfileSet uint8

// Available profile sets. Further sets (Temperley, Albrecht–Shanahan) can be
// added without changing the API.
const (
	// KrumhanslKessler is the Krumhansl–Kessler (1982) probe-tone profile
	// pair, the default.
	KrumhanslKessler ProfileSet = iota
)

// Krumhansl–Kessler probe-tone profiles, tonic first.
var (
	kkMajor = [pitchClasses]float64{6.35, 2.23, 3.48, 2.33, 4.38, 4.09, 2.52, 5.19, 2.39, 3.66, 2.29, 2.88}
	kkMinor = [pitchClasses]float64{6.33, 2.68, 3.52, 5.38, 2.60, 3.53, 2.54, 4.75, 3.98, 2.69, 3.34, 3.17}
)

// String returns the name of the profile set, or "ProfileSet(n)" for an
// unknown value.
func (p ProfileSet) String() string {
	if p == KrumhanslKessler {
		return "Krumhansl-Kessler"
	}

	return fmt.Sprintf("ProfileSet(%d)", uint8(p))
}

// Profile returns the profile of the given mode, tonic first (index 0 is the
// tonic, index 7 the fifth). It returns an error wrapping [ErrInvalidOption]
// for an unknown profile set or mode.
func (p ProfileSet) Profile(mode Mode) ([12]float64, error) {
	if p != KrumhanslKessler {
		return [12]float64{}, fmt.Errorf("%w: unknown profile set %d", ErrInvalidOption, uint8(p))
	}

	switch mode {
	case Major:
		return kkMajor, nil
	case Minor:
		return kkMinor, nil
	default:
		return [12]float64{}, fmt.Errorf("%w: unknown mode %d", ErrInvalidOption, uint8(mode))
	}
}

// Candidate is one of the 24 major and minor keys together with the
// correlation of its rotated profile with the chroma.
type Candidate struct {
	// Tonic is the key's tonic pitch class.
	Tonic pitch.PitchClass
	// Mode is the key's mode.
	Mode Mode
	// Correlation is the Pearson correlation of the chroma with the key's
	// profile, in [-1, 1].
	Correlation float64
}

// String returns the key name, for example "G major" (sharp spelling).
func (c Candidate) String() string { return keyName(c.Tonic, c.Mode) }

func keyName(tonic pitch.PitchClass, mode Mode) string { return tonic.String() + " " + mode.String() }

// Key is the result of [EstimateKey]. The zero Key is C major (with zero
// correlations); [Chords] and [Chorder.Chords] read only Tonic and Mode.
type Key struct {
	// Tonic is the tonic pitch class of the estimated key.
	Tonic pitch.PitchClass
	// Mode is the mode of the estimated key.
	Mode Mode
	// Correlation is the Pearson correlation of the chroma with the key's
	// profile. It is not rounded.
	Correlation float64
	// RunnerUp is the second key: the next best correlation, or, when the
	// relative tie was resolved, the other key of the tie.
	RunnerUp Candidate
	// Relative is the relative minor (of a major key) or relative major (of
	// a minor key) with its correlation.
	Relative Candidate
	// Margin is Correlation − RunnerUp.Correlation. It is negative when
	// tonic evidence chose the relative key over a slightly better
	// correlating one.
	Margin float64
	// Ambiguous reports that the best correlating key and its relative were
	// within the tie margin, so the tonic evidence decided between them.
	Ambiguous bool
	// Tied holds the tied pair when Ambiguous is set, the key that was
	// leading at that point first. It is zero otherwise.
	Tied [2]Candidate
}

// String returns the key name, for example "E minor".
func (k Key) String() string { return keyName(k.Tonic, k.Mode) }

// Scale returns the key's scale: [pitch.ScaleMajor] for a major key and
// [pitch.ScaleNaturalMinor] for a minor key. An invalid tonic or mode yields
// the zero [pitch.Scale], which contains no pitch class.
func (k Key) Scale() pitch.Scale {
	if k.Tonic >= pitchClasses {
		return pitch.Scale{}
	}

	switch k.Mode {
	case Major:
		return pitch.ScaleMajor(k.Tonic)
	case Minor:
		return pitch.ScaleNaturalMinor(k.Tonic)
	default:
		return pitch.Scale{}
	}
}

// Diatonic reports whether pc belongs to the key's scale (natural minor for
// minor keys).
func (k Key) Diatonic(pc pitch.PitchClass) bool { return k.Scale().Contains(pc) }

// Sharps returns the number of sharps in the key signature, negative for
// flats, in [-5, 6]. A minor key has the signature of its relative major, and
// F♯ major (G♭ major) is spelled with six sharps.
func (k Key) Sharps() int {
	major := int(k.Tonic) % pitchClasses
	if k.Mode == Minor {
		major = (major + 3) % pitchClasses
	}

	s := major * 7 % pitchClasses
	if s > 6 {
		s -= pitchClasses
	}

	return s
}

func (k Key) validate() error {
	if k.Tonic >= pitchClasses || !k.Mode.valid() {
		return fmt.Errorf("%w: key tonic %d, mode %d", ErrInvalidInput, uint8(k.Tonic), uint8(k.Mode))
	}

	return nil
}

// KeyOption configures [EstimateKey]. Options return an error wrapping
// [ErrInvalidOption] for out-of-range values.
type KeyOption func(*keyConfig) error

type keyConfig struct {
	tie      float64
	evidence [pitchClasses]float64
	profiles ProfileSet
}

// WithTieMargin sets the correlation margin within which the best key and
// its relative count as tied (default [DefaultTieMargin]). The tie is strict:
// the difference must be below the margin, so 0 disables tie breaking.
func WithTieMargin(margin float64) KeyOption {
	return func(cfg *keyConfig) error {
		if !finite(margin) || margin < 0 {
			return fmt.Errorf("%w: tie margin must be >= 0, got %g", ErrInvalidOption, margin)
		}

		cfg.tie = margin

		return nil
	}
}

// WithTonicEvidence supplies per pitch class evidence for the tonic, used
// only to break a tie between relative keys: the relative replaces the best
// correlating key when its tonic has strictly more evidence. Bass weight at
// section starts and ends is a good choice. Without this option the evidence
// is all zero and the correlation order stands.
func WithTonicEvidence(evidence [12]float64) KeyOption {
	return func(cfg *keyConfig) error {
		for i, v := range evidence {
			if !finite(v) {
				return fmt.Errorf("%w: tonic evidence %d is %g", ErrInvalidOption, i, v)
			}
		}

		cfg.evidence = evidence

		return nil
	}
}

// WithProfileSet selects the key profiles (default [KrumhanslKessler]).
func WithProfileSet(set ProfileSet) KeyOption {
	return func(cfg *keyConfig) error {
		if set != KrumhanslKessler {
			return fmt.Errorf("%w: unknown profile set %d", ErrInvalidOption, uint8(set))
		}

		cfg.profiles = set

		return nil
	}
}

func newKeyConfig(opts []KeyOption) (keyConfig, error) {
	cfg := keyConfig{tie: DefaultTieMargin, profiles: KrumhanslKessler}

	for i, opt := range opts {
		if opt == nil {
			return cfg, fmt.Errorf("%w: key option %d", ErrNilOption, i)
		}

		err := opt(&cfg)
		if err != nil {
			return cfg, err
		}
	}

	return cfg, nil
}

type keyScore struct {
	tonic int
	mode  Mode
	r     float64
}

func (s keyScore) candidate() Candidate {
	return Candidate{Tonic: pitch.PitchClass(s.tonic), Mode: s.mode, Correlation: s.r}
}

// relativeOf returns the tonic and mode of the relative key.
func relativeOf(tonic int, mode Mode) (int, Mode) {
	if mode == Minor {
		return (tonic + 3) % pitchClasses, Major
	}

	return (tonic + 9) % pitchClasses, Minor
}

// EstimateKey estimates the key of a chroma (pitch-class) profile, indexed by
// [pitch.PitchClass]. It correlates the chroma with the major and minor
// profiles (default [KrumhanslKessler]) rotated to all 12 tonics and picks the
// best of the 24 keys.
//
// Relative keys (C major and A minor) share their pitch set, so their
// correlations are often close. When the relative of the best key correlates
// within the tie margin (default [DefaultTieMargin]), the tonic evidence of
// [WithTonicEvidence] decides between them and [Key.Ambiguous] is set.
//
// The chroma values must be finite and must not all be equal; a flat chroma
// returns an error wrapping [ErrFlatChroma].
func EstimateKey(chroma [12]float64, opts ...KeyOption) (Key, error) {
	cfg, err := newKeyConfig(opts)
	if err != nil {
		return Key{}, err
	}

	flat := true

	for i, v := range chroma {
		if !finite(v) {
			return Key{}, fmt.Errorf("%w: chroma %d is %g", ErrInvalidInput, i, v)
		}

		flat = flat && v == chroma[0]
	}

	if flat {
		return Key{}, fmt.Errorf("%w: all twelve values are %g", ErrFlatChroma, chroma[0])
	}

	major, _ := cfg.profiles.Profile(Major)
	minor, _ := cfg.profiles.Profile(Minor)

	var scores [2 * pitchClasses]keyScore

	n := 0

	for tonic := range pitchClasses {
		for _, mode := range [2]Mode{Major, Minor} {
			profile := major
			if mode == Minor {
				profile = minor
			}

			var rot [pitchClasses]float64
			for i := range rot {
				rot[(tonic+i)%pitchClasses] = profile[i]
			}

			scores[n] = keyScore{tonic: tonic, mode: mode, r: pearson(chroma, rot)}
			n++
		}
	}

	sortScores(&scores)

	best, runner := scores[0], scores[1]
	relTonic, _ := relativeOf(best.tonic, best.mode)

	var key Key

	// The loop deliberately keeps comparing against the current best after a
	// swap, as the AudioVisualizer original does: if the relative wins, a
	// later candidate on the same tonic in the other mode (its parallel key)
	// within the margin becomes the runner-up.
	for _, s := range scores[1:] {
		if s.tonic == relTonic && s.mode != best.mode && best.r-s.r < cfg.tie {
			key.Ambiguous = true
			key.Tied = [2]Candidate{best.candidate(), s.candidate()}

			if cfg.evidence[s.tonic] > cfg.evidence[best.tonic] {
				best, runner = s, best
			} else {
				runner = s
			}
		}
	}

	key.Tonic, key.Mode, key.Correlation = pitch.PitchClass(best.tonic), best.mode, best.r
	key.RunnerUp = runner.candidate()
	key.Margin = best.r - runner.r

	relTonic, relMode := relativeOf(best.tonic, best.mode)

	for _, s := range scores {
		if s.tonic == relTonic && s.mode == relMode {
			key.Relative = s.candidate()
		}
	}

	return key, nil
}

// sortScores sorts by descending correlation. It is a stable insertion sort,
// so equal correlations keep the generation order (C major, C minor, C♯
// major, ...), the same order sort.SliceStable produces.
func sortScores(s *[2 * pitchClasses]keyScore) {
	for i := 1; i < len(s); i++ {
		x := s[i]
		j := i

		for j > 0 && x.r > s[j-1].r {
			s[j] = s[j-1]
			j--
		}

		s[j] = x
	}
}

// pearson is the Pearson correlation of a and b, 0 when either is constant.
// The operation order matches the AudioVisualizer original.
func pearson(a, b [pitchClasses]float64) float64 {
	ma, mb := 0.0, 0.0
	for i := range a {
		ma += a[i] / pitchClasses
		mb += b[i] / pitchClasses
	}

	num, da, db := 0.0, 0.0, 0.0
	for i := range a {
		num += (a[i] - ma) * (b[i] - mb)
		da += (a[i] - ma) * (a[i] - ma)
		db += (b[i] - mb) * (b[i] - mb)
	}

	if da == 0 || db == 0 {
		return 0
	}

	return num / math.Sqrt(da*db)
}

func finite(x float64) bool { return !math.IsNaN(x) && !math.IsInf(x, 0) }
