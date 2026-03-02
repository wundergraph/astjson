package astjson

import (
	"strings"
	"testing"

	"github.com/wundergraph/go-arena"
)

// Sink vars prevent dead-code elimination by the compiler.
var (
	sinkValue   *Value
	sinkBytes   []byte
	sinkString  string
	sinkInt     int
	sinkFloat64 float64
	sinkBool    bool
	sinkErr     error
)

// fixture pairs a name with its JSON data for table-driven benchmarks.
type fixture struct {
	name string
	data string
}

// fixtures is the default set (excludes 20mb for speed).
var fixtures = []fixture{
	{"small", smallFixture},
	{"medium", mediumFixture},
	{"large", largeFixture},
	{"canada", canadaFixture},
	{"citm", citmFixture},
	{"twitter", twitterFixture},
}

// bunchFieldsFixture is an 871-key object for large-object benchmarks.
var bunchFieldsFixture = getFromFile("testdata/bunchFields.json")

// ---------------------------------------------------------------------------
// Section 1: Parsing
// ---------------------------------------------------------------------------

func BenchmarkSTParse(b *testing.B) {
	for _, f := range fixtures {
		b.Run(f.name, func(b *testing.B) {
			benchmarkSTParse(b, f.data)
		})
	}
	b.Run("20mb", func(b *testing.B) {
		benchmarkSTParse(b, huge20MbFixture)
	})
}

func benchmarkSTParse(b *testing.B, data string) {
	var p Parser
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for b.Loop() {
		v, err := p.Parse(data)
		if err != nil {
			b.Fatal(err)
		}
		sinkValue = v
	}
}

func BenchmarkSTParseArena(b *testing.B) {
	for _, f := range fixtures {
		b.Run(f.name, func(b *testing.B) {
			benchmarkSTParseArena(b, f.data, 2*1024*1024)
		})
	}
	b.Run("20mb", func(b *testing.B) {
		benchmarkSTParseArena(b, huge20MbFixture, 32*1024*1024)
	})
}

func benchmarkSTParseArena(b *testing.B, data string, arenaSize int) {
	var p Parser
	a := arena.NewMonotonicArena(arena.WithMinBufferSize(arenaSize))
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for b.Loop() {
		v, err := p.ParseWithArena(a, data)
		if err != nil {
			b.Fatal(err)
		}
		sinkValue = v
		a.Reset()
	}
}

func BenchmarkSTParseBytes(b *testing.B) {
	for _, name := range []string{"small", "medium", "large", "twitter"} {
		var data string
		for _, f := range fixtures {
			if f.name == name {
				data = f.data
				break
			}
		}
		b.Run(name, func(b *testing.B) {
			var p Parser
			bb := []byte(data)
			b.ReportAllocs()
			b.SetBytes(int64(len(bb)))
			b.ResetTimer()
			for b.Loop() {
				v, err := p.ParseBytes(bb)
				if err != nil {
					b.Fatal(err)
				}
				sinkValue = v
			}
		})
	}
}

func BenchmarkSTParseBytesArena(b *testing.B) {
	for _, name := range []string{"small", "medium", "large", "twitter"} {
		var data string
		for _, f := range fixtures {
			if f.name == name {
				data = f.data
				break
			}
		}
		b.Run(name, func(b *testing.B) {
			var p Parser
			a := arena.NewMonotonicArena(arena.WithMinBufferSize(2 * 1024 * 1024))
			bb := []byte(data)
			b.ReportAllocs()
			b.SetBytes(int64(len(bb)))
			b.ResetTimer()
			for b.Loop() {
				v, err := p.ParseBytesWithArena(a, bb)
				if err != nil {
					b.Fatal(err)
				}
				sinkValue = v
				a.Reset()
			}
		})
	}
}

