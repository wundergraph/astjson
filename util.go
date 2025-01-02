package astjson

import (
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

func (v *Value) AppendArrayItems(right *Value) {
	if v.t != TypeArray || right.t != TypeArray {
		return
	}
	v.a = append(v.a, right.a...)
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
