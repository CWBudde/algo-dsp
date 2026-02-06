package buffer

import "testing"

func TestNewZeroFilled(t *testing.T) {
	b := New(8)
	if b.Len() != 8 {
		t.Fatalf("Len() = %d, want 8", b.Len())
	}
	for i, v := range b.Samples() {
		if v != 0 {
			t.Fatalf("Samples()[%d] = %v, want 0", i, v)
		}
	}
}

func TestNewNegativeLength(t *testing.T) {
	b := New(-1)
	if b.Len() != 0 {
		t.Fatalf("Len() = %d, want 0 for negative input", b.Len())
	}
}

func TestFromSliceSharesMemory(t *testing.T) {
	s := []float64{1, 2, 3}
	b := FromSlice(s)
	b.Samples()[0] = 99
	if s[0] != 99 {
		t.Fatal("FromSlice should share underlying memory")
	}
}

func TestGrowPreservesData(t *testing.T) {
	b := New(4)
	b.Samples()[0] = 42
	b.Grow(16)
	if b.Cap() < 16 {
		t.Fatalf("Cap() = %d, want >= 16", b.Cap())
	}
	if b.Len() != 4 {
		t.Fatalf("Len() = %d, want 4 after Grow", b.Len())
	}
	if b.Samples()[0] != 42 {
		t.Fatal("Grow did not preserve data")
	}
}

func TestGrowNoOpWhenSufficient(t *testing.T) {
	b := New(4)
	origCap := b.Cap()
	b.Grow(origCap)
	if b.Cap() != origCap {
		t.Fatal("Grow should be no-op when capacity is sufficient")
	}
}

func TestResizeGrow(t *testing.T) {
	b := New(2)
	b.Samples()[0] = 1
	b.Samples()[1] = 2
	b.Resize(4)
	if b.Len() != 4 {
		t.Fatalf("Len() = %d, want 4", b.Len())
	}
	if b.Samples()[0] != 1 || b.Samples()[1] != 2 {
		t.Fatal("Resize did not preserve existing data")
	}
	if b.Samples()[2] != 0 || b.Samples()[3] != 0 {
		t.Fatal("Resize did not zero new elements")
	}
}

func TestResizeShrink(t *testing.T) {
	b := New(8)
	b.Samples()[0] = 5
	b.Resize(2)
	if b.Len() != 2 {
		t.Fatalf("Len() = %d, want 2", b.Len())
	}
	if b.Samples()[0] != 5 {
		t.Fatal("Resize shrink did not preserve data")
	}
}

func TestResizeNegative(t *testing.T) {
	b := New(4)
	b.Resize(-1)
	if b.Len() != 0 {
		t.Fatalf("Len() = %d, want 0", b.Len())
	}
}

func TestResizeReuseClearsStaleData(t *testing.T) {
	b := New(4)
	b.Samples()[0] = 1
	b.Samples()[1] = 2
	b.Samples()[2] = 3
	b.Samples()[3] = 4
	b.Resize(2)
	b.Resize(4)
	// Elements 2 and 3 should be zeroed even though capacity was reused.
	if b.Samples()[2] != 0 || b.Samples()[3] != 0 {
		t.Fatalf("stale data visible after Resize: %v", b.Samples())
	}
}

func TestZero(t *testing.T) {
	b := FromSlice([]float64{1, 2, 3})
	b.Zero()
	for i, v := range b.Samples() {
		if v != 0 {
			t.Fatalf("Samples()[%d] = %v after Zero", i, v)
		}
	}
}

func TestZeroRange(t *testing.T) {
	b := FromSlice([]float64{1, 2, 3, 4, 5})
	b.ZeroRange(1, 4)
	want := []float64{1, 0, 0, 0, 5}
	for i, v := range b.Samples() {
		if v != want[i] {
			t.Fatalf("index %d: got %v, want %v", i, v, want[i])
		}
	}
}

func TestZeroRangeClamps(t *testing.T) {
	b := FromSlice([]float64{1, 2, 3})
	b.ZeroRange(-5, 100)
	for i, v := range b.Samples() {
		if v != 0 {
			t.Fatalf("index %d: got %v, want 0", i, v)
		}
	}
}

func TestCopyIsDeep(t *testing.T) {
	b := FromSlice([]float64{1, 2, 3})
	c := b.Copy()
	c.Samples()[0] = 99
	if b.Samples()[0] == 99 {
		t.Fatal("Copy should not share memory")
	}
	if c.Samples()[0] != 99 {
		t.Fatal("Copy content mismatch")
	}
}