func BenchmarkSTParseRawString(b *testing.B) {
	cases := []struct {
		name string
		s    string // includes opening quote already stripped
	}{
		{"empty", `"`},
		{"short", `hello"`},
		{"medium", `abcdefghijklmnopqrstuvwxyz012345678901234567890123"`},
		{"with_escape", `hello\"world\\nfoo"`},
		{"unicode", `\u0048\u0065\u006C\u006C\u006F"`},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(tc.s)))
			b.ResetTimer()
			for b.Loop() {
				rs, _, err := parseRawString(tc.s)
				if err != nil {
					b.Fatal(err)
				}
				sinkString = rs
			}
		})
	}
}

func BenchmarkSTParseRawNumber(b *testing.B) {
	cases := []struct {
		name string
		s    string
	}{
		{"int", "12345,"},
		{"float", "123.456,"},
		{"exp", "123.456e+78,"},
		{"negative", "-12345.6789,"},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(tc.s)))
			b.ResetTimer()
			for b.Loop() {
				rn, _, err := parseRawNumber(tc.s)
				if err != nil {
					b.Fatal(err)
				}
				sinkString = rn
			}
		})
	}
}

func BenchmarkSTParseRawKey(b *testing.B) {
	cases := []struct {
		name string
		s    string // after the opening quote
	}{
		{"simple", `username"`},
		{"long", strings.Repeat("a", 100) + `"`},
		{"with_escape", `user\"name"`},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(tc.s)))
			b.ResetTimer()
			for b.Loop() {
				k, _, err := parseRawKey(tc.s)
				if err != nil {
					b.Fatal(err)
				}
				sinkString = k
			}
		})
	}
}

func BenchmarkSTSkipWS(b *testing.B) {
	cases := []struct {
		name string
		s    string
	}{
		{"none", `{"key": 1}`},
		{"short", `   {"key": 1}`},
		{"long", strings.Repeat(" ", 256) + `{"key": 1}`},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(tc.s)))
			b.ResetTimer()
			for b.Loop() {
				sinkString = skipWS(tc.s)
			}
		})
	}
}

func BenchmarkSTUnescapeStringBestEffort(b *testing.B) {
	cases := []struct {
		name string
		s    string
	}{
		{"no_escape", "hello world plain text"},
		{"simple_escape", `hello\nworld\ttab`},
		{"unicode_escape", `\u0048\u0065\u006C\u006C\u006F`},
		{"surrogate_pair", `\uD83D\uDE00 smile`},
	}
	for _, tc := range cases {
		b.Run("heap/"+tc.name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(tc.s)))
			b.ResetTimer()
			for b.Loop() {
				sinkString = unescapeStringBestEffort(nil, tc.s)
			}
		})
		b.Run("arena/"+tc.name, func(b *testing.B) {
			a := arena.NewMonotonicArena(arena.WithMinBufferSize(4096))
			b.ReportAllocs()
			b.SetBytes(int64(len(tc.s)))
			b.ResetTimer()
			for b.Loop() {
				sinkString = unescapeStringBestEffort(a, tc.s)
				a.Reset()
			}
		})
	}
}

func BenchmarkSTEscapeString(b *testing.B) {
	cases := []struct {
		name string
		s    string
	}{
		{"no_special", "hello world plain text no special chars here"},
		{"with_quotes", `hello "world" said "foo"`},
		{"with_control", "hello\nworld\ttab\rreturn"},
		{"mixed", "he said \"hi\"\nbye\\done"},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			dst := make([]byte, 0, 256)
			b.ReportAllocs()
			b.SetBytes(int64(len(tc.s)))
			b.ResetTimer()
			for b.Loop() {
				sinkBytes = escapeString(dst[:0], tc.s)
			}
		})
	}
}

func BenchmarkSTHasSpecialChars(b *testing.B) {
	plain100 := strings.Repeat("abcdefghij", 10)
	cases := []struct {
		name string
		s    string
	}{
		{"none_short", "hello"},
		{"none_long", plain100},
		{"early_hit", `ab"cd`},
		{"late_hit", plain100[:99] + `"`},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(tc.s)))
			b.ResetTimer()
			for b.Loop() {
				sinkBool = hasSpecialChars(tc.s)
			}
		})
	}
}

