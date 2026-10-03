package harmony

// viterbi decodes the best state path through nw windows of ns states with a
// constant switch cost. scores holds the per-window state scores row by row
// (nw·ns values, -Inf for impossible states); acc and next are scratch rows of
// ns values, back holds nw·ns back pointers, and the decoded states are
// written to path[:nw]. It does not allocate.
//
// Staying in a state wins ties with switching, and among equal scores the
// lower state index wins, so the result is deterministic. Only the best
// previous state needs to be considered for a switch because the switch cost
// is the same for every pair of states, which makes a step O(ns) rather than
// O(ns²). The operation order matches AudioVisualizer's DetectChords.
//
// It stays unexported until a second user appears (Phase 45 decision gate);
// a general HMM decoder would then move to stats/hmm.
func viterbi(scores []float64, nw, ns int, switchCost float64, acc, next []float64, back, path []int32) {
	if nw == 0 {
		return
	}

	copy(acc[:ns], scores[:ns])

	for w := 1; w < nw; w++ {
		row := scores[w*ns : (w+1)*ns]
		ptr := back[w*ns : (w+1)*ns]

		bestPrev := 0

		for s := range ns {
			if acc[s] > acc[bestPrev] {
				bestPrev = s
			}
		}

		for s := range ns {
			stay, move := acc[s], acc[bestPrev]-switchCost
			if stay >= move {
				next[s], ptr[s] = stay+row[s], int32(s)
			} else {
				next[s], ptr[s] = move+row[s], int32(bestPrev)
			}
		}

		acc, next = next, acc
	}

	last := 0

	for s := range ns {
		if acc[s] > acc[last] {
			last = s
		}
	}

	for w := nw - 1; w >= 0; w-- {
		path[w] = int32(last)
		if w > 0 {
			last = int(back[w*ns+last])
		}
	}
}
