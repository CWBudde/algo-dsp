package buffer

// Buffer wraps a float64 slice with reuse-friendly semantics.
// DSP functions accept raw []float64; use Samples() to bridge.
type Buffer struct {
	samples []float64
}

// New returns a zero-filled Buffer of the given length.
func New(length int) *Buffer {
	if length < 0 {
		length = 0
	}
	return &Buffer{samples: make([]float64, length)}
}

// FromSlice wraps an existing slice without copying.
// Mutations to the slice are visible through the Buffer and vice versa.
func FromSlice(s []float64) *Buffer {
	return &Buffer{samples: s}
}

// Samples returns the underlying slice.
func (b *Buffer) Samples() []float64 {
	return b.samples
}

// Len returns the current number of samples.
func (b *Buffer) Len() int {
	return len(b.samples)
}

// Cap returns the current capacity of the backing slice.
func (b *Buffer) Cap() int {
	return cap(b.samples)
}

// Grow ensures capacity is at least n, preserving existing data.
// If the current capacity is already >= n this is a no-op.
func (b *Buffer) Grow(n int) {
	if n <= cap(b.samples) {
		return
	}
	grown := make([]float64, len(b.samples), n)
	copy(grown, b.samples)
	b.samples = grown
}

// Resize sets the length to n, reusing existing capacity when possible.
// New elements beyond the previous length are zeroed.
func (b *Buffer) Resize(n int) {
	if n < 0 {
		n = 0
	}
	oldLen := len(b.samples)
	if n <= cap(b.samples) {
		b.samples = b.samples[:n]
	} else {
		s := make([]float64, n)
		copy(s, b.samples)
		b.samples = s
	}
	// Zero any newly exposed elements that may have stale data from
	// previous use of the backing array.
	if n > oldLen {
		for i := oldLen; i < n; i++ {
			b.samples[i] = 0
		}
	}
}

// Zero sets all samples to 0.
func (b *Buffer) Zero() {
	for i := range b.samples {
		b.samples[i] = 0
	}
}

// ZeroRange sets samples in [start, end) to 0.
// Indices are clamped to valid bounds.
func (b *Buffer) ZeroRange(start, end int) {
	if start < 0 {
		start = 0
	}
	if end > len(b.samples) {
		end = len(b.samples)
	}
	for i := start; i < end; i++ {
		b.samples[i] = 0
	}
}

// Copy returns a deep copy of the buffer.
func (b *Buffer) Copy() *Buffer {
	s := make([]float64, len(b.samples))
	copy(s, b.samples)
	return &Buffer{samples: s}
}
