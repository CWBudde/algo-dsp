package structure

import (
	"errors"
	"fmt"
	"math"
)

// Sentinel errors returned by this package. Errors are wrapped with context,
// so test for them with errors.Is.
var (
	// ErrInvalidArgument reports an argument or option value outside its
	// valid range, including non-finite input values.
	ErrInvalidArgument = errors.New("structure: invalid argument")
	// ErrShape reports rows of unequal length, blocks with unequal row
	// counts, a non-square matrix, or a destination of the wrong size.
	ErrShape = errors.New("structure: shape mismatch")
	// ErrNilOption reports a nil option.
	ErrNilOption = errors.New("structure: nil option")
)

// Defaults of the AudioVisualizer story analysis, in rows of the matrix the
// functions work on (beats for novelty and peaks, bars for labels).
const (
	// DefaultHalfWidth is the default half-width of the [FooteNovelty]
	// kernel (8 beats).
	DefaultHalfWidth = 8
	// DefaultPeakRadius is the default radius of the [Peaks] local-maximum
	// test (±4 beats).
	DefaultPeakRadius = 4
	// DefaultPeakSigma is the default [Peaks] threshold in standard
	// deviations above the mean novelty.
	DefaultPeakSigma = 1.0
	// DefaultPhraseRows is the default phrase length passed to [Label]
	// (4 bars).
	DefaultPhraseRows = 4
	// DefaultSame is the default [Label] similarity from which a phrase
	// reuses an earlier letter.
	DefaultSame = 0.6
	// DefaultVariant is the default [Label] similarity from which a phrase
	// becomes a primed variant of an earlier letter.
	DefaultVariant = 0.35
)

// zeroSD is the standard deviation below which [ZScore] treats a dimension
// as constant.
const zeroSD = 1e-12

// checkRows reports whether all rows have the same length and returns it.
func checkRows(rows [][]float64) (int, error) {
	if len(rows) == 0 {
		return 0, nil
	}

	dims := len(rows[0])

	for i, r := range rows {
		if len(r) != dims {
			return 0, fmt.Errorf("%w: row %d has %d values, row 0 has %d", ErrShape, i, len(r), dims)
		}
	}

	return dims, nil
}

// checkFinite reports whether every value of rows is finite. It does not
// allocate unless it fails.
func checkFinite(name string, rows [][]float64) error {
	for i, r := range rows {
		for j, v := range r {
			if math.IsNaN(v) || math.IsInf(v, 0) {
				return fmt.Errorf("%w: %s[%d][%d] is %g", ErrInvalidArgument, name, i, j, v)
			}
		}
	}

	return nil
}

// checkSquare reports whether s is an n×n matrix.
func checkSquare(s [][]float64) error {
	for i, r := range s {
		if len(r) != len(s) {
			return fmt.Errorf("%w: matrix row %d has %d values, want %d", ErrShape, i, len(r), len(s))
		}
	}

	return nil
}

// ZScore standardises each dimension (column) of rows in place to zero mean
// and unit population standard deviation. Dimensions whose standard
// deviation is at most 1e-12 (constant dimensions) become 0. All rows must
// have the same length; otherwise ZScore returns an error wrapping
// [ErrShape] and leaves rows unchanged.
func ZScore(rows [][]float64) error {
	_, err := checkRows(rows)
	if err != nil {
		return err
	}

	zscore(rows)

	return nil
}

// zscore is [ZScore] on validated rows, in the reference operation order.
func zscore(rows [][]float64) {
	if len(rows) == 0 {
		return
	}

	n := float64(len(rows))

	for d := range rows[0] {
		mean, sq := 0.0, 0.0
		for _, r := range rows {
			mean += r[d] / n
		}

		for _, r := range rows {
			sq += (r[d] - mean) * (r[d] - mean)
		}

		sd := math.Sqrt(sq / n)

		for _, r := range rows {
			if sd > zeroSD {
				r[d] = (r[d] - mean) / sd
			} else {
				r[d] = 0
			}
		}
	}
}

