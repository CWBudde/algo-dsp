package delay

import "github.com/cwbudde/algo-dsp/dsp/interp"

// TimeHermite32 is the four-point fractional delay used by Delphi ASIO &
// VST's TDelayLineTime32. The source's cursor and interpolation history give
// an integer latency of max(1, trunc(samples)-1) before interpolation.
type TimeHermite32 struct {
	buffer     []float32
	history    [4]float32
	fraction   float64
	integer    int
	position   int
	written    int
	activeSize int
}

// NewTimeHermite32 allocates capacity for every supported delay setting.
// maxSamples must be nonnegative.
func NewTimeHermite32(maxSamples int) *TimeHermite32 {
	return &TimeHermite32{buffer: make([]float32, max(1, maxSamples)), activeSize: 1}
}

// SetSamples changes the delay without allocating. Changing the integer part
// discards the old ring contents and keeps the four interpolation taps.
func (d *TimeHermite32) SetSamples(samples float64) {
	samples = min(max(0, samples), float64(len(d.buffer)))
	n := int(samples)

	d.fraction = float64(float32(samples - float64(n)))
	if n != d.integer {
		d.integer = n
		d.activeSize = max(1, n-1)
		d.position = 0
		d.written = 0
	}
}

// ProcessSample writes a sample and reads the interpolated detector tap.
func (d *TimeHermite32) ProcessSample(input float32) float32 {
	d.buffer[d.position] = input

	d.position++
	if d.position == d.activeSize {
		d.position = 0
	}

	d.written = min(d.activeSize, d.written+1)

	var tap float32
	if d.written == d.activeSize {
		tap = d.buffer[d.position]
	}

	d.history = [4]float32{tap, d.history[0], d.history[1], d.history[2]}

	return float32(interp.Hermite4(d.fraction, float64(d.history[0]), float64(d.history[1]),
		float64(d.history[2]), float64(d.history[3])))
}
