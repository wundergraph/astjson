package astjson

import (
	"strings"
	"testing"

	"github.com/wundergraph/go-arena"
)

// This file adds benchmark coverage for areas not covered by
// parser_timing_test.go / parser_test.go so that we can measure targeted
// optimizations driven by pprof.
//
// Existing benchmarks (not duplicated here):
//   BenchmarkParse, BenchmarkParseArena, BenchmarkParseArenaAndGet
//   BenchmarkParseRawString, BenchmarkParseRawNumber
//   BenchmarkObjectGet, BenchmarkMarshalTo, BenchmarkValidate
//   BenchmarkDeepCopy, BenchmarkStructuralCopy[WithTransform]
//   BenchmarkStringBytes, BenchmarkMarshalStringValue, BenchmarkMarshalObjectKey
//   BenchmarkParseComparison

var (
	benchFixtureBunch = getFromFile("testdata/bunchFields.json")
	sinkString        string
	sinkBytes         []byte
	sinkBool          bool
	sinkValue         *Value
)

type benchFixture struct {
	name string
	data string
}

func allFixtures() []benchFixture {
	return []benchFixture{
		{"small", smallFixture},
		{"medium", mediumFixture},
		{"large", largeFixture},
		{"canada", canadaFixture},
		{"citm", citmFixture},
		{"twitter", twitterFixture},
	}
}

// ---------------------------------------------------------------------------
// Parse: sweep across all fixtures (heap)
// ---------------------------------------------------------------------------

func BenchmarkParseAll(b *testing.B) {
	for _, f := range allFixtures() {
		b.Run(f.name, func(b *testing.B) {
			var p Parser
			b.ReportAllocs()
			b.SetBytes(int64(len(f.data)))
			for i := 0; i < b.N; i++ {
				v, err := p.Parse(f.data)
				if err != nil {
					b.Fatal(err)
				}
				sinkValue = v
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Parse: sweep across all fixtures (arena / two-pass)
// ---------------------------------------------------------------------------

func BenchmarkParseArenaAll(b *testing.B) {
	for _, f := range allFixtures() {
		b.Run(f.name, func(b *testing.B) {
			var p Parser
			a := arena.NewMonotonicArena(arena.WithMinBufferSize(2 * 1024 * 1024))
			b.ReportAllocs()
			b.SetBytes(int64(len(f.data)))
			for i := 0; i < b.N; i++ {
				v, err := p.ParseWithArena(a, f.data)
				if err != nil {
					b.Fatal(err)
				}
				sinkValue = v
				a.Reset()
			}
		})
	}
}

// ---------------------------------------------------------------------------
// skipWS
// ---------------------------------------------------------------------------

func BenchmarkSkipWS(b *testing.B) {
	cases := []struct {
		name string
		s    string
	}{
		{"none", `{"key":1}`},
		{"short", `   {"key":1}`},
		{"long", strings.Repeat(" ", 256) + `{"key":1}`},
		{"mixed", "\t\n\r " + `{"key":1}`},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(tc.s)))
			for i := 0; i < b.N; i++ {
				sinkString = skipWS(tc.s)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// parseRawKey
// ---------------------------------------------------------------------------

func BenchmarkParseRawKey(b *testing.B) {
	cases := []struct {
		name string
		s    string
	}{
		{"simple", `username"`},
		{"medium", `created_at_timestamp_utc"`},
		{"long", strings.Repeat("a", 100) + `"`},
		{"with_escape", `user\"name"`},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(tc.s)))
			for i := 0; i < b.N; i++ {
				rs, _, _, err := parseRawKey(tc.s)
				if err != nil {
					b.Fatal(err)
				}
				sinkString = rs
			}
		})
	}
}

// ---------------------------------------------------------------------------
// unescapeStringBestEffortInfo
// ---------------------------------------------------------------------------

func BenchmarkUnescapeStringBestEffortInfo(b *testing.B) {
	cases := []struct {
		name string
		s    string
	}{
		{"no_escape_short", "hello world"},
		{"no_escape_long", strings.Repeat("plain ascii payload ", 20)},
		{"simple_escape", `hello\nworld\ttab\"quote`},
		{"unicode_escape", `\u0048\u0065\u006C\u006C\u006F world`},
		{"surrogate_pair", `\uD83D\uDE00 smile \uD83D\uDE00`},
	}
	for _, tc := range cases {
		b.Run("heap/"+tc.name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(tc.s)))
			for i := 0; i < b.N; i++ {
				s, ok := unescapeStringBestEffortInfo(nil, tc.s)
				sinkString = s
				sinkBool = ok
			}
		})
		b.Run("arena/"+tc.name, func(b *testing.B) {
			a := arena.NewMonotonicArena(arena.WithMinBufferSize(4096))
			b.ReportAllocs()
			b.SetBytes(int64(len(tc.s)))
			for i := 0; i < b.N; i++ {
				s, ok := unescapeStringBestEffortInfo(a, tc.s)
				sinkString = s
				sinkBool = ok
				a.Reset()
			}
		})
	}
}