// Block is one feature group of a row vector, for example the 12-bin chroma
// of every beat. Rows holds one vector per row of the analysis (beat or bar)
// and Weight its share in the concatenated vector.
type Block struct {
	// Rows is the group's feature matrix, [row][dim].
	Rows [][]float64
	// Weight scales the group after normalisation; it must be finite.
	Weight float64
}

// Blocks mixes heterogeneous feature groups into one vector per row. Each
// block is z-scored in place (see [ZScore]; Blocks modifies the block rows),
// then every row of the block is L2-normalised and scaled by the block's
// weight (rows with zero norm stay zero), and the blocks are concatenated in
// order. Every block must have the same number of rows and equal-length rows.
// With L2-normalised groups the weights set each group's share of a row's
// energy, so a 5-band dB group and a 12-bin chroma group can be mixed in a
// chosen ratio.
//
// Blocks returns nil for no blocks. The result rows share one backing array.
func Blocks(blocks []Block) ([][]float64, error) {
	if len(blocks) == 0 {
		return nil, nil
	}

	rows := len(blocks[0].Rows)
	total := 0

	for k, b := range blocks {
		if len(b.Rows) != rows {
			return nil, fmt.Errorf("%w: block %d has %d rows, block 0 has %d", ErrShape, k, len(b.Rows), rows)
		}

		dims, err := checkRows(b.Rows)
		if err != nil {
			return nil, fmt.Errorf("block %d: %w", k, err)
		}

		if math.IsNaN(b.Weight) || math.IsInf(b.Weight, 0) {
			return nil, fmt.Errorf("%w: block %d weight %g", ErrInvalidArgument, k, b.Weight)
		}

		total += dims
	}

	backing := make([]float64, rows*total)
	out := make([][]float64, rows)

	for u := range out {
		out[u] = backing[u*total : (u+1)*total : (u+1)*total]
	}

	offset := 0

	for _, b := range blocks {
		zscore(b.Rows)

		for u, v := range b.Rows {
			norm := 0.0
			for _, x := range v {
				norm += x * x
			}

			norm = math.Sqrt(norm)
			dst := out[u][offset : offset+len(v)]

			for d, x := range v {
				if norm > 0 {
					x *= b.Weight / norm
				}

				dst[d] = x
			}
		}

		if rows > 0 {
			offset += len(b.Rows[0])
		}
	}

	return out, nil
}

// cosine is the cosine similarity of a and b, 0 if either has zero norm.
func cosine(a, b []float64) float64 {
	dot, na, nb := 0.0, 0.0, 0.0
	for i := range a {
		dot += a[i] * b[i]
		na += a[i] * a[i]
		nb += b[i] * b[i]
	}

	if na == 0 || nb == 0 {
		return 0
	}

	return dot / math.Sqrt(na*nb)
}

// SelfSimilarity returns the n×n cosine self-similarity matrix of n rows:
// entry (i, j) is the cosine of rows i and j, in [-1, 1], and 0 when either
// row is all zeros. All rows must have the same length, and all values must
// be finite (else the error wraps [ErrInvalidArgument]).
func SelfSimilarity(rows [][]float64) ([][]float64, error) {
	err := checkSimilarityRows(rows)
	if err != nil {
		return nil, err
	}

	n := len(rows)
	backing := make([]float64, n*n)
	s := make([][]float64, n)

	for i := range s {
		s[i] = backing[i*n : (i+1)*n : (i+1)*n]
	}

	selfSimilarity(s, rows)

	return s, nil
}

// SelfSimilarityInto writes the cosine self-similarity matrix of rows (see
// [SelfSimilarity]) into dst, which must have len(rows) rows of len(rows)
// values. It does not allocate.
func SelfSimilarityInto(dst, rows [][]float64) error {
	err := checkSimilarityRows(rows)
	if err != nil {
		return err
	}

	if len(dst) != len(rows) {
		return fmt.Errorf("%w: destination has %d rows, want %d", ErrShape, len(dst), len(rows))
	}

	err = checkSquare(dst)
	if err != nil {
		return fmt.Errorf("destination: %w", err)
	}

	selfSimilarity(dst, rows)

	return nil
}

