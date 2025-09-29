package astjson

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
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
			rs, tail, err := parseRawString(s)
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