// ---------------------------------------------------------------------------
// hasSpecialChars / escapeStringSlowPath
// ---------------------------------------------------------------------------

func BenchmarkHasSpecialChars(b *testing.B) {
	cases := []struct {
		name string
		s    string
	}{
		{"plain_short", "hello world"},
		{"plain_long", strings.Repeat("abcdef012345", 20)},
		{"has_quote", `hello "world"`},
		{"has_control", "hello\nworld\ttab"},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(tc.s)))
			for i := 0; i < b.N; i++ {
				sinkBool = hasSpecialChars(tc.s)
			}
		})
	}
}

func BenchmarkEscapeStringSlowPath(b *testing.B) {
	cases := []struct {
		name string
		s    string
	}{
		{"with_quotes", `hello "world" said "foo"`},
		{"with_control", "hello\nworld\ttab\rreturn"},
		{"mixed", "he said \"hi\"\nbye\\done"},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			dst := make([]byte, 0, 256)
			b.ReportAllocs()
			b.SetBytes(int64(len(tc.s)))
			for i := 0; i < b.N; i++ {
				sinkBytes = escapeStringSlowPath(dst[:0], tc.s)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// MergeValues — the most important missing coverage
// ---------------------------------------------------------------------------

func BenchmarkMergeValues(b *testing.B) {
	// Identical objects – common case, should be fast no-op merges.
	b.Run("identical/small", func(b *testing.B) {
		benchmarkMergeValues(b, smallFixture, smallFixture)
	})
	b.Run("identical/medium", func(b *testing.B) {
		benchmarkMergeValues(b, mediumFixture, mediumFixture)
	})
	b.Run("identical/twitter", func(b *testing.B) {
		benchmarkMergeValues(b, twitterFixture, twitterFixture)
	})

	// Shallow scalar merges.
	b.Run("numbers_equal", func(b *testing.B) {
		benchmarkMergeValues(b, `1.25`, `1.25`)
	})
	b.Run("numbers_diff", func(b *testing.B) {
		benchmarkMergeValues(b, `1.25`, `1.26`)
	})
	b.Run("numbers_equal_repr_diff", func(b *testing.B) {
		benchmarkMergeValues(b, `1.0`, `1.00`)
	})
	b.Run("strings_equal", func(b *testing.B) {
		benchmarkMergeValues(b, `"hello world"`, `"hello world"`)
	})
	b.Run("strings_diff", func(b *testing.B) {
		benchmarkMergeValues(b, `"hello world"`, `"hello earth"`)
	})
	b.Run("bools_equal", func(b *testing.B) {
		benchmarkMergeValues(b, `true`, `true`)
	})
	b.Run("bools_diff", func(b *testing.B) {
		benchmarkMergeValues(b, `true`, `false`)
	})
}

func benchmarkMergeValues(b *testing.B, aStr, bStr string) {
	b.ReportAllocs()
	// One parse per call is the steady state – both halves fresh each iter.
	var pa, pb2 Parser
	for i := 0; i < b.N; i++ {
		av, err := pa.Parse(aStr)
		if err != nil {
			b.Fatal(err)
		}
		bv, err := pb2.Parse(bStr)
		if err != nil {
			b.Fatal(err)
		}
		merged, err := MergeValues(nil, av, bv)
		if err != nil {
			b.Fatal(err)
		}
		sinkValue = merged
	}
}

// MergeValuesWithChanges exercises the path where the merge actually mutates
// fields in an existing key — targeting the direct-kv-mutation path.
func BenchmarkMergeValuesWithChanges(b *testing.B) {
	// Build a 50-key object where every value differs between a and b.
	var sb strings.Builder
	sb.WriteByte('{')
	for i := 0; i < 50; i++ {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(`"key_`)
		sb.WriteString(intToStr(i))
		sb.WriteString(`":`)
		sb.WriteString(intToStr(i))
	}
	sb.WriteByte('}')
	aStr := sb.String()
	sb.Reset()
	sb.WriteByte('{')
	for i := 0; i < 50; i++ {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(`"key_`)
		sb.WriteString(intToStr(i))
		sb.WriteString(`":`)
		sb.WriteString(intToStr(i + 1000)) // different value
	}
	sb.WriteByte('}')
	bStr := sb.String()

	b.Run("50keys_all_different", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			av := MustParse(aStr)
			bv := MustParse(bStr)
			merged, err := MergeValues(nil, av, bv)
			if err != nil {
				b.Fatal(err)
			}
			sinkValue = merged
		}
	})
}

func intToStr(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [12]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

// MergeValuesOnly isolates the merge cost by parsing once up front.
func BenchmarkMergeValuesOnly(b *testing.B) {
	cases := []struct {
		name string
		a, b string
	}{
		{"strings_equal", `"hello world"`, `"hello world"`},
		{"strings_diff", `"hello world"`, `"hello earth"`},
		{"numbers_equal", `1.25`, `1.25`},
		{"numbers_equal_repr_diff", `1.0`, `1.00`},
		{"numbers_diff", `1.25`, `1.26`},
		{"bools_equal", `true`, `true`},
		{"identical_small", smallFixture, smallFixture},
		{"identical_medium", mediumFixture, mediumFixture},
		{"identical_twitter", twitterFixture, twitterFixture},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			// MergeValues mutates its left-hand input for object/array cases,
			// so re-parse fresh inputs each iteration (excluded from timing)
			// to keep the benchmark measuring only the merge cost.
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				av := MustParse(tc.a)
				bv := MustParse(tc.b)
				b.StartTimer()
				merged, err := MergeValues(nil, av, bv)
				if err != nil {
					b.Fatal(err)
				}
				sinkValue = merged
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Object.Get on big objects (motivates kvIndex experiment)
// ---------------------------------------------------------------------------

func BenchmarkObjectGetBunchFields(b *testing.B) {
	var p Parser
	v, err := p.Parse(benchFixtureBunch)
	if err != nil {
		b.Fatal(err)
	}
	o := v.GetObject()
	if o == nil {
		b.Fatal("expected object")
	}
	if len(o.kvs) == 0 {
		b.Fatal("expected object with at least one key")
	}
	// Pick a key from the middle of the object.
	middleKey := o.kvs[len(o.kvs)/2].k
	lastKey := o.kvs[len(o.kvs)-1].k
	firstKey := o.kvs[0].k

	b.Run("first", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			sinkValue = o.Get(firstKey)
		}
	})
	b.Run("middle", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			sinkValue = o.Get(middleKey)
		}
	})
	b.Run("last", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			sinkValue = o.Get(lastKey)
		}
	})
	b.Run("miss", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			sinkValue = o.Get("does_not_exist_key_name")
		}
	})
}
