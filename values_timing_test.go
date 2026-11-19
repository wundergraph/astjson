package astjson

import (
	"testing"

	"github.com/wundergraph/go-arena"
)

// BenchmarkSharedValues measures TrueValue, FalseValue and SetNull, alone and
// as part of building a document. Each case handles 100 items per iteration.
func BenchmarkSharedValues(b *testing.B) {
	parsed := make([]*Value, 100)
	for i := range parsed {
		parsed[i] = MustParse(`{"id":1,"seller":{"name":"a"}}`)
	}

	b.Run("BooleanValues", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			for range 50 {
				TrueValue()
				FalseValue()
			}
		}
	})

	var sink *Value

	benchmarks := []struct {
		name string
		run  func(a arena.Arena)
	}{
		{"SetNull", func(a arena.Arena) {
			for _, record := range parsed {
				SetNull(a, record, "seller")
			}
		}},
		{"BuildRecords", func(a arena.Arena) {
			sink = ArrayValue(a)
			for i := range 100 {
				record := ObjectValue(a)
				record.Set(a, "id", IntValue(a, i))
				record.Set(a, "name", StringValue(a, "record"))
				record.Set(a, "price", FloatValue(a, 9.5))
				record.Set(a, "active", TrueValue())
				record.Set(a, "verified", FalseValue())
				record.Set(a, "seller", ObjectValue(a))
				SetNull(a, record, "seller")
				sink.SetArrayItem(a, i, record)
			}
		}},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name+"/heap", func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				bm.run(nil)
			}
		})
		b.Run(bm.name+"/arena", func(b *testing.B) {
			a := arena.NewMonotonicArena()
			for b.Loop() {
				bm.run(a)
				b.ReportMetric(float64(a.Len()), "arena-B/op")
				a.Reset()
			}
		})
	}
	_ = sink
}
