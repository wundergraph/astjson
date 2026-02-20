package astjson

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"strconv"
	"testing"

	"github.com/wundergraph/go-arena"
)

// forceGC triggers aggressive garbage collection with heap pressure
// to maximize the chance of collecting unreachable heap objects.
func forceGC() {
	// Allocate heap pressure to trigger GC
	waste := make([][]byte, 100)
	for i := range waste {
		waste[i] = make([]byte, 1<<10)
	}
	runtime.GC()
	runtime.GC()
	runtime.KeepAlive(waste)
}

// heapString returns a heap-allocated string that has no other references.
// The //go:noinline directive prevents the compiler from optimizing away
// the allocation or keeping it on the stack.
//
//go:noinline
func heapString(prefix string, i int) string {
	return fmt.Sprintf("%s_%d_%s", prefix, i, "padding_to_ensure_heap_allocation")
}

// heapBytes returns a heap-allocated byte slice that has no other references.
//
//go:noinline
func heapBytes(prefix string, i int) []byte {
	return []byte(fmt.Sprintf("%s_%d_%s", prefix, i, "padding_to_ensure_heap_allocation"))
}

const gcTestIterations = 1000

func TestArenaGCSafety_StringValue(t *testing.T) {
	old := debug.SetGCPercent(1)
	defer debug.SetGCPercent(old)

	for i := 0; i < gcTestIterations; i++ {
		a := arena.NewMonotonicArena()
		s := heapString("hello", i)
		v := StringValue(a, s)
		forceGC()
		got := v.String()
		expected := `"` + s + `"`
		if got != expected {
			t.Fatalf("iteration %d: got %q, want %q", i, got, expected)
		}
		runtime.KeepAlive(a)
	}
}

func TestArenaGCSafety_IntValue(t *testing.T) {
	old := debug.SetGCPercent(1)
	defer debug.SetGCPercent(old)

	for i := 0; i < gcTestIterations; i++ {
		a := arena.NewMonotonicArena()
		v := IntValue(a, i*12345)
		forceGC()
		got := v.String()
		expected := strconv.Itoa(i * 12345)
		if got != expected {
			t.Fatalf("iteration %d: got %q, want %q", i, got, expected)
		}
		runtime.KeepAlive(a)
	}
}

func TestArenaGCSafety_FloatValue(t *testing.T) {
	old := debug.SetGCPercent(1)
	defer debug.SetGCPercent(old)

	for i := 0; i < gcTestIterations; i++ {
		a := arena.NewMonotonicArena()
		f := 3.14159 * float64(i)
		v := FloatValue(a, f)
		forceGC()
		got := v.String()
		expected := strconv.FormatFloat(f, 'g', -1, 64)
		if got != expected {
			t.Fatalf("iteration %d: got %q, want %q", i, got, expected)
		}
		runtime.KeepAlive(a)
	}
}

func TestArenaGCSafety_NumberValue(t *testing.T) {
	old := debug.SetGCPercent(1)
	defer debug.SetGCPercent(old)

	for i := 0; i < gcTestIterations; i++ {
		a := arena.NewMonotonicArena()
		s := heapString("42", i)
		v := NumberValue(a, s)
		forceGC()
		got := v.String()
		if got != s {
			t.Fatalf("iteration %d: got %q, want %q", i, got, s)
		}
		runtime.KeepAlive(a)
	}
}

func TestArenaGCSafety_StringValueBytes(t *testing.T) {
	old := debug.SetGCPercent(1)
	defer debug.SetGCPercent(old)

	for i := 0; i < gcTestIterations; i++ {
		a := arena.NewMonotonicArena()
		// Allocate the byte slice on the arena so it shares the arena's lifetime
		src := heapBytes("bytes", i)
		b := arena.AllocateSlice[byte](a, len(src), len(src))
		copy(b, src)
		v := StringValueBytes(a, b)
		forceGC()
		got := v.String()
		expected := `"` + string(src) + `"`
		if got != expected {
			t.Fatalf("iteration %d: got %q, want %q", i, got, expected)
		}
		runtime.KeepAlive(a)
	}
}

func TestArenaGCSafety_TrueValue(t *testing.T) {
	old := debug.SetGCPercent(1)
	defer debug.SetGCPercent(old)

	for i := 0; i < gcTestIterations; i++ {
		a := arena.NewMonotonicArena()
		v := TrueValue(a)
		forceGC()
		if v.Type() != TypeTrue {
			t.Fatalf("iteration %d: expected TypeTrue, got %s", i, v.Type())
		}
		if v.String() != "true" {
			t.Fatalf("iteration %d: got %q, want %q", i, v.String(), "true")
		}
		runtime.KeepAlive(a)
	}
}

func TestArenaGCSafety_FalseValue(t *testing.T) {
	old := debug.SetGCPercent(1)
	defer debug.SetGCPercent(old)

	for i := 0; i < gcTestIterations; i++ {
		a := arena.NewMonotonicArena()
		v := FalseValue(a)
		forceGC()
		if v.Type() != TypeFalse {
			t.Fatalf("iteration %d: expected TypeFalse, got %s", i, v.Type())
		}
		if v.String() != "false" {
			t.Fatalf("iteration %d: got %q, want %q", i, v.String(), "false")
		}
		runtime.KeepAlive(a)
	}
}

func TestArenaGCSafety_ObjectValue(t *testing.T) {
	old := debug.SetGCPercent(1)
	defer debug.SetGCPercent(old)

	for i := 0; i < gcTestIterations; i++ {
		a := arena.NewMonotonicArena()
		v := ObjectValue(a)
		forceGC()
		if v.Type() != TypeObject {
			t.Fatalf("iteration %d: expected TypeObject, got %s", i, v.Type())
		}
		if v.String() != "{}" {
			t.Fatalf("iteration %d: got %q, want %q", i, v.String(), "{}")
		}
		runtime.KeepAlive(a)
	}
}

func TestArenaGCSafety_ArrayValue(t *testing.T) {
	old := debug.SetGCPercent(1)
	defer debug.SetGCPercent(old)

	for i := 0; i < gcTestIterations; i++ {
		a := arena.NewMonotonicArena()
		v := ArrayValue(a)
		forceGC()
		if v.Type() != TypeArray {
			t.Fatalf("iteration %d: expected TypeArray, got %s", i, v.Type())
		}
		if v.String() != "[]" {
			t.Fatalf("iteration %d: got %q, want %q", i, v.String(), "[]")
		}
		runtime.KeepAlive(a)
	}
}

func TestArenaGCSafety_ObjectSet(t *testing.T) {
	old := debug.SetGCPercent(1)
	defer debug.SetGCPercent(old)

	for i := 0; i < gcTestIterations; i++ {
		a := arena.NewMonotonicArena()
		obj := ObjectValue(a)
		key := heapString("key", i)
		val := StringValue(a, heapString("val", i))
		obj.Set(a, key, val)
		forceGC()
		got := string(obj.MarshalTo(nil))
		expected := `{"` + key + `":"` + heapString("val", i) + `"}`
		if got != expected {
			t.Fatalf("iteration %d: got %q, want %q", i, got, expected)
		}
		runtime.KeepAlive(a)
	}
}

func TestArenaGCSafety_ValueSet(t *testing.T) {
	old := debug.SetGCPercent(1)
	defer debug.SetGCPercent(old)

	for i := 0; i < gcTestIterations; i++ {
		a := arena.NewMonotonicArena()
		// Test on object
		obj := ObjectValue(a)
		key := heapString("key", i)
		val := IntValue(a, i)
		obj.Set(a, key, val)
		forceGC()
		result := obj.Get(key)
		if result == nil {
			t.Fatalf("iteration %d: expected non-nil result", i)
		}
		n, err := result.Int()
		if err != nil {
			t.Fatalf("iteration %d: unexpected error: %s", i, err)
		}
		if n != i {
			t.Fatalf("iteration %d: got %d, want %d", i, n, i)
		}

		// Test on array
		arr := ArrayValue(a)
		arr.SetArrayItem(a, 0, IntValue(a, i*2))
		forceGC()
		n, err = arr.GetArray()[0].Int()
		if err != nil {
			t.Fatalf("iteration %d: unexpected array error: %s", i, err)
		}
		if n != i*2 {
			t.Fatalf("iteration %d: got %d, want %d", i, n, i*2)
		}
		runtime.KeepAlive(a)
	}
}

