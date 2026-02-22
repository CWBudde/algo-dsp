package core

import "testing"

func TestEnsureLenReuse(t *testing.T) {
	buf := make([]float64, 4, 8)

	out := EnsureLen(buf, 6)
	if len(out) != 6 {
		t.Fatalf("len = %d, want 6", len(out))
	}

	if cap(out) != cap(buf) {
		t.Fatalf("cap = %d, want %d", cap(out), cap(buf))
	}
}

func TestCopyInto(t *testing.T) {
	dst := make([]float64, 2)

	n := CopyInto(dst, []float64{1, 2, 3})
	if n != 2 {
		t.Fatalf("n = %d, want 2", n)
	}

	if dst[0] != 1 || dst[1] != 2 {
		t.Fatalf("unexpected dst: %#v", dst)
	}
}

func TestZero(t *testing.T) {
	buf := []float64{1, 2, 3}
	Zero(buf)

	for i, v := range buf {
		if v != 0 {
			t.Fatalf("buf[%d] = %v, want 0", i, v)
		}
	}
}
