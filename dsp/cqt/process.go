package cqt

import (
	"fmt"
	"math"

	"github.com/cwbudde/algo-dsp/dsp/conv"
	"github.com/cwbudde/algo-dsp/dsp/core"
)

// sample is the element type of the output slices.
type sample interface{ ~float32 | ~float64 }

// topLength returns the length of the top octave's signal for n input
// samples, or false if the signal cannot be processed: it is empty, too short
// to be decimated down to the lowest octave (every decimation needs at least
// 2 input samples), or, with [PadReflect] outside the [NNAudio] preset, its
// lowest octave is not longer than nfft/2 samples and cannot be reflected.
func (t *Transform) topLength(n int) (int, bool) {
	if n < 1 {
		return 0, false
	}

	if t.factor > 1 {
		n = decimatedLength(n, lowpassTaps, t.factor)
		if n < 1 {
			return 0, false
		}
	}

	m := n
	for range t.nOctaves - 1 {
		m = decimatedLength(m, lowpassTaps, 2)
		if m < 1 {
			return 0, false
		}
	}

	// The octaves shrink monotonically, so the lowest one decides.
	if t.padding == PadReflect && !t.zeroPadShort && m <= t.nFFT/2 {
		return 0, false
	}

	return n, true
}

// decimatedLength is the output length of decimate: torch conv1d with
// padding (taps-1)/2 and stride factor, or 0 if the padded input is shorter
// than the filter.
func decimatedLength(n, taps, factor int) int {
	padded := n + 2*((taps-1)/2)
	if padded < taps {
		return 0
	}

	return (padded-taps)/factor + 1
}

// FrameCount returns the number of frames the transform produces for n input
// samples, or 0 if n samples are too short to be processed (see
// [ErrSignalTooShort]). All octaves yield the same number of frames,
// (L+2*(nfft/2)-nfft)/hop + 1 = L/hop + 1 for a top-octave signal of L
// samples, so without early downsampling the count is n/Hop()+1 (integer
// division).
//
// This is torch.stft(center=True)'s convention, which nnAudio inherits. It
// differs from stft.Transform.FrameCount, which returns ceil(n/hop) for
// centred framing: one frame fewer when hop divides n, the same count
// otherwise.
func (t *Transform) FrameCount(n int) int {
	top, ok := t.topLength(n)
	if !ok {
		return 0
	}

	return (top+2*(t.nFFT/2)-t.nFFT)/t.hop + 1
}

// OutputLen returns the number of output values for n input samples:
// FrameCount(n)*Bins(), times 2 for complex output.
func (t *Transform) OutputLen(n int) int {
	l := t.FrameCount(n) * t.nBins
	if t.output == OutputComplex {
		l *= 2
	}

	return l
}

// Process returns the constant-Q transform of x in a newly allocated slice
// of OutputLen(len(x)) values. See [Transform.ProcessInto] for the layout.
func (t *Transform) Process(x []float64) ([]float64, error) {
	if t.FrameCount(len(x)) == 0 {
		return nil, t.tooShort(len(x))
	}

	dst := make([]float64, t.OutputLen(len(x)))
	process(t, dst, x)

	return dst, nil
}

// ProcessInto writes the constant-Q transform of x to dst[:OutputLen(len(x))]
// and leaves the rest of dst untouched.
//
// The layout is frame-major. With [OutputMagnitude], dst[frame*Bins()+bin]
// is the magnitude of bin bin in frame frame. With [OutputComplex],
// dst[2*(frame*Bins()+bin)] is the real part and the following element
// the imaginary part. Bins are ordered by frequency, lowest first.
//
// ProcessInto returns [ErrSignalTooShort] if FrameCount(len(x)) is 0 and
// [ErrShortDst] if dst is too short. It does not allocate once its scratch
// buffers have grown to the input length. dst may overlap x; x is then read
// from a copy, because the lower octaves are decimated from x after the top
// octave's results have been written.
//
// ProcessInto runs the same phases as a [Stepper] with an unlimited budget;
// use a Stepper to spread one transform over several bounded steps.
func (t *Transform) ProcessInto(dst, x []float64) error {
	err := t.checkLengths(len(dst), len(x))
	if err != nil {
		return err
	}

	if core.Overlaps(dst, x) {
		x = loadInput(&t.pass, x)
	}

	process(t, dst, x)

	return nil
}

