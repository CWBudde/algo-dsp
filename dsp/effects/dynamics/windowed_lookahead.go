package dynamics

import (
	"fmt"
	"math"
)

// LookaheadAttack selects how the detector smooths peak reduction.
type LookaheadAttack uint8

const (
	// LookaheadLinear uses a single rectangular moving-average window.
	LookaheadLinear LookaheadAttack = iota
	// LookaheadParabolic cascades two half-length moving-average windows.
	LookaheadParabolic
)

// WindowedLookahead tracks peaks ahead of delayed audio and smooths the
// resulting gain reduction. It implements the detector used by the Delphi
// ASIO and VST Project's feedforward lookahead limiter.
type WindowedLookahead struct {
	history []float32
	windows [2][]float32
	samples int
	shape   LookaheadAttack

	inputFactor   float64
	outputFactor  float64
	releaseFactor float64
	inverseLength float64

	position       int
	windowPosition int
	holdRemaining  int
	holdValue      float32
	peak           float32
	sums           [2]float64
}

// NewWindowedLookahead reserves storage for the maximum even lookahead length.
// Configure must be called before processing.
func NewWindowedLookahead(maxSamples int) (*WindowedLookahead, error) {
	if maxSamples < 2 || maxSamples%2 != 0 {
		return nil, fmt.Errorf("maximum lookahead must be a positive even length: %d", maxSamples)
	}

	l := &WindowedLookahead{history: make([]float32, maxSamples)}
	for i := range l.windows {
		l.windows[i] = make([]float32, maxSamples/2)
	}

	return l, nil
}

// Configure sets the detector and output gain, release half-life in
// milliseconds, and attack window. Changing the lookahead length clears the
// delay and detector history. Successful calls reuse the reserved storage.
func (l *WindowedLookahead) Configure(sampleRate float64, samples int, inputDB, outputDB, releaseMS float64, shape LookaheadAttack) error {
	if err := validateSampleRate(sampleRate); err != nil {
		return err
	}

	if samples < 2 || samples > len(l.history) || samples%2 != 0 {
		return fmt.Errorf("lookahead must be even and between 2 and %d samples: %d", len(l.history), samples)
	}

	if !isFinite(inputDB) || !isFinite(outputDB) || !isFinite(releaseMS) || releaseMS <= 0 {
		return fmt.Errorf("invalid lookahead gain or release")
	}

	if shape != LookaheadLinear && shape != LookaheadParabolic {
		return fmt.Errorf("unknown lookahead attack: %d", shape)
	}

	if l.samples != samples {
		l.samples = samples
		l.inverseLength = 1 / float64(samples)
		l.Reset()
	}

	l.shape = shape
	l.inputFactor = float64(float32(math.Pow(10, inputDB/20)))
	l.outputFactor = float64(float32(math.Pow(10, outputDB/20)))
	l.releaseFactor = math.Exp2(-1 / (releaseMS * 0.001 * sampleRate))

	return nil
}

// Reset clears the delay, held peak and attack windows.
func (l *WindowedLookahead) Reset() {
	clear(l.history)
	clear(l.windows[0])
	clear(l.windows[1])
	l.position = 0
	l.windowPosition = 0
	l.holdRemaining = 0
	l.holdValue = 0
	l.peak = 0
	l.sums = [2]float64{}
}

// ProcessSample returns the delayed sample and the gain to apply at this
// instant. With linked channels the caller can apply the most restrictive
// gain to independently delayed audio.
func (l *WindowedLookahead) ProcessSample(input float32) (delayed, gain float32) {
	l.trackPeak(float32(math.Abs(float64(input))))
	delayed = l.history[l.position]
	l.history[l.position] = input

	l.position++
	if l.position == l.samples {
		l.position = 0
	}

	l.peak = float32(l.releaseFactor * float64(l.peak))

	driven := float32(l.inputFactor * float64(l.holdValue))
	if driven > 1 {
		l.peak = max(l.peak, float32((float64(driven)-1)/float64(driven)))
	}

	reduction := l.smoothPeak()
	gain = float32(l.outputFactor * l.inputFactor * (1 - float64(reduction)))

	return delayed, gain
}

func (l *WindowedLookahead) trackPeak(magnitude float32) {
	switch {
	case magnitude >= l.holdValue:
		l.holdValue = magnitude
		l.holdRemaining = l.samples
	case l.holdRemaining > 0:
		l.holdRemaining--
	default:
		l.holdValue = float32(math.Abs(float64(l.history[0])))
		next := 0

		for i := 1; i < l.samples; i++ {
			candidate := float32(math.Abs(float64(l.history[i])))
			if candidate > l.holdValue {
				l.holdValue = candidate
				next = i + 1
			}
		}

		l.holdRemaining = next - l.position
		if l.holdRemaining < 0 {
			l.holdRemaining += l.samples
		}

		if magnitude > l.holdValue {
			l.holdValue = magnitude
			l.holdRemaining = l.samples
		}
	}
}

func (l *WindowedLookahead) smoothPeak() float32 {
	position := l.windowPosition

	var reduction float32

	if l.shape == LookaheadLinear {
		l.sums[1] += float64(l.peak) - float64(l.windows[1][position])
		l.windows[1][position] = l.windows[0][position]
		l.windows[0][position] = l.peak
		reduction = float32(l.sums[1] * l.inverseLength)
	} else {
		l.sums[0] += float64(l.peak) - float64(l.windows[0][position])
		l.windows[0][position] = l.peak
		first := float32(l.sums[0] * 2 * l.inverseLength)
		l.sums[1] += float64(first) - float64(l.windows[1][position])
		l.windows[1][position] = first
		reduction = float32(l.sums[1] * 2 * l.inverseLength)
	}

	l.windowPosition++
	if l.windowPosition == l.samples/2 {
		l.windowPosition = 0
	}

	return reduction
}
