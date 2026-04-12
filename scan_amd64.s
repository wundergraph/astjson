#include "textflag.h"

// func findQuoteOrBackslash(s string) int
//
// Scans s for the first '"' (0x22) or '\\' (0x5C) byte using SSE2.
// Processes 16 bytes per iteration with PCMPEQB + POR + PMOVMSKB.
// Falls back to a byte-at-a-time tail loop for the final <16 bytes.
TEXT ·findQuoteOrBackslash(SB), NOSPLIT, $0-24
	MOVQ	s_base+0(FP), SI	// SI = pointer to string data
	MOVQ	s_len+8(FP), DX		// DX = length of string
	XORQ	AX, AX			// AX = current offset (0)

	// Build 16-byte patterns in XMM registers
	MOVQ	$0x2222222222222222, CX
	MOVQ	CX, X1
	PUNPCKLQDQ	X1, X1		// X1 = 16 bytes of 0x22 ('"')
	MOVQ	$0x5C5C5C5C5C5C5C5C, CX
	MOVQ	CX, X2
	PUNPCKLQDQ	X2, X2		// X2 = 16 bytes of 0x5C ('\\')

	// Calculate end of full 16-byte blocks
	MOVQ	DX, CX
	ANDQ	$~15, CX		// CX = length rounded down to 16

	CMPQ	AX, CX
	JGE	fqob_tail		// skip SIMD if less than 16 bytes

fqob_loop16:
	MOVOU	(SI)(AX*1), X0		// load 16 unaligned bytes from s[AX]
	MOVO	X0, X3			// copy for second comparison
	PCMPEQB	X1, X0			// X0[i] = 0xFF where byte == '"'
	PCMPEQB	X2, X3			// X3[i] = 0xFF where byte == '\\'
	POR	X0, X3			// X3[i] = 0xFF where either matched
	PMOVMSKB	X3, DI		// DI = 16-bit mask of matches
	TESTL	DI, DI
	JNZ	fqob_found16		// at least one match in this block

	ADDQ	$16, AX
	CMPQ	AX, CX
	JL	fqob_loop16

fqob_tail:
	// Process remaining bytes one at a time
	CMPQ	AX, DX
	JGE	fqob_notfound

fqob_tail_loop:
	MOVBLZX	(SI)(AX*1), DI
	CMPL	DI, $0x22		// '"'
	JE	fqob_done
	CMPL	DI, $0x5C		// '\\'
	JE	fqob_done
	INCQ	AX
	CMPQ	AX, DX
	JL	fqob_tail_loop

fqob_notfound:
	MOVQ	DX, ret+16(FP)		// return len(s)
	RET

fqob_found16:
	BSFL	DI, DI			// find lowest set bit
	ADDQ	DI, AX			// AX = base offset + bit position

fqob_done:
	MOVQ	AX, ret+16(FP)		// return offset
	RET

// func countWhitespace(s string) int
//
// Counts leading JSON whitespace bytes (0x20, 0x09, 0x0A, 0x0D) using SSE2.
// Processes 16 bytes per iteration. Returns count of leading whitespace bytes.
TEXT ·countWhitespace(SB), NOSPLIT, $0-24
	MOVQ	s_base+0(FP), SI
	MOVQ	s_len+8(FP), DX
	XORQ	AX, AX			// AX = current offset

	// Build whitespace patterns
	MOVQ	$0x2020202020202020, CX	// space
	MOVQ	CX, X1
	PUNPCKLQDQ	X1, X1
	MOVQ	$0x0909090909090909, CX	// tab
	MOVQ	CX, X2
	PUNPCKLQDQ	X2, X2
	MOVQ	$0x0A0A0A0A0A0A0A0A, CX	// newline
	MOVQ	CX, X3
	PUNPCKLQDQ	X3, X3
	MOVQ	$0x0D0D0D0D0D0D0D0D, CX	// carriage return
	MOVQ	CX, X4
	PUNPCKLQDQ	X4, X4

	MOVQ	DX, CX
	ANDQ	$~15, CX
	CMPQ	AX, CX
	JGE	cw_tail

cw_loop16:
	MOVOU	(SI)(AX*1), X0
	MOVO	X0, X5
	MOVO	X0, X6
	MOVO	X0, X7
	PCMPEQB	X1, X0			// match space
	PCMPEQB	X2, X5			// match tab
	PCMPEQB	X3, X6			// match newline
	PCMPEQB	X4, X7			// match CR
	POR	X5, X0
	POR	X7, X6
	POR	X6, X0			// X0 = all whitespace matches
	PMOVMSKB	X0, DI
	CMPL	DI, $0xFFFF		// all 16 bytes are whitespace?
	JNE	cw_found16

	ADDQ	$16, AX
	CMPQ	AX, CX
	JL	cw_loop16

cw_tail:
	CMPQ	AX, DX
	JGE	cw_done

cw_tail_loop:
	MOVBLZX	(SI)(AX*1), DI
	CMPL	DI, $0x20		// space
	JE	cw_next
	CMPL	DI, $0x09		// tab
	JE	cw_next
	CMPL	DI, $0x0A		// newline
	JE	cw_next
	CMPL	DI, $0x0D		// CR
	JE	cw_next
	JMP	cw_done

