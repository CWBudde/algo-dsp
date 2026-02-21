package reverb

func hermite4(t, xm1, x0, x1, x2 float64) float64 {
	c0 := x0
	c1 := 0.5 * (x1 - xm1)
	c2 := xm1 - 2.5*x0 + 2*x1 - 0.5*x2
	c3 := 0.5*(x2-xm1) + 1.5*(x0-x1)
	return ((c3*t+c2)*t+c1)*t + c0
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
