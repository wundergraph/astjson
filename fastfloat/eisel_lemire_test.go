package fastfloat

import (
	"math"
	"strconv"
	"testing"
)

func TestEiselLemire64(t *testing.T) {
	tests := []struct {
		s    string
		want float64
	}{
		{"0", 0},
		{"1", 1},
		{"-1", -1},
		{"1.5", 1.5},
		{"1.23456789", 1.23456789},
		{"123456789.123456789", 123456789.123456789},
		{"1e10", 1e10},
		{"1e-10", 1e-10},
		{"1.23e15", 1.23e15},
		{"-1.23e-15", -1.23e-15},
		{"3.141592653589793", 3.141592653589793},
		{"2.2250738585072014e-308", 2.2250738585072014e-308}, // smallest normal
		{"1.7976931348623157e308", 1.7976931348623157e308},   // largest
	}
	for _, tt := range tests {
		got, err := Parse(tt.s)
		if err != nil {
			t.Errorf("Parse(%q) error: %v", tt.s, err)
			continue
		}
		want := tt.want
		if got != want {
			t.Errorf("Parse(%q) = %v, want %v (bits: got %016x, want %016x)",
				tt.s, got, want, math.Float64bits(got), math.Float64bits(want))
		}
	}
}

func TestEiselLemire64Direct(t *testing.T) {
	// Test the eiselLemire64 function directly.
	tests := []struct {
		neg      bool
		mantissa uint64
		exp10    int
		want     float64
		wantOK   bool
	}{
		{false, 0, 0, 0, true},
		{true, 0, 0, math.Float64frombits(0x8000000000000000), true}, // -0
		{false, 1, 0, 1.0, true},
		{true, 1, 0, -1.0, true},
		{false, 15, -1, 1.5, false},  // ambiguous for Eisel-Lemire, needs fallback
		{false, 5, 0, 5.0, true},
		{false, 1, 10, 1e10, true},
		{false, 1, -10, 1e-10, true},
		{false, 123, 13, 1.23e15, true},
		{true, 123, -17, -1.23e-15, true},
		{false, 3141592653589793, -15, 3.141592653589793, true},
		{false, 22250738585072014, -324, 2.2250738585072014e-308, true}, // smallest normal
		{false, 17976931348623157, 292, 1.7976931348623157e308, true},   // largest
	}
	for _, tt := range tests {
		got, ok := eiselLemire64(tt.neg, tt.mantissa, tt.exp10)
		if ok != tt.wantOK {
			t.Errorf("eiselLemire64(%v, %d, %d) ok = %v, want %v",
				tt.neg, tt.mantissa, tt.exp10, ok, tt.wantOK)
			continue
		}
		if ok && got != tt.want {
			t.Errorf("eiselLemire64(%v, %d, %d) = %v, want %v (bits: got %016x, want %016x)",
				tt.neg, tt.mantissa, tt.exp10, got, tt.want,
				math.Float64bits(got), math.Float64bits(tt.want))
		}
	}
}

func TestEiselLemire64OutOfRange(t *testing.T) {
	// These should return false (fallback needed).
	tests := []struct {
		mantissa uint64
		exp10    int
	}{
		{1, -400},  // below min exp10
		{1, 400},   // above max exp10
		{1, -349},  // just below min exp10
		{1, 348},   // just above max exp10
	}
	for _, tt := range tests {
		_, ok := eiselLemire64(false, tt.mantissa, tt.exp10)
		if ok {
			t.Errorf("eiselLemire64(false, %d, %d) should have returned false", tt.mantissa, tt.exp10)
		}
	}
}

func BenchmarkParseFloat(b *testing.B) {
	numbers := []string{
		"1", "123", "123456", "123.456",
		"1234567890.1234567", "-1.32434e+12",
		"3.141592653589793", "2.2250738585072014e-308",
	}
	for _, s := range numbers {
		b.Run("fastfloat/"+s, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				Parse(s)
			}
		})
		b.Run("strconv/"+s, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				strconv.ParseFloat(s, 64)
			}
		})
	}
}
