package conv

import (
	"errors"
	"fmt"

	algofft "github.com/cwbudde/algo-fft"
)

// Errors specific to partitioned convolution.
var (
	ErrInvalidBlockOrder    = errors.New("conv: invalid block order")
	ErrEmptyImpulseResponse = errors.New("conv: empty impulse response")
	ErrStageIndexOutOfRange = errors.New("conv: stage index out of range")
)

// PartitionedConvolutionT implements non-uniformly partitioned overlap-add
// convolution (UPOLA) for efficient processing of long impulse responses.
//
// The IR is split into stages with exponentially increasing partition sizes.
// Smaller partitions run more frequently (low latency), larger partitions
// run less frequently (CPU efficiency via modulo scheduling).
//
// Latency = 2^minBlockOrder samples (64–512 for real-time audio).
//
// Based on the algorithm from TLowLatencyConvolution32 (DAV_DspConvolution.pas).
type PartitionedConvolutionT[F algofft.Float, C algofft.Complex] struct {
	// kernel configuration
	kernelLen       int
	kernelLenPadded int

	// latency configuration
	minBlockOrder int
	maxBlockOrder int
	latency       int // = 1 << minBlockOrder

	// ring buffers
	inputBuffer    []F
	outputBuffer   []F
	inputBufSize   int
	inputHistSize  int
	outputHistSize int
	blockPos       int

	stages []*partStageT[F, C]
}

// PartitionedConvolution is the float64 specialization.
type PartitionedConvolution = PartitionedConvolutionT[float64, complex128]

// PartitionedConvolution32 is the float32 specialization.
type PartitionedConvolution32 = PartitionedConvolutionT[float32, complex64]

// partStageT is a single partition stage used internally.
type partStageT[F algofft.Float, C algofft.Complex] struct {
	fftOrder  int
	fftSize   int // = 1 << (fftOrder+1), double the partition size
	partSize  int // = 1 << fftOrder
	outputPos int // offset into output buffer for overlap-add
	latency   int // system latency = 1 << minBlockOrder
	mod       int // current modulo counter
	modAnd    int // (partSize/latency - 1), bitmask for mod

	irSpectra  [][]C
	fft        *fftEngine[C]
	signalBuf  []C // size fftSize, input packing / IFFT scratch
	signalFreq []C // size fftSize, FFT of input
	convolved  []C // size fftSize, for multi-block accumulation
	convTime   []F // size fftSize, IFFT output unpacked
}

// newPartStage creates a new partition stage.
// irOrder is the log2 of the partition size,
// startPos is the offset into the full kernel,
// latency is the system latency in samples,
// count is the number of IR blocks in this stage.
func newPartStage[F algofft.Float, C algofft.Complex](irOrder, startPos, latency, count int) (*partStageT[F, C], error) {
	partSize := 1 << irOrder
	fftSize := 1 << (irOrder + 1) // zero-padded to 2*partSize

	fft, err := newFFTEngine[C](fftSize)
	if err != nil {
		return nil, fmt.Errorf("conv: partitioned stage FFT init (order=%d): %w", irOrder, err)
	}

	irSpectra := make([][]C, count)
	for i := range irSpectra {
		irSpectra[i] = make([]C, fftSize)
	}

	modAnd := partSize/latency - 1

	return &partStageT[F, C]{
		fftOrder:   irOrder,
		fftSize:    fftSize,
		partSize:   partSize,
		outputPos:  startPos,
		latency:    latency,
		mod:        0,
		modAnd:     modAnd,
		irSpectra:  irSpectra,
		fft:        fft,
		signalBuf:  make([]C, fftSize),
		signalFreq: make([]C, fftSize),
		convolved:  make([]C, fftSize),
		convTime:   make([]F, fftSize),
	}, nil
}

// calculateIRSpectra pre-computes the frequency-domain representation of
// each kernel block for this stage. The kernel data for block i starts at
// outputPos + i*partSize in the full kernel slice.
func (s *partStageT[F, C]) calculateIRSpectra(kernel []F) error {
	for blockIdx := range s.irSpectra {
		clear(s.signalBuf)

		kernelStart := s.outputPos + blockIdx*s.partSize
		kernelEnd := min(kernelStart+s.partSize, len(kernel))

		if kernelStart < len(kernel) {
			// IR data is placed in the right half (upper partSize samples) of signalBuf.
			chunk := kernel[kernelStart:kernelEnd]
			packReal(s.signalBuf[s.partSize:s.partSize+len(chunk)], chunk)
		}

		s.fft.Forward(s.irSpectra[blockIdx], s.signalBuf)
	}

	return nil
}

