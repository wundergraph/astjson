package astjson

import (
	"strconv"

	"github.com/wundergraph/go-arena"
)

// Del deletes the entry with the given key from o.
func (o *Object) Del(key string) {
	if o == nil {
		return
	}
	// Keys are always pre-unescaped during parsing and Object.Set,
	// so direct comparison is sufficient.
	for i, kv := range o.kvs {
		if kv.k == key {
			o.kvs = append(o.kvs[:i], o.kvs[i+1:]...)
			o.kvs[:len(o.kvs)+1][len(o.kvs)] = nil // clear hidden slot for GC
			return
		}
	}
}

// Del deletes the entry with the given key from array or object v.
func (v *Value) Del(key string) {
	if v == nil {
		return
	}
	if v.t == TypeObject {
		v.o.Del(key)
		return
	}
	if v.t == TypeArray {
		n, err := strconv.Atoi(key)
		if err != nil || n < 0 || n >= len(v.a) {
			return
		}
		v.a = append(v.a[:n], v.a[n+1:]...)
		v.a[:len(v.a)+1][len(v.a)] = nil // clear hidden slot for GC
	}
}

// Set sets (key, value) entry in the o.
//
// The value must be unchanged during o lifetime.
//
// GC safety: when o is arena-allocated (a is non-nil), value must also be
// arena-allocated from the same arena, or be a package-level singleton.
// Storing a heap-allocated *Value in arena memory is unsafe because the GC
// does not trace pointers within arena buffers. Use [DeepCopy] to copy a
// heap-allocated value onto the arena before passing it here. See the package
// documentation section "Mixing Arena and Heap Values" for details.
func (o *Object) Set(a arena.Arena, key string, value *Value) {
	if o == nil {
		return
	}
	if value == nil {
		value = valueNull
	}

	// Try substituting already existing entry with the given key.
	for i := range o.kvs {
		if !o.kvs[i].keyUnescaped {
			o.unescapeKey(a, o.kvs[i])
		}
		if o.kvs[i].k == key {
			o.kvs[i].v = value
			return
		}
	}

	// Add new entry.
	kv := o.getKV(a)
	kv.k = arenaString(a, key)
	kv.v = value
	kv.keyUnescaped = true // New keys are already unescaped since they come from user input
}

// Set sets (key, value) entry in the array or object v.
//
// The value must be unchanged during v lifetime.
func (v *Value) Set(a arena.Arena, key string, value *Value) {
	if v == nil {
		return
	}
	if v.t == TypeObject {
		v.o.Set(a, key, value)
		return
	}
	if v.t == TypeArray {
		idx, err := strconv.Atoi(key)
		if err != nil || idx < 0 {
			return
		}
		v.SetArrayItem(a, idx, value)
	}
}

// SetArrayItem sets the value in the array v at idx position.
//
// The value must be unchanged during v lifetime.
//
// GC safety: when v is arena-allocated (a is non-nil), value must also be
// arena-allocated from the same arena, or be a package-level singleton.
// Use [DeepCopy] to copy a heap-allocated value onto the arena before passing
// it here. See the package documentation section "Mixing Arena and Heap Values".
func (v *Value) SetArrayItem(a arena.Arena, idx int, value *Value) {
	if v == nil || v.t != TypeArray {
		return
	}
	for idx >= len(v.a) {
		v.a = arena.SliceAppend(a, v.a, valueNull)
	}
	v.a[idx] = value
}
