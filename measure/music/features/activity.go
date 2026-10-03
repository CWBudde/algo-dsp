package features

import (
	"fmt"
	"maps"
	"math"
	"slices"

	"github.com/cwbudde/algo-dsp/dsp/core"
)

// Defaults of [Activity].
const (
	// DefaultActivityThresholdDB is the default activity threshold: a track
	// is active in a span whose level is more than 12 dB below its reference
	// level.
	DefaultActivityThresholdDB = -12.0
	// DefaultActivityPercentile is the default percentile of a track's
	// above-gate frame levels that serves as its reference level.
	DefaultActivityPercentile = 0.95
)

// TrackActivity is the activity of one track, as returned by [Activity].
type TrackActivity struct {
	// ReferenceDB is the track's reference level in dBFS: the percentile of
	// its frame levels above the gate. It is the -120 dB floor when no frame
	// exceeds the gate.
	ReferenceDB float64
	// SpanDB holds the level of each span in dBFS: the RMS (power mean) of
	// the frame levels in the span, -120 dB for a span without frames.
	SpanDB []float64
	// Active reports for each span whether SpanDB exceeds ReferenceDB plus
	// the threshold. A track without frames above the gate is never active.
	Active []bool
}

// ActivityOption configures [Activity].
type ActivityOption func(*activityConfig) error

type activityConfig struct {
	thresholdDB float64
	gate        float64
	percentile  float64
}

// WithActivityThreshold sets the activity threshold in dB relative to the
// track's reference level (default -12 dB). A span is active when its level
// is strictly greater than reference + db.
func WithActivityThreshold(db float64) ActivityOption {
	return func(c *activityConfig) error {
		if math.IsNaN(db) || math.IsInf(db, 0) {
			return fmt.Errorf("%w: activity threshold %v dB", ErrInvalidArgument, db)
		}

		c.thresholdDB = db

		return nil
	}
}

// WithActivityGate sets the linear frame level (default [DefaultGate],
// -80 dBFS) that a frame must exceed to count towards the reference level.
func WithActivityGate(gate float64) ActivityOption {
	return func(c *activityConfig) error {
		if !(gate >= 0) || math.IsInf(gate, 0) {
			return fmt.Errorf("%w: activity gate %v", ErrInvalidArgument, gate)
		}

		c.gate = gate

		return nil
	}
}

// WithActivityPercentile sets the percentile p (0..1, default 0.95) of the
// above-gate frame levels used as the reference level. It selects the
// element at index floor(p·(count-1)) of the sorted levels; note that
// [Percentile] rounds the index instead.
func WithActivityPercentile(p float64) ActivityOption {
	return func(c *activityConfig) error {
		if !(p >= 0 && p <= 1) {
			return fmt.Errorf("%w: activity percentile %v", ErrInvalidArgument, p)
		}

		c.percentile = p

		return nil
	}
}

// Activity reports, for each named track and each span, whether the track
// is active in that span: whether the span's level is within the threshold
// (-12 dB by default) of the track's own reference level, its 95th
// percentile frame level. Because each track is measured against itself, a
// quiet track that is present counts as active just like a loud one.
//
// levels maps a track name to its linear frame levels (for example
// [Frames].RMS) at frameRate frames per second; the levels must be finite
// and non-negative. spans are [Start, End) intervals in seconds with
// End >= Start; a span covers the frames ceil(Start·frameRate) up to
// ceil(End·frameRate), clamped to the track. Spans may extend beyond the
// track (for example a pickup bar before time 0).
//
// Per track, with the defaults:
//
//	ReferenceDB = dB(sorted{v > gate}[floor(0.95·(count-1))])
//	SpanDB[s]   = dB(sqrt(Σ v² / frames in s))
//	Active[s]   = SpanDB[s] > ReferenceDB - 12
//
// where dB is 20·log10 with a 1e-6 (-120 dB) floor. This is the bar
// activity of AudioVisualizer's internal/story/roles.go, reproduced bit for
// bit, except that a track without any frame above the gate is inactive
// everywhere (the app would mark it active). The result has one entry per
// track and is never nil.
func Activity(levels map[string][]float64, frameRate float64, spans []Interval, opts ...ActivityOption) (map[string]TrackActivity, error) {
	cfg := activityConfig{
		thresholdDB: DefaultActivityThresholdDB,
		gate:        DefaultGate,
		percentile:  DefaultActivityPercentile,
	}

	for i, opt := range opts {
		if opt == nil {
			return nil, fmt.Errorf("%w: activity option %d", ErrNilOption, i)
		}

		err := opt(&cfg)
		if err != nil {
			return nil, err
		}
	}

	err := validateActivity(levels, frameRate, spans)
	if err != nil {
		return nil, err
	}

	out := make(map[string]TrackActivity, len(levels))

	longest := 0
	for _, v := range levels {
		longest = max(longest, len(v))
	}

	above := make([]float64, 0, longest)

	for name, v := range levels {
		above = above[:0]

		for _, x := range v {
			if x > cfg.gate {
				above = append(above, x)
			}
		}

		act := TrackActivity{
			ReferenceDB: core.LinearToDBFloor(0, DBFloor),
			SpanDB:      make([]float64, len(spans)),
			Active:      make([]bool, len(spans)),
		}

		if len(above) > 0 {
			slices.Sort(above)
			act.ReferenceDB = core.LinearToDBFloor(above[int(cfg.percentile*float64(len(above)-1))], DBFloor)
		}

		for s, span := range spans {
			act.SpanDB[s] = spanLevelDB(v, span, frameRate)
			act.Active[s] = len(above) > 0 && act.SpanDB[s] > act.ReferenceDB+cfg.thresholdDB
		}

		out[name] = act
	}

	return out, nil
}

// spanLevelDB is the RMS of the levels in span, in dBFS.
func spanLevelDB(v []float64, span Interval, frameRate float64) float64 {
	lo := frameIndex(span.Start, frameRate, len(v))
	hi := max(lo, frameIndex(span.End, frameRate, len(v)))

	sum := 0.0
	for i := lo; i < hi; i++ {
		sum += v[i] * v[i]
	}

	if hi <= lo {
		return core.LinearToDBFloor(0, DBFloor)
	}

	return core.LinearToDBFloor(math.Sqrt(sum/float64(hi-lo)), DBFloor)
}

// frameIndex returns ceil(t·frameRate) clamped to [0, n].
func frameIndex(t, frameRate float64, n int) int {
	f := math.Ceil(t * frameRate)
	if !(f > 0) {
		return 0
	}

	if f >= float64(n) {
		return n
	}

	return int(f)
}

func validateActivity(levels map[string][]float64, frameRate float64, spans []Interval) error {
	if !(frameRate > 0) || math.IsInf(frameRate, 0) {
		return fmt.Errorf("%w: frame rate %v", ErrInvalidArgument, frameRate)
	}

	for i, s := range spans {
		if math.IsNaN(s.Start) || math.IsInf(s.Start, 0) || math.IsNaN(s.End) || math.IsInf(s.End, 0) || s.End < s.Start {
			return fmt.Errorf("%w: span %d [%v, %v)", ErrInvalidArgument, i, s.Start, s.End)
		}
	}

	for _, name := range slices.Sorted(maps.Keys(levels)) {
		for i, x := range levels[name] {
			if !(x >= 0) || math.IsInf(x, 0) {
				return fmt.Errorf("%w: track %q frame %d level %v", ErrInvalidArgument, name, i, x)
			}
		}
	}

	return nil
}
