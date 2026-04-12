#include "textflag.h"

// func findQuoteOrBackslash(s string) int
//
// Scans s for the first '"' (0x22) or '\\' (0x5C) byte using NEON.
// Processes 16 bytes per iteration with VCMEQ + VORR.
// Uses RBIT+CLZ on extracted 64-bit halves to locate the first match.
// Falls back to byte-at-a-time for the final <16 bytes.
TEXT ·findQuoteOrBackslash(SB), NOSPLIT, $0-24
	MOVD	s_base+0(FP), R0	// R0 = string data pointer
	MOVD	s_len+8(FP), R1		// R1 = string length
	MOVD	ZR, R2			// R2 = current offset

	// Broadcast comparison bytes to NEON registers
	MOVD	$0x22, R3
	VDUP	R3, V1.B16		// V1 = 16× '"' (0x22)
	MOVD	$0x5C, R3
	VDUP	R3, V2.B16		// V2 = 16× '\\' (0x5C)

	// End of full 16-byte blocks
	AND	$~15, R1, R3		// R3 = length rounded down to 16

	CMP	R3, R2
	BGE	fqob_tail

fqob_loop16:
	ADD	R0, R2, R4		// R4 = base + offset
	VLD1	(R4), [V0.B16]		// Load 16 bytes
	VCMEQ	V1.B16, V0.B16, V3.B16	// V3[i] = 0xFF where byte == '"'
	VCMEQ	V2.B16, V0.B16, V4.B16	// V4[i] = 0xFF where byte == '\\'
	VORR	V3.B16, V4.B16, V5.B16	// V5 = combined matches

	// Extract to GP registers and check for any match
	VMOV	V5.D[0], R5		// Low 8 bytes of match result
	VMOV	V5.D[1], R6		// High 8 bytes of match result
	ORR	R5, R6, R7
	CBNZ	R7, fqob_found16

	ADD	$16, R2, R2
	CMP	R3, R2
	BLT	fqob_loop16

fqob_tail:
	// Process remaining bytes one at a time
	CMP	R1, R2
	BGE	fqob_notfound

fqob_tail_loop:
	MOVBU	(R0)(R2), R5
	CMP	$0x22, R5		// '"'
	BEQ	fqob_done
	CMP	$0x5C, R5		// '\\'
	BEQ	fqob_done
	ADD	$1, R2, R2
	CMP	R1, R2
	BLT	fqob_tail_loop

fqob_notfound:
	MOVD	R1, ret+16(FP)		// return len(s)
	RET

fqob_found16:
	// Find exact position. Each matching byte is 0xFF (all 8 bits set).
	// Use RBIT+CLZ to find the first set bit = first matching byte × 8.
	CBZ	R5, fqob_found_high
	RBIT	R5, R5
	CLZ	R5, R5
	LSR	$3, R5, R5		// byte index in low half (0-7)
	ADD	R5, R2, R2
	B	fqob_done

fqob_found_high:
	RBIT	R6, R6
	CLZ	R6, R6
	LSR	$3, R6, R6		// byte index in high half (0-7)
	ADD	$8, R6, R6		// adjust for high half offset
	ADD	R6, R2, R2

fqob_done:
	MOVD	R2, ret+16(FP)		// return offset
	RET

// func countWhitespace(s string) int
//
// Counts leading JSON whitespace bytes (0x20, 0x09, 0x0A, 0x0D) using NEON.
// Processes 16 bytes per iteration. Returns count of leading whitespace bytes.
TEXT ·countWhitespace(SB), NOSPLIT, $0-24
	MOVD	s_base+0(FP), R0
	MOVD	s_len+8(FP), R1
	MOVD	ZR, R2			// current offset

	// Broadcast whitespace patterns to NEON registers
	MOVD	$0x20, R3
	VDUP	R3, V1.B16		// V1 = 16× space
	MOVD	$0x09, R3
	VDUP	R3, V2.B16		// V2 = 16× tab
	MOVD	$0x0A, R3
	VDUP	R3, V3.B16		// V3 = 16× newline
	MOVD	$0x0D, R3
	VDUP	R3, V4.B16		// V4 = 16× carriage return

	// End of full 16-byte blocks
	AND	$~15, R1, R3
	// Preload NOT constant for inversion
	MOVD	$-1, R9

	CMP	R3, R2
	BGE	cw_tail

