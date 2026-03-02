package astjson

const (
	charWS      uint8 = 1 << 0 // whitespace: space, tab, newline, CR
	charNumChar uint8 = 1 << 1 // valid in number: digits, ., -, +, e, E
	charEscape  uint8 = 1 << 2 // needs escaping in JSON string: ", \, < 0x20
)

// charFlags is a 256-byte lookup table for character classification.
// Replaces multi-branch comparisons in hot loops with a single table lookup.
var charFlags [256]uint8

// hexDigit maps ASCII bytes to their hex digit value (0-15).
// Invalid hex chars are mapped to 0xFF.
var hexDigit [256]uint8

func init() {
	// Whitespace
	charFlags[0x20] |= charWS // space
	charFlags[0x09] |= charWS // tab
	charFlags[0x0A] |= charWS // newline
	charFlags[0x0D] |= charWS // carriage return

	// Number characters
	for c := byte('0'); c <= '9'; c++ {
		charFlags[c] |= charNumChar
	}
	charFlags['.'] |= charNumChar
	charFlags['-'] |= charNumChar
	charFlags['+'] |= charNumChar
	charFlags['e'] |= charNumChar
	charFlags['E'] |= charNumChar

	// Characters that need escaping in JSON strings
	charFlags['"'] |= charEscape
	charFlags['\\'] |= charEscape
	for c := range 0x20 {
		charFlags[c] |= charEscape
	}

	// Hex digit lookup (0xFF = invalid)
	for i := range hexDigit {
		hexDigit[i] = 0xFF
	}
	for c := byte('0'); c <= '9'; c++ {
		hexDigit[c] = c - '0'
	}
	for c := byte('a'); c <= 'f'; c++ {
		hexDigit[c] = c - 'a' + 10
	}
	for c := byte('A'); c <= 'F'; c++ {
		hexDigit[c] = c - 'A' + 10
	}
}

// parseHex4 parses 4 hex digits from s into a uint16.
// Returns the value and true on success, or 0 and false on invalid input.
func parseHex4(s string) (uint16, bool) {
	a, b, c, d := hexDigit[s[0]], hexDigit[s[1]], hexDigit[s[2]], hexDigit[s[3]]
	// Valid hex digits are 0..15 (low nibble); invalid sentinel 0xFF has high bits set.
	if (a|b|c|d)&0xF0 != 0 {
		return 0, false
	}
	return uint16(a)<<12 | uint16(b)<<8 | uint16(c)<<4 | uint16(d), true
}
