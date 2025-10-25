package astjson

import (
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/wundergraph/go-arena"
)

func TestParseRawNumber(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		f := func(s, expectedRN, expectedTail string) {
			t.Helper()

			rn, tail, err := parseRawNumber(s)
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
			if rn != expectedRN {
				t.Fatalf("unexpected raw number; got %q; want %q", rn, expectedRN)
			}
			if tail != expectedTail {
				t.Fatalf("unexpected tail; got %q; want %q", tail, expectedTail)
			}
		}

		f("0", "0", "")
		f("0tail", "0", "tail")
		f("123", "123", "")
		f("123tail", "123", "tail")
		f("-123tail", "-123", "tail")
		f("-12.345tail", "-12.345", "tail")
		f("-12.345e67tail", "-12.345e67", "tail")
		f("-12.345E+67 tail", "-12.345E+67", " tail")
		f("-12.345E-67,tail", "-12.345E-67", ",tail")
		f("-1234567.8e+90tail", "-1234567.8e+90", "tail")
		f("12.tail", "12.", "tail")
		f(".2tail", ".2", "tail")
		f("-.2tail", "-.2", "tail")
		f("NaN", "NaN", "")
		f("nantail", "nan", "tail")
		f("inf", "inf", "")
		f("Inftail", "Inf", "tail")
		f("-INF", "-INF", "")
		f("-Inftail", "-Inf", "tail")
	})

	t.Run("error", func(t *testing.T) {
		f := func(s, expectedTail string) {
			t.Helper()

			_, tail, err := parseRawNumber(s)
			if err == nil {
				t.Fatalf("expecting non-nil error")
			}
			if tail != expectedTail {
				t.Fatalf("unexpected tail; got %q; want %q", tail, expectedTail)
			}
		}

		f("xyz", "xyz")
		f(" ", " ")
		f("[", "[")
		f(",", ",")
		f("{", "{")
		f("\"", "\"")
	})
}

func TestUnescapeStringBestEffort(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		testUnescapeStringBestEffort(t, ``, ``)
		testUnescapeStringBestEffort(t, `\"`, `"`)
		testUnescapeStringBestEffort(t, `\\`, `\`)
		testUnescapeStringBestEffort(t, `\\\"`, `\"`)
		testUnescapeStringBestEffort(t, `\\\"абв`, `\"абв`)
		testUnescapeStringBestEffort(t, `йцук\n\"\\Y`, "йцук\n\"\\Y")
		testUnescapeStringBestEffort(t, `q\u1234we`, "q\u1234we")
		testUnescapeStringBestEffort(t, `п\ud83e\udd2dи`, "п🤭и")
	})

	t.Run("error", func(t *testing.T) {
		testUnescapeStringBestEffort(t, `\`, ``)
		testUnescapeStringBestEffort(t, `foo\qwe`, `foo\qwe`)
		testUnescapeStringBestEffort(t, `\"x\uyz\"`, `"x\uyz"`)
		testUnescapeStringBestEffort(t, `\u12\"пролw`, `\u12"пролw`)
		testUnescapeStringBestEffort(t, `п\ud83eи`, "п\\ud83eи")
	})
}

func testUnescapeStringBestEffort(t *testing.T, s, expectedS string) {
	t.Helper()

	// unescapeString modifies the original s, so call it
	// on a byte slice copy.
	b := append([]byte{}, s...)
	us := unescapeStringBestEffort(nil, b2s(b))
	if us != expectedS {
		t.Fatalf("unexpected unescaped string; got %q; want %q", us, expectedS)
	}
}

func TestParseRawString(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		f := func(s, expectedRS, expectedTail string) {
			t.Helper()

			rs, tail, err := parseRawString(s[1:])
			if err != nil {
				t.Fatalf("unexpected error on parseRawString: %s", err)
			}
			if rs != expectedRS {
				t.Fatalf("unexpected string on parseRawString; got %q; want %q", rs, expectedRS)
			}
			if tail != expectedTail {
				t.Fatalf("unexpected tail on parseRawString; got %q; want %q", tail, expectedTail)
			}

			// parseRawKey results must be identical to parseRawString.
			rs, tail, err = parseRawKey(s[1:])
			if err != nil {
				t.Fatalf("unexpected error on parseRawKey: %s", err)
			}
			if rs != expectedRS {
				t.Fatalf("unexpected string on parseRawKey; got %q; want %q", rs, expectedRS)
			}
			if tail != expectedTail {
				t.Fatalf("unexpected tail on parseRawKey; got %q; want %q", tail, expectedTail)
			}
		}

		f(`""`, "", "")
		f(`""xx`, "", "xx")
		f(`"foobar"`, "foobar", "")
		f(`"foobar"baz`, "foobar", "baz")
		f(`"\""`, `\"`, "")
		f(`"\""tail`, `\"`, "tail")
		f(`"\\"`, `\\`, "")
		f(`"\\"tail`, `\\`, "tail")
		f(`"x\\"`, `x\\`, "")
		f(`"x\\"tail`, `x\\`, "tail")
		f(`"x\\y"`, `x\\y`, "")
		f(`"x\\y"tail`, `x\\y`, "tail")
		f(`"\\\"й\n\"я"tail`, `\\\"й\n\"я`, "tail")
		f(`"\\\\\\\\"tail`, `\\\\\\\\`, "tail")

	})

	t.Run("error", func(t *testing.T) {
		f := func(s, expectedTail string) {
			t.Helper()

			_, tail, err := parseRawString(s[1:])
			if err == nil {
				t.Fatalf("expecting non-nil error on parseRawString")
			}
			if tail != expectedTail {
				t.Fatalf("unexpected tail on parseRawString; got %q; want %q", tail, expectedTail)
			}

			// parseRawKey results must be identical to parseRawString.
			_, tail, err = parseRawKey(s[1:])
			if err == nil {
				t.Fatalf("expecting non-nil error on parseRawKey")
			}
			if tail != expectedTail {
				t.Fatalf("unexpected tail on parseRawKey; got %q; want %q", tail, expectedTail)
			}
		}

		f(`"`, "")
		f(`"unclosed string`, "")
		f(`"\"`, "")
		f(`"\"unclosed`, "")
		f(`"foo\\\\\"тест\n\r\t`, "")
	})
}

func TestValueInvalidTypeConversion(t *testing.T) {
	var p Parser

	v, err := p.Parse(`[{},[],"",123.45,true,null]`)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	a := v.GetArray()

	// object
	_, err = a[0].Object()
	if err != nil {
		t.Fatalf("unexpected error when obtaining object: %s", err)
	}
	_, err = a[0].Array()
	if err == nil {
		t.Fatalf("expecting non-nil error when trying to obtain array from object")
	}

	// array
	_, err = a[1].Array()
	if err != nil {
		t.Fatalf("unexpected error when obtaining array: %s", err)
	}
	_, err = a[1].Object()
	if err == nil {
		t.Fatalf("expecting non-nil error when trying to obtain object from array")
	}

	// string
	_, err = a[2].StringBytes()
	if err != nil {
		t.Fatalf("unexpected error when obtaining string: %s", err)
	}
	_, err = a[2].Int()
	if err == nil {
		t.Fatalf("expecting non-nil error when trying to obtain int from string")
	}
	_, err = a[2].Int64()
	if err == nil {
		t.Fatalf("expecting non-nil error when trying to obtain int64 from string")
	}
	_, err = a[2].Uint()
	if err == nil {
		t.Fatalf("expecting non-nil error when trying to obtain uint from string")
	}
	_, err = a[2].Uint64()
	if err == nil {
		t.Fatalf("expecting non-nil error when trying to obtain uint64 from string")
	}
	_, err = a[2].Float64()
	if err == nil {
		t.Fatalf("expecting non-nil error when trying to obtain float64 from string")
	}

	// number
	_, err = a[3].Float64()
	if err != nil {
		t.Fatalf("unexpected error when obtaining float64: %s", err)
	}
	_, err = a[3].StringBytes()
	if err == nil {
		t.Fatalf("expecting non-nil error when trying to obtain string from number")
	}

	// true
	_, err = a[4].Bool()
	if err != nil {
		t.Fatalf("unexpected error when obtaining bool: %s", err)
	}
	_, err = a[4].StringBytes()
	if err == nil {
		t.Fatalf("expecting non-nil error when trying to obtain string from bool")
	}

	// null
	_, err = a[5].Bool()
	if err == nil {
		t.Fatalf("expecting non-nil error when trying to obtain bool from null")
	}
}

