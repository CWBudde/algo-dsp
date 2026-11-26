package delay

import "testing"

func TestTimeHermite32ImpulseAndResize(t *testing.T) {
	d := NewTimeHermite32(64)
	for _, tc := range []struct {
		samples float64
		want    []float32
	}{
		{5.5, []float32{0, 0, 0, -0.0625, 0.5625, 0.5625, -0.0625, 0}},
		{5, []float32{0, 0, 0, 0, 1, 0, 0, 0}},
		{1, []float32{0, 1, 0, 0, 0, 0, 0, 0}},
		{0.5, []float32{-0.0625, 0.5625, 0.5625, -0.0625, 0, 0, 0, 0}},
	} {
		d.SetSamples(tc.samples)

		for i, want := range tc.want {
			var sample float32
			if i == 0 {
				sample = 1
			}

			if got := d.ProcessSample(sample); got != want {
				t.Errorf("delay %g sample %d: got %g, want %g", tc.samples, i, got, want)
			}
		}
	}

	if got := testing.AllocsPerRun(50, func() {
		d.SetSamples(0.5)
		d.SetSamples(63.5)
		d.ProcessSample(1)
	}); got != 0 {
		t.Errorf("delay resize allocated %g times", got)
	}
}

func TestTimeHermite32ReserveRetainsTap(t *testing.T) {
	d := NewTimeHermite32(5)
	d.SetSamples(5)
	d.ProcessSample(1)
	d.ProcessSample(0)
	d.ReserveSamples(16)

	for index, want := range []float32{0, 0, 1, 0} {
		if got := d.ProcessSample(0); got != want {
			t.Errorf("frame %d got %g, want %g", index+2, got, want)
		}
	}
}
