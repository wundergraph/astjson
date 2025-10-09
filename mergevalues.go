package astjson

import (
	"bytes"
	"errors"

	"github.com/wundergraph/go-arena"
)

var (
	ErrMergeDifferentTypes        = errors.New("cannot merge different types")
	ErrMergeDifferingArrayLengths = errors.New("cannot merge arrays of differing lengths")
	ErrMergeUnknownType           = errors.New("cannot merge unknown type")
)

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
		ao.unescapeKeys(ar)
		bo.unescapeKeys(ar)
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
		if b.Type() != TypeNull {
			return b, true, nil
		}
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

func MergeValuesWithPath(ar arena.Arena, a, b *Value, path ...string) (v *Value, changed bool, err error) {
	if len(path) == 0 {
		return MergeValues(ar, a, b)
	}
	root := &Value{
		t: TypeObject,
	}
	current := root
	for i := 0; i < len(path)-1; i++ {
		current.Set(ar, path[i], &Value{t: TypeObject})
		current = current.Get(path[i])
	}
	current.Set(ar, path[len(path)-1], b)
	return MergeValues(ar, a, root)
}
