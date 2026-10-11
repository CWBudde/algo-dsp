package core

import "unsafe"

// Overlaps reports whether a and b share any element.
//
// The test is based on the slices' lengths, not their capacities: two slices
// that only share spare capacity (for example buf[:2] and buf[2:4]) do not
// overlap. Empty or nil slices never overlap anything. Overlaps does not
// allocate.
//
// Use it to reject or special-case in-place processing where a destination
// buffer aliases a source buffer.
func Overlaps(a, b []float64) bool {
	if len(a) == 0 || len(b) == 0 {
		return false
	}

	a0 := uintptr(unsafe.Pointer(unsafe.SliceData(a)))
	b0 := uintptr(unsafe.Pointer(unsafe.SliceData(b)))
	size := unsafe.Sizeof(a[0])

	return a0 < b0+uintptr(len(b))*size && b0 < a0+uintptr(len(a))*size
}
