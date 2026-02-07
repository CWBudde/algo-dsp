//go:build arm64 && !purego

#include "textflag.h"

// func processBlockNEON(buf []float64, b0, b1, b2, a1, a2, d0, d1 float64) (newD0, newD1 float64)
TEXT ·processBlockNEON(SB), NOSPLIT, $0-96
	MOVD buf_base+0(FP), R0
	MOVD buf_len+8(FP), R1

	FMOVD b0+24(FP), F20
	FMOVD b1+32(FP), F21
	FMOVD b2+40(FP), F22
	FMOVD a1+48(FP), F23
	FMOVD a2+56(FP), F24
	FMOVD d0+64(FP), F10
	FMOVD d1+72(FP), F11

loop2_check:
	CMP $2, R1
	BLT tail_check

	// x0/y0 and intermediate state (d0n/d1n)
	FMOVD (R0), F0
	FMULD F20, F0, F2
	FADDD F10, F2, F2

	FMULD F21, F0, F3
	FMULD F23, F2, F4
	FSUBD F4, F3, F3
	FADDD F11, F3, F3

	FMULD F22, F0, F4
	FMULD F24, F2, F5
	FSUBD F5, F4, F4

	// x1/y1 and final state update
	FMOVD 8(R0), F1
	FMULD F20, F1, F6
	FADDD F3, F6, F6

	FMULD F21, F1, F7
	FMULD F23, F6, F8
	FSUBD F8, F7, F7
	FADDD F4, F7, F10

	FMULD F22, F1, F8
	FMULD F24, F6, F9
	FSUBD F9, F8, F11

	FMOVD F2, (R0)
	FMOVD F6, 8(R0)

	ADD $16, R0
	SUB $2, R1
	B loop2_check

tail_check:
	CMP $0, R1
	BEQ done

	FMOVD (R0), F0
	FMULD F20, F0, F2
	FADDD F10, F2, F2

	FMULD F21, F0, F3
	FMULD F23, F2, F4
	FSUBD F4, F3, F3
	FADDD F11, F3, F10

	FMULD F22, F0, F4
	FMULD F24, F2, F5
	FSUBD F5, F4, F11

	FMOVD F2, (R0)

done:
	FMOVD F10, newD0+80(FP)
	FMOVD F11, newD1+88(FP)
	RET

