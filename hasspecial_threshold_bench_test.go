package astjson

import (
	"fmt"
	"strings"
	"testing"
)

// This file benchmarks several implementations of hasSpecialChars across a
// sweep of string lengths to identify the threshold (if any) at which a
// SIMD-friendly variant overtakes the per-byte table-lookup loop. The
// "common" case for these calls is a clean string (no `"`, no `\`, no
// control chars) — that is the cost-dominant path because every byte must
// be inspected.
//
// Variants under test:
//
//   - Loop:        current implementation (table lookup, byte-at-a-time).
//   - IndexAny:    strings.IndexAny with the 34-byte escape set (uses an
//                  asciiSet bitmap; not SIMD, but tighter inner loop).
//   - SWAR:        process 8 bytes at a time as a uint64 with SWAR masks
//                  for `"`, `\`, and any byte < 0x20. Tail handled with
//                  the byte loop.
//   - Hybrid:      length-dispatched: byte loop for short strings,
//                  SWAR for longer ones. The threshold constant is set
//                  from the sweep results.
//
// The benchmark feeds clean ASCII content so every variant runs to
// completion (no early bail-out from finding a special char). This
// matches the dominant production case.

// escapeSetString contains every byte that hasSpecialChars must report as
// "special": '"', '\\', and the 30 control bytes 0x00..0x1F.
var escapeSetString = func() string {
	b := make([]byte, 0, 32)
	for c := 0; c < 0x20; c++ {
		b = append(b, byte(c))
	}
	b = append(b, '"', '\\')
	return string(b)
}()

// ---------------------------------------------------------------------------
// Variant implementations
// ---------------------------------------------------------------------------

// hasSpecialCharsLoop is the current production implementation, copied
// here so the baseline cannot be changed accidentally by edits to
// parser.go's hasSpecialChars during the experiment.
func hasSpecialCharsLoop(s string) bool {
	for i := 0; i < len(s); i++ {
		if charFlags[s[i]]&charEscape != 0 {
			return true
		}
	}
	return false
}

// hasSpecialCharsIndexAny relies on strings.IndexAny. For ASCII-only
// charsets this uses an asciiSet bitmap with a tight inner loop.
func hasSpecialCharsIndexAny(s string) bool {
	return strings.IndexAny(s, escapeSetString) >= 0
}

const (
	swarLo     uint64 = 0x0101010101010101
	swarHi     uint64 = 0x8080808080808080
	swarQuote  uint64 = 0x2222222222222222 // '"' broadcast
	swarBSlash uint64 = 0x5C5C5C5C5C5C5C5C // '\\' broadcast
	swar0x20   uint64 = 0x2020202020202020
)

// chunkHasSpecial returns true if any of the 8 bytes in v is `"`, `\\`,
// or < 0x20. Pure SWAR — no branches.
func chunkHasSpecial(v uint64) bool {
	// Bytes < 0x20: ((v - 0x20*lo) & ^v) & hi
	ctrl := (v - swar0x20) & ^v
	// Bytes == '"': hasZeroByte(v ^ quote)
	q := v ^ swarQuote
	qmask := (q - swarLo) & ^q
	// Bytes == '\\': hasZeroByte(v ^ bslash)
	sl := v ^ swarBSlash
	smask := (sl - swarLo) & ^sl
	return (ctrl|qmask|smask)&swarHi != 0
}

// load64 reads 8 little-endian bytes from s starting at i. The caller
// must ensure i+8 <= len(s).
func load64(s string, i int) uint64 {
	// Manual little-endian load avoids dependence on unsafe and lets the
	// compiler emit a single MOV on amd64/arm64.
	_ = s[i+7]
	return uint64(s[i]) |
		uint64(s[i+1])<<8 |
		uint64(s[i+2])<<16 |
		uint64(s[i+3])<<24 |
		uint64(s[i+4])<<32 |
		uint64(s[i+5])<<40 |
		uint64(s[i+6])<<48 |
		uint64(s[i+7])<<56
}

