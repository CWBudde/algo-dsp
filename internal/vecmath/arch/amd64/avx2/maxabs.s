//go:build !purego && amd64

#include "textflag.h"

// Absolute-value mask for float64 lanes: clear sign bit.
DATA ·absMask<>(SB)/8, $0x7fffffffffffffff
DATA ·absMask<>+8(SB)/8, $0x7fffffffffffffff
DATA ·absMask<>+16(SB)/8, $0x7fffffffffffffff
DATA ·absMask<>+24(SB)/8, $0x7fffffffffffffff
GLOBL ·absMask<>(SB), RODATA, $32

// func maxAbsAVX2(x []float64) float64
TEXT ·maxAbsAVX2(SB), NOSPLIT, $32-32
	MOVQ x_base+0(FP), DI
	MOVQ x_len+8(FP), CX

	CMPQ CX, $4
	JL   maxabs_scalar_init

	VMOVUPD ·absMask<>(SB), Y2

	// Seed vector max with first 4 elements.
	VMOVUPD (DI), Y0
	VANDPD  Y2, Y0, Y1
	ADDQ $32, DI
	SUBQ $4, CX

	CMPQ CX, $4
	JL   maxabs_reduce

maxabs_avx2_loop:
	VMOVUPD (DI), Y0
	VANDPD  Y2, Y0, Y0
	VMAXPD  Y0, Y1, Y1
	ADDQ $32, DI
	SUBQ $4, CX
	CMPQ CX, $4
	JGE  maxabs_avx2_loop

maxabs_reduce:
	VMOVUPD Y1, 0(SP)
	MOVSD 0(SP), X0
	MAXSD 8(SP), X0
	MAXSD 16(SP), X0
	MAXSD 24(SP), X0

	MOVQ $0x7fffffffffffffff, BX
	MOVQ BX, X3
	TESTQ CX, CX
	JZ    maxabs_done_avx2

maxabs_scalar_tail:
	MOVSD (DI), X2
	ANDPD X3, X2
	MAXSD X2, X0
	ADDQ $8, DI
	DECQ CX
	JNZ  maxabs_scalar_tail

maxabs_done_avx2:
	MOVSD X0, ret+24(FP)
	VZEROUPPER
	RET

maxabs_scalar_init:
	MOVQ $0x7fffffffffffffff, AX
	MOVQ AX, X3

	MOVSD (DI), X0
	ANDPD X3, X0
	ADDQ $8, DI
	DECQ CX
	JZ   maxabs_done_scalar

maxabs_scalar_loop:
	MOVSD (DI), X2
	ANDPD X3, X2
	MAXSD X2, X0
	ADDQ $8, DI
	DECQ CX
	JNZ  maxabs_scalar_loop

maxabs_done_scalar:
	MOVSD X0, ret+24(FP)
	RET
