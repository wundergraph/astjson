package astjson

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/wundergraph/go-arena"
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

func TestStringValueTracksMarshalEscapeFlags(t *testing.T) {
	plain := StringValue(nil, "plain")
	require.False(t, plain.stringRaw)
	require.False(t, plain.stringHasEscapes)
	require.False(t, plain.stringNeedsEscape)

	escaped := StringValue(nil, "he said \"hi\"\n")
	require.False(t, escaped.stringRaw)
	require.False(t, escaped.stringHasEscapes)
	require.True(t, escaped.stringNeedsEscape)
}

func TestDeepCopyReusesLiteralSingletons(t *testing.T) {
	a := arena.NewMonotonicArena()
	var parser Parser

	require.Same(t, valueTrue, parser.DeepCopy(a, TrueValue(nil)))
	require.Same(t, valueFalse, parser.DeepCopy(a, FalseValue(nil)))
	require.Same(t, valueNull, parser.DeepCopy(a, &Value{t: TypeNull}))
}

func TestDeepCopyReusesNestedLiteralSingletons(t *testing.T) {
	src := ObjectValue(nil)
	src.Set(nil, "t", TrueValue(nil))
	src.Set(nil, "f", FalseValue(nil))
	src.Set(nil, "n", &Value{t: TypeNull})

	arr := ArrayValue(nil)
	arr.SetArrayItem(nil, 0, TrueValue(nil))
	arr.SetArrayItem(nil, 1, FalseValue(nil))
	arr.SetArrayItem(nil, 2, &Value{t: TypeNull})
	src.Set(nil, "arr", arr)

	a := arena.NewMonotonicArena()
	var parser Parser
	cp := parser.DeepCopy(a, src)

	require.NotSame(t, src, cp)
	require.Same(t, valueTrue, cp.Get("t"))
	require.Same(t, valueFalse, cp.Get("f"))
	require.Same(t, valueNull, cp.Get("n"))

	items := cp.GetArray("arr")
	require.Len(t, items, 3)
	require.Same(t, valueTrue, items[0])
	require.Same(t, valueFalse, items[1])
	require.Same(t, valueNull, items[2])
}

func TestPlanDeepCopyScratchReuse(t *testing.T) {
	src := MustParse(`{"a":{"b":"x"},"c":[1,2,3],"d":{"e":{"f":true}}}`)

	var scratch arenaPlanScratch
	plan := planDeepCopyWithScratch(src, &scratch)
	objectCap := cap(scratch.deepCopyObjectSizes)
	arrayCap := cap(scratch.deepCopyArraySizes)
	if objectCap == 0 {
		t.Fatalf("expected object scratch capacity to be retained")
	}
	if arrayCap == 0 {
		t.Fatalf("expected array scratch capacity to be retained")
	}
	if len(plan.objectSizes) == 0 {
		t.Fatalf("expected plan to contain object sizes")
	}
	if len(plan.arraySizes) == 0 {
		t.Fatalf("expected plan to contain array sizes")
	}

	plan = planDeepCopyWithScratch(MustParse(`{"x":1}`), &scratch)
	if cap(scratch.deepCopyObjectSizes) != objectCap {
		t.Fatalf("object scratch capacity changed: got %d want %d", cap(scratch.deepCopyObjectSizes), objectCap)
	}
	if cap(scratch.deepCopyArraySizes) != arrayCap {
		t.Fatalf("array scratch capacity changed: got %d want %d", cap(scratch.deepCopyArraySizes), arrayCap)
	}
	if len(plan.objectSizes) != 1 {
		t.Fatalf("expected second plan to contain one object size, got %d", len(plan.objectSizes))
	}
	if len(plan.arraySizes) != 0 {
		t.Fatalf("expected second plan to contain no array sizes, got %d", len(plan.arraySizes))
	}
}

func TestStructuralCopyClonesContainersAndAliasesLeaves(t *testing.T) {
	a := arena.NewMonotonicArena()

	leafString := StringValue(a, "hello")
	leafNumber := IntValue(a, 42)
	leafBool := TrueValue(a)

	nested := ObjectValue(a)
	nested.Set(a, "name", leafString)

	innerObj := ObjectValue(a)
	innerObj.Set(a, "ok", leafBool)

	arr := ArrayValue(a)
	arr.SetArrayItem(a, 0, leafNumber)
	arr.SetArrayItem(a, 1, innerObj)

	src := ObjectValue(a)
	src.Set(a, "nested", nested)
	src.Set(a, "arr", arr)
	src.Set(a, "scalar", leafString)

	var parser Parser
	cp := parser.StructuralCopy(a, src)

	require.NotSame(t, src, cp)
	require.NotSame(t, nested, cp.Get("nested"))
	require.NotSame(t, arr, cp.Get("arr"))
	require.Same(t, leafString, cp.Get("nested").Get("name"))
	require.Same(t, leafString, cp.Get("scalar"))

	items := cp.GetArray("arr")
	require.Len(t, items, 2)
	require.Same(t, leafNumber, items[0])
	require.NotSame(t, innerObj, items[1])
	require.Same(t, leafBool, items[1].Get("ok"))

	cp.Get("nested").Set(a, "extra", IntValue(a, 7))
	require.Nil(t, nested.Get("extra"))
}

func TestStructuralCopyNilArena(t *testing.T) {
	v := StringValue(nil, "hello")
	var parser Parser
	require.Same(t, v, parser.StructuralCopy(nil, v))
}

func TestStructuralCopyNilValue(t *testing.T) {
	a := arena.NewMonotonicArena()
	var parser Parser
	require.Nil(t, parser.StructuralCopy(a, nil))
}
