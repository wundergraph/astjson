package astjson

import (
	"strconv"
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
	require.False(t, plain.stringNeedsEscape)

	escaped := StringValue(nil, "he said \"hi\"\n")
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

// TestDeepCopy_BatchEntityPattern replicates the GraphQL batch entity cache
// pattern: parse multiple entities on the same arena using the same parser,
// DeepCopy each one, and place them into an array at specific indices.
// This is the exact pattern used by the GraphQL resolver's mergeBatchCacheHit.
func TestDeepCopy_BatchEntityPattern(t *testing.T) {
	a := arena.NewMonotonicArena(arena.WithMinBufferSize(4096))
	var p Parser

	// Parse two different entities (simulating L2 cache results parsed onto the same arena)
	entity0, err := p.ParseBytesWithArena(a, []byte(`{"id":"r1","body":"A highly effective form of birth control.","author":{"id":"1"}}`))
	require.NoError(t, err)
	entity1, err := p.ParseBytesWithArena(a, []byte(`{"id":"r2","body":"Fedoras are one of the most fashionable hats around.","author":{"id":"2"}}`))
	require.NoError(t, err)

	// Build a response array (simulating entityArray in mergeBatchCacheHit)
	entityArray := ArrayValue(a)
	entityArray.SetArrayItem(a, 0, NullValue)
	entityArray.SetArrayItem(a, 1, NullValue)

	// DeepCopy each entity and place at its batch index (same pattern as loader)
	entityArray.SetArrayItem(a, 0, p.DeepCopy(a, entity0))
	entityArray.SetArrayItem(a, 1, p.DeepCopy(a, entity1))

	// Verify: each array element must have its own distinct body
	items := entityArray.GetArray()
	require.Len(t, items, 2)

	body0 := string(items[0].Get("body").GetStringBytes())
	body1 := string(items[1].Get("body").GetStringBytes())

	require.Equal(t, "A highly effective form of birth control.", body0,
		"entity 0 body must be the Trilby review, not Fedora's")
	require.Equal(t, "Fedoras are one of the most fashionable hats around.", body1,
		"entity 1 body must be the Fedora review")

	// Also verify nested objects are independent
	author0 := string(items[0].Get("author").Get("id").GetStringBytes())
	author1 := string(items[1].Get("author").Get("id").GetStringBytes())
	require.Equal(t, "1", author0)
	require.Equal(t, "2", author1)
}

// TestDeepCopy_BatchEntityPatternWithMerge extends the batch pattern to include
// MergeValues after placing cached entities, simulating the full merge flow.
func TestDeepCopy_BatchEntityPatternWithMerge(t *testing.T) {
	a := arena.NewMonotonicArena(arena.WithMinBufferSize(4096))
	var p Parser

	// Simulate existing items in the response tree (product data already merged)
	item0, err := p.ParseBytesWithArena(a, []byte(`{"name":"Trilby"}`))
	require.NoError(t, err)
	item1, err := p.ParseBytesWithArena(a, []byte(`{"name":"Fedora"}`))
	require.NoError(t, err)

	// Parse cached review entities
	review0, err := p.ParseBytesWithArena(a, []byte(`{"body":"A highly effective form of birth control."}`))
	require.NoError(t, err)
	review1, err := p.ParseBytesWithArena(a, []byte(`{"body":"Fedoras are one of the most fashionable hats around."}`))
	require.NoError(t, err)

	// DeepCopy and merge each review into its item (simulating MergeValues in mergeResult)
	_, err = MergeValues(a, item0, p.DeepCopy(a, review0))
	require.NoError(t, err)
	_, err = MergeValues(a, item1, p.DeepCopy(a, review1))
	require.NoError(t, err)

	// Verify each item has the correct body
	require.Equal(t, "Trilby", string(item0.Get("name").GetStringBytes()))
	require.Equal(t, "A highly effective form of birth control.", string(item0.Get("body").GetStringBytes()),
		"item0 must have Trilby's review body")

	require.Equal(t, "Fedora", string(item1.Get("name").GetStringBytes()))
	require.Equal(t, "Fedoras are one of the most fashionable hats around.", string(item1.Get("body").GetStringBytes()),
		"item1 must have Fedora's review body")
}

// TestDeepCopy_InterleaveParseAndCopy simulates the GraphQL resolver flow:
// parse a large response, parse L2 entities, then DeepCopy the entities.
// All operations use the same parser and arena.
func TestDeepCopy_InterleaveParseAndCopy(t *testing.T) {
	a := arena.NewMonotonicArena(arena.WithMinBufferSize(4096))
	var p Parser

	// Step 1: Parse L2 entities (simulating bulkL2Lookup)
	entity0, err := p.ParseBytesWithArena(a, []byte(`{"id":"r1","body":"A highly effective form of birth control.","author":{"id":"1"}}`))
	require.NoError(t, err)
	entity1, err := p.ParseBytesWithArena(a, []byte(`{"id":"r2","body":"Fedoras are one of the most fashionable hats around.","author":{"id":"2"}}`))
	require.NoError(t, err)

	// Step 2: Parse a root response (simulating mergeResult for the root fetch,
	// which happens BEFORE mergeResult for the entity fetch in Phase 4)
	rootResponse, err := p.ParseBytesWithArena(a, []byte(`{"data":{"topProducts":[{"name":"Trilby","upc":"top-1"},{"name":"Fedora","upc":"top-2"}]}}`))
	require.NoError(t, err)
	require.NotNil(t, rootResponse)

	// Step 3: DeepCopy entities (simulating mergeBatchCacheHit in the entity fetch)
	entityArray := ArrayValue(a)
	entityArray.SetArrayItem(a, 0, NullValue)
	entityArray.SetArrayItem(a, 1, NullValue)
	entityArray.SetArrayItem(a, 0, p.DeepCopy(a, entity0))
	entityArray.SetArrayItem(a, 1, p.DeepCopy(a, entity1))

	// Verify each entity is independent and correct
	items := entityArray.GetArray()
	require.Len(t, items, 2)

	body0 := string(items[0].Get("body").GetStringBytes())
	body1 := string(items[1].Get("body").GetStringBytes())
	require.Equal(t, "A highly effective form of birth control.", body0,
		"entity 0 body corrupted after interleaved parse")
	require.Equal(t, "Fedoras are one of the most fashionable hats around.", body1,
		"entity 1 body corrupted after interleaved parse")
}

// TestDeepCopy_MultipleSequentialCopies verifies that calling DeepCopy
// multiple times in sequence on the same parser produces independent copies.
func TestDeepCopy_MultipleSequentialCopies(t *testing.T) {
	a := arena.NewMonotonicArena(arena.WithMinBufferSize(4096))
	var p Parser

	entities := make([]*Value, 10)
	for i := range entities {
		json := []byte(`{"id":"` + string(rune('a'+i)) + `","value":` + strconv.Itoa(i) + `}`)
		v, err := p.ParseBytesWithArena(a, json)
		require.NoError(t, err)
		entities[i] = v
	}

	// DeepCopy all entities sequentially using same parser
	copies := make([]*Value, len(entities))
	for i, e := range entities {
		copies[i] = p.DeepCopy(a, e)
	}

	// Verify all copies are independent and correct
	for i, cp := range copies {
		id := string(cp.Get("id").GetStringBytes())
		expected := string(rune('a' + i))
		require.Equal(t, expected, id, "copy %d has wrong id", i)
	}
}

// TestDeepCopy_SetAfterCopyDoesNotCorruptSiblings verifies that adding a new
// field to a DeepCopy'd object via Object.Set does not corrupt sibling objects
// in the same DeepCopy'd tree. This is a regression test for a bug where
// allocObjectRefs returned slices with excess capacity extending into the next
// object's slab region, causing arena.SliceAppend to overwrite sibling KV entries.
func TestDeepCopy_SetAfterCopyDoesNotCorruptSiblings(t *testing.T) {
	a := arena.NewMonotonicArena(arena.WithMinBufferSize(4096))
	var p Parser

	// Source: an object containing an array of two sibling objects
	src, err := p.ParseBytesWithArena(a, []byte(`{"items":[{"name":"Alice","id":"1"},{"name":"Bob","id":"2"}]}`))
	require.NoError(t, err)

	// DeepCopy the whole tree
	cp := p.DeepCopy(a, src)

	// Get the two sibling objects from the copied array
	items := cp.Get("items").GetArray()
	require.Len(t, items, 2)
	alice := items[0]
	bob := items[1]

	// Add a new field to Alice — this must NOT corrupt Bob
	alice.Set(a, "extra", StringValue(a, "new-field"))

	// Verify Alice has the new field
	require.Equal(t, "new-field", string(alice.Get("extra").GetStringBytes()))

	// Verify Bob is unchanged
	require.Equal(t, "Bob", string(bob.Get("name").GetStringBytes()),
		"Bob's name was corrupted by Set on Alice")
	require.Equal(t, "2", string(bob.Get("id").GetStringBytes()),
		"Bob's id was corrupted by Set on Alice")
	require.Nil(t, bob.Get("extra"),
		"Bob should not have Alice's extra field")
}

// TestStructuralCopy_SetAfterCopyDoesNotCorruptSiblings is the same test but
// for StructuralCopy. Both use the same slab allocator so both need the fix.
func TestStructuralCopy_SetAfterCopyDoesNotCorruptSiblings(t *testing.T) {
	a := arena.NewMonotonicArena(arena.WithMinBufferSize(4096))
	var p Parser

	src, err := p.ParseBytesWithArena(a, []byte(`{"items":[{"name":"Alice","id":"1"},{"name":"Bob","id":"2"}]}`))
	require.NoError(t, err)

	cp := p.StructuralCopy(a, src)

	items := cp.Get("items").GetArray()
	require.Len(t, items, 2)
	alice := items[0]
	bob := items[1]

	alice.Set(a, "extra", StringValue(a, "new-field"))

	require.Equal(t, "new-field", string(alice.Get("extra").GetStringBytes()))
	require.Equal(t, "Bob", string(bob.Get("name").GetStringBytes()),
		"Bob's name was corrupted by Set on Alice")
	require.Equal(t, "2", string(bob.Get("id").GetStringBytes()),
		"Bob's id was corrupted by Set on Alice")
	require.Nil(t, bob.Get("extra"),
		"Bob should not have Alice's extra field")
}
