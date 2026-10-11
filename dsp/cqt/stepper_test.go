package cqt

import (
	"errors"
	"math"
	"slices"
	"sync"
	"testing"
)

// stepBudgets are the maxWork values every configuration is stepped with:
// one unit per step, a prime that never lines up with a unit boundary, the
// budget the basic-pitch benchmarks use, and a single unlimited step.
var stepBudgets = []int{1, 997, 50_000, math.MaxInt}

// bitsEqual reports whether a and b have the same length and the same bit
// pattern element by element, so that -0 differs from +0 and NaN equals an
// identical NaN.
func bitsEqual[T float32 | float64](a, b []T) bool {
	return slices.EqualFunc(a, b, func(x, y T) bool {
		switch v := any(x).(type) {
		case float32:
			return math.Float32bits(v) == math.Float32bits(any(y).(float32))
		default:
			return math.Float64bits(any(x).(float64)) == math.Float64bits(any(y).(float64))
		}
	})
}

// toFloat32 converts x to float32; testSignal is float32-rounded, so the
// conversion is exact.
func toFloat32(x []float64) []float32 {
	x32 := make([]float32, len(x))
	for i, v := range x {
		x32[i] = float32(v)
	}

	return x32
}

// maxUnit returns the largest indivisible unit of tr's passes: one frame of
// the costliest octave or one decimated sample.
func maxUnit(tr *Transform) int {
	unit := lowpassTaps
	for o := range tr.nOctaves {
		unit = max(unit, tr.frameTaps(o))
	}

	return unit
}

// stepStats summarizes one stepped run.
type stepStats struct {
	steps     int
	work      int // summed work of all steps
	overshoot int // largest work-maxWork of a non-final step
}

// checkSteps steps s with maxWork until it is done and checks every step:
// a non-final step does at least maxWork and less than maxWork+maxUnit(tr)
// units, the final one at least 1; Progress starts at 0, increases strictly,
// stays below 1 until done and is exactly 1 at done.
func checkSteps[T float32 | float64](tb testing.TB, tr *Transform, s *Stepper[T], maxWork int) stepStats {
	tb.Helper()

	unit := maxUnit(tr)
	total := s.p.total

	if total < 1 {
		tb.Fatalf("pass has %d work units", total)
	}

	if p := s.Progress(); p != 0 {
		tb.Fatalf("Progress before the first step = %v, want 0", p)
	}

	var st stepStats

	prev := 0.0

	for {
		before := s.p.done

		done, err := s.Step(maxWork)
		if err != nil {
			tb.Fatalf("step %d: %v", st.steps, err)
		}

		work := s.p.done - before
		st.steps++
		st.work += work

		if work < 1 {
			tb.Fatalf("step %d did %d work units", st.steps, work)
		}

		p := s.Progress()

		if done {
			if p != 1 {
				tb.Fatalf("Progress at done = %v, want 1", p)
			}

			if s.p.done != total {
				tb.Fatalf("done after %d of %d work units", s.p.done, total)
			}

			return st
		}

		// work-maxWork instead of work against maxWork+unit, which would
		// overflow for math.MaxInt (that budget never takes a non-final step).
		if work < maxWork || work-maxWork >= unit {
			tb.Fatalf("step %d did %d work units for maxWork %d (largest unit %d)",
				st.steps, work, maxWork, unit)
		}

		st.overshoot = max(st.overshoot, work-maxWork)

		if !(p > prev) || !(p < 1) {
			tb.Fatalf("step %d: Progress %v after %v, want strictly increasing and below 1", st.steps, p, prev)
		}

		prev = p

		if st.steps > total {
			tb.Fatalf("not done after %d steps for %d work units", st.steps, total)
		}
	}
}

