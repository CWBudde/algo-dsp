package core

import "testing"

func TestOverlaps(t *testing.T) {
	buf := make([]float64, 8, 16)
	other := make([]float64, 8)

	tests := []struct {
		name string
		a, b []float64
		want bool
	}{
		{name: "disjoint same array", a: buf[0:2], b: buf[5:7], want: false},
		{name: "adjacent a before b", a: buf[0:3], b: buf[3:6], want: false},
		{name: "adjacent b before a", a: buf[3:6], b: buf[0:3], want: false},
		{name: "partial overlap", a: buf[0:4], b: buf[3:6], want: true},
		{name: "partial overlap reversed", a: buf[3:6], b: buf[0:4], want: true},
		{name: "identical", a: buf, b: buf, want: true},
		{name: "a contains b", a: buf, b: buf[2:4], want: true},
		{name: "b contains a", a: buf[2:4], b: buf, want: true},
		{name: "single shared element", a: buf[0:3], b: buf[2:3], want: true},
		{name: "a empty", a: buf[2:2], b: buf, want: false},
		{name: "b empty", a: buf, b: buf[2:2], want: false},
		{name: "both nil", a: nil, b: nil, want: false},
		{name: "a nil", a: nil, b: buf, want: false},
		{name: "shared capacity only", a: buf[:2], b: buf[2:4], want: false},
		{name: "tail capacity only", a: buf[:8], b: buf[8:12], want: false},
		{name: "different arrays", a: buf, b: other, want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := Overlaps(tc.a, tc.b); got != tc.want {
				t.Fatalf("Overlaps = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestOverlapsZeroAlloc(t *testing.T) {
	buf := make([]float64, 8)

	allocs := testing.AllocsPerRun(100, func() {
		_ = Overlaps(buf[:4], buf[2:6])
	})
	if allocs != 0 {
		t.Fatalf("allocs = %v, want 0", allocs)
	}
}
