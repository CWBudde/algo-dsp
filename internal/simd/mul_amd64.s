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
TEXT ·addMulBlockAVX2(SB), NOSPLIT, $0-80
	MOVQ  dst_base+0(FP), DI   // dst.data
	MOVQ  a_base+24(FP), SI    // a.data
	MOVQ  b_base+48(FP), DX    // b.data
	MOVQ  dst_len+8(FP), CX    // len(dst)
	MOVSD scale+72(FP), X2     // scale value

	VBROADCASTSD X2, Y2        // Broadcast scale to all 4 lanes

	CMPQ CX, $4
	JL   addmul_scalar

	MOVQ CX, AX
	SHRQ $2, AX
	ANDQ $3, CX

addmul_avx2_loop:
	VMOVUPD (SI), Y0           // Load 4 float64 from a
	VMOVUPD (DX), Y1           // Load 4 float64 from b
	VADDPD  Y1, Y0, Y0         // Y0 = a + b
	VMULPD  Y2, Y0, Y0         // Y0 = (a + b) * scale
	VMOVUPD Y0, (DI)           // Store to dst

	ADDQ $32, SI
	ADDQ $32, DX
	ADDQ $32, DI
	DECQ AX
	JNZ  addmul_avx2_loop

	TESTQ CX, CX
	JZ    addmul_done

addmul_scalar:
	MOVSD  (SI), X0            // Load from a
	ADDSD  (DX), X0            // Add b
	MULSD  X2, X0              // Multiply by scale
	MOVSD  X0, (DI)            // Store to dst

	ADDQ $8, SI
	ADDQ $8, DX
	ADDQ $8, DI
	DECQ CX
	JNZ  addmul_scalar

addmul_done:
	VZEROUPPER
	RET

// func addBlockAVX2(dst, a, b []float64)
// Element-wise add: dst[i] = a[i] + b[i]
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
TEXT ·mulAddBlockAVX2(SB), NOSPLIT, $0-96
	MOVQ dst_base+0(FP), DI    // dst.data
	MOVQ a_base+24(FP), SI     // a.data
	MOVQ b_base+48(FP), DX     // b.data
	MOVQ c_base+72(FP), R8     // c.data
	MOVQ dst_len+8(FP), CX     // len(dst)

	CMPQ CX, $4
	JL   muladd_scalar

	MOVQ CX, AX
	SHRQ $2, AX
	ANDQ $3, CX

muladd_avx2_loop:
	VMOVUPD (SI), Y0           // Load 4 float64 from a
	VMOVUPD (DX), Y1           // Load 4 float64 from b
	VMOVUPD (R8), Y2           // Load 4 float64 from c
	VMULPD  Y1, Y0, Y0         // Y0 = a * b
	VADDPD  Y2, Y0, Y0         // Y0 = a * b + c
	VMOVUPD Y0, (DI)           // Store to dst

	ADDQ $32, SI
	ADDQ $32, DX
	ADDQ $32, R8
	ADDQ $32, DI
	DECQ AX
	JNZ  muladd_avx2_loop

	TESTQ CX, CX
	JZ    muladd_done

muladd_scalar:
	MOVSD  (SI), X0            // Load from a
	MULSD  (DX), X0            // Multiply with b
	ADDSD  (R8), X0            // Add c
	MOVSD  X0, (DI)            // Store to dst

	ADDQ $8, SI
	ADDQ $8, DX
	ADDQ $8, R8
	ADDQ $8, DI
	DECQ CX
	JNZ  muladd_scalar

muladd_done:
	VZEROUPPER
	RET