func BenchmarkSTParseInlineSmall(b *testing.B) {
	cases := []struct {
		name string
		s    string
	}{
		{"null", "null"},
		{"true", "true"},
		{"false", "false"},
		{"number", "12345"},
		{"string", `"hello world"`},
		{"empty_object", "{}"},
		{"empty_array", "[]"},
		{"small_object", `{"a":1,"b":"x","c":true}`},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			var p Parser
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				v, err := p.Parse(tc.s)
				if err != nil {
					b.Fatal(err)
				}
				sinkValue = v
			}
		})
	}
}

func BenchmarkSTParseDeepNesting(b *testing.B) {
	// Build 100-level nested object: {"a":{"a":{"a":...1...}}}
	depth := 100
	s := "1"
	for range depth {
		s = `{"a":` + s + `}`
	}
	var p Parser
	b.ReportAllocs()
	b.SetBytes(int64(len(s)))
	b.ResetTimer()
	for b.Loop() {
		v, err := p.Parse(s)
		if err != nil {
			b.Fatal(err)
		}
		sinkValue = v
	}
}

// ---------------------------------------------------------------------------
// Section 2: Value Access
// ---------------------------------------------------------------------------

func BenchmarkSTValueGet(b *testing.B) {
	var p Parser
	v, err := p.Parse(twitterFixture)
	if err != nil {
		b.Fatal(err)
	}

	cases := []struct {
		name string
		keys []string
	}{
		{"shallow", []string{"statuses"}},
		{"deep_2", []string{"statuses", "0"}},
		{"deep_3", []string{"statuses", "0", "user"}},
		{"miss", []string{"nonexistent"}},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				sinkValue = v.Get(tc.keys...)
			}
		})
	}
}

func BenchmarkSTObjectGet(b *testing.B) {
	var p Parser

	// Small object: twitter statuses[0] (~25 keys)
	tv, err := p.Parse(twitterFixture)
	if err != nil {
		b.Fatal(err)
	}
	smallObj := tv.Get("statuses", "0").GetObject()

	// Large object: bunchFields (871 keys)
	bv, err := p.Parse(bunchFieldsFixture)
	if err != nil {
		b.Fatal(err)
	}
	largeObj := bv.GetObject()

	b.Run("small_hit", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			sinkValue = smallObj.Get("user")
		}
	})
	b.Run("small_miss", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			sinkValue = smallObj.Get("nonexistent_key_xyz")
		}
	})
	b.Run("large_first", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			sinkValue = largeObj.Get("4")
		}
	})
	b.Run("large_last", func(b *testing.B) {
		// Get the last key
		var lastKey string
		largeObj.Visit(func(key []byte, v *Value) {
			lastKey = string(key)
		})
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			sinkValue = largeObj.Get(lastKey)
		}
	})
	b.Run("large_miss", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			sinkValue = largeObj.Get("nonexistent_key_xyz")
		}
	})
}

func BenchmarkSTObjectVisit(b *testing.B) {
	var p Parser

	tv, err := p.Parse(twitterFixture)
	if err != nil {
		b.Fatal(err)
	}
	obj := tv.Get("statuses", "0").GetObject()

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		obj.Visit(func(key []byte, v *Value) {
			sinkBytes = key
		})
	}
}

func BenchmarkSTGetStringBytes(b *testing.B) {
	var p Parser
	v, _ := p.Parse(`{"name":"hello world"}`)
	sv := v.Get("name")
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		sb, err := sv.StringBytes()
		if err != nil {
			b.Fatal(err)
		}
		sinkBytes = sb
	}
}

func BenchmarkSTGetInt(b *testing.B) {
	var p Parser
	v, _ := p.Parse(`{"count":12345}`)
	nv := v.Get("count")
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		n, err := nv.Int()
		if err != nil {
			b.Fatal(err)
		}
		sinkInt = n
	}
}

