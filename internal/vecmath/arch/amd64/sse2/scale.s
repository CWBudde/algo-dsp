//go:build !purego && amd64

#include "textflag.h"

// func scaleBlockSSE2(dst, src []float64, scale float64)
// Scale: dst[i] = src[i] * scale
// Uses SSE2 to process 2 float64 values at once
TEXT ·scaleBlockSSE2(SB), NOSPLIT, $0-56
	MOVQ  dst_base+0(FP), DI  // dst.data
	MOVQ  src_base+24(FP), SI // src.data
	MOVQ  dst_len+8(FP), CX   // len(dst)
	MOVSD scale+48(FP), X1    // scale value

	// Broadcast scale to both lanes of X1
	UNPCKLPD X1, X1           // X1[0] = X1[1] = scale

	CMPQ CX, $2
	JL   scaleblock_scalar

	MOVQ CX, AX
	SHRQ $1, AX
	ANDQ $1, CX

scaleblock_sse2_loop:
	MOVUPD (SI), X0          // Load 2 float64 from src
	MULPD  X1, X0            // X0 = src * scale
	MOVUPD X0, (DI)          // Store to dst

	ADDQ $16, SI
	ADDQ $16, DI
	DECQ AX
	JNZ  scaleblock_sse2_loop

	TESTQ CX, CX
	JZ    scaleblock_done

scaleblock_scalar:
	MOVSD  (SI), X0
	MULSD  X1, X0
	MOVSD  X0, (DI)

	ADDQ $8, SI
	ADDQ $8, DI
	DECQ CX
	JNZ  scaleblock_scalar

scaleblock_done:
	RET

// func scaleBlockInPlaceSSE2(dst []float64, scale float64)
// In-place scale: dst[i] *= scale
TEXT ·scaleBlockInPlaceSSE2(SB), NOSPLIT, $0-32
	MOVQ  dst_base+0(FP), DI  // dst.data
	MOVQ  dst_len+8(FP), CX   // len(dst)
	MOVSD scale+24(FP), X1    // scale value

	// Broadcast scale to both lanes of X1
	UNPCKLPD X1, X1

	CMPQ CX, $2
	JL   scaleinplace_scalar

	MOVQ CX, AX
	SHRQ $1, AX
	ANDQ $1, CX

scaleinplace_sse2_loop:
	MOVUPD (DI), X0
	MULPD  X1, X0
	MOVUPD X0, (DI)

	ADDQ $16, DI
	DECQ AX
	JNZ  scaleinplace_sse2_loop

	TESTQ CX, CX
	JZ    scaleinplace_done

scaleinplace_scalar:
	MOVSD  (DI), X0
	MULSD  X1, X0
	MOVSD  X0, (DI)

	ADDQ $8, DI
	DECQ CX
	JNZ  scaleinplace_scalar

scaleinplace_done:
	RET
