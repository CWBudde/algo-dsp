package time

import (
	"fmt"
	"math"
)

// Analysis contains finite streaming time statistics. CrestFactor is a linear
// peak/RMS ratio and is zero for silence. ClippedSamples counts |sample|>=1.
type Analysis struct {
	Samples, ZeroCrossings, ClippedSamples int64
	Peak, RMS, DC, CrestFactor             float64
}

// Accumulator retains bounded scalar state for streaming statistics. Add and
// Add32 reject nonfinite samples or a frame-limit violation before mutation.
// Zero crossings require adjacent strictly opposite signs; zeros do not count.
type Accumulator struct {
	limit, samples, crossings, clipped   int64
	peak, mean, scale, squares, previous float64
}

// NewAccumulator configures a positive maximum number of samples.
func NewAccumulator(maxFrames int64) (*Accumulator, error) {
	if maxFrames < 1 {
		return nil, fmt.Errorf("time accumulator: maximum frames must be positive")
	}

	return &Accumulator{limit: maxFrames}, nil
}

// Add incorporates a finite float64 block without allocation.
func (a *Accumulator) Add(samples []float64) error { return accumulate(a, samples) }

// Add32 incorporates a finite float32 block without allocation.
func (a *Accumulator) Add32(samples []float32) error { return accumulate(a, samples) }

func accumulate[T ~float32 | ~float64](a *Accumulator, samples []T) error {
	if a == nil || a.limit < 1 || int64(len(samples)) > a.limit-a.samples {
		return fmt.Errorf("time accumulator: sample limit exceeded or unconfigured")
	}

	for _, raw := range samples {
		x := float64(raw)
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return fmt.Errorf("time accumulator: nonfinite input")
		}
	}

	for _, raw := range samples {
		x := float64(raw)
		if a.samples > 0 && ((a.previous < 0 && x > 0) || (a.previous > 0 && x < 0)) {
			a.crossings++
		}

		a.previous = x
		a.samples++
		n := float64(a.samples)
		a.mean = a.mean*((n-1)/n) + x/n
		v := math.Abs(x)

		a.peak = math.Max(a.peak, v)
		if v >= 1 {
			a.clipped++
		}

		if v > 0 {
			if v > a.scale {
				r := a.scale / v
				a.squares = 1 + a.squares*r*r
				a.scale = v
			} else {
				r := v / a.scale
				a.squares += r * r
			}
		}
	}

	return nil
}

// Result returns the current statistics without changing state or allocating.
func (a *Accumulator) Result() Analysis {
	if a == nil || a.samples == 0 {
		return Analysis{}
	}

	rms := a.scale * math.Sqrt(a.squares/float64(a.samples))

	crest := 0.0
	if rms > 0 {
		crest = a.peak / rms
	}

	return Analysis{Samples: a.samples, ZeroCrossings: a.crossings, ClippedSamples: a.clipped, Peak: a.peak, RMS: rms, DC: a.mean, CrestFactor: crest}
}

// Reset clears measurements while retaining the configured sample limit.
func (a *Accumulator) Reset() {
	if a != nil {
		limit := a.limit
		*a = Accumulator{limit: limit}
	}
}

// ClipDetector emits half-open runs at which |sample|>=threshold. Runs that
// cross block boundaries are emitted once; Flush closes the final run.
type ClipDetector struct {
	threshold     float64
	next, start   int64
	started, open bool
}

// NewClipDetector requires a finite positive amplitude threshold.
func NewClipDetector(threshold float64) (*ClipDetector, error) {
	if threshold <= 0 || math.IsNaN(threshold) || math.IsInf(threshold, 0) {
		return nil, fmt.Errorf("clip detector: threshold must be finite and positive")
	}

	return &ClipDetector{threshold: threshold}, nil
}

// Process reads sequential finite float64 samples. emit must be nonnil and
// must not reenter the detector; sample validation happens before any emission.
func (d *ClipDetector) Process(src []float64, offset int64, emit func(start, end int64)) error {
	return detectClips(d, src, offset, emit)
}

// Process32 is the allocation-free float32 form of Process.
func (d *ClipDetector) Process32(src []float32, offset int64, emit func(start, end int64)) error {
	return detectClips(d, src, offset, emit)
}

func detectClips[T ~float32 | ~float64](d *ClipDetector, src []T, offset int64, emit func(start, end int64)) error {
	if d == nil || d.threshold <= 0 || emit == nil || offset < 0 || int64(len(src)) > math.MaxInt64-offset || (d.started && offset != d.next) {
		return fmt.Errorf("clip detector: invalid state, callback or nonsequential range")
	}

	for _, raw := range src {
		x := float64(raw)
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return fmt.Errorf("clip detector: nonfinite input")
		}
	}

	for i, raw := range src {
		position := offset + int64(i)

		if math.Abs(float64(raw)) >= d.threshold {
			if !d.open {
				d.start = position
				d.open = true
			}
		} else if d.open {
			emit(d.start, position)
			d.open = false
		}
	}

	d.next = offset + int64(len(src))
	d.started = true

	return nil
}

// Flush emits an unfinished final run once. It does not reset stream offsets.
func (d *ClipDetector) Flush(emit func(start, end int64)) {
	if d != nil && d.open && emit != nil {
		emit(d.start, d.next)
		d.open = false
	}
}

// Reset clears runs and offsets while retaining the threshold.
func (d *ClipDetector) Reset() {
	if d != nil {
		threshold := d.threshold
		*d = ClipDetector{threshold: threshold}
	}
}