func TestArenaGCSafety_SetArrayItem(t *testing.T) {
	old := debug.SetGCPercent(1)
	defer debug.SetGCPercent(old)

	for i := 0; i < gcTestIterations; i++ {
		a := arena.NewMonotonicArena()
		arr := ArrayValue(a)
		for j := 0; j < 5; j++ {
			arr.SetArrayItem(a, j, IntValue(a, i*10+j))
		}
		forceGC()
		items := arr.GetArray()
		if len(items) != 5 {
			t.Fatalf("iteration %d: expected 5 items, got %d", i, len(items))
		}
		for j := 0; j < 5; j++ {
			n, err := items[j].Int()
			if err != nil {
				t.Fatalf("iteration %d, item %d: unexpected error: %s", i, j, err)
			}
			if n != i*10+j {
				t.Fatalf("iteration %d, item %d: got %d, want %d", i, j, n, i*10+j)
			}
		}
		runtime.KeepAlive(a)
	}
}

func TestArenaGCSafety_AppendToArray(t *testing.T) {
	old := debug.SetGCPercent(1)
	defer debug.SetGCPercent(old)

	for i := 0; i < gcTestIterations; i++ {
		a := arena.NewMonotonicArena()
		arr := ArrayValue(a)
		for j := 0; j < 5; j++ {
			AppendToArray(a, arr, StringValue(a, heapString("item", i*10+j)))
		}
		forceGC()
		got := string(arr.MarshalTo(nil))
		items := arr.GetArray()
		if len(items) != 5 {
			t.Fatalf("iteration %d: expected 5 items, got %d (output: %s)", i, len(items), got)
		}
		for j := 0; j < 5; j++ {
			sb, err := items[j].StringBytes()
			if err != nil {
				t.Fatalf("iteration %d, item %d: unexpected error: %s", i, j, err)
			}
			expected := heapString("item", i*10+j)
			if string(sb) != expected {
				t.Fatalf("iteration %d, item %d: got %q, want %q", i, j, string(sb), expected)
			}
		}
		runtime.KeepAlive(a)
	}
}

func TestArenaGCSafety_AppendArrayItems(t *testing.T) {
	old := debug.SetGCPercent(1)
	defer debug.SetGCPercent(old)

	for i := 0; i < gcTestIterations; i++ {
		a := arena.NewMonotonicArena()
		left := ArrayValue(a)
		AppendToArray(a, left, IntValue(a, 1))
		AppendToArray(a, left, IntValue(a, 2))

		right := ArrayValue(a)
		AppendToArray(a, right, IntValue(a, 3))
		AppendToArray(a, right, IntValue(a, 4))

		left.AppendArrayItems(a, right)
		forceGC()

		items := left.GetArray()
		if len(items) != 4 {
			t.Fatalf("iteration %d: expected 4 items, got %d", i, len(items))
		}
		for j, expected := range []int{1, 2, 3, 4} {
			n, err := items[j].Int()
			if err != nil {
				t.Fatalf("iteration %d, item %d: unexpected error: %s", i, j, err)
			}
			if n != expected {
				t.Fatalf("iteration %d, item %d: got %d, want %d", i, j, n, expected)
			}
		}
		runtime.KeepAlive(a)
	}
}

func TestArenaGCSafety_SetValue(t *testing.T) {
	old := debug.SetGCPercent(1)
	defer debug.SetGCPercent(old)

	for i := 0; i < gcTestIterations; i++ {
		a := arena.NewMonotonicArena()
		obj := ObjectValue(a)
		val := StringValue(a, heapString("deep_value", i))
		SetValue(a, obj, val, "level1", "level2", "level3")
		forceGC()
		result := obj.Get("level1")
		if result == nil {
			t.Fatalf("iteration %d: level1 is nil", i)
		}
		result = result.Get("level2")
		if result == nil {
			t.Fatalf("iteration %d: level2 is nil", i)
		}
		result = result.Get("level3")
		if result == nil {
			t.Fatalf("iteration %d: level3 is nil", i)
		}
		sb, err := result.StringBytes()
		if err != nil {
			t.Fatalf("iteration %d: unexpected error: %s", i, err)
		}
		expected := heapString("deep_value", i)
		if string(sb) != expected {
			t.Fatalf("iteration %d: got %q, want %q", i, string(sb), expected)
		}
		runtime.KeepAlive(a)
	}
}

func TestArenaGCSafety_SetNull(t *testing.T) {
	old := debug.SetGCPercent(1)
	defer debug.SetGCPercent(old)

	for i := 0; i < gcTestIterations; i++ {
		a := arena.NewMonotonicArena()
		obj := ObjectValue(a)
		obj.Set(a, "key", StringValue(a, heapString("value", i)))
		SetNull(a, obj, "key")
		forceGC()
		result := obj.Get("key")
		if result == nil {
			t.Fatalf("iteration %d: expected non-nil result", i)
		}
		if result.Type() != TypeNull {
			t.Fatalf("iteration %d: expected TypeNull, got %s", i, result.Type())
		}
		got := string(obj.MarshalTo(nil))
		if got != `{"key":null}` {
			t.Fatalf("iteration %d: got %q, want %q", i, got, `{"key":null}`)
		}
		runtime.KeepAlive(a)
	}
}

func TestArenaGCSafety_MergeValues(t *testing.T) {
	old := debug.SetGCPercent(1)
	defer debug.SetGCPercent(old)

	for i := 0; i < gcTestIterations; i++ {
		a := arena.NewMonotonicArena()
		var p Parser
		left, err := p.ParseWithArena(a, fmt.Sprintf(`{"a":"%d","b":"left"}`, i))
		if err != nil {
			t.Fatalf("iteration %d: parse left: %s", i, err)
		}
		right, err := p.ParseWithArena(a, fmt.Sprintf(`{"b":"right","c":"%d"}`, i))
		if err != nil {
			t.Fatalf("iteration %d: parse right: %s", i, err)
		}
		merged, _, err := MergeValues(a, left, right)
		if err != nil {
			t.Fatalf("iteration %d: merge: %s", i, err)
		}
		forceGC()
		// Verify all keys survived GC
		aVal := merged.Get("a")
		if aVal == nil {
			t.Fatalf("iteration %d: key 'a' is nil", i)
		}
		sb, _ := aVal.StringBytes()
		if string(sb) != strconv.Itoa(i) {
			t.Fatalf("iteration %d: key 'a' got %q, want %q", i, string(sb), strconv.Itoa(i))
		}
		bVal := merged.Get("b")
		if bVal == nil {
			t.Fatalf("iteration %d: key 'b' is nil", i)
		}
		sb, _ = bVal.StringBytes()
		if string(sb) != "right" {
			t.Fatalf("iteration %d: key 'b' got %q, want %q", i, string(sb), "right")
		}
		cVal := merged.Get("c")
		if cVal == nil {
			t.Fatalf("iteration %d: key 'c' is nil", i)
		}
		sb, _ = cVal.StringBytes()
		if string(sb) != strconv.Itoa(i) {
			t.Fatalf("iteration %d: key 'c' got %q, want %q", i, string(sb), strconv.Itoa(i))
		}
		runtime.KeepAlive(a)
	}
}

func TestArenaGCSafety_MergeValuesWithPath(t *testing.T) {
	old := debug.SetGCPercent(1)
	defer debug.SetGCPercent(old)

	for i := 0; i < gcTestIterations; i++ {
		a := arena.NewMonotonicArena()
		var p Parser
		left, err := p.ParseWithArena(a, `{"data":{"existing":"value"}}`)
		if err != nil {
			t.Fatalf("iteration %d: parse left: %s", i, err)
		}
		right := StringValue(a, heapString("merged", i))
		merged, _, err := MergeValuesWithPath(a, left, right, "data", "nested")
		if err != nil {
			t.Fatalf("iteration %d: merge: %s", i, err)
		}
		forceGC()
		result := merged.Get("data", "nested")
		if result == nil {
			t.Fatalf("iteration %d: nested result is nil", i)
		}
		sb, _ := result.StringBytes()
		expected := heapString("merged", i)
		if string(sb) != expected {
			t.Fatalf("iteration %d: got %q, want %q", i, string(sb), expected)
		}
		// Verify existing data survived
		existing := merged.Get("data", "existing")
		if existing == nil {
			t.Fatalf("iteration %d: existing is nil", i)
		}
		sb, _ = existing.StringBytes()
		if string(sb) != "value" {
			t.Fatalf("iteration %d: existing got %q, want %q", i, string(sb), "value")
		}
		runtime.KeepAlive(a)
	}
}