// TestStepperBitIdentical steps every oracle configuration (the eight 46.1
// reference configurations incl. BasicPitch(), NNAudio(), the short-signal
// zero-pad quirk and early downsampling, plus generic ones with a partial
// octave and PadReflect) with each budget of stepBudgets, in float64 and in
// float32, and requires the output to be bit-identical to ProcessInto and
// ProcessInto32. checkSteps checks the per-step bounds and Progress on every
// run, and the summed work must equal Work(n). The destination is prefilled
// with NaN, so a value the steps never write cannot pass, and has a tail past
// OutputLen that must stay untouched.
func TestStepperBitIdentical(t *testing.T) {
	t.Parallel()

	for _, tc := range oracleCases() {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tr, err := New(tc.sr, tc.opts...)
			if err != nil {
				t.Fatal(err)
			}

			x := testSignal(t)[:tc.length]
			x32 := toFloat32(x)
			need, work := tr.OutputLen(len(x)), tr.Work(len(x))

			want := make([]float64, need)
			if err := tr.ProcessInto(want, x); err != nil {
				t.Fatal(err)
			}

			want32 := make([]float32, need)
			if err := tr.ProcessInto32(want32, x32); err != nil {
				t.Fatal(err)
			}

			const tail = 3

			for _, maxWork := range stepBudgets {
				dst := make([]float64, need+tail)
				for i := range dst {
					dst[i] = math.NaN()
				}

				s, err := tr.NewStepper(dst, x)
				if err != nil {
					t.Fatal(err)
				}

				st := checkSteps(t, tr, s, maxWork)
				if st.work != work {
					t.Errorf("maxWork %d: steps did %d work units, Work(%d) = %d", maxWork, st.work, len(x), work)
				}

				if !bitsEqual(dst[:need], want) {
					t.Errorf("maxWork %d: float64 output differs from ProcessInto", maxWork)
				}

				for i, v := range dst[need:] {
					if !math.IsNaN(v) {
						t.Errorf("maxWork %d: dst[OutputLen+%d] = %v, want untouched", maxWork, i, v)
					}
				}

				dst32 := make([]float32, need+tail)
				for i := range dst32 {
					dst32[i] = float32(math.NaN())
				}

				s32, err := tr.NewStepper32(dst32, x32)
				if err != nil {
					t.Fatal(err)
				}

				st32 := checkSteps(t, tr, s32, maxWork)
				if st32.work != work {
					t.Errorf("maxWork %d: float32 steps did %d work units, Work = %d", maxWork, st32.work, work)
				}

				if !bitsEqual(dst32[:need], want32) {
					t.Errorf("maxWork %d: float32 output differs from ProcessInto32", maxWork)
				}

				for i, v := range dst32[need:] {
					if !math.IsNaN(float64(v)) {
						t.Errorf("maxWork %d: dst32[OutputLen+%d] = %v, want untouched", maxWork, i, v)
					}
				}

				if maxWork != math.MaxInt {
					t.Logf("maxWork %d: %d steps, max overshoot %d (largest unit %d, Work %d)",
						maxWork, st.steps, st.overshoot, maxUnit(tr), work)
				}
			}
		})
	}
}

// TestStepperUnitLimit checks the spec's bound on the indivisible units: one
// frame of an octave costs at most one frame of the whole kernel set (the
// lowest octave may crop filters), and a decimated sample costs the 256
// low-pass taps, far below 1024 decimated samples.
func TestStepperUnitLimit(t *testing.T) {
	t.Parallel()

	for _, tc := range oracleCases() {
		tr, err := New(tc.sr, tc.opts...)
		if err != nil {
			t.Fatal(err)
		}

		kernelSet := 0
		for _, k := range tr.kernels {
			kernelSet += len(k.re)
		}

		for o := range tr.nOctaves {
			if taps := tr.frameTaps(o); taps < 1 || taps > kernelSet {
				t.Errorf("%s: octave %d frame costs %d units, kernel set %d", tc.name, o, taps, kernelSet)
			}
		}

		if len(tr.lowpass) != lowpassTaps || (tr.earlyLowpass != nil && len(tr.earlyLowpass) != lowpassTaps) {
			t.Errorf("%s: low-pass lengths %d, %d, want %d", tc.name, len(tr.lowpass), len(tr.earlyLowpass), lowpassTaps)
		}

		if unit := maxUnit(tr); unit > max(kernelSet, 1024*lowpassTaps) {
			t.Errorf("%s: largest unit %d exceeds the spec's limit", tc.name, unit)
		}
	}
}

