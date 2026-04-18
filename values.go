package astjson

import (
	"strconv"

	"github.com/wundergraph/go-arena"
)

// StringValue creates a JSON string Value containing s.
//
// When a is non-nil, both the Value struct and the string's backing bytes are
// allocated on the arena (the string is copied). The caller may drop references
// to s immediately.
//
// When a is nil, the Value is heap-allocated and references s directly. The
// caller must keep s reachable for the lifetime of the returned Value.
func StringValue(a arena.Arena, s string) *Value {
	v := arena.Allocate[Value](a)
	v.t = TypeString
	v.s = arenaString(a, s)
	v.stringNeedsEscape = hasSpecialChars(s)
	v.noEscapeSubtree = !v.stringNeedsEscape
	return v
}

// StringValueBytes creates a JSON string Value from a byte slice.
//
// When a is non-nil, both the Value struct and the bytes are copied onto the
// arena. The caller may reuse or discard b immediately.
//
// When a is nil, the Value is heap-allocated and references b's underlying
// memory directly via zero-copy conversion. The caller must not modify b and
// must keep it reachable for the lifetime of the returned Value.
func StringValueBytes(a arena.Arena, b []byte) *Value {
	v := arena.Allocate[Value](a)
	v.t = TypeString
	if a != nil {
		ab := arena.AllocateSlice[byte](a, len(b), len(b))
		copy(ab, b)
		v.s = b2s(ab)
	} else {
		v.s = b2s(b)
	}
	v.stringNeedsEscape = hasSpecialChars(v.s)
	v.noEscapeSubtree = !v.stringNeedsEscape
	return v
}

// IntValue creates a JSON number Value from an int.
//
// When a is non-nil, both the Value struct and the number's string
// representation are allocated on the arena.
// When a is nil, the Value is heap-allocated normally.
func IntValue(a arena.Arena, i int) *Value {
	v := arena.Allocate[Value](a)
	v.t = TypeNumber
	v.s = arenaString(a, strconv.Itoa(i))
	v.noEscapeSubtree = true
	return v
}

// FloatValue creates a JSON number Value from a float64.
//
// When a is non-nil, both the Value struct and the number's string
// representation are allocated on the arena.
// When a is nil, the Value is heap-allocated normally.
func FloatValue(a arena.Arena, f float64) *Value {
	v := arena.Allocate[Value](a)
	v.t = TypeNumber
	v.s = arenaString(a, strconv.FormatFloat(f, 'g', -1, 64))
	v.noEscapeSubtree = true
	return v
}

// NumberValue creates a JSON number Value from a raw numeric string.
//
// The string s must be a valid JSON number (e.g. "123", "3.14", "1e10").
// No validation is performed.
//
// When a is non-nil, both the Value struct and the string's backing bytes are
// allocated on the arena (the string is copied).
// When a is nil, the Value is heap-allocated and references s directly.
func NumberValue(a arena.Arena, s string) *Value {
	v := arena.Allocate[Value](a)
	v.t = TypeNumber
	v.s = arenaString(a, s)
	v.noEscapeSubtree = true
	return v
}

// TrueValue creates a JSON true Value.
//
// When a is non-nil, the Value struct is allocated on the arena.
// When a is nil, it is heap-allocated.
func TrueValue(a arena.Arena) *Value {
	v := arena.Allocate[Value](a)
	v.t = TypeTrue
	v.noEscapeSubtree = true
	return v
}

// FalseValue creates a JSON false Value.
//
// When a is non-nil, the Value struct is allocated on the arena.
// When a is nil, it is heap-allocated.
func FalseValue(a arena.Arena) *Value {
	v := arena.Allocate[Value](a)
	v.t = TypeFalse
	v.noEscapeSubtree = true
	return v
}

// ObjectValue creates an empty JSON object Value.
//
// Use [Object.Set] or [Value.Set] to add entries. Object keys and entry
// backing slices are allocated on the arena when a is non-nil.
//
// When a is nil, the Value is heap-allocated.
func ObjectValue(a arena.Arena) *Value {
	v := arena.Allocate[Value](a)
	v.t = TypeObject
	v.noEscapeSubtree = true
	return v
}

// CoerceToString returns a JSON string Value representing v.
//
// v is never mutated. For TypeString, v is returned unchanged. For
// every other type, a new Value is allocated on a (heap when a is nil).
// This is safe when v may be aliased — notably, [Parser.StructuralCopy]
// aliases scalars from source to copy, so mutating the copy in place
// would corrupt the source.
//
// Contents of the returned string:
//   - number: the original digits (e.g. -1.5e10 → "-1.5e10")
//   - true/false/null: the JSON literal
//   - object/array: the marshaled JSON text (e.g. {"a":1} → "{\"a\":1}")
//
// For numbers and literals, the backing bytes are aliased from v or from
// package-level constants, so no byte-level copy occurs. For objects and
// arrays, the marshaled text is copied onto a.
func (v *Value) CoerceToString(a arena.Arena) *Value {
	switch v.t {
	case TypeString:
		return v
	case TypeNumber:
		return newStringValue(a, v.s)
	case TypeTrue:
		return newStringValue(a, "true")
	case TypeFalse:
		return newStringValue(a, "false")
	case TypeNull:
		return newStringValue(a, "null")
	case TypeObject, TypeArray:
		b := v.MarshalTo(nil)
		nv := arena.Allocate[Value](a)
		nv.t = TypeString
		nv.s = arenaString(a, b2s(b))
		nv.stringNeedsEscape = hasSpecialChars(nv.s)
		nv.noEscapeSubtree = !nv.stringNeedsEscape
		return nv
	default:
		return v
	}
}

func newStringValue(a arena.Arena, s string) *Value {
	nv := arena.Allocate[Value](a)
	nv.t = TypeString
	nv.s = s
	nv.stringNeedsEscape = false
	nv.noEscapeSubtree = true
	return nv
}

// ArrayValue creates an empty JSON array Value.
//
// Use [Value.SetArrayItem] or [AppendToArray] to add elements. The array's
// backing slice is grown on the arena when a is non-nil.
//
// When a is nil, the Value is heap-allocated.
func ArrayValue(a arena.Arena) *Value {
	v := arena.Allocate[Value](a)
	v.t = TypeArray
	v.noEscapeSubtree = true
	return v
}
