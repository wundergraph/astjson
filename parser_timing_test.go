package astjson

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/wundergraph/go-arena"
)

func BenchmarkParseRawString(b *testing.B) {
	for _, s := range []string{`""`, `"a"`, `"abcd"`, `"abcdefghijk"`, `"qwertyuiopasdfghjklzxcvb"`} {
		b.Run(s, func(b *testing.B) {
			benchmarkParseRawString(b, s)
		})
	}
}

func benchmarkParseRawString(b *testing.B, s string) {
	b.ReportAllocs()
	b.SetBytes(int64(len(s)))
	s = s[1:] // skip the opening '"'
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			rs, tail, _, err := parseRawStringInfo(s)
			if err != nil {
				panic(fmt.Errorf("cannot parse %q: %s", s, err))
			}
			if rs != s[:len(s)-1] {
				panic(fmt.Errorf("invalid string obtained; got %q; want %q", rs, s[:len(s)-1]))
			}
			if len(tail) > 0 {
				panic(fmt.Errorf("non-empty tail got: %q", tail))
			}
		}
	})
}

func BenchmarkParseRawNumber(b *testing.B) {
	for _, s := range []string{"1", "1234", "123456", "-1234", "1234567890.1234567", "-1.32434e+12"} {
		b.Run(s, func(b *testing.B) {
			benchmarkParseRawNumber(b, s)
		})
	}
}

func benchmarkParseRawNumber(b *testing.B, s string) {
	b.ReportAllocs()
	b.SetBytes(int64(len(s)))
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			rn, tail, err := parseRawNumber(s)
			if err != nil {
				panic(fmt.Errorf("cannot parse %q: %s", s, err))
			}
			if rn != s {
				panic(fmt.Errorf("invalid number obtained; got %q; want %q", rn, s))
			}
			if len(tail) > 0 {
				panic(fmt.Errorf("non-empty tail got: %q", tail))
			}
		}
	})
}

var (
	// small, medium and large fixtures are from https://github.com/buger/jsonparser/blob/f04e003e4115787c6272636780bc206e5ffad6c4/benchmark/benchmark.go
	smallFixture  = getFromFile("testdata/small.json")
	mediumFixture = getFromFile("testdata/medium.json")
	largeFixture  = getFromFile("testdata/large.json")

	// canada, citm and twitter fixtures are from https://github.com/serde-rs/json-benchmark/tree/0db02e043b3ae87dc5065e7acb8654c1f7670c43/data
	canadaFixture  = getFromFile("testdata/canada.json")
	citmFixture    = getFromFile("testdata/citm_catalog.json")
	twitterFixture = getFromFile("testdata/twitter.json")

	// 20mb is a huge (stressful) fixture from https://examplefile.com/code/json/20-mb-json
	huge20MbFixture = getFromFile("testdata/20mb.json")
)

func getFromFile(filename string) string {
	data, err := os.ReadFile(filename)
	if err != nil {
		panic(fmt.Errorf("cannot read %s: %s", filename, err))
	}
	return string(data)
}

func BenchmarkObjectGet(b *testing.B) {
	for _, itemsCount := range []int{10, 100, 1000, 10000, 100000} {
		b.Run(fmt.Sprintf("items_%d", itemsCount), func(b *testing.B) {
			for _, lookupsCount := range []int{0, 1, 2, 4, 8, 16, 32, 64} {
				b.Run(fmt.Sprintf("lookups_%d", lookupsCount), func(b *testing.B) {
					benchmarkObjectGet(b, itemsCount, lookupsCount)
				})
			}
		})
	}
}

func benchmarkObjectGet(b *testing.B, itemsCount, lookupsCount int) {
	var benchPool Parser
	b.StopTimer()
	var ss []string
	for i := 0; i < itemsCount; i++ {
		s := fmt.Sprintf(`"key_%d": "value_%d"`, i, i)
		ss = append(ss, s)
	}
	s := "{" + strings.Join(ss, ",") + "}"
	key := fmt.Sprintf("key_%d", len(ss)/2)
	expectedValue := fmt.Sprintf("value_%d", len(ss)/2)
	b.StartTimer()
	b.ReportAllocs()
	b.SetBytes(int64(len(s)))

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			v, err := benchPool.Parse(s)
			if err != nil {
				panic(fmt.Errorf("unexpected error: %s", err))
			}
			o := v.GetObject()
			for i := 0; i < lookupsCount; i++ {
				sb := o.Get(key).GetStringBytes()
				if string(sb) != expectedValue {
					panic(fmt.Errorf("unexpected value; got %q; want %q", sb, expectedValue))
				}
			}
		}
	})
}

