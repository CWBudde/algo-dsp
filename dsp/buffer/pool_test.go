package buffer

import "testing"

func TestPoolGetReturnsZeroed(t *testing.T) {
	p := NewPool()

	b := p.Get(8)
	if b.Len() != 8 {
		t.Fatalf("Len() = %d, want 8", b.Len())
	}

	for i, v := range b.Samples() {
		if v != 0 {
			t.Fatalf("Samples()[%d] = %v, want 0", i, v)
		}
	}

	p.Put(b)
}

func TestPoolReuseIsZeroed(t *testing.T) {
	p := NewPool()

	// Get, write data, return.
	b := p.Get(4)
	b.Samples()[0] = 42
	b.Samples()[1] = 43
	p.Put(b)

	// Get again — should be zeroed regardless of reuse.
	b2 := p.Get(4)
	for i, v := range b2.Samples() {
		if v != 0 {
			t.Fatalf("reused Samples()[%d] = %v, want 0", i, v)
		}
	}

	p.Put(b2)
}

func TestPoolPutNilSafe(_ *testing.T) {
	p := NewPool()
	p.Put(nil) // must not panic
}