func checkSimilarityRows(rows [][]float64) error {
	_, err := checkRows(rows)
	if err != nil {
		return err
	}

	return checkFinite("rows", rows)
}

// selfSimilarity fills the symmetric matrix s. cosine(a, b) and cosine(b, a)
// are bit-identical (the products commute), so only the upper triangle is
// computed.
func selfSimilarity(s, rows [][]float64) {
	for i := range rows {
		for j := i; j < len(rows); j++ {
			c := cosine(rows[i], rows[j])
			s[i][j], s[j][i] = c, c
		}
	}
}

// FooteNovelty correlates a Gaussian-tapered checkerboard kernel along the
// diagonal of the self-similarity matrix ssm (Foote 2000). Entry t scores a
// boundary just before row t:
//
//	v(t) = Σ_{a,b=-h}^{h-1} sign(a,b) · exp(-((a+½)² + (b+½)²) / (2σ²)) · ssm[t+a][t+b]
//
// with half-width h = halfWidth, σ = h/2, sign +1 where a and b lie on the
// same side of t and -1 otherwise; entries outside the matrix are skipped.
// Negative scores are clipped to 0 and the curve is scaled to a maximum of 1
// (an all-zero curve stays zero). Near the matrix edges the kernel is
// truncated, so a homogeneous start or end scores up to a quarter of an
// ideal boundary (similarity +1 within and -1 across the sections).
//
// ssm must be square with finite values (else the error wraps [ErrShape] or
// [ErrInvalidArgument]) and halfWidth at least 1 ([DefaultHalfWidth] is 8).
func FooteNovelty(ssm [][]float64, halfWidth int) ([]float64, error) {
	err := checkNovelty(ssm, halfWidth)
	if err != nil {
		return nil, err
	}

	out := make([]float64, len(ssm))
	footeNovelty(out, ssm, halfWidth)

	return out, nil
}

// FooteNoveltyInto writes [FooteNovelty] of ssm into dst, which must have
// len(ssm) values. It does not allocate.
func FooteNoveltyInto(dst []float64, ssm [][]float64, halfWidth int) error {
	err := checkNovelty(ssm, halfWidth)
	if err != nil {
		return err
	}

	if len(dst) != len(ssm) {
		return fmt.Errorf("%w: destination has %d values, want %d", ErrShape, len(dst), len(ssm))
	}

	footeNovelty(dst, ssm, halfWidth)

	return nil
}

func checkNovelty(ssm [][]float64, halfWidth int) error {
	if halfWidth < 1 {
		return fmt.Errorf("%w: half-width must be >= 1, got %d", ErrInvalidArgument, halfWidth)
	}

	err := checkSquare(ssm)
	if err != nil {
		return err
	}

	return checkFinite("ssm", ssm)
}

// footeNovelty computes the novelty curve in the reference operation order.
// The kernel loops are clamped to the matrix instead of skipping
// out-of-range entries; the in-range terms are summed in the same order.
func footeNovelty(out []float64, s [][]float64, halfWidth int) {
	n := len(s)
	sigma := float64(halfWidth) / 2
	top := 0.0

	for t := range n {
		lo, hi := max(-halfWidth, -t), min(halfWidth, n-t)
		v := 0.0

		for a := lo; a < hi; a++ {
			row := s[t+a]

			for b := lo; b < hi; b++ {
				sign := 1.0
				if (a < 0) != (b < 0) {
					sign = -1
				}

				x, y := float64(a)+0.5, float64(b)+0.5
				v += sign * math.Exp(-(x*x+y*y)/(2*sigma*sigma)) * row[t+b]
			}
		}

		out[t] = math.Max(0, v)
		top = math.Max(top, out[t])
	}

	for t := range out {
		if top > 0 {
			out[t] /= top
		}
	}
}