// ProcessInto32 is [Transform.ProcessInto] for float32 input and output. The
// input is converted to float64 and the whole transform is computed in
// float64; only the result is rounded to float32.
func (t *Transform) ProcessInto32(dst, x []float32) error {
	err := t.checkLengths(len(dst), len(x))
	if err != nil {
		return err
	}

	process(t, dst, loadInput(&t.pass, x))

	return nil
}

func (t *Transform) tooShort(n int) error {
	reflect := ""
	if t.padding == PadReflect && !t.zeroPadShort {
		reflect = fmt.Sprintf(", reflect padding needs the lowest octave longer than %d samples", t.nFFT/2)
	}

	return fmt.Errorf("%w: %d samples for %d octaves (early downsampling factor %d%s)",
		ErrSignalTooShort, n, t.nOctaves, t.factor, reflect)
}

func (t *Transform) checkLengths(nDst, nx int) error {
	if t.FrameCount(nx) == 0 {
		return t.tooShort(nx)
	}

	if need := t.OutputLen(nx); nDst < need {
		return fmt.Errorf("%w: %d values, need %d", ErrShortDst, nDst, need)
	}

	return nil
}

// process runs the Transform's own pass over x with an unlimited budget.
// The caller has validated the lengths.
func process[T sample](t *Transform, dst []T, x []float64) {
	t.pass.start(t, x)
	advance(&t.pass, dst, math.MaxInt)
}

// Work returns the number of work units a transform of n input samples
// takes, or 0 if n samples are too short to be processed. A work unit is one
// multiply-accumulate: kernel taps times frames for each octave's kernel
// correlation, and 256 low-pass taps times output samples for each
// decimation (including early downsampling). Padding, the float32
// conversion and the magnitude are not counted. Use it to size the budgets
// passed to [Stepper.Step].
func (t *Transform) Work(n int) int {
	frames := t.FrameCount(n)
	if frames == 0 {
		return 0
	}

	work, m := 0, n
	if t.factor > 1 {
		m = decimatedLength(m, lowpassTaps, t.factor)
		work += lowpassTaps * m
	}

	for o := range t.nOctaves {
		if o > 0 {
			m = decimatedLength(m, lowpassTaps, 2)
			work += lowpassTaps * m
		}

		work += frames * t.frameTaps(o)
	}

	return work
}

// binBase returns the output bin of filter 0 in octave o (0 is the top
// octave). The stacked rows run from the lowest octave's first filter to the
// top octave's last; the lowest nOctaves*nFilters-nBins rows are dropped, so
// filters of the lowest octave may map below bin 0 and are skipped.
func (t *Transform) binBase(o int) int {
	drop := t.nOctaves*t.nFilters - t.nBins

	return (t.nOctaves-1-o)*t.nFilters - drop
}

// frameTaps returns the work of one frame in octave o: the summed length of
// the kernels that are not cropped.
func (t *Transform) frameTaps(o int) int {
	taps := 0
	for f := max(0, -t.binBase(o)); f < t.nFilters; f++ {
		taps += len(t.kernels[f].re)
	}

	return taps
}

// phase is a stage of a pass. Each phase except phasePad is divided into
// indivisible units: one output sample of a decimation, one frame of a
// kernel correlation.
type phase uint8

const (
	phaseDone      phase = iota // the pass is complete (zero value)
	phaseEarly                  // early downsampling of the input
	phasePad                    // padding of the current octave, no work units
	phaseCorrelate              // kernel correlation of the current octave
	phaseDecimate               // 2:1 decimation to the next octave
)

// pass is one resumable run of a Transform over one signal: nnAudio
// CQT2010v2.forward split into phases. Optional early downsampling comes
// first; then each octave, from the top one down, is padded and correlated
// with the kernels at its hop, and every octave but the lowest is low-passed
// and decimated by 2 for the next with half the hop. A pass owns its scratch
// buffers and only reads the Transform's immutable configuration, so
// several passes may share one Transform.
type pass struct {
	t      *Transform
	frames int
	phase  phase
	octave int       // current octave, 0 is the top one
	pos    int       // next frame or decimated sample of the phase
	hop    int       // hop of the current octave
	cur    []float64 // signal of the current octave
	next   []float64 // decimation target
	slot   int       // octave buffer the next 2:1 decimation writes to
	done   int       // work units performed
	total  int       // work units of the whole pass

	// Scratch, grown by start to the input length and reused.
	bufs   [2][]float64 // ping-pong octave signals
	padded []float64    // padded octave signal
	in64   []float64    // copy of the input in float64
}

