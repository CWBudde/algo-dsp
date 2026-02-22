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
	d, err := New(16)
	if err != nil {
		t.Fatal(err)
	}

	if d.Len() != 16 {
		t.Fatalf("Len: got %d want 16", d.Len())
	}

	if d.mode != interp.Hermite {
		t.Fatalf("default mode: got %v want Hermite", d.mode)
	}
}

func TestNewWithOptions(t *testing.T) {
	d, err := New(16, WithMode(interp.Linear))
	if err != nil {
		t.Fatal(err)
	}

	if d.mode != interp.Linear {
		t.Fatalf("mode: got %v want Linear", d.mode)
	}
}

// --- integer Read/Write ---

func TestReadWrite(t *testing.T) {
	d, err := New(8)
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 8; i++ {
		d.Write(float64(i))
	}
	// delay=1 => most recently written (7)
	if got := d.Read(1); got != 7 {
		t.Fatalf("got %v want 7", got)
	}
	// delay=3 => 3 samples back from write head
	if got := d.Read(3); got != 5 {
		t.Fatalf("got %v want 5", got)
	}
}

func TestReadWraparound(t *testing.T) {
	d, err := New(4)
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 10; i++ {
		d.Write(float64(i))
	}
	// buffer should contain [8, 9, 6, 7], writePos=2
	// Read(1) = most recent = 9
	if got := d.Read(1); got != 9 {
		t.Fatalf("got %v want 9", got)
	}
}

func TestReset(t *testing.T) {
	d, err := New(4)
	if err != nil {
		t.Fatal(err)
	}

	d.Write(1)
	d.Write(2)
	d.Reset()

	for i := 0; i < 4; i++ {
		if got := d.Read(i); got != 0 {
			t.Fatalf("after reset Read(%d): got %v want 0", i, got)
		}
	}
}

// --- fractional read with default (Hermite) ---

func TestReadFractionalLinearRamp(t *testing.T) {
	d, err := New(16)
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < d.Len(); i++ {
		d.Write(float64(i))
	}

	if got := d.ReadFractional(3.5); got < 12.49 || got > 12.51 {
		t.Fatalf("got %v want about 12.5", got)
	}
}

func TestReadFractionalNegativeClamped(t *testing.T) {
	d, err := New(8)
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 8; i++ {
		d.Write(float64(i + 1))
	}

	got := d.ReadFractional(-1.0)
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
	d, err := New(32, WithMode(interp.Linear))
	if err != nil {
		t.Fatal(err)
	}

	fillRamp(d)
	// With a linear ramp, linear interpolation is exact.
	got := d.ReadFractional(5.5)

	want := float64(d.Len()) - 5.5 // 26.5
	if !approxEqual(got, want, 1e-10) {
		t.Fatalf("Linear: got %v want %v", got, want)
	}
}

func TestReadFractionalHermite(t *testing.T) {
	d, err := New(32, WithMode(interp.Hermite))
	if err != nil {
		t.Fatal(err)
	}

	fillRamp(d)
	got := d.ReadFractional(5.5)

	want := float64(d.Len()) - 5.5
	if !approxEqual(got, want, 1e-10) {
		t.Fatalf("Hermite: got %v want %v", got, want)
	}
}

func TestReadFractionalLagrange(t *testing.T) {
	d, err := New(32, WithMode(interp.Lagrange3))
	if err != nil {
		t.Fatal(err)
	}

	fillRamp(d)
	got := d.ReadFractional(5.5)

	want := float64(d.Len()) - 5.5
	if !approxEqual(got, want, 1e-10) {
		t.Fatalf("Lagrange3: got %v want %v", got, want)
	}
}

func TestReadFractionalLanczos(t *testing.T) {
	d, err := New(64, WithMode(interp.Lanczos3))
	if err != nil {
		t.Fatal(err)
	}

	fillRamp(d)
	got := d.ReadFractional(10.5)
	want := float64(d.Len()) - 10.5
	// Lanczos is approximate on a finite ramp.
	if !approxEqual(got, want, 0.5) {
		t.Fatalf("Lanczos3: got %v want ~%v", got, want)
	}
}

