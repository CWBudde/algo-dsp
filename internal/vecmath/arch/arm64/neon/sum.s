//go:build !purego && arm64

#include "textflag.h"

// func sumNEON(x []float64) float64
TEXT ·sumNEON(SB), NOSPLIT, $0-32
	MOVD x_base+0(FP), R0
	MOVD x_len+8(FP), R1

	// Check if we have at least 2 elements for NEON
	CMP $2, R1
	BLT scalar_init

	// Initialize accumulator to zero
	VEOR V0.B16, V0.B16, V0.B16

	// Process 2 elements at a time with NEON
	CMP $2, R1
	BLT reduce

vec_loop:
	VLD1.P 16(R0), [V1.D2]
	VFADDD V1.D2, V0.D2, V0.D2
	SUB $2, R1
	CMP $2, R1
	BGE vec_loop

reduce:
	// Horizontal reduction: sum V0.D[0] + V0.D[1]
	FMOVD V0.D[0], F2
	FMOVD V0.D[1], F3
	FADDD F3, F2, F2

	// Handle remaining element if any
	CBZ R1, done

	FMOVD (R0), F3
	FADDD F3, F2, F2

done:
	FMOVD F2, ret+24(FP)
	RET

scalar_init:
	// Initialize scalar accumulator
	FMOVD $0, F2

	// Process all elements with scalar code
scalar_loop:
	FMOVD (R0), F3
	FADDD F3, F2, F2
	ADD $8, R0
	SUB $1, R1
	CBNZ R1, scalar_loop

	FMOVD F2, ret+24(FP)
	RET
