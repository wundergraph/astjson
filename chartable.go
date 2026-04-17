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
}