func TestArenaGCSafety_ParseWithArena(t *testing.T) {
	old := debug.SetGCPercent(1)
	defer debug.SetGCPercent(old)

	jsonInput := `{
		"name": "test",
		"age": 42,
		"score": 3.14,
		"active": true,
		"deleted": false,
		"address": null,
		"tags": ["go", "arena", "gc"],
		"nested": {
			"key": "value",
			"count": 100
		}
	}`

	for i := 0; i < gcTestIterations; i++ {
		a := arena.NewMonotonicArena()
		var p Parser
		v, err := p.ParseWithArena(a, jsonInput)
		if err != nil {
			t.Fatalf("iteration %d: parse: %s", i, err)
		}
		forceGC()

		// Exercise all read methods after GC

		// Type
		if v.Type() != TypeObject {
			t.Fatalf("iteration %d: expected TypeObject, got %s", i, v.Type())
		}

		// Exists
		if !v.Exists("name") {
			t.Fatalf("iteration %d: name should exist", i)
		}
		if v.Exists("nonexistent") {
			t.Fatalf("iteration %d: nonexistent should not exist", i)
		}

		// Get + StringBytes
		name := v.Get("name")
		if name == nil {
			t.Fatalf("iteration %d: name is nil", i)
		}
		sb, err := name.StringBytes()
		if err != nil {
			t.Fatalf("iteration %d: StringBytes: %s", i, err)
		}
		if string(sb) != "test" {
			t.Fatalf("iteration %d: name got %q, want %q", i, string(sb), "test")
		}

		// GetStringBytes
		sb = v.GetStringBytes("name")
		if string(sb) != "test" {
			t.Fatalf("iteration %d: GetStringBytes got %q, want %q", i, string(sb), "test")
		}

		// GetInt
		age := v.GetInt("age")
		if age != 42 {
			t.Fatalf("iteration %d: age got %d, want 42", i, age)
		}

		// Int
		ageVal := v.Get("age")
		ageInt, err := ageVal.Int()
		if err != nil {
			t.Fatalf("iteration %d: Int: %s", i, err)
		}
		if ageInt != 42 {
			t.Fatalf("iteration %d: Int got %d, want 42", i, ageInt)
		}

		// GetFloat64
		score := v.GetFloat64("score")
		if score != 3.14 {
			t.Fatalf("iteration %d: score got %f, want 3.14", i, score)
		}

		// Float64
		scoreVal := v.Get("score")
		scoreFloat, err := scoreVal.Float64()
		if err != nil {
			t.Fatalf("iteration %d: Float64: %s", i, err)
		}
		if scoreFloat != 3.14 {
			t.Fatalf("iteration %d: Float64 got %f, want 3.14", i, scoreFloat)
		}

		// GetBool
		active := v.GetBool("active")
		if !active {
			t.Fatalf("iteration %d: active should be true", i)
		}

		// Bool
		activeVal := v.Get("active")
		activeBool, err := activeVal.Bool()
		if err != nil {
			t.Fatalf("iteration %d: Bool: %s", i, err)
		}
		if !activeBool {
			t.Fatalf("iteration %d: Bool should be true", i)
		}

		// GetArray
		tags := v.GetArray("tags")
		if len(tags) != 3 {
			t.Fatalf("iteration %d: tags length got %d, want 3", i, len(tags))
		}
		tagBytes, _ := tags[0].StringBytes()
		if string(tagBytes) != "go" {
			t.Fatalf("iteration %d: first tag got %q, want %q", i, string(tagBytes), "go")
		}

		// Array
		tagsVal := v.Get("tags")
		tagsArr, err := tagsVal.Array()
		if err != nil {
			t.Fatalf("iteration %d: Array: %s", i, err)
		}
		if len(tagsArr) != 3 {
			t.Fatalf("iteration %d: Array length got %d, want 3", i, len(tagsArr))
		}

		// GetObject
		nested := v.GetObject("nested")
		if nested == nil {
			t.Fatalf("iteration %d: nested is nil", i)
		}
		if nested.Len() != 2 {
			t.Fatalf("iteration %d: nested length got %d, want 2", i, nested.Len())
		}

		// Object
		obj, err := v.Object()
		if err != nil {
			t.Fatalf("iteration %d: Object: %s", i, err)
		}

		// Object.Len
		if obj.Len() != 8 {
			t.Fatalf("iteration %d: object length got %d, want 8", i, obj.Len())
		}

		// Object.Get
		nameFromObj := obj.Get("name")
		if nameFromObj == nil {
			t.Fatalf("iteration %d: Object.Get name is nil", i)
		}

		// Object.Visit
		keyCount := 0
		obj.Visit(func(key []byte, val *Value) {
			keyCount++
			if len(key) == 0 {
				t.Fatalf("iteration %d: empty key in Visit", i)
			}
		})
		if keyCount != 8 {
			t.Fatalf("iteration %d: Visit counted %d keys, want 8", i, keyCount)
		}

		// MarshalTo + String
		marshaled := string(v.MarshalTo(nil))
		if len(marshaled) == 0 {
			t.Fatalf("iteration %d: MarshalTo returned empty", i)
		}
		str := v.String()
		if str != marshaled {
			t.Fatalf("iteration %d: String() != MarshalTo()", i)
		}

		// Uint, Int64, Uint64 on number value
		ageUint, err := ageVal.Uint()
		if err != nil {
			t.Fatalf("iteration %d: Uint: %s", i, err)
		}
		if ageUint != 42 {
			t.Fatalf("iteration %d: Uint got %d, want 42", i, ageUint)
		}
		ageInt64, err := ageVal.Int64()
		if err != nil {
			t.Fatalf("iteration %d: Int64: %s", i, err)
		}
		if ageInt64 != 42 {
			t.Fatalf("iteration %d: Int64 got %d, want 42", i, ageInt64)
		}
		ageUint64, err := ageVal.Uint64()
		if err != nil {
			t.Fatalf("iteration %d: Uint64: %s", i, err)
		}
		if ageUint64 != 42 {
			t.Fatalf("iteration %d: Uint64 got %d, want 42", i, ageUint64)
		}

		// GetUint, GetInt64, GetUint64
		if v.GetUint("age") != 42 {
			t.Fatalf("iteration %d: GetUint got %d, want 42", i, v.GetUint("age"))
		}
		if v.GetInt64("age") != 42 {
			t.Fatalf("iteration %d: GetInt64 got %d, want 42", i, v.GetInt64("age"))
		}
		if v.GetUint64("age") != 42 {
			t.Fatalf("iteration %d: GetUint64 got %d, want 42", i, v.GetUint64("age"))
		}
		runtime.KeepAlive(a)
	}
}

func TestArenaGCSafety_ParseBytesWithArena(t *testing.T) {
	old := debug.SetGCPercent(1)
	defer debug.SetGCPercent(old)

	jsonInput := []byte(`{"key":"value","num":123,"arr":[1,2,3]}`)

	for i := 0; i < gcTestIterations; i++ {
		a := arena.NewMonotonicArena()
		// Copy input to arena-allocated buffer
		buf := arena.AllocateSlice[byte](a, len(jsonInput), len(jsonInput))
		copy(buf, jsonInput)
		var p Parser
		v, err := p.ParseBytesWithArena(a, buf)
		if err != nil {
			t.Fatalf("iteration %d: parse: %s", i, err)
		}
		forceGC()
		got := string(v.MarshalTo(nil))
		expected := `{"key":"value","num":123,"arr":[1,2,3]}`
		if got != expected {
			t.Fatalf("iteration %d: got %q, want %q", i, got, expected)
		}
		runtime.KeepAlive(a)
	}
}