// hasSpecialCharsSWAR processes the string 8 bytes at a time using SWAR
// masks, then handles the tail with the byte loop.
func hasSpecialCharsSWAR(s string) bool {
	i := 0
	for i+8 <= len(s) {
		if chunkHasSpecial(load64(s, i)) {
			return true
		}
		i += 8
	}
	for ; i < len(s); i++ {
		if charFlags[s[i]]&charEscape != 0 {
			return true
		}
	}
	return false
}

// hybridThreshold is the length below which the hybrid variant uses the
// byte loop. Set from sweep results — see the BenchmarkHasSpecialSweep
// numbers in this file's commit message.
const hybridThreshold = 8

// hasSpecialCharsHybrid dispatches by length: short strings use the
// byte loop (fewer setup costs), longer strings use SWAR.
func hasSpecialCharsHybrid(s string) bool {
	if len(s) < hybridThreshold {
		for i := 0; i < len(s); i++ {
			if charFlags[s[i]]&charEscape != 0 {
				return true
			}
		}
		return false
	}
	return hasSpecialCharsSWAR(s)
}

// ---------------------------------------------------------------------------
// Correctness check — make sure the variants agree before benchmarking.
// ---------------------------------------------------------------------------

func TestHasSpecialCharsVariantsAgree(t *testing.T) {
	inputs := []string{
		"",
		"a",
		"abc",
		"abcdefghijklmnop",              // 16 bytes, clean
		strings.Repeat("abcdefgh", 8),   // 64 bytes, clean
		strings.Repeat("abcdefgh", 128), // 1024 bytes, clean
		`"`,
		`\`,
		"\x00",
		"\x1f",
		"abcd\"efgh",
		"abcdefgh\\ijkl",
		strings.Repeat("a", 100) + "\x05",
		strings.Repeat("a", 7) + "\"",       // tail of 8-byte chunk
		strings.Repeat("a", 8) + "\\",       // first byte of next chunk
		"\x20" + strings.Repeat("a", 100),   // 0x20 must NOT count
		strings.Repeat("a", 50) + "\x20abc", // ditto, mid-string
	}
	for _, in := range inputs {
		want := hasSpecialCharsLoop(in)
		if got := hasSpecialCharsIndexAny(in); got != want {
			t.Errorf("IndexAny(%q) = %v, want %v", in, got, want)
		}
		if got := hasSpecialCharsSWAR(in); got != want {
			t.Errorf("SWAR(%q) = %v, want %v", in, got, want)
		}
		if got := hasSpecialCharsHybrid(in); got != want {
			t.Errorf("Hybrid(%q) = %v, want %v", in, got, want)
		}
	}
}

// ---------------------------------------------------------------------------
// Sweep benchmark — finds the crossover point.
// ---------------------------------------------------------------------------

// benchSizes covers the range from "tiny key" to "long string body".
var benchSizes = []int{1, 2, 4, 8, 12, 16, 20, 24, 32, 48, 64, 96, 128, 256, 512, 1024, 4096}

// makeClean returns an n-byte ASCII string with no special characters.
// Content is deterministic so the inner-loop branch predictor sees the
// same data each iteration.
func makeClean(n int) string {
	const alphabet = "abcdefghijklmnopqrstuvwxyz0123456789"
	var b strings.Builder
	b.Grow(n)
	for i := 0; i < n; i++ {
		b.WriteByte(alphabet[i%len(alphabet)])
	}
	return b.String()
}

func BenchmarkHasSpecialSweep(b *testing.B) {
	variants := []struct {
		name string
		fn   func(string) bool
	}{
		{"Loop", hasSpecialCharsLoop},
		{"IndexAny", hasSpecialCharsIndexAny},
		{"SWAR", hasSpecialCharsSWAR},
		{"Hybrid", hasSpecialCharsHybrid},
	}
	for _, n := range benchSizes {
		s := makeClean(n)
		for _, v := range variants {
			b.Run(fmt.Sprintf("len=%d/%s", n, v.name), func(b *testing.B) {
				b.SetBytes(int64(n))
				for i := 0; i < b.N; i++ {
					sinkBool = v.fn(s)
				}
			})
		}
	}
}
