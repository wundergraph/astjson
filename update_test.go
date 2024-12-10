package astjson

import (
	"encoding/json"
	"fmt"
	"strings"
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

func TestMergeWithSwap(t *testing.T) {
	left := MustParse(`{"a":{"b":1,"c":2,"e":[],"f":[1],"h":[1,2,3]}}`)
	right := MustParse(`{"a":{"b":2,"d":3,"e":[1,2,3],"g":[1],"h":[4,5,6]}}`)
	out, _ := MergeValues(left, right)
	assert.Equal(t, `{"a":{"b":2,"c":2,"e":[1,2,3],"f":[1],"h":[4,5,6],"d":3,"g":[1]}}`, out.String())
}

type RootObject struct {
	Child *ChildObject `json:"child"`
}

type ChildObject struct {
	GrandChild *GrandChildObject `json:"grand_child"`
}

type GrandChildObject struct {
	Items []string `json:"items"`
}

func BenchmarkValue_SetArrayItem(b *testing.B) {

	root := &RootObject{
		Child: &ChildObject{
			GrandChild: &GrandChildObject{
				Items: make([]string, 0),
			},
		},
	}

	l, err := json.Marshal(root)
	assert.NoError(b, err)

	root.Child.GrandChild.Items = make([]string, 1024*1024)

	for i := 0; i < 1024*1024; i++ {
		root.Child.GrandChild.Items[i] = strings.Repeat("a", 1024)
	}

	r, err := json.Marshal(root)
	assert.NoError(b, err)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		l, _ := ParseBytesWithoutCache(l)
		r, _ := ParseBytesWithoutCache(r)
		out, _ := MergeValues(l, r)
		arr := out.GetArray("child", "grand_child", "items")
		assert.Len(b, arr, 1024*1024)
	}
}

func BenchmarkMergeValuesWithATonOfRecursion(b *testing.B) {
	b.ReportAllocs()

	left := MustParse(`{"a":{}}`)
	str := fmt.Sprintf(
		`{"ba":{"bb":{"bc":{"bd":[%s, %s, %s], "be": {"bf": %s}}}}}`,
		objectWithRecursion(10),
		objectWithRecursion(20),
		objectWithRecursion(2),
		objectWithRecursion(3))

	expected := fmt.Sprintf(
		`{"a":{},"ba":{"bb":{"bc":{"bd":[%s,%s,%s],"be":{"bf":%s}}}}}`,
		objectWithRecursion(10),
		objectWithRecursion(20),
		objectWithRecursion(2),
		objectWithRecursion(3))

	_ = expected

	right := MustParse(str)

	for i := 0; i < b.N; i++ {
		_, _ = MergeValues(left, right)
		/*v, _ := MergeValues(left, right)
		if v.String() != expected {
			assert.Equal(b, expected, v.String())
		}*/
	}

	fmt.Printf("left: %s\n", left.String())
}

func objectWithRecursion(depth int) string {
	if depth == 0 {
		return `{}`
	}
	return `{"a":` + objectWithRecursion(depth-1) + `}`
}
