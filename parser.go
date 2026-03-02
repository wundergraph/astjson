package astjson

import (
	"errors"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/wundergraph/astjson/fastfloat"
	"github.com/wundergraph/go-arena"
)

// Sentinel errors for static error messages.
// Using pre-allocated errors avoids fmt.Errorf allocations and removes the fmt
// import, which can improve inlining budgets for functions in this file.
var (
	errEmptyString         = errors.New("cannot parse empty string")
	errMaxDepth            = errors.New("too big depth for the nested JSON; it exceeds 300")
	errMissingClosingBracket = errors.New("missing ']'")
	errMissingClosingBrace = errors.New("missing '}'")
	errMissingCommaArray   = errors.New("missing ',' after array value")
	errMissingCommaObject  = errors.New("missing ',' after object value")
	errUnexpectedEndArray  = errors.New("unexpected end of array")
	errUnexpectedEndObject = errors.New("unexpected end of object")
	errMissingOpenQuote    = errors.New(`cannot find opening '"' for object key`)
	errMissingColon        = errors.New("missing ':' after object key")
	errMissingClosingQuote = errors.New(`missing closing '"'`)
)

// parseContext holds per-parse state including slab allocators that amortize
// arena allocation overhead by allocating Values and kvs in batches.
type parseContext struct {
	a  arena.Arena
	vs valueSlab
	ks kvSlab
}

// valueSlab allocates Values in batches to amortize arena overhead.
// Starts with a small batch and doubles up to maxSlabSize.
type valueSlab struct {
	values []Value
	pos    int
}

const (
	minSlabSize = 8
	maxSlabSize = 64
)

func (s *valueSlab) get(a arena.Arena) *Value {
	if a == nil {
		return new(Value)
	}
	if s.pos >= len(s.values) {
		size := len(s.values) * 2
		if size < minSlabSize {
			size = minSlabSize
		} else if size > maxSlabSize {
			size = maxSlabSize
		}
		s.values = arena.AllocateSlice[Value](a, size, size)
		s.pos = 0
	}
	v := &s.values[s.pos]
	s.pos++
	return v
}

// kvSlab allocates kv structs in batches to amortize arena overhead.
type kvSlab struct {
	kvs []kv
	pos int
}

func (s *kvSlab) get(a arena.Arena) *kv {
	if a == nil {
		return new(kv)
	}
	if s.pos >= len(s.kvs) {
		size := len(s.kvs) * 2
		if size < minSlabSize {
			size = minSlabSize
		} else if size > maxSlabSize {
			size = maxSlabSize
		}
		s.kvs = arena.AllocateSlice[kv](a, size, size)
		s.pos = 0
	}
	k := &s.kvs[s.pos]
	s.pos++
	return k
}

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
		return p.parse(a, b2s(ab))
	}
	return p.parse(nil, b2s(b))
}

