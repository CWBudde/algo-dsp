//go:build !purego && amd64

#include "textflag.h"

// func mulBlockAVX2(dst, a, b []float64)
// Element-wise multiply: dst[i] = a[i] * b[i]
// Uses AVX2 to process 4 float64 values at once
TEXT ·scaleBlockAVX2(SB), NOSPLIT, $0-56
	MOVQ  dst_base+0(FP), DI  // dst.data
	MOVQ  src_base+24(FP), SI // src.data
	MOVQ  dst_len+8(FP), CX   // len(dst)
	MOVSD scale+48(FP), X1    // scale value

	// Broadcast scale to all 4 lanes of Y1
	VBROADCASTSD X1, Y1

	CMPQ CX, $4
	JL   scaleblock_scalar

	MOVQ CX, AX
	SHRQ $2, AX
	ANDQ $3, CX

scaleblock_avx2_loop:
	VMOVUPD (SI), Y0         // Load 4 float64 from src
	VMULPD  Y1, Y0, Y0       // Y0 = src * scale
	VMOVUPD Y0, (DI)         // Store to dst

	ADDQ $32, SI
	ADDQ $32, DI
	DECQ AX
	JNZ  scaleblock_avx2_loop

	TESTQ CX, CX
	JZ    scaleblock_done

scaleblock_scalar:
	MOVSD  (SI), X0
	MULSD  X1, X0
	MOVSD  X0, (DI)

	ADDQ $8, SI
	ADDQ $8, DI
	DECQ CX
	JNZ  scaleblock_scalar

scaleblock_done:
	VZEROUPPER
	RET

// func scaleBlockInPlaceAVX2(dst []float64, scale float64)
// In-place scale: dst[i] *= scale
TEXT ·scaleBlockInPlaceAVX2(SB), NOSPLIT, $0-32
	MOVQ  dst_base+0(FP), DI  // dst.data
	MOVQ  dst_len+8(FP), CX   // len(dst)
	MOVSD scale+24(FP), X1    // scale value

	VBROADCASTSD X1, Y1

	CMPQ CX, $4
	JL   scaleinplace_scalar

	MOVQ CX, AX
	SHRQ $2, AX
	ANDQ $3, CX

scaleinplace_avx2_loop:
	VMOVUPD (DI), Y0
	VMULPD  Y1, Y0, Y0
	VMOVUPD Y0, (DI)

	ADDQ $32, DI
	DECQ AX
	JNZ  scaleinplace_avx2_loop

	TESTQ CX, CX
	JZ    scaleinplace_done

scaleinplace_scalar:
	MOVSD  (DI), X0
	MULSD  X1, X0
	MOVSD  X0, (DI)

	ADDQ $8, DI
	DECQ CX
	JNZ  scaleinplace_scalar

scaleinplace_done:
	VZEROUPPER
	RET

// func addMulBlockAVX2(dst, a, b []float64, scale float64)
// Fused add-multiply: dst[i] = (a[i] + b[i]) * scale
