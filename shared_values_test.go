package astjson

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wundergraph/go-arena"
)

func TestBooleanValuesAreShared(t *testing.T) {
	// Not parallel: testing.AllocsPerRun does not allow it.
	a := arena.NewMonotonicArena()
	parsed, err := ParseWithArena(a, `[true,false]`)
	require.NoError(t, err)
	items := parsed.GetArray()
	used := a.Len()

	assert.Same(t, items[0], TrueValue(nil), "TrueValue must return the Value the parser returns for true")
	assert.Same(t, items[0], TrueValue(a))
	assert.Same(t, items[1], FalseValue(nil), "FalseValue must return the Value the parser returns for false")
	assert.Same(t, items[1], FalseValue(a))
	assert.Equal(t, used, a.Len(), "boolean values must not take arena memory")

	allocations := testing.AllocsPerRun(100, func() {
		_ = TrueValue(nil)
		_ = FalseValue(nil)
	})
	assert.Zero(t, allocations)
}

func TestSetNullStoresSharedNull(t *testing.T) {
	// Not parallel: testing.AllocsPerRun does not allow it.
	a := arena.NewMonotonicArena()
	v, err := ParseWithArena(a, `{"seller":{"name":"a"},"items":[{"id":1}]}`)
	require.NoError(t, err)
	used := a.Len()

	// Without an arena on arena data: nothing may be allocated anywhere.
	allocations := testing.AllocsPerRun(100, func() {
		SetNull(nil, v, "seller")
		SetNull(nil, v, "items", "0")
	})
	assert.Zero(t, allocations)

	SetNull(a, v, "seller")
	assert.Equal(t, used, a.Len(), "SetNull on an existing path must not take arena memory")

	assert.Same(t, NullValue, v.Get("seller"))
	assert.Same(t, NullValue, v.Get("items", "0"))
	assert.Equal(t, `{"seller":null,"items":[null]}`, string(v.MarshalTo(nil)))
}

func TestSetNullCreatesMissingPath(t *testing.T) {
	t.Parallel()

	v := MustParse(`{"a":1}`)
	SetNull(nil, v, "b", "c")

	assert.Equal(t, `{"a":1,"b":{"c":null}}`, string(v.MarshalTo(nil)))
}

func TestSetValueAndSetNullIgnoreEmptyPath(t *testing.T) {
	t.Parallel()

	v := MustParse(`{"a":1}`)
	SetNull(nil, v)
	SetValue(nil, v, MustParse(`2`))

	assert.Equal(t, `{"a":1}`, string(v.MarshalTo(nil)))
}
