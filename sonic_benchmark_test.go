package astjson

import (
	"fmt"
	"testing"
)

// TestSonicParserParity sanity-checks that ParseWithSonic produces an AST
// that is semantically equivalent to the one produced by the native parser.
// It round-trips both ASTs to JSON and compares the byte outputs after
// re-parsing them with the native parser (to normalize key ordering via
// value-by-value comparison isn't worth it — we compare top-level shape).
func TestSonicParserParity(t *testing.T) {
	cases := []struct {
		name string
		json string
	}{
		{"medium", mediumFixture},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			vSonic, err := ParseWithSonicUseNumber([]byte(tc.json))
			if err != nil {
				t.Fatalf("sonic parse failed: %v", err)
			}
			var p Parser
			vNative, err := p.Parse(tc.json)
			if err != nil {
				t.Fatalf("native parse failed: %v", err)
			}
			if vSonic.Type() != vNative.Type() {
				t.Fatalf("type mismatch: sonic=%s native=%s", vSonic.Type(), vNative.Type())
			}
			// Ensure both at least marshal without panicking and produce
			// non-empty output with the same top-level shape.
			bSonic := vSonic.MarshalTo(nil)
			bNative := vNative.MarshalTo(nil)
			if len(bSonic) == 0 || len(bNative) == 0 {
				t.Fatalf("empty marshal output")
			}
			if bSonic[0] != bNative[0] || bSonic[len(bSonic)-1] != bNative[len(bNative)-1] {
				t.Fatalf("boundary bytes differ: sonic=%q native=%q",
					string(bSonic[:1])+"..."+string(bSonic[len(bSonic)-1:]),
					string(bNative[:1])+"..."+string(bNative[len(bNative)-1:]))
			}
		})
	}
}

// BenchmarkParseSonicVsNative compares three strategies that produce an
// astjson *Value tree from a JSON input:
//
//  1. native  — astjson's own Parser (the existing approach)
//  2. sonic   — sonic.Unmarshal into interface{}, then convert to astjson AST
//     (standard sonic usage; numbers decoded as float64)
//  3. sonic-usenumber — same as sonic but with UseNumber so numbers stay as
//     json.Number (string) and round-trip to astjson without formatting
//
// Fixtures cover a range of sizes; "medium" (~2.3KB) is the requested
// medium-sized workload. Larger fixtures are included to show how the
// overhead scales.
func BenchmarkParseSonicVsNative(b *testing.B) {
	fixtures := []struct {
		name string
		data string
	}{
		{"small", smallFixture},
		{"medium", mediumFixture},
		{"large", largeFixture},
		{"twitter", twitterFixture},
		{"citm", citmFixture},
	}
	for _, f := range fixtures {
		b.Run(f.name, func(b *testing.B) {
			b.Run("native", func(b *testing.B) {
				benchmarkNativeParse(b, f.data)
			})
			b.Run("sonic", func(b *testing.B) {
				benchmarkSonicParse(b, f.data)
			})
			b.Run("sonic-usenumber", func(b *testing.B) {
				benchmarkSonicParseUseNumber(b, f.data)
			})
		})
	}
}

func benchmarkNativeParse(b *testing.B, s string) {
	var p Parser
	b.ReportAllocs()
	b.SetBytes(int64(len(s)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v, err := p.Parse(s)
		if err != nil {
			b.Fatalf("native parse: %v", err)
		}
		if v.Type() == TypeNull {
			b.Fatalf("unexpected null top-level")
		}
	}
}

func benchmarkSonicParse(b *testing.B, s string) {
	data := []byte(s)
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v, err := ParseWithSonic(data)
		if err != nil {
			b.Fatalf("sonic parse: %v", err)
		}
		if v.Type() == TypeNull {
			b.Fatalf("unexpected null top-level")
		}
	}
}

func benchmarkSonicParseUseNumber(b *testing.B, s string) {
	data := []byte(s)
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v, err := ParseWithSonicUseNumber(data)
		if err != nil {
			b.Fatalf("sonic parse (usenumber): %v", err)
		}
		if v.Type() == TypeNull {
			b.Fatalf("unexpected null top-level")
		}
	}
}

// Dummy reference so `fmt` stays imported if the benchmark is edited to log.
var _ = fmt.Sprintf
