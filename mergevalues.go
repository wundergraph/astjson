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
// merged pairwise (arrays must have equal length). For scalars (numbers,
// strings, booleans, null), b replaces a unconditionally — no value
// comparison is performed.
//
// The arena ar is used for any new allocations during the merge (new object
// entries, key copies). Both a and b should have been allocated using the same
// arena (or both on the heap) to avoid mixing memory lifetimes.
//
// If a is nil, returns (b, nil). If b is nil, returns (a, nil).
func MergeValues(ar arena.Arena, a, b *Value) (*Value, error) {
	if a == nil {
		return b, nil
	}
	if b == nil {
		return a, nil
	}
	at, bt := a.t, b.t
	if bt == TypeNull && at == TypeObject {
		// We assume that null was returned in an error case for resolving a
		// nested object field. Since a is an object, keep it and discard the
		// null on the right.
		return a, nil
	}
	if at != bt {
		// Types only compose when both are boolean — true and false are
		// interchangeable. Anything else is an error.
		aBool := at == TypeTrue || at == TypeFalse
		bBool := bt == TypeTrue || bt == TypeFalse
		if !aBool || !bBool {
			return nil, ErrMergeDifferentTypes
		}
		return b, nil
	}
	switch at {
	case TypeObject:
		ao, _ := a.Object()
		bo, _ := b.Object()
		for i := range bo.kvs {
			if !bo.kvs[i].keyUnescaped {
				bo.unescapeKey(ar, bo.kvs[i])
			}
		}
		for i := range bo.kvs {
			k := bo.kvs[i].k
			r := bo.kvs[i].v
			// Inline the kv lookup so we can mutate akv.v directly when the
			// key already exists — avoids the O(|A|) linear scan inside
			// Object.Set for matching keys.
			var akv *kv
			for j := range ao.kvs {
				if ao.kvs[j].k == k {
					akv = ao.kvs[j]
					break
				}
			}
			if akv == nil {
				ao.Set(ar, k, r)
				continue
			}
			n, err := MergeValues(ar, akv.v, r)
			if err != nil {
				return nil, err
			}
			akv.v = n
		}
		return a, nil
	case TypeArray:
		aa, _ := a.Array()
		ba, _ := b.Array()
		if len(aa) == 0 {
			return b, nil
		}
		if len(ba) == 0 {
			return a, nil
		}
		if len(aa) != len(ba) {
			return nil, ErrMergeDifferingArrayLengths
		}
		for i := range aa {
			n, err := MergeValues(ar, aa[i], ba[i])
			if err != nil {
				return nil, err
			}
			aa[i] = n
		}
		return a, nil
	case TypeTrue, TypeFalse, TypeNull, TypeNumber, TypeString:
		return b, nil
	default:
		return nil, ErrMergeUnknownType
	}
}

// MergeValuesWithPath wraps b in a nested object structure at the given path,
// then merges the result into a using [MergeValues]. For example, with
// path ["foo", "bar"], b is wrapped as {"foo": {"bar": b}} before merging.
//
// If path is empty, behaves identically to [MergeValues].
//
// The arena ar is used for allocating the wrapper objects and during the merge.
func MergeValuesWithPath(ar arena.Arena, a, b *Value, path ...string) (*Value, error) {
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
