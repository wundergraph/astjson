package astjson

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/wundergraph/astjson/fastfloat"
	"github.com/wundergraph/go-arena"
)

// ParseError wraps a JSON parsing error.
type ParseError struct {
	Err error
}

// Error returns the error message. Returns an empty string if p is nil.
func (p *ParseError) Error() string {
	if p == nil {
		return ""
	}
	return p.Err.Error()
}

// NewParseError wraps err in a ParseError. Returns nil if err is nil.
func NewParseError(err error) *ParseError {
	if err == nil {
		return nil
	}

	return &ParseError{Err: err}
}

// Parser parses JSON.
//
// Parser may be re-used for subsequent parsing.
//
// Parser supports two allocation modes: heap mode (Parse, ParseBytes) where
// all values are heap-allocated and GC-managed, and arena mode
// (ParseWithArena, ParseBytesWithArena) where all values and their backing
// data are allocated on a caller-provided arena. See the package
// documentation for details on GC safety.
//
// Parser cannot be used from concurrent goroutines.
// Use per-goroutine parsers or ParserPool instead.
type Parser struct {
	arenaScratch *arenaPlanScratch
}

func (p *Parser) ensureArenaScratch() *arenaPlanScratch {
	if p == nil {
		return nil
	}
	if p.arenaScratch == nil {
		p.arenaScratch = &arenaPlanScratch{}
	}
	return p.arenaScratch
}

// Parse parses s containing JSON.
//
// The returned value is valid until the next call to Parse*.
//
// Use Scanner if a stream of JSON values must be parsed.
func (p *Parser) Parse(s string) (*Value, error) {
	return p.parse(nil, s)
}

// ParseWithArena parses s containing JSON, allocating all values on the arena.
//
// The input string s is copied onto the arena before parsing, so the caller
// may drop references to s immediately after this call returns. All parsed
// Values, their string data, object keys, and array backing slices live
// entirely in arena memory, making the result independent of the GC.
//
// The returned value is valid for the lifetime of the arena.
//
// When a is nil, behaves identically to Parse (heap allocation).
func (p *Parser) ParseWithArena(a arena.Arena, s string) (*Value, error) {
	if a != nil {
		s = arenaString(a, s)
		return parseArenaTwoPass(p, a, s)
	}
	return p.parse(a, s)
}

// ParseBytes parses b containing JSON.
//
// The returned Value is valid until the next call to Parse*.
//
// Use Scanner if a stream of JSON values must be parsed.
func (p *Parser) ParseBytes(b []byte) (*Value, error) {
	return p.parse(nil, b2s(b))
}

// ParseBytesWithArena parses b containing JSON, allocating all values on the
// arena.
//
// The input bytes b are copied onto the arena before parsing, so the caller
// may reuse or discard b immediately after this call returns. All parsed
// Values, their string data, object keys, and array backing slices live
// entirely in arena memory, making the result independent of the GC.
//
// The returned value is valid for the lifetime of the arena.
//
// When a is nil, behaves identically to ParseBytes (heap allocation). In that
// case the caller must not modify b while the returned Value is in use, as it
// may reference b's underlying memory via zero-copy conversion.
func (p *Parser) ParseBytesWithArena(a arena.Arena, b []byte) (*Value, error) {
	if a != nil {
		ab := arena.AllocateSlice[byte](a, len(b), len(b))
		copy(ab, b)
		return parseArenaTwoPass(p, a, b2s(ab))
	}
	return p.parse(nil, b2s(b))
}

func (p *Parser) parse(a arena.Arena, s string) (*Value, error) {
	s = skipWS(s)

	v, tail, err := parseValue(a, s, 0)
	if err != nil {
		return nil, NewParseError(fmt.Errorf("cannot parse JSON: %s; unparsed tail: %q", err, startEndString(tail)))
	}
	tail = skipWS(tail)
	if len(tail) > 0 {
		return nil, NewParseError(fmt.Errorf("unexpected tail: %q", startEndString(tail)))
	}
	return v, nil
}

func skipWS(s string) string {
	if len(s) == 0 || s[0] > 0x20 {
		// Fast path - most common case
		return s
	}
	return skipWSSlow(s)
}

func skipWSSlow(s string) string {
	if len(s) == 0 {
		return s
	}

	// Branch prediction optimization: check most common whitespace first
	// Space (0x20) is most common, then newline, tab, carriage return
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c != 0x20 { // Most common whitespace
			if c != 0x0A && c != 0x09 && c != 0x0D {
				return s[i:]
			}
		}
	}
	return ""
}