// TestStepperWork recounts Work(n) independently of the pass: the octave
// lengths and the frame count come from the oracle's own length arithmetic,
// the kernel supports from the Kernels() rows, and the lowest octave drops
// its surplus filters below bin 0. Every decimation (including early
// downsampling) costs 256 units per output sample. Work(n) is 0 exactly when
// FrameCount(n) is 0.
func TestStepperWork(t *testing.T) {
	t.Parallel()

	lengths := []int{
		0, 1, 2, 3, 127, 128, 129, 255, 256, 257, 511, 512, 513, 1000, 1031, 1032,
		4000, 8249, 8250, 10000, 20000, 33023, 33024, 43844, 44100,
	}

	for _, tc := range oracleCases() {
		tr, err := New(tc.sr, tc.opts...)
		if err != nil {
			t.Fatal(err)
		}

		spec := oracleSpecFor(t, tc.opts, tc.zeroPadShort)
		rows := tr.Kernels()
		filters, octaves := len(rows), oracleOctaves(tr)

		// The rows are zero outside the stored kernel, so the stored length
		// is the full support.
		taps := make([]int, filters)

		for f, row := range rows {
			k := tr.kernels[f]
			for i, v := range row {
				if v != 0 && (i < k.start || i >= k.start+len(k.re)) {
					t.Fatalf("%s: kernel %d is non-zero at %d outside [%d, %d)",
						tc.name, f, i, k.start, k.start+len(k.re))
				}
			}

			taps[f] = len(k.re)
		}

		drop := octaves*filters - tr.Bins()

		for _, n := range append(lengths, tc.length) {
			lens, frames, ok := oracleOctaveLens(t, tr, spec, n)

			got := tr.Work(n)
			if (got == 0) != (tr.FrameCount(n) == 0) {
				t.Errorf("%s: Work(%d) = %d, FrameCount %d", tc.name, n, got, tr.FrameCount(n))
			}

			if !ok {
				if got != 0 {
					t.Errorf("%s: Work(%d) = %d for an unprocessable length, want 0", tc.name, n, got)
				}

				continue
			}

			want := 0
			if tr.DownsampleFactor() > 1 {
				want += oracleTaps * lens[0]
			}

			for o := range octaves {
				if o > 0 {
					want += oracleTaps * lens[o]
				}

				first := 0
				if o == octaves-1 {
					first = drop
				}

				for f := first; f < filters; f++ {
					want += frames * taps[f]
				}
			}

			if got != want {
				t.Errorf("%s: Work(%d) = %d, recount %d", tc.name, n, got, want)
			}
		}
	}
}

// TestStepperOverlap places dst over x: the Stepper copies x on NewStepper,
// so writing the output cannot disturb the input it still has to read.
func TestStepperOverlap(t *testing.T) {
	t.Parallel()

	tr := newBasicPitch(t)
	x := sine(8192, 440, 22050)
	need := tr.OutputLen(len(x))

	want, err := tr.Process(x)
	if err != nil {
		t.Fatal(err)
	}

	for _, offset := range []int{0, 1, need / 2, need - 1} {
		buf := make([]float64, offset+max(need, len(x))+need)
		in := buf[offset : offset+len(x)]

		for _, span := range [][2]int{{0, need}, {offset + len(x) - 1, offset + len(x) - 1 + need}} {
			copy(in, x)

			ref := make([]float64, len(buf))
			copy(ref, buf)

			if err := tr.ProcessInto(ref[span[0]:span[1]], ref[offset:offset+len(x)]); err != nil {
				t.Fatal(err)
			}

			s, err := tr.NewStepper(buf[span[0]:span[1]], in)
			if err != nil {
				t.Fatal(err)
			}

			checkSteps(t, tr, s, 997)

			dst := buf[span[0]:span[1]]
			if !bitsEqual(dst, want) || !bitsEqual(dst, ref[span[0]:span[1]]) {
				t.Errorf("offset %d, dst [%d, %d): stepped output differs from ProcessInto", offset, span[0], span[1])
			}
		}
	}
}

