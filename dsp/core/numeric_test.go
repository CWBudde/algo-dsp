package core

import (
	"math"
	"testing"
)

func TestClamp(t *testing.T) {
	tests := []struct {
		name     string
		value    float64
		min      float64
		max      float64
		expected float64
	}{
		{name: "inside", value: 0.5, min: 0, max: 1, expected: 0.5},
		{name: "below", value: -1, min: 0, max: 1, expected: 0},
		{name: "above", value: 2, min: 0, max: 1, expected: 1},
		{name: "swapped", value: 2, min: 1, max: 0, expected: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Clamp(tt.value, tt.min, tt.max)
			if got != tt.expected {
				t.Fatalf("Clamp() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestNearlyEqual(t *testing.T) {
	if !NearlyEqual(1.0, 1.0+1e-13, 1e-12) {
		t.Fatal("expected values to be nearly equal")
	}

	if NearlyEqual(1.0, 1.1, 1e-3) {
		t.Fatal("expected values to differ")
	}
}

func TestFlushDenormals(t *testing.T) {
	if got := FlushDenormals(1e-40); got != 0 {
		t.Fatalf("FlushDenormals(1e-40) = %v, want 0", got)
	}

	if got := FlushDenormals(-1e-40); got != 0 {
		t.Fatalf("FlushDenormals(-1e-40) = %v, want 0", got)
	}

	if got := FlushDenormals(1e-8); got != 1e-8 {
		t.Fatalf("FlushDenormals(1e-8) = %v, want 1e-8", got)
	}
}

func TestDBConversions(t *testing.T) {
	linear := DBToLinear(-6)

	db := LinearToDB(linear)
	if !NearlyEqual(db, -6, 1e-10) {
		t.Fatalf("LinearToDB(DBToLinear(-6)) = %v, want -6", db)
	}

	if !math.IsInf(LinearToDB(0), -1) {
		t.Fatal("expected -Inf for zero")
	}

	if !math.IsNaN(LinearToDB(-1)) {
		t.Fatal("expected NaN for negative amplitude")
	}
}

func TestDBPowerConversions(t *testing.T) {
	// 3 dB power ~ 2x linear power
	p := DBPowerToLinear(3)
	if !NearlyEqual(p, 2.0, 0.01) {
		t.Fatalf("DBPowerToLinear(3) = %v, want ~2.0", p)
	}

	// Round-trip
	db := LinearPowerToDB(p)
	if !NearlyEqual(db, 3.0, 1e-10) {
		t.Fatalf("LinearPowerToDB(DBPowerToLinear(3)) = %v, want 3", db)
	}

	if !math.IsInf(LinearPowerToDB(0), -1) {
		t.Fatal("expected -Inf for zero power")
	}

	if !math.IsNaN(LinearPowerToDB(-1)) {
		t.Fatal("expected NaN for negative power")
	}
}

func TestDBFloorConversions(t *testing.T) {
	t.Parallel()

	nan, inf := math.NaN(), math.Inf(1)
	tests := []struct {
		name          string
		value, floor  float64
		amplitudeWant float64
		powerWant     float64
	}{
		{name: "above floor", value: 0.5, floor: 1e-6, amplitudeWant: LinearToDB(0.5), powerWant: LinearPowerToDB(0.5)},
		{name: "equal floor", value: 1e-6, floor: 1e-6, amplitudeWant: LinearToDB(1e-6), powerWant: -60},
		{name: "zero floored", value: 0, floor: 1e-6, amplitudeWant: LinearToDB(1e-6), powerWant: -60},
		{name: "negative floored", value: -1, floor: 1e-6, amplitudeWant: LinearToDB(1e-6), powerWant: -60},
		{name: "tiny floored", value: 1e-300, floor: 1e-12, amplitudeWant: LinearToDB(1e-12), powerWant: -120},
		{name: "infinity", value: inf, floor: 1e-6, amplitudeWant: inf, powerWant: inf},
		{name: "nan input", value: nan, floor: 1e-6, amplitudeWant: nan, powerWant: nan},
		{name: "zero floor zero", value: 0, floor: 0, amplitudeWant: math.Inf(-1), powerWant: math.Inf(-1)},
		{name: "negative floor", value: -1, floor: -1, amplitudeWant: nan, powerWant: nan},
		{name: "nan floor", value: 0, floor: nan, amplitudeWant: math.Inf(-1), powerWant: math.Inf(-1)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			checkDB(t, "LinearToDBFloor", LinearToDBFloor(tt.value, tt.floor), tt.amplitudeWant)
			checkDB(t, "LinearPowerToDBFloor", LinearPowerToDBFloor(tt.value, tt.floor), tt.powerWant)
		})
	}

	// Bit-identical to the inline idiom 20*log10(max(x, floor)).
	for _, x := range []float64{1e-9, 1e-6, 3.7e-4, 0.25, 1, 17} {
		if got, want := LinearToDBFloor(x, 1e-6), 20*math.Log10(math.Max(x, 1e-6)); got != want {
			t.Fatalf("LinearToDBFloor(%g) = %v, want %v", x, got, want)
		}
	}

	if got := LinearToDBFloor(0, 1e-6); !NearlyEqual(got, -120, 1e-12) {
		t.Fatalf("LinearToDBFloor(0, 1e-6) = %v, want -120", got)
	}
}

func checkDB(t *testing.T, name string, got, want float64) {
	t.Helper()

	if math.IsNaN(want) {
		if !math.IsNaN(got) {
			t.Fatalf("%s = %v, want NaN", name, got)
		}

		return
	}

	if got != want && !NearlyEqual(got, want, 1e-12) {
		t.Fatalf("%s = %v, want %v", name, got, want)
	}
}
