//go:build !purego && arm64

#include "textflag.h"

// Absolute-value mask for float64 lanes: clear sign bit.
DATA ·absMask<>(SB)/8, $0x7fffffffffffffff
DATA ·absMask<>+8(SB)/8, $0x7fffffffffffffff
GLOBL ·absMask<>(SB), RODATA, $16

// func maxAbsNEON(x []float64) float64
TEXT ·maxAbsNEON(SB), NOSPLIT, $0-32
	MOVD x_base+0(FP), R0
	MOVD x_len+8(FP), R1

	CMP $2, R1
	BLT scalar_init

	MOVD $·absMask<>(SB), R8
	VLD1 (R8), [V31.D2]

	// Seed max from first two elements.
	VLD1.P 16(R0), [V0.D2]
	VAND V31.B16, V0.B16, V0.B16
	VMOV V0.D[0], R3
	VMOV V0.D[1], R4
	CMP R4, R3
	CSEL CS, R4, R3, R3

	ANDS $1, R1, R5 // tail = len % 2
	LSR $1, R1, R2  // pairs = len / 2
	SUB $1, R2
	CBZ R2, tail

vec_loop:
	VLD1.P 16(R0), [V0.D2]
	VAND V31.B16, V0.B16, V0.B16

	VMOV V0.D[0], R4
	CMP R4, R3
	CSEL CS, R4, R3, R3

	VMOV V0.D[1], R4
	CMP R4, R3
	CSEL CS, R4, R3, R3

	SUB $1, R2
	CBNZ R2, vec_loop

 tail:
	CBZ R5, done_bits

	MOVD (R0), R4
	MOVD $0x7fffffffffffffff, R9
	AND R9, R4, R4
	CMP R4, R3
	CSEL CS, R4, R3, R3

 done_bits:
	MOVD R3, ret+24(FP)
	RET

scalar_init:
	FMOVD (R0), F0
	FABSD F0, F0
	ADD $8, R0
	SUB $1, R1
	CBZ R1, done_fp

scalar_loop:
	FMOVD (R0), F1
	FABSD F1, F1
	FMAXD F1, F0, F0
	ADD $8, R0
	SUB $1, R1
	CBNZ R1, scalar_loop

 done_fp:
	FMOVD F0, ret+24(FP)
	RET
