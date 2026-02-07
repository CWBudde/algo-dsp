//go:build !purego && amd64

#include "textflag.h"

// func addMulBlockSSE2(dst, a, b []float64, scale float64)
// Fused add-multiply: dst[i] = (a[i] + b[i]) * scale
// Uses SSE2 to process 2 float64 values at once
TEXT ·addMulBlockSSE2(SB), NOSPLIT, $0-80
	MOVQ  dst_base+0(FP), DI   // dst.data
	MOVQ  a_base+24(FP), SI    // a.data
	MOVQ  b_base+48(FP), DX    // b.data
	MOVQ  dst_len+8(FP), CX    // len(dst)
	MOVSD scale+72(FP), X2     // scale value

	// Broadcast scale to both lanes
	UNPCKLPD X2, X2

	CMPQ CX, $2
	JL   addmul_scalar

	MOVQ CX, AX
	SHRQ $1, AX
	ANDQ $1, CX

addmul_sse2_loop:
	MOVUPD (SI), X0            // Load 2 float64 from a
	MOVUPD (DX), X1            // Load 2 float64 from b
	ADDPD  X1, X0              // X0 = a + b
	MULPD  X2, X0              // X0 = (a + b) * scale
	MOVUPD X0, (DI)            // Store to dst

	ADDQ $16, SI
	ADDQ $16, DX
	ADDQ $16, DI
	DECQ AX
	JNZ  addmul_sse2_loop

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
	RET

// func mulAddBlockSSE2(dst, a, b, c []float64)
// Fused multiply-add: dst[i] = a[i] * b[i] + c[i]
TEXT ·mulAddBlockSSE2(SB), NOSPLIT, $0-96
	MOVQ dst_base+0(FP), DI    // dst.data
	MOVQ a_base+24(FP), SI     // a.data
	MOVQ b_base+48(FP), DX     // b.data
	MOVQ c_base+72(FP), R8     // c.data
	MOVQ dst_len+8(FP), CX     // len(dst)

	CMPQ CX, $2
	JL   muladd_scalar

	MOVQ CX, AX
	SHRQ $1, AX
	ANDQ $1, CX

muladd_sse2_loop:
	MOVUPD (SI), X0            // Load 2 float64 from a
	MOVUPD (DX), X1            // Load 2 float64 from b
	MOVUPD (R8), X2            // Load 2 float64 from c
	MULPD  X1, X0              // X0 = a * b
	ADDPD  X2, X0              // X0 = a * b + c
	MOVUPD X0, (DI)            // Store to dst

	ADDQ $16, SI
	ADDQ $16, DX
	ADDQ $16, R8
	ADDQ $16, DI
	DECQ AX
	JNZ  muladd_sse2_loop

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
	RET
