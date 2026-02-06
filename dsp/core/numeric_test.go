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
