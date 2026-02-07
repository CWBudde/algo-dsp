//go:build !purego && arm64

#include "textflag.h"

// func mulBlockNEON(dst, a, b []float64)
// Element-wise multiply: dst[i] = a[i] * b[i]
// Uses NEON-style paired operations for 2 float64 values
TEXT ·mulBlockNEON(SB), NOSPLIT, $0-72
	MOVD dst_base+0(FP), R0   // dst.data
	MOVD a_base+24(FP), R1    // a.data
	MOVD b_base+48(FP), R2    // b.data
	MOVD dst_len+8(FP), R3    // len(dst)

	CMP $2, R3
	BLT mulblock_scalar

	ANDS $1, R3, R5
	LSR $1, R3, R4

mulblock_neon_loop:
	FLDPD (R1), (F0, F1)
	FLDPD (R2), (F2, F3)
	FMULD F2, F0, F0
	FMULD F3, F1, F1
	FSTPD (F0, F1), (R0)

	ADD $16, R1
	ADD $16, R2
	ADD $16, R0
	SUBS $1, R4
	BNE mulblock_neon_loop

	CBZ R5, mulblock_done

mulblock_scalar:
	FMOVD (R1), F0
	FMOVD (R2), F1
	FMULD F1, F0, F0
	FMOVD F0, (R0)

	ADD $8, R1
	ADD $8, R2
	ADD $8, R0
	SUBS $1, R5
	BNE mulblock_scalar

mulblock_done:
	RET

// func mulBlockInPlaceNEON(dst, src []float64)
// In-place multiply: dst[i] *= src[i]
TEXT ·mulBlockInPlaceNEON(SB), NOSPLIT, $0-48
	MOVD dst_base+0(FP), R0
	MOVD src_base+24(FP), R1
	MOVD dst_len+8(FP), R3

	CMP $2, R3
	BLT mulinplace_scalar

	ANDS $1, R3, R5
	LSR $1, R3, R4

mulinplace_neon_loop:
	FLDPD (R0), (F0, F1)
	FLDPD (R1), (F2, F3)
	FMULD F2, F0, F0
	FMULD F3, F1, F1
	FSTPD (F0, F1), (R0)

	ADD $16, R0
	ADD $16, R1
	SUBS $1, R4
	BNE mulinplace_neon_loop

	CBZ R5, mulinplace_done

mulinplace_scalar:
	FMOVD (R0), F0
	FMOVD (R1), F1
	FMULD F1, F0, F0
	FMOVD F0, (R0)

	ADD $8, R0
	ADD $8, R1
	SUBS $1, R5
	BNE mulinplace_scalar

mulinplace_done:
	RET
