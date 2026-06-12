package astjson

import (
	"os"
	"path/filepath"
	"testing"
)

// BenchmarkMarshalCleanFastPath measures MarshalTo on an escape-free
// payload (the small.json fixture) with the noEscapeSubtree fast path
// enabled vs. disabled.
//
// The "slow" variant recursively clears noEscapeSubtree on every node so
// no level takes its own fast path — an honest comparison against the
// per-node escape-check path.
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
		clearNoEscapeSubtreeRecursive(v)
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			dst = v.MarshalTo(dst[:0])
		}
		b.StopTimer()
		v.RecomputeEscapeHint()
	})
}

// clearNoEscapeSubtreeRecursive disables the fast path on every node of v's
// subtree, forcing MarshalTo onto the per-node escape-check path.
func clearNoEscapeSubtreeRecursive(v *Value) {
	if v == nil {
		return
	}
	v.noEscapeSubtree = false
	switch v.Type() {
	case TypeObject:
		for _, kv := range v.o.kvs {
			clearNoEscapeSubtreeRecursive(kv.v)
		}
	case TypeArray:
		for _, item := range v.a {
			clearNoEscapeSubtreeRecursive(item)
		}
	}
}
