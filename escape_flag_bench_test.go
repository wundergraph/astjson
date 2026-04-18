package astjson

import (
	"os"
	"path/filepath"
	"testing"
)

// BenchmarkMarshalCleanFastPath measures MarshalTo on an escape-free
// payload (the small.json fixture) with the noEscapeSubtree fast path
// enabled vs. disabled.
func BenchmarkMarshalCleanFastPath(b *testing.B) {
	payload, err := os.ReadFile(filepath.Join("testdata", "small.json"))
	if err != nil {
		b.Fatalf("read fixture: %v", err)
	}
	var p Parser
	v, err := p.ParseBytes(payload)
	if err != nil {
		b.Fatalf("parse: %v", err)
	}
	if !v.noEscapeSubtree {
		b.Fatalf("fixture expected to be escape-free, but noEscapeSubtree=false")
	}
	var dst []byte

	b.Run("fast", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			dst = v.MarshalTo(dst[:0])
		}
	})
	b.Run("slow", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// Force slow path by clearing the flag on every iteration's entry.
			v.noEscapeSubtree = false
			dst = v.MarshalTo(dst[:0])
		}
		// Restore for other benchmarks.
		v.RecomputeEscapeHint()
	})
}
