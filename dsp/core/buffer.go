package core

// EnsureLen returns a slice with the requested length, reusing buf capacity if possible.
func EnsureLen(buf []float64, n int) []float64 {
	if n <= 0 {
		return buf[:0]
	}
	if cap(buf) >= n {
		return buf[:n]
	}
	return make([]float64, n)
}

// Zero sets all values in buf to 0.
func Zero(buf []float64) {
	for i := range buf {
		buf[i] = 0
	}
}

// CopyInto copies src into dst and returns the number of copied elements.
func CopyInto(dst, src []float64) int {
	n := len(dst)
	if len(src) < n {
		n = len(src)
	}
	copy(dst[:n], src[:n])
	return n
}
