//go:build !purego && amd64

#include "textflag.h"

// func mulBlockAVX2(dst, a, b []float64)
// Element-wise multiply: dst[i] = a[i] * b[i]
// Uses AVX2 to process 4 float64 values at once
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
