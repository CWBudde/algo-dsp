package restoration

import (
	"context"
	"fmt"

	"github.com/cwbudde/algo-dsp/dsp/stft"
)

// NoiseCapture owns STFT framing and the averaged profile for a bounded source.
// Capture uses complete Hann windows at FFTSize/4 hops, starting at sample zero,
// and a final right-aligned window. Short sources (at least half a window) are
// reflected to fill one window; capture never dilutes power with zero padding.
// Step performs at most one FFT and allocates no scratch. One caller owns capture.
type NoiseCapture struct {
	profile                 *NoiseProfile
	transform               *stft.STFT
	reader                  Reader
	raw                     []float64
	bins                    []complex128
	length, start, progress int64
	done                    bool
	failure                 error
}

// NewNoiseCapture prepares framing. reader copies requested valid intervals from
// a source of length samples. Invalid format or length is rejected before reading.
func NewNoiseCapture(length int64, reader Reader, fftSize int, sampleRate float64) (*NoiseCapture, error) {
	profile, err := NewNoiseProfile(fftSize, sampleRate)
	if err != nil {
		return nil, err
	}

	if reader == nil || length < int64(fftSize/2) || length > 1<<52 {
		return nil, fmt.Errorf("restoration.capture: invalid source")
	}

	transform, err := stft.New(fftSize, fftSize/4, stft.WithCenter(stft.PadNone))
	if err != nil {
		return nil, fmt.Errorf("restoration.capture: transform: %w", err)
	}

	return &NoiseCapture{profile: profile, transform: transform, reader: reader, length: length, raw: make([]float64, fftSize), bins: make([]complex128, fftSize/2+1)}, nil
}

// Progress returns the source sample count covered by completed windows.
func (c *NoiseCapture) Progress() int64 { return c.progress }

// Step captures one window, honouring cancellation and rejecting short/nonfinite
// reads. Failures are terminal and no incomplete profile can be retrieved.
func (c *NoiseCapture) Step(ctx context.Context) (bool, error) {
	if c == nil || ctx == nil {
		return false, fmt.Errorf("restoration.capture: invalid context or state")
	}

	if c.failure != nil {
		return false, c.failure
	}

	if c.done {
		return true, nil
	}

	if err := ctx.Err(); err != nil {
		return c.fail(err)
	}

	count := int(min(int64(len(c.raw)), c.length-c.start))
	if c.reader(c.raw[:count], c.start) != count {
		return c.fail(fmt.Errorf("short source read"))
	}

	for i := count; i < len(c.raw); i++ {
		position := i % (2 * (count - 1))
		if position >= count {
			position = 2*(count-1) - position
		}

		c.raw[i] = c.raw[position]
	}

	if err := c.transform.FrameInto(c.bins, c.raw, 0); err != nil {
		return c.fail(err)
	}

	if err := ctx.Err(); err != nil {
		return c.fail(err)
	}

	if err := c.profile.AddSpectrum(c.bins); err != nil {
		return c.fail(err)
	}

	c.progress = min(c.length, c.start+int64(len(c.raw)))

	c.done = c.progress == c.length
	if !c.done {
		c.start = min(c.start+int64(len(c.raw)/4), c.length-int64(len(c.raw)))
	}

	return c.done, nil
}

// Profile returns the complete profile only after successful capture. The caller
// owns the returned profile; no further spectra should be added during reduction.
func (c *NoiseCapture) Profile() (*NoiseProfile, error) {
	if c == nil {
		return nil, fmt.Errorf("restoration.capture: nil capture")
	}

	if c.failure != nil {
		return nil, c.failure
	}

	if !c.done {
		return nil, fmt.Errorf("restoration.capture: incomplete profile")
	}

	return c.profile, nil
}

func (c *NoiseCapture) fail(err error) (bool, error) {
	c.failure = fmt.Errorf("restoration.capture: %w", err)
	return false, c.failure
}
