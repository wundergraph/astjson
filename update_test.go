package astjson

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestObjectDelSet(t *testing.T) {
	var p Parser
	var o *Object

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
	o.Set("new_key", vNew)

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
	o.Set("x", MustParse(`[3]`))
}

func TestValueDelSet(t *testing.T) {
	var p Parser
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
	va.Set("0", vNew)

	// Add third element to the array
	vNew = MustParse(`[3]`)
	va.Set("3", vNew)

	// Add invalid array index to the array
	va.Set("invalid", MustParse(`"nonsense"`))

	str := v.String()
	strExpected := `{"x":["foobar",3,null,[3]]}`
	if str != strExpected {
		t.Fatalf("unexpected string representation for o: got %q; want %q", str, strExpected)
	}

	// Set and Del function as no-op on nil value
	v = nil
	v.Del("x")
	v.Set("x", MustParse(`[]`))
	v.SetArrayItem(1, MustParse(`[]`))
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

func BenchmarkValue_SetArrayItem(b *testing.B) {
	input := []byte(`1`)
	leftInput := make([]any, 2)
	for i := 0; i < 2; i++ {
		err := json.Unmarshal(input, &leftInput[i])
		assert.NoError(b, err)
	}
	rightInput := make([]any, 1024*1024)
	for i := 0; i < 1024*1024; i++ {
		err := json.Unmarshal(input, &rightInput[i])
		assert.NoError(b, err)
	}

	left, err := json.Marshal(leftInput)
	assert.NoError(b, err)
	right, err := json.Marshal(rightInput)
	assert.NoError(b, err)

	expectedLen := 2 + 1024*1024

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		l, _ := ParseBytesWithoutCache(left)
		r, _ := ParseBytesWithoutCache(right)
		out, _ := MergeValues(l, r)
		if len(out.GetArray()) != expectedLen {
			b.Fatalf("unexpected length; got %d; want %d", len(out.GetArray()), expectedLen)
		}
	}
}