// TestStepperReset reuses one Stepper: for a second signal, mid-run, after
// a rejected Reset, and for a longer input.
func TestStepperReset(t *testing.T) {
	t.Parallel()

	tr := newBasicPitch(t)
	x := testSignal(t)
	need := tr.OutputLen(len(x))

	// A second signal of the same length: reversed, negated and halved
	// (exact in float32 too).
	y := make([]float64, len(x))
	for i, v := range x {
		y[len(x)-1-i] = -0.5 * v
	}

	wantX, err := tr.Process(x)
	if err != nil {
		t.Fatal(err)
	}

	wantY, err := tr.Process(y)
	if err != nil {
		t.Fatal(err)
	}

	if bitsEqual(wantX, wantY) {
		t.Fatal("the two test signals give the same output")
	}

	// Every reference from tr's own ProcessInto pass is computed here: the
	// parallel subtests below share tr, and a Transform is not safe for
	// concurrent use (only its Steppers are).
	x32, y32 := toFloat32(x), toFloat32(y)
	want32X, want32Y := make([]float32, need), make([]float32, need)

	if err := tr.ProcessInto32(want32X, x32); err != nil {
		t.Fatal(err)
	}

	if err := tr.ProcessInto32(want32Y, y32); err != nil {
		t.Fatal(err)
	}

	short := x[:20000]

	wantShort, err := tr.Process(short)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("two signals", func(t *testing.T) {
		t.Parallel()

		dst := make([]float64, need)

		s, err := tr.NewStepper(dst, x)
		if err != nil {
			t.Fatal(err)
		}

		checkSteps(t, tr, s, 50_000)

		if !bitsEqual(dst, wantX) {
			t.Error("first signal: output differs from ProcessInto")
		}

		if err := s.Reset(dst, y); err != nil {
			t.Fatal(err)
		}

		checkSteps(t, tr, s, 997)

		if !bitsEqual(dst, wantY) {
			t.Error("second signal: output differs from ProcessInto")
		}

		// float32 likewise.
		dst32 := make([]float32, need)

		s32, err := tr.NewStepper32(dst32, x32)
		if err != nil {
			t.Fatal(err)
		}

		checkSteps(t, tr, s32, 997)

		if !bitsEqual(dst32, want32X) {
			t.Error("float32, first signal: output differs from ProcessInto32")
		}

		if err := s32.Reset(dst32, y32); err != nil {
			t.Fatal(err)
		}

		checkSteps(t, tr, s32, 50_000)

		if !bitsEqual(dst32, want32Y) {
			t.Error("float32, second signal: output differs from ProcessInto32")
		}
	})

	t.Run("mid-run", func(t *testing.T) {
		t.Parallel()

		dst := make([]float64, need)

		s, err := tr.NewStepper(dst, x)
		if err != nil {
			t.Fatal(err)
		}

		// Stop the first pass in the middle of the second octave's
		// correlation, past the top octave's correlation and its decimation.
		for s.p.octave < 1 || s.p.phase != phaseCorrelate || s.p.pos == 0 {
			done, err := s.Step(997)
			if err != nil || done {
				t.Fatalf("Step: done %v, %v", done, err)
			}
		}

		if err := s.Reset(dst, y); err != nil {
			t.Fatal(err)
		}

		// The unfinished pass is discarded: the new one starts from zero.
		checkSteps(t, tr, s, 997)

		if !bitsEqual(dst, wantY) {
			t.Error("output after a mid-run Reset differs from ProcessInto")
		}
	})

	t.Run("rejected", func(t *testing.T) {
		t.Parallel()

		dst := make([]float64, need)

		s, err := tr.NewStepper(dst, x)
		if err != nil {
			t.Fatal(err)
		}

		for range 5 {
			if _, err := s.Step(997); err != nil {
				t.Fatal(err)
			}
		}

		progress, done, phase, pos := s.Progress(), s.p.done, s.p.phase, s.p.pos
		other := make([]float64, need)

		cases := []struct {
			name string
			dst  []float64
			x    []float64
			want error
		}{
			{"too short", other, x[:100], ErrSignalTooShort},
			{"empty", other, nil, ErrSignalTooShort},
			{"short dst", other[:need-1], y, ErrShortDst},
		}

		for _, c := range cases {
			if err := s.Reset(c.dst, c.x); !errors.Is(err, c.want) {
				t.Errorf("%s: Reset error %v, want %v", c.name, err, c.want)
			}

			if s.Progress() != progress || s.p.done != done || s.p.phase != phase || s.p.pos != pos ||
				&s.dst[0] != &dst[0] {
				t.Fatalf("%s: rejected Reset changed the Stepper", c.name)
			}
		}

		// The original pass continues to the correct result, and nothing was
		// written to the rejected destination.
		for {
			finished, err := s.Step(50_000)
			if err != nil {
				t.Fatal(err)
			}

			if finished {
				break
			}
		}

		if !bitsEqual(dst, wantX) {
			t.Error("output after rejected Resets differs from ProcessInto")
		}

		if slices.ContainsFunc(other, func(v float64) bool { return v != 0 }) {
			t.Error("a rejected Reset's destination was written")
		}
	})

	t.Run("longer input", func(t *testing.T) {
		t.Parallel()

		dst := make([]float64, need)

		s, err := tr.NewStepper(dst, short)
		if err != nil {
			t.Fatal(err)
		}

		checkSteps(t, tr, s, 50_000)

		if !bitsEqual(dst[:len(wantShort)], wantShort) {
			t.Error("short input: output differs from ProcessInto")
		}

		// Reset grows the scratch buffers for the longer input.
		if err := s.Reset(dst, y); err != nil {
			t.Fatal(err)
		}

		checkSteps(t, tr, s, 50_000)

		if !bitsEqual(dst, wantY) {
			t.Error("longer input: output differs from ProcessInto")
		}
	})
}

