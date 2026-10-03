package structure

import (
	"fmt"
	"math"
)

// DefaultPrime is the default suffix of a variant label ("A'"). Use
// [WithPrime] for the typographic prime ′ (U+2032).
const DefaultPrime = "'"

// Phrase is the label of one unit (phrase) found by [Label].
type Phrase struct {
	// Label is the letter, with the prime suffix for a variant: "A", "B",
	// "A'". Letters beyond Z carry a digit: "A1", "B1", … for the 27th,
	// 28th, … letter.
	Label string
	// Letter is the letter index (0 for A).
	Letter int
	// Variant reports a primed variant of an earlier letter.
	Variant bool
	// Similarity is the mean diagonal similarity of this unit to the first
	// unit carrying its letter (to itself for a new letter).
	Similarity float64
}

// LabelOption configures [Label]. Options return an error wrapping
// [ErrInvalidArgument] for invalid values.
type LabelOption func(*labelConfig) error

type labelConfig struct {
	lower bool
	prime string
}

// WithLowercase writes the labels in lower case ("a", "b", "a'"), for
// example to tell bar labels from phrase labels.
func WithLowercase() LabelOption {
	return func(cfg *labelConfig) error {
		cfg.lower = true

		return nil
	}
}

// WithPrime sets the variant suffix (default [DefaultPrime]). It must not be
// empty.
func WithPrime(prime string) LabelOption {
	return func(cfg *labelConfig) error {
		if prime == "" {
			return fmt.Errorf("%w: empty prime", ErrInvalidArgument)
		}

		cfg.prime = prime

		return nil
	}
}

// Label assigns letters to consecutive units of unit rows of the
// self-similarity matrix ssm, for example phrases of 4 bars of a bar SSM, or
// single bars with unit 1. The similarity of units u and v is the mean of
// the diagonal entries ssm[u·unit+k][v·unit+k] inside the matrix (a short
// last unit compares only its rows).
//
// Units are labelled in order. Unit u is compared with every earlier unit
// and takes the letter of the most similar one (the lowest letter on a
// tie):
//
//   - similarity ≥ same: the same letter ("A");
//   - similarity ≥ variant: a primed variant of it ("A'");
//   - otherwise, or for the first unit: the next new letter.
//
// ssm must be square, unit at least 1, and same and variant finite
// ([DefaultPhraseRows], [DefaultSame] and [DefaultVariant] are 4, 0.6 and
// 0.35). Label returns one [Phrase] per unit, ceil(len(ssm)/unit) in all.
func Label(ssm [][]float64, unit int, same, variant float64, opts ...LabelOption) ([]Phrase, error) {
	if unit < 1 {
		return nil, fmt.Errorf("%w: unit must be >= 1, got %d", ErrInvalidArgument, unit)
	}

	if math.IsNaN(same) || math.IsInf(same, 0) || math.IsNaN(variant) || math.IsInf(variant, 0) {
		return nil, fmt.Errorf("%w: thresholds same %g, variant %g", ErrInvalidArgument, same, variant)
	}

	err := checkSquare(ssm)
	if err != nil {
		return nil, err
	}

	cfg := labelConfig{prime: DefaultPrime}

	for i, opt := range opts {
		if opt == nil {
			return nil, fmt.Errorf("%w: option %d", ErrNilOption, i)
		}

		err := opt(&cfg)
		if err != nil {
			return nil, err
		}
	}

	units := (len(ssm) + unit - 1) / unit
	phrases := make([]Phrase, units)

	var first []int // first unit of each letter

	for u := range phrases {
		bestLetter, bestSim := -1, math.Inf(-1)

		for letter := range first {
			for v := range u {
				if phrases[v].Letter == letter {
					if s := unitSimilarity(ssm, unit, u, v); s > bestSim {
						bestLetter, bestSim = letter, s
					}
				}
			}
		}

		p := &phrases[u]

		switch {
		case bestLetter >= 0 && bestSim >= same:
		case bestLetter >= 0 && bestSim >= variant:
			p.Variant = true
		default:
			bestLetter = len(first)
			first = append(first, u)
		}

		p.Letter = bestLetter
		p.Label = letterName(bestLetter, cfg.lower)

		if p.Variant {
			p.Label += cfg.prime
		}

		p.Similarity = unitSimilarity(ssm, unit, u, first[bestLetter])
	}

	return phrases, nil
}

// unitSimilarity is the mean diagonal similarity of units a and b. Every
// unit starts inside the matrix, so at least one entry is averaged.
func unitSimilarity(s [][]float64, size, a, b int) float64 {
	v, n := 0.0, 0

	for k := range size {
		i, j := a*size+k, b*size+k
		if i < len(s) && j < len(s) {
			v += s[i][j]
			n++
		}
	}

	return v / float64(n)
}

// letterName names letter i: A..Z, then A1..Z1, A2, … (lower case on
// request).
func letterName(i int, lower bool) string {
	a := 'A'
	if lower {
		a = 'a'
	}

	if i < 26 {
		return string(a + rune(i))
	}

	return string(a+rune(i%26)) + string(rune('0'+i/26))
}
