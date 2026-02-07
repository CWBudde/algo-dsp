//go:build !purego && arm64

#include "textflag.h"

// func addMulBlockNEON(dst, a, b []float64, scale float64)
// Fused add-multiply: dst[i] = (a[i] + b[i]) * scale
// Uses NEON-style paired operations for 2 float64 values
TEXT ·addMulBlockNEON(SB), NOSPLIT, $0-80
	MOVD  dst_base+0(FP), R0
	MOVD  a_base+24(FP), R1
	MOVD  b_base+48(FP), R2
	MOVD  dst_len+8(FP), R3
	FMOVD scale+72(FP), F4    // scale in F4

	CMP $2, R3
	BLT addmul_scalar

	ANDS $1, R3, R5
	LSR $1, R3, R4

addmul_neon_loop:
	FLDPD (R1), (F0, F1)      // Load a[0], a[1]
	FLDPD (R2), (F2, F3)      // Load b[0], b[1]
	FADDD F2, F0, F0          // a[0] + b[0]
	FADDD F3, F1, F1          // a[1] + b[1]
	FMULD F4, F0, F0          // (a[0] + b[0]) * scale
	FMULD F4, F1, F1          // (a[1] + b[1]) * scale
	FSTPD (F0, F1), (R0)

	ADD $16, R1
	ADD $16, R2
	ADD $16, R0
	SUBS $1, R4
	BNE addmul_neon_loop

	CBZ R5, addmul_done

addmul_scalar:
	FMOVD (R1), F0
	FMOVD (R2), F1
	FADDD F1, F0, F0
	FMULD F4, F0, F0
	FMOVD F0, (R0)

	ADD $8, R1
	ADD $8, R2
	ADD $8, R0
	SUBS $1, R5
	BNE addmul_scalar

addmul_done:
	RET

// func mulAddBlockNEON(dst, a, b, c []float64)
// Fused multiply-add: dst[i] = a[i] * b[i] + c[i]
TEXT ·mulAddBlockNEON(SB), NOSPLIT, $0-96
	MOVD dst_base+0(FP), R0
	MOVD a_base+24(FP), R1
	MOVD b_base+48(FP), R2
	MOVD c_base+72(FP), R8
	MOVD dst_len+8(FP), R3

	CMP $2, R3
	BLT muladd_scalar

	ANDS $1, R3, R5
	LSR $1, R3, R4

muladd_neon_loop:
	FLDPD (R1), (F0, F1)      // Load a[0], a[1]
	FLDPD (R2), (F2, F3)      // Load b[0], b[1]
	FLDPD (R8), (F4, F5)      // Load c[0], c[1]
	FMULD F2, F0, F0          // a[0] * b[0]
	FMULD F3, F1, F1          // a[1] * b[1]
	FADDD F4, F0, F0          // a[0] * b[0] + c[0]
	FADDD F5, F1, F1          // a[1] * b[1] + c[1]
	FSTPD (F0, F1), (R0)

	ADD $16, R1
	ADD $16, R2
	ADD $16, R8
	ADD $16, R0
	SUBS $1, R4
	BNE muladd_neon_loop

	CBZ R5, muladd_done

muladd_scalar:
	FMOVD (R1), F0
	FMOVD (R2), F1
	FMOVD (R8), F2
	FMULD F1, F0, F0
	FADDD F2, F0, F0
	FMOVD F0, (R0)

	ADD $8, R1
	ADD $8, R2
	ADD $8, R8
	ADD $8, R0
	SUBS $1, R5
	BNE muladd_scalar

muladd_done:
	RET