// TestStepperZeroAllocs checks that Step and Reset do not allocate once
// NewStepper has grown the scratch buffers, in float64 and float32.
func TestStepperZeroAllocs(t *testing.T) {
	tr := newBasicPitch(t)
	x := testSignal(t)
	need := tr.OutputLen(len(x))

	dst := make([]float64, need)

	s, err := tr.NewStepper(dst, x)
	if err != nil {
		t.Fatal(err)
	}

	if a := testing.AllocsPerRun(10, func() { _, _ = s.Step(997) }); a != 0 {
		t.Errorf("Step: %v allocs/op, want 0", a)
	}

	if a := testing.AllocsPerRun(3, func() { _ = s.Reset(dst, x) }); a != 0 {
		t.Errorf("Reset: %v allocs/op, want 0", a)
	}

	// A whole pass of steps, and a Reset with a shorter input.
	full := func() {
		_ = s.Reset(dst, x)

		for {
			done, err := s.Step(50_000)
			if done || err != nil {
				return
			}
		}
	}

	if a := testing.AllocsPerRun(2, full); a != 0 {
		t.Errorf("Reset and a full pass of Steps: %v allocs/op, want 0", a)
	}

	if a := testing.AllocsPerRun(3, func() { _ = s.Reset(dst, x[:20000]) }); a != 0 {
		t.Errorf("Reset (shorter input): %v allocs/op, want 0", a)
	}

	x32 := toFloat32(x)
	dst32 := make([]float32, need)

	s32, err := tr.NewStepper32(dst32, x32)
	if err != nil {
		t.Fatal(err)
	}

	if a := testing.AllocsPerRun(10, func() { _, _ = s32.Step(997) }); a != 0 {
		t.Errorf("Step (float32): %v allocs/op, want 0", a)
	}

	if a := testing.AllocsPerRun(3, func() { _ = s32.Reset(dst32, x32) }); a != 0 {
		t.Errorf("Reset (float32): %v allocs/op, want 0", a)
	}

	full32 := func() {
		_ = s32.Reset(dst32, x32)

		for {
			done, err := s32.Step(50_000)
			if done || err != nil {
				return
			}
		}
	}

	if a := testing.AllocsPerRun(2, full32); a != 0 {
		t.Errorf("Reset and a full pass of Steps (float32): %v allocs/op, want 0", a)
	}
}

