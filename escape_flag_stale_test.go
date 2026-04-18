//go:build !astjson_debug

package astjson

import (
	"testing"

	"github.com/wundergraph/go-arena"
)

// TestMarshalFastPathStaleAncestorSafety guards against a stale-true flag on
// an ancestor causing marshalToClean to recurse into a dirty descendant
// without re-checking. The fast path must recurse via MarshalTo so each
// child re-checks its own hint.
//
// Excluded from the astjson_debug build because the verifier is designed
// to panic precisely on the stale-flag scenario this test constructs; the
// panic path is covered separately by escape_flag_debug_test.go.
func TestMarshalFastPathStaleAncestorSafety(t *testing.T) {
	a := arena.NewMonotonicArena()
	var p Parser
	v, err := p.ParseWithArena(a, `{"outer":{"inner":1}}`)
	if err != nil {
		t.Fatalf("parse: %s", err)
	}
	// Mutate the inner object through a sub-handle. This updates
	// inner.noEscapeSubtree correctly, but the root's flag stays stale-true.
	inner := v.Get("outer")
	inner.Set(a, "dirty\nkey", StringValue(a, "val"))
	if inner.noEscapeSubtree {
		t.Fatal("inner flag should be invalidated after Set with escaped key")
	}
	if !v.noEscapeSubtree {
		t.Fatal("root flag is expected to be stale-true in this scenario")
	}
	// MarshalTo must still emit valid JSON despite the stale root hint.
	got := string(v.MarshalTo(nil))
	want := `{"outer":{"inner":1,"dirty\nkey":"val"}}`
	if got != want {
		t.Fatalf("stale-ancestor fast path emitted wrong bytes\n  got:  %q\n  want: %q", got, want)
	}
}