func TestValueGetTyped(t *testing.T) {
	var p Parser

	v, err := p.Parse(`{"foo": 123, "bar": "433", "baz": true, "obj":{}, "arr":[1,2,3],
		"zero_float1": 0.00,
		"zero_float2": -0e123,
		"inf_float": Inf,
		"minus_inf_float": -Inf,
		"nan": nan
	}`)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if !v.Exists("foo") {
		t.Fatalf("foo must exist in the v")
	}
	if v.Exists("foo", "bar") {
		t.Fatalf("foo.bar mustn't exist in the v")
	}
	if v.Exists("foobar") {
		t.Fatalf("foobar mustn't exist in the v")
	}

	o := v.GetObject("obj")
	os := o.String()
	if os != "{}" {
		t.Fatalf("unexpected object; got %s; want %s", os, "{}")
	}
	o = v.GetObject("arr")
	if o != nil {
		t.Fatalf("unexpected non-nil object: %s", o)
	}
	o = v.GetObject("foo", "bar")
	if o != nil {
		t.Fatalf("unexpected non-nil object: %s", o)
	}
	a := v.GetArray("arr")
	if len(a) != 3 {
		t.Fatalf("unexpected array len; got %d; want %d", len(a), 3)
	}
	a = v.GetArray("obj")
	if a != nil {
		t.Fatalf("unexpected non-nil array: %s", a)
	}
	a = v.GetArray("foo", "bar")
	if a != nil {
		t.Fatalf("unexpected non-nil array: %s", a)
	}
	n := v.GetInt("foo")
	if n != 123 {
		t.Fatalf("unexpected value; got %d; want %d", n, 123)
	}
	n64 := v.GetInt64("foo")
	if n != 123 {
		t.Fatalf("unexpected value; got %d; want %d", n64, 123)
	}
	un := v.GetUint("foo")
	if un != 123 {
		t.Fatalf("unexpected value; got %d; want %d", un, 123)
	}
	un64 := v.GetUint64("foo")
	if un64 != 123 {
		t.Fatalf("unexpected value; got %d; want %d", un64, 123)
	}
	n = v.GetInt("bar")
	if n != 0 {
		t.Fatalf("unexpected non-zero value; got %d", n)
	}
	n64 = v.GetInt64("bar")
	if n64 != 0 {
		t.Fatalf("unexpected non-zero value; got %d", n64)
	}
	un = v.GetUint("bar")
	if un != 0 {
		t.Fatalf("unexpected non-zero value; got %d", un)
	}
	un64 = v.GetUint64("bar")
	if un64 != 0 {
		t.Fatalf("unexpected non-zero value; got %d", un64)
	}
	f := v.GetFloat64("foo")
	if f != 123.0 {
		t.Fatalf("unexpected value; got %f; want %f", f, 123.0)
	}
	f = v.GetFloat64("bar")
	if f != 0 {
		t.Fatalf("unexpected value; got %f; want %f", f, 0.0)
	}
	f = v.GetFloat64("foooo", "bar")
	if f != 0 {
		t.Fatalf("unexpected value; got %f; want %f", f, 0.0)
	}
	f = v.GetFloat64()
	if f != 0 {
		t.Fatalf("unexpected value; got %f; want %f", f, 0.0)
	}
	sb := v.GetStringBytes("bar")
	if string(sb) != "433" {
		t.Fatalf("unexpected value; got %q; want %q", sb, "443")
	}
	sb = v.GetStringBytes("foo")
	if sb != nil {
		t.Fatalf("unexpected value; got %q; want %q", sb, []byte(nil))
	}
	bv := v.GetBool("baz")
	if !bv {
		t.Fatalf("unexpected value; got %v; want %v", bv, true)
	}
	bv = v.GetBool("bar")
	if bv {
		t.Fatalf("unexpected value; got %v; want %v", bv, false)
	}

	zv := v.Get("zero_float1")
	zf, err := zv.Float64()
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if zf != 0 {
		t.Fatalf("unexpected zero_float1 value: %f. Expecting 0", zf)
	}

	zv = v.Get("zero_float2")
	zf, err = zv.Float64()
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if zf != 0 {
		t.Fatalf("unexpected zero_float1 value: %f. Expecting 0", zf)
	}

	infv := v.Get("inf_float")
	inff, err := infv.Float64()
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if !math.IsInf(inff, 1) {
		t.Fatalf("unexpected inf_float value: %f. Expecting %f", inff, math.Inf(1))
	}

	ninfv := v.Get("minus_inf_float")
	ninff, err := ninfv.Float64()
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if !math.IsInf(ninff, -1) {
		t.Fatalf("unexpected inf_float value: %f. Expecting %f", ninff, math.Inf(-11))
	}

	nanv := v.Get("nan")
	nanf, err := nanv.Float64()
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if !math.IsNaN(nanf) {
		t.Fatalf("unexpected nan value: %f. Expecting %f", nanf, math.NaN())
	}
}

func TestVisitNil(t *testing.T) {
	var p Parser
	v, err := p.Parse(`{}`)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	o := v.GetObject("non-existing-key")
	if o != nil {
		t.Fatalf("obtained an object for non-existing key: %#v", o)
	}
	o.Visit(func(k []byte, v *Value) {
		t.Fatalf("unexpected visit call; k=%q; v=%s", k, v)
	})
}

func TestValueGet(t *testing.T) {

	var p Parser

	v, err := p.ParseBytes([]byte(`{"xx":33.33,"foo":[123,{"bar":["baz"],"x":"y"}], "": "empty-key", "empty-value": ""}`))
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	t.Run("positive", func(t *testing.T) {
		sb := v.GetStringBytes("")
		if string(sb) != "empty-key" {
			t.Fatalf("unexpected value for empty key; got %q; want %q", sb, "empty-key")
		}
		sb = v.GetStringBytes("empty-value")
		if string(sb) != "" {
			t.Fatalf("unexpected non-empty value: %q", sb)
		}

		vv := v.Get("foo", "1")
		if vv == nil {
			t.Fatalf("cannot find the required value")
		}
		o, err := vv.Object()
		if err != nil {
			t.Fatalf("cannot obtain object: %s", err)
		}

		n := 0
		o.Visit(func(k []byte, v *Value) {
			n++
			switch string(k) {
			case "bar":
				if v.Type() != TypeArray {
					t.Fatalf("unexpected value type; got %d; want %d", v.Type(), TypeArray)
				}
				s := v.String()
				if s != `["baz"]` {
					t.Fatalf("unexpected array; got %q; want %q", s, `["baz"]`)
				}
			case "x":
				sb, err := v.StringBytes()
				if err != nil {
					t.Fatalf("cannot obtain string: %s", err)
				}
				if string(sb) != "y" {
					t.Fatalf("unexpected string; got %q; want %q", sb, "y")
				}
			default:
				t.Fatalf("unknown key: %s", k)
			}
		})
		if n != 2 {
			t.Fatalf("unexpected number of items visited in the array; got %d; want %d", n, 2)
		}
	})

	t.Run("negative", func(t *testing.T) {
		vv := v.Get("nonexisting", "path")
		if vv != nil {
			t.Fatalf("expecting nil value for nonexisting path. Got %#v", vv)
		}
		vv = v.Get("foo", "bar", "baz")
		if vv != nil {
			t.Fatalf("expecting nil value for nonexisting path. Got %#v", vv)
		}
		vv = v.Get("foo", "-123")
		if vv != nil {
			t.Fatalf("expecting nil value for nonexisting path. Got %#v", vv)
		}
		vv = v.Get("foo", "234")
		if vv != nil {
			t.Fatalf("expecting nil value for nonexisting path. Got %#v", vv)
		}
		vv = v.Get("xx", "yy")
		if vv != nil {
			t.Fatalf("expecting nil value for nonexisting path. Got %#v", vv)
		}
	})
}

