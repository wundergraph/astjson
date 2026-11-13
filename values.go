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
	return v
}

// TrueValue returns the JSON true Value.
//
// It returns the same shared Value the parser returns for a parsed `true`, so
// it allocates nothing and a is unused. The shared Value is a package-level
// variable the GC always sees, which makes it safe to store in both heap and
// arena containers.
func TrueValue(_ arena.Arena) *Value {
	return valueTrue
}

// FalseValue returns the JSON false Value.
//
// It returns the same shared Value the parser returns for a parsed `false`, so
// it allocates nothing and a is unused. See [TrueValue].
func FalseValue(_ arena.Arena) *Value {
	return valueFalse
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
	return v
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
	return v
}