func TestReadFractionalSinc(t *testing.T) {
	d, err := New(64, WithMode(interp.Sinc), WithSincN(4))
	if err != nil {
		t.Fatal(err)
	}

	fillRamp(d)
	got := d.ReadFractional(10.5)

	want := float64(d.Len()) - 10.5
	if !approxEqual(got, want, 0.5) {
		t.Fatalf("Sinc: got %v want ~%v", got, want)
	}
}

func TestReadFractionalAllpass(t *testing.T) {
	d, err := New(64, WithMode(interp.Allpass))
	if err != nil {
		t.Fatal(err)
	}
	// Fill with DC to let the allpass state settle.
	for i := 0; i < d.Len(); i++ {
		d.Write(50.0)
	}

	for i := 0; i < 50; i++ {
		d.ReadFractional(10.5)
	}
	// Now fill with ramp and verify the output is finite and in range.
	d.Reset()
	fillRamp(d)

	got := d.ReadFractional(10.5)
	if math.IsNaN(got) || math.IsInf(got, 0) {
		t.Fatalf("Allpass: produced %v", got)
	}
	// Allpass state is reset, so first-call accuracy is limited;
	// just check it's in the right ballpark.
	want := float64(d.Len()) - 10.5
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

	for _, tc := range modes {
		d, err := New(32, WithMode(tc.mode))
		if err != nil {
			t.Fatal(err)
		}
		// Fill with constant value.
		for i := 0; i < d.Len(); i++ {
			d.Write(42.0)
		}

		got := d.ReadFractional(5.3)
		if !approxEqual(got, 42.0, 1e-6) {
			t.Fatalf("%s DC: got %v want 42", tc.name, got)
		}
	}
}

// --- allpass DC convergence (needs state settling) ---

func TestAllpassDCConvergence(t *testing.T) {
	d, err := New(32, WithMode(interp.Allpass))
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < d.Len(); i++ {
		d.Write(10.0)
	}
	// Read multiple times at the same delay to let the allpass state settle.
	var got float64
	for i := 0; i < 100; i++ {
		got = d.ReadFractional(5.3)
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

	for _, tc := range modes {
		d, err := New(size, WithMode(tc.mode))
		if err != nil {
			t.Fatal(err)
		}

		for i := 0; i < size; i++ {
			d.Write(math.Sin(2 * math.Pi * freq * float64(i)))
		}

		delay := 20.37
		// Read(k) for integer k returns sample written at index (size-k),
		// so fractional delay d corresponds to sample index (size-d).
		exactSample := float64(size) - delay
		want := math.Sin(2 * math.Pi * freq * exactSample)
		got := d.ReadFractional(delay)

		err2 := math.Abs(got - want)
		if err2 > tc.tol {
			t.Fatalf("%s sine: got %v want %v (err=%e, tol=%e)",
				tc.name, got, want, err2, tc.tol)
		}
	}
}

// --- WithSincN option ---

func TestWithSincN(t *testing.T) {
	d, err := New(64, WithMode(interp.Sinc), WithSincN(4))
	if err != nil {
		t.Fatal(err)
	}

	if d.sincHalfN != 4 {
		t.Fatalf("sincHalfN: got %d want 4", d.sincHalfN)
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
	d, _ := New(1024, WithMode(interp.Linear))
	fillRamp(d)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		d.ReadFractional(100.37)
	}
}

func BenchmarkReadFractionalHermite(b *testing.B) {
	d, _ := New(1024, WithMode(interp.Hermite))
	fillRamp(d)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		d.ReadFractional(100.37)
	}
}

func BenchmarkReadFractionalLanczos(b *testing.B) {
	d, _ := New(1024, WithMode(interp.Lanczos3))
	fillRamp(d)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		d.ReadFractional(100.37)
	}
}

func BenchmarkReadFractionalSinc(b *testing.B) {
	d, _ := New(1024, WithMode(interp.Sinc))
	fillRamp(d)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		d.ReadFractional(100.37)
	}
}

func BenchmarkReadFractionalAllpass(b *testing.B) {
	d, _ := New(1024, WithMode(interp.Allpass))
	fillRamp(d)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		d.ReadFractional(100.37)
	}
}