// loadInput converts x to float64 into the pass's input buffer and returns
// it.
func loadInput[T sample](p *pass, x []T) []float64 {
	p.in64 = core.EnsureLen(p.in64, len(x))
	for i, v := range x {
		p.in64[i] = float64(v)
	}

	return p.in64
}

// start prepares the pass for x, whose length t has validated, and grows
// every scratch buffer the pass needs, so that advance does not allocate. x
// is read until the pass is done; it must not overlap the destination.
func (p *pass) start(t *Transform, x []float64) {
	p.t = t
	p.frames = t.FrameCount(len(x))
	p.total = t.Work(len(x))
	p.done = 0
	p.octave, p.pos, p.hop, p.slot = 0, 0, t.hop, 0

	// The decimated signals alternate between the two octave buffers and
	// shrink, so each buffer needs the length of the first signal written
	// to it.
	top, n := len(x), len(x)
	if t.factor > 1 {
		top = decimatedLength(n, lowpassTaps, t.factor)
	}

	var need [2]int

	written, m := 0, top
	if t.factor > 1 {
		need[0], written = top, 1
	}

	for range t.nOctaves - 1 {
		if written >= len(need) {
			break
		}

		m = decimatedLength(m, lowpassTaps, 2)
		need[written] = m
		written++
	}

	for i := range need {
		p.bufs[i] = core.EnsureLen(p.bufs[i], need[i])
	}

	p.padded = core.EnsureLen(p.padded, top+2*(t.nFFT/2))

	p.cur, p.next, p.phase = x, nil, phasePad
	if t.factor > 1 {
		p.next, p.phase = p.bufs[0][:top], phaseEarly
	}
}

// advance runs the pass until at least maxWork work units are done or the
// pass is complete, and returns the units performed. The unit in progress
// when the budget runs out is finished, so a call exceeds maxWork by less
// than one unit; padding counts no work. maxWork must be at least 1.
func advance[T sample](p *pass, dst []T, maxWork int) int {
	t := p.t
	work := 0

	for work < maxWork && p.phase != phaseDone {
		switch p.phase {
		case phaseDone:
		case phaseEarly:
			work += p.decimate(t.earlyLowpass, t.factor, maxWork-work)
			if p.pos == len(p.next) {
				p.cur, p.next, p.slot = p.next, nil, 1
				p.phase = phasePad
			}
		case phasePad:
			p.pad()
			p.pos = 0
			p.phase = phaseCorrelate
		case phaseCorrelate:
			taps := t.frameTaps(p.octave)
			k := min(p.frames-p.pos, units(maxWork-work, taps))
			correlate(t, dst, p.padded, p.hop, t.binBase(p.octave), p.pos, p.pos+k)
			p.pos += k
			work += k * taps

			if p.pos == p.frames {
				p.endOctave()
			}
		case phaseDecimate:
			work += p.decimate(t.lowpass, 2, maxWork-work)
			if p.pos == len(p.next) {
				p.cur, p.next, p.slot = p.next, nil, 1-p.slot
				p.octave++
				p.hop /= 2
				p.phase = phasePad
			}
		}
	}

	p.done += work

	return work
}

// endOctave follows the correlation of the current octave: the next phase
// decimates it for the octave below, or the pass is complete.
func (p *pass) endOctave() {
	t := p.t
	if p.octave == t.nOctaves-1 {
		// Drop the references to the caller's input.
		p.cur, p.next = nil, nil
		p.phase = phaseDone

		return
	}

	p.next = p.bufs[p.slot][:decimatedLength(len(p.cur), lowpassTaps, 2)]
	p.pos = 0
	p.phase = phaseDecimate
}

// units returns the number of units of size size that cover budget, at least
// 1: ceil(budget/size) without overflow.
func units(budget, size int) int {
	k := budget / size
	if budget%size != 0 {
		k++
	}

	return max(k, 1)
}