func (p *Parser) parse(a arena.Arena, s string) (*Value, error) {
	ctx := parseContext{a: a}
	s = skipWS(s)

	v, tail, err := parseValue(&ctx, s, 0)
	if err != nil {
		return nil, NewParseError(errors.New("cannot parse JSON: " + err.Error() + "; unparsed tail: " + strconv.Quote(startEndString(tail))))
	}
	tail = skipWS(tail)
	if len(tail) > 0 {
		return nil, NewParseError(errors.New("unexpected tail: " + strconv.Quote(startEndString(tail))))
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
	for i := 0; i < len(s); i++ {
		if charFlags[s[i]]&charWS == 0 {
			return s[i:]
		}
	}
	return ""
}

// kv represents a key-value pair in JSON objects.
// Cache-friendly layout: hot data first
type kv struct {
	keyUnescaped bool   // 1 byte - tracks if this specific key has been unescaped
	k            string // 16 bytes
	v            *Value // 8 bytes
	// Total: 25 bytes - still fits in cache line
}

// MaxDepth is the maximum depth for nested JSON.
const MaxDepth = 300

func parseValue(ctx *parseContext, s string, depth int) (*Value, string, error) {
	if len(s) == 0 {
		return nil, s, errEmptyString
	}
	depth++
	if depth > MaxDepth {
		return nil, s, errMaxDepth
	}

	// Branch prediction optimization: order by frequency
	// Most JSON contains strings and numbers, then objects, then arrays, then literals
	switch s[0] {
	case '"':
		// String - most common in JSON
		ss, tail, err := parseRawString(s[1:])
		if err != nil {
			return nil, tail, errors.New("cannot parse string: " + err.Error())
		}
		v := ctx.vs.get(ctx.a)
		v.t = TypeString
		v.s = unescapeStringBestEffort(ctx.a, ss)
		return v, tail, nil
	case '{':
		// Object - very common
		v, tail, err := parseObject(ctx, s[1:], depth)
		if err != nil {
			return nil, tail, errors.New("cannot parse object: " + err.Error())
		}
		return v, tail, nil
	case '[':
		// Array - common
		v, tail, err := parseArray(ctx, s[1:], depth)
		if err != nil {
			return nil, tail, errors.New("cannot parse array: " + err.Error())
		}
		return v, tail, nil
	case 't':
		// true literal - less common
		if len(s) < len("true") || s[:len("true")] != "true" {
			return nil, s, errors.New("unexpected value found: " + strconv.Quote(s))
		}
		return valueTrue, s[len("true"):], nil
	case 'f':
		// false literal - less common
		if len(s) < len("false") || s[:len("false")] != "false" {
			return nil, s, errors.New("unexpected value found: " + strconv.Quote(s))
		}
		return valueFalse, s[len("false"):], nil
	case 'n':
		// null literal - less common
		if len(s) < len("null") || s[:len("null")] != "null" {
			// Try parsing NaN
			if len(s) >= 3 && (s[0]|0x20) == 'n' && (s[1]|0x20) == 'a' && (s[2]|0x20) == 'n' {
				v := ctx.vs.get(ctx.a)
				v.t = TypeNumber
				v.s = s[:3]
				return v, s[3:], nil
			}
			return nil, s, errors.New("unexpected value found: " + strconv.Quote(s))
		}
		return valueNull, s[len("null"):], nil
	default:
		// Number - very common, but handled last due to complex parsing
		ns, tail, err := parseRawNumber(s)
		if err != nil {
			return nil, tail, errors.New("cannot parse number: " + err.Error())
		}
		v := ctx.vs.get(ctx.a)
		v.t = TypeNumber
		v.s = ns
		return v, tail, nil
	}
}

func parseArray(ctx *parseContext, s string, depth int) (*Value, string, error) {
	s = skipWS(s)
	if len(s) == 0 {
		return nil, s, errMissingClosingBracket
	}

	if s[0] == ']' {
		v := ctx.vs.get(ctx.a)
		v.t = TypeArray
		v.a = v.a[:0]
		return v, s[1:], nil
	}

	arr := ctx.vs.get(ctx.a)
	arr.t = TypeArray
	arr.a = arena.AllocateSlice[*Value](ctx.a, 0, 8)
	for {
		var v *Value
		var err error

		s = skipWS(s)
		v, s, err = parseValue(ctx, s, depth)
		if err != nil {
			return nil, s, errors.New("cannot parse array value: " + err.Error())
		}
		if len(arr.a) < cap(arr.a) {
			arr.a = append(arr.a, v)
		} else {
			arr.a = arena.SliceAppend(ctx.a, arr.a, v)
		}

		s = skipWS(s)
		if len(s) == 0 {
			return nil, s, errUnexpectedEndArray
		}
		if s[0] == ',' {
			s = s[1:]
			continue
		}
		if s[0] == ']' {
			s = s[1:]
			return arr, s, nil
		}
		return nil, s, errMissingCommaArray
	}
}

func parseObject(ctx *parseContext, s string, depth int) (*Value, string, error) {
	s = skipWS(s)
	if len(s) == 0 {
		return nil, s, errMissingClosingBrace
	}

	if s[0] == '}' {
		v := ctx.vs.get(ctx.a)
		v.t = TypeObject
		v.o.reset()
		return v, s[1:], nil
	}

	o := ctx.vs.get(ctx.a)
	o.t = TypeObject
	o.o.kvs = arena.AllocateSlice[*kv](ctx.a, 0, 8)
	for {
		var err error
		// Inline kv allocation from slab instead of calling getKV
		// (getKV is kept unchanged for Object.Set in update.go)
		newKV := ctx.ks.get(ctx.a)
		if len(o.o.kvs) < cap(o.o.kvs) {
			o.o.kvs = append(o.o.kvs, newKV)
		} else {
			o.o.kvs = arena.SliceAppend(ctx.a, o.o.kvs, newKV)
		}

		// Parse key.
		s = skipWS(s)
		if len(s) == 0 || s[0] != '"' {
			return nil, s, errMissingOpenQuote
		}
		newKV.k, s, err = parseRawKey(s[1:])
		if err != nil {
			return nil, s, errors.New("cannot parse object key: " + err.Error())
		}
		newKV.k = unescapeStringBestEffort(ctx.a, newKV.k)
		newKV.keyUnescaped = true
		s = skipWS(s)
		if len(s) == 0 || s[0] != ':' {
			return nil, s, errMissingColon
		}
		s = s[1:]

		// Parse value
		s = skipWS(s)
		newKV.v, s, err = parseValue(ctx, s, depth)
		if err != nil {
			return nil, s, errors.New("cannot parse object value: " + err.Error())
		}
		s = skipWS(s)
		if len(s) == 0 {
			return nil, s, errUnexpectedEndObject
		}
		if s[0] == ',' {
			s = s[1:]
			continue
		}
		if s[0] == '}' {
			return o, s[1:], nil
		}
		return nil, s, errMissingCommaObject
	}
}

func escapeString(dst []byte, s string) []byte {
	if !hasSpecialChars(s) {
		// Fast path - nothing to escape.
		dst = append(dst, '"')
		dst = append(dst, s...)
		dst = append(dst, '"')
		return dst
	}

	// Slow path.
	return escapeStringSlowPath(dst, s)
}

func hasSpecialChars(s string) bool {
	for i := 0; i < len(s); i++ {
		if charFlags[s[i]]&charEscape != 0 {
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

func unescapeStringBestEffort(a arena.Arena, s string) string {
	n := strings.IndexByte(s, '\\')
	if n < 0 {
		// Fast path - nothing to unescape.
		return s
	}

	// Pre-allocate buffer to len(s) — unescaped is always <= escaped length.
	// Use direct indexing instead of per-character SliceAppend.
	b := arena.AllocateSlice[byte](a, len(s), len(s))
	w := copy(b, s[:n])
	s = s[n+1:]

	for len(s) > 0 {
		ch := s[0]
		s = s[1:]
		switch ch {
		case '"':
			b[w] = '"'
			w++
		case '\\':
			b[w] = '\\'
			w++
		case '/':
			b[w] = '/'
			w++
		case 'b':
			b[w] = '\b'
			w++
		case 'f':
			b[w] = '\f'
			w++
		case 'n':
			b[w] = '\n'
			w++
		case 'r':
			b[w] = '\r'
			w++
		case 't':
			b[w] = '\t'
			w++
		case 'u':
			if len(s) < 4 {
				b[w] = '\\'
				b[w+1] = 'u'
				w += 2
				break
			}
			xs := s[:4]
			x, ok := parseHex4(xs)
			if !ok {
				b[w] = '\\'
				b[w+1] = 'u'
				w += 2
				break
			}
			s = s[4:]
			if !utf16.IsSurrogate(rune(x)) {
				w += utf8.EncodeRune(b[w:], rune(x))
				break
			}

			// Surrogate.
			// See https://en.wikipedia.org/wiki/Universal_Character_Set_characters#Surrogates
			if len(s) < 6 || s[0] != '\\' || s[1] != 'u' {
				b[w] = '\\'
				b[w+1] = 'u'
				w += 2
				w += copy(b[w:], xs)
				break
			}
			x1, ok := parseHex4(s[2:6])
			if !ok {
				b[w] = '\\'
				b[w+1] = 'u'
				w += 2
				w += copy(b[w:], xs)
				break
			}
			r := utf16.DecodeRune(rune(x), rune(x1))
			w += utf8.EncodeRune(b[w:], r)
			s = s[6:]
		default:
			b[w] = '\\'
			b[w+1] = ch
			w += 2
		}
		n = strings.IndexByte(s, '\\')
		if n < 0 {
			w += copy(b[w:], s)
			break
		}
		w += copy(b[w:], s[:n])
		s = s[n+1:]
	}
	return b2s(b[:w])
}

// parseRawKey is similar to parseRawString, but is optimized
// for small-sized keys without escape sequences.
func parseRawKey(s string) (string, string, error) {
	n := strings.IndexByte(s, '"')
	if n < 0 {
		return s, "", errMissingClosingQuote
	}
	// Check if the key portion contains an escape sequence.
	if strings.IndexByte(s[:n], '\\') >= 0 {
		return parseRawString(s)
	}
	return s[:n], s[n+1:], nil
}

func parseRawString(s string) (string, string, error) {
	n := strings.IndexByte(s, '"')
	if n < 0 {
		return s, "", errMissingClosingQuote
	}
	if n == 0 || s[n-1] != '\\' {
		// Fast path. No escaped ".
		return s[:n], s[n+1:], nil
	}

	// Slow path - possible escaped " found.
	ss := s
	for {
		i := n - 1
		for i > 0 && s[i-1] == '\\' {
			i--
		}
		if uint(n-i)%2 == 0 {
			return ss[:len(ss)-len(s)+n], s[n+1:], nil
		}
		s = s[n+1:]

		n = strings.IndexByte(s, '"')
		if n < 0 {
			return ss, "", errMissingClosingQuote
		}
		if n == 0 || s[n-1] != '\\' {
			return ss[:len(ss)-len(s)+n], s[n+1:], nil
		}
	}
}

func parseRawNumber(s string) (string, string, error) {
	// The caller must ensure len(s) > 0

	// Find the end of the number.
	for i := 0; i < len(s); i++ {
		if charFlags[s[i]]&charNumChar != 0 {
			continue
		}
		if i == 0 || i == 1 && (s[0] == '-' || s[0] == '+') {
			if len(s[i:]) >= 3 {
				xs := s[i : i+3]
				if ((xs[0]|0x20) == 'i' && (xs[1]|0x20) == 'n' && (xs[2]|0x20) == 'f') ||
					((xs[0]|0x20) == 'n' && (xs[1]|0x20) == 'a' && (xs[2]|0x20) == 'n') {
					return s[:i+3], s[i+3:], nil
				}
			}
			return "", s, errors.New("unexpected char: " + strconv.Quote(s[:1]))
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
	kvs     []*kv          // HOT: frequently accessed
	kvIndex map[string]int // lazily-built reverse index for O(1) lookups on objects with >16 keys; invalidated on Del/Set
}

func (o *Object) reset() {
	o.kvs = o.kvs[:0]
	o.kvIndex = nil
}

// MarshalTo appends marshaled o to dst and returns the result.
func (o *Object) MarshalTo(dst []byte) []byte {
	dst = append(dst, '{')
	for i, kv := range o.kvs {
		if kv.keyUnescaped {
			dst = escapeString(dst, kv.k)
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
		o.kvs = arena.AllocateSlice[*kv](a, 0, 4)
	}
	newKV := arena.Allocate[kv](a)
	if len(o.kvs) < cap(o.kvs) {
		o.kvs = append(o.kvs, newKV)
	} else {
		o.kvs = arena.SliceAppend(a, o.kvs, newKV)
	}
	return o.kvs[len(o.kvs)-1]
}

// unescapeKey unescapes a specific key.
// Callers must check kv.keyUnescaped before calling.
func (o *Object) unescapeKey(a arena.Arena, kv *kv) {
	kv.k = unescapeStringBestEffort(a, kv.k)
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
	// For large objects, use a lazily-built hash map for O(1) lookup.
	if len(o.kvs) > 16 {
		if o.kvIndex == nil {
			o.kvIndex = make(map[string]int, len(o.kvs))
			for i, kv := range o.kvs {
				// Store first occurrence to match linear scan semantics.
				if _, exists := o.kvIndex[kv.k]; !exists {
					o.kvIndex[kv.k] = i
				}
			}
		}
		if i, ok := o.kvIndex[key]; ok {
			return o.kvs[i].v
		}
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
	t Type     // HOT: accessed on every operation - 1 byte
	s string   // HOT: frequently accessed for strings/numbers - 16 bytes
	a []*Value // HOT: frequently accessed for arrays - 24 bytes
	o Object   // COLD: less frequently accessed - 24 bytes
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
		return escapeString(dst, v.s)
	case TypeNumber:
		return append(dst, v.s...)
	case TypeTrue:
		return append(dst, "true"...)
	case TypeFalse:
		return append(dst, "false"...)
	case TypeNull:
		return append(dst, "null"...)
	default:
		panic("BUG: unexpected Value type: " + strconv.Itoa(int(v.t)))
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
type Type uint8

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
		panic("BUG: unknown Value type: " + strconv.Itoa(int(t)))
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
		return nil, errors.New("value doesn't contain object; it contains " + v.Type().String())
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
		return nil, errors.New("value doesn't contain array; it contains " + v.Type().String())
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
		return nil, errors.New("value doesn't contain string; it contains " + v.Type().String())
	}
	return s2b(v.s), nil
}

// Float64 returns the underlying JSON number for the v.
//
// Use GetFloat64 if you don't need error handling.
func (v *Value) Float64() (float64, error) {
	if v.Type() != TypeNumber {
		return 0, errors.New("value doesn't contain number; it contains " + v.Type().String())
	}
	return fastfloat.Parse(v.s)
}

// Int returns the underlying JSON int for the v.
//
// Use GetInt if you don't need error handling.
func (v *Value) Int() (int, error) {
	if v.Type() != TypeNumber {
		return 0, errors.New("value doesn't contain number; it contains " + v.Type().String())
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
		return 0, errors.New("value doesn't contain number; it contains " + v.Type().String())
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
		return 0, errors.New("value doesn't contain number; it contains " + v.Type().String())
	}
	return fastfloat.ParseInt64(v.s)
}

// Uint64 returns the underlying JSON uint64 for the v.
//
// Use GetInt64 if you don't need error handling.
func (v *Value) Uint64() (uint64, error) {
	if v.Type() != TypeNumber {
		return 0, errors.New("value doesn't contain number; it contains " + v.Type().String())
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
	return false, errors.New("value doesn't contain bool; it contains " + v.Type().String())
}

var (
	valueTrue  = &Value{t: TypeTrue}
	valueFalse = &Value{t: TypeFalse}
	valueNull  = &Value{t: TypeNull}
)