// kv represents a key-value pair in JSON objects.
// Cache-friendly layout: hot data first
type kv struct {
	keyUnescaped   bool // tracks if this specific key has been unescaped
	keyNeedsEscape bool // keyUnescaped only: decoded key needs escaping on marshal
	k              string
	v              *Value
}

// MaxDepth is the maximum depth for nested JSON.
const MaxDepth = 300

func parseValue(a arena.Arena, s string, depth int) (*Value, string, error) {
	if len(s) == 0 {
		return nil, s, fmt.Errorf("cannot parse empty string")
	}
	depth++
	if depth > MaxDepth {
		return nil, s, fmt.Errorf("too big depth for the nested JSON; it exceeds %d", MaxDepth)
	}

	// Branch prediction optimization: order by frequency
	// Most JSON contains strings and numbers, then objects, then arrays, then literals
	switch s[0] {
	case '"':
		// String - most common in JSON
		ss, tail, hasEscape, err := parseRawStringInfo(s[1:])
		if err != nil {
			return nil, tail, fmt.Errorf("cannot parse string: %s", err)
		}
		v := arena.Allocate[Value](a)
		v.t = TypeString
		if hasEscape {
			v.s, v.stringNeedsEscape = unescapeStringBestEffortInfo(a, ss)
		} else {
			v.s = ss
		}
		return v, tail, nil
	case '{':
		// Object - very common
		v, tail, err := parseObject(a, s[1:], depth)
		if err != nil {
			return nil, tail, fmt.Errorf("cannot parse object: %s", err)
		}
		return v, tail, nil
	case '[':
		// Array - common
		v, tail, err := parseArray(a, s[1:], depth)
		if err != nil {
			return nil, tail, fmt.Errorf("cannot parse array: %s", err)
		}
		return v, tail, nil
	case 't':
		// true literal - less common
		if len(s) < len("true") || s[:len("true")] != "true" {
			return nil, s, fmt.Errorf("unexpected value found: %q", s)
		}
		return valueTrue, s[len("true"):], nil
	case 'f':
		// false literal - less common
		if len(s) < len("false") || s[:len("false")] != "false" {
			return nil, s, fmt.Errorf("unexpected value found: %q", s)
		}
		return valueFalse, s[len("false"):], nil
	case 'n':
		// null literal - less common
		if len(s) < len("null") || s[:len("null")] != "null" {
			// Try parsing NaN
			if len(s) >= 3 && strings.EqualFold(s[:3], "nan") {
				v := arena.Allocate[Value](a)
				v.t = TypeNumber
				v.s = s[:3]
				return v, s[3:], nil
			}
			return nil, s, fmt.Errorf("unexpected value found: %q", s)
		}
		return valueNull, s[len("null"):], nil
	default:
		// Number - very common, but handled last due to complex parsing
		ns, tail, err := parseRawNumber(s)
		if err != nil {
			return nil, tail, fmt.Errorf("cannot parse number: %s", err)
		}
		v := arena.Allocate[Value](a)
		v.t = TypeNumber
		v.s = ns
		return v, tail, nil
	}
}

func parseArray(a arena.Arena, s string, depth int) (*Value, string, error) {
	s = skipWS(s)
	if len(s) == 0 {
		return nil, s, fmt.Errorf("missing ']'")
	}

	if s[0] == ']' {
		v := arena.Allocate[Value](a)
		v.t = TypeArray
		v.a = v.a[:0]
		return v, s[1:], nil
	}

	arr := arena.Allocate[Value](a)
	arr.t = TypeArray
	arr.a = arr.a[:0]
	for {
		var v *Value
		var err error

		s = skipWS(s)
		v, s, err = parseValue(a, s, depth)
		if err != nil {
			return nil, s, fmt.Errorf("cannot parse array value: %s", err)
		}
		if arr.a == nil {
			arr.a = arena.AllocateSlice[*Value](a, 1, 1)
			arr.a[0] = v
		} else {
			arr.a = arena.SliceAppend(a, arr.a, v)
		}

		s = skipWS(s)
		if len(s) == 0 {
			return nil, s, fmt.Errorf("unexpected end of array")
		}
		if s[0] == ',' {
			s = s[1:]
			continue
		}
		if s[0] == ']' {
			s = s[1:]
			return arr, s, nil
		}
		return nil, s, fmt.Errorf("missing ',' after array value")
	}
}