func BenchmarkSTGetFloat64(b *testing.B) {
	var p Parser
	v, _ := p.Parse(`{"price":123.456}`)
	fv := v.Get("price")
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		f, err := fv.Float64()
		if err != nil {
			b.Fatal(err)
		}
		sinkFloat64 = f
	}
}

func BenchmarkSTGetBool(b *testing.B) {
	var p Parser
	v, _ := p.Parse(`{"active":true}`)
	bv := v.Get("active")
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		sinkBool = bv.GetBool()
	}
}

// ---------------------------------------------------------------------------
// Section 3: Merging
// ---------------------------------------------------------------------------

func BenchmarkSTMergeValuesObject(b *testing.B) {
	b.Run("small", func(b *testing.B) {
		a := arena.NewMonotonicArena(arena.WithMinBufferSize(4096))
		var p Parser
		aVal, _ := p.ParseWithArena(a, `{"x":1,"y":2,"z":3}`)
		bVal, _ := p.ParseWithArena(a, `{"y":20,"w":4}`)
		aBytes := []byte(aVal.String())
		bBytes := []byte(bVal.String())
		a.Reset()
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			av, _ := p.ParseBytesWithArena(a, aBytes)
			bv, _ := p.ParseBytesWithArena(a, bBytes)
			v, _, err := MergeValues(a, av, bv)
			if err != nil {
				b.Fatal(err)
			}
			sinkValue = v
			a.Reset()
		}
	})
	b.Run("medium", func(b *testing.B) {
		// Build two 10-key objects with 3 overlapping keys
		a := arena.NewMonotonicArena(arena.WithMinBufferSize(4096))
		var p Parser
		aJSON := `{"a":1,"b":2,"c":3,"d":4,"e":5,"f":6,"g":7,"h":8,"i":9,"j":10}`
		bJSON := `{"h":80,"i":90,"j":100,"k":11,"l":12}`
		aBytes := []byte(aJSON)
		bBytes := []byte(bJSON)
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			av, _ := p.ParseBytesWithArena(a, aBytes)
			bv, _ := p.ParseBytesWithArena(a, bBytes)
			v, _, err := MergeValues(a, av, bv)
			if err != nil {
				b.Fatal(err)
			}
			sinkValue = v
			a.Reset()
		}
	})
	b.Run("large", func(b *testing.B) {
		// Merge two copies of twitter statuses[0] (realistic large object merge)
		a := arena.NewMonotonicArena(arena.WithMinBufferSize(2 * 1024 * 1024))
		var p Parser
		tv, _ := p.Parse(twitterFixture)
		obj := tv.Get("statuses", "0")
		objJSON := obj.String()
		objBytes := []byte(objJSON)
		b.ReportAllocs()
		b.SetBytes(int64(len(objBytes)))
		b.ResetTimer()
		for b.Loop() {
			av, _ := p.ParseBytesWithArena(a, objBytes)
			bv, _ := p.ParseBytesWithArena(a, objBytes)
			v, _, err := MergeValues(a, av, bv)
			if err != nil {
				b.Fatal(err)
			}
			sinkValue = v
			a.Reset()
		}
	})
}

func BenchmarkSTMergeValuesArray(b *testing.B) {
	a := arena.NewMonotonicArena(arena.WithMinBufferSize(4096))
	var p Parser
	aBytes := []byte(`[1,2,3,4,5,6,7,8,9,10]`)
	bBytes := []byte(`[11,12,13,14,15,16,17,18,19,20]`)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		av, _ := p.ParseBytesWithArena(a, aBytes)
		bv, _ := p.ParseBytesWithArena(a, bBytes)
		v, _, err := MergeValues(a, av, bv)
		if err != nil {
			b.Fatal(err)
		}
		sinkValue = v
		a.Reset()
	}
}

