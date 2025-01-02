package astjson

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMergeValues(t *testing.T) {
	t.Parallel()
	t.Run("left nil", func(t *testing.T) {
		t.Parallel()
		b := MustParse(`{"b":2}`)
		merged, changed, err := MergeValues(nil, b)
		require.NoError(t, err)
		require.Equal(t, true, changed)
		out := merged.MarshalTo(nil)
		require.Equal(t, `{"b":2}`, string(out))
		out = merged.Get("b").MarshalTo(out[:0])
		require.Equal(t, `2`, string(out))
	})
	t.Run("right nil", func(t *testing.T) {
		t.Parallel()
		a := MustParse(`{"a":1}`)
		merged, changed, err := MergeValues(a, nil)
		require.NoError(t, err)
		require.Equal(t, false, changed)
		out := merged.MarshalTo(nil)
		require.Equal(t, `{"a":1}`, string(out))
		out = merged.Get("a").MarshalTo(out[:0])
		require.Equal(t, `1`, string(out))
	})
	t.Run("type mismatch err", func(t *testing.T) {
		t.Parallel()
		a, b := MustParse(`{"a":1}`), MustParse(`{"a":true}`)
		merged, changed, err := MergeValues(a, b)
		require.Equal(t, ErrMergeDifferentTypes, err)
		require.Nil(t, merged)
		require.Equal(t, false, changed)
	})
	t.Run("bool type mismatch ok", func(t *testing.T) {
		t.Parallel()
		a, b := MustParse(`true`), MustParse(`false`)
		merged, changed, err := MergeValues(a, b)
		require.NoError(t, err)
		require.Equal(t, true, changed)
		out := merged.MarshalTo(nil)
		require.Equal(t, `false`, string(out))
	})
	t.Run("bool type mismatch ok reverse", func(t *testing.T) {
		t.Parallel()
		a, b := MustParse(`false`), MustParse(`true`)
		merged, changed, err := MergeValues(a, b)
		require.NoError(t, err)
		require.Equal(t, true, changed)
		out := merged.MarshalTo(nil)
		require.Equal(t, `true`, string(out))
	})
	t.Run("integers", func(t *testing.T) {
		t.Parallel()
		a, b := MustParse(`1`), MustParse(`2`)
		merged, changed, err := MergeValues(a, b)
		require.NoError(t, err)
		require.Equal(t, true, changed)
		out := merged.MarshalTo(nil)
		require.Equal(t, `2`, string(out))
	})
	t.Run("integers reverse", func(t *testing.T) {
		t.Parallel()
		a, b := MustParse(`2`), MustParse(`1`)
		merged, changed, err := MergeValues(a, b)
		require.NoError(t, err)
		require.Equal(t, true, changed)
		out := merged.MarshalTo(nil)
		require.Equal(t, `1`, string(out))
	})
	t.Run("integers equal", func(t *testing.T) {
		t.Parallel()
		a, b := MustParse(`1`), MustParse(`1`)
		merged, changed, err := MergeValues(a, b)
		require.NoError(t, err)
		require.Equal(t, false, changed)
		out := merged.MarshalTo(nil)
		require.Equal(t, `1`, string(out))
	})
	t.Run("floats", func(t *testing.T) {
		t.Parallel()
		a, b := MustParse(`1.1`), MustParse(`2.2`)
		merged, changed, err := MergeValues(a, b)
		require.NoError(t, err)
		require.Equal(t, true, changed)
		out := merged.MarshalTo(nil)
		require.Equal(t, `2.2`, string(out))
	})
	t.Run("floats reverse", func(t *testing.T) {
		t.Parallel()
		a, b := MustParse(`2.2`), MustParse(`1.1`)
		merged, changed, err := MergeValues(a, b)
		require.NoError(t, err)
		require.Equal(t, true, changed)
		out := merged.MarshalTo(nil)
		require.Equal(t, `1.1`, string(out))
	})
	t.Run("floats equal", func(t *testing.T) {
		t.Parallel()
		a, b := MustParse(`1.1`), MustParse(`1.1`)
		merged, changed, err := MergeValues(a, b)
		require.NoError(t, err)
		require.Equal(t, false, changed)
		out := merged.MarshalTo(nil)
		require.Equal(t, `1.1`, string(out))
	})
	t.Run("arrays", func(t *testing.T) {
		t.Parallel()
		a, b := MustParse(`[1,2]`), MustParse(`[3,4]`)
		merged, changed, err := MergeValues(a, b)
		require.NoError(t, err)
		require.Equal(t, false, changed)
		out := merged.MarshalTo(nil)
		require.Equal(t, `[3,4]`, string(out))
	})
	t.Run("left array empty", func(t *testing.T) {
		t.Parallel()
		a, b := MustParse(`[]`), MustParse(`[1,2]`)
		merged, changed, err := MergeValues(a, b)
		require.NoError(t, err)
		require.Equal(t, true, changed)
		out := merged.MarshalTo(nil)
		require.Equal(t, `[1,2]`, string(out))
	})
	t.Run("right array empty", func(t *testing.T) {
		t.Parallel()
		a, b := MustParse(`[1,2]`), MustParse(`[]`)
		merged, changed, err := MergeValues(a, b)
		require.NoError(t, err)
		require.Equal(t, false, changed)
		out := merged.MarshalTo(nil)
		require.Equal(t, `[1,2]`, string(out))
	})
	t.Run("err differing array lengths", func(t *testing.T) {
		t.Parallel()
		a, b := MustParse(`[1,2]`), MustParse(`[3]`)
		merged, changed, err := MergeValues(a, b)
		require.Equal(t, ErrMergeDifferingArrayLengths, err)
		require.Nil(t, merged)
		require.Equal(t, false, changed)
	})
	t.Run("err merging array item", func(t *testing.T) {
		t.Parallel()
		a, b := MustParse(`[1,2]`), MustParse(`[3,"a"]`)
		merged, changed, err := MergeValues(a, b)
		require.Error(t, err)
		require.Nil(t, merged)
		require.Equal(t, false, changed)
	})
	t.Run("false false", func(t *testing.T) {
		t.Parallel()
		a, b := MustParse(`false`), MustParse(`false`)
		merged, changed, err := MergeValues(a, b)
		require.NoError(t, err)
		require.Equal(t, false, changed)
		out := merged.MarshalTo(nil)
		require.Equal(t, `false`, string(out))
	})
	t.Run("true true", func(t *testing.T) {
		t.Parallel()
		a, b := MustParse(`true`), MustParse(`true`)
		merged, changed, err := MergeValues(a, b)
		require.NoError(t, err)
		require.Equal(t, false, changed)
		out := merged.MarshalTo(nil)
		require.Equal(t, `true`, string(out))
	})
	t.Run("null null", func(t *testing.T) {
		t.Parallel()
		a, b := MustParse(`null`), MustParse(`null`)
		merged, changed, err := MergeValues(a, b)
		require.NoError(t, err)
		require.Equal(t, false, changed)
		out := merged.MarshalTo(nil)
		require.Equal(t, `null`, string(out))
	})
	t.Run("null not null", func(t *testing.T) {
		t.Parallel()
		a, b := MustParse(`null`), MustParse(`1`)
		merged, changed, err := MergeValues(a, b)
		require.NoError(t, err)
		require.Equal(t, true, changed)
		out := merged.MarshalTo(nil)
		require.Equal(t, `1`, string(out))
	})
	t.Run("null not null reverse", func(t *testing.T) {
		t.Parallel()
		a, b := MustParse(`1`), MustParse(`null`)
		merged, changed, err := MergeValues(a, b)
		require.NoError(t, err)
		require.Equal(t, true, changed)
		out := merged.MarshalTo(nil)
		require.Equal(t, `null`, string(out))
	})
	t.Run("array objects", func(t *testing.T) {
		t.Parallel()
		a, b := MustParse(`[{"a":1,"b":2},{"x":1}]`), MustParse(`[{"a":2,"b":3,"c":4},{"y":1}]`)
		merged, changed, err := MergeValues(a, b)
		require.NoError(t, err)
		require.Equal(t, false, changed)
		out := merged.MarshalTo(nil)
		require.Equal(t, `[{"a":2,"b":3,"c":4},{"x":1,"y":1}]`, string(out))
	})
	t.Run("objects", func(t *testing.T) {
		t.Parallel()
		a, b := MustParse(`{"a":{"b":1}}`), MustParse(`{"a":{"c":2}}`)
		merged, changed, err := MergeValues(a, b)
		require.NoError(t, err)
		require.Equal(t, false, changed)
		out := merged.MarshalTo(nil)
		require.Equal(t, `{"a":{"b":1,"c":2}}`, string(out))
	})
	t.Run("strings", func(t *testing.T) {
		t.Parallel()
		a, b := MustParse(`"a"`), MustParse(`"b"`)
		merged, changed, err := MergeValues(a, b)
		require.NoError(t, err)
		require.Equal(t, true, changed)
		out := merged.MarshalTo(nil)
		require.Equal(t, `"b"`, string(out))
	})
	t.Run("strings equal", func(t *testing.T) {
		t.Parallel()
		a, b := MustParse(`"a"`), MustParse(`"a"`)
		merged, changed, err := MergeValues(a, b)
		require.NoError(t, err)
		require.Equal(t, false, changed)
		out := merged.MarshalTo(nil)
		require.Equal(t, `"a"`, string(out))
	})
	t.Run("with path", func(t *testing.T) {
		t.Parallel()
		a, b := MustParse(`{"a":{"b":1}}`), MustParse(`{"c":2}`)
		merged, changed, err := MergeValuesWithPath(a, b, "a")
		require.NoError(t, err)
		require.Equal(t, false, changed)
		out := merged.MarshalTo(nil)
		require.Equal(t, `{"a":{"b":1,"c":2}}`, string(out))
		e := MustParse(`{"e":true}`)
		merged, changed, err = MergeValuesWithPath(merged, e, "a", "d")
		require.NoError(t, err)
		require.Equal(t, false, changed)
		out = merged.MarshalTo(out[:0])
		require.Equal(t, `{"a":{"b":1,"c":2,"d":{"e":true}}}`, string(out))
	})
	t.Run("with empty path", func(t *testing.T) {
		t.Parallel()
		a, b := MustParse(`{"a":1}`), MustParse(`{"b":2}`)
		merged, changed, err := MergeValuesWithPath(a, b)
		require.NoError(t, err)
		require.Equal(t, false, changed)
		out := merged.MarshalTo(nil)
		require.Equal(t, `{"a":1,"b":2}`, string(out))
		out = merged.Get("b").MarshalTo(out[:0])
		require.Equal(t, `2`, string(out))
	})
	t.Run("merge with swap", func(t *testing.T) {
		t.Parallel()
		left := MustParse(`{"a":{"b":1,"c":2,"e":[],"f":[1],"h":[1,2,3]}}`)
		right := MustParse(`{"a":{"b":2,"d":3,"e":[1,2,3],"g":[1],"h":[4,5,6]}}`)
		out, _, err := MergeValues(left, right)
		require.NoError(t, err)
		require.Equal(t, `{"a":{"b":2,"c":2,"e":[1,2,3],"f":[1],"h":[4,5,6],"d":3,"g":[1]}}`, out.String())
	})
}
