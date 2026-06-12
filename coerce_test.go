package astjson

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/wundergraph/go-arena"
)

func TestCoerceToString(t *testing.T) {
	t.Parallel()

	t.Run("number integer", func(t *testing.T) {
		t.Parallel()
		v := MustParse(`1`)
		v = v.CoerceToString(nil)
		require.Equal(t, TypeString, v.Type())
		require.Equal(t, `"1"`, string(v.MarshalTo(nil)))
	})

	t.Run("number float", func(t *testing.T) {
		t.Parallel()
		v := MustParse(`3.14`)
		v = v.CoerceToString(nil)
		require.Equal(t, TypeString, v.Type())
		require.Equal(t, `"3.14"`, string(v.MarshalTo(nil)))
	})

	t.Run("number negative exponent", func(t *testing.T) {
		t.Parallel()
		v := MustParse(`-1.5e10`)
		v = v.CoerceToString(nil)
		require.Equal(t, TypeString, v.Type())
		require.Equal(t, `"-1.5e10"`, string(v.MarshalTo(nil)))
	})

	t.Run("true", func(t *testing.T) {
		t.Parallel()
		v := MustParse(`true`)
		v = v.CoerceToString(nil)
		require.Equal(t, TypeString, v.Type())
		require.Equal(t, `"true"`, string(v.MarshalTo(nil)))
	})

	t.Run("false", func(t *testing.T) {
		t.Parallel()
		v := MustParse(`false`)
		v = v.CoerceToString(nil)
		require.Equal(t, TypeString, v.Type())
		require.Equal(t, `"false"`, string(v.MarshalTo(nil)))
	})

	t.Run("null", func(t *testing.T) {
		t.Parallel()
		v := MustParse(`null`)
		v = v.CoerceToString(nil)
		require.Equal(t, TypeString, v.Type())
		require.Equal(t, `"null"`, string(v.MarshalTo(nil)))
	})

	t.Run("string unchanged", func(t *testing.T) {
		t.Parallel()
		v := MustParse(`"hello"`)
		v = v.CoerceToString(nil)
		require.Equal(t, TypeString, v.Type())
		require.Equal(t, `"hello"`, string(v.MarshalTo(nil)))
	})

	t.Run("string with escapes preserved", func(t *testing.T) {
		t.Parallel()
		v := MustParse(`"a\"b"`)
		v = v.CoerceToString(nil)
		require.Equal(t, TypeString, v.Type())
		require.Equal(t, `"a\"b"`, string(v.MarshalTo(nil)))
	})

	t.Run("object with nil arena", func(t *testing.T) {
		t.Parallel()
		v := MustParse(`{"a":1,"b":"x"}`)
		v = v.CoerceToString(nil)
		require.Equal(t, TypeString, v.Type())
		require.Equal(t, `"{\"a\":1,\"b\":\"x\"}"`, string(v.MarshalTo(nil)))
	})

	t.Run("object with arena", func(t *testing.T) {
		t.Parallel()
		a := arena.NewMonotonicArena()
		v, err := ParseWithArena(a, `{"k":true}`)
		require.NoError(t, err)
		orig := v
		coerced := v.CoerceToString(a)
		require.Equal(t, TypeString, coerced.Type())
		require.Equal(t, `"{\"k\":true}"`, string(coerced.MarshalTo(nil)))
		require.Equal(t, TypeObject, orig.Type(), "source must not be mutated")
		require.Equal(t, `{"k":true}`, string(orig.MarshalTo(nil)))
		require.NotSame(t, orig, coerced)
	})

	t.Run("true with arena", func(t *testing.T) {
		t.Parallel()
		a := arena.NewMonotonicArena()
		v, err := ParseWithArena(a, `true`)
		require.NoError(t, err)
		orig := v
		coerced := v.CoerceToString(a)
		require.Equal(t, TypeString, coerced.Type())
		require.Equal(t, `"true"`, string(coerced.MarshalTo(nil)))
		require.Equal(t, TypeTrue, orig.Type(), "source must not be mutated")
		require.NotSame(t, orig, coerced)
	})

	t.Run("false with arena", func(t *testing.T) {
		t.Parallel()
		a := arena.NewMonotonicArena()
		v, err := ParseWithArena(a, `false`)
		require.NoError(t, err)
		v = v.CoerceToString(a)
		require.Equal(t, TypeString, v.Type())
		require.Equal(t, `"false"`, string(v.MarshalTo(nil)))
	})

	t.Run("null with arena", func(t *testing.T) {
		t.Parallel()
		a := arena.NewMonotonicArena()
		v, err := ParseWithArena(a, `null`)
		require.NoError(t, err)
		v = v.CoerceToString(a)
		require.Equal(t, TypeString, v.Type())
		require.Equal(t, `"null"`, string(v.MarshalTo(nil)))
	})

	t.Run("number with arena", func(t *testing.T) {
		t.Parallel()
		a := arena.NewMonotonicArena()
		v, err := ParseWithArena(a, `42`)
		require.NoError(t, err)
		orig := v
		coerced := v.CoerceToString(a)
		require.Equal(t, TypeString, coerced.Type())
		require.Equal(t, `"42"`, string(coerced.MarshalTo(nil)))
		require.Equal(t, TypeNumber, orig.Type(), "source must not be mutated")
		require.NotSame(t, orig, coerced)
	})

	t.Run("array with arena", func(t *testing.T) {
		t.Parallel()
		a := arena.NewMonotonicArena()
		v, err := ParseWithArena(a, `[1,"x",true,null]`)
		require.NoError(t, err)
		orig := v
		coerced := v.CoerceToString(a)
		require.Equal(t, TypeString, coerced.Type())
		require.Equal(t, `"[1,\"x\",true,null]"`, string(coerced.MarshalTo(nil)))
		require.Equal(t, TypeArray, orig.Type(), "source must not be mutated")
		require.Equal(t, `[1,"x",true,null]`, string(orig.MarshalTo(nil)))
		require.NotSame(t, orig, coerced)
	})

	t.Run("empty object", func(t *testing.T) {
		t.Parallel()
		v := MustParse(`{}`)
		v = v.CoerceToString(nil)
		require.Equal(t, TypeString, v.Type())
		require.Equal(t, `"{}"`, string(v.MarshalTo(nil)))
	})

	t.Run("array", func(t *testing.T) {
		t.Parallel()
		v := MustParse(`[1,2,3]`)
		v = v.CoerceToString(nil)
		require.Equal(t, TypeString, v.Type())
		require.Equal(t, `"[1,2,3]"`, string(v.MarshalTo(nil)))
	})

	t.Run("array with strings", func(t *testing.T) {
		t.Parallel()
		v := MustParse(`["a","b"]`)
		v = v.CoerceToString(nil)
		require.Equal(t, TypeString, v.Type())
		require.Equal(t, `"[\"a\",\"b\"]"`, string(v.MarshalTo(nil)))
	})

	t.Run("empty array", func(t *testing.T) {
		t.Parallel()
		v := MustParse(`[]`)
		v = v.CoerceToString(nil)
		require.Equal(t, TypeString, v.Type())
		require.Equal(t, `"[]"`, string(v.MarshalTo(nil)))
	})

	t.Run("nested", func(t *testing.T) {
		t.Parallel()
		v := MustParse(`{"a":[1,{"b":null}]}`)
		v = v.CoerceToString(nil)
		require.Equal(t, TypeString, v.Type())
		require.Equal(t, `"{\"a\":[1,{\"b\":null}]}"`, string(v.MarshalTo(nil)))
	})

	t.Run("StringBytes after coercion", func(t *testing.T) {
		t.Parallel()
		v := MustParse(`42`)
		v = v.CoerceToString(nil)
		b, err := v.StringBytes()
		require.NoError(t, err)
		require.Equal(t, "42", string(b))
	})
}