func parseObject(a arena.Arena, s string, depth int) (*Value, string, error) {
	s = skipWS(s)
	if len(s) == 0 {
		return nil, s, fmt.Errorf("missing '}'")
	}

	if s[0] == '}' {
		v := arena.Allocate[Value](a)
		v.t = TypeObject
		v.o.reset()
		return v, s[1:], nil
	}

	o := arena.Allocate[Value](a)
	o.t = TypeObject
	o.o.reset()
	for {
		var err error
		kv := o.o.getKV(a)

		// Parse key.
		s = skipWS(s)
		if len(s) == 0 || s[0] != '"' {
			return nil, s, fmt.Errorf(`cannot find opening '"" for object key`)
		}
		var keyHasEscape bool
		kv.k, s, keyHasEscape, err = parseRawKey(s[1:])
		if err != nil {
			return nil, s, fmt.Errorf("cannot parse object key: %s", err)
		}
		if keyHasEscape {
			kv.k, kv.keyNeedsEscape = unescapeStringBestEffortInfo(a, kv.k)
		}
		kv.keyUnescaped = true
		s = skipWS(s)
		if len(s) == 0 || s[0] != ':' {
			return nil, s, fmt.Errorf("missing ':' after object key")
		}
		s = s[1:]

		// Parse value
		s = skipWS(s)
		kv.v, s, err = parseValue(a, s, depth)
		if err != nil {
			return nil, s, fmt.Errorf("cannot parse object value: %s", err)
		}
		s = skipWS(s)
		if len(s) == 0 {
			return nil, s, fmt.Errorf("unexpected end of object")
		}
		if s[0] == ',' {
			s = s[1:]
			continue
		}
		if s[0] == '}' {
			return o, s[1:], nil
		}
		return nil, s, fmt.Errorf("missing ',' after object value")
	}
}

func appendQuotedString(dst []byte, s string, needsEscape bool) []byte {
	if !needsEscape {
		dst = append(dst, '"')
		dst = append(dst, s...)
		dst = append(dst, '"')
		return dst
	}
	return escapeStringSlowPath(dst, s)
}

func hasSpecialChars(s string) bool {
	// Branch prediction optimization: check most common cases first
	for i := 0; i < len(s); i++ {
		c := s[i]
		// Most common special chars first
		if c == '"' || c == '\\' {
			return true
		}
		// Control characters - less common
		if c < 0x20 {
			return true
		}
	}
	return false
}

func escapeStringSlowPath(dst []byte, s string) []byte {
	dst = append(dst, '"')
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '"':
			// quotation mark
			dst = append(dst, []byte{'\\', '"'}...)
		case c == '\\':
			// reverse solidus
			dst = append(dst, []byte{'\\', '\\'}...)
		case c >= 0x20:
			// default, rest below are control chars
			dst = append(dst, c)
		case c == 0x08:
			dst = append(dst, []byte{'\\', 'b'}...)
		case c < 0x09:
			dst = append(dst, []byte{'\\', 'u', '0', '0', '0', '0' + c}...)
		case c == 0x09:
			dst = append(dst, []byte{'\\', 't'}...)
		case c == 0x0a:
			dst = append(dst, []byte{'\\', 'n'}...)
		case c == 0x0c:
			dst = append(dst, []byte{'\\', 'f'}...)
		case c == 0x0d:
			dst = append(dst, []byte{'\\', 'r'}...)
		case c < 0x10:
			dst = append(dst, []byte{'\\', 'u', '0', '0', '0', 0x57 + c}...)
		case c < 0x1a:
			dst = append(dst, []byte{'\\', 'u', '0', '0', '1', 0x20 + c}...)
		case c < 0x20: // lint:ignore
			dst = append(dst, []byte{'\\', 'u', '0', '0', '1', 0x47 + c}...)
		}
	}
	dst = append(dst, '"')
	return dst
}

func unescapeStringBestEffortInfo(a arena.Arena, s string) (string, bool) {
	if strings.IndexByte(s, '\\') < 0 {
		return s, hasSpecialChars(s)
	}
	n, _ := decodeStringBestEffort(nil, s)
	buf := arena.AllocateSlice[byte](a, n, n)
	_, needsEscape := decodeStringBestEffort(buf, s)
	return b2s(buf), needsEscape
}

func runeNeedsEscaping(r rune) bool {
	return r == '"' || r == '\\' || r < 0x20
}