// process runs the stage for one latency block.
// inputBuf is the full input ring buffer; outputBuf is the full output accumulator.
func (s *partStageT[F, C]) process(inputBuf []F, outputBuf []F) {
	if s.mod != 0 {
		s.mod = (s.mod + 1) & s.modAnd
		return
	}

	// Pack last fftSize samples from inputBuf as real-valued complex.
	inputStart := len(inputBuf) - s.fftSize
	clear(s.signalBuf)
	packReal(s.signalBuf, inputBuf[inputStart:inputStart+s.fftSize])

	s.fft.Forward(s.signalFreq, s.signalBuf)

	if len(s.irSpectra) == 1 {
		// Single-block fast path: multiply in-place, IFFT.
		irSpec := s.irSpectra[0]
		for i := range s.signalBuf {
			s.signalBuf[i] = s.signalFreq[i] * irSpec[i]
		}
		s.fft.Inverse(s.signalBuf, s.signalBuf)
		unpackReal(s.convTime, s.signalBuf)

		outPos := s.outputPos + s.latency - s.partSize
		if outPos >= 0 && outPos+s.partSize <= len(outputBuf) {
			for i := range s.partSize {
				outputBuf[outPos+i] += s.convTime[i]
			}
		}
	} else {
		// Multi-block path: IFFT each block individually and overlap-add.
		for blockIdx, irSpec := range s.irSpectra {
			for i := range s.signalBuf {
				s.signalBuf[i] = s.signalFreq[i] * irSpec[i]
			}
			s.fft.Inverse(s.signalBuf, s.signalBuf)
			unpackReal(s.convTime, s.signalBuf)

			outPos := s.outputPos + s.latency - s.partSize + blockIdx*s.partSize
			if outPos >= 0 && outPos+s.partSize <= len(outputBuf) {
				for i := range s.partSize {
					outputBuf[outPos+i] += s.convTime[i]
				}
			}
		}
	}

	s.mod = (s.mod + 1) & s.modAnd
}

// truncLog2 returns floor(log2(n)) for n >= 1.
func truncLog2(n int) int {
	if n <= 0 {
		return 0
	}

	result := 0
	for n > 1 {
		n >>= 1
		result++
	}

	return result
}

// bitCountToBits returns (2 << n) - 1, i.e. the value with all bits set up to bit n.
func bitCountToBits(n int) int {
	return (2 << n) - 1
}

// NewPartitionedConvolutionT creates a non-uniformly partitioned overlap-add
// convolver with the given impulse response kernel.
//
// minBlockOrder controls latency: latency = 2^minBlockOrder samples.
// maxBlockOrder caps the maximum partition size: maxPartSize = 2^maxBlockOrder.
// Typical values: minBlockOrder=6 (64 samples), maxBlockOrder=13 (8192 samples).
func NewPartitionedConvolutionT[F algofft.Float, C algofft.Complex](
	kernel []F, minBlockOrder, maxBlockOrder int,
) (*PartitionedConvolutionT[F, C], error) {
	if len(kernel) == 0 {
		return nil, ErrEmptyImpulseResponse
	}

	if minBlockOrder < 1 {
		return nil, fmt.Errorf("%w: minBlockOrder must be >= 1, got %d", ErrInvalidBlockOrder, minBlockOrder)
	}

	if maxBlockOrder < minBlockOrder {
		return nil, fmt.Errorf("%w: maxBlockOrder (%d) must be >= minBlockOrder (%d)",
			ErrInvalidBlockOrder, maxBlockOrder, minBlockOrder)
	}

	latency := 1 << minBlockOrder
	minBlockSize := latency

	// Pad kernel length to a multiple of minBlockSize.
	kernelLen := len(kernel)
	kernelLenPadded := ((kernelLen + minBlockSize - 1) / minBlockSize) * minBlockSize

	stages, err := partitionIR[F, C](kernel, kernelLenPadded, minBlockOrder, maxBlockOrder, latency)
	if err != nil {
		return nil, err
	}

	// Determine the max IR order actually used for buffer sizing.
	maxIROrd := minBlockOrder
	if len(stages) > 0 {
		last := stages[len(stages)-1]
		maxIROrd = last.fftOrder
	}

	inputBufSize := 2 << maxIROrd
	inputHistSize := inputBufSize - latency
	outputHistSize := max(0, kernelLenPadded-latency)

	return &PartitionedConvolutionT[F, C]{
		kernelLen:       kernelLen,
		kernelLenPadded: kernelLenPadded,
		minBlockOrder:   minBlockOrder,
		maxBlockOrder:   maxBlockOrder,
		latency:         latency,
		inputBuffer:     make([]F, inputBufSize),
		outputBuffer:    make([]F, outputHistSize+latency),
		inputBufSize:    inputBufSize,
		inputHistSize:   inputHistSize,
		outputHistSize:  outputHistSize,
		blockPos:        0,
		stages:          stages,
	}, nil
}

