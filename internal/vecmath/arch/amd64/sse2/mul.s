//go:build !purego && amd64

#include "textflag.h"

// func mulBlockSSE2(dst, a, b []float64)
// Element-wise multiply: dst[i] = a[i] * b[i]
// Uses SSE2 to process 2 float64 values at once
TEXT ·mulBlockSSE2(SB), NOSPLIT, $0-72
	// Load slice headers
	MOVQ dst_base+0(FP), DI   // dst.data
	MOVQ a_base+24(FP), SI    // a.data
	MOVQ b_base+48(FP), DX    // b.data
	MOVQ dst_len+8(FP), CX    // len(dst)

	// Check if we have at least 2 elements for SSE2
	CMPQ CX, $2
	JL   mulblock_scalar

	// Calculate number of SSE2 iterations (2 elements per iter)
	MOVQ CX, AX
	SHRQ $1, AX              // AX = len / 2
	ANDQ $1, CX              // CX = len % 2 (remainder)

mulblock_sse2_loop:
	MOVUPD (SI), X0          // Load 2 float64 from a
	MOVUPD (DX), X1          // Load 2 float64 from b
	MULPD  X1, X0            // X0 = a * b
	MOVUPD X0, (DI)          // Store to dst

	ADDQ $16, SI             // Advance pointers (2 * 8 bytes)
	ADDQ $16, DX
	ADDQ $16, DI
	DECQ AX
	JNZ  mulblock_sse2_loop

	// Handle remainder with scalar loop
	TESTQ CX, CX
	JZ    mulblock_done

mulblock_scalar:
	MOVSD  (SI), X0          // Load 1 float64 from a
	MULSD  (DX), X0          // Multiply with b
	MOVSD  X0, (DI)          // Store to dst

	ADDQ $8, SI
	ADDQ $8, DX
	ADDQ $8, DI
	DECQ CX
	JNZ  mulblock_scalar

mulblock_done:
	RET

// func mulBlockInPlaceSSE2(dst, src []float64)
// In-place element-wise multiply: dst[i] *= src[i]
TEXT ·mulBlockInPlaceSSE2(SB), NOSPLIT, $0-48
	MOVQ dst_base+0(FP), DI   // dst.data
	MOVQ src_base+24(FP), SI  // src.data
	MOVQ dst_len+8(FP), CX    // len(dst)

	CMPQ CX, $2
	JL   mulinplace_scalar

	MOVQ CX, AX
	SHRQ $1, AX
	ANDQ $1, CX

mulinplace_sse2_loop:
	MOVUPD (DI), X0          // Load 2 float64 from dst
	MOVUPD (SI), X1          // Load 2 float64 from src
	MULPD  X1, X0            // X0 = dst * src
	MOVUPD X0, (DI)          // Store back to dst

	ADDQ $16, SI
	ADDQ $16, DI
	DECQ AX
	JNZ  mulinplace_sse2_loop

	TESTQ CX, CX
	JZ    mulinplace_done

mulinplace_scalar:
	MOVSD  (DI), X0
	MULSD  (SI), X0
	MOVSD  X0, (DI)

	ADDQ $8, SI
	ADDQ $8, DI
	DECQ CX
	JNZ  mulinplace_scalar

mulinplace_done:
	RET