// TestStepperErrors checks the length validation of NewStepper and
// NewStepper32, the budget check of Step and Step after done.
func TestStepperErrors(t *testing.T) {
	t.Parallel()

	tr := newBasicPitch(t)
	x := sine(8192, 440, 22050)
	need := tr.OutputLen(len(x))

	s, err := tr.NewStepper(make([]float64, need), x[:100])
	if !errors.Is(err, ErrSignalTooShort) || s != nil {
		t.Errorf("NewStepper, short x: %v, %v; want nil, ErrSignalTooShort", s, err)
	}

	s, err = tr.NewStepper(make([]float64, need), nil)
	if !errors.Is(err, ErrSignalTooShort) || s != nil {
		t.Errorf("NewStepper, empty x: %v, %v; want nil, ErrSignalTooShort", s, err)
	}

	s, err = tr.NewStepper(make([]float64, need-1), x)
	if !errors.Is(err, ErrShortDst) || s != nil {
		t.Errorf("NewStepper, short dst: %v, %v; want nil, ErrShortDst", s, err)
	}

	x32 := toFloat32(x)

	s32, err := tr.NewStepper32(make([]float32, need), x32[:100])
	if !errors.Is(err, ErrSignalTooShort) || s32 != nil {
		t.Errorf("NewStepper32, short x: %v, %v; want nil, ErrSignalTooShort", s32, err)
	}

	s32, err = tr.NewStepper32(make([]float32, need-1), x32)
	if !errors.Is(err, ErrShortDst) || s32 != nil {
		t.Errorf("NewStepper32, short dst: %v, %v; want nil, ErrShortDst", s32, err)
	}

	dst := make([]float64, need)

	s, err = tr.NewStepper(dst, x)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := s.Step(997); err != nil {
		t.Fatal(err)
	}

	// A rejected budget changes nothing.
	progress, done, phase, pos := s.Progress(), s.p.done, s.p.phase, s.p.pos

	for _, maxWork := range []int{0, -1, math.MinInt} {
		finished, err := s.Step(maxWork)
		if !errors.Is(err, ErrMaxWork) || finished {
			t.Errorf("Step(%d) = %v, %v; want false, ErrMaxWork", maxWork, finished, err)
		}

		if s.Progress() != progress || s.p.done != done || s.p.phase != phase || s.p.pos != pos {
			t.Errorf("Step(%d) changed the Stepper", maxWork)
		}
	}

	if finished, err := s.Step(math.MaxInt); err != nil || !finished {
		t.Fatalf("Step(MaxInt) = %v, %v; want true, nil", finished, err)
	}

	out := slices.Clone(dst)

	for _, maxWork := range []int{1, math.MaxInt} {
		finished, err := s.Step(maxWork)
		if !errors.Is(err, ErrStepperDone) || !finished {
			t.Errorf("Step(%d) after done = %v, %v; want true, ErrStepperDone", maxWork, finished, err)
		}
	}

	if p := s.Progress(); p != 1 {
		t.Errorf("Progress after done = %v, want 1", p)
	}

	if !bitsEqual(dst, out) {
		t.Error("Step after done changed the output")
	}

	// A budget below 1 is reported before the stepper being done.
	if _, err := s.Step(0); !errors.Is(err, ErrMaxWork) {
		t.Errorf("Step(0) after done: %v, want ErrMaxWork", err)
	}
}