// parseRawKey is similar to parseRawString, but is optimized
// for small-sized keys without escape sequences.
func parseRawKey(s string) (string, string, bool, error) {
	for i := 0; i < len(s); i++ {
		if s[i] == '"' {
			// Fast path.
			return s[:i], s[i+1:], false, nil
		}
		if s[i] == '\\' {
			// Slow path.
			return parseRawStringInfo(s)
		}
	}
	return s, "", false, fmt.Errorf(`missing closing '"'`)
}

// parseRawStringInfo scans the JSON string payload starting immediately after
// the opening quote.
//
// It returns:
//   - raw: the substring between the opening and closing quotes, without the
//     surrounding quote bytes
//   - tail: the remaining input immediately after the closing quote
//   - hasEscape: whether raw contains at least one backslash escape sequence
//   - err: a non-nil error if no valid closing quote is found
func parseRawStringInfo(s string) (string, string, bool, error) {
	n := strings.IndexByte(s, '"')
	if n < 0 {
		return s, "", false, fmt.Errorf(`missing closing '"'`)
	}
	if strings.IndexByte(s[:n], '\\') < 0 {
		// Fast path. No escape sequences before the closing quote.
		return s[:n], s[n+1:], false, nil
	}
	if n == 0 || s[n-1] != '\\' {
		// Fast path. Escape sequences exist, but the closing quote isn't escaped.
		return s[:n], s[n+1:], true, nil
	}

	// Slow path - possible escaped " found.
	ss := s
	for {
		i := n - 1
		for i > 0 && s[i-1] == '\\' {
			i--
		}
		if uint(n-i)%2 == 0 {
			return ss[:len(ss)-len(s)+n], s[n+1:], true, nil
		}
		s = s[n+1:]

		n = strings.IndexByte(s, '"')
		if n < 0 {
			return ss, "", true, fmt.Errorf(`missing closing '"'`)
		}
		if n == 0 || s[n-1] != '\\' {
			return ss[:len(ss)-len(s)+n], s[n+1:], true, nil
		}
	}
}

func parseRawNumber(s string) (string, string, error) {
	// The caller must ensure len(s) > 0

	// Find the end of the number.
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if (ch >= '0' && ch <= '9') || ch == '.' || ch == '-' || ch == 'e' || ch == 'E' || ch == '+' {
			continue
		}
		if i == 0 || i == 1 && (s[0] == '-' || s[0] == '+') {
			if len(s[i:]) >= 3 {
				xs := s[i : i+3]
				if strings.EqualFold(xs, "inf") || strings.EqualFold(xs, "nan") {
					return s[:i+3], s[i+3:], nil
				}
			}
			return "", s, fmt.Errorf("unexpected char: %q", s[:1])
		}
		ns := s[:i]
		s = s[i:]
		return ns, s, nil
	}
	return s, "", nil
}

// Object represents JSON object.
//
// Object cannot be used from concurrent goroutines.
// Use per-goroutine parsers or ParserPool instead.
//
// Cache-friendly layout: hot data first
type Object struct {
	kvs []*kv // HOT: frequently accessed - 24 bytes
	// Total: 24 bytes - compact and cache-friendly
}

func (o *Object) reset() {
	o.kvs = o.kvs[:0]
}

// MarshalTo appends marshaled o to dst and returns the result.
func (o *Object) MarshalTo(dst []byte) []byte {
	dst = append(dst, '{')
	for i, kv := range o.kvs {
		if kv.keyUnescaped {
			dst = appendQuotedString(dst, kv.k, kv.keyNeedsEscape)
		} else {
			dst = append(dst, '"')
			dst = append(dst, kv.k...)
			dst = append(dst, '"')
		}
		dst = append(dst, ':')
		dst = kv.v.MarshalTo(dst)
		if i != len(o.kvs)-1 {
			dst = append(dst, ',')
		}
	}
	dst = append(dst, '}')
	return dst
}

// String returns string representation for the o.
//
// This function is for debugging purposes only. It isn't optimized for speed.
// See MarshalTo instead.
func (o *Object) String() string {
	b := o.MarshalTo(nil)
	// It is safe converting b to string without allocation, since b is no longer
	// reachable after this line.
	return b2s(b)
}