func BenchmarkMarshalTo(b *testing.B) {
	b.Run("small", func(b *testing.B) {
		benchmarkMarshalTo(b, smallFixture)
	})
	b.Run("medium", func(b *testing.B) {
		benchmarkMarshalTo(b, mediumFixture)
	})
	b.Run("large", func(b *testing.B) {
		benchmarkMarshalTo(b, largeFixture)
	})
	b.Run("canada", func(b *testing.B) {
		benchmarkMarshalTo(b, canadaFixture)
	})
	b.Run("citm", func(b *testing.B) {
		benchmarkMarshalTo(b, citmFixture)
	})
	b.Run("twitter", func(b *testing.B) {
		benchmarkMarshalTo(b, twitterFixture)
	})
	b.Run("20mb", func(b *testing.B) {
		benchmarkMarshalTo(b, huge20MbFixture)
	})
}

var benchPoolMarshalTo Parser
var benchDeepCopySink *Value
var benchParserDeepCopy Parser
var benchParserStructuralCopy Parser
var benchStringBytesSink []byte
var benchMarshalBytesSink []byte

func BenchmarkDeepCopy(b *testing.B) {
	b.Run("small", func(b *testing.B) {
		benchmarkDeepCopy(b, smallFixture)
	})
	b.Run("medium", func(b *testing.B) {
		benchmarkDeepCopy(b, mediumFixture)
	})
	b.Run("canada", func(b *testing.B) {
		benchmarkDeepCopy(b, canadaFixture)
	})
	b.Run("20mb", func(b *testing.B) {
		benchmarkDeepCopy(b, huge20MbFixture)
	})
}

func BenchmarkStructuralCopy(b *testing.B) {
	b.Run("small", func(b *testing.B) {
		benchmarkStructuralCopy(b, smallFixture)
	})
	b.Run("medium", func(b *testing.B) {
		benchmarkStructuralCopy(b, mediumFixture)
	})
	b.Run("canada", func(b *testing.B) {
		benchmarkStructuralCopy(b, canadaFixture)
	})
	b.Run("20mb", func(b *testing.B) {
		benchmarkStructuralCopy(b, huge20MbFixture)
	})
}

func BenchmarkStringBytesRaw(b *testing.B) {
	b.Run("plain", func(b *testing.B) {
		benchmarkStringBytesRaw(b, Value{
			t:         TypeString,
			stringRaw: true,
			s:         "plain string value",
		})
	})
	b.Run("escaped", func(b *testing.B) {
		benchmarkStringBytesRaw(b, Value{
			t:         TypeString,
			stringRaw: true,
			s:         `he said \"hi\"\n`,
		})
	})
}

func BenchmarkMarshalStringValue(b *testing.B) {
	b.Run("plain", func(b *testing.B) {
		benchmarkMarshalStringValue(b, StringValue(nil, "plain string value"))
	})
	b.Run("escaped", func(b *testing.B) {
		benchmarkMarshalStringValue(b, StringValue(nil, "he said \"hi\"\n"))
	})
}

func BenchmarkMarshalObjectKey(b *testing.B) {
	b.Run("plain", func(b *testing.B) {
		benchmarkMarshalObjectKey(b, "plain_key")
	})
	b.Run("escaped", func(b *testing.B) {
		benchmarkMarshalObjectKey(b, "he said \"hi\"\n")
	})
}

func benchmarkMarshalTo(b *testing.B, s string) {
	v, err := benchPoolMarshalTo.Parse(s)
	if err != nil {
		panic(fmt.Errorf("unexpected error: %s", err))
	}

	b.ReportAllocs()
	b.SetBytes(int64(len(s)))
	b.RunParallel(func(pb *testing.PB) {
		var b []byte
		for pb.Next() {
			// It is ok calling v.MarshalTo from concurrent
			// goroutines, since MarshalTo doesn't modify v.
			b = v.MarshalTo(b[:0])
		}
	})
}

func benchmarkDeepCopy(b *testing.B, s string) {
	src, err := Parse(s)
	if err != nil {
		panic(fmt.Errorf("unexpected error: %s", err))
	}

	a := arena.NewMonotonicArena()
	for i := 0; i < 2; i++ {
		a.Reset()
		benchDeepCopySink = benchParserDeepCopy.DeepCopy(a, src)
	}
	a.Reset()
	b.ReportAllocs()
	b.SetBytes(int64(len(s)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		a.Reset()
		benchDeepCopySink = benchParserDeepCopy.DeepCopy(a, src)
	}
}

func benchmarkStructuralCopy(b *testing.B, s string) {
	srcArena := arena.NewMonotonicArena()
	var srcParser Parser
	src, err := srcParser.ParseWithArena(srcArena, s)
	if err != nil {
		panic(fmt.Errorf("unexpected error: %s", err))
	}

	dstArena := arena.NewMonotonicArena()
	for i := 0; i < 2; i++ {
		dstArena.Reset()
		benchDeepCopySink = benchParserStructuralCopy.StructuralCopy(dstArena, src)
	}
	dstArena.Reset()
	b.ReportAllocs()
	b.SetBytes(int64(len(s)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dstArena.Reset()
		benchDeepCopySink = benchParserStructuralCopy.StructuralCopy(dstArena, src)
	}
	runtime.KeepAlive(srcArena)
}

func benchmarkStringBytesRaw(b *testing.B, template Value) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		v := template
		sb, err := v.StringBytes()
		if err != nil {
			panic(err)
		}
		benchStringBytesSink = sb
	}
}

