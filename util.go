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

// NullValue is a heap-allocated JSON null singleton. It is safe to use from
// any context (heap or arena) since it is a package-level global that is always
// reachable by the GC.
var (
	NullValue = MustParse(`null`)
)

// AppendToArray appends value to the end of array. Does nothing if array is
// not of TypeArray. The arena a is used to grow the array's backing slice.
//
// GC safety: when array is arena-allocated (a is non-nil), value must also be
// arena-allocated from the same arena, or be a package-level singleton.
// See the package documentation section "Mixing Arena and Heap Values".
func AppendToArray(a arena.Arena, array, value *Value) {
	if array.Type() != TypeArray {
		return
	}
	items, _ := array.Array()
	array.SetArrayItem(a, len(items), value)
}

// SetValue sets value at the nested key path within v. Intermediate object
// nodes are created on the arena as needed when they don't exist. The path
// must have at least one element.
//
// Object keys created along the path are copied onto the arena when a is
// non-nil, ensuring GC safety. The same arena/heap mixing rules as
// [Object.Set] apply to the value argument.
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

// SetNull sets a null value at the nested key path within v. The null Value is
// allocated on the arena when a is non-nil. See [SetValue] for path behavior.
func SetNull(a arena.Arena, v *Value, path ...string) {
	null := arena.Allocate[Value](a)
	null.t = TypeNull
	SetValue(a, v, null, path...)
}

// ValueIsNonNull reports whether v is non-nil and not TypeNull.
func ValueIsNonNull(v *Value) bool {
	if v == nil {
		return false
	}
	if v.Type() == TypeNull {
		return false
	}
	return true
}

// AppendArrayItems appends all elements from right into v. Both v and right
// must be TypeArray; does nothing otherwise. The arena a is used to grow v's
// backing slice.
//
// GC safety: when v is arena-allocated (a is non-nil), right and its elements
// must also be arena-allocated from the same arena. See the package
// documentation section "Mixing Arena and Heap Values".
func (v *Value) AppendArrayItems(a arena.Arena, right *Value) {
	if v.t != TypeArray || right.t != TypeArray {
		return
	}
	for _, item := range right.a {
		v.a = arena.SliceAppend(a, v.a, item)
	}
}

// ValueIsNull reports whether v is nil or TypeNull.
func ValueIsNull(v *Value) bool {
	return !ValueIsNonNull(v)
}

// DeduplicateObjectKeysRecursively removes duplicate object keys from v and
// all nested objects and arrays, keeping the first occurrence of each key.
// This modifies v in place and does not require an arena.
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
	// Heap-allocated: maps cannot be placed on the arena. The allocation is
	// bounded by the number of unique keys at each object level.
	seen := make(map[string]struct{})
	n := 0
	for _, kv := range o.kvs {
		if _, ok := seen[kv.k]; ok {
			continue
		}
		seen[kv.k] = struct{}{}
		o.kvs[n] = kv
		n++
		DeduplicateObjectKeysRecursively(kv.v)
	}
	for i := n; i < len(o.kvs); i++ {
		o.kvs[i] = nil // clear trailing slots for GC
	}
	o.kvs = o.kvs[:n]
}
