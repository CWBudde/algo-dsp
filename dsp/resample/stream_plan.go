package resample

import (
	"errors"
	"math"
	"math/bits"
)

// ErrInvalidStream indicates an invalid block size, duration, or stream state.
var ErrInvalidStream = errors.New("resample: invalid stream configuration or state")

// FrameCount returns ceil(frames*outRate/inRate) without intermediate overflow.
// Invalid rates return ErrInvalidRate; negative frames return ErrInvalidStream.
// A count that cannot fit int64 returns ErrOutputTooLarge.
func FrameCount(frames int64, inRate, outRate int) (int64, error) {
	return convertFrame(frames, inRate, outRate, true)
}

// FramePosition maps a source coordinate to the nearest output frame, rounding
// half frames upwards. It has the same validation and overflow rules as FrameCount.
func FramePosition(frame int64, inRate, outRate int) (int64, error) {
	return convertFrame(frame, inRate, outRate, false)
}

func convertFrame(frame int64, inRate, outRate int, ceil bool) (int64, error) {
	if inRate <= 0 || outRate <= 0 {
		return 0, ErrInvalidRate
	}

	if frame < 0 {
		return 0, ErrInvalidStream
	}

	high, low := bits.Mul64(uint64(frame), uint64(outRate))

	if !ceil {
		var carry uint64

		low, carry = bits.Add64(low, uint64(inRate)/2, 0)
		high += carry
	}

	if high >= uint64(inRate) {
		return 0, ErrOutputTooLarge
	}

	quotient, remainder := bits.Div64(high, low, uint64(inRate))
	if quotient > math.MaxInt64 || (ceil && remainder != 0 && quotient == math.MaxInt64) {
		return 0, ErrOutputTooLarge
	}

	if ceil && remainder != 0 {
		quotient++
	}

	return int64(quotient), nil
}

// StreamPlan describes an exact integer-rate, delay-compensated stream without
// allocating coefficients. A host can check WorkspaceBytes before NewStream.
// Downsampling increases the quality profile's branch length to preserve the
// anti-aliasing transition width. Equal rates use an unfiltered identity copy.
// The zero value is invalid.
type StreamPlan struct {
	inRate, outRate, up, down     int
	inputBlock, outputBlock, taps int
	quality                       Quality
}

// NewStreamPlan preflights filter sizes and block capacities without allocating.
// inputBlockFrames must be positive; quality must be one of the predefined modes.
func NewStreamPlan(inRate, outRate, inputBlockFrames int, quality Quality) (StreamPlan, error) {
	if inRate <= 0 || outRate <= 0 {
		return StreamPlan{}, ErrInvalidRate
	}

	if inputBlockFrames <= 0 || quality < QualityFast || quality > QualityBest {
		return StreamPlan{}, ErrInvalidStream
	}

	g := gcd(inRate, outRate)
	p := StreamPlan{inRate: inRate, outRate: outRate, up: outRate / g, down: inRate / g, inputBlock: inputBlockFrames, quality: quality}
	maxInt := int(^uint(0) >> 1)

	count, err := FrameCount(int64(inputBlockFrames), inRate, outRate)
	if err != nil || count >= int64(maxInt) {
		return StreamPlan{}, ErrOutputTooLarge
	}

	p.outputBlock = int(count) + 1
	if p.up == p.down {
		p.outputBlock = inputBlockFrames
		return p, nil
	}

	factor := p.down / p.up
	if p.down%p.up != 0 {
		factor++
	}

	base := QualityProfile(quality).TapsPerPhase
	if factor > maxInt/base {
		return StreamPlan{}, ErrOutputTooLarge
	}

	p.taps = base * factor
	if p.up > maxInt/p.taps {
		return StreamPlan{}, ErrOutputTooLarge
	}

	if _, err := p.WorkspaceBytes(1); err != nil {
		return StreamPlan{}, err
	}

	return p, nil
}

// Ratio returns the reduced exact output/input ratio.
func (p StreamPlan) Ratio() (up, down int) { return p.up, p.down }

// InputBlockFrames returns the maximum source frames accepted per call.
func (p StreamPlan) InputBlockFrames() int { return p.inputBlock }

// OutputBlockFrames returns a capacity sufficient for every ProcessInto and
// FlushInto block, including any rational phase and initial delay removal.
func (p StreamPlan) OutputBlockFrames() int { return p.outputBlock }

// InputFramesForOutputLimit returns a conservative input block size within the
// plan's current limit whose output fits outputLimit at every rational phase.
// Rebuild the plan with this smaller size to reduce its scratch reservation.
func (p StreamPlan) InputFramesForOutputLimit(outputLimit int) (int, error) {
	if p.inputBlock <= 0 || outputLimit <= 0 {
		return 0, ErrInvalidStream
	}

	if p.up == p.down {
		return min(p.inputBlock, outputLimit), nil
	}

	if outputLimit == 1 {
		return 0, ErrShortDst
	}

	high, low := bits.Mul64(uint64(outputLimit-1), uint64(p.down))
	if high >= uint64(p.up) {
		return p.inputBlock, nil
	}

	q, _ := bits.Div64(high, low, uint64(p.up))
	if q > uint64(p.inputBlock) {
		q = uint64(p.inputBlock)
	}

	n := int64(q)
	if n == 0 {
		return 0, ErrShortDst
	}

	return int(n), nil
}

// WorkspaceBytes conservatively estimates shared prototype/polyphase storage,
// phase headers, a fixed 512-byte object allowance per channel, independent
// channel histories, and each stream's internal raw
// output and zero-input scratch. Caller-owned input/output slices and source or
// destination sample storage are excluded. Clone shares filter coefficients.
// No storage is allocated, and arithmetic overflow returns ErrOutputTooLarge.
func (p StreamPlan) WorkspaceBytes(channels int) (int64, error) {
	if p.inputBlock <= 0 || channels <= 0 {
		return 0, ErrInvalidStream
	}

	if uint64(channels) > math.MaxInt64/512 {
		return 0, ErrOutputTooLarge
	}

	if p.up == p.down {
		return int64(channels) * 512, nil
	}

	total := uint64(p.taps) * uint64(p.up)
	if total > math.MaxInt64/16 {
		return 0, ErrOutputTooLarge
	}

	total *= 16

	if uint64(p.taps) > math.MaxInt64/8 || uint64(p.outputBlock) > math.MaxInt64/8 || uint64(p.inputBlock) > math.MaxInt64/8 {
		return 0, ErrOutputTooLarge
	}

	terms := [][2]uint64{{uint64(channels), 512}, {uint64(p.up), 32}, {uint64(channels), uint64(p.taps) * 8}, {uint64(channels), uint64(p.outputBlock) * 8}, {uint64(channels), uint64(p.inputBlock) * 8}}
	for _, term := range terms {
		if term[1] > math.MaxInt64 || term[0] > (math.MaxInt64-total)/term[1] {
			return 0, ErrOutputTooLarge
		}

		total += term[0] * term[1]
	}

	return int64(total), nil
}
