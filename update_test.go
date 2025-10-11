package astjson

import (
	"strings"
	"testing"

	"github.com/wundergraph/go-arena"
)

func TestObjectDelSet(t *testing.T) {
	var p Parser
	var o *Object
	a := arena.NewMonotonicArena()

	o.Del("xx")

	v, err := p.Parse(`{"fo\no": "bar", "x": [1,2,3]}`)
	if err != nil {
		t.Fatalf("unexpected error during parse: %s", err)
	}
	o, err = v.Object()
	if err != nil {
		t.Fatalf("cannot obtain object: %s", err)
	}

	// Delete x
	o.Del("x")
	if o.Len() != 1 {
		t.Fatalf("unexpected number of items left; got %d; want %d", o.Len(), 1)
	}

	// Try deleting non-existing value
	o.Del("xxx")
	if o.Len() != 1 {
		t.Fatalf("unexpected number of items left; got %d; want %d", o.Len(), 1)
	}

	// Set new value
	vNew := MustParse(`{"foo":[1,2,3]}`)
	o.Set(a, "new_key", vNew)

	// Delete item with escaped key
	o.Del("fo\no")
	if o.Len() != 1 {
		t.Fatalf("unexpected number of items left; got %d; want %d", o.Len(), 1)
	}

	str := o.String()
	strExpected := `{"new_key":{"foo":[1,2,3]}}`
	if str != strExpected {
		t.Fatalf("unexpected string representation for o: got %q; want %q", str, strExpected)
	}

	// Set and Del function as no-op on nil value
	o = nil
	o.Del("x")
	o.Set(a, "x", MustParse(`[3]`))
}

func TestValueDelSet(t *testing.T) {
	var p Parser
	ar := arena.NewMonotonicArena()
	v, err := p.Parse(`{"xx": 123, "x": [1,2,3]}`)
	if err != nil {
		t.Fatalf("unexpected error during parse: %s", err)
	}

	// Delete xx
	v.Del("xx")
	n := v.GetObject().Len()
	if n != 1 {
		t.Fatalf("unexpected number of items left; got %d; want %d", n, 1)
	}

	// Try deleting non-existing value in the array
	va := v.Get("x")
	va.Del("foobar")

	// Delete middle element in the array
	va.Del("1")
	a := v.GetArray("x")
	if len(a) != 2 {
		t.Fatalf("unexpected number of items left in the array; got %d; want %d", len(a), 2)
	}

	// Update the first element in the array
	vNew := MustParse(`"foobar"`)
	va.Set(ar, "0", vNew)

	// Add third element to the array
	vNew = MustParse(`[3]`)
	va.Set(ar, "3", vNew)

	// Add invalid array index to the array
	va.Set(ar, "invalid", MustParse(`"nonsense"`))

	str := v.String()
	strExpected := `{"x":["foobar",3,null,[3]]}`
	if str != strExpected {
		t.Fatalf("unexpected string representation for o: got %q; want %q", str, strExpected)
	}

	// Set and Del function as no-op on nil value
	v = nil
	v.Del("x")
	v.Set(ar, "x", MustParse(`[]`))
	v.SetArrayItem(ar, 1, MustParse(`[]`))
}

func TestValue_AppendArrayItems(t *testing.T) {
	left := MustParse(`[1,2,3]`)
	right := MustParse(`[4,5,6]`)
	left.AppendArrayItems(right)
	if len(left.GetArray()) != 6 {
		t.Fatalf("unexpected length; got %d; want %d", len(left.GetArray()), 6)
	}
	out := left.MarshalTo(nil)
	if string(out) != `[1,2,3,4,5,6]` {
		t.Fatalf("unexpected output; got %q; want %q", out, `[1,2,3,4,5,6]`)
	}
}