func (o *Object) getKV(a arena.Arena) *kv {
	if o.kvs == nil {
		o.kvs = arena.AllocateSlice[*kv](a, 0, 1)
	}
	o.kvs = arena.SliceAppend(a, o.kvs, arena.Allocate[kv](a))
	return o.kvs[len(o.kvs)-1]
}

// unescapeKey unescapes a specific key.
// Callers must check kv.keyUnescaped before calling.
func (o *Object) unescapeKey(a arena.Arena, kv *kv) {
	kv.k, kv.keyNeedsEscape = unescapeStringBestEffortInfo(a, kv.k)
	kv.keyUnescaped = true
}

// Len returns the number of items in the o.
func (o *Object) Len() int {
	return len(o.kvs)
}

// Get returns the value for the given key in the o.
//
// Returns nil if the value for the given key isn't found.
//
// The returned value is valid until Parse is called on the Parser returned o.
func (o *Object) Get(key string) *Value {
	if o == nil {
		return nil
	}
	// Keys are always pre-unescaped during parsing and Object.Set,
	// so direct comparison is sufficient.
	for _, kv := range o.kvs {
		if kv.k == key {
			return kv.v
		}
	}
	return nil
}

// Visit calls f for each item in the o in the original order
// of the parsed JSON.
//
// f cannot hold key and/or v after returning.
func (o *Object) Visit(f func(key []byte, v *Value)) {
	if o == nil {
		return
	}
	// Keys are always pre-unescaped during parsing and Object.Set.
	for _, kv := range o.kvs {
		f(s2b(kv.k), kv.v)
	}
}

// Value represents any JSON value.
//
// Call Type in order to determine the actual type of the JSON value.
//
// Value cannot be used from concurrent goroutines.
// Use per-goroutine parsers or ParserPool instead.
//
// Cache-friendly layout: hot data first, compact structure
type Value struct {
	t                 Type // HOT: accessed on every operation
	stringRaw         bool // TypeString only: s contains raw JSON string contents
	stringHasEscapes  bool // TypeString+stringRaw only: raw string contains backslash escapes
	stringNeedsEscape bool // TypeString+!stringRaw only: decoded string needs escaping on marshal
	s                 string
	a                 []*Value
	o                 Object
}

// MarshalTo appends marshaled v to dst and returns the result.
func (v *Value) MarshalTo(dst []byte) []byte {
	switch v.t {
	case TypeObject:
		return v.o.MarshalTo(dst)
	case TypeArray:
		dst = append(dst, '[')
		for i, vv := range v.a {
			dst = vv.MarshalTo(dst)
			if i != len(v.a)-1 {
				dst = append(dst, ',')
			}
		}
		dst = append(dst, ']')
		return dst
	case TypeString:
		if v.stringRaw {
			dst = append(dst, '"')
			dst = append(dst, v.s...)
			dst = append(dst, '"')
			return dst
		}
		return appendQuotedString(dst, v.s, v.stringNeedsEscape)
	case TypeNumber:
		return append(dst, v.s...)
	case TypeTrue:
		return append(dst, "true"...)
	case TypeFalse:
		return append(dst, "false"...)
	case TypeNull:
		return append(dst, "null"...)
	default:
		panic(fmt.Errorf("BUG: unexpected Value type: %d", v.t))
	}
}

// String returns string representation of the v.
//
// The function is for debugging purposes only. It isn't optimized for speed.
// See MarshalTo instead.
//
// Don't confuse this function with StringBytes, which must be called
// for obtaining the underlying JSON string for the v.
func (v *Value) String() string {
	b := v.MarshalTo(nil)
	// It is safe converting b to string without allocation, since b is no longer
	// reachable after this line.
	return b2s(b)
}

// Type represents JSON type.
type Type int

const (
	// TypeNull is JSON null.
	TypeNull Type = 0

	// TypeObject is JSON object type.
	TypeObject Type = 1

	// TypeArray is JSON array type.
	TypeArray Type = 2

	// TypeString is JSON string type.
	TypeString Type = 3

	// TypeNumber is JSON number type.
	TypeNumber Type = 4

	// TypeTrue is JSON true.
	TypeTrue Type = 5

	// TypeFalse is JSON false.
	TypeFalse Type = 6
)

// String returns string representation of t.
func (t Type) String() string {
	switch t {
	case TypeObject:
		return "object"
	case TypeArray:
		return "array"
	case TypeString:
		return "string"
	case TypeNumber:
		return "number"
	case TypeTrue:
		return "true"
	case TypeFalse:
		return "false"
	case TypeNull:
		return "null"

	// typeRawString is skipped intentionally,
	// since it shouldn't be visible to user.
	default:
		panic(fmt.Errorf("BUG: unknown Value type: %d", t))
	}
}

