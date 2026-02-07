//go:build !purego && amd64

#include "textflag.h"

// func powerSSE2(dst, re, im []float64)
// Computes power (magnitude squared): dst[i] = re[i]^2 + im[i]^2
// Uses SSE2 to process 2 float64 values at once
TEXT ·powerSSE2(SB), NOSPLIT, $0-72
	MOVQ dst_base+0(FP), DI    // dst.data
	MOVQ re_base+24(FP), SI    // re.data
	MOVQ im_base+48(FP), DX    // im.data
	MOVQ dst_len+8(FP), CX     // len(dst)

	CMPQ CX, $2
	JL   power_scalar

	MOVQ CX, AX
	SHRQ $1, AX                // AX = count / 2 (number of SSE2 iterations)
	ANDQ $1, CX                // CX = count % 2 (remainder for scalar)

power_sse2_loop:
	MOVUPD (SI), X0            // Load 2 float64 from re
	MOVUPD (DX), X1            // Load 2 float64 from im
	MULPD  X0, X0              // X0 = re^2
	MULPD  X1, X1              // X1 = im^2
	ADDPD  X1, X0              // X0 = re^2 + im^2
	MOVUPD X0, (DI)            // Store to dst

	ADDQ $16, SI
	ADDQ $16, DX
	ADDQ $16, DI
	DECQ AX
	JNZ  power_sse2_loop

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
	RET
