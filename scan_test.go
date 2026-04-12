package astjson

import (
	"strconv"
	"strings"
	"testing"
)

func TestFindQuoteOrBackslash(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"", 0},
		{`"`, 0},
		{`\`, 0},
		{"a", 1},
		{"abc", 3},
		{`abc"`, 3},
		{`abc\`, 3},
		{`abc"def`, 3},
		{`abc\def`, 3},
		{`"abc`, 0},
		{`\abc`, 0},
		// Boundary tests around 16-byte SIMD blocks
		{strings.Repeat("a", 15) + `"`, 15},
		{strings.Repeat("a", 15) + `\`, 15},
		{strings.Repeat("a", 16) + `"`, 16},
		{strings.Repeat("a", 16) + `\`, 16},
		{strings.Repeat("a", 17) + `"`, 17},
		{strings.Repeat("a", 17) + `\`, 17},
		{strings.Repeat("a", 31) + `"`, 31},
		{strings.Repeat("a", 32) + `"`, 32},
		{strings.Repeat("a", 33) + `\`, 33},
		{strings.Repeat("a", 63) + `"`, 63},
		{strings.Repeat("a", 64) + `\`, 64},
		{strings.Repeat("a", 100) + `"`, 100},
		{strings.Repeat("a", 255) + `\`, 255},
		{strings.Repeat("a", 256) + `"`, 256},
		// No match
		{strings.Repeat("a", 1000), 1000},
		// Match at position 0 in longer string
		{`"` + strings.Repeat("a", 100), 0},
		{`\` + strings.Repeat("a", 100), 0},
		// Multiple candidates - should find first
		{`abc"def\ghi`, 3},
		{`abc\def"ghi`, 3},
	}
	for _, tt := range tests {
		got := findQuoteOrBackslash(tt.input)
		if got != tt.want {
			t.Errorf("findQuoteOrBackslash(len=%d) = %d, want %d", len(tt.input), got, tt.want)
		}
	}
}

func TestCountWhitespace(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"", 0},
		{"a", 0},
		{" ", 1},
		{"\t", 1},
		{"\n", 1},
		{"\r", 1},
		{"  ", 2},
		{"  a", 2},
		{" \t\n\r", 4},
		{" \t\n\ra", 4},
		{"abc", 0},
		// Boundary tests
		{strings.Repeat(" ", 15) + "a", 15},
		{strings.Repeat(" ", 16) + "a", 16},
		{strings.Repeat(" ", 17) + "a", 17},
		{strings.Repeat(" ", 31) + "a", 31},
		{strings.Repeat(" ", 32) + "a", 32},
		{strings.Repeat(" ", 33) + "a", 33},
		{strings.Repeat(" ", 100) + "a", 100},
		// All whitespace
		{strings.Repeat(" ", 100), 100},
		{strings.Repeat("\t", 50), 50},
		// Mixed whitespace
		{" \t \n \r " + "x", 7},
		{strings.Repeat(" \t\n\r", 10) + "x", 40},
	}
	for _, tt := range tests {
		got := countWhitespace(tt.input)
		if got != tt.want {
			t.Errorf("countWhitespace(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestFindNonNumber(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"", 0},
		{"1", 1},
		{"123", 3},
		{"1234567890", 10},
		{"123.456", 7},
		{"-123", 4},
		{"+123", 4},
		{"1e10", 4},
		{"1E10", 4},
		{"-1.23e+10", 9},
		{"1.23E-10", 8},
		// NaN/Inf letters are NOT recognized by findNonNumber;
		// they are handled by parseRawNumber's special-case logic.
		{"NaN", 0},
		{"Infinity", 0},
		{"inf", 0},
		// Non-number characters
		{"abc", 0},
		{"1,2", 1},
		{"1 2", 1},
		{"1]", 1},
		{"1}", 1},
		{"1:2", 1},
		{" 1", 0},
		// Longer numbers
		{strings.Repeat("1", 50) + " ", 50},
	}
	for _, tt := range tests {
		got := findNonNumber(tt.input)
		if got != tt.want {
			t.Errorf("findNonNumber(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

// Benchmarks for the scan primitives
func BenchmarkFindQuoteOrBackslash(b *testing.B) {
	for _, size := range []int{4, 8, 16, 32, 64, 128, 256, 1024} {
		s := strings.Repeat("a", size-1) + `"`
		name := "len_" + strconv.Itoa(size)
		b.Run(name, func(b *testing.B) {
			b.SetBytes(int64(len(s)))
			for i := 0; i < b.N; i++ {
				findQuoteOrBackslash(s)
			}
		})
	}
}

func BenchmarkCountWhitespace(b *testing.B) {
	for _, size := range []int{4, 8, 16, 32, 64, 128, 256, 1024} {
		s := strings.Repeat(" ", size-1) + "a"
		name := "len_" + strconv.Itoa(size)
		b.Run(name, func(b *testing.B) {
			b.SetBytes(int64(len(s)))
			for i := 0; i < b.N; i++ {
				countWhitespace(s)
			}
		})
	}
}

func BenchmarkFindNonNumber(b *testing.B) {
	for _, size := range []int{4, 8, 16, 32} {
		s := strings.Repeat("1", size-1) + " "
		name := "len_" + strconv.Itoa(size)
		b.Run(name, func(b *testing.B) {
			b.SetBytes(int64(len(s)))
			for i := 0; i < b.N; i++ {
				findNonNumber(s)
			}
		})
	}
}