// Type returns the type of the v.
func (v *Value) Type() Type {
	return v.t
}

// Exists returns true if the field exists for the given keys path.
//
// Array indexes may be represented as decimal numbers in keys.
func (v *Value) Exists(keys ...string) bool {
	v = v.Get(keys...)
	return v != nil
}

// Get returns value by the given keys path.
//
// Array indexes may be represented as decimal numbers in keys.
//
// nil is returned for non-existing keys path.
//
// The returned value is valid until Parse is called on the Parser returned v.
func (v *Value) Get(keys ...string) *Value {
	if v == nil {
		return nil
	}
	for _, key := range keys {
		switch v.t {
		case TypeObject:
			v = v.o.Get(key)
			if v == nil {
				return nil
			}
		case TypeArray:
			n, err := strconv.Atoi(key)
			if err != nil || n < 0 || n >= len(v.a) {
				return nil
			}
			v = v.a[n]
		default:
			return nil
		}
	}
	return v
}

// GetObject returns object value by the given keys path.
//
// Array indexes may be represented as decimal numbers in keys.
//
// nil is returned for non-existing keys path or for invalid value type.
//
// The returned object is valid until Parse is called on the Parser returned v.
func (v *Value) GetObject(keys ...string) *Object {
	v = v.Get(keys...)
	if v == nil || v.t != TypeObject {
		return nil
	}
	return &v.o
}

// GetArray returns array value by the given keys path.
//
// Array indexes may be represented as decimal numbers in keys.
//
// nil is returned for non-existing keys path or for invalid value type.
//
// The returned array is valid until Parse is called on the Parser returned v.
func (v *Value) GetArray(keys ...string) []*Value {
	v = v.Get(keys...)
	if v == nil || v.t != TypeArray {
		return nil
	}
	return v.a
}

// GetFloat64 returns float64 value by the given keys path.
//
// Array indexes may be represented as decimal numbers in keys.
//
// 0 is returned for non-existing keys path or for invalid value type.
func (v *Value) GetFloat64(keys ...string) float64 {
	v = v.Get(keys...)
	if v == nil || v.Type() != TypeNumber {
		return 0
	}
	return fastfloat.ParseBestEffort(v.s)
}

// GetInt returns int value by the given keys path.
//
// Array indexes may be represented as decimal numbers in keys.
//
// 0 is returned for non-existing keys path or for invalid value type.
func (v *Value) GetInt(keys ...string) int {
	v = v.Get(keys...)
	if v == nil || v.Type() != TypeNumber {
		return 0
	}
	n := fastfloat.ParseInt64BestEffort(v.s)
	return int(n)
}

// GetUint returns uint value by the given keys path.
//
// Array indexes may be represented as decimal numbers in keys.
//
// 0 is returned for non-existing keys path or for invalid value type.
func (v *Value) GetUint(keys ...string) uint {
	v = v.Get(keys...)
	if v == nil || v.Type() != TypeNumber {
		return 0
	}
	n := fastfloat.ParseUint64BestEffort(v.s)
	return uint(n)
}

// GetInt64 returns int64 value by the given keys path.
//
// Array indexes may be represented as decimal numbers in keys.
//
// 0 is returned for non-existing keys path or for invalid value type.
func (v *Value) GetInt64(keys ...string) int64 {
	v = v.Get(keys...)
	if v == nil || v.Type() != TypeNumber {
		return 0
	}
	return fastfloat.ParseInt64BestEffort(v.s)
}

// GetUint64 returns uint64 value by the given keys path.
//
// Array indexes may be represented as decimal numbers in keys.
//
// 0 is returned for non-existing keys path or for invalid value type.
func (v *Value) GetUint64(keys ...string) uint64 {
	v = v.Get(keys...)
	if v == nil || v.Type() != TypeNumber {
		return 0
	}
	return fastfloat.ParseUint64BestEffort(v.s)
}

// GetStringBytes returns string value by the given keys path.
//
// Array indexes may be represented as decimal numbers in keys.
//
// nil is returned for non-existing keys path or for invalid value type.
//
// The returned string is valid until Parse is called on the Parser returned v.
func (v *Value) GetStringBytes(keys ...string) []byte {
	v = v.Get(keys...)
	if v == nil || v.Type() != TypeString {
		return nil
	}
	v.ensureDecodedString()
	return s2b(v.s)
}

