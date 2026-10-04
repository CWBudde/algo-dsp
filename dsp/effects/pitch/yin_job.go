package pitch

import (
	"fmt"
	"math"
)

// YINJob evaluates the existing exact YIN detector cooperatively. It retains
// owned frame/difference buffers; no downsampling, search truncation or changed
// pitch criterion is introduced. One work unit is one scalar RMS/difference,
// normalization or search iteration. Steps and resets allocate nothing.
type YINJob struct {
	d                       YINDetector
	input                   []float64
	phase, index, tau, best int
	sum, running            float64
	estimate                PitchEstimate
	active, done, crossed   bool
}

// NewYINJob snapshots a configured detector and reserves independent scratch.
// Jobs accept frames up to65536 samples; later detector configuration changes
// cannot affect an existing job.
func NewYINJob(detector *YINDetector) (*YINJob, error) {
	if detector == nil || detector.frameSize < 1 || detector.frameSize > 65536 || detector.integration < 1 {
		return nil, fmt.Errorf("yin.job.new: invalid or oversized detector")
	}

	j := &YINJob{d: *detector, input: make([]float64, detector.frameSize)}
	j.d.diff = make([]float64, len(detector.diff))
	j.d.cmnd = make([]float64, len(detector.cmnd))

	return j, nil
}

// Begin copies the next finite frame. It rejects invalid input without changing
// a pending or completed result. Only FrameSize samples are used, as in Detect.
func (j *YINJob) Begin(frame []float64) error {
	if j == nil || len(j.input) == 0 {
		return fmt.Errorf("yin.job.begin: unconfigured job")
	}

	if len(frame) < len(j.input) {
		return fmt.Errorf("yin.job.begin: short frame")
	}

	for _, x := range frame[:len(j.input)] {
		if math.IsNaN(x) || math.IsInf(x, 0) || math.Abs(x) > 1e100 {
			return fmt.Errorf("yin.job.begin: nonfinite or unrepresentable frame")
		}
	}

	copy(j.input, frame)
	j.phase = 0
	j.index = 0
	j.tau = 1
	j.best = j.d.tauMin
	j.sum = 0
	j.running = 0
	j.estimate = PitchEstimate{}
	j.active = true
	j.done = false
	j.crossed = false
	j.d.diff[0] = 0
	j.d.cmnd[0] = 1

	return nil
}

// Step evaluates at most maxWork scalar iterations, bounded to65536 per call.
// Completion returns true and permits Result; callers may cancel between steps.
func (j *YINJob) Step(maxWork int) (bool, error) {
	if j == nil || !j.active || maxWork < 1 || maxWork > 65536 {
		return false, fmt.Errorf("yin.job.step: invalid state or work budget")
	}

	if j.done {
		return true, nil
	}

	for work := 0; work < maxWork && !j.done; work++ {
		switch j.phase {
		case 0:
			x := j.input[j.index]
			j.sum += x * x

			j.index++
			if j.index == len(j.input) {
				j.estimate.RMS = math.Sqrt(j.sum / float64(len(j.input)))
				j.index = 0
				j.sum = 0

				j.phase = 1
				if j.estimate.RMS < j.d.silenceRMS {
					j.estimate.Aperiodicity = 1
					j.done = true
				}
			}
		case 1:
			delta := j.input[j.index] - j.input[j.index+j.tau]
			j.sum += delta * delta

			j.index++
			if j.index == j.d.integration {
				j.d.diff[j.tau] = j.sum
				j.tau++
				j.index = 0

				j.sum = 0
				if j.tau > j.d.tauMax {
					j.tau = 1
					j.phase = 2
				}
			}
		case 2:
			j.running += j.d.diff[j.tau]

			j.d.cmnd[j.tau] = 1
			if j.running >= yinTiny {
				j.d.cmnd[j.tau] = j.d.diff[j.tau] * float64(j.tau) / j.running
			}

			j.tau++
			if j.tau > j.d.tauMax {
				j.tau = j.d.tauMin
				j.phase = 3
			}
		case 3:
			if j.d.cmnd[j.tau] < j.d.cmnd[j.best] {
				j.best = j.tau
			}

			if j.d.cmnd[j.tau] < j.d.threshold {
				j.crossed = true
				j.phase = 4
			} else {
				j.tau++
				if j.tau > j.d.tauMax {
					j.tau = j.best
					j.phase = 5
				}
			}
		case 4:
			if j.tau+1 <= j.d.tauMax && j.d.cmnd[j.tau+1] < j.d.cmnd[j.tau] {
				j.tau++
			} else {
				j.phase = 5
			}
		case 5:
			refined, value := j.d.refine(j.tau)
			j.estimate.Aperiodicity = clamp01(value)

			j.estimate.Confidence = clamp01(1 - j.estimate.Aperiodicity)
			if j.crossed {
				j.estimate.Tau = refined
				j.estimate.FrequencyHz = j.d.sampleRate / refined
				j.estimate.Voiced = true
			}

			j.done = true
		}
	}

	return j.done, nil
}

// Result returns the completed estimate without allocating.
func (j *YINJob) Result() (PitchEstimate, error) {
	if j == nil || !j.done {
		return PitchEstimate{}, fmt.Errorf("yin.job.result: unfinished analysis")
	}

	return j.estimate, nil
}

// Reset cancels pending work and clears retained samples and scratch.
func (j *YINJob) Reset() {
	clear(j.input)
	clear(j.d.diff)
	clear(j.d.cmnd)
	j.active = false
	j.done = false
}