// TestStepperZero checks the zero Stepper: it is done and has no Transform
// to Reset with.
func TestStepperZero(t *testing.T) {
	t.Parallel()

	var s Stepper[float64]

	if p := s.Progress(); p != 1 {
		t.Errorf("Progress = %v, want 1", p)
	}

	if done, err := s.Step(1); !errors.Is(err, ErrStepperDone) || !done {
		t.Errorf("Step = %v, %v; want true, ErrStepperDone", done, err)
	}

	if _, err := s.Step(0); !errors.Is(err, ErrMaxWork) {
		t.Errorf("Step(0) = %v, want ErrMaxWork", err)
	}

	if err := s.Reset(make([]float64, 1000), make([]float64, 1000)); !errors.Is(err, ErrStepperDone) {
		t.Errorf("Reset = %v, want an error wrapping ErrStepperDone", err)
	}

	var s32 Stepper[float32]

	if done, err := s32.Step(1); !errors.Is(err, ErrStepperDone) || !done || s32.Progress() != 1 {
		t.Errorf("float32: Step = %v, %v, Progress %v; want true, ErrStepperDone, 1", done, err, s32.Progress())
	}
}

// TestStepperIndependent runs ProcessInto and ProcessInto32 on the Stepper's
// Transform, with other signals and lengths, between the steps: they use the
// Transform's own pass, so the Stepper's output is unaffected.
func TestStepperIndependent(t *testing.T) {
	t.Parallel()

	tr := newBasicPitch(t)
	x := testSignal(t)
	need := tr.OutputLen(len(x))

	want, err := tr.Process(x)
	if err != nil {
		t.Fatal(err)
	}

	other := sine(20000, 440, 22050)
	otherDst := make([]float64, tr.OutputLen(len(other)))
	other32 := toFloat32(other)
	otherDst32 := make([]float32, len(otherDst))

	dst := make([]float64, need)

	s, err := tr.NewStepper(dst, x)
	if err != nil {
		t.Fatal(err)
	}

	// About a dozen steps, each followed by a full ProcessInto: under -race
	// a basic-pitch ProcessInto takes over 50 ms.
	for step := 0; ; step++ {
		done, err := s.Step(1_000_000)
		if err != nil {
			t.Fatal(err)
		}

		if done {
			break
		}

		if step%2 == 0 {
			err = tr.ProcessInto(otherDst, other)
		} else {
			err = tr.ProcessInto32(otherDst32, other32)
		}

		if err != nil {
			t.Fatal(err)
		}
	}

	if !bitsEqual(dst, want) {
		t.Error("output differs from ProcessInto after interleaved ProcessInto calls")
	}
}

// TestStepperConcurrent runs two Steppers of one Transform on different
// goroutines with different budgets; under -race this checks that they share
// only the Transform's immutable configuration.
func TestStepperConcurrent(t *testing.T) {
	t.Parallel()

	tr := newBasicPitch(t)
	x := testSignal(t)

	want, err := tr.Process(x)
	if err != nil {
		t.Fatal(err)
	}

	budgets := []int{997, 50_000}
	results := make([][]float64, len(budgets))
	errs := make([]error, len(budgets))
	steppers := make([]*Stepper[float64], len(budgets))

	for w := range budgets {
		results[w] = make([]float64, len(want))

		steppers[w], err = tr.NewStepper(results[w], x)
		if err != nil {
			t.Fatal(err)
		}
	}

	var wg sync.WaitGroup

	for w, s := range steppers {
		wg.Go(func() {
			for {
				done, err := s.Step(budgets[w])
				if done || err != nil {
					errs[w] = err

					return
				}
			}
		})
	}

	wg.Wait()

	for w := range budgets {
		if errs[w] != nil {
			t.Fatalf("stepper %d: %v", w, errs[w])
		}

		if !bitsEqual(results[w], want) {
			t.Errorf("stepper %d (maxWork %d): output differs from ProcessInto", w, budgets[w])
		}
	}
}
