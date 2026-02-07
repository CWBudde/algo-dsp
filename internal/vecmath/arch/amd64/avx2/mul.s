//go:build !purego && amd64

#include "textflag.h"

// func mulBlockAVX2(dst, a, b []float64)
// Element-wise multiply: dst[i] = a[i] * b[i]
// Uses AVX2 to process 4 float64 values at once
TEXT ·mulBlockAVX2(SB), NOSPLIT, $0-72
	// Load slice headers
	MOVQ dst_base+0(FP), DI   // dst.data
	MOVQ a_base+24(FP), SI    // a.data
	MOVQ b_base+48(FP), DX    // b.data
	MOVQ dst_len+8(FP), CX    // len(dst)

	// Check if we have at least 4 elements for AVX2
	CMPQ CX, $4
	JL   mulblock_scalar

	// Calculate number of AVX2 iterations (4 elements per iter)
	MOVQ CX, AX
	SHRQ $2, AX              // AX = len / 4
	ANDQ $3, CX              // CX = len % 4 (remainder)

mulblock_avx2_loop:
	VMOVUPD (SI), Y0         // Load 4 float64 from a
	VMOVUPD (DX), Y1         // Load 4 float64 from b
	VMULPD  Y1, Y0, Y0       // Y0 = a * b
	VMOVUPD Y0, (DI)         // Store to dst

	ADDQ $32, SI             // Advance pointers (4 * 8 bytes)
	ADDQ $32, DX
	ADDQ $32, DI
	DECQ AX
	JNZ  mulblock_avx2_loop

	// Handle remainder with scalar loop
	TESTQ CX, CX
	JZ    mulblock_done

mulblock_scalar:
	MOVSD  (SI), X0          // Load 1 float64 from a
	MULSD  (DX), X0          // Multiply with b
	MOVSD  X0, (DI)          // Store to dst

	ADDQ $8, SI
	ADDQ $8, DX
	ADDQ $8, DI
	DECQ CX
	JNZ  mulblock_scalar

mulblock_done:
	VZEROUPPER               // Clear upper YMM to avoid AVX-SSE transition penalty
	RET

// func mulBlockInPlaceAVX2(dst, src []float64)
// In-place element-wise multiply: dst[i] *= src[i]
TEXT ·mulBlockInPlaceAVX2(SB), NOSPLIT, $0-48
	MOVQ dst_base+0(FP), DI   // dst.data
	MOVQ src_base+24(FP), SI  // src.data
	MOVQ dst_len+8(FP), CX    // len(dst)

	CMPQ CX, $4
	JL   mulinplace_scalar

	MOVQ CX, AX
	SHRQ $2, AX
	ANDQ $3, CX

mulinplace_avx2_loop:
	VMOVUPD (DI), Y0         // Load 4 float64 from dst
	VMOVUPD (SI), Y1         // Load 4 float64 from src
	VMULPD  Y1, Y0, Y0       // Y0 = dst * src
	VMOVUPD Y0, (DI)         // Store back to dst

	ADDQ $32, SI
	ADDQ $32, DI
	DECQ AX
	JNZ  mulinplace_avx2_loop

	TESTQ CX, CX
	JZ    mulinplace_done

mulinplace_scalar:
	MOVSD  (DI), X0
	MULSD  (SI), X0
	MOVSD  X0, (DI)

	ADDQ $8, SI
	ADDQ $8, DI
	DECQ CX
	JNZ  mulinplace_scalar

mulinplace_done:
	VZEROUPPER
	RET

// func scaleBlockAVX2(dst, src []float64, scale float64)
// Scale: dst[i] = src[i] * scale
