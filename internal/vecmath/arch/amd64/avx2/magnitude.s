//go:build !purego && amd64

#include "textflag.h"

// func magnitudeAVX2(dst, re, im []float64)
// Computes magnitude: dst[i] = sqrt(re[i]^2 + im[i]^2)
// Uses AVX2 to process 4 float64 values at once
TEXT ·magnitudeAVX2(SB), NOSPLIT, $0-72
	MOVQ dst_base+0(FP), DI    // dst.data
	MOVQ re_base+24(FP), SI    // re.data
	MOVQ im_base+48(FP), DX    // im.data
	MOVQ dst_len+8(FP), CX     // len(dst)

	CMPQ CX, $4
	JL   magnitude_scalar

	MOVQ CX, AX
	SHRQ $2, AX                // AX = count / 4 (number of AVX2 iterations)
	ANDQ $3, CX                // CX = count % 4 (remainder for scalar)

magnitude_avx2_loop:
	VMOVUPD (SI), Y0           // Load 4 float64 from re
	VMOVUPD (DX), Y1           // Load 4 float64 from im
	VMULPD  Y0, Y0, Y0         // Y0 = re^2
	VMULPD  Y1, Y1, Y1         // Y1 = im^2
	VADDPD  Y1, Y0, Y0         // Y0 = re^2 + im^2
	VSQRTPD Y0, Y0             // Y0 = sqrt(re^2 + im^2)
	VMOVUPD Y0, (DI)           // Store to dst

	ADDQ $32, SI
	ADDQ $32, DX
	ADDQ $32, DI
	DECQ AX
	JNZ  magnitude_avx2_loop

	TESTQ CX, CX
	JZ    magnitude_done

magnitude_scalar:
	MOVSD  (SI), X0            // X0 = re[i]
	MOVSD  (DX), X1            // X1 = im[i]
	MULSD  X0, X0              // X0 = re^2
	MULSD  X1, X1              // X1 = im^2
	ADDSD  X1, X0              // X0 = re^2 + im^2
	SQRTSD X0, X0              // X0 = sqrt(re^2 + im^2)
	MOVSD  X0, (DI)            // Store to dst

	ADDQ $8, SI
	ADDQ $8, DX
	ADDQ $8, DI
	DECQ CX
	JNZ  magnitude_scalar

magnitude_done:
	VZEROUPPER
	RET