cw_loop16:
	ADD	R0, R2, R4
	VLD1	(R4), [V0.B16]		// Load 16 bytes
	VCMEQ	V1.B16, V0.B16, V5.B16	// match space
	VCMEQ	V2.B16, V0.B16, V6.B16	// match tab
	VCMEQ	V3.B16, V0.B16, V7.B16	// match newline
	VCMEQ	V4.B16, V0.B16, V8.B16	// match CR
	VORR	V5.B16, V6.B16, V5.B16	// space | tab
	VORR	V7.B16, V8.B16, V7.B16	// newline | CR
	VORR	V5.B16, V7.B16, V5.B16	// all whitespace matches

	// Check if ALL 16 bytes are whitespace (all 0xFF)
	VMOV	V5.D[0], R5		// low 8 bytes
	VMOV	V5.D[1], R6		// high 8 bytes
	AND	R5, R6, R7		// R7 = R5 & R6
	ADD	$1, R7, R8		// if R7 was all-ones, R8 overflows to 0
	CBZ	R8, cw_all_ws

	// Not all whitespace — find first non-WS byte.
	// Invert: WS bytes (0xFF) → 0x00, non-WS bytes (0x00) → 0xFF
	EOR	R9, R5, R5		// R5 = ~R5
	CBNZ	R5, cw_found_low
	EOR	R9, R6, R6		// R6 = ~R6 (must have a match)
	RBIT	R6, R6
	CLZ	R6, R6
	LSR	$3, R6, R6
	ADD	$8, R6, R6		// high half offset
	ADD	R6, R2, R2
	B	cw_done

cw_found_low:
	RBIT	R5, R5
	CLZ	R5, R5
	LSR	$3, R5, R5
	ADD	R5, R2, R2
	B	cw_done

cw_all_ws:
	ADD	$16, R2, R2
	CMP	R3, R2
	BLT	cw_loop16

cw_tail:
	CMP	R1, R2
	BGE	cw_done

cw_tail_loop:
	MOVBU	(R0)(R2), R5
	CMP	$0x20, R5		// space
	BEQ	cw_next
	CMP	$0x09, R5		// tab
	BEQ	cw_next
	CMP	$0x0A, R5		// newline
	BEQ	cw_next
	CMP	$0x0D, R5		// CR
	BEQ	cw_next
	B	cw_done

cw_next:
	ADD	$1, R2, R2
	CMP	R1, R2
	BLT	cw_tail_loop

cw_done:
	MOVD	R2, ret+16(FP)
	RET

// func findNonNumber(s string) int
//
// Finds the first byte that is NOT a JSON number character.
// Valid: '0'-'9', '.', '-', '+', 'e', 'E'
// Uses a byte-at-a-time lookup table (numbers are typically short).
TEXT ·findNonNumber(SB), NOSPLIT, $0-24
	MOVD	s_base+0(FP), R0
	MOVD	s_len+8(FP), R1
	MOVD	ZR, R2

	CMP	R1, R2
	BGE	fnn_done

	MOVD	$numtab<>(SB), R3	// R3 = lookup table base

fnn_loop:
	MOVBU	(R0)(R2), R4		// load byte
	MOVBU	(R3)(R4), R5		// lookup
	CBZ	R5, fnn_done		// not a number char
	ADD	$1, R2, R2
	CMP	R1, R2
	BLT	fnn_loop

fnn_done:
	MOVD	R2, ret+16(FP)
	RET

// Lookup table: 1 = valid number char, 0 = not
// Valid: '0'-'9' (0x30-0x39), '+' (0x2B), '-' (0x2D), '.' (0x2E),
//        'E' (0x45), 'e' (0x65)
//
// Little-endian byte layout: byte at lowest offset = LSB of uint64.
DATA numtab<>+0x00(SB)/8, $0x0000000000000000  // 0x00-0x07
DATA numtab<>+0x08(SB)/8, $0x0000000000000000  // 0x08-0x0F
DATA numtab<>+0x10(SB)/8, $0x0000000000000000  // 0x10-0x17
DATA numtab<>+0x18(SB)/8, $0x0000000000000000  // 0x18-0x1F
DATA numtab<>+0x20(SB)/8, $0x0000000000000000  // 0x20-0x27
DATA numtab<>+0x28(SB)/8, $0x0001010001000000  // 0x28-0x2F: '+'(0x2B)=1, '-'(0x2D)=1, '.'(0x2E)=1
DATA numtab<>+0x30(SB)/8, $0x0101010101010101  // 0x30-0x37: '0'-'7' all 1
DATA numtab<>+0x38(SB)/8, $0x0000000000000101  // 0x38-0x3F: '8'=1, '9'=1
DATA numtab<>+0x40(SB)/8, $0x0000010000000000  // 0x40-0x47: 'E'(0x45)=1
DATA numtab<>+0x48(SB)/8, $0x0000000000000000  // 0x48-0x4F
DATA numtab<>+0x50(SB)/8, $0x0000000000000000  // 0x50-0x57
DATA numtab<>+0x58(SB)/8, $0x0000000000000000  // 0x58-0x5F
DATA numtab<>+0x60(SB)/8, $0x0000010000000000  // 0x60-0x67: 'e'(0x65)=1
DATA numtab<>+0x68(SB)/8, $0x0000000000000000  // 0x68-0x6F
DATA numtab<>+0x70(SB)/8, $0x0000000000000000  // 0x70-0x77
DATA numtab<>+0x78(SB)/8, $0x0000000000000000  // 0x78-0x7F
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