func TestArenaGCSafety_ObjectDel(t *testing.T) {
	old := debug.SetGCPercent(1)
	defer debug.SetGCPercent(old)

	for i := 0; i < gcTestIterations; i++ {
		a := arena.NewMonotonicArena()
		var p Parser
		v, err := p.ParseWithArena(a, `{"keep":"yes","remove":"no","also_keep":"yes"}`)
		if err != nil {
			t.Fatalf("iteration %d: parse: %s", i, err)
		}
		o, _ := v.Object()
		o.Del("remove")
		forceGC()
		if o.Len() != 2 {
			t.Fatalf("iteration %d: expected 2 keys, got %d", i, o.Len())
		}
		if o.Get("keep") == nil {
			t.Fatalf("iteration %d: 'keep' should exist", i)
		}
		if o.Get("also_keep") == nil {
			t.Fatalf("iteration %d: 'also_keep' should exist", i)
		}
		runtime.KeepAlive(a)
	}
}

func TestArenaGCSafety_ValueDel(t *testing.T) {
	old := debug.SetGCPercent(1)
	defer debug.SetGCPercent(old)

	for i := 0; i < gcTestIterations; i++ {
		a := arena.NewMonotonicArena()
		var p Parser
		// Test del on array
		v, err := p.ParseWithArena(a, `[1,2,3,4,5]`)
		if err != nil {
			t.Fatalf("iteration %d: parse: %s", i, err)
		}
		v.Del("2") // Remove middle element
		forceGC()
		items := v.GetArray()
		if len(items) != 4 {
			t.Fatalf("iteration %d: expected 4 items, got %d", i, len(items))
		}

		// Test del on object
		v2, err := p.ParseWithArena(a, `{"a":1,"b":2}`)
		if err != nil {
			t.Fatalf("iteration %d: parse v2: %s", i, err)
		}
		v2.Del("a")
		forceGC()
		if v2.GetObject().Len() != 1 {
			t.Fatalf("iteration %d: expected 1 key, got %d", i, v2.GetObject().Len())
		}
		runtime.KeepAlive(a)
	}
}

func TestArenaGCSafety_DeduplicateObjectKeysRecursively(t *testing.T) {
	old := debug.SetGCPercent(1)
	defer debug.SetGCPercent(old)

	for i := 0; i < gcTestIterations; i++ {
		a := arena.NewMonotonicArena()
		var p Parser
		v, err := p.ParseWithArena(a, `{"a":1,"b":2,"a":3,"nested":{"x":1,"x":2}}`)
		if err != nil {
			t.Fatalf("iteration %d: parse: %s", i, err)
		}
		DeduplicateObjectKeysRecursively(v)
		forceGC()
		o, _ := v.Object()
		if o.Len() != 3 {
			t.Fatalf("iteration %d: expected 3 keys after dedup, got %d", i, o.Len())
		}
		nested := v.GetObject("nested")
		if nested == nil {
			t.Fatalf("iteration %d: nested is nil", i)
		}
		if nested.Len() != 1 {
			t.Fatalf("iteration %d: nested expected 1 key after dedup, got %d", i, nested.Len())
		}
		runtime.KeepAlive(a)
	}
}

func TestArenaGCSafety_ValueIsNull(t *testing.T) {
	old := debug.SetGCPercent(1)
	defer debug.SetGCPercent(old)

	for i := 0; i < gcTestIterations; i++ {
		a := arena.NewMonotonicArena()
		null := arena.Allocate[Value](a)
		null.t = TypeNull
		nonNull := StringValue(a, heapString("notnull", i))
		forceGC()
		if !ValueIsNull(null) {
			t.Fatalf("iteration %d: expected null to be null", i)
		}
		if ValueIsNull(nonNull) {
			t.Fatalf("iteration %d: expected nonNull to not be null", i)
		}
		if ValueIsNonNull(null) {
			t.Fatalf("iteration %d: expected null to not be non-null", i)
		}
		if !ValueIsNonNull(nonNull) {
			t.Fatalf("iteration %d: expected nonNull to be non-null", i)
		}
		runtime.KeepAlive(a)
	}
}

func TestArenaGCSafety_ParseWithArena_EscapedKeys(t *testing.T) {
	old := debug.SetGCPercent(1)
	defer debug.SetGCPercent(old)

	// Test that keys with escape sequences are properly arena-allocated during parsing
	jsonInput := `{"normal":"a","escaped\tkey":"b","unicode\u0041key":"c"}`

	for i := 0; i < gcTestIterations; i++ {
		a := arena.NewMonotonicArena()
		var p Parser
		v, err := p.ParseWithArena(a, jsonInput)
		if err != nil {
			t.Fatalf("iteration %d: parse: %s", i, err)
		}
		forceGC()

		// Normal key
		normal := v.Get("normal")
		if normal == nil {
			t.Fatalf("iteration %d: normal is nil", i)
		}

		// Escaped key (tab)
		escaped := v.Get("escaped\tkey")
		if escaped == nil {
			t.Fatalf("iteration %d: escaped tab key is nil", i)
		}
		sb, _ := escaped.StringBytes()
		if string(sb) != "b" {
			t.Fatalf("iteration %d: escaped key value got %q, want %q", i, string(sb), "b")
		}

		// Unicode escape key
		unicode := v.Get("unicodeAkey")
		if unicode == nil {
			t.Fatalf("iteration %d: unicode key is nil", i)
		}
		sb, _ = unicode.StringBytes()
		if string(sb) != "c" {
			t.Fatalf("iteration %d: unicode key value got %q, want %q", i, string(sb), "c")
		}

		// Verify marshaling round-trip
		marshaled := string(v.MarshalTo(nil))
		if len(marshaled) == 0 {
			t.Fatalf("iteration %d: MarshalTo returned empty", i)
		}
		runtime.KeepAlive(a)
	}
}

func TestArenaGCSafety_ParseWithArena_EscapedStrings(t *testing.T) {
	old := debug.SetGCPercent(1)
	defer debug.SetGCPercent(old)

	// Test that string values with escape sequences are properly arena-allocated
	jsonInput := `{"a":"hello\tworld","b":"line1\nline2","c":"quote\"inside","d":"unicode\u0041"}`

	for i := 0; i < gcTestIterations; i++ {
		a := arena.NewMonotonicArena()
		var p Parser
		v, err := p.ParseWithArena(a, jsonInput)
		if err != nil {
			t.Fatalf("iteration %d: parse: %s", i, err)
		}
		forceGC()

		tests := map[string]string{
			"a": "hello\tworld",
			"b": "line1\nline2",
			"c": "quote\"inside",
			"d": "unicodeA",
		}
		for key, expected := range tests {
			val := v.Get(key)
			if val == nil {
				t.Fatalf("iteration %d: key %q is nil", i, key)
			}
			sb, err := val.StringBytes()
			if err != nil {
				t.Fatalf("iteration %d: key %q StringBytes: %s", i, key, err)
			}
			if string(sb) != expected {
				t.Fatalf("iteration %d: key %q got %q, want %q", i, key, string(sb), expected)
			}
		}
		runtime.KeepAlive(a)
	}
}