func TestObjectSetComprehensive(t *testing.T) {
	t.Run("nil object", func(t *testing.T) {
		var o *Object
		a := arena.NewMonotonicArena()
		o.Set(a, "key", MustParse(`"value"`))
		// Should not panic and should be no-op
	})

	t.Run("nil value becomes null", func(t *testing.T) {
		var p Parser
		v, err := p.Parse(`{"existing": "value"}`)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		o, err := v.Object()
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		a := arena.NewMonotonicArena()

		o.Set(a, "new_key", nil)
		result := o.Get("new_key")
		if result == nil {
			t.Fatalf("expected non-nil result")
		}
		if result.Type() != TypeNull {
			t.Fatalf("expected null type, got %s", result.Type())
		}
	})

	t.Run("set new key", func(t *testing.T) {
		var p Parser
		v, err := p.Parse(`{"existing": "value"}`)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		o, err := v.Object()
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		a := arena.NewMonotonicArena()

		newValue := MustParse(`"new_value"`)
		o.Set(a, "new_key", newValue)

		result := o.Get("new_key")
		if result == nil {
			t.Fatalf("expected non-nil result")
		}
		if string(result.GetStringBytes()) != "new_value" {
			t.Fatalf("expected 'new_value', got %q", string(result.GetStringBytes()))
		}

		// Verify the key was marked as unescaped
		for _, kv := range o.kvs {
			if kv.k == "new_key" && !kv.keyUnescaped {
				t.Fatalf("expected new key to be marked as unescaped")
			}
		}
	})

	t.Run("update existing key", func(t *testing.T) {
		var p Parser
		v, err := p.Parse(`{"existing": "old_value"}`)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		o, err := v.Object()
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		a := arena.NewMonotonicArena()

		newValue := MustParse(`"new_value"`)
		o.Set(a, "existing", newValue)

		result := o.Get("existing")
		if result == nil {
			t.Fatalf("expected non-nil result")
		}
		if string(result.GetStringBytes()) != "new_value" {
			t.Fatalf("expected 'new_value', got %q", string(result.GetStringBytes()))
		}

		// Verify object length didn't change
		if o.Len() != 1 {
			t.Fatalf("expected length 1, got %d", o.Len())
		}
	})

	t.Run("set key with escape sequences", func(t *testing.T) {
		var p Parser
		v, err := p.Parse(`{"normal_key": "value"}`)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		o, err := v.Object()
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		a := arena.NewMonotonicArena()

		// Set a key that contains escape sequences
		o.Set(a, "key\\with\\escapes", MustParse(`"escaped_value"`))

		result := o.Get("key\\with\\escapes")
		if result == nil {
			t.Fatalf("expected non-nil result")
		}
		if string(result.GetStringBytes()) != "escaped_value" {
			t.Fatalf("expected 'escaped_value', got %q", string(result.GetStringBytes()))
		}
	})

	t.Run("update existing escaped key", func(t *testing.T) {
		var p Parser
		v, err := p.Parse(`{"key\\with\\escapes": "old_value"}`)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		o, err := v.Object()
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		a := arena.NewMonotonicArena()

		// Update the existing escaped key
		o.Set(a, "key\\with\\escapes", MustParse(`"new_value"`))

		result := o.Get("key\\with\\escapes")
		if result == nil {
			t.Fatalf("expected non-nil result")
		}
		if string(result.GetStringBytes()) != "new_value" {
			t.Fatalf("expected 'new_value', got %q", string(result.GetStringBytes()))
		}
	})

	t.Run("set multiple keys", func(t *testing.T) {
		var p Parser
		v, err := p.Parse(`{}`)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		o, err := v.Object()
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		a := arena.NewMonotonicArena()

		// Set multiple keys
		o.Set(a, "key1", MustParse(`"value1"`))
		o.Set(a, "key2", MustParse(`"value2"`))
		o.Set(a, "key3", MustParse(`"value3"`))

		if o.Len() != 3 {
			t.Fatalf("expected length 3, got %d", o.Len())
		}

		// Verify all keys are accessible
		if string(o.Get("key1").GetStringBytes()) != "value1" {
			t.Fatalf("unexpected value for key1")
		}
		if string(o.Get("key2").GetStringBytes()) != "value2" {
			t.Fatalf("unexpected value for key2")
		}
		if string(o.Get("key3").GetStringBytes()) != "value3" {
			t.Fatalf("unexpected value for key3")
		}
	})

	t.Run("set different value types", func(t *testing.T) {
		var p Parser
		v, err := p.Parse(`{}`)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		o, err := v.Object()
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		a := arena.NewMonotonicArena()

		// Set different types of values
		o.Set(a, "string_key", MustParse(`"string_value"`))
		o.Set(a, "number_key", MustParse(`123`))
		o.Set(a, "bool_key", MustParse(`true`))
		o.Set(a, "null_key", MustParse(`null`))
		o.Set(a, "array_key", MustParse(`[1,2,3]`))
		o.Set(a, "object_key", MustParse(`{"nested": "value"}`))

		if o.Len() != 6 {
			t.Fatalf("expected length 6, got %d", o.Len())
		}

		// Verify types
		if o.Get("string_key").Type() != TypeString {
			t.Fatalf("expected string type")
		}
		if o.Get("number_key").Type() != TypeNumber {
			t.Fatalf("expected number type")
		}
		if o.Get("bool_key").Type() != TypeTrue {
			t.Fatalf("expected true type")
		}
		if o.Get("null_key").Type() != TypeNull {
			t.Fatalf("expected null type")
		}
		if o.Get("array_key").Type() != TypeArray {
			t.Fatalf("expected array type")
		}
		if o.Get("object_key").Type() != TypeObject {
			t.Fatalf("expected object type")
		}
	})

	t.Run("set with nil arena", func(t *testing.T) {
		var p Parser
		v, err := p.Parse(`{"existing": "value"}`)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		o, err := v.Object()
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		// Set with nil arena should still work (falls back to heap allocation)
		o.Set(nil, "new_key", MustParse(`"new_value"`))

		result := o.Get("new_key")
		if result == nil {
			t.Fatalf("expected non-nil result")
		}
		if string(result.GetStringBytes()) != "new_value" {
			t.Fatalf("expected 'new_value', got %q", string(result.GetStringBytes()))
		}
	})

	t.Run("set empty key", func(t *testing.T) {
		var p Parser
		v, err := p.Parse(`{}`)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		o, err := v.Object()
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		a := arena.NewMonotonicArena()

		o.Set(a, "", MustParse(`"empty_key_value"`))

		result := o.Get("")
		if result == nil {
			t.Fatalf("expected non-nil result")
		}
		if string(result.GetStringBytes()) != "empty_key_value" {
			t.Fatalf("expected 'empty_key_value', got %q", string(result.GetStringBytes()))
		}
	})

	t.Run("set key with special characters", func(t *testing.T) {
		var p Parser
		v, err := p.Parse(`{}`)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		o, err := v.Object()
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		a := arena.NewMonotonicArena()

		specialKeys := []string{
			"key with spaces",
			"key-with-dashes",
			"key_with_underscores",
			"key.with.dots",
			"key/with/slashes",
			"key:with:colons",
			"key;with;semicolons",
			"key,with,commas",
			"key[with]brackets",
			"key{with}braces",
			"key(with)parentheses",
			"key<with>angles",
			"key\"with\"quotes",
			"key'with'apostrophes",
			"key\nwith\nnewlines",
			"key\twith\ttabs",
			"key\rwith\rcarriage",
		}

		for i, key := range specialKeys {
			o.Set(a, key, MustParse(`"value"`))
			result := o.Get(key)
			if result == nil {
				t.Fatalf("expected non-nil result for key %d: %q", i, key)
			}
		}

		if o.Len() != len(specialKeys) {
			t.Fatalf("expected length %d, got %d", len(specialKeys), o.Len())
		}
	})

	t.Run("set unicode keys", func(t *testing.T) {
		var p Parser
		v, err := p.Parse(`{}`)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		o, err := v.Object()
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		a := arena.NewMonotonicArena()

		unicodeKeys := []string{
			"ключ",       // Russian
			"键",          // Chinese
			"キー",         // Japanese
			"مفتاح",      // Arabic
			"🔑",          // Emoji
			"café",       // French with accent
			"naïve",      // French with diaeresis
			"café naïve", // Combined
		}

		for i, key := range unicodeKeys {
			o.Set(a, key, MustParse(`"value"`))
			result := o.Get(key)
			if result == nil {
				t.Fatalf("expected non-nil result for unicode key %d: %q", i, key)
			}
		}

		if o.Len() != len(unicodeKeys) {
			t.Fatalf("expected length %d, got %d", len(unicodeKeys), o.Len())
		}
	})
}

