//go:build !purego && arm64

#include "textflag.h"

// func dotProductNEON(a, b []float64) float64
TEXT ·dotProductNEON(SB), NOSPLIT, $0-56
	MOVD a_base+0(FP), R0
	MOVD b_base+24(FP), R1
	MOVD a_len+8(FP), R2

	// Check if we have at least 2 elements for NEON
	CMP $2, R2
	BLT scalar_init

	// Initialize accumulator to zero
	VEOR V0.B16, V0.B16, V0.B16

	// Process 2 elements at a time with NEON
	CMP $2, R2
	BLT reduce

vec_loop:
	VLD1.P 16(R0), [V1.D2]
	VLD1.P 16(R1), [V2.D2]
	VFMULD V2.D2, V1.D2, V1.D2
	VFADDD V1.D2, V0.D2, V0.D2
	SUB $2, R2
	CMP $2, R2
	BGE vec_loop

reduce:
	// Horizontal reduction: sum V0.D[0] + V0.D[1]
	FMOVD V0.D[0], F3
	FMOVD V0.D[1], F4
	FADDD F4, F3, F3

	// Handle remaining element if any
	CBZ R2, done

	FMOVD (R0), F4
	FMOVD (R1), F5
	FMULD F5, F4, F4
	FADDD F4, F3, F3

done:
	FMOVD F3, ret+48(FP)
	RET

scalar_init:
	// Initialize scalar accumulator
	FMOVD $0, F3

	// Process all elements with scalar code
scalar_loop:
	FMOVD (R0), F4
	FMOVD (R1), F5
	FMULD F5, F4, F4
	FADDD F4, F3, F3
	ADD $8, R0
	ADD $8, R1
	SUB $1, R2
	CBNZ R2, scalar_loop

	FMOVD F3, ret+48(FP)
	RET