func TestArenaGCSafety_ComplexWorkflow(t *testing.T) {
	old := debug.SetGCPercent(1)
	defer debug.SetGCPercent(old)

	// Simulate a realistic workflow: parse, modify, merge, marshal
	for i := 0; i < gcTestIterations; i++ {
		a := arena.NewMonotonicArena()
		var p Parser

		// Parse base document
		base, err := p.ParseWithArena(a, `{"users":[{"name":"alice","age":30}],"count":1}`)
		if err != nil {
			t.Fatalf("iteration %d: parse base: %s", i, err)
		}

		// Create a new user and append to array
		newUser := ObjectValue(a)
		newUser.Set(a, "name", StringValue(a, heapString("bob", i)))
		newUser.Set(a, "age", IntValue(a, 25+i))
		users := base.Get("users")
		AppendToArray(a, users, newUser)

		// Update count
		base.Set(a, "count", IntValue(a, 2))

		// Merge with additional data
		extra, err := p.ParseWithArena(a, `{"metadata":{"version":"1.0"}}`)
		if err != nil {
			t.Fatalf("iteration %d: parse extra: %s", i, err)
		}
		merged, _, err := MergeValues(a, base, extra)
		if err != nil {
			t.Fatalf("iteration %d: merge: %s", i, err)
		}

		forceGC()

		// Verify everything survived GC
		usersArr := merged.GetArray("users")
		if len(usersArr) != 2 {
			t.Fatalf("iteration %d: expected 2 users, got %d", i, len(usersArr))
		}

		// Verify first user
		alice := usersArr[0]
		aliceName, _ := alice.Get("name").StringBytes()
		if string(aliceName) != "alice" {
			t.Fatalf("iteration %d: first user name got %q, want %q", i, string(aliceName), "alice")
		}

		// Verify second user (heap-allocated name)
		bob := usersArr[1]
		bobName, _ := bob.Get("name").StringBytes()
		expected := heapString("bob", i)
		if string(bobName) != expected {
			t.Fatalf("iteration %d: second user name got %q, want %q", i, string(bobName), expected)
		}
		bobAge, _ := bob.Get("age").Int()
		if bobAge != 25+i {
			t.Fatalf("iteration %d: bob age got %d, want %d", i, bobAge, 25+i)
		}

		// Verify count
		count, _ := merged.Get("count").Int()
		if count != 2 {
			t.Fatalf("iteration %d: count got %d, want 2", i, count)
		}

		// Verify merged metadata
		version := merged.Get("metadata", "version")
		if version == nil {
			t.Fatalf("iteration %d: metadata.version is nil", i)
		}
		vb, _ := version.StringBytes()
		if string(vb) != "1.0" {
			t.Fatalf("iteration %d: version got %q, want %q", i, string(vb), "1.0")
		}

		// Final marshal to verify integrity
		marshaled := string(merged.MarshalTo(nil))
		if len(marshaled) == 0 {
			t.Fatalf("iteration %d: MarshalTo returned empty", i)
		}
		runtime.KeepAlive(a)
	}
}

// heapJSON returns a heap-allocated JSON string with unique values per iteration.
// The //go:noinline directive ensures the string is truly heap-allocated.
//
//go:noinline
func heapJSON(i int) string {
	return fmt.Sprintf(`{"name":"user_%d","age":%d,"score":3.14,"active":true,"tags":["tag_%d","arena"],"nested":{"key":"val_%d","count":%d}}`,
		i, 20+i, i, i, i*10)
}

// TestArenaGCSafety_ParseWithArena_HeapInput tests that ParseWithArena is safe
// when the input string is heap-allocated and goes out of scope before GC.
// This catches the bug where parsed substrings (numbers, unescaped strings, keys)
// reference the input's backing array which the GC can collect.
func TestArenaGCSafety_ParseWithArena_HeapInput(t *testing.T) {
	old := debug.SetGCPercent(1)
	defer debug.SetGCPercent(old)

	for i := 0; i < gcTestIterations; i++ {
		a := arena.NewMonotonicArena()
		var p Parser
		// Use a heap-allocated JSON string (not a literal)
		input := heapJSON(i)
		v, err := p.ParseWithArena(a, input)
		if err != nil {
			t.Fatalf("iteration %d: parse: %s", i, err)
		}
		// Drop reference to input so GC can collect it
		input = ""
		_ = input
		forceGC()

		// Verify string values (substrings of input)
		name := v.GetStringBytes("name")
		expected := fmt.Sprintf("user_%d", i)
		if string(name) != expected {
			t.Fatalf("iteration %d: name got %q, want %q", i, string(name), expected)
		}

		// Verify number values (substrings of input)
		age := v.GetInt("age")
		if age != 20+i {
			t.Fatalf("iteration %d: age got %d, want %d", i, age, 20+i)
		}
		score := v.GetFloat64("score")
		if score != 3.14 {
			t.Fatalf("iteration %d: score got %f, want 3.14", i, score)
		}

		// Verify boolean
		if !v.GetBool("active") {
			t.Fatalf("iteration %d: active should be true", i)
		}

		// Verify array with string elements
		tags := v.GetArray("tags")
		if len(tags) != 2 {
			t.Fatalf("iteration %d: expected 2 tags, got %d", i, len(tags))
		}
		tag0, _ := tags[0].StringBytes()
		expectedTag := fmt.Sprintf("tag_%d", i)
		if string(tag0) != expectedTag {
			t.Fatalf("iteration %d: tag[0] got %q, want %q", i, string(tag0), expectedTag)
		}

		// Verify nested object (keys and values are substrings of input)
		nested := v.Get("nested")
		if nested == nil {
			t.Fatalf("iteration %d: nested is nil", i)
		}
		nestedKey := nested.GetStringBytes("key")
		expectedVal := fmt.Sprintf("val_%d", i)
		if string(nestedKey) != expectedVal {
			t.Fatalf("iteration %d: nested.key got %q, want %q", i, string(nestedKey), expectedVal)
		}
		nestedCount := nested.GetInt("count")
		if nestedCount != i*10 {
			t.Fatalf("iteration %d: nested.count got %d, want %d", i, nestedCount, i*10)
		}

		// Verify full marshal round-trip
		marshaled := string(v.MarshalTo(nil))
		if len(marshaled) == 0 {
			t.Fatalf("iteration %d: MarshalTo returned empty", i)
		}
		runtime.KeepAlive(a)
	}
}

// TestArenaGCSafety_ParseBytesWithArena_HeapInput tests that ParseBytesWithArena
// is safe when the input byte slice is heap-allocated and not pre-copied to the arena.
func TestArenaGCSafety_ParseBytesWithArena_HeapInput(t *testing.T) {
	old := debug.SetGCPercent(1)
	defer debug.SetGCPercent(old)

	for i := 0; i < gcTestIterations; i++ {
		a := arena.NewMonotonicArena()
		var p Parser
		// Use heap-allocated bytes directly — no manual arena copy
		input := []byte(heapJSON(i))
		v, err := p.ParseBytesWithArena(a, input)
		if err != nil {
			t.Fatalf("iteration %d: parse: %s", i, err)
		}
		// Drop reference to input so GC can collect the backing array
		clear(input)
		forceGC()

		name := v.GetStringBytes("name")
		expected := fmt.Sprintf("user_%d", i)
		if string(name) != expected {
			t.Fatalf("iteration %d: name got %q, want %q", i, string(name), expected)
		}
		age := v.GetInt("age")
		if age != 20+i {
			t.Fatalf("iteration %d: age got %d, want %d", i, age, 20+i)
		}
		marshaled := string(v.MarshalTo(nil))
		if len(marshaled) == 0 {
			t.Fatalf("iteration %d: MarshalTo returned empty", i)
		}
		runtime.KeepAlive(a)
	}
}

// TestArenaGCSafety_StringValueBytes_HeapInput tests that StringValueBytes
// is safe when called with heap-allocated bytes without pre-copying to the arena.
func TestArenaGCSafety_StringValueBytes_HeapInput(t *testing.T) {
	old := debug.SetGCPercent(1)
	defer debug.SetGCPercent(old)

	for i := 0; i < gcTestIterations; i++ {
		a := arena.NewMonotonicArena()
		// Pass heap bytes directly — no manual arena copy
		src := heapBytes("direct", i)
		v := StringValueBytes(a, src)
		// Drop reference to src so GC can collect the backing array
		clear(src)
		forceGC()
		got := v.String()
		expected := `"` + fmt.Sprintf("direct_%d_padding_to_ensure_heap_allocation", i) + `"`
		if got != expected {
			t.Fatalf("iteration %d: got %q, want %q", i, got, expected)
		}
		runtime.KeepAlive(a)
	}
}

