//go:build !amd64 && !arm64

package astjson

// findQuoteOrBackslash returns the index of the first '"' (0x22) or '\\' (0x5C)
// byte in s, or len(s) if neither is found.
func findQuoteOrBackslash(s string) int {
	for i := 0; i < len(s); i++ {
		if s[i] == '"' || s[i] == '\\' {
			return i
		}
	}
	return len(s)
}

// countWhitespace returns the number of leading JSON whitespace bytes
// (space 0x20, tab 0x09, newline 0x0A, carriage return 0x0D) in s.
func countWhitespace(s string) int {
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case ' ', '\t', '\n', '\r':
			continue
		default:
			return i
		}
	}
	return len(s)
}

// findNonNumber returns the index of the first byte in s that is NOT a JSON
// number character (0-9, '.', '-', '+', 'e', 'E').
// Returns len(s) if all bytes are number characters.
// Note: NaN/Inf letters are NOT included; they are handled separately
// in parseRawNumber via a special-case check.
func findNonNumber(s string) int {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= '0' && c <= '9') || c == '.' || c == '-' || c == '+' || c == 'e' || c == 'E' {
			continue
		}
		return i
	}
	return len(s)
}
