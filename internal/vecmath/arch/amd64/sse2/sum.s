//go:build !purego && amd64

#include "textflag.h"

// func sumSSE2(x []float64) float64
TEXT ·sumSSE2(SB), NOSPLIT, $16-32
	MOVQ x_base+0(FP), DI
	MOVQ x_len+8(FP), CX

	// Check if we have at least 2 elements for SSE2
	CMPQ CX, $2
	JL   sum_scalar_init

	// Initialize accumulator to zero
	XORPD X0, X0

	// Process 2 elements at a time with SSE2
	CMPQ CX, $2
	JL   sum_reduce

sum_sse2_loop:
	ADDPD (DI), X0
	ADDQ $16, DI
	SUBQ $2, CX
	CMPQ CX, $2
	JGE  sum_sse2_loop

sum_reduce:
	// Horizontal reduction of X0
	// X0 contains [a, b]
	MOVUPD X0, 0(SP)
	MOVSD 0(SP), X1
	ADDSD 8(SP), X1

	// Handle remaining element if any
	TESTQ CX, CX
	JZ    sum_done_sse2

	MOVSD (DI), X2
	ADDSD X2, X1

sum_done_sse2:
	MOVSD X1, ret+24(FP)
	RET

sum_scalar_init:
	// Initialize scalar accumulator
	XORPD X1, X1

	// Process all elements with scalar code
sum_scalar_loop:
	MOVSD (DI), X2
	ADDSD X2, X1
	ADDQ $8, DI
	DECQ CX
	JNZ  sum_scalar_loop

	MOVSD X1, ret+24(FP)
	RET