// GetBool returns bool value by the given keys path.
//
// Array indexes may be represented as decimal numbers in keys.
//
// false is returned for non-existing keys path or for invalid value type.
func (v *Value) GetBool(keys ...string) bool {
	v = v.Get(keys...)
	if v != nil && v.t == TypeTrue {
		return true
	}
	return false
}

// Object returns the underlying JSON object for the v.
//
// The returned object is valid until Parse is called on the Parser returned v.
//
// Use GetObject if you don't need error handling.
func (v *Value) Object() (*Object, error) {
	if v.t != TypeObject {
		return nil, fmt.Errorf("value doesn't contain object; it contains %s", v.Type())
	}
	return &v.o, nil
}

// Array returns the underlying JSON array for the v.
//
// The returned array is valid until Parse is called on the Parser returned v.
//
// Use GetArray if you don't need error handling.
func (v *Value) Array() ([]*Value, error) {
	if v.t != TypeArray {
		return nil, fmt.Errorf("value doesn't contain array; it contains %s", v.Type())
	}
	return v.a, nil
}

// StringBytes returns the underlying JSON string for the v.
//
// The returned string is valid until Parse is called on the Parser returned v.
//
// Use GetStringBytes if you don't need error handling.
func (v *Value) StringBytes() ([]byte, error) {
	if v.Type() != TypeString {
		return nil, fmt.Errorf("value doesn't contain string; it contains %s", v.Type())
	}
	v.ensureDecodedString()
	return s2b(v.s), nil
}

func (v *Value) ensureDecodedString() {
	if v == nil || v.t != TypeString || !v.stringRaw {
		return
	}
	if !v.stringHasEscapes {
		v.stringRaw = false
		v.stringNeedsEscape = false
		return
	}
	v.s, v.stringNeedsEscape = unescapeStringBestEffortInfo(nil, v.s)
	v.stringRaw = false
	v.stringHasEscapes = false
}

// Float64 returns the underlying JSON number for the v.
//
// Use GetFloat64 if you don't need error handling.
func (v *Value) Float64() (float64, error) {
	if v.Type() != TypeNumber {
		return 0, fmt.Errorf("value doesn't contain number; it contains %s", v.Type())
	}
	return fastfloat.Parse(v.s)
}

// Int returns the underlying JSON int for the v.
//
// Use GetInt if you don't need error handling.
func (v *Value) Int() (int, error) {
	if v.Type() != TypeNumber {
		return 0, fmt.Errorf("value doesn't contain number; it contains %s", v.Type())
	}
	n, err := fastfloat.ParseInt64(v.s)
	if err != nil {
		return 0, err
	}
	return int(n), nil
}

// Uint returns the underlying JSON uint for the v.
//
// Use GetInt if you don't need error handling.
func (v *Value) Uint() (uint, error) {
	if v.Type() != TypeNumber {
		return 0, fmt.Errorf("value doesn't contain number; it contains %s", v.Type())
	}
	n, err := fastfloat.ParseUint64(v.s)
	if err != nil {
		return 0, err
	}
	return uint(n), nil
}

// Int64 returns the underlying JSON int64 for the v.
//
// Use GetInt64 if you don't need error handling.
func (v *Value) Int64() (int64, error) {
	if v.Type() != TypeNumber {
		return 0, fmt.Errorf("value doesn't contain number; it contains %s", v.Type())
	}
	return fastfloat.ParseInt64(v.s)
}

// Uint64 returns the underlying JSON uint64 for the v.
//
// Use GetInt64 if you don't need error handling.
func (v *Value) Uint64() (uint64, error) {
	if v.Type() != TypeNumber {
		return 0, fmt.Errorf("value doesn't contain number; it contains %s", v.Type())
	}
	return fastfloat.ParseUint64(v.s)
}

// Bool returns the underlying JSON bool for the v.
//
// Use GetBool if you don't need error handling.
func (v *Value) Bool() (bool, error) {
	if v.t == TypeTrue {
		return true, nil
	}
	if v.t == TypeFalse {
		return false, nil
	}
	return false, fmt.Errorf("value doesn't contain bool; it contains %s", v.Type())
}

var (
	valueTrue  = &Value{t: TypeTrue}
	valueFalse = &Value{t: TypeFalse}
	valueNull  = &Value{t: TypeNull}
)