func TestObjectSetEdgeCases(t *testing.T) {
	t.Run("set same key multiple times", func(t *testing.T) {
		var p Parser
		v, err := p.Parse(`{}`)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		o, err := v.Object()
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		a := arena.NewMonotonicArena()

		// Set the same key multiple times
		o.Set(a, "key", MustParse(`"value1"`))
		o.Set(a, "key", MustParse(`"value2"`))
		o.Set(a, "key", MustParse(`"value3"`))

		if o.Len() != 1 {
			t.Fatalf("expected length 1, got %d", o.Len())
		}

		result := o.Get("key")
		if result == nil {
			t.Fatalf("expected non-nil result")
		}
		if string(result.GetStringBytes()) != "value3" {
			t.Fatalf("expected 'value3', got %q", string(result.GetStringBytes()))
		}
	})

	t.Run("set after get operation", func(t *testing.T) {
		var p Parser
		v, err := p.Parse(`{"existing": "value"}`)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		o, err := v.Object()
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		a := arena.NewMonotonicArena()

		// Get existing key (this might trigger unescaping)
		existing := o.Get("existing")
		if existing == nil {
			t.Fatalf("expected non-nil result")
		}

		// Now set a new key
		o.Set(a, "new_key", MustParse(`"new_value"`))

		result := o.Get("new_key")
		if result == nil {
			t.Fatalf("expected non-nil result")
		}
		if string(result.GetStringBytes()) != "new_value" {
			t.Fatalf("expected 'new_value', got %q", string(result.GetStringBytes()))
		}
	})

	t.Run("set with very long key", func(t *testing.T) {
		var p Parser
		v, err := p.Parse(`{}`)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		o, err := v.Object()
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		a := arena.NewMonotonicArena()

		// Create a very long key
		longKey := strings.Repeat("a", 10000)
		o.Set(a, longKey, MustParse(`"value"`))

		result := o.Get(longKey)
		if result == nil {
			t.Fatalf("expected non-nil result")
		}
		if string(result.GetStringBytes()) != "value" {
			t.Fatalf("expected 'value', got %q", string(result.GetStringBytes()))
		}
	})

	t.Run("set with very long value", func(t *testing.T) {
		var p Parser
		v, err := p.Parse(`{}`)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		o, err := v.Object()
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		a := arena.NewMonotonicArena()

		// Create a very long value
		longValue := strings.Repeat("x", 10000)
		o.Set(a, "key", MustParse(`"`+longValue+`"`))

		result := o.Get("key")
		if result == nil {
			t.Fatalf("expected non-nil result")
		}
		if string(result.GetStringBytes()) != longValue {
			t.Fatalf("expected long value, got different length: %d", len(string(result.GetStringBytes())))
		}
	})
}

func TestObjectDelWithNilArena(t *testing.T) {
	// Test that Del method works correctly when unescapeKeys is called with nil arena
	var p Parser
	v, err := p.Parse(`{"fo\no": "bar", "x": [1,2,3], "escaped\\key": "value"}`)
	if err != nil {
		t.Fatalf("unexpected error during parse: %s", err)
	}
	o, err := v.Object()
	if err != nil {
		t.Fatalf("cannot obtain object: %s", err)
	}

	// This should trigger the slow path and call unescapeKeys(nil)
	// The go-arena library should handle nil arena gracefully
	o.Del("fo\no")
	if o.Len() != 2 {
		t.Fatalf("unexpected number of items left; got %d; want %d", o.Len(), 2)
	}

	// Test with another escaped key
	o.Del("escaped\\key")
	if o.Len() != 1 {
		t.Fatalf("unexpected number of items left; got %d; want %d", o.Len(), 1)
	}

	// Verify the remaining key
	if o.Get("x") == nil {
		t.Fatalf("expected key 'x' to still exist")
	}
}
