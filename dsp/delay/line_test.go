package delay

import (
	"math"
	"testing"

	"github.com/cwbudde/algo-dsp/dsp/interp"
)

func approxEqual(a, b, eps float64) bool {
	return math.Abs(a-b) < eps
}

// --- construction and validation ---

func TestNewValidation(t *testing.T) {
	if _, err := New(0); err == nil {
		t.Fatal("expected error for size=0")
	}

	if _, err := New(-1); err == nil {
		t.Fatal("expected error for size=-1")
	}
}

func TestNewDefaults(t *testing.T) {
	delayLine, err := New(16)
	if err != nil {
		t.Fatal(err)
	}

	if delayLine.Len() != 16 {
		t.Fatalf("Len: got %d want 16", delayLine.Len())
	}

	if delayLine.mode != interp.Hermite {
		t.Fatalf("default mode: got %v want Hermite", delayLine.mode)
	}
}

func TestNewWithOptions(t *testing.T) {
	delayLine, err := New(16, WithMode(interp.Linear))
	if err != nil {
		t.Fatal(err)
	}

	if delayLine.mode != interp.Linear {
		t.Fatalf("mode: got %v want Linear", delayLine.mode)
	}
}

// --- integer Read/Write ---

func TestReadWrite(t *testing.T) {
	delayLine, err := New(8)
	if err != nil {
		t.Fatal(err)
	}

	for i := range 8 {
		delayLine.Write(float64(i))
	}
	// delay=1 => most recently written (7)
	if got := delayLine.Read(1); got != 7 {
		t.Fatalf("got %v want 7", got)
	}
	// delay=3 => 3 samples back from write head
	if got := delayLine.Read(3); got != 5 {
		t.Fatalf("got %v want 5", got)
	}
}

func TestReadWraparound(t *testing.T) {
	delayLine, err := New(4)
	if err != nil {
		t.Fatal(err)
	}

	for i := range 10 {
		delayLine.Write(float64(i))
	}
	// buffer should contain [8, 9, 6, 7], writePos=2
	// Read(1) = most recent = 9
	if got := delayLine.Read(1); got != 9 {
		t.Fatalf("got %v want 9", got)
	}
}

func TestReset(t *testing.T) {
	delayLine, err := New(4)
	if err != nil {
		t.Fatal(err)
	}

	delayLine.Write(1)
	delayLine.Write(2)
	delayLine.Reset()

	for i := range 4 {
		if got := delayLine.Read(i); got != 0 {
			t.Fatalf("after reset Read(%d): got %v want 0", i, got)
		}
	}
}

// --- fractional read with default (Hermite) ---

func TestReadFractionalLinearRamp(t *testing.T) {
	delayLine, err := New(16)
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < delayLine.Len(); i++ {
		delayLine.Write(float64(i))
	}

	if got := delayLine.ReadFractional(3.5); got < 12.49 || got > 12.51 {
		t.Fatalf("got %v want about 12.5", got)
	}
}

func TestReadFractionalNegativeClamped(t *testing.T) {
	delayLine, err := New(8)
	if err != nil {
		t.Fatal(err)
	}

	for i := range 8 {
		delayLine.Write(float64(i + 1))
	}

	got := delayLine.ReadFractional(-1.0)
	// negative delay clamped to 0
	if math.IsNaN(got) || math.IsInf(got, 0) {
		t.Fatalf("negative delay produced %v", got)
	}
}

// --- all modes on a linear ramp (DC-preserving and approximate linearity) ---

// fillRamp fills a delay line with a linear ramp [0, 1, 2, ..., size-1].
func fillRamp(d *Line) {
	for i := 0; i < d.Len(); i++ {
		d.Write(float64(i))
	}
}

func TestReadFractionalLinear(t *testing.T) {
	delayLine, err := New(32, WithMode(interp.Linear))
	if err != nil {
		t.Fatal(err)
	}

	fillRamp(delayLine)
	// With a linear ramp, linear interpolation is exact.
	got := delayLine.ReadFractional(5.5)

	want := float64(delayLine.Len()) - 5.5 // 26.5
	if !approxEqual(got, want, 1e-10) {
		t.Fatalf("Linear: got %v want %v", got, want)
	}
}

