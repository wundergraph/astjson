package astjson

import (
	"errors"

	"github.com/wundergraph/go-arena"
)

var (
	// ErrMergeDifferentTypes is returned when merging two values of incompatible types.
	ErrMergeDifferentTypes = errors.New("cannot merge different types")
	// ErrMergeDifferingArrayLengths is returned when merging arrays of different lengths.
	ErrMergeDifferingArrayLengths = errors.New("cannot merge arrays of differing lengths")
	// ErrMergeUnknownType is returned when merging a value with an unrecognized type.
	ErrMergeUnknownType = errors.New("cannot merge unknown type")
)

// MergeValues recursively merges b into a and returns the result. For objects,
// keys from b are added to or replace keys in a. For arrays, elements are
// merged pairwise (arrays must have equal length). For scalars, b replaces a
// when the values differ.
//
// The arena ar is used for any new allocations during the merge (new object
// entries, key copies). Both a and b should have been allocated using the same
// arena (or both on the heap) to avoid mixing memory lifetimes.
//
// Returns the merged value, whether a was changed, and any error.
// If a is nil, returns (b, true, nil). If b is nil, returns (a, false, nil).
func MergeValues(ar arena.Arena, a, b *Value) (v *Value, changed bool, err error) {
	if a == nil {
		return b, true, nil
	}
	if b == nil {
		return a, false, nil
	}
	at, bt := a.t, b.t
	if bt == TypeNull && at == TypeObject {
		// we assume that null was returned in an error case for resolving a nested object field
		// as we've got an object on the left side, we don't override the whole object with null
		// instead, we keep the left object and discard the null on the right side
		return a, false, nil
	}
	if at != bt {
		// Only compute boolean compatibility when types actually differ
		aBool := at == TypeTrue || at == TypeFalse
		bBool := bt == TypeTrue || bt == TypeFalse
		if !aBool || !bBool {
			return nil, false, ErrMergeDifferentTypes
		}
		// Types differ but both are booleans — b replaces a
		return b, true, nil
	}
	switch at {
	case TypeObject:
		ao, _ := a.Object()
		bo, _ := b.Object()
		// Unescape keys as needed during iteration
		for i := range bo.kvs {
			if !bo.kvs[i].keyUnescaped {
				bo.unescapeKey(ar, bo.kvs[i])
			}
		}
		for i := range bo.kvs {
			k := bo.kvs[i].k
			r := bo.kvs[i].v
			l := ao.Get(k)
			if l == nil {
				ao.Set(ar, k, r)
				continue
			}
			n, changed, err := MergeValues(ar, l, r)
			if err != nil {
				return nil, false, err
			}
			if changed {
				ao.Set(ar, k, n)
			}
		}
		return a, false, nil
	case TypeArray:
		aa, _ := a.Array()
		ba, _ := b.Array()
		if len(aa) == 0 {
			return b, true, nil
		}
		if len(ba) == 0 {
			return a, false, nil
		}
		if len(aa) != len(ba) {
			return nil, false, ErrMergeDifferingArrayLengths
		}
		for i := range aa {
			n, changed, err := MergeValues(ar, aa[i], ba[i])
			if err != nil {
				return nil, false, err
			}
			if changed {
				aa[i] = n
			}
		}
		return a, false, nil
	case TypeTrue, TypeFalse, TypeNull:
		// at == bt guaranteed by the check above, no change needed
		return a, false, nil
	case TypeNumber:
		// Fast path: if raw number strings are identical, values are equal.
		// This avoids expensive float64 parsing in the common case.
		if a.s == b.s {
			return a, false, nil
		}
		af, aErr := a.Float64()
		bf, bErr := b.Float64()
		if aErr != nil || bErr != nil || af != bf {
			return b, true, nil
		}
		return a, false, nil
	case TypeString:
		if a.s != b.s {
			return b, true, nil
		}
		return a, false, nil
	default:
		return nil, false, ErrMergeUnknownType
	}
}

// MergeValuesWithPath wraps b in a nested object structure at the given path,
// then merges the result into a using [MergeValues]. For example, with
// path ["foo", "bar"], b is wrapped as {"foo": {"bar": b}} before merging.
//
// If path is empty, behaves identically to [MergeValues].
//
// The arena ar is used for allocating the wrapper objects and during the merge.
func MergeValuesWithPath(ar arena.Arena, a, b *Value, path ...string) (v *Value, changed bool, err error) {
	if len(path) == 0 {
		return MergeValues(ar, a, b)
	}
	root := ObjectValue(ar)
	current := root
	for i := 0; i < len(path)-1; i++ {
		current.Set(ar, path[i], ObjectValue(ar))
		current = current.Get(path[i])
	}
	current.Set(ar, path[len(path)-1], b)
	return MergeValues(ar, a, root)
}
