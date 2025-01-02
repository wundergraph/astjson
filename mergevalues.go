package astjson

import (
	"bytes"
	"errors"
)

var (
	ErrMergeDifferentTypes        = errors.New("cannot merge different types")
	ErrMergeDifferingArrayLengths = errors.New("cannot merge arrays of differing lengths")
	ErrMergeUnknownType           = errors.New("cannot merge unknown type")
)

func MergeValues(a, b *Value) (v *Value, changed bool, err error) {
	if a == nil {
		return b, true, nil
	}
	if b == nil {
		return a, false, nil
	}
	aBool, bBool := a.Type() == TypeTrue || a.Type() == TypeFalse, b.Type() == TypeTrue || b.Type() == TypeFalse
	booleans := aBool && bBool
	oneIsNull := a.Type() == TypeNull || b.Type() == TypeNull
	if a.Type() != b.Type() && !booleans && !oneIsNull {
		return nil, false, ErrMergeDifferentTypes
	}
	switch a.Type() {
	case TypeObject:
		ao, _ := a.Object()
		bo, _ := b.Object()
		ao.unescapeKeys()
		bo.unescapeKeys()
		for i := range bo.kvs {
			k := bo.kvs[i].k
			r := bo.kvs[i].v
			l := ao.Get(k)
			if l == nil {
				ao.Set(k, r)
				continue
			}
			n, changed, err := MergeValues(l, r)
			if err != nil {
				return nil, false, err
			}
			if changed {
				ao.Set(k, n)
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
			n, changed, err := MergeValues(aa[i], ba[i])
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

func MergeValuesWithPath(a, b *Value, path ...string) (v *Value, changed bool, err error) {
	if len(path) == 0 {
		return MergeValues(a, b)
	}
	root := &Value{
		t: TypeObject,
	}
	current := root
	for i := 0; i < len(path)-1; i++ {
		current.Set(path[i], &Value{t: TypeObject})
		current = current.Get(path[i])
	}
	current.Set(path[len(path)-1], b)
	return MergeValues(a, root)
}
