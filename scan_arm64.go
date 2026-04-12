//go:build arm64

package astjson

// findQuoteOrBackslash returns the index of the first '"' (0x22) or '\\' (0x5C)
// byte in s, or len(s) if neither is found.
// Implemented in scan_arm64.s using NEON SIMD instructions.
//
//go:noescape
func findQuoteOrBackslash(s string) int

// countWhitespace returns the number of leading JSON whitespace bytes
// (space 0x20, tab 0x09, newline 0x0A, carriage return 0x0D) in s.
// Implemented in scan_arm64.s using NEON SIMD instructions.
//
//go:noescape
func countWhitespace(s string) int

// findNonNumber returns the index of the first byte in s that is NOT a JSON
// number character (0-9, '.', '-', '+', 'e', 'E').
// Returns len(s) if all bytes are number characters.
// Implemented in scan_arm64.s.
//
//go:noescape
func findNonNumber(s string) int