// partitionIR builds the stage list for the given kernel configuration.
func partitionIR[F algofft.Float, C algofft.Complex](
	kernel []F, kernelLenPadded, minBlockOrder, maxBlockOrder, latency int,
) ([]*partStageT[F, C], error) {
	minBlockSize := 1 << minBlockOrder

	// Determine the effective maximum IR order.
	maxIROrd := truncLog2(kernelLenPadded+minBlockSize) - 1

	// Compute residual IR size after the stages below maxIROrd.
	resIRSize := kernelLenPadded - (bitCountToBits(maxIROrd) - bitCountToBits(minBlockOrder-1))

	if resIRSize > 0 && (resIRSize>>maxIROrd)&1 == 0 && maxIROrd > minBlockOrder {
		maxIROrd--
	}

	if maxIROrd > maxBlockOrder {
		maxIROrd = maxBlockOrder
	}

	// Recalculate resIRSize with updated maxIROrd.
	resIRSize = kernelLenPadded - (bitCountToBits(maxIROrd) - bitCountToBits(minBlockOrder-1))

	var stages []*partStageT[F, C]
	startPos := 0

	for order := minBlockOrder; order < maxIROrd; order++ {
		count := 1 + ((resIRSize >> order) & 1)
		stage, err := newPartStage[F, C](order, startPos, latency, count)
		if err != nil {
			return nil, err
		}

		if err := stage.calculateIRSpectra(kernel); err != nil {
			return nil, err
		}

		stages = append(stages, stage)
		startPos += count * (1 << order)
		resIRSize -= (count - 1) * (1 << order)
	}

	// Last (largest) stage.
	count := 1
	if maxIROrd > 0 {
		count = max(1, 1+resIRSize/(1<<maxIROrd))
	}

	stage, err := newPartStage[F, C](maxIROrd, startPos, latency, count)
	if err != nil {
		return nil, err
	}

	if err := stage.calculateIRSpectra(kernel); err != nil {
		return nil, err
	}

	stages = append(stages, stage)

	return stages, nil
}

// NewPartitionedConvolution creates a float64 partitioned convolution.
func NewPartitionedConvolution(kernel []float64, minBlockOrder, maxBlockOrder int) (*PartitionedConvolution, error) {
	return NewPartitionedConvolutionT[float64, complex128](kernel, minBlockOrder, maxBlockOrder)
}

// NewPartitionedConvolution32 creates a float32 partitioned convolution.
func NewPartitionedConvolution32(kernel []float32, minBlockOrder, maxBlockOrder int) (*PartitionedConvolution32, error) {
	return NewPartitionedConvolutionT[float32, complex64](kernel, minBlockOrder, maxBlockOrder)
}

// ProcessBlock convolves an arbitrary-length block of input samples.
// The output slice must be the same length as input. Output is delayed by
// Latency() samples (the first Latency() output samples are from the
// beginning of the previously convolved signal).
func (p *PartitionedConvolutionT[F, C]) ProcessBlock(input, output []F) error {
	if len(input) != len(output) {
		return fmt.Errorf("%w: input length %d != output length %d",
			ErrLengthMismatch, len(input), len(output))
	}

	inPos := 0
	remaining := len(input)
	latency := p.latency

	for remaining > 0 {
		// How many samples can we fill until the next latency-block boundary.
		chunk := min(latency-p.blockPos, remaining)

		// Append chunk to end of input ring buffer.
		ibEnd := len(p.inputBuffer)
		copy(p.inputBuffer[ibEnd-latency+p.blockPos:], input[inPos:inPos+chunk])

		// Read output.
		obStart := p.blockPos
		copy(output[inPos:inPos+chunk], p.outputBuffer[obStart:obStart+chunk])

		p.blockPos += chunk
		inPos += chunk
		remaining -= chunk

		if p.blockPos == latency {
			// Full latency block assembled: run all stages.

			// Shift output buffer left by latency, zero the tail.
			outLen := len(p.outputBuffer)
			copy(p.outputBuffer, p.outputBuffer[latency:])
			clear(p.outputBuffer[outLen-latency:])

			// Run all stages.
			for _, s := range p.stages {
				s.process(p.inputBuffer, p.outputBuffer)
			}

			// Shift input buffer left by latency.
			copy(p.inputBuffer, p.inputBuffer[latency:])
			clear(p.inputBuffer[len(p.inputBuffer)-latency:])

			p.blockPos = 0
		}
	}

	return nil
}

// Reset clears all internal state, ready for a fresh signal stream.
func (p *PartitionedConvolutionT[F, C]) Reset() {
	clear(p.inputBuffer)
	clear(p.outputBuffer)
	p.blockPos = 0

	for _, s := range p.stages {
		s.mod = 0
	}
}

// Latency returns the processing latency in samples (= 2^minBlockOrder).
func (p *PartitionedConvolutionT[F, C]) Latency() int {
	return p.latency
}

// KernelLen returns the original kernel length.
func (p *PartitionedConvolutionT[F, C]) KernelLen() int {
	return p.kernelLen
}

// StageCount returns the number of partition stages.
func (p *PartitionedConvolutionT[F, C]) StageCount() int {
	return len(p.stages)
}

// StageInfo returns information about stage at index.
// partSize is the partition size in samples, blockCount is the number of
// IR blocks in this stage.
func (p *PartitionedConvolutionT[F, C]) StageInfo(index int) (partSize, blockCount int, err error) {
	if index < 0 || index >= len(p.stages) {
		return 0, 0, fmt.Errorf("%w: index %d, have %d stages",
			ErrStageIndexOutOfRange, index, len(p.stages))
	}

	s := p.stages[index]
	return s.partSize, len(s.irSpectra), nil
}