func TestReadFractionalHermite(t *testing.T) {
	delayLine, err := New(32, WithMode(interp.Hermite))
	if err != nil {
		t.Fatal(err)
	}

	fillRamp(delayLine)
	got := delayLine.ReadFractional(5.5)

	want := float64(delayLine.Len()) - 5.5
	if !approxEqual(got, want, 1e-10) {
		t.Fatalf("Hermite: got %v want %v", got, want)
	}
}

func TestReadFractionalLagrange(t *testing.T) {
	delayLine, err := New(32, WithMode(interp.Lagrange3))
	if err != nil {
		t.Fatal(err)
	}

	fillRamp(delayLine)
	got := delayLine.ReadFractional(5.5)

	want := float64(delayLine.Len()) - 5.5
	if !approxEqual(got, want, 1e-10) {
		t.Fatalf("Lagrange3: got %v want %v", got, want)
	}
}

func TestReadFractionalLanczos(t *testing.T) {
	delayLine, err := New(64, WithMode(interp.Lanczos3))
	if err != nil {
		t.Fatal(err)
	}

	fillRamp(delayLine)
	got := delayLine.ReadFractional(10.5)
	want := float64(delayLine.Len()) - 10.5
	// Lanczos is approximate on a finite ramp.
	if !approxEqual(got, want, 0.5) {
		t.Fatalf("Lanczos3: got %v want ~%v", got, want)
	}
}

func TestReadFractionalSinc(t *testing.T) {
	delayLine, err := New(64, WithMode(interp.Sinc), WithSincN(4))
	if err != nil {
		t.Fatal(err)
	}

	fillRamp(delayLine)
	got := delayLine.ReadFractional(10.5)

	want := float64(delayLine.Len()) - 10.5
	if !approxEqual(got, want, 0.5) {
		t.Fatalf("Sinc: got %v want ~%v", got, want)
	}
}

func TestReadFractionalAllpass(t *testing.T) {
	delayLine, err := New(64, WithMode(interp.Allpass))
	if err != nil {
		t.Fatal(err)
	}
	// Fill with DC to let the allpass state settle.
	for i := 0; i < delayLine.Len(); i++ {
		delayLine.Write(50.0)
	}

	for range 50 {
		delayLine.ReadFractional(10.5)
	}
	// Now fill with ramp and verify the output is finite and in range.
	delayLine.Reset()
	fillRamp(delayLine)

	got := delayLine.ReadFractional(10.5)
	if math.IsNaN(got) || math.IsInf(got, 0) {
		t.Fatalf("Allpass: produced %v", got)
	}
	// Allpass state is reset, so first-call accuracy is limited;
	// just check it's in the right ballpark.
	want := float64(delayLine.Len()) - 10.5
	if math.Abs(got-want) > 30 {
		t.Fatalf("Allpass: got %v, expected roughly %v", got, want)
	}
}

// --- DC preservation across all modes ---

func TestAllModesDCPreservation(t *testing.T) {
	modes := []struct {
		name string
		mode interp.Mode
	}{
		{"Linear", interp.Linear},
		{"Hermite", interp.Hermite},
		{"Lagrange3", interp.Lagrange3},
		{"Lanczos3", interp.Lanczos3},
		{"Sinc", interp.Sinc},
	}

	for _, testCase := range modes {
		delayLine, err := New(32, WithMode(testCase.mode))
		if err != nil {
			t.Fatal(err)
		}
		// Fill with constant value.
		for i := 0; i < delayLine.Len(); i++ {
			delayLine.Write(42.0)
		}

		got := delayLine.ReadFractional(5.3)
		if !approxEqual(got, 42.0, 1e-6) {
			t.Fatalf("%s DC: got %v want 42", testCase.name, got)
		}
	}
}

// --- allpass DC convergence (needs state settling) ---

