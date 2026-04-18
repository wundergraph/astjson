//go:build astjson_debug

package astjson

import (
	"strings"
	"testing"
)

// TestDebugVerifierCatchesStaleFlag confirms that under the astjson_debug
// build tag, MarshalTo panics when a root's noEscapeSubtree is stale-true
// because a descendant was mutated without invalidating the root.
func TestDebugVerifierCatchesStaleFlag(t *testing.T) {
	var p Parser
	v, err := p.Parse(`{"a":{"b":"clean"}}`)
	if err != nil {
		t.Fatalf("parse: %s", err)
	}
	if !v.noEscapeSubtree {
		t.Fatal("expected clean after parse")
	}
	// Stash a dirty string into a descendant without invalidating the root.
	inner := v.Get("a", "b")
	inner.s = "a\nb"
	inner.stringNeedsEscape = true
	inner.noEscapeSubtree = false
	// Root still says clean; MarshalTo fast path should panic.
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic from debug verifier")
		}
		msg, ok := r.(string)
		if !ok || !strings.Contains(msg, "stale noEscapeSubtree") {
			t.Fatalf("unexpected panic value: %v", r)
		}
	}()
	_ = v.MarshalTo(nil)
}
