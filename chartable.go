package astjson

// Character classification lookup table. A single 256-byte table replaces
// multi-branch comparisons in several parser hot paths (skipWSSlow,
// hasSpecialChars, parseRawNumber). The table is small enough to live in L1
// and, crucially, a single load + mask instruction replaces a chain of
// branches.
const (
	charWS      uint8 = 1 << 0 // JSON whitespace: space, tab, newline, CR
	charNumChar uint8 = 1 << 1 // valid in a JSON number: digits, '.', '-', '+', 'e', 'E'
	charEscape  uint8 = 1 << 2 // needs escaping when marshalled in a JSON string
)

var charFlags [256]uint8

// hexDigit maps an ASCII byte to its hex digit value (0-15).
// Non-hex bytes map to 0xFF, which carries a high-bit sentinel so four
// table lookups can be OR'd together and validated in one AND.
var hexDigit [256]uint8

func init() {
	// Whitespace.
	charFlags[0x20] |= charWS // space
	charFlags[0x09] |= charWS // tab
	charFlags[0x0A] |= charWS // newline
	charFlags[0x0D] |= charWS // carriage return

	// Number characters.
	for c := byte('0'); c <= '9'; c++ {
		charFlags[c] |= charNumChar
	}
	charFlags['.'] |= charNumChar
	charFlags['-'] |= charNumChar
	charFlags['+'] |= charNumChar
	charFlags['e'] |= charNumChar
	charFlags['E'] |= charNumChar

	// Characters that need escaping when marshalling a JSON string.
	charFlags['"'] |= charEscape
	charFlags['\\'] |= charEscape
	for c := 0; c < 0x20; c++ {
		charFlags[c] |= charEscape
	}

	// Hex digit table (0xFF == invalid).
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

// parseHex4 decodes exactly four ASCII hex digits into a uint16.
// Returns (value, true) on success, or (0, false) if any byte is not a hex
// digit. Caller must ensure len(s) >= 4.
func parseHex4(s string) (uint16, bool) {
	a, b, c, d := hexDigit[s[0]], hexDigit[s[1]], hexDigit[s[2]], hexDigit[s[3]]
	// Valid digits are in 0..15 (low nibble). The invalid sentinel 0xFF has
	// its high bits set, so a single AND after OR'ing all four covers
	// detection of any invalid digit.
	if (a|b|c|d)&0xF0 != 0 {
		return 0, false
	}
	return uint16(a)<<12 | uint16(b)<<8 | uint16(c)<<4 | uint16(d), true
}
