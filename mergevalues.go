package astjson

import (
	"bytes"
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
	if b.Type() == TypeNull && a.Type() == TypeObject {
		// we assume that null was returned in an error case for resolving a nested object field
		// as we've got an object on the left side, we don't override the whole object with null
		// instead, we keep the left object and discard the null on the right side
		return a, false, nil
	}
	aBool, bBool := a.Type() == TypeTrue || a.Type() == TypeFalse, b.Type() == TypeTrue || b.Type() == TypeFalse
	booleans := aBool && bBool
	if a.Type() != b.Type() && !booleans {
		return nil, false, ErrMergeDifferentTypes
	}
	switch a.Type() {
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
	case TypeFalse:
		if b.Type() == TypeTrue {
			return b, true, nil
		}
		return a, false, nil
	case TypeTrue:
		if b.Type() == TypeFalse {
			return b, true, nil
		}
		return a, false, nil
	case TypeNull:
		return a, false, nil
	case TypeNumber:
		af, _ := a.Float64()
		bf, _ := b.Float64()
		if af != bf {
			return b, true, nil
		}
		return a, false, nil
	case TypeString:
		as, _ := a.StringBytes()
		bs, _ := b.StringBytes()
		if !bytes.Equal(as, bs) {
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