cw_next:
	INCQ	AX
	CMPQ	AX, DX
	JL	cw_tail_loop

cw_done:
	MOVQ	AX, ret+16(FP)
	RET

cw_found16:
	XORL	$0xFFFF, DI		// invert: set bits = non-whitespace
	BSFL	DI, DI			// find first non-whitespace
	ADDQ	DI, AX
	MOVQ	AX, ret+16(FP)
	RET

// func findNonNumber(s string) int
//
// Finds the first byte that is NOT a JSON number character.
// Valid number characters: '0'-'9', '.', '-', '+', 'e', 'E'
// NaN/Inf letters are handled separately by parseRawNumber.
// Uses a byte-at-a-time approach with a 256-byte lookup table since numbers
// are typically short (1-20 chars) and SIMD setup cost would dominate.
TEXT ·findNonNumber(SB), NOSPLIT, $0-24
	MOVQ	s_base+0(FP), SI
	MOVQ	s_len+8(FP), DX
	XORQ	AX, AX

	CMPQ	AX, DX
	JGE	fnn_done

	LEAQ	numtab<>(SB), BX	// BX = lookup table base

fnn_loop:
	MOVBLZX	(SI)(AX*1), CX		// load byte
	MOVBLZX	(BX)(CX*1), DI		// lookup
	TESTL	DI, DI
	JZ	fnn_done		// not a number char
	INCQ	AX
	CMPQ	AX, DX
	JL	fnn_loop

fnn_done:
	MOVQ	AX, ret+16(FP)
	RET

// Lookup table: 1 = valid number char, 0 = not
// Valid: '0'-'9' (0x30-0x39), '+' (0x2B), '-' (0x2D), '.' (0x2E),
//        'E' (0x45), 'e' (0x65)
//
// Little-endian byte layout: byte at lowest offset = LSB of uint64.
DATA numtab<>+0x00(SB)/8, $0x0000000000000000  // 0x00-0x07: all zero
DATA numtab<>+0x08(SB)/8, $0x0000000000000000  // 0x08-0x0F: all zero
DATA numtab<>+0x10(SB)/8, $0x0000000000000000  // 0x10-0x17: all zero
DATA numtab<>+0x18(SB)/8, $0x0000000000000000  // 0x18-0x1F: all zero
DATA numtab<>+0x20(SB)/8, $0x0000000000000000  // 0x20-0x27: all zero
DATA numtab<>+0x28(SB)/8, $0x0001010001000000  // 0x28-0x2F: '+'(0x2B)=1, '-'(0x2D)=1, '.'(0x2E)=1
DATA numtab<>+0x30(SB)/8, $0x0101010101010101  // 0x30-0x37: '0'-'7' all 1
DATA numtab<>+0x38(SB)/8, $0x0000000000000101  // 0x38-0x3F: '8'=1, '9'=1
DATA numtab<>+0x40(SB)/8, $0x0000010000000000  // 0x40-0x47: 'E'(0x45)=1
DATA numtab<>+0x48(SB)/8, $0x0000000000000000  // 0x48-0x4F: all zero
DATA numtab<>+0x50(SB)/8, $0x0000000000000000  // 0x50-0x57: all zero
DATA numtab<>+0x58(SB)/8, $0x0000000000000000  // 0x58-0x5F: all zero
DATA numtab<>+0x60(SB)/8, $0x0000010000000000  // 0x60-0x67: 'e'(0x65)=1
DATA numtab<>+0x68(SB)/8, $0x0000000000000000  // 0x68-0x6F: all zero
DATA numtab<>+0x70(SB)/8, $0x0000000000000000  // 0x70-0x77: all zero
DATA numtab<>+0x78(SB)/8, $0x0000000000000000  // 0x78-0x7F: all zero
// 0x80-0xFF: all zeros (128 bytes)
DATA numtab<>+0x80(SB)/8, $0x0000000000000000
DATA numtab<>+0x88(SB)/8, $0x0000000000000000
DATA numtab<>+0x90(SB)/8, $0x0000000000000000
DATA numtab<>+0x98(SB)/8, $0x0000000000000000
DATA numtab<>+0xa0(SB)/8, $0x0000000000000000
DATA numtab<>+0xa8(SB)/8, $0x0000000000000000
DATA numtab<>+0xb0(SB)/8, $0x0000000000000000
DATA numtab<>+0xb8(SB)/8, $0x0000000000000000
DATA numtab<>+0xc0(SB)/8, $0x0000000000000000
DATA numtab<>+0xc8(SB)/8, $0x0000000000000000
DATA numtab<>+0xd0(SB)/8, $0x0000000000000000
DATA numtab<>+0xd8(SB)/8, $0x0000000000000000
DATA numtab<>+0xe0(SB)/8, $0x0000000000000000
DATA numtab<>+0xe8(SB)/8, $0x0000000000000000
DATA numtab<>+0xf0(SB)/8, $0x0000000000000000
DATA numtab<>+0xf8(SB)/8, $0x0000000000000000
GLOBL numtab<>(SB), (NOPTR+RODATA), $256
