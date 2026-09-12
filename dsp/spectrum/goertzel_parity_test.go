package spectrum

import (
	"math"
	"testing"
)

// TestGoertzelBankProcessBlockBitExact pins the claim in GoertzelBank.ProcessBlock's
// doc comment: advancing four bins in one pass must produce exactly the state that
// one pass per bin produces, not merely a close approximation. The comparison is ==
// on purpose -- a tolerance here would hide a compiler contracting the multiply-add
// differently in the four-wide body than in the single-bin one.
func TestGoertzelBankProcessBlockBitExact(t *testing.T) {
	freqs := []float64{110, 220, 440, 697, 770, 852, 941, 1209, 1336}

	for bins := 1; bins <= len(freqs); bins++ {
		for _, n := range []int{0, 1, 3, 4, 7, 8, 15, 1024} {
			buf := make([]float64, n)
			for i := range buf {
				buf[i] = math.Sin(2*math.Pi*440*float64(i)/8000) + 0.25*math.Cos(float64(i))
			}

			bank, err := NewGoertzelBank(freqs[:bins], 8000)
			if err != nil {
				t.Fatalf("NewGoertzelBank(%d bins): %v", bins, err)
			}

			bank.ProcessBlock(buf)

			for i := range bins {
				ref, err := NewGoertzel(freqs[i], 8000)
				if err != nil {
					t.Fatalf("NewGoertzel: %v", err)
				}

				ref.ProcessBlock(buf)

				got := bank.Bin(i)
				if got.s0 != ref.s0 || got.s1 != ref.s1 {
					t.Errorf("bins=%d n=%d bin %d: state (%v, %v), want (%v, %v)",
						bins, n, i, got.s0, got.s1, ref.s0, ref.s1)
				}
			}
		}
	}
}

// TestGoertzelBankProcessBlockMatchesProcessSample checks the grouped pass against
// the sample-at-a-time path, which interleaves the bins in the other order.
func TestGoertzelBankProcessBlockMatchesProcessSample(t *testing.T) {
	freqs := []float64{697, 770, 852, 941, 1209, 1336, 1477}

	buf := make([]float64, 512)
	for i := range buf {
		buf[i] = math.Sin(2*math.Pi*770*float64(i)/8000) * 0.7
	}

	block, err := NewGoertzelBank(freqs, 8000)
	if err != nil {
		t.Fatalf("NewGoertzelBank: %v", err)
	}

	sample, err := NewGoertzelBank(freqs, 8000)
	if err != nil {
		t.Fatalf("NewGoertzelBank: %v", err)
	}

	block.ProcessBlock(buf)

	for _, x := range buf {
		sample.ProcessSample(x)
	}

	for i := range freqs {
		if block.Bin(i).Power() != sample.Bin(i).Power() {
			t.Errorf("bin %d: ProcessBlock power %v, ProcessSample power %v",
				i, block.Bin(i).Power(), sample.Bin(i).Power())
		}
	}
}

// TestGoertzelBankProcessBlockAllocs pins the grouped pass as allocation-free.
func TestGoertzelBankProcessBlockAllocs(t *testing.T) {
	bank, err := NewGoertzelBank([]float64{697, 770, 852, 941, 1209, 1336}, 8000)
	if err != nil {
		t.Fatalf("NewGoertzelBank: %v", err)
	}

	buf := make([]float64, 256)

	if got := testing.AllocsPerRun(100, func() { bank.ProcessBlock(buf) }); got != 0 {
		t.Errorf("ProcessBlock allocated %v times per run, want 0", got)
	}
}