func BenchmarkSTMergeValuesScalar(b *testing.B) {
	b.Run("string", func(b *testing.B) {
		var p Parser
		aVal, _ := p.Parse(`"hello"`)
		bVal, _ := p.Parse(`"world"`)
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			v, _, err := MergeValues(nil, aVal, bVal)
			if err != nil {
				b.Fatal(err)
			}
			sinkValue = v
		}
	})
	b.Run("number", func(b *testing.B) {
		var p Parser
		aVal, _ := p.Parse(`123`)
		bVal, _ := p.Parse(`456`)
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			v, _, err := MergeValues(nil, aVal, bVal)
			if err != nil {
				b.Fatal(err)
			}
			sinkValue = v
		}
	})
	b.Run("bool", func(b *testing.B) {
		var p Parser
		aVal, _ := p.Parse(`true`)
		bVal, _ := p.Parse(`false`)
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			v, _, err := MergeValues(nil, aVal, bVal)
			if err != nil {
				b.Fatal(err)
			}
			sinkValue = v
		}
	})
}

func BenchmarkSTMergeValuesWithPath(b *testing.B) {
	b.Run("depth_1", func(b *testing.B) {
		a := arena.NewMonotonicArena(arena.WithMinBufferSize(4096))
		var p Parser
		aBytes := []byte(`{"data":"old_value"}`)
		bBytes := []byte(`"new_value"`)
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			av, _ := p.ParseBytesWithArena(a, aBytes)
			bv, _ := p.ParseBytesWithArena(a, bBytes)
			v, _, err := MergeValuesWithPath(a, av, bv, "data")
			if err != nil {
				b.Fatal(err)
			}
			sinkValue = v
			a.Reset()
		}
	})
	b.Run("depth_3", func(b *testing.B) {
		a := arena.NewMonotonicArena(arena.WithMinBufferSize(4096))
		var p Parser
		aBytes := []byte(`{"data":{"user":{"name":"old"}}}`)
		bBytes := []byte(`"new_name"`)
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			av, _ := p.ParseBytesWithArena(a, aBytes)
			bv, _ := p.ParseBytesWithArena(a, bBytes)
			v, _, err := MergeValuesWithPath(a, av, bv, "data", "user", "name")
			if err != nil {
				b.Fatal(err)
			}
			sinkValue = v
			a.Reset()
		}
	})
}

func BenchmarkSTMergeValuesNested(b *testing.B) {
	// 5 levels of nesting with overlapping keys at each level
	a := arena.NewMonotonicArena(arena.WithMinBufferSize(4096))
	var p Parser
	aBytes := []byte(`{"l1":{"l2":{"l3":{"l4":{"l5":"a_val"},"x":1},"y":2},"z":3},"w":4}`)
	bBytes := []byte(`{"l1":{"l2":{"l3":{"l4":{"l5":"b_val"},"x":10},"y":20},"z":30},"w":40}`)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		av, _ := p.ParseBytesWithArena(a, aBytes)
		bv, _ := p.ParseBytesWithArena(a, bBytes)
		v, _, err := MergeValues(a, av, bv)
		if err != nil {
			b.Fatal(err)
		}
		sinkValue = v
		a.Reset()
	}
}

// ---------------------------------------------------------------------------
// Section 4: Value Creation
// ---------------------------------------------------------------------------

func BenchmarkSTValueCreationHeap(b *testing.B) {
	b.Run("string", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			sinkValue = StringValue(nil, "hello world")
		}
	})
	b.Run("int", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			sinkValue = IntValue(nil, 12345)
		}
	})
	b.Run("float", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			sinkValue = FloatValue(nil, 123.456)
		}
	})
	b.Run("true", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			sinkValue = TrueValue(nil)
		}
	})
	b.Run("false", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			sinkValue = FalseValue(nil)
		}
	})
	b.Run("object", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			sinkValue = ObjectValue(nil)
		}
	})
	b.Run("array", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			sinkValue = ArrayValue(nil)
		}
	})
}