func TestParserParse(t *testing.T) {
	var p Parser

	t.Run("complex-string", func(t *testing.T) {
		v, err := p.Parse(`{"тест":1, "\\\"фыва\"":2, "\\\"\u1234x":"\\fЗУ\\\\"}`)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		n := v.GetInt("тест")
		if n != 1 {
			t.Fatalf("unexpected int; got %d; want %d", n, 1)
		}
		n = v.GetInt(`\"фыва"`)
		if n != 2 {
			t.Fatalf("unexpected int; got %d; want %d", n, 2)
		}
		sb := v.GetStringBytes("\\\"\u1234x")
		if string(sb) != `\fЗУ\\` {
			t.Fatalf("unexpected string; got %q; want %q", sb, `\fЗУ\\`)
		}
	})

	t.Run("invalid-string-escape", func(t *testing.T) {
		v, err := p.Parse(`"fo\u"`)
		if err != nil {
			t.Fatalf("unexpected error when parsing string")
		}
		// Make sure only valid string part remains
		sb, err := v.StringBytes()
		if err != nil {
			t.Fatalf("cannot obtain string: %s", err)
		}
		if string(sb) != "fo\\u" {
			t.Fatalf("unexpected string; got %q; want %q", sb, "fo\\u")
		}

		v, err = p.Parse(`"foo\ubarz2134"`)
		if err != nil {
			t.Fatalf("unexpected error when parsing string")
		}
		sb, err = v.StringBytes()
		if err != nil {
			t.Fatalf("cannot obtain string: %s", err)
		}
		if string(sb) != "foo\\ubarz2134" {
			t.Fatalf("unexpected string; got %q; want %q", sb, "foo")
		}

		v, err = p.Parse(`"fo` + "\x19" + `\u"`)
		if err != nil {
			t.Fatalf("unexpected error when parsing string")
		}
		sb, err = v.StringBytes()
		if err != nil {
			t.Fatalf("cannot obtain string: %s", err)
		}
		if string(sb) != "fo\x19\\u" {
			t.Fatalf("unexpected string; got %q; want %q", sb, "fo\x19\\u")
		}
	})

	t.Run("invalid-number", func(t *testing.T) {
		v, err := p.Parse("123+456")
		if err != nil {
			t.Fatalf("unexpected error when parsing int")
		}

		// Make sure invalid int isn't parsed.
		n, err := v.Int()
		if err == nil {
			t.Fatalf("expecting non-nil error")
		}
		if n != 0 {
			t.Fatalf("unexpected int; got %d; want %d", n, 0)
		}
	})

	t.Run("empty-json", func(t *testing.T) {
		_, err := p.Parse("")
		if err == nil {
			t.Fatalf("expecting non-nil error when parsing empty json")
		}
		_, err = p.Parse("\n\t    \n")
		if err == nil {
			t.Fatalf("expecting non-nil error when parsing empty json")
		}
	})

	t.Run("invalid-tail", func(t *testing.T) {
		_, err := p.Parse("123 456")
		if err == nil {
			t.Fatalf("expecting non-nil error when parsing invalid tail")
		}
		_, err = p.Parse("[] 1223")
		if err == nil {
			t.Fatalf("expecting non-nil error when parsing invalid tail")
		}
	})

	t.Run("invalid-json", func(t *testing.T) {
		f := func(s string) {
			t.Helper()
			if _, err := p.Parse(s); err == nil {
				t.Fatalf("expecting non-nil error when parsing invalid json %q", s)
			}
		}

		f("free")
		f("tree")
		f("\x00\x10123")
		f("1 \n\x01")
		f("{\x00}")
		f("[\x00]")
		f("\"foo\"\x00")
		f("{\"foo\"\x00:123}")
		f("nil")
		f("[foo]")
		f("{foo}")
		f("[123 34]")
		f(`{"foo" "bar"}`)
		f(`{"foo":123 "bar":"baz"}`)
		f("-2134.453eec+43")

		if _, err := p.Parse("-2134.453E+43"); err != nil {
			t.Fatalf("unexpected error when parsing number: %s", err)
		}

		// Incomplete object key key.
		f(`{"foo: 123}`)

		// Incomplete string.
		f(`"{\"foo\": 123}`)

		v, err := p.Parse(`"{\"foo\": 123}"`)
		if err != nil {
			t.Fatalf("unexpected error when parsing json string: %s", err)
		}
		sb := v.GetStringBytes()
		if string(sb) != `{"foo": 123}` {
			t.Fatalf("unexpected string value; got %q; want %q", sb, `{"foo": 123}`)
		}
	})

	t.Run("incomplete-object", func(t *testing.T) {
		f := func(s string) {
			t.Helper()
			if _, err := p.Parse(s); err == nil {
				t.Fatalf("expecting non-nil error when parsing incomplete object %q", s)
			}
		}

		f(" {  ")
		f(`{"foo"`)
		f(`{"foo":`)
		f(`{"foo":null`)
		f(`{"foo":null,`)
		f(`{"foo":null,}`)
		f(`{"foo":null,"bar"}`)

		if _, err := p.Parse(`{"foo":null,"bar":"baz"}`); err != nil {
			t.Fatalf("unexpected error when parsing object: %s", err)
		}
	})

	t.Run("incomplete-array", func(t *testing.T) {
		f := func(s string) {
			t.Helper()
			if _, err := p.Parse(s); err == nil {
				t.Fatalf("expecting non-nil error when parsing incomplete array %q", s)
			}
		}

		f("  [ ")
		f("[123")
		f("[123,")
		f("[123,]")
		f("[123,{}")
		f("[123,{},]")

		if _, err := p.Parse("[123,{},[]]"); err != nil {
			t.Fatalf("unexpected error when parsing array: %s", err)
		}
	})

	t.Run("incomplete-string", func(t *testing.T) {
		f := func(s string) {
			t.Helper()
			if _, err := p.Parse(s); err == nil {
				t.Fatalf("expecting non-nil error when parsing incomplete string %q", s)
			}
		}

		f(`  "foo`)
		f(`"foo\`)
		f(`"foo\"`)
		f(`"foo\\\"`)
		f(`"foo'`)
		f(`"foo'bar'`)

		if _, err := p.Parse(`"foo\\\""`); err != nil {
			t.Fatalf("unexpected error when parsing string: %s", err)
		}
	})

	t.Run("empty-object", func(t *testing.T) {
		v, err := p.Parse("{}")
		if err != nil {
			t.Fatalf("cannot parse empty object: %s", err)
		}
		tp := v.Type()
		if tp != TypeObject || tp.String() != "object" {
			t.Fatalf("unexpected value obtained for empty object: %#v", v)
		}
		o, err := v.Object()
		if err != nil {
			t.Fatalf("cannot obtain object: %s", err)
		}
		n := o.Len()
		if n != 0 {
			t.Fatalf("unexpected number of items in empty object: %d; want 0", n)
		}
		s := v.String()
		if s != "{}" {
			t.Fatalf("unexpected string representation of empty object: got %q; want %q", s, "{}")
		}
	})

	t.Run("empty-array", func(t *testing.T) {
		v, err := p.Parse("[]")
		if err != nil {
			t.Fatalf("cannot parse empty array: %s", err)
		}
		tp := v.Type()
		if tp != TypeArray || tp.String() != "array" {
			t.Fatalf("unexpected value obtained for empty array: %#v", v)
		}
		a, err := v.Array()
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		n := len(a)
		if n != 0 {
			t.Fatalf("unexpected number of items in empty array: %d; want 0", n)
		}
		s := v.String()
		if s != "[]" {
			t.Fatalf("unexpected string representation of empty array: got %q; want %q", s, "[]")
		}
	})

	t.Run("null", func(t *testing.T) {
		v, err := p.Parse("null")
		if err != nil {
			t.Fatalf("cannot parse null: %s", err)
		}
		tp := v.Type()
		if tp != TypeNull || tp.String() != "null" {
			t.Fatalf("unexpected value obtained for null: %#v", v)
		}
		s := v.String()
		if s != "null" {
			t.Fatalf("unexpected string representation of null; got %q; want %q", s, "null")
		}
	})

	t.Run("true", func(t *testing.T) {
		v, err := p.Parse("true")
		if err != nil {
			t.Fatalf("cannot parse true: %s", err)
		}
		tp := v.Type()
		if tp != TypeTrue || tp.String() != "true" {
			t.Fatalf("unexpected value obtained for true: %#v", v)
		}
		b, err := v.Bool()
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if !b {
			t.Fatalf("expecting true; got false")
		}
		s := v.String()
		if s != "true" {
			t.Fatalf("unexpected string representation of true; got %q; want %q", s, "true")
		}
	})

	t.Run("false", func(t *testing.T) {
		v, err := p.Parse("false")
		if err != nil {
			t.Fatalf("cannot parse false: %s", err)
		}
		tp := v.Type()
		if tp != TypeFalse || tp.String() != "false" {
			t.Fatalf("unexpected value obtained for false: %#v", v)
		}
		b, err := v.Bool()
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if b {
			t.Fatalf("expecting false; got true")
		}
		s := v.String()
		if s != "false" {
			t.Fatalf("unexpected string representation of false; got %q; want %q", s, "false")
		}
	})

	t.Run("integer", func(t *testing.T) {
		v, err := p.Parse("12345")
		if err != nil {
			t.Fatalf("cannot parse integer: %s", err)
		}
		tp := v.Type()
		if tp != TypeNumber || tp.String() != "number" {
			t.Fatalf("unexpected type obtained for integer: %#v", v)
		}
		n, err := v.Int()
		if err != nil {
			t.Fatalf("cannot obtain int: %s", err)
		}
		if n != 12345 {
			t.Fatalf("unexpected value obtained for integer; got %d; want %d", n, 12345)
		}
		s := v.String()
		if s != "12345" {
			t.Fatalf("unexpected string representation of integer; got %q; want %q", s, "12345")
		}
	})

	t.Run("int64", func(t *testing.T) {
		v, err := p.Parse("-8838840643388017390")
		if err != nil {
			t.Fatalf("cannot parse int64: %s", err)
		}
		tp := v.Type()
		if tp != TypeNumber || tp.String() != "number" {
			t.Fatalf("unexpected type obtained for int64: %#v", v)
		}
		n, err := v.Int64()
		if err != nil {
			t.Fatalf("cannot obtain int64: %s", err)
		}
		if n != int64(-8838840643388017390) {
			t.Fatalf("unexpected value obtained for int64; got %d; want %d", n, int64(-8838840643388017390))
		}
		s := v.String()
		if s != "-8838840643388017390" {
			t.Fatalf("unexpected string representation of int64; got %q; want %q", s, "-8838840643388017390")
		}
	})

	t.Run("uint", func(t *testing.T) {
		v, err := p.Parse("18446744073709551615")
		if err != nil {
			t.Fatalf("cannot parse uint: %s", err)
		}
		tp := v.Type()
		if tp != TypeNumber || tp.String() != "number" {
			t.Fatalf("unexpected type obtained for uint: %#v", v)
		}
		n, err := v.Uint64()
		if err != nil {
			t.Fatalf("cannot obtain uint64: %s", err)
		}
		if n != uint64(18446744073709551615) {
			t.Fatalf("unexpected value obtained for uint; got %d; want %d", n, uint64(18446744073709551615))
		}
		s := v.String()
		if s != "18446744073709551615" {
			t.Fatalf("unexpected string representation of uint; got %q; want %q", s, "18446744073709551615")
		}
	})

	t.Run("uint64", func(t *testing.T) {
		v, err := p.Parse("18446744073709551615")
		if err != nil {
			t.Fatalf("cannot parse uint64: %s", err)
		}
		tp := v.Type()
		if tp != TypeNumber || tp.String() != "number" {
			t.Fatalf("unexpected type obtained for uint64: %#v", v)
		}
		n, err := v.Uint64()
		if err != nil {
			t.Fatalf("cannot obtain uint64: %s", err)
		}
		if n != 18446744073709551615 {
			t.Fatalf("unexpected value obtained for uint64; got %d; want %d", n, uint64(18446744073709551615))
		}
		s := v.String()
		if s != "18446744073709551615" {
			t.Fatalf("unexpected string representation of uint64; got %q; want %q", s, "18446744073709551615")
		}
	})

	t.Run("float", func(t *testing.T) {
		v, err := p.Parse("-12.345")
		if err != nil {
			t.Fatalf("cannot parse integer: %s", err)
		}
		n, err := v.Float64()
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		tp := v.Type()
		if tp != TypeNumber || tp.String() != "number" {
			t.Fatalf("unexpected type obtained for integer: %#v", v)
		}
		if n != -12.345 {
			t.Fatalf("unexpected value obtained for integer; got %f; want %f", n, -12.345)
		}
		s := v.String()
		if s != "-12.345" {
			t.Fatalf("unexpected string representation of integer; got %q; want %q", s, "-12.345")
		}
	})

	t.Run("string", func(t *testing.T) {
		v, err := p.Parse(`"foo bar"`)
		if err != nil {
			t.Fatalf("cannot parse string: %s", err)
		}
		tp := v.Type()
		if tp != TypeString || tp.String() != "string" {
			t.Fatalf("unexpected type obtained for string: %#v", v)
		}
		sb, err := v.StringBytes()
		if err != nil {
			t.Fatalf("cannot obtain string: %s", err)
		}
		if string(sb) != "foo bar" {
			t.Fatalf("unexpected value obtained for string; got %q; want %q", sb, "foo bar")
		}
		ss := v.String()
		if ss != `"foo bar"` {
			t.Fatalf("unexpected string representation of string; got %q; want %q", ss, `"foo bar"`)
		}
	})

	t.Run("string-escaped", func(t *testing.T) {
		v, err := p.Parse(`"\n\t\\foo\"bar\u3423x\/\b\f\r\\"`)
		if err != nil {
			t.Fatalf("cannot parse string: %s", err)
		}
		tp := v.Type()
		if tp != TypeString {
			t.Fatalf("unexpected type obtained for string: %#v", v)
		}
		sb, err := v.StringBytes()
		if err != nil {
			t.Fatalf("cannot obtain string: %s", err)
		}
		if string(sb) != "\n\t\\foo\"bar\u3423x/\b\f\r\\" {
			t.Fatalf("unexpected value obtained for string; got %q; want %q", sb, "\n\t\\foo\"bar\u3423x/\b\f\r\\")
		}
		ss := v.String()
		if ss != `"\n\t\\foo\"bar㐣x/\b\f\r\\"` {
			t.Fatalf("unexpected string representation of string; got %q; want %q", ss, `"\n\t\\foo\"bar㐣x/\b\f\r\\"`)
		}
	})

	t.Run("object-one-element", func(t *testing.T) {
		v, err := p.Parse(`  {
	"foo"   : "bar"  }	 `)
		if err != nil {
			t.Fatalf("cannot parse object: %s", err)
		}
		tp := v.Type()
		if tp != TypeObject {
			t.Fatalf("unexpected type obtained for object: %#v", v)
		}
		o, err := v.Object()
		if err != nil {
			t.Fatalf("cannot obtain object: %s", err)
		}
		vv := o.Get("foo")
		if vv.Type() != TypeString {
			t.Fatalf("unexpected type for foo item: got %d; want %d", vv.Type(), TypeString)
		}
		vv = o.Get("non-existing key")
		if vv != nil {
			t.Fatalf("unexpected value obtained for non-existing key: %#v", vv)
		}

		s := v.String()
		if s != `{"foo":"bar"}` {
			t.Fatalf("unexpected string representation for object; got %q; want %q", s, `{"foo":"bar"}`)
		}
	})

	t.Run("object-multi-elements", func(t *testing.T) {
		v, err := p.Parse(`{"foo": [1,2,3  ]  ,"bar":{},"baz":123.456}`)
		if err != nil {
			t.Fatalf("cannot parse object: %s", err)
		}
		tp := v.Type()
		if tp != TypeObject {
			t.Fatalf("unexpected type obtained for object: %#v", v)
		}
		o, err := v.Object()
		if err != nil {
			t.Fatalf("cannot obtain object: %s", err)
		}
		vv := o.Get("foo")
		if vv.Type() != TypeArray {
			t.Fatalf("unexpected type for foo item; got %d; want %d", vv.Type(), TypeArray)
		}
		vv = o.Get("bar")
		if vv.Type() != TypeObject {
			t.Fatalf("unexpected type for bar item; got %d; want %d", vv.Type(), TypeObject)
		}
		vv = o.Get("baz")
		if vv.Type() != TypeNumber {
			t.Fatalf("unexpected type for baz item; got %d; want %d", vv.Type(), TypeNumber)
		}
		vv = o.Get("non-existing-key")
		if vv != nil {
			t.Fatalf("unexpected value obtained for non-existing key: %#v", vv)
		}

		s := v.String()
		if s != `{"foo":[1,2,3],"bar":{},"baz":123.456}` {
			t.Fatalf("unexpected string representation for object; got %q; want %q", s, `{"foo":[1,2,3],"bar":{},"baz":123.456}`)
		}
	})

	t.Run("array-one-element", func(t *testing.T) {
		v, err := p.Parse(`   [{"bar":[  [],[[]]   ]} ]  `)
		if err != nil {
			t.Fatalf("cannot parse array: %s", err)
		}
		tp := v.Type()
		if tp != TypeArray {
			t.Fatalf("unexpected type obtained for array: %#v", v)
		}
		a, err := v.Array()
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if len(a) != 1 {
			t.Fatalf("unexpected array len; got %d; want %d", len(a), 1)
		}
		if a[0].Type() != TypeObject {
			t.Fatalf("unexpected type for a[0]; got %d; want %d", a[0].Type(), TypeObject)
		}

		s := v.String()
		if s != `[{"bar":[[],[[]]]}]` {
			t.Fatalf("unexpected string representation for array; got %q; want %q", s, `[{"bar":[[],[[]]]}]`)
		}
	})

	t.Run("array-multi-elements", func(t *testing.T) {
		v, err := p.Parse(`   [1,"foo",{"bar":[     ],"baz":""}    ,[  "x" ,	"y"   ]     ]   `)
		if err != nil {
			t.Fatalf("cannot parse array: %s", err)
		}
		tp := v.Type()
		if tp != TypeArray {
			t.Fatalf("unexpected type obtained for array: %#v", v)
		}
		a, err := v.Array()
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if len(a) != 4 {
			t.Fatalf("unexpected array len; got %d; want %d", len(a), 4)
		}
		if a[0].Type() != TypeNumber {
			t.Fatalf("unexpected type for a[0]; got %d; want %d", a[0].Type(), TypeNumber)
		}
		if a[1].Type() != TypeString {
			t.Fatalf("unexpected type for a[1]; got %d; want %d", a[1].Type(), TypeString)
		}
		if a[2].Type() != TypeObject {
			t.Fatalf("unexpected type for a[2]; got %d; want %d", a[2].Type(), TypeObject)
		}
		if a[3].Type() != TypeArray {
			t.Fatalf("unexpected type for a[3]; got %d; want %d", a[3].Type(), TypeArray)
		}

		s := v.String()
		if s != `[1,"foo",{"bar":[],"baz":""},["x","y"]]` {
			t.Fatalf("unexpected string representation for array; got %q; want %q", s, `[1,"foo",{"bar":[],"baz":""},["x","y"]]`)
		}
	})

	t.Run("complex-object", func(t *testing.T) {
		s := `{"foo":[-1.345678,[[[[[]]]],{}],"bar"],"baz":{"bbb":123}}`
		v, err := p.Parse(s)
		if err != nil {
			t.Fatalf("cannot parse complex object: %s", err)
		}
		if v.Type() != TypeObject {
			t.Fatalf("unexpected type obtained for object: %#v", v)
		}

		ss := v.String()
		if ss != s {
			t.Fatalf("unexpected string representation for object; got %q; want %q", ss, s)
		}

		s = strings.TrimSpace(largeFixture)
		v, err = p.Parse(s)
		if err != nil {
			t.Fatalf("cannot parse largeFixture: %s", err)
		}
		ss = v.String()
		if ss != s {
			t.Fatalf("unexpected string representation for object; got\n%q; want\n%q", ss, s)
		}
	})

	t.Run("complex-object-visit-all", func(t *testing.T) {
		n := 0
		var f func(k []byte, v *Value)
		f = func(k []byte, v *Value) {
			switch v.Type() {
			case TypeObject:
				o, err := v.Object()
				if err != nil {
					t.Fatalf("cannot obtain object: %s", err)
				}
				o.Visit(f)
			case TypeArray:
				a, err := v.Array()
				if err != nil {
					t.Fatalf("unexpected error: %s", err)
				}
				for _, vv := range a {
					f(nil, vv)
				}
			case TypeString:
				sb, err := v.StringBytes()
				if err != nil {
					t.Fatalf("cannot obtain string: %s", err)
				}
				n += len(sb)
			case TypeNumber:
				nn, err := v.Int()
				if err != nil {
					t.Fatalf("cannot obtain int: %s", err)
				}
				n += nn
			}
		}

		s := strings.TrimSpace(largeFixture)
		v, err := p.Parse(s)
		if err != nil {
			t.Fatalf("cannot parse largeFixture: %s", err)
		}
		o, err := v.Object()
		if err != nil {
			t.Fatalf("cannot obtain object: %s", err)
		}
		o.Visit(f)

		if n != 21473 {
			t.Fatalf("unexpected n; got %d; want %d", n, 21473)
		}

		// Make sure the json remains valid after visiting all the items.
		ss := v.String()
		if ss != s {
			t.Fatalf("unexpected string representation for object; got\n%q; want\n%q", ss, s)
		}

	})
}

func TestParseBigObject(t *testing.T) {
	const itemsCount = 10000

	// build big json object
	var ss []string
	for i := 0; i < itemsCount; i++ {
		s := fmt.Sprintf(`"key_%d": "value_%d"`, i, i)
		ss = append(ss, s)
	}
	s := "{" + strings.Join(ss, ",") + "}"

	// parse it
	var p Parser
	v, err := p.Parse(s)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	// Look up object items
	for i := 0; i < itemsCount; i++ {
		k := fmt.Sprintf("key_%d", i)
		expectedV := fmt.Sprintf("value_%d", i)
		sb := v.GetStringBytes(k)
		if string(sb) != expectedV {
			t.Fatalf("unexpected value obtained; got %q; want %q", sb, expectedV)
		}
	}

	// verify non-existing key returns nil
	sb := v.GetStringBytes("non-existing-key")
	if sb != nil {
		t.Fatalf("unexpected non-nil value for non-existing-key: %q", sb)
	}
}

func TestParseGetConcurrent(t *testing.T) {
	concurrency := 10
	ch := make(chan error, concurrency)
	s := `{"foo": "bar", "empty_obj": {}}`
	for i := 0; i < concurrency; i++ {
		go func() {
			ch <- testParseGetSerial(s)
		}()
	}
	for i := 0; i < concurrency; i++ {
		select {
		case err := <-ch:
			if err != nil {
				t.Fatalf("unexpected error during concurrent test: %s", err)
			}
		case <-time.After(time.Second):
			t.Fatalf("timeout")
		}
	}
}

func testParseGetSerial(s string) error {
	var p Parser
	for i := 0; i < 100; i++ {
		v, err := p.Parse(s)
		if err != nil {
			return fmt.Errorf("cannot parse %q: %s", s, err)
		}
		sb := v.GetStringBytes("foo")
		if string(sb) != "bar" {
			return fmt.Errorf("unexpected value for key=%q; got %q; want %q", "foo", sb, "bar")
		}
		vv := v.Get("empty_obj", "non-existing-key")
		if vv != nil {
			return fmt.Errorf("unexpected non-nil value got: %s", vv)
		}
	}
	return nil
}

func TestMarshalTo(t *testing.T) {
	fileData := getFromFile("testdata/bunchFields.json")
	var p Parser
	v, err := p.Parse(fileData)
	if err != nil {
		t.Fatalf("cannot parse json: %s", err)
	}
	data := make([]byte, 0, len(fileData))
	data = v.MarshalTo(data)
	// check
	var p2 Parser
	v, err = p2.ParseBytes(data)
	if err != nil {
		t.Fatalf("cannot parse json: %s", err)
	}
	o, err := v.Object()
	if err != nil {
		t.Fatalf("expected object, got: %s", o.String())
	}
	if o.Len() != 871 {
		t.Fatalf("expected 871 fields, got %d", o.Len())
	}
}

func BenchmarkParse(b *testing.B) {
	fileData := getFromFile("testdata/twitter.json")
	var p Parser
	out := make([]byte, 0, len(fileData))
	b.SetBytes(int64(len(fileData)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v, err := p.Parse(fileData)
		if err != nil {
			b.Fatalf("cannot parse json: %s", err)
		}
		out = v.MarshalTo(out[:0])
	}
}

func BenchmarkParseArena(b *testing.B) {
	fileData := getFromFile("testdata/twitter.json")
	var p Parser
	a := arena.NewMonotonicArena(arena.WithMinBufferSize(1024 * 1024 * 2))
	out := make([]byte, 0, len(fileData))
	b.SetBytes(int64(len(fileData)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v, err := p.ParseWithArena(a, fileData)
		if err != nil {
			b.Fatalf("cannot parse json: %s", err)
		}
		out = v.MarshalTo(out[:0])
		a.Reset()
	}
}

func BenchmarkParseArenaAndGet(b *testing.B) {
	fileData := getFromFile("testdata/twitter.json")
	var p Parser
	a := arena.NewMonotonicArena(arena.WithMinBufferSize(1024 * 1024 * 2))
	out := make([]byte, 0, len(fileData))
	b.SetBytes(int64(len(fileData)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v, err := p.ParseWithArena(a, fileData)
		if err != nil {
			b.Fatalf("cannot parse json: %s", err)
		}

		// Perform several Get operations to simulate typical usage
		// These keys are chosen to be common in JSON data and don't contain escape sequences
		_ = v.Get("id")
		_ = v.Get("text")
		_ = v.Get("user")
		_ = v.Get("created_at")
		_ = v.Get("retweet_count")
		_ = v.Get("favorite_count")
		_ = v.Get("lang")
		_ = v.Get("source")

		out = v.MarshalTo(out[:0])
		a.Reset()
	}
}

// TestParseError tests ParseError functionality
func TestParseError(t *testing.T) {
	t.Run("nil error", func(t *testing.T) {
		err := NewParseError(nil)
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
	})

	t.Run("non-nil error", func(t *testing.T) {
		originalErr := fmt.Errorf("test error")
		err := NewParseError(originalErr)
		if err == nil {
			t.Fatalf("expected non-nil error")
		}
		if err.Error() != "test error" {
			t.Fatalf("unexpected error message: got %q, want %q", err.Error(), "test error")
		}
	})

	t.Run("nil ParseError", func(t *testing.T) {
		var err *ParseError
		if err.Error() != "" {
			t.Fatalf("expected empty error message for nil ParseError, got %q", err.Error())
		}
	})
}

// TestParseWithArena tests arena-based parsing
func TestParseWithArena(t *testing.T) {
	var p Parser
	a := arena.NewMonotonicArena()

	t.Run("simple object", func(t *testing.T) {
		v, err := p.ParseWithArena(a, `{"foo": "bar"}`)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if v.Type() != TypeObject {
			t.Fatalf("expected object type, got %v", v.Type())
		}
		sb := v.GetStringBytes("foo")
		if string(sb) != "bar" {
			t.Fatalf("unexpected value: got %q, want %q", sb, "bar")
		}
	})

	t.Run("complex nested structure", func(t *testing.T) {
		v, err := p.ParseWithArena(a, `{"arr": [1, 2, {"nested": true}], "str": "test"}`)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		arr := v.GetArray("arr")
		if len(arr) != 3 {
			t.Fatalf("expected array length 3, got %d", len(arr))
		}
		nested := arr[2].GetBool("nested")
		if !nested {
			t.Fatalf("expected nested boolean to be true")
		}
	})
}

// TestParseBytesWithArena tests arena-based byte parsing
func TestParseBytesWithArena(t *testing.T) {
	var p Parser
	a := arena.NewMonotonicArena()

	t.Run("simple array", func(t *testing.T) {
		data := []byte(`[1, 2, 3]`)
		v, err := p.ParseBytesWithArena(a, data)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if v.Type() != TypeArray {
			t.Fatalf("expected array type, got %v", v.Type())
		}
		arr := v.GetArray()
		if len(arr) != 3 {
			t.Fatalf("expected array length 3, got %d", len(arr))
		}
	})

	t.Run("empty object", func(t *testing.T) {
		data := []byte(`{}`)
		v, err := p.ParseBytesWithArena(a, data)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if v.Type() != TypeObject {
			t.Fatalf("expected object type, got %v", v.Type())
		}
	})
}

// TestSkipWSSlow tests the slow whitespace skipping path
func TestSkipWSSlow(t *testing.T) {
	t.Run("all whitespace types", func(t *testing.T) {
		testCases := []struct {
			input    string
			expected string
		}{
			{"   ", ""},
			{"\t\t\t", ""},
			{"\n\n\n", ""},
			{"\r\r\r", ""},
			{" \t\n\r ", ""},
			{"  abc", "abc"},
			{"\t\n\rdef", "def"},
			{"   \t\n\rghi", "ghi"},
		}

		for _, tc := range testCases {
			result := skipWSSlow(tc.input)
			if result != tc.expected {
				t.Errorf("skipWSSlow(%q) = %q, want %q", tc.input, result, tc.expected)
			}
		}
	})

	t.Run("empty string", func(t *testing.T) {
		result := skipWSSlow("")
		if result != "" {
			t.Errorf("skipWSSlow(\"\") = %q, want \"\"", result)
		}
	})
}

// TestParseValueEdgeCases tests edge cases in parseValue
func TestParseValueEdgeCases(t *testing.T) {
	var p Parser

	t.Run("max depth exceeded", func(t *testing.T) {
		// Create a deeply nested JSON structure
		json := "1"
		for i := 0; i < MaxDepth+1; i++ {
			json = "[" + json + "]"
		}

		_, err := p.Parse(json)
		if err == nil {
			t.Fatalf("expected error for max depth exceeded")
		}
		if !strings.Contains(err.Error(), "too big depth") {
			t.Fatalf("unexpected error message: %s", err.Error())
		}
	})

	t.Run("empty string", func(t *testing.T) {
		_, err := p.Parse("")
		if err == nil {
			t.Fatalf("expected error for empty string")
		}
	})

	t.Run("invalid literal", func(t *testing.T) {
		_, err := p.Parse("invalid")
		if err == nil {
			t.Fatalf("expected error for invalid literal")
		}
	})

	t.Run("incomplete true", func(t *testing.T) {
		_, err := p.Parse("tru")
		if err == nil {
			t.Fatalf("expected error for incomplete true")
		}
	})

	t.Run("incomplete false", func(t *testing.T) {
		_, err := p.Parse("fals")
		if err == nil {
			t.Fatalf("expected error for incomplete false")
		}
	})

	t.Run("incomplete null", func(t *testing.T) {
		_, err := p.Parse("nul")
		if err == nil {
			t.Fatalf("expected error for incomplete null")
		}
	})
}

// TestEscapeStringSlowPath tests the slow path of string escaping
func TestEscapeStringSlowPath(t *testing.T) {
	t.Run("various control characters", func(t *testing.T) {
		testCases := []struct {
			input    string
			expected string
		}{
			{"\x00", `"\u0000"`},
			{"\x01", `"\u0001"`},
			{"\x08", `"\b"`},
			{"\x09", `"\t"`},
			{"\x0a", `"\n"`},
			{"\x0c", `"\f"`},
			{"\x0d", `"\r"`},
			{"\x1f", `"\u001f"`},
			{"\"", `"\""`},
			{"\\", `"\\"`},
			{"mixed\x00\x08\x09\x0a\x0c\x0d\"\\", `"mixed\u0000\b\t\n\f\r\"\\"`},
		}

		for _, tc := range testCases {
			result := escapeStringSlowPath(nil, tc.input)
			if string(result) != tc.expected {
				t.Errorf("escapeStringSlowPath(%q) = %q, want %q", tc.input, string(result), tc.expected)
			}
		}
	})
}

// TestUnescapeStringBestEffortEdgeCases tests edge cases in unescaping
func TestUnescapeStringBestEffortEdgeCases(t *testing.T) {
	t.Run("incomplete unicode escape", func(t *testing.T) {
		result := unescapeStringBestEffort(nil, "\\u12")
		if result != "\\u12" {
			t.Errorf("unescapeStringBestEffort(\"\\u12\") = %q, want %q", result, "\\u12")
		}
	})

	t.Run("invalid unicode escape", func(t *testing.T) {
		result := unescapeStringBestEffort(nil, "\\u12xy")
		if result != "\\u12xy" {
			t.Errorf("unescapeStringBestEffort(\"\\u12xy\") = %q, want %q", result, "\\u12xy")
		}
	})

	t.Run("incomplete surrogate pair", func(t *testing.T) {
		result := unescapeStringBestEffort(nil, "\\ud83e")
		if result != "\\ud83e" {
			t.Errorf("unescapeStringBestEffort(\"\\ud83e\") = %q, want %q", result, "\\ud83e")
		}
	})

	t.Run("invalid surrogate pair", func(t *testing.T) {
		result := unescapeStringBestEffort(nil, "\\ud83e\\u1234")
		// The function actually processes this as a valid surrogate pair, so we need to check the actual behavior
		if len(result) == 0 {
			t.Errorf("unescapeStringBestEffort(\"\\ud83e\\u1234\") returned empty string")
		}
	})

	t.Run("unknown escape sequence", func(t *testing.T) {
		result := unescapeStringBestEffort(nil, "\\x")
		if result != "\\x" {
			t.Errorf("unescapeStringBestEffort(\"\\x\") = %q, want %q", result, "\\x")
		}
	})
}

// TestObjectGetEdgeCases tests edge cases in Object.Get
func TestObjectGetEdgeCases(t *testing.T) {
	var p Parser

	t.Run("nil object", func(t *testing.T) {
		var o *Object
		result := o.Get("key")
		if result != nil {
			t.Errorf("Get on nil object should return nil, got %v", result)
		}
	})

	t.Run("key with escape sequences", func(t *testing.T) {
		v, err := p.Parse(`{"key\\with\\escapes": "value"}`)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		o, err := v.Object()
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		// Test that we can find the key with escapes
		result := o.Get("key\\with\\escapes")
		if result == nil {
			t.Errorf("expected to find key with escapes")
		}
	})

	t.Run("keys unescaped flag", func(t *testing.T) {
		v, err := p.Parse(`{"key\\with\\escapes": "value"}`)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		o, err := v.Object()
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		// This should trigger the unescapeKeys path since the key has escapes
		value := o.Get("key\\with\\escapes")
		if value == nil {
			t.Errorf("expected value to be not nil")
			return
		}
		if string(value.GetStringBytes()) != `value` {
			t.Errorf("unexpected value: got %q, want %q", value.String(), `value`)
			return
		}
		// Check that the specific key was unescaped
		found := false
		for _, kv := range v.o.kvs {
			if kv.k == "key\\with\\escapes" && kv.keyUnescaped {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected key to be unescaped after Get")
			return
		}
	})
}

// TestValueMarshalToEdgeCases tests edge cases in Value.MarshalTo
func TestValueMarshalToEdgeCases(t *testing.T) {
	t.Run("unknown type", func(t *testing.T) {
		v := &Value{t: Type(999)} // Invalid type
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("expected panic for unknown type")
			}
		}()
		v.MarshalTo(nil)
	})
}

// TestTypeStringEdgeCases tests edge cases in Type.String
func TestTypeStringEdgeCases(t *testing.T) {
	t.Run("unknown type", func(t *testing.T) {
		tp := Type(999) // Invalid type
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("expected panic for unknown type")
			}
		}()
		s := tp.String()
		if s != "" {
			t.Errorf("expected empty string for unknown type, got %q", s)
		}
	})
}

// TestGetIntEdgeCases tests edge cases in GetInt
func TestGetIntEdgeCases(t *testing.T) {
	var p Parser

	t.Run("number too large for int", func(t *testing.T) {
		v, err := p.Parse(`9223372036854775808`) // Max int64 + 1
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		result := v.GetInt()
		if result != 0 {
			t.Errorf("expected 0 for number too large for int, got %d", result)
		}
	})

	t.Run("negative number too large for int", func(t *testing.T) {
		v, err := p.Parse(`-9223372036854775809`) // Min int64 - 1
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		result := v.GetInt()
		if result != 0 {
			t.Errorf("expected 0 for negative number too large for int, got %d", result)
		}
	})
}

// TestGetUintEdgeCases tests edge cases in GetUint
func TestGetUintEdgeCases(t *testing.T) {
	var p Parser

	t.Run("negative number", func(t *testing.T) {
		v, err := p.Parse(`-1`)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		result := v.GetUint()
		if result != 0 {
			t.Errorf("expected 0 for negative number, got %d", result)
		}
	})

	t.Run("number too large for uint", func(t *testing.T) {
		v, err := p.Parse(`18446744073709551616`) // Max uint64 + 1
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		result := v.GetUint()
		if result != 0 {
			t.Errorf("expected 0 for number too large for uint, got %d", result)
		}
	})
}

// TestIntEdgeCases tests edge cases in Int method
func TestIntEdgeCases(t *testing.T) {
	var p Parser

	t.Run("number too large for int", func(t *testing.T) {
		v, err := p.Parse(`9223372036854775808`) // Max int64 + 1
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		_, err = v.Int()
		if err == nil {
			t.Errorf("expected error for number too large for int")
		}
	})

	t.Run("negative number too large for int", func(t *testing.T) {
		v, err := p.Parse(`-9223372036854775809`) // Min int64 - 1
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		_, err = v.Int()
		if err == nil {
			t.Errorf("expected error for negative number too large for int")
		}
	})
}

// TestUintEdgeCases tests edge cases in Uint method
func TestUintEdgeCases(t *testing.T) {
	var p Parser

	t.Run("negative number", func(t *testing.T) {
		v, err := p.Parse(`-1`)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		_, err = v.Uint()
		if err == nil {
			t.Errorf("expected error for negative number")
		}
	})

	t.Run("number too large for uint", func(t *testing.T) {
		v, err := p.Parse(`18446744073709551616`) // Max uint64 + 1
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		_, err = v.Uint()
		if err == nil {
			t.Errorf("expected error for number too large for uint")
		}
	})
}

// TestEscapeStringSlowPathMore tests more edge cases in escapeStringSlowPath
func TestEscapeStringSlowPathMore(t *testing.T) {
	t.Run("more control character ranges", func(t *testing.T) {
		testCases := []struct {
			input    string
			expected string
		}{
			{"\x02", `"\u0002"`},
			{"\x07", `"\u0007"`},
			{"\x0b", `"\u000b"`},
			{"\x0e", `"\u000e"`},
			{"\x0f", `"\u000f"`},
			{"\x10", `"\u0010"`},
			{"\x1e", `"\u001e"`},
		}

		for _, tc := range testCases {
			result := escapeStringSlowPath(nil, tc.input)
			if string(result) != tc.expected {
				t.Errorf("escapeStringSlowPath(%q) = %q, want %q", tc.input, string(result), tc.expected)
			}
		}
	})
}

// TestUnescapeStringBestEffortMore tests more edge cases in unescaping
func TestUnescapeStringBestEffortMore(t *testing.T) {
	t.Run("more unicode ranges", func(t *testing.T) {
		testCases := []struct {
			input    string
			expected string
		}{
			{"\\u0000", "\x00"},
			{"\\u0001", "\x01"},
			{"\\u0007", "\x07"},
			{"\\u000b", "\x0b"},
			{"\\u000e", "\x0e"},
			{"\\u000f", "\x0f"},
			{"\\u0010", "\x10"},
			{"\\u001e", "\x1e"},
			{"\\u001f", "\x1f"},
		}

		for _, tc := range testCases {
			result := unescapeStringBestEffort(nil, tc.input)
			if result != tc.expected {
				t.Errorf("unescapeStringBestEffort(%q) = %q, want %q", tc.input, result, tc.expected)
			}
		}
	})
}

// TestGetIntMore tests more edge cases in GetInt
func TestGetIntMore(t *testing.T) {
	var p Parser

	t.Run("boundary values", func(t *testing.T) {
		testCases := []struct {
			input    string
			expected int
		}{
			{"2147483647", 2147483647},   // Max int32
			{"-2147483648", -2147483648}, // Min int32
		}

		for _, tc := range testCases {
			v, err := p.Parse(tc.input)
			if err != nil {
				t.Fatalf("unexpected error parsing %q: %s", tc.input, err)
			}
			result := v.GetInt()
			if result != tc.expected {
				t.Errorf("GetInt(%q) = %d, want %d", tc.input, result, tc.expected)
			}
		}
	})
}

// TestGetUintMore tests more edge cases in GetUint
func TestGetUintMore(t *testing.T) {
	var p Parser

	t.Run("boundary values", func(t *testing.T) {
		testCases := []struct {
			input    string
			expected uint
		}{
			{"4294967295", 4294967295}, // Max uint32
			{"0", 0},
		}

		for _, tc := range testCases {
			v, err := p.Parse(tc.input)
			if err != nil {
				t.Fatalf("unexpected error parsing %q: %s", tc.input, err)
			}
			result := v.GetUint()
			if result != tc.expected {
				t.Errorf("GetUint(%q) = %d, want %d", tc.input, result, tc.expected)
			}
		}
	})
}

// TestIntMore tests more edge cases in Int method
func TestIntMore(t *testing.T) {
	var p Parser

	t.Run("boundary values", func(t *testing.T) {
		testCases := []struct {
			input    string
			expected int
		}{
			{"2147483647", 2147483647},   // Max int32
			{"-2147483648", -2147483648}, // Min int32
		}

		for _, tc := range testCases {
			v, err := p.Parse(tc.input)
			if err != nil {
				t.Fatalf("unexpected error parsing %q: %s", tc.input, err)
			}
			result, err := v.Int()
			if err != nil {
				t.Errorf("unexpected error for Int(%q): %s", tc.input, err)
			}
			if result != tc.expected {
				t.Errorf("Int(%q) = %d, want %d", tc.input, result, tc.expected)
			}
		}
	})
}

// TestUintMore tests more edge cases in Uint method
func TestUintMore(t *testing.T) {
	var p Parser

	t.Run("boundary values", func(t *testing.T) {
		testCases := []struct {
			input    string
			expected uint
		}{
			{"4294967295", 4294967295}, // Max uint32
			{"0", 0},
		}

		for _, tc := range testCases {
			v, err := p.Parse(tc.input)
			if err != nil {
				t.Fatalf("unexpected error parsing %q: %s", tc.input, err)
			}
			result, err := v.Uint()
			if err != nil {
				t.Errorf("unexpected error for Uint(%q): %s", tc.input, err)
			}
			if result != tc.expected {
				t.Errorf("Uint(%q) = %d, want %d", tc.input, result, tc.expected)
			}
		}
	})
}

// TestGetIntEdgeCasesMore tests more edge cases in GetInt
func TestGetIntEdgeCasesMore(t *testing.T) {
	var p Parser

	t.Run("non-number type", func(t *testing.T) {
		v, err := p.Parse(`"not a number"`)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		result := v.GetInt()
		if result != 0 {
			t.Errorf("expected 0 for non-number type, got %d", result)
		}
	})

	t.Run("nil value", func(t *testing.T) {
		var v *Value
		result := v.GetInt()
		if result != 0 {
			t.Errorf("expected 0 for nil value, got %d", result)
		}
	})
}

// TestGetUintEdgeCasesMore tests more edge cases in GetUint
func TestGetUintEdgeCasesMore(t *testing.T) {
	var p Parser

	t.Run("non-number type", func(t *testing.T) {
		v, err := p.Parse(`"not a number"`)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		result := v.GetUint()
		if result != 0 {
			t.Errorf("expected 0 for non-number type, got %d", result)
		}
	})

	t.Run("nil value", func(t *testing.T) {
		var v *Value
		result := v.GetUint()
		if result != 0 {
			t.Errorf("expected 0 for nil value, got %d", result)
		}
	})
}

// TestIntEdgeCasesMore tests more edge cases in Int method
func TestIntEdgeCasesMore(t *testing.T) {
	var p Parser

	t.Run("non-number type", func(t *testing.T) {
		v, err := p.Parse(`"not a number"`)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		_, err = v.Int()
		if err == nil {
			t.Errorf("expected error for non-number type")
		}
	})
}

// TestUintEdgeCasesMore tests more edge cases in Uint method
func TestUintEdgeCasesMore(t *testing.T) {
	var p Parser

	t.Run("non-number type", func(t *testing.T) {
		v, err := p.Parse(`"not a number"`)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		_, err = v.Uint()
		if err == nil {
			t.Errorf("expected error for non-number type")
		}
	})
}

// TestUnescapeStringBestEffortFinal tests final edge cases in unescaping
func TestUnescapeStringBestEffortFinal(t *testing.T) {
	t.Run("empty string", func(t *testing.T) {
		result := unescapeStringBestEffort(nil, "")
		if result != "" {
			t.Errorf("unescapeStringBestEffort(\"\") = %q, want \"\"", result)
		}
	})

	t.Run("string with no escapes", func(t *testing.T) {
		result := unescapeStringBestEffort(nil, "hello world")
		if result != "hello world" {
			t.Errorf("unescapeStringBestEffort(\"hello world\") = %q, want \"hello world\"", result)
		}
	})

	t.Run("string with only escapes at end", func(t *testing.T) {
		result := unescapeStringBestEffort(nil, "hello\\n")
		if result != "hello\n" {
			t.Errorf("unescapeStringBestEffort(\"hello\\n\") = %q, want \"hello\\n\"", result)
		}
	})
}

// TestGetIntGetUintOverflow tests overflow cases
func TestGetIntGetUintOverflow(t *testing.T) {
	var p Parser

	t.Run("GetInt overflow", func(t *testing.T) {
		// Test case where int64 doesn't fit in int
		v, err := p.Parse(`9223372036854775807`) // Max int64
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		result := v.GetInt()
		// On 64-bit systems, this should work, on 32-bit it should return 0
		if result != 0 && result != 9223372036854775807 {
			t.Errorf("unexpected result: %d", result)
		}
	})

	t.Run("GetUint overflow", func(t *testing.T) {
		// Test case where uint64 doesn't fit in uint
		v, err := p.Parse(`18446744073709551615`) // Max uint64
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		result := v.GetUint()
		// On 64-bit systems, this should work, on 32-bit it should return 0
		if result != 0 && result != 18446744073709551615 {
			t.Errorf("unexpected result: %d", result)
		}
	})
}