// TestArenaGCSafety_DeepCopy_ObjectSet verifies that DeepCopy makes it safe to
// store a heap-allocated *Value into an arena-allocated object via Object.Set.
// Without DeepCopy the kv.v pointer lives in noscan arena memory, making the
// heap Value invisible to the GC. With DeepCopy the stored value is fully
// arena-allocated.
func TestArenaGCSafety_DeepCopy_ObjectSet(t *testing.T) {
	old := debug.SetGCPercent(1)
	defer debug.SetGCPercent(old)

	for i := 0; i < gcTestIterations; i++ {
		a := arena.NewMonotonicArena()

		obj := ObjectValue(a)
		heapVal := StringValue(nil, heapString("safe", i))

		// DeepCopy copies heapVal into arena a before storing.
		obj.Set(a, "key", DeepCopy(a, heapVal))

		// Drop the only external reference to heapVal.
		heapVal = nil //nolint:ineffassign
		forceGC()

		got := obj.Get("key")
		expected := heapString("safe", i)
		sb, _ := got.StringBytes()
		if string(sb) != expected {
			t.Fatalf("iteration %d: got %q, want %q", i, string(sb), expected)
		}
		marshaled := string(obj.MarshalTo(nil))
		if len(marshaled) == 0 {
			t.Fatalf("iteration %d: MarshalTo returned empty", i)
		}
		runtime.KeepAlive(a)
	}
}

// TestArenaGCSafety_DeepCopy_SetArrayItem verifies that DeepCopy makes it safe
// to store a heap-allocated *Value into an arena-allocated array via SetArrayItem.
func TestArenaGCSafety_DeepCopy_SetArrayItem(t *testing.T) {
	old := debug.SetGCPercent(1)
	defer debug.SetGCPercent(old)

	for i := 0; i < gcTestIterations; i++ {
		a := arena.NewMonotonicArena()

		arr := ArrayValue(a)
		heapVal := IntValue(nil, i)

		arr.SetArrayItem(a, 0, DeepCopy(a, heapVal))

		heapVal = nil //nolint:ineffassign
		forceGC()

		items, _ := arr.Array()
		if len(items) != 1 {
			t.Fatalf("iteration %d: want 1 item, got %d", i, len(items))
		}
		got := items[0].GetInt()
		if got != i {
			t.Fatalf("iteration %d: got %d, want %d", i, got, i)
		}
		marshaled := string(arr.MarshalTo(nil))
		if len(marshaled) == 0 {
			t.Fatalf("iteration %d: MarshalTo returned empty", i)
		}
		runtime.KeepAlive(a)
	}
}

// TestArenaGCSafety_DeepCopy_NestedObject verifies that DeepCopy recursively
// copies nested objects and arrays onto the arena.
func TestArenaGCSafety_DeepCopy_NestedObject(t *testing.T) {
	old := debug.SetGCPercent(1)
	defer debug.SetGCPercent(old)

	for i := 0; i < gcTestIterations; i++ {
		a := arena.NewMonotonicArena()

		// Build a nested heap value: {"name": "...", "scores": [1, 2, 3]}
		heapObj := ObjectValue(nil)
		heapObj.Set(nil, "name", StringValue(nil, heapString("nested", i)))
		heapArr := ArrayValue(nil)
		heapArr.SetArrayItem(nil, 0, IntValue(nil, i))
		heapArr.SetArrayItem(nil, 1, IntValue(nil, i+1))
		heapArr.SetArrayItem(nil, 2, IntValue(nil, i+2))
		heapObj.Set(nil, "scores", heapArr)

		arenaContainer := ObjectValue(a)
		arenaContainer.Set(a, "data", DeepCopy(a, heapObj))

		// Drop all heap references.
		heapObj = nil //nolint:ineffassign
		heapArr = nil //nolint:ineffassign
		forceGC()

		got := arenaContainer.Get("data", "name")
		expected := heapString("nested", i)
		sb, _ := got.StringBytes()
		if string(sb) != expected {
			t.Fatalf("iteration %d: name got %q, want %q", i, string(sb), expected)
		}

		scores := arenaContainer.GetArray("data", "scores")
		if len(scores) != 3 {
			t.Fatalf("iteration %d: want 3 scores, got %d", i, len(scores))
		}
		if scores[0].GetInt() != i || scores[1].GetInt() != i+1 || scores[2].GetInt() != i+2 {
			t.Fatalf("iteration %d: unexpected scores", i)
		}

		marshaled := string(arenaContainer.MarshalTo(nil))
		if len(marshaled) == 0 {
			t.Fatalf("iteration %d: MarshalTo returned empty", i)
		}
		runtime.KeepAlive(a)
	}
}

// TestArenaGCSafety_DeepCopy_NilArena verifies that DeepCopy(nil, v) is a
// no-op and returns v unchanged.
func TestArenaGCSafety_DeepCopy_NilArena(t *testing.T) {
	v := StringValue(nil, "hello")
	got := DeepCopy(nil, v)
	if got != v {
		t.Fatal("DeepCopy(nil, v) must return v unchanged")
	}
}

// TestArenaGCSafety_DeepCopy_NilValue verifies that DeepCopy(a, nil) returns nil.
func TestArenaGCSafety_DeepCopy_NilValue(t *testing.T) {
	a := arena.NewMonotonicArena()
	got := DeepCopy(a, nil)
	if got != nil {
		t.Fatal("DeepCopy(a, nil) must return nil")
	}
	runtime.KeepAlive(a)
}

// TestArenaGCSafety_DeepCopy_EmptyObject verifies that DeepCopy handles an
// empty object (zero keys) without error.
func TestArenaGCSafety_DeepCopy_EmptyObject(t *testing.T) {
	a := arena.NewMonotonicArena()
	obj := ObjectValue(a)
	cp := DeepCopy(a, obj)
	if cp.Type() != TypeObject {
		t.Fatalf("expected TypeObject, got %v", cp.Type())
	}
	o, _ := cp.Object()
	if o.Len() != 0 {
		t.Fatalf("expected empty object, got %d keys", o.Len())
	}
	runtime.KeepAlive(a)
}

func TestArenaGCSafety_UnescapeAllBranches(t *testing.T) {
	old := debug.SetGCPercent(1)
	defer debug.SetGCPercent(old)

	// JSON exercises every branch of unescapeStringBestEffort:
	// simple escapes: \\ \/ \b \f \r \t \n \"
	// unicode non-surrogate: \u0041 (A)
	// surrogate pair: \uD83D\uDE00 (😀)
	// lone high surrogate (no low follows): \uD800X
	// unknown escape: \x
	jsonInput := `{
		"backslash": "a\\b",
		"slash": "a\/b",
		"backspace": "a\bb",
		"formfeed": "a\fb",
		"carriage": "a\rb",
		"tab": "a\tb",
		"newline": "a\nb",
		"quote": "a\"b",
		"unicode": "\u0041\u0042\u0043",
		"surrogate": "\uD83D\uDE00",
		"lone_surrogate": "\uD800X",
		"unknown_esc": "\x",
		"mixed": "a\tb\nc\\d\u0041"
	}`

	expected := map[string]string{
		"backslash":      "a\\b",
		"slash":          "a/b",
		"backspace":      "a\bb",
		"formfeed":       "a\fb",
		"carriage":       "a\rb",
		"tab":            "a\tb",
		"newline":        "a\nb",
		"quote":          "a\"b",
		"unicode":        "ABC",
		"surrogate":      "😀",
		"lone_surrogate": "\\uD800X",
		"unknown_esc":    "\\x",
		"mixed":          "a\tb\nc\\dA",
	}

	for i := 0; i < gcTestIterations; i++ {
		a := arena.NewMonotonicArena()
		var p Parser
		v, err := p.ParseWithArena(a, jsonInput)
		if err != nil {
			t.Fatalf("iteration %d: parse: %s", i, err)
		}
		forceGC()

		for key, want := range expected {
			got := v.Get(key)
			if got == nil {
				t.Fatalf("iteration %d: key %q is nil", i, key)
			}
			sb, err := got.StringBytes()
			if err != nil {
				t.Fatalf("iteration %d: key %q StringBytes: %s", i, key, err)
			}
			if string(sb) != want {
				t.Fatalf("iteration %d: key %q got %q, want %q", i, key, string(sb), want)
			}
		}

		marshaled := string(v.MarshalTo(nil))
		if len(marshaled) == 0 {
			t.Fatalf("iteration %d: MarshalTo returned empty", i)
		}
		runtime.KeepAlive(a)
	}
}