func TestCoerceToStringNoAllocForStringPassthrough(t *testing.T) {
	v := MustParse(`"hello"`)
	allocs := testing.AllocsPerRun(100, func() {
		v = v.CoerceToString(nil)
	})
	require.Zero(t, allocs, "TypeString returns v unchanged; must not allocate")
}

func TestCoerceToStringArenaBackedNoHeapAlloc(t *testing.T) {
	a := arena.NewMonotonicArena()
	num, err := ParseWithArena(a, `42`)
	require.NoError(t, err)
	tru, err := ParseWithArena(a, `true`)
	require.NoError(t, err)

	numAllocs := testing.AllocsPerRun(100, func() {
		num.CoerceToString(a)
	})
	require.Zero(t, numAllocs, "arena-backed number coercion must not hit heap")

	truAllocs := testing.AllocsPerRun(100, func() {
		tru.CoerceToString(a)
	})
	require.Zero(t, truAllocs, "arena-backed bool coercion must not hit heap")
}

// Regression test for the aliasing bug: StructuralCopy aliases scalars
// from source to copy. Mutating the alias via CoerceToString used to
// corrupt the source. CoerceToString must always allocate a new Value.
func TestCoerceToStringDoesNotCorruptStructurallyCopiedScalar(t *testing.T) {
	a := arena.NewMonotonicArena()
	var p Parser
	src, err := p.ParseWithArena(a, `42`)
	require.NoError(t, err)

	cp := p.StructuralCopy(a, src)
	// Scalars are aliased, so src and cp may be the same pointer.
	coerced := cp.CoerceToString(a)

	require.Equal(t, TypeString, coerced.Type())
	require.Equal(t, TypeNumber, src.Type(), "source must not be mutated through aliased scalar")
	require.Equal(t, `42`, string(src.MarshalTo(nil)))
}
