package astjson

import (
	"bytes"
	"unsafe"
)

func b2s(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

func s2b(s string) (b []byte) {
	return unsafe.Slice(unsafe.StringData(s), len(s))
}

const maxStartEndStringLen = 80

func startEndString(s string) string {
	if len(s) <= maxStartEndStringLen {
		return s
	}
	start := s[:40]
	end := s[len(s)-40:]
	return start + "..." + end
}

var (
	NullValue = MustParse(`null`)
)

func MergeValues(a, b *Value) (*Value, bool) {
	if a == nil {
		return b, true
	}
	if b == nil {
		return a, false
	}
	if a.Type() != b.Type() {
		return a, false
	}
	switch a.Type() {
	case TypeObject:
		ao, _ := a.Object()
		bo, _ := b.Object()
		ao.Visit(func(key []byte, l *Value) {
			sKey := b2s(key)
			r := bo.Get(sKey)
			if r == nil {
				return
			}
			merged, changed := MergeValues(l, r)
			if changed {
				ao.Set(b2s(key), merged)
			}
		})
		bo.Visit(func(key []byte, r *Value) {
			sKey := b2s(key)
			if ao.Get(sKey) != nil {
				return
			}
			ao.Set(sKey, r)
		})
		return a, false
	case TypeArray:
		aa, _ := a.Array()
		ba, _ := b.Array()
		for i := 0; i < len(ba); i++ {
			a.SetArrayItem(len(aa)+i, ba[i])
		}
		return a, false
	case TypeFalse:
		if b.Type() == TypeTrue {
			return b, true
		}
		return a, false
	case TypeTrue:
		if b.Type() == TypeFalse {
			return b, true
		}
		return a, false
	case TypeNull:
		if b.Type() != TypeNull {
			return b, true
		}
		return a, false
	case TypeNumber:
		af, _ := a.Float64()
		bf, _ := b.Float64()
		if af != bf {
			return b, true
		}
		return a, false
	case TypeString:
		as, _ := a.StringBytes()
		bs, _ := b.StringBytes()
		if !bytes.Equal(as, bs) {
			return b, true
		}
		return a, false
	default:
		return b, true
	}
}

func MergeValuesWithPath(a, b *Value, path ...string) (*Value, bool) {
	if len(path) == 0 {
		return MergeValues(a, b)
	}
	root := MustParseBytes([]byte(`{}`))
	current := root
	for i := 0; i < len(path)-1; i++ {
		current.Set(path[i], MustParseBytes([]byte(`{}`)))
		current = current.Get(path[i])
	}
	current.Set(path[len(path)-1], b)
	return MergeValues(a, root)
}

func AppendToArray(array, value *Value) {
	if array.Type() != TypeArray {
		return
	}
	items, _ := array.Array()
	array.SetArrayItem(len(items), value)
}

func SetValue(v *Value, value *Value, path ...string) {
	for i := 0; i < len(path)-1; i++ {
		parent := v
		v = v.Get(path[i])
		if v == nil {
			child := MustParse(`{}`)
			parent.Set(path[i], child)
			v = child
		}
	}
	v.Set(path[len(path)-1], value)
}

func SetNull(v *Value, path ...string) {
	SetValue(v, MustParse(`null`), path...)
}

func ValueIsNonNull(v *Value) bool {
	if v == nil {
		return false
	}
	if v.Type() == TypeNull {
		return false
	}
	return true
}

func ValueIsNull(v *Value) bool {
	return !ValueIsNonNull(v)
}

func DeduplicateObjectKeysRecursively(v *Value) {
	if v.Type() == TypeArray {
		a := v.GetArray()
		for _, e := range a {
			DeduplicateObjectKeysRecursively(e)
		}
	}
	if v.Type() != TypeObject {
		return
	}
	o, _ := v.Object()
	seen := make(map[string]struct{})
	o.Visit(func(k []byte, v *Value) {
		key := string(k)
		if _, ok := seen[key]; ok {
			o.Del(key)
			return
		} else {
			seen[key] = struct{}{}
		}
		DeduplicateObjectKeysRecursively(v)
	})
}