func TestArenaGCSafety_MergeValues_ScalarReplacement(t *testing.T) {
	old := debug.SetGCPercent(1)
	defer debug.SetGCPercent(old)

	for i := 0; i < gcTestIterations; i++ {
		a := arena.NewMonotonicArena()
		var p Parser

		left, err := p.ParseWithArena(a, fmt.Sprintf(`{"n":%d,"s":"old_%d","b":true}`, i, i))
		if err != nil {
			t.Fatalf("iteration %d: parse left: %s", i, err)
		}
		right, err := p.ParseWithArena(a, fmt.Sprintf(`{"n":%d,"s":"new_%d","b":false}`, i+1000, i))
		if err != nil {
			t.Fatalf("iteration %d: parse right: %s", i, err)
		}
		merged, _, err := MergeValues(a, left, right)
		if err != nil {
			t.Fatalf("iteration %d: merge: %s", i, err)
		}
		forceGC()

		n, _ := merged.Get("n").Float64()
		if n != float64(i+1000) {
			t.Fatalf("iteration %d: number got %v, want %v", i, n, i+1000)
		}

		sb, _ := merged.Get("s").StringBytes()
		expectedStr := fmt.Sprintf("new_%d", i)
		if string(sb) != expectedStr {
			t.Fatalf("iteration %d: string got %q, want %q", i, string(sb), expectedStr)
		}

		bVal := merged.Get("b")
		if bVal.Type() != TypeFalse {
			t.Fatalf("iteration %d: bool got %s, want TypeFalse", i, bVal.Type())
		}
		runtime.KeepAlive(a)
	}
}

func TestArenaGCSafety_MergeValues_RecursiveObjects(t *testing.T) {
	old := debug.SetGCPercent(1)
	defer debug.SetGCPercent(old)

	for i := 0; i < gcTestIterations; i++ {
		a := arena.NewMonotonicArena()
		var p Parser

		leftJSON := fmt.Sprintf(`{"a":{"x":%d,"y":"old_%d"},"b":{"m":"keep"}}`, i, i)
		rightJSON := fmt.Sprintf(`{"a":{"y":"new_%d","z":%d},"b":{"n":"added"}}`, i, i*10)

		left, err := p.ParseWithArena(a, leftJSON)
		if err != nil {
			t.Fatalf("iteration %d: parse left: %s", i, err)
		}
		right, err := p.ParseWithArena(a, rightJSON)
		if err != nil {
			t.Fatalf("iteration %d: parse right: %s", i, err)
		}
		merged, _, err := MergeValues(a, left, right)
		if err != nil {
			t.Fatalf("iteration %d: merge: %s", i, err)
		}
		forceGC()

		aObj := merged.Get("a")
		if aObj == nil {
			t.Fatalf("iteration %d: key 'a' is nil", i)
		}
		x, _ := aObj.Get("x").Int()
		if x != i {
			t.Fatalf("iteration %d: a.x got %d, want %d", i, x, i)
		}
		yb, _ := aObj.Get("y").StringBytes()
		expectedY := fmt.Sprintf("new_%d", i)
		if string(yb) != expectedY {
			t.Fatalf("iteration %d: a.y got %q, want %q", i, string(yb), expectedY)
		}
		z, _ := aObj.Get("z").Int()
		if z != i*10 {
			t.Fatalf("iteration %d: a.z got %d, want %d", i, z, i*10)
		}

		bObj := merged.Get("b")
		mb, _ := bObj.Get("m").StringBytes()
		if string(mb) != "keep" {
			t.Fatalf("iteration %d: b.m got %q, want %q", i, string(mb), "keep")
		}
		nb, _ := bObj.Get("n").StringBytes()
		if string(nb) != "added" {
			t.Fatalf("iteration %d: b.n got %q, want %q", i, string(nb), "added")
		}

		marshaled := string(merged.MarshalTo(nil))
		if len(marshaled) == 0 {
			t.Fatalf("iteration %d: MarshalTo returned empty", i)
		}
		runtime.KeepAlive(a)
	}
}

func TestArenaGCSafety_MergeValues_EmptyArrays(t *testing.T) {
	old := debug.SetGCPercent(1)
	defer debug.SetGCPercent(old)

	for i := 0; i < gcTestIterations; i++ {
		a := arena.NewMonotonicArena()
		var p Parser

		left, err := p.ParseWithArena(a, `[]`)
		if err != nil {
			t.Fatalf("iteration %d: parse left: %s", i, err)
		}
		right, err := p.ParseWithArena(a, fmt.Sprintf(`[%d,%d,%d]`, i, i+1, i+2))
		if err != nil {
			t.Fatalf("iteration %d: parse right: %s", i, err)
		}
		merged, changed, err := MergeValues(a, left, right)
		if err != nil {
			t.Fatalf("iteration %d: merge empty+full: %s", i, err)
		}
		if !changed {
			t.Fatalf("iteration %d: expected changed=true for empty left", i)
		}
		forceGC()
		arr := merged.GetArray()
		if len(arr) != 3 {
			t.Fatalf("iteration %d: expected 3 items, got %d", i, len(arr))
		}
		n, _ := arr[0].Int()
		if n != i {
			t.Fatalf("iteration %d: arr[0] got %d, want %d", i, n, i)
		}

		left2, err := p.ParseWithArena(a, fmt.Sprintf(`[%d,%d]`, i*10, i*10+1))
		if err != nil {
			t.Fatalf("iteration %d: parse left2: %s", i, err)
		}
		right2, err := p.ParseWithArena(a, `[]`)
		if err != nil {
			t.Fatalf("iteration %d: parse right2: %s", i, err)
		}
		merged2, changed2, err := MergeValues(a, left2, right2)
		if err != nil {
			t.Fatalf("iteration %d: merge full+empty: %s", i, err)
		}
		if changed2 {
			t.Fatalf("iteration %d: expected changed=false for empty right", i)
		}
		forceGC()
		arr2 := merged2.GetArray()
		if len(arr2) != 2 {
			t.Fatalf("iteration %d: expected 2 items, got %d", i, len(arr2))
		}
		runtime.KeepAlive(a)
	}
}

func TestArenaGCSafety_MergeValues_NullHandling(t *testing.T) {
	old := debug.SetGCPercent(1)
	defer debug.SetGCPercent(old)

	for i := 0; i < gcTestIterations; i++ {
		a := arena.NewMonotonicArena()
		var p Parser

		left, err := p.ParseWithArena(a, fmt.Sprintf(`{"nested":{"val":%d}}`, i))
		if err != nil {
			t.Fatalf("iteration %d: parse left: %s", i, err)
		}
		right, err := p.ParseWithArena(a, `null`)
		if err != nil {
			t.Fatalf("iteration %d: parse right: %s", i, err)
		}
		merged, changed, err := MergeValues(a, left, right)
		if err != nil {
			t.Fatalf("iteration %d: merge: %s", i, err)
		}
		if changed {
			t.Fatalf("iteration %d: expected changed=false for null right on object left", i)
		}
		forceGC()

		nested := merged.Get("nested")
		if nested == nil {
			t.Fatalf("iteration %d: nested is nil", i)
		}
		val, _ := nested.Get("val").Int()
		if val != i {
			t.Fatalf("iteration %d: nested.val got %d, want %d", i, val, i)
		}
		runtime.KeepAlive(a)
	}
}

