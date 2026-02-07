//go:build !purego && amd64

#include "textflag.h"

// func powerAVX2(dst, re, im []float64)
// Computes power (magnitude squared): dst[i] = re[i]^2 + im[i]^2
// Uses AVX2 to process 4 float64 values at once
TEXT ·powerAVX2(SB), NOSPLIT, $0-72
	MOVQ dst_base+0(FP), DI    // dst.data
	MOVQ re_base+24(FP), SI    // re.data
	MOVQ im_base+48(FP), DX    // im.data
	MOVQ dst_len+8(FP), CX     // len(dst)

	CMPQ CX, $4
	JL   power_scalar

	MOVQ CX, AX
	SHRQ $2, AX                // AX = count / 4 (number of AVX2 iterations)
	ANDQ $3, CX                // CX = count % 4 (remainder for scalar)

power_avx2_loop:
	VMOVUPD (SI), Y0           // Load 4 float64 from re
	VMOVUPD (DX), Y1           // Load 4 float64 from im
	VMULPD  Y0, Y0, Y0         // Y0 = re^2
	VMULPD  Y1, Y1, Y1         // Y1 = im^2
	VADDPD  Y1, Y0, Y0         // Y0 = re^2 + im^2
	VMOVUPD Y0, (DI)           // Store to dst

	ADDQ $32, SI
	ADDQ $32, DX
	ADDQ $32, DI
	DECQ AX
	JNZ  power_avx2_loop

	TESTQ CX, CX
	JZ    power_done

power_scalar:
	MOVSD  (SI), X0            // X0 = re[i]
	MOVSD  (DX), X1            // X1 = im[i]
	MULSD  X0, X0              // X0 = re^2
	MULSD  X1, X1              // X1 = im^2
	ADDSD  X1, X0              // X0 = re^2 + im^2
	MOVSD  X0, (DI)            // Store to dst

	ADDQ $8, SI
	ADDQ $8, DX
	ADDQ $8, DI
	DECQ CX
	JNZ  power_scalar

power_done:
	VZEROUPPER
	RET
