//go:build !purego && amd64

#include "textflag.h"

// func addBlockSSE2(dst, a, b []float64)
// Element-wise add: dst[i] = a[i] + b[i]
// Uses SSE2 to process 2 float64 values at once
TEXT ·addBlockSSE2(SB), NOSPLIT, $0-72
	MOVQ dst_base+0(FP), DI    // dst.data
	MOVQ a_base+24(FP), SI     // a.data
	MOVQ b_base+48(FP), DX     // b.data
	MOVQ dst_len+8(FP), CX     // len(dst)

	CMPQ CX, $2
	JL   addblock_scalar

	MOVQ CX, AX
	SHRQ $1, AX                // AX = len / 2 (pairs of float64)
	ANDQ $1, CX                // CX = len % 2 (remainder)

addblock_sse2_loop:
	MOVUPD (SI), X0            // Load 2 float64 from a
	MOVUPD (DX), X1            // Load 2 float64 from b
	ADDPD  X1, X0              // X0 = a + b
	MOVUPD X0, (DI)            // Store to dst

	ADDQ $16, SI
	ADDQ $16, DX
	ADDQ $16, DI
	DECQ AX
	JNZ  addblock_sse2_loop

	TESTQ CX, CX
	JZ    addblock_done

addblock_scalar:
	MOVSD  (SI), X0
	ADDSD  (DX), X0
	MOVSD  X0, (DI)

	ADDQ $8, SI
	ADDQ $8, DX
	ADDQ $8, DI
	DECQ CX
	JNZ  addblock_scalar

addblock_done:
	RET

// func addBlockInPlaceSSE2(dst, src []float64)
// In-place add: dst[i] += src[i]
TEXT ·addBlockInPlaceSSE2(SB), NOSPLIT, $0-48
	MOVQ dst_base+0(FP), DI    // dst.data
	MOVQ src_base+24(FP), SI   // src.data
	MOVQ dst_len+8(FP), CX     // len(dst)

	CMPQ CX, $2
	JL   addinplace_scalar

	MOVQ CX, AX
	SHRQ $1, AX                // AX = len / 2 (pairs of float64)
	ANDQ $1, CX                // CX = len % 2 (remainder)

addinplace_sse2_loop:
	MOVUPD (DI), X0            // Load 2 float64 from dst
	MOVUPD (SI), X1            // Load 2 float64 from src
	ADDPD  X1, X0              // X0 = dst + src
	MOVUPD X0, (DI)            // Store back to dst

	ADDQ $16, SI
	ADDQ $16, DI
	DECQ AX
	JNZ  addinplace_sse2_loop

	TESTQ CX, CX
	JZ    addinplace_done

addinplace_scalar:
	MOVSD  (DI), X0
	ADDSD  (SI), X0
	MOVSD  X0, (DI)

	ADDQ $8, SI
	ADDQ $8, DI
	DECQ CX
	JNZ  addinplace_scalar

addinplace_done:
	RET
