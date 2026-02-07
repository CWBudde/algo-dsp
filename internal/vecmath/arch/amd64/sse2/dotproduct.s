//go:build !purego && amd64

#include "textflag.h"

// func dotProductSSE2(a, b []float64) float64
TEXT ·dotProductSSE2(SB), NOSPLIT, $16-56
	MOVQ a_base+0(FP), DI
	MOVQ b_base+24(FP), SI
	MOVQ a_len+8(FP), CX

	// Check if we have at least 2 elements for SSE2
	CMPQ CX, $2
	JL   dot_scalar_init

	// Initialize accumulator to zero
	XORPD X0, X0

	// Process 2 elements at a time with SSE2
	CMPQ CX, $2
	JL   dot_reduce

dot_sse2_loop:
	MOVUPD (DI), X1
	MOVUPD (SI), X2
	MULPD  X2, X1
	ADDPD  X1, X0
	ADDQ $16, DI
	ADDQ $16, SI
	SUBQ $2, CX
	CMPQ CX, $2
	JGE  dot_sse2_loop

dot_reduce:
	// Horizontal reduction of X0
	// X0 contains [a, b]
	MOVUPD X0, 0(SP)
	MOVSD 0(SP), X1
	ADDSD 8(SP), X1

	// Handle remaining element if any
	TESTQ CX, CX
	JZ    dot_done_sse2

	MOVSD (DI), X2
	MOVSD (SI), X3
	MULSD X3, X2
	ADDSD X2, X1

dot_done_sse2:
	MOVSD X1, ret+48(FP)
	RET

dot_scalar_init:
	// Initialize scalar accumulator
	XORPD X1, X1

	// Process all elements with scalar code
dot_scalar_loop:
	MOVSD (DI), X2
	MOVSD (SI), X3
	MULSD X3, X2
	ADDSD X2, X1
	ADDQ $8, DI
	ADDQ $8, SI
	DECQ CX
	JNZ  dot_scalar_loop

	MOVSD X1, ret+48(FP)
	RET