func BenchmarkSTValueCreationArena(b *testing.B) {
	a := arena.NewMonotonicArena(arena.WithMinBufferSize(1024 * 1024))
	b.Run("string", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			sinkValue = StringValue(a, "hello world")
			a.Reset()
		}
	})
	b.Run("int", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			sinkValue = IntValue(a, 12345)
			a.Reset()
		}
	})
	b.Run("float", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			sinkValue = FloatValue(a, 123.456)
			a.Reset()
		}
	})
	b.Run("true", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			sinkValue = TrueValue(a)
			a.Reset()
		}
	})
	b.Run("false", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			sinkValue = FalseValue(a)
			a.Reset()
		}
	})
	b.Run("object", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			sinkValue = ObjectValue(a)
			a.Reset()
		}
	})
	b.Run("array", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			sinkValue = ArrayValue(a)
			a.Reset()
		}
	})
}

// ---------------------------------------------------------------------------
// Section 5: Mutation
// ---------------------------------------------------------------------------

func BenchmarkSTObjectSet(b *testing.B) {
	b.Run("new_key", func(b *testing.B) {
		a := arena.NewMonotonicArena(arena.WithMinBufferSize(4096))
		var p Parser
		base := []byte(`{"a":1,"b":2,"c":3}`)
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			v, _ := p.ParseBytesWithArena(a, base)
			v.Set(a, "d", IntValue(a, 4))
			sinkValue = v
			a.Reset()
		}
	})
	b.Run("existing_key", func(b *testing.B) {
		a := arena.NewMonotonicArena(arena.WithMinBufferSize(4096))
		var p Parser
		base := []byte(`{"a":1,"b":2,"c":3}`)
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			v, _ := p.ParseBytesWithArena(a, base)
			v.Set(a, "b", IntValue(a, 20))
			sinkValue = v
			a.Reset()
		}
	})
}

func BenchmarkSTObjectDel(b *testing.B) {
	b.Run("first_key", func(b *testing.B) {
		var p Parser
		base := `{"a":1,"b":2,"c":3,"d":4,"e":5}`
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			v, _ := p.Parse(base)
			v.Del("a")
			sinkValue = v
		}
	})
	b.Run("last_key", func(b *testing.B) {
		var p Parser
		base := `{"a":1,"b":2,"c":3,"d":4,"e":5}`
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			v, _ := p.Parse(base)
			v.Del("e")
			sinkValue = v
		}
	})
	b.Run("miss", func(b *testing.B) {
		var p Parser
		base := `{"a":1,"b":2,"c":3,"d":4,"e":5}`
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			v, _ := p.Parse(base)
			v.Del("nonexistent")
			sinkValue = v
		}
	})
}

func BenchmarkSTSetArrayItem(b *testing.B) {
	b.Run("replace", func(b *testing.B) {
		a := arena.NewMonotonicArena(arena.WithMinBufferSize(4096))
		var p Parser
		base := []byte(`[1,2,3,4,5]`)
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			v, _ := p.ParseBytesWithArena(a, base)
			v.SetArrayItem(a, 2, IntValue(a, 30))
			sinkValue = v
			a.Reset()
		}
	})
	b.Run("append", func(b *testing.B) {
		a := arena.NewMonotonicArena(arena.WithMinBufferSize(4096))
		var p Parser
		base := []byte(`[1,2,3,4,5]`)
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			v, _ := p.ParseBytesWithArena(a, base)
			v.SetArrayItem(a, 5, IntValue(a, 6))
			sinkValue = v
			a.Reset()
		}
	})
}

func BenchmarkSTAppendToArray(b *testing.B) {
	a := arena.NewMonotonicArena(arena.WithMinBufferSize(4096))
	var p Parser
	base := []byte(`[1,2,3,4,5,6,7,8,9,10]`)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		v, _ := p.ParseBytesWithArena(a, base)
		AppendToArray(a, v, IntValue(a, 11))
		sinkValue = v
		a.Reset()
	}
}

// ---------------------------------------------------------------------------
// Section 6: Serialization
// ---------------------------------------------------------------------------

