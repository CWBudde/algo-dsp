//go:build !purego && arm64

#include "textflag.h"

// func powerNEON(dst, re, im []float64)
// Computes power (magnitude squared): dst[i] = re[i]^2 + im[i]^2
// Uses NEON to process 2 float64 values at once
TEXT ·powerNEON(SB), NOSPLIT, $0-72
	MOVD dst_base+0(FP), R0    // dst.data
	MOVD re_base+24(FP), R1    // re.data
	MOVD im_base+48(FP), R2    // im.data
	MOVD dst_len+8(FP), R3     // len(dst)

	CMP  $2, R3
	BLT  power_scalar

	MOVD R3, R4
	LSR  $1, R4                // R4 = count / 2 (number of NEON iterations)
	AND  $1, R3, R3            // R3 = count % 2 (remainder for scalar)

power_neon_loop:
	VLD1 (R1), [V0.D2]         // Load 2 float64 from re
	VLD1 (R2), [V1.D2]         // Load 2 float64 from im
	FMUL V0.D2, V0.D2, V0.D2   // V0 = re^2
	FMUL V1.D2, V1.D2, V1.D2   // V1 = im^2
	FADD V1.D2, V0.D2, V0.D2   // V0 = re^2 + im^2
	VST1 [V0.D2], (R0)         // Store to dst

	ADD  $16, R1
	ADD  $16, R2
	ADD  $16, R0
	SUBS $1, R4
	BNE  power_neon_loop

	CMP  $0, R3
	BEQ  power_done

power_scalar:
	FMOVD (R1), F0             // F0 = re[i]
	FMOVD (R2), F1             // F1 = im[i]
	FMULD F0, F0, F0           // F0 = re^2
	FMULD F1, F1, F1           // F1 = im^2
	FADDD F1, F0, F0           // F0 = re^2 + im^2
	FMOVD F0, (R0)             // Store to dst

	ADD  $8, R1
	ADD  $8, R2
	ADD  $8, R0
	SUBS $1, R3
	BNE  power_scalar

power_done:
	RET
