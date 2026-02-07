package vecmath

import "math"

// Benchmark sizes shared across all benchmark files
var benchSizes = []struct {
	name string
	size int
}{
	{"16", 16},
	{"64", 64},
	{"256", 256},
	{"1K", 1024},
	{"4K", 4096},
	{"16K", 16384},
	{"64K", 65536},
}

// Test helper functions shared across all test files

func closeEnough(a, b float64) bool {
	const epsilon = 1e-14
	if a == b {
		return true
	}
	diff := math.Abs(a - b)
	if a == 0 || b == 0 {
		return diff < epsilon
	}
	return diff/math.Max(math.Abs(a), math.Abs(b)) < epsilon
}

func sizeStr(n int) string {
	return "n=" + itoa(n)
}

func floatStr(f float64) string {
	if f == 0.0 {
		return "0"
	}
	if f == 1.0 {
		return "1"
	}
	if f == -1.0 {
		return "-1"
	}
	if f == 0.5 {
		return "0.5"
	}
	if f == 2.0 {
		return "2"
	}
	return "pi"
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}
