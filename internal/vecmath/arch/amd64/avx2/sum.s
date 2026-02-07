//go:build !purego && amd64

#include "textflag.h"

// func sumAVX2(x []float64) float64
TEXT ·sumAVX2(SB), NOSPLIT, $32-32
	MOVQ x_base+0(FP), DI
	MOVQ x_len+8(FP), CX

	// Check if we have at least 4 elements for AVX2
	CMPQ CX, $4
	JL   sum_scalar_init

	// Initialize accumulator to zero
	VXORPD Y0, Y0, Y0

	// Process 4 elements at a time with AVX2
	CMPQ CX, $4
	JL   sum_reduce

sum_avx2_loop:
	VADDPD (DI), Y0, Y0
	ADDQ $32, DI
	SUBQ $4, CX
	CMPQ CX, $4
	JGE  sum_avx2_loop

sum_reduce:
	// Horizontal reduction of Y0
	// Y0 contains [a, b, c, d]
	VMOVUPD Y0, 0(SP)
	MOVSD 0(SP), X1
	ADDSD 8(SP), X1
	ADDSD 16(SP), X1
	ADDSD 24(SP), X1

	// Handle remaining elements (scalar tail)
	TESTQ CX, CX
	JZ    sum_done_avx2

sum_scalar_tail:
	MOVSD (DI), X2
	ADDSD X2, X1
	ADDQ $8, DI
	DECQ CX
	JNZ  sum_scalar_tail

sum_done_avx2:
	MOVSD X1, ret+24(FP)
	VZEROUPPER
	RET

sum_scalar_init:
	// Initialize scalar accumulator
	VXORPD X1, X1, X1

	// Process all elements with scalar code
sum_scalar_loop:
	MOVSD (DI), X2
	ADDSD X2, X1
	ADDQ $8, DI
	DECQ CX
	JNZ  sum_scalar_loop

	MOVSD X1, ret+24(FP)
	RET
