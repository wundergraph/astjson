package astjson

import (
	"strconv"

	"github.com/wundergraph/go-arena"
)

func StringValue(a arena.Arena, s string) *Value {
	v := arena.Allocate[Value](a)
	v.t = TypeString
	v.s = arenaString(a, s)
	return v
}

func StringValueBytes(a arena.Arena, b []byte) *Value {
	v := arena.Allocate[Value](a)
	v.t = TypeString
	v.s = b2s(b)
	return v
}

func IntValue(a arena.Arena, i int) *Value {
	v := arena.Allocate[Value](a)
	v.t = TypeNumber
	v.s = arenaString(a, strconv.Itoa(i))
	return v
}

func FloatValue(a arena.Arena, f float64) *Value {
	v := arena.Allocate[Value](a)
	v.t = TypeNumber
	v.s = arenaString(a, strconv.FormatFloat(f, 'g', -1, 64))
	return v
}

func NumberValue(a arena.Arena, s string) *Value {
	v := arena.Allocate[Value](a)
	v.t = TypeNumber
	v.s = arenaString(a, s)
	return v
}

func TrueValue(a arena.Arena) *Value {
	v := arena.Allocate[Value](a)
	v.t = TypeTrue
	return v
}

func FalseValue(a arena.Arena) *Value {
	v := arena.Allocate[Value](a)
	v.t = TypeFalse
	return v
}

func ObjectValue(a arena.Arena) *Value {
	v := arena.Allocate[Value](a)
	v.t = TypeObject
	return v
}

func ArrayValue(a arena.Arena) *Value {
	v := arena.Allocate[Value](a)
	v.t = TypeArray
	return v
}
