package astjson

import (
	"encoding/json"
	"strconv"

	"github.com/bytedance/sonic"
)

// sonicDecoderCfg uses sonic defaults but with UseNumber to avoid lossy
// float64 conversion. astjson stores numbers as raw strings internally, so
// json.Number is the natural intermediate representation.
var sonicDecoderCfg = sonic.Config{
	UseNumber: true,
}.Froze()

// ParseWithSonic parses JSON using sonic (standard Unmarshal into interface{})
// and converts the decoded tree into an astjson Value.
//
// This is intended as a benchmark baseline: it exercises the typical way
// applications integrate sonic, then walks the decoded interface{} tree to
// produce an astjson AST.
func ParseWithSonic(data []byte) (*Value, error) {
	var v interface{}
	if err := sonic.Unmarshal(data, &v); err != nil {
		return nil, err
	}
	return interfaceToValue(v), nil
}

// ParseWithSonicUseNumber is the same as ParseWithSonic but configures sonic
// to decode JSON numbers into json.Number (string) instead of float64. This
// preserves full precision and matches how astjson represents numbers
// internally, removing the strconv.FormatFloat round-trip.
func ParseWithSonicUseNumber(data []byte) (*Value, error) {
	var v interface{}
	if err := sonicDecoderCfg.Unmarshal(data, &v); err != nil {
		return nil, err
	}
	return interfaceToValue(v), nil
}

// interfaceToValue converts a Go value produced by sonic.Unmarshal (or
// encoding/json) into an astjson Value tree. All resulting values are
// heap-allocated (arena == nil), mirroring the default Parser.Parse mode.
func interfaceToValue(src interface{}) *Value {
	switch x := src.(type) {
	case nil:
		return valueNull
	case bool:
		if x {
			return valueTrue
		}
		return valueFalse
	case string:
		return StringValue(nil, x)
	case json.Number:
		return NumberValue(nil, string(x))
	case float64:
		// Fallback path when numbers are decoded as float64. Round-trip
		// via strconv to fit astjson's string-backed number representation.
		return NumberValue(nil, strconv.FormatFloat(x, 'g', -1, 64))
	case map[string]interface{}:
		obj := ObjectValue(nil)
		for k, v := range x {
			obj.Set(nil, k, interfaceToValue(v))
		}
		return obj
	case []interface{}:
		arr := ArrayValue(nil)
		for i, v := range x {
			arr.SetArrayItem(nil, i, interfaceToValue(v))
		}
		return arr
	default:
		return valueNull
	}
}
