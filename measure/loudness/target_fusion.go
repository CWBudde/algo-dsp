package loudness

import "fmt"

// Preflight has already validated every channel and sample before these paths
// mutate filter/measurement state. Unit-weight mono/stereo filter directly into
// a sequential hop sum, avoiding scratch zeroing, per-channel scratch updates
// and a separate aggregation pass. Coefficient, channel, multiplication and
// addition order match the retained generic path; no rate-specific constants,
// pairwise reductions or alternative summation order are used.
func processTargetMono[T ~float32 | ~float64](a *TargetAnalyzer, input []T, peak float64) error {
	s, h := a.shelf, a.highpass
	weight := a.weights[0]
	state := a.filters[0]
	s0, s1, h0, h1 := state.s0, state.s1, state.h0, state.h1

	a.peak = peak
	for start := 0; start < len(input); {
		count := min(len(input)-start, int(a.nextHop-a.frames))
		sum := a.hopSum

		for _, sample := range input[start : start+count] {
			x := float64(sample)
			y := s.B0*x + s0
			s0 = s.B1*x - s.A1*y + s1
			s1 = s.B2*x - s.A2*y
			z := h.B0*y + h0
			h0 = h.B1*y - h.A1*z + h1
			h1 = h.B2*y - h.A2*z
			energy := 0.0
			energy += weight * z * z
			sum += energy
		}

		if !integratedFinite(s0) || !integratedFinite(s1) || !integratedFinite(h0) || !integratedFinite(h1) {
			return a.fail(fmt.Errorf("loudness.target.process: filter state: %w", ErrNumericalOverflow))
		}

		if err := advanceTargetFusedSegment(a, sum, count); err != nil {
			return err
		}

		start += count
	}

	a.filters[0] = targetFilterState{s0: s0, s1: s1, h0: h0, h1: h1}

	return nil
}

func processTargetStereo[T ~float32 | ~float64](a *TargetAnalyzer, left, right []T, peak float64) error {
	s, h := a.shelf, a.highpass
	weight0, weight1 := a.weights[0], a.weights[1]
	state0, state1 := a.filters[0], a.filters[1]
	s00, s01, h00, h01 := state0.s0, state0.s1, state0.h0, state0.h1
	s10, s11, h10, h11 := state1.s0, state1.s1, state1.h0, state1.h1

	a.peak = peak
	for start := 0; start < len(left); {
		count := min(len(left)-start, int(a.nextHop-a.frames))
		leftSegment, rightSegment := left[start:start+count], right[start:start+count]
		// Atomic preflight guarantees equal lengths; this hint establishes the
		// right-channel loop bound for the compiler without per-frame checks.
		_ = rightSegment[len(leftSegment)-1]
		sum := a.hopSum

		for frame, sample := range leftSegment {
			x0 := float64(sample)
			y0 := s.B0*x0 + s00
			s00 = s.B1*x0 - s.A1*y0 + s01
			s01 = s.B2*x0 - s.A2*y0
			z0 := h.B0*y0 + h00
			h00 = h.B1*y0 - h.A1*z0 + h01
			h01 = h.B2*y0 - h.A2*z0
			x1 := float64(rightSegment[frame])
			y1 := s.B0*x1 + s10
			s10 = s.B1*x1 - s.A1*y1 + s11
			s11 = s.B2*x1 - s.A2*y1
			z1 := h.B0*y1 + h10
			h10 = h.B1*y1 - h.A1*z1 + h11
			h11 = h.B2*y1 - h.A2*z1
			energy := 0.0
			energy += weight0 * z0 * z0
			energy += weight1 * z1 * z1
			sum += energy
		}

		if !integratedFinite(s00) || !integratedFinite(s01) || !integratedFinite(h00) || !integratedFinite(h01) ||
			!integratedFinite(s10) || !integratedFinite(s11) || !integratedFinite(h10) || !integratedFinite(h11) {
			return a.fail(fmt.Errorf("loudness.target.process: filter state: %w", ErrNumericalOverflow))
		}

		if err := advanceTargetFusedSegment(a, sum, count); err != nil {
			return err
		}

		start += count
	}

	a.filters[0] = targetFilterState{s0: s00, s1: s01, h0: h00, h1: h01}
	a.filters[1] = targetFilterState{s0: s10, s1: s11, h0: h10, h1: h11}

	return nil
}

func advanceTargetFusedSegment(a *TargetAnalyzer, sum float64, count int) error {
	// Positive energy cannot recover from Inf/NaN by cancellation. Validate
	// every complete or partial segment, not only completed hops, so terminal
	// overflow is always reported in this bounded ProcessPlanar call.
	if !integratedFinite(sum) {
		return a.fail(fmt.Errorf("loudness.target.process: hop energy: %w", ErrNumericalOverflow))
	}

	a.hopSum = sum

	a.frames += int64(count)
	if a.frames == a.nextHop {
		return a.recordHop()
	}

	return nil
}
