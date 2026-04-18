//go:build astjson_debug

package astjson

import "fmt"

// debugVerifyEscapeHint walks v bottom-up and panics if noEscapeSubtree is
// stale-true anywhere — i.e., true on a node where a descendant key or
// string actually requires JSON escaping. This catches bugs where a
// caller mutated through a sub-handle without invalidating ancestors.
//
// Only compiled under the astjson_debug build tag; zero cost in production.
func (v *Value) debugVerifyEscapeHint() {
	if v == nil {
		return
	}
	actual := v.recomputeActualNoEscape()
	if v.noEscapeSubtree && !actual {
		panic(fmt.Sprintf(
			"astjson: stale noEscapeSubtree=true detected on %s node; a descendant needs escaping — caller mutated through a sub-handle without RecomputeEscapeHint",
			v.t,
		))
	}
}

// recomputeActualNoEscape returns the ground-truth answer by walking
// children, ignoring cached flags. Used only by the debug verifier.
func (v *Value) recomputeActualNoEscape() bool {
	if v == nil {
		return true
	}
	switch v.t {
	case TypeString:
		return !v.stringNeedsEscape
	case TypeObject:
		for _, kv := range v.o.kvs {
			if kv.keyNeedsEscape {
				return false
			}
			if !kv.v.recomputeActualNoEscape() {
				return false
			}
		}
		return true
	case TypeArray:
		for _, item := range v.a {
			if !item.recomputeActualNoEscape() {
				return false
			}
		}
		return true
	default:
		return true
	}
}