func benchmarkMarshalStringValue(b *testing.B, v *Value) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		benchMarshalBytesSink = v.MarshalTo(benchMarshalBytesSink[:0])
	}
}

func benchmarkMarshalObjectKey(b *testing.B, key string) {
	obj := ObjectValue(nil)
	obj.Set(nil, key, IntValue(nil, 1))
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		benchMarshalBytesSink = obj.MarshalTo(benchMarshalBytesSink[:0])
	}
}

func BenchmarkParseComparison(b *testing.B) {
	b.Run("small", func(b *testing.B) {
		benchmarkParse(b, smallFixture)
	})
	b.Run("medium", func(b *testing.B) {
		benchmarkParse(b, mediumFixture)
	})
	b.Run("large", func(b *testing.B) {
		benchmarkParse(b, largeFixture)
	})
	b.Run("canada", func(b *testing.B) {
		benchmarkParse(b, canadaFixture)
	})
	b.Run("citm", func(b *testing.B) {
		benchmarkParse(b, citmFixture)
	})
	b.Run("twitter", func(b *testing.B) {
		benchmarkParse(b, twitterFixture)
	})
}

func benchmarkParse(b *testing.B, s string) {
	b.Run("stdjson-map", func(b *testing.B) {
		benchmarkStdJSONParseMap(b, s)
	})
	b.Run("stdjson-struct", func(b *testing.B) {
		benchmarkStdJSONParseStruct(b, s)
	})
	b.Run("stdjson-empty-struct", func(b *testing.B) {
		benchmarkStdJSONParseEmptyStruct(b, s)
	})
	b.Run("fastjson", func(b *testing.B) {
		benchmarkFastJSONParse(b, s)
	})
	b.Run("fastjson-get", func(b *testing.B) {
		benchmarkFastJSONParseGet(b, s)
	})
}

func benchmarkFastJSONParse(b *testing.B, s string) {
	var benchPool Parser
	b.ReportAllocs()
	b.SetBytes(int64(len(s)))
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			v, err := benchPool.Parse(s)
			if err != nil {
				panic(fmt.Errorf("unexpected error: %s", err))
			}
			if v.Type() != TypeObject {
				panic(fmt.Errorf("unexpected value type; got %s; want %s", v.Type(), TypeObject))
			}
		}
	})
}

func benchmarkFastJSONParseGet(b *testing.B, s string) {
	var benchPool Parser
	b.ReportAllocs()
	b.SetBytes(int64(len(s)))
	b.RunParallel(func(pb *testing.PB) {
		var n int
		for pb.Next() {
			v, err := benchPool.Parse(s)
			if err != nil {
				panic(fmt.Errorf("unexpected error: %s", err))
			}
			n += v.GetInt("sid")
			n += len(v.GetStringBytes("uuid"))
			p := v.Get("person")
			if p != nil {
				n++
			}
			c := v.Get("company")
			if c != nil {
				n++
			}
			u := v.Get("users")
			if u != nil {
				n++
			}
			a := v.GetArray("features")
			n += len(a)
			a = v.GetArray("topicSubTopics")
			n += len(a)
			o := v.Get("search_metadata")
			if o != nil {
				n++
			}
		}
	})
}

func benchmarkStdJSONParseMap(b *testing.B, s string) {
	b.ReportAllocs()
	b.SetBytes(int64(len(s)))
	bb := s2b(s)
	b.RunParallel(func(pb *testing.PB) {
		var m map[string]interface{}
		for pb.Next() {
			if err := json.Unmarshal(bb, &m); err != nil {
				panic(fmt.Errorf("unexpected error: %s", err))
			}
		}
	})
}

func benchmarkStdJSONParseStruct(b *testing.B, s string) {
	b.ReportAllocs()
	b.SetBytes(int64(len(s)))
	bb := s2b(s)
	b.RunParallel(func(pb *testing.PB) {
		var m struct {
			Sid            int
			UUID           string
			Person         map[string]interface{}
			Company        map[string]interface{}
			Users          []interface{}
			Features       []map[string]interface{}
			TopicSubTopics map[string]interface{}
			SearchMetadata map[string]interface{}
		}
		for pb.Next() {
			if err := json.Unmarshal(bb, &m); err != nil {
				panic(fmt.Errorf("unexpected error: %s", err))
			}
		}
	})
}

func benchmarkStdJSONParseEmptyStruct(b *testing.B, s string) {
	b.ReportAllocs()
	b.SetBytes(int64(len(s)))
	bb := s2b(s)
	b.RunParallel(func(pb *testing.PB) {
		var m struct{}
		for pb.Next() {
			if err := json.Unmarshal(bb, &m); err != nil {
				panic(fmt.Errorf("unexpected error: %s", err))
			}
		}
	})
}
