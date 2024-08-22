package astjson

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStartEndString(t *testing.T) {
	f := func(s, expectedResult string) {
		t.Helper()
		result := startEndString(s)
		if result != expectedResult {
			t.Fatalf("unexpected result for startEndString(%q); got %q; want %q", s, result, expectedResult)
		}
	}
	f("", "")
	f("foo", "foo")

	getString := func(n int) string {
		b := make([]byte, 0, n)
		for i := 0; i < n; i++ {
			b = append(b, 'a'+byte(i%26))
		}
		return string(b)
	}
	s := getString(maxStartEndStringLen)
	f(s, s)

	f(getString(maxStartEndStringLen+1), "abcdefghijklmnopqrstuvwxyzabcdefghijklmn...pqrstuvwxyzabcdefghijklmnopqrstuvwxyzabc")
	f(getString(100*maxStartEndStringLen), "abcdefghijklmnopqrstuvwxyzabcdefghijklmn...efghijklmnopqrstuvwxyzabcdefghijklmnopqr")
}

func TestMergeValues(t *testing.T) {
	a, b := MustParse(`{"a":1}`), MustParse(`{"b":2}`)
	merged, changed := MergeValues(a, b)
	require.Equal(t, false, changed)
	out := merged.MarshalTo(nil)
	require.Equal(t, `{"a":1,"b":2}`, string(out))
	out = merged.Get("b").MarshalTo(out[:0])
	require.Equal(t, `2`, string(out))
}

func TestMergeValuesArray(t *testing.T) {
	a, b := MustParse(`[1,2]`), MustParse(`[3,4]`)
	merged, changed := MergeValues(a, b)
	require.Equal(t, false, changed)
	out := merged.MarshalTo(nil)
	require.Equal(t, `[1,2,3,4]`, string(out))
}

func TestMergeValuesNestedObjects(t *testing.T) {
	a, b := MustParse(`{"a":{"b":1}}`), MustParse(`{"a":{"c":2}}`)
	merged, changed := MergeValues(a, b)
	require.Equal(t, false, changed)
	out := merged.MarshalTo(nil)
	require.Equal(t, `{"a":{"b":1,"c":2}}`, string(out))
}

func TestMergeValuesWithPath(t *testing.T) {
	a, b := MustParse(`{"a":{"b":1}}`), MustParse(`{"c":2}`)
	merged, changed := MergeValuesWithPath(a, b, "a")
	require.Equal(t, false, changed)
	out := merged.MarshalTo(nil)
	require.Equal(t, `{"a":{"b":1,"c":2}}`, string(out))
	e := MustParse(`{"e":true}`)
	merged, changed = MergeValuesWithPath(merged, e, "a", "d")
	require.Equal(t, false, changed)
	out = merged.MarshalTo(out[:0])
	require.Equal(t, `{"a":{"b":1,"c":2,"d":{"e":true}}}`, string(out))
}

func TestGetArray(t *testing.T) {
	a := MustParse(`[{"name":"Jens"},{"name":"Jannik"}]`)
	arr, err := a.Array()
	require.NoError(t, err)
	require.Equal(t, 2, len(arr))
	jens := arr[0].MarshalTo(nil)
	require.Equal(t, `{"name":"Jens"}`, string(jens))
	jannik := arr[1].MarshalTo(nil)
	require.Equal(t, `{"name":"Jannik"}`, string(jannik))
}

func TestSetNull(t *testing.T) {
	a := MustParse(`{"name":"Jens"}`)
	SetNull(a, "name")
	out := a.MarshalTo(nil)
	require.Equal(t, `{"name":null}`, string(out))

	b := MustParse(`{"person":{"name":"Jens"}}`)
	SetNull(b, "person", "name")
	out = b.MarshalTo(nil)
	require.Equal(t, `{"person":{"name":null}}`, string(out))
}

func TestSetWithNonExistingPath(t *testing.T) {
	a := MustParse(`{}`)
	SetValue(a, MustParse(`1`), "a", "b")
	out := a.MarshalTo(nil)
	require.Equal(t, `{"a":{"b":1}}`, string(out))
}

func TestAppendToArray(t *testing.T) {
	a := MustParse(`[1,2]`)
	AppendToArray(a, MustParse(`3`))
	out := a.MarshalTo(nil)
	require.Equal(t, `[1,2,3]`, string(out))
}
