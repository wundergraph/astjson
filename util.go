package astjson

import (
	"unsafe"

	"github.com/wundergraph/go-arena"
)

func b2s(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

func s2b(s string) (b []byte) {
	return unsafe.Slice(unsafe.StringData(s), len(s))
}

// arenaString copies the string bytes onto the arena when a is non-nil.
// When a is nil, returns s unchanged (heap string in heap struct is GC-safe).
func arenaString(a arena.Arena, s string) string {
	if a == nil {
		return s
	}
	b := arena.AllocateSlice[byte](a, len(s), len(s))
	copy(b, s)
	return b2s(b)
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

func AppendToArray(a arena.Arena, array, value *Value) {
	if array.Type() != TypeArray {
		return
	}
	items, _ := array.Array()
	array.SetArrayItem(a, len(items), value)
}

func SetValue(a arena.Arena, v *Value, value *Value, path ...string) {
	for i := 0; i < len(path)-1; i++ {
		parent := v
		v = v.Get(path[i])
		if v == nil {
			child := ObjectValue(a)
			parent.Set(a, path[i], child)
			v = parent.Get(path[i])
		}
	}
	v.Set(a, path[len(path)-1], value)
}

func SetNull(a arena.Arena, v *Value, path ...string) {
	null := arena.Allocate[Value](a)
	null.t = TypeNull
	SetValue(a, v, null, path...)
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

func (v *Value) AppendArrayItems(a arena.Arena, right *Value) {
	if v.t != TypeArray || right.t != TypeArray {
		return
	}
	for _, item := range right.a {
		v.a = arena.SliceAppend(a, v.a, item)
	}
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
