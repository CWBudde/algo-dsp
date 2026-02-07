//go:build !purego && amd64

#include "textflag.h"

DATA ·absMask<>(SB)/8, $0x7fffffffffffffff
DATA ·absMask<>+8(SB)/8, $0x7fffffffffffffff
GLOBL ·absMask<>(SB), RODATA, $16

// func maxAbsSSE2(x []float64) float64
TEXT ·maxAbsSSE2(SB), NOSPLIT, $16-32
	MOVQ x_base+0(FP), DI
	MOVQ x_len+8(FP), CX

	CMPQ CX, $2
	JL   maxabs_scalar_init

	MOVUPD ·absMask<>(SB), X3

	// Seed vector max with first 2 elements.
	MOVUPD (DI), X0
	ANDPD X3, X0
	ADDQ $16, DI
	SUBQ $2, CX

	CMPQ CX, $2
	JL   maxabs_reduce

maxabs_sse2_loop:
	MOVUPD (DI), X1
	ANDPD X3, X1
	MAXPD X1, X0
	ADDQ $16, DI
	SUBQ $2, CX
	CMPQ CX, $2
	JGE  maxabs_sse2_loop

maxabs_reduce:
	MOVUPD X0, 0(SP)
	MOVSD 0(SP), X2
	MAXSD 8(SP), X2
	TESTQ CX, CX
	JZ    maxabs_done_sse2

	MOVSD (DI), X1
	ANDPD X3, X1
	MAXSD X1, X2

maxabs_done_sse2:
	MOVSD X2, ret+24(FP)
	RET

maxabs_scalar_init:
	MOVQ $0x7fffffffffffffff, AX
	MOVQ AX, X3
	MOVSD (DI), X2
	ANDPD X3, X2
	ADDQ $8, DI
	DECQ CX
	JZ   maxabs_done_scalar

maxabs_scalar_loop:
	MOVSD (DI), X1
	ANDPD X3, X1
	MAXSD X1, X2
	ADDQ $8, DI
	DECQ CX
	JNZ  maxabs_scalar_loop

maxabs_done_scalar:
	MOVSD X2, ret+24(FP)
	RET
