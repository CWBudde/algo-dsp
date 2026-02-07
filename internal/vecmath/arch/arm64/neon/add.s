//go:build !purego && arm64

#include "textflag.h"

// func addBlockNEON(dst, a, b []float64)
// Element-wise add: dst[i] = a[i] + b[i]
// Uses NEON to process 2 float64 values at once
TEXT ·addBlockNEON(SB), NOSPLIT, $0-72
	MOVD dst_base+0(FP), R0   // dst.data
	MOVD a_base+24(FP), R1    // a.data
	MOVD b_base+48(FP), R2    // b.data
	MOVD dst_len+8(FP), R3    // len(dst)

	CMP $2, R3
	BLT addblock_scalar

	// R4 = len / 2 (pairs)
	// R5 = len % 2 (remainder)
	ANDS $1, R3, R5
	LSR $1, R3, R4

addblock_neon_loop:
	VLD1.P 16(R1), [V0.D2]    // Load 2 float64 from a
	VLD1.P 16(R2), [V1.D2]    // Load 2 float64 from b
	FADD V1.D2, V0.D2, V0.D2  // V0 = a + b
	VST1.P [V0.D2], 16(R0)    // Store to dst

	SUBS $1, R4
	BNE addblock_neon_loop

	CBZ R5, addblock_done

addblock_scalar:
	FMOVD (R1), F0            // Load from a
	FMOVD (R2), F1            // Load from b
	FADDD F1, F0, F0          // F0 = a + b
	FMOVD F0, (R0)            // Store to dst

	ADD $8, R1
	ADD $8, R2
	ADD $8, R0
	SUBS $1, R5
	BNE addblock_scalar

addblock_done:
	RET

// func addBlockInPlaceNEON(dst, src []float64)
// In-place add: dst[i] += src[i]
TEXT ·addBlockInPlaceNEON(SB), NOSPLIT, $0-48
	MOVD dst_base+0(FP), R0   // dst.data
	MOVD src_base+24(FP), R1  // src.data
	MOVD dst_len+8(FP), R3    // len(dst)

	CMP $2, R3
	BLT addinplace_scalar

	ANDS $1, R3, R5
	LSR $1, R3, R4

addinplace_neon_loop:
	VLD1.P 16(R0), [V0.D2]    // Load 2 float64 from dst
	VLD1.P 16(R1), [V1.D2]    // Load 2 float64 from src
	FADD V1.D2, V0.D2, V0.D2  // V0 = dst + src

	// Store back (need to rewind R0 since we used post-increment)
	SUB $16, R0
	VST1.P [V0.D2], 16(R0)    // Store back to dst

	SUBS $1, R4
	BNE addinplace_neon_loop

	CBZ R5, addinplace_done

addinplace_scalar:
	FMOVD (R0), F0            // Load from dst
	FMOVD (R1), F1            // Load from src
	FADDD F1, F0, F0          // F0 = dst + src
	FMOVD F0, (R0)            // Store back to dst

	ADD $8, R0
	ADD $8, R1
	SUBS $1, R5
	BNE addinplace_scalar

addinplace_done:
	RET