func BenchmarkSTMarshalTo(b *testing.B) {
	var p Parser
	for _, f := range fixtures {
		b.Run(f.name, func(b *testing.B) {
			v, err := p.Parse(f.data)
			if err != nil {
				b.Fatal(err)
			}
			dst := make([]byte, 0, len(f.data))
			b.ReportAllocs()
			b.SetBytes(int64(len(f.data)))
			b.ResetTimer()
			for b.Loop() {
				sinkBytes = v.MarshalTo(dst[:0])
			}
		})
	}
	b.Run("20mb", func(b *testing.B) {
		v, err := p.Parse(huge20MbFixture)
		if err != nil {
			b.Fatal(err)
		}
		dst := make([]byte, 0, len(huge20MbFixture))
		b.ReportAllocs()
		b.SetBytes(int64(len(huge20MbFixture)))
		b.ResetTimer()
		for b.Loop() {
			sinkBytes = v.MarshalTo(dst[:0])
		}
	})
}

func BenchmarkSTMarshalToArena(b *testing.B) {
	for _, f := range fixtures {
		b.Run(f.name, func(b *testing.B) {
			var p Parser
			a := arena.NewMonotonicArena(arena.WithMinBufferSize(2 * 1024 * 1024))
			dst := make([]byte, 0, len(f.data))
			b.ReportAllocs()
			b.SetBytes(int64(len(f.data)))
			b.ResetTimer()
			for b.Loop() {
				v, err := p.ParseWithArena(a, f.data)
				if err != nil {
					b.Fatal(err)
				}
				sinkBytes = v.MarshalTo(dst[:0])
				a.Reset()
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Section 7: Utilities
// ---------------------------------------------------------------------------

func BenchmarkSTDeepCopy(b *testing.B) {
	subsets := []struct {
		name string
		data string
	}{
		{"small", smallFixture},
		{"medium", mediumFixture},
		{"twitter", twitterFixture},
	}
	for _, s := range subsets {
		b.Run(s.name, func(b *testing.B) {
			var p Parser
			v, err := p.Parse(s.data)
			if err != nil {
				b.Fatal(err)
			}
			a := arena.NewMonotonicArena(arena.WithMinBufferSize(2 * 1024 * 1024))
			b.ReportAllocs()
			b.SetBytes(int64(len(s.data)))
			b.ResetTimer()
			for b.Loop() {
				sinkValue = DeepCopy(a, v)
				a.Reset()
			}
		})
	}
}

func BenchmarkSTDeduplicateObjectKeys(b *testing.B) {
	b.Run("small", func(b *testing.B) {
		var p Parser
		data := `{"a":1,"b":2,"a":3,"c":4,"b":5}`
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			v, _ := p.Parse(data)
			DeduplicateObjectKeysRecursively(v)
			sinkValue = v
		}
	})
	b.Run("large", func(b *testing.B) {
		// Use bunchFields fixture (871 keys, all unique — tests the scan cost)
		var p Parser
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			v, _ := p.Parse(bunchFieldsFixture)
			DeduplicateObjectKeysRecursively(v)
			sinkValue = v
		}
	})
}

func BenchmarkSTSetValue(b *testing.B) {
	b.Run("existing_path", func(b *testing.B) {
		a := arena.NewMonotonicArena(arena.WithMinBufferSize(4096))
		var p Parser
		base := []byte(`{"data":{"user":{"name":"old"}}}`)
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			v, _ := p.ParseBytesWithArena(a, base)
			SetValue(a, v, StringValue(a, "new"), "data", "user", "name")
			sinkValue = v
			a.Reset()
		}
	})
	b.Run("new_path_depth2", func(b *testing.B) {
		a := arena.NewMonotonicArena(arena.WithMinBufferSize(4096))
		var p Parser
		base := []byte(`{"data":{}}`)
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			v, _ := p.ParseBytesWithArena(a, base)
			SetValue(a, v, StringValue(a, "value"), "data", "newkey")
			sinkValue = v
			a.Reset()
		}
	})
	b.Run("new_path_depth4", func(b *testing.B) {
		a := arena.NewMonotonicArena(arena.WithMinBufferSize(4096))
		var p Parser
		base := []byte(`{}`)
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			v, _ := p.ParseBytesWithArena(a, base)
			SetValue(a, v, StringValue(a, "value"), "a", "b", "c", "d")
			sinkValue = v
			a.Reset()
		}
	})
}

