//go:build !purego && amd64

#include "textflag.h"

// func mulBlockAVX2(dst, a, b []float64)
// Element-wise multiply: dst[i] = a[i] * b[i]
// Uses AVX2 to process 4 float64 values at once
TEXT ·addBlockAVX2(SB), NOSPLIT, $0-72
	MOVQ dst_base+0(FP), DI    // dst.data
	MOVQ a_base+24(FP), SI     // a.data
	MOVQ b_base+48(FP), DX     // b.data
	MOVQ dst_len+8(FP), CX     // len(dst)

	CMPQ CX, $4
	JL   addblock_scalar

	MOVQ CX, AX
	SHRQ $2, AX
	ANDQ $3, CX

addblock_avx2_loop:
	VMOVUPD (SI), Y0           // Load 4 float64 from a
	VMOVUPD (DX), Y1           // Load 4 float64 from b
	VADDPD  Y1, Y0, Y0         // Y0 = a + b
	VMOVUPD Y0, (DI)           // Store to dst

	ADDQ $32, SI
	ADDQ $32, DX
	ADDQ $32, DI
	DECQ AX
	JNZ  addblock_avx2_loop

	TESTQ CX, CX
	JZ    addblock_done

addblock_scalar:
	MOVSD  (SI), X0
	ADDSD  (DX), X0
	MOVSD  X0, (DI)

	ADDQ $8, SI
	ADDQ $8, DX
	ADDQ $8, DI
	DECQ CX
	JNZ  addblock_scalar

addblock_done:
	VZEROUPPER
	RET

// func addBlockInPlaceAVX2(dst, src []float64)
// In-place add: dst[i] += src[i]
TEXT ·addBlockInPlaceAVX2(SB), NOSPLIT, $0-48
	MOVQ dst_base+0(FP), DI    // dst.data
	MOVQ src_base+24(FP), SI   // src.data
	MOVQ dst_len+8(FP), CX     // len(dst)

	CMPQ CX, $4
	JL   addinplace_scalar

	MOVQ CX, AX
	SHRQ $2, AX
	ANDQ $3, CX

addinplace_avx2_loop:
	VMOVUPD (DI), Y0           // Load 4 float64 from dst
	VMOVUPD (SI), Y1           // Load 4 float64 from src
	VADDPD  Y1, Y0, Y0         // Y0 = dst + src
	VMOVUPD Y0, (DI)           // Store back to dst

	ADDQ $32, SI
	ADDQ $32, DI
	DECQ AX
	JNZ  addinplace_avx2_loop

	TESTQ CX, CX
	JZ    addinplace_done

addinplace_scalar:
	MOVSD  (DI), X0
	ADDSD  (SI), X0
	MOVSD  X0, (DI)

	ADDQ $8, SI
	ADDQ $8, DI
	DECQ CX
	JNZ  addinplace_scalar

addinplace_done:
	VZEROUPPER
	RET

// func mulAddBlockAVX2(dst, a, b, c []float64)
// Fused multiply-add: dst[i] = a[i] * b[i] + c[i]
// Note: Uses VFMADD if FMA is available, otherwise VMULPD + VADDPD