func TestAllpassDCConvergence(t *testing.T) {
	delayLine, err := New(32, WithMode(interp.Allpass))
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < delayLine.Len(); i++ {
		delayLine.Write(10.0)
	}
	// Read multiple times at the same delay to let the allpass state settle.
	var got float64
	for range 100 {
		got = delayLine.ReadFractional(5.3)
	}

	if !approxEqual(got, 10.0, 1e-4) {
		t.Fatalf("Allpass DC: got %v want 10", got)
	}
}

// --- sine wave quality test across modes ---

func TestAllModesSineQuality(t *testing.T) {
	// Write a low-frequency sine into a large buffer and verify
	// that fractional reads are close to the analytic value.
	freq := 0.02 // low frequency relative to sample rate
	size := 256

	modes := []struct {
		name string
		mode interp.Mode
		tol  float64
	}{
		{"Linear", interp.Linear, 0.01},
		{"Hermite", interp.Hermite, 1e-4},
		{"Lagrange3", interp.Lagrange3, 1e-4},
		{"Lanczos3", interp.Lanczos3, 0.01},
		{"Sinc", interp.Sinc, 1e-3},
	}

	for _, testCase := range modes {
		delayLine, err := New(size, WithMode(testCase.mode))
		if err != nil {
			t.Fatal(err)
		}

		for i := range size {
			delayLine.Write(math.Sin(2 * math.Pi * freq * float64(i)))
		}

		delay := 20.37
		// Read(k) for integer k returns sample written at index (size-k),
		// so fractional delay d corresponds to sample index (size-d).
		exactSample := float64(size) - delay
		want := math.Sin(2 * math.Pi * freq * exactSample)
		got := delayLine.ReadFractional(delay)

		err2 := math.Abs(got - want)
		if err2 > testCase.tol {
			t.Fatalf("%s sine: got %v want %v (err=%e, tol=%e)",
				testCase.name, got, want, err2, testCase.tol)
		}
	}
}

// --- WithSincN option ---

func TestWithSincN(t *testing.T) {
	delayLine, err := New(64, WithMode(interp.Sinc), WithSincN(4))
	if err != nil {
		t.Fatal(err)
	}

	if delayLine.sincHalfN != 4 {
		t.Fatalf("sincHalfN: got %d want 4", delayLine.sincHalfN)
	}
}

func TestWithSincNIgnoresInvalid(t *testing.T) {
	d, err := New(64, WithMode(interp.Sinc), WithSincN(0))
	if err != nil {
		t.Fatal(err)
	}
	// Should keep default of 8.
	if d.sincHalfN != 8 {
		t.Fatalf("sincHalfN: got %d want 8", d.sincHalfN)
	}
}

// --- benchmarks ---

func BenchmarkReadFractionalLinear(b *testing.B) {
	delayLine, _ := New(1024, WithMode(interp.Linear))
	fillRamp(delayLine)

	for b.Loop() {
		delayLine.ReadFractional(100.37)
	}
}

func BenchmarkReadFractionalHermite(b *testing.B) {
	delayLine, _ := New(1024, WithMode(interp.Hermite))
	fillRamp(delayLine)
	b.ResetTimer()

	for range b.N {
		delayLine.ReadFractional(100.37)
	}
}

func BenchmarkReadFractionalLanczos(b *testing.B) {
	delayLine, _ := New(1024, WithMode(interp.Lanczos3))
	fillRamp(delayLine)
	b.ResetTimer()

	for range b.N {
		delayLine.ReadFractional(100.37)
	}
}

func BenchmarkReadFractionalSinc(b *testing.B) {
	delayLine, _ := New(1024, WithMode(interp.Sinc))
	fillRamp(delayLine)

	for b.Loop() {
		delayLine.ReadFractional(100.37)
	}
}

func BenchmarkReadFractionalAllpass(b *testing.B) {
	delayLine, _ := New(1024, WithMode(interp.Allpass))
	fillRamp(delayLine)

	for b.Loop() {
		delayLine.ReadFractional(100.37)
	}
}
