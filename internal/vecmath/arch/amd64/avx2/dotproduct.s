//go:build !purego && amd64

#include "textflag.h"

// func dotProductAVX2(a, b []float64) float64
TEXT ·dotProductAVX2(SB), NOSPLIT, $32-56
	MOVQ a_base+0(FP), DI
	MOVQ b_base+24(FP), SI
	MOVQ a_len+8(FP), CX

	// Check if we have at least 4 elements for AVX2
	CMPQ CX, $4
	JL   dot_scalar_init

	// Initialize accumulator to zero
	VXORPD Y0, Y0, Y0

	// Process 4 elements at a time with AVX2
	CMPQ CX, $4
	JL   dot_reduce

dot_avx2_loop:
	VMOVUPD (DI), Y1
	VMOVUPD (SI), Y2
	VMULPD  Y2, Y1, Y1
	VADDPD  Y1, Y0, Y0
	ADDQ $32, DI
	ADDQ $32, SI
	SUBQ $4, CX
	CMPQ CX, $4
	JGE  dot_avx2_loop

dot_reduce:
	// Horizontal reduction of Y0
	// Y0 contains [a, b, c, d]
	VMOVUPD Y0, 0(SP)
	MOVSD 0(SP), X1
	ADDSD 8(SP), X1
	ADDSD 16(SP), X1
	ADDSD 24(SP), X1

	// Handle remaining elements (scalar tail)
	TESTQ CX, CX
	JZ    dot_done_avx2

dot_scalar_tail:
	MOVSD (DI), X2
	MOVSD (SI), X3
	MULSD X3, X2
	ADDSD X2, X1
	ADDQ $8, DI
	ADDQ $8, SI
	DECQ CX
	JNZ  dot_scalar_tail

dot_done_avx2:
	MOVSD X1, ret+48(FP)
	VZEROUPPER
	RET

dot_scalar_init:
	// Initialize scalar accumulator
	VXORPD X1, X1, X1

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