func TestArenaGCSafety_ArenaBufferGrowth(t *testing.T) {
	old := debug.SetGCPercent(1)
	defer debug.SetGCPercent(old)

	const iterations = 100

	for i := 0; i < iterations; i++ {
		// Use a small buffer size to force multiple arena buffers
		a := arena.NewMonotonicArena(arena.WithMinBufferSize(1024))
		var p Parser

		buf := []byte(`[`)
		for j := 0; j < 200; j++ {
			if j > 0 {
				buf = append(buf, ',')
			}
			entry := fmt.Sprintf(`{"id":%d,"name":"item_%d_%d","value":"val_%d_%d_padding_to_increase_size"}`,
				j, i, j, i, j)
			buf = append(buf, entry...)
		}
		buf = append(buf, ']')

		v, err := p.ParseBytesWithArena(a, buf)
		if err != nil {
			t.Fatalf("iteration %d: parse: %s", i, err)
		}
		forceGC()

		arr := v.GetArray()
		if len(arr) != 200 {
			t.Fatalf("iteration %d: expected 200 items, got %d", i, len(arr))
		}

		for _, idx := range []int{0, 1, 50, 99, 150, 199} {
			elem := arr[idx]
			id, _ := elem.Get("id").Int()
			if id != idx {
				t.Fatalf("iteration %d: arr[%d].id got %d, want %d", i, idx, id, idx)
			}
			name, _ := elem.Get("name").StringBytes()
			expectedName := fmt.Sprintf("item_%d_%d", i, idx)
			if string(name) != expectedName {
				t.Fatalf("iteration %d: arr[%d].name got %q, want %q", i, idx, string(name), expectedName)
			}
		}

		marshaled := v.MarshalTo(nil)
		if len(marshaled) == 0 {
			t.Fatalf("iteration %d: MarshalTo returned empty", i)
		}
		runtime.KeepAlive(a)
	}
}

func TestArenaGCSafety_ObjectSet_Overwrite(t *testing.T) {
	old := debug.SetGCPercent(1)
	defer debug.SetGCPercent(old)

	for i := 0; i < gcTestIterations; i++ {
		a := arena.NewMonotonicArena()
		obj := ObjectValue(a)

		key := heapString("key", i)
		firstVal := StringValue(a, heapString("first", i))
		obj.Set(a, key, firstVal)

		secondVal := StringValue(a, heapString("second", i))
		obj.Set(a, key, secondVal)

		forceGC()

		got := obj.Get(key)
		if got == nil {
			t.Fatalf("iteration %d: key not found after overwrite", i)
		}
		sb, _ := got.StringBytes()
		expected := heapString("second", i)
		if string(sb) != expected {
			t.Fatalf("iteration %d: got %q, want %q", i, string(sb), expected)
		}

		o, _ := obj.Object()
		if o.Len() != 1 {
			t.Fatalf("iteration %d: expected 1 key, got %d", i, o.Len())
		}
		runtime.KeepAlive(a)
	}
}

func TestArenaGCSafety_NaNInf(t *testing.T) {
	old := debug.SetGCPercent(1)
	defer debug.SetGCPercent(old)

	cases := []struct {
		input string
		want  string
	}{
		{"NaN", "NaN"},
		{"nan", "nan"},
		{"Inf", "Inf"},
		{"-Inf", "-Inf"},
		{"+Inf", "+Inf"},
	}

	for i := 0; i < gcTestIterations; i++ {
		a := arena.NewMonotonicArena()
		var p Parser

		for _, tc := range cases {
			v, err := p.ParseWithArena(a, tc.input)
			if err != nil {
				t.Fatalf("iteration %d: parse %q: %s", i, tc.input, err)
			}
			forceGC()

			if v.Type() != TypeNumber {
				t.Fatalf("iteration %d: %q type got %s, want TypeNumber", i, tc.input, v.Type())
			}
			got := v.String()
			if got != tc.want {
				t.Fatalf("iteration %d: %q String() got %q, want %q", i, tc.input, got, tc.want)
			}
		}
		runtime.KeepAlive(a)
	}
}

func TestArenaGCSafety_EmptyStringValues(t *testing.T) {
	old := debug.SetGCPercent(1)
	defer debug.SetGCPercent(old)

	for i := 0; i < gcTestIterations; i++ {
		a := arena.NewMonotonicArena()
		var p Parser

		v, err := p.ParseWithArena(a, `{"empty":"","arr":["",""],"nested":{"also_empty":""}}`)
		if err != nil {
			t.Fatalf("iteration %d: parse: %s", i, err)
		}
		forceGC()

		sb, _ := v.Get("empty").StringBytes()
		if len(sb) != 0 {
			t.Fatalf("iteration %d: empty got %q, want empty", i, string(sb))
		}

		arr := v.GetArray("arr")
		if len(arr) != 2 {
			t.Fatalf("iteration %d: arr length got %d, want 2", i, len(arr))
		}
		for j, elem := range arr {
			sb, _ := elem.StringBytes()
			if len(sb) != 0 {
				t.Fatalf("iteration %d: arr[%d] got %q, want empty", i, j, string(sb))
			}
		}

		sb, _ = v.Get("nested", "also_empty").StringBytes()
		if len(sb) != 0 {
			t.Fatalf("iteration %d: nested.also_empty got %q, want empty", i, string(sb))
		}

		got := string(v.MarshalTo(nil))
		expected := `{"empty":"","arr":["",""],"nested":{"also_empty":""}}`
		if got != expected {
			t.Fatalf("iteration %d: marshal got %q, want %q", i, got, expected)
		}
		runtime.KeepAlive(a)
	}
}

//go:noinline
func buildLargeEscapedJSON(i int) string {
	s := `{"escaped":"`
	for j := 0; j < 100; j++ {
		s += fmt.Sprintf(`\t\n\r\\\/\b\f\u00%02X`, 0x41+(j%26))
	}
	s += fmt.Sprintf(`_%d"}`, i)
	return s
}

func TestArenaGCSafety_LargeEscapedStrings(t *testing.T) {
	old := debug.SetGCPercent(1)
	defer debug.SetGCPercent(old)

	for i := 0; i < gcTestIterations; i++ {
		a := arena.NewMonotonicArena()
		var p Parser

		input := buildLargeEscapedJSON(i)
		v, err := p.ParseWithArena(a, input)
		if err != nil {
			t.Fatalf("iteration %d: parse: %s", i, err)
		}
		input = ""
		_ = input
		forceGC()

		val := v.Get("escaped")
		if val == nil {
			t.Fatalf("iteration %d: escaped is nil", i)
		}
		sb, _ := val.StringBytes()
		suffix := fmt.Sprintf("_%d", i)
		if len(sb) == 0 || string(sb[len(sb)-len(suffix):]) != suffix {
			t.Fatalf("iteration %d: string doesn't end with %q", i, suffix)
		}
		if len(sb) < 100 {
			t.Fatalf("iteration %d: decoded string too short: %d", i, len(sb))
		}

		marshaled := string(v.MarshalTo(nil))
		if len(marshaled) == 0 {
			t.Fatalf("iteration %d: MarshalTo returned empty", i)
		}
		runtime.KeepAlive(a)
	}
}

func TestArenaGCSafety_ManyObjectKeys(t *testing.T) {
	old := debug.SetGCPercent(1)
	defer debug.SetGCPercent(old)

	const iterations = 200
	const numKeys = 200

	for i := 0; i < iterations; i++ {
		a := arena.NewMonotonicArena()
		var p Parser

		buf := []byte(`{`)
		for j := 0; j < numKeys; j++ {
			if j > 0 {
				buf = append(buf, ',')
			}
			entry := fmt.Sprintf(`"key_%d_%d":"val_%d_%d"`, i, j, i, j)
			buf = append(buf, entry...)
		}
		buf = append(buf, '}')

		v, err := p.ParseBytesWithArena(a, buf)
		if err != nil {
			t.Fatalf("iteration %d: parse: %s", i, err)
		}
		forceGC()

		o, _ := v.Object()
		if o.Len() != numKeys {
			t.Fatalf("iteration %d: expected %d keys, got %d", i, numKeys, o.Len())
		}

		for _, idx := range []int{0, 1, 50, 99, 150, 199} {
			key := fmt.Sprintf("key_%d_%d", i, idx)
			val := v.Get(key)
			if val == nil {
				t.Fatalf("iteration %d: key %q is nil", i, key)
			}
			sb, _ := val.StringBytes()
			expected := fmt.Sprintf("val_%d_%d", i, idx)
			if string(sb) != expected {
				t.Fatalf("iteration %d: key %q got %q, want %q", i, key, string(sb), expected)
			}
		}

		marshaled := v.MarshalTo(nil)
		if len(marshaled) == 0 {
			t.Fatalf("iteration %d: MarshalTo returned empty", i)
		}
		runtime.KeepAlive(a)
	}
}