// decimate computes the next decimated samples of p.cur into p.next, as many
// as cover budget, and returns their work.
func (p *pass) decimate(h []float64, factor, budget int) int {
	k := min(len(p.next)-p.pos, units(budget, len(h)))
	decimateRange(p.next, p.cur, h, factor, p.pos, p.pos+k)
	p.pos += k

	return k * len(h)
}

// pad extends the current octave's signal by nfft/2 samples on both sides
// into p.padded.
func (p *pass) pad() {
	t := p.t
	x := p.cur
	pad := t.nFFT / 2
	n := len(x)
	xp := p.padded[:n+2*pad]
	p.padded = xp

	if t.padding == PadReflect && pad < n {
		// Cannot fail: pad < n, xp has n+2*pad elements and is a scratch
		// buffer distinct from x.
		_ = core.PadReflect(xp, x, pad, pad)
	} else {
		// Zero padding, or under the NNAudio preset an octave that torch's
		// reflection pad rejects (pad >= n), which nnAudio zero pads.
		// Without the preset topLength has already rejected such signals.
		clear(xp[:pad])
		copy(xp[pad:], x)
		clear(xp[pad+n:])
	}
}

// decimateRange is nnAudio's downsampling_by_n restricted to the output
// samples [from, to): torch conv1d (cross-correlation) of x with h, zero
// padding (len(h)-1)/2 on both sides and stride factor. dst must have
// decimatedLength(len(x), len(h), factor) elements and must not overlap x or
// h. Every output sample is computed from absolute positions, so splitting
// the range does not change any value.
func decimateRange(dst, x, h []float64, factor, from, to int) {
	// Cannot fail: factor >= 2, h is a non-empty filter and dst is a scratch
	// buffer distinct from x and h.
	_ = conv.CorrelateStridedInto(dst[from:to], x, h, -(len(h)-1)/2+from*factor, factor)
}

// correlate applies the kernels to the frames [from, to) of one octave's
// padded signal xp with stride hop and writes filter f to output bin
// binBase+f; filters below bin 0 are skipped.
func correlate[T sample](t *Transform, dst []T, xp []float64, hop, binBase, from, to int) {
	fFirst := max(0, -binBase)
	complexOut := t.output == OutputComplex

	for fr := from; fr < to; fr++ {
		base := fr * hop
		row := fr * t.nBins

		for f := fFirst; f < t.nFilters; f++ {
			k := &t.kernels[f]
			re, im := dot2(xp[base+k.start:], k.re, k.im)

			b := binBase + f
			s := t.scale[b]
			re *= s
			im *= -s // torch: CQT_imag = -conv1d(x, kernel_imag)

			if complexOut {
				dst[2*(row+b)] = T(re)
				dst[2*(row+b)+1] = T(im)
			} else {
				dst[row+b] = T(math.Sqrt(re*re + im*im))
			}
		}
	}
}

// dot2 returns the dot products of x[:len(a)] with a and with b in one pass.
//
// The kernel correlation deliberately does not use conv.CorrelateStridedInto,
// unlike decimate. On the basic-pitch benchmark (BenchmarkProcessIntoBasicPitch,
// Apple M5 Pro, 46.2) computing each filter's real and imaginary rows with two
// CorrelateStridedInto calls and scattering them into the frame-major output
// was 8-14% slower than this fused loop, and two vecmath.DotProduct calls per
// window (which read x twice) were 10% slower. Both exceed the 5% budget, so
// the fused real+imaginary loop stays.
func dot2(x, a, b []float64) (float64, float64) {
	n := len(a)
	x = x[:n]
	b = b[:n]

	var r0, r1, r2, r3, i0, i1, i2, i3 float64

	i := 0
	for ; i+4 <= n; i += 4 {
		x0, x1, x2, x3 := x[i], x[i+1], x[i+2], x[i+3]
		r0 += x0 * a[i]
		r1 += x1 * a[i+1]
		r2 += x2 * a[i+2]
		r3 += x3 * a[i+3]
		i0 += x0 * b[i]
		i1 += x1 * b[i+1]
		i2 += x2 * b[i+2]
		i3 += x3 * b[i+3]
	}

	for ; i < n; i++ {
		r0 += x[i] * a[i]
		i0 += x[i] * b[i]
	}

	return (r0 + r1) + (r2 + r3), (i0 + i1) + (i2 + i3)
}
