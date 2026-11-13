package astjson

import (
	"strconv"
	"testing"

	"github.com/wundergraph/go-arena"
)

// BenchmarkBuildRecords measures building a document the way a caller that
// converts another format to JSON does (for example the gRPC datasource in
// graphql-go-tools): one object per record and one value constructor call per
// field. Two of the seven fields are booleans, and every fourth record has its
// nested object nulled afterwards, as a renderer does when a null bubbles up
// to a nullable parent.
func BenchmarkBuildRecords(b *testing.B) {
	names := make([]string, 100)
	for i := range names {
		names[i] = "record-" + strconv.Itoa(i)
	}

	build := func(a arena.Arena) *Value {
		records := ArrayValue(a)
		for i, name := range names {
			seller := ObjectValue(a)
			seller.Set(a, "name", StringValue(a, name))

			record := ObjectValue(a)
			record.Set(a, "id", IntValue(a, i))
			record.Set(a, "name", StringValue(a, name))
			record.Set(a, "price", FloatValue(a, 9.5))
			record.Set(a, "currency", StringValue(a, "EUR"))
			if i%2 == 0 {
				record.Set(a, "active", TrueValue(a))
			} else {
				record.Set(a, "active", FalseValue(a))
			}
			if i%3 == 0 {
				record.Set(a, "verified", TrueValue(a))
			} else {
				record.Set(a, "verified", FalseValue(a))
			}
			record.Set(a, "seller", seller)
			if i%4 == 0 {
				SetNull(a, record, "seller")
			}
			records.SetArrayItem(a, i, record)
		}
		return records
	}

	b.Run("heap", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			_ = build(nil)
		}
	})

	b.Run("arena", func(b *testing.B) {
		a := arena.NewMonotonicArena()
		b.ReportAllocs()
		arenaBytes := 0
		for b.Loop() {
			_ = build(a)
			arenaBytes = a.Len()
			a.Reset()
		}
		b.ReportMetric(float64(arenaBytes), "arena-B/op")
	})
}
