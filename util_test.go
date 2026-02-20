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
	SetNull(nil, a, "name")
	out := a.MarshalTo(nil)
	require.Equal(t, `{"name":null}`, string(out))

	b := MustParse(`{"person":{"name":"Jens"}}`)
	SetNull(nil, b, "person", "name")
	out = b.MarshalTo(nil)
	require.Equal(t, `{"person":{"name":null}}`, string(out))
}

func TestSetWithNonExistingPath(t *testing.T) {
	a := MustParse(`{}`)
	SetValue(nil, a, MustParse(`1`), "a", "b")
	out := a.MarshalTo(nil)
	require.Equal(t, `{"a":{"b":1}}`, string(out))
}

func TestAppendToArray(t *testing.T) {
	a := MustParse(`[1,2]`)
	AppendToArray(nil, a, MustParse(`3`))
	out := a.MarshalTo(nil)
	require.Equal(t, `[1,2,3]`, string(out))
}

func TestAppendToArrayNonArray(t *testing.T) {
	v := MustParse(`"not an array"`)
	AppendToArray(nil, v, MustParse(`1`))
	require.Equal(t, TypeString, v.Type())
}

func TestValueIsNonNullNil(t *testing.T) {
	require.False(t, ValueIsNonNull(nil))
}

func TestAppendArrayItemsNonArray(t *testing.T) {
	left := MustParse(`"not array"`)
	right := MustParse(`[1,2]`)
	left.AppendArrayItems(nil, right)
	require.Equal(t, TypeString, left.Type())

	left2 := MustParse(`[1,2]`)
	right2 := MustParse(`"not array"`)
	left2.AppendArrayItems(nil, right2)
	require.Equal(t, 2, len(left2.GetArray()))
}

func TestDeduplicateObjectKeysRecursivelyArray(t *testing.T) {
	v := MustParse(`[{"a":1,"a":2},{"b":1}]`)
	DeduplicateObjectKeysRecursively(v)
	arr := v.GetArray()
	require.Equal(t, 1, arr[0].GetObject().Len())
}

func TestDeduplicateObjectKeysRecursivelyTripleDuplicate(t *testing.T) {
	v := MustParse(`{"a":1,"a":2,"a":3}`)
	DeduplicateObjectKeysRecursively(v)
	o := v.GetObject()
	require.Equal(t, 1, o.Len())
	require.Equal(t, "1", o.Get("a").String())
}

func TestStringValueBytesNilArena(t *testing.T) {
	b := []byte("hello")
	v := StringValueBytes(nil, b)
	sb, err := v.StringBytes()
	require.NoError(t, err)
	require.Equal(t, "hello", string(sb))
}