func BenchmarkSTSetNull(b *testing.B) {
	b.Run("depth_1", func(b *testing.B) {
		a := arena.NewMonotonicArena(arena.WithMinBufferSize(4096))
		var p Parser
		base := []byte(`{"data":"value"}`)
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			v, _ := p.ParseBytesWithArena(a, base)
			SetNull(a, v, "data")
			sinkValue = v
			a.Reset()
		}
	})
	b.Run("depth_3", func(b *testing.B) {
		a := arena.NewMonotonicArena(arena.WithMinBufferSize(4096))
		var p Parser
		base := []byte(`{"a":{"b":{"c":"value"}}}`)
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			v, _ := p.ParseBytesWithArena(a, base)
			SetNull(a, v, "a", "b", "c")
			sinkValue = v
			a.Reset()
		}
	})
}

// ---------------------------------------------------------------------------
// Section 8: Validation
// ---------------------------------------------------------------------------

func BenchmarkSTValidate(b *testing.B) {
	for _, f := range fixtures {
		b.Run(f.name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(f.data)))
			b.ResetTimer()
			for b.Loop() {
				sinkErr = Validate(f.data)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Section 9: Scanner
// ---------------------------------------------------------------------------

func BenchmarkSTScanner(b *testing.B) {
	// Concatenate 100 copies of small.json separated by whitespace
	parts := make([]string, 100)
	for i := range parts {
		parts[i] = smallFixture
	}
	data := strings.Join(parts, "\n")

	var sc Scanner
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for b.Loop() {
		sc.Init(data)
		for sc.Next() {
			sinkValue = sc.Value()
		}
		if sc.Error() != nil {
			b.Fatal(sc.Error())
		}
	}
}

// ---------------------------------------------------------------------------
// Section 10: End-to-End
// ---------------------------------------------------------------------------

func BenchmarkSTParseAndGetMultiple(b *testing.B) {
	b.Run("heap", func(b *testing.B) {
		var p Parser
		b.ReportAllocs()
		b.SetBytes(int64(len(twitterFixture)))
		b.ResetTimer()
		for b.Loop() {
			v, err := p.Parse(twitterFixture)
			if err != nil {
				b.Fatal(err)
			}
			_ = v.Get("statuses")
			_ = v.Get("statuses", "0", "user")
			_ = v.Get("search_metadata")
			_ = v.Get("statuses", "0", "text")
			sinkValue = v
		}
	})
	b.Run("arena", func(b *testing.B) {
		var p Parser
		a := arena.NewMonotonicArena(arena.WithMinBufferSize(2 * 1024 * 1024))
		b.ReportAllocs()
		b.SetBytes(int64(len(twitterFixture)))
		b.ResetTimer()
		for b.Loop() {
			v, err := p.ParseWithArena(a, twitterFixture)
			if err != nil {
				b.Fatal(err)
			}
			_ = v.Get("statuses")
			_ = v.Get("statuses", "0", "user")
			_ = v.Get("search_metadata")
			_ = v.Get("statuses", "0", "text")
			sinkValue = v
			a.Reset()
		}
	})
}

func BenchmarkSTParseModifyMarshal(b *testing.B) {
	var p Parser
	data := smallFixture
	dst := make([]byte, 0, len(data)*2)
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for b.Loop() {
		v, err := p.Parse(data)
		if err != nil {
			b.Fatal(err)
		}
		v.Set(nil, "newkey", StringValue(nil, "newval"))
		sinkBytes = v.MarshalTo(dst[:0])
	}
}
