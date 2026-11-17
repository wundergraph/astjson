package astjson

import (
	"strconv"
	"testing"

	"github.com/wundergraph/go-arena"
)

// BenchmarkSharedValues measures the value constructors and SetNull, each
// without an arena and with one. Every case handles 100 items per iteration.
func BenchmarkSharedValues(b *testing.B) {
	names := make([]string, 100)
	for i := range names {
		names[i] = "record-" + strconv.Itoa(i)
	}

	parsed := make([]*Value, 100)
	for i := range parsed {
		record, err := Parse(`{"id":1,"seller":{"name":"a"},"price":9.5}`)
		if err != nil {
			b.Fatal(err)
		}
		parsed[i] = record
	}

	boolValue := func(a arena.Arena, v bool) *Value {
		if v {
			return TrueValue(a)
		}
		return FalseValue(a)
	}

	var sink *Value

	benchmarks := []struct {
		name string
		run  func(a arena.Arena)
	}{
		{
			// The boolean constructors alone: the upper bound of what a
			// boolean-only payload gains.
			name: "BooleanValues",
			run: func(a arena.Arena) {
				for i := range 100 {
					sink = boolValue(a, i%2 == 0)
				}
			},
		},
		{
			// Nulling an existing field, which is what a renderer does when it
			// bubbles a null up to a nullable parent.
			name: "SetNull",
			run: func(a arena.Arena) {
				for _, record := range parsed {
					SetNull(a, record, "seller")
				}
			},
		},
		{
			// Building a document the way a caller that converts another format
			// to JSON does (for example the gRPC datasource in graphql-go-tools):
			// one constructor call per field, two of the seven fields booleans,
			// and the nested object of every fourth record nulled afterwards.
			name: "BuildRecords",
			run: func(a arena.Arena) {
				records := ArrayValue(a)
				for i, name := range names {
					seller := ObjectValue(a)
					seller.Set(a, "name", StringValue(a, name))

					record := ObjectValue(a)
					record.Set(a, "id", IntValue(a, i))
					record.Set(a, "name", StringValue(a, name))
					record.Set(a, "price", FloatValue(a, 9.5))
					record.Set(a, "currency", StringValue(a, "EUR"))
					record.Set(a, "active", boolValue(a, i%2 == 0))
					record.Set(a, "verified", boolValue(a, i%3 == 0))
					record.Set(a, "seller", seller)
					if i%4 == 0 {
						SetNull(a, record, "seller")
					}
					records.SetArrayItem(a, i, record)
				}
				sink = records
			},
		},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			b.Run("heap", func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					bm.run(nil)
				}
			})

			b.Run("arena", func(b *testing.B) {
				a := arena.NewMonotonicArena()
				b.ReportAllocs()
				arenaBytes := 0
				for b.Loop() {
					bm.run(a)
					arenaBytes = a.Len()
					a.Reset()
				}
				b.ReportMetric(float64(arenaBytes), "arena-B/op")
			})
		})
	}
	_ = sink
}
