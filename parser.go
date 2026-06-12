package astjson

import (
	"errors"
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

// maxArenaScratchCap bounds the capacity of the planner scratch buffers
// retained on a Parser between parses. A single outlier parse can grow these
// slices to the size of the largest document seen; without a cap, a pooled
// or long-lived Parser would permanently hold that peak capacity. Any buffer
// whose cap exceeds this bound is reallocated at the start of the next parse.
const maxArenaScratchCap = 4096

func (p *Parser) ensureArenaScratch() *arenaPlanScratch {
	if p == nil {
		return nil
	}
	if p.arenaScratch == nil {
		p.arenaScratch = &arenaPlanScratch{}
		return p.arenaScratch
	}
	s := p.arenaScratch
	capBoundInts(&s.objectSizes)
	capBoundInts(&s.arraySizes)
	capBoundSpans(&s.keySpans)
	capBoundSpans(&s.stringSpans)
	capBoundInts(&s.deepCopyObjectSizes)
	capBoundInts(&s.deepCopyArraySizes)
	return s
}

func capBoundInts(b *[]int) {
	if cap(*b) > maxArenaScratchCap {
		*b = make([]int, 0, maxArenaScratchCap)
	}
}

func capBoundSpans(b *[]arenaStringSpan) {
	if cap(*b) > maxArenaScratchCap {
		*b = make([]arenaStringSpan, 0, maxArenaScratchCap)
	}
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
	keyUnescaped   bool // tracks if this specific key has been unescaped
	keyNeedsEscape bool // keyUnescaped only: decoded key needs escaping on marshal
	k              string
	v              *Value
}

// MaxDepth is the maximum depth for nested JSON.
const MaxDepth = 300

func parseValue(a arena.Arena, s string, depth int) (*Value, string, error) {
	if len(s) == 0 {
		return nil, s, errParseEmpty
	}
	depth++
	if depth > MaxDepth {
		return nil, s, errParseMaxDepth
	}

	// Branch prediction optimization: order by frequency
	// Most JSON contains strings and numbers, then objects, then arrays, then literals
	switch s[0] {
	case '"':
		// String - most common in JSON
		ss, tail, hasEscape, err := parseRawStringInfo(s[1:])
		if err != nil {
			return nil, tail, errors.New("cannot parse string: " + err.Error())
		}
		v := arena.Allocate[Value](a)
		v.t = TypeString
		if hasEscape {
			v.s, v.stringNeedsEscape = unescapeStringBestEffortInfo(a, ss)
		} else {
			v.s = ss
			v.stringNeedsEscape = hasSpecialChars(ss)
		}
		v.noEscapeSubtree = !v.stringNeedsEscape
		return v, tail, nil
	case '{':
		// Object - very common
		v, tail, err := parseObject(a, s[1:], depth)
		if err != nil {
			return nil, tail, errors.New("cannot parse object: " + err.Error())
		}
		return v, tail, nil
	case '[':
		// Array - common
		v, tail, err := parseArray(a, s[1:], depth)
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
			if len(s) >= 3 && strings.EqualFold(s[:3], "nan") {
				v := arena.Allocate[Value](a)
				v.t = TypeNumber
				v.s = s[:3]
				v.noEscapeSubtree = true
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
		v := arena.Allocate[Value](a)
		v.t = TypeNumber
		v.s = ns
		v.noEscapeSubtree = true
		return v, tail, nil
	}
}

func parseArray(a arena.Arena, s string, depth int) (*Value, string, error) {
	s = skipWS(s)
	if len(s) == 0 {
		return nil, s, errParseMissingCloseBracket
	}

	if s[0] == ']' {
		v := arena.Allocate[Value](a)
		v.t = TypeArray
		v.a = v.a[:0]
		v.noEscapeSubtree = true
		return v, s[1:], nil
	}

	arr := arena.Allocate[Value](a)
	arr.t = TypeArray
	arr.a = arr.a[:0]
	clean := true
	for {
		var v *Value
		var err error

		s = skipWS(s)
		v, s, err = parseValue(a, s, depth)
		if err != nil {
			return nil, s, errors.New("cannot parse array value: " + err.Error())
		}
		if arr.a == nil {
			arr.a = arena.AllocateSlice[*Value](a, 1, 1)
			arr.a[0] = v
		} else {
			arr.a = arena.SliceAppend(a, arr.a, v)
		}
		clean = clean && valueIsEscapeFree(v)

		s = skipWS(s)
		if len(s) == 0 {
			return nil, s, errParseUnexpectedEndArray
		}
		if s[0] == ',' {
			s = s[1:]
			continue
		}
		if s[0] == ']' {
			s = s[1:]
			arr.noEscapeSubtree = clean
			return arr, s, nil
		}
		return nil, s, errParseMissingCommaArray
	}
}

func parseObject(a arena.Arena, s string, depth int) (*Value, string, error) {
	s = skipWS(s)
	if len(s) == 0 {
		return nil, s, errParseMissingCloseBrace
	}

	if s[0] == '}' {
		v := arena.Allocate[Value](a)
		v.t = TypeObject
		v.o.reset()
		v.noEscapeSubtree = true
		return v, s[1:], nil
	}

	o := arena.Allocate[Value](a)
	o.t = TypeObject
	o.o.reset()
	clean := true
	for {
		var err error
		kv := o.o.getKV(a)

		// Parse key.
		s = skipWS(s)
		if len(s) == 0 || s[0] != '"' {
			return nil, s, errParseMissingOpenQuote
		}
		var keyHasEscape bool
		kv.k, s, keyHasEscape, err = parseRawKey(s[1:])
		if err != nil {
			return nil, s, errors.New("cannot parse object key: " + err.Error())
		}
		if keyHasEscape {
			kv.k, kv.keyNeedsEscape = unescapeStringBestEffortInfo(a, kv.k)
		} else {
			kv.keyNeedsEscape = hasSpecialChars(kv.k)
		}
		kv.keyUnescaped = true
		s = skipWS(s)
		if len(s) == 0 || s[0] != ':' {
			return nil, s, errParseMissingColon
		}
		s = s[1:]

		// Parse value
		s = skipWS(s)
		kv.v, s, err = parseValue(a, s, depth)
		if err != nil {
			return nil, s, errors.New("cannot parse object value: " + err.Error())
		}
		clean = clean && !kv.keyNeedsEscape && valueIsEscapeFree(kv.v)
		s = skipWS(s)
		if len(s) == 0 {
			return nil, s, errParseUnexpectedEndObject
		}
		if s[0] == ',' {
			s = s[1:]
			continue
		}
		if s[0] == '}' {
			o.noEscapeSubtree = clean
			return o, s[1:], nil
		}
		return nil, s, errParseMissingCommaObject
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

// valueIsEscapeFree reports whether v contributes no escape-requiring
// content to an enclosing container's noEscapeSubtree aggregate. For
// TypeString this means the decoded string does not need escaping; for
// containers it relies on their noEscapeSubtree flag; scalars without a
// payload (true/false/null/number) never contribute and return true.
func valueIsEscapeFree(v *Value) bool {
	if v == nil {
		return true
	}
	switch v.t {
	case TypeString:
		return !v.stringNeedsEscape
	case TypeObject, TypeArray:
		return v.noEscapeSubtree
	default:
		return true
	}
}

// RecomputeEscapeHint walks v bottom-up and refreshes the noEscapeSubtree
// flag from the current state of keys and string values. Call this after
// mutating through a sub-handle of a larger tree if you plan to rely on
// the hint at a higher level — mutation APIs only update the flag on the
// directly-mutated node and cannot invalidate ancestors.
func (v *Value) RecomputeEscapeHint() bool {
	if v == nil {
		return true
	}
	switch v.t {
	case TypeString:
		v.noEscapeSubtree = !v.stringNeedsEscape
	case TypeObject:
		clean := true
		for _, kv := range v.o.kvs {
			childClean := kv.v.RecomputeEscapeHint()
			clean = clean && !kv.keyNeedsEscape && childClean
		}
		v.noEscapeSubtree = clean
	case TypeArray:
		clean := true
		for _, item := range v.a {
			clean = clean && item.RecomputeEscapeHint()
		}
		v.noEscapeSubtree = clean
	default:
		v.noEscapeSubtree = true
	}
	return v.noEscapeSubtree
}

// hasSpecialChars reports whether s contains any byte that requires
// escaping in a JSON string: '"', '\\', or a control byte (< 0x20).
//
// ----------------------------------------------------------------------
// What is SWAR?
// ----------------------------------------------------------------------
//
// SWAR stands for "SIMD Within A Register". It is a way to do parallel
// byte operations using only ordinary integer instructions — no actual
// SIMD/vector hardware required. The idea is:
//
//  1. Pack 8 bytes side-by-side into one uint64. A 64-bit register is
//     really "8 lanes of 1 byte each" if you squint at it.
//  2. Apply scalar ALU ops (+ - & | ^ ~) to the whole uint64 at once.
//     Those ops naturally act on each lane in parallel — as long as
//     you choose ops that don't let a lane corrupt its neighbours.
//  3. One ALU op now does 8 bytes of work. Loading 1 byte and loading
//     8 bytes both cost a single MOV, so we are getting roughly 8× the
//     throughput per cycle without touching NEON/AVX at all.
//
// The fundamental primitive is the Mycroft "haszero" trick:
//
//	hasZeroByte(x) = (x - 0x0101010101010101) & ^x & 0x8080808080808080
//
// Why it works, lane by lane:
//   - Subtracting 1 from a byte that is 0x00 underflows to 0xFF, which
//     sets that lane's high bit. Subtracting 1 from any byte in 1..255
//     leaves the high bit clear (or unchanged).
//   - "& ^x" guards against bytes whose high bit was already set in x
//     (e.g. 0x80): without this, those would falsely register as "zero".
//   - "& 0x80..80" keeps only the eight high bits — one bit per lane —
//     so a single non-zero test answers "any lane matched?" in one branch.
//
// Every other predicate we want is a one-line transform on top of haszero:
//   - "byte == c"   →  hasZeroByte(x ^ broadcast(c))   (XOR makes c → 0)
//   - "byte <  N"   →  ((x - N*lo) & ^x) & hi          (underflow trick)
//
// Caveat — borrow propagation. The subtraction is a real 64-bit subtract,
// so an underflow in one lane borrows from the next-higher lane and can
// corrupt its result. That can yield extra "matches", but only ever
// in lanes adjacent to a *real* match. Since we only ask "did anything
// match?", the boolean answer is still correct. (If we needed the index
// of the first match, we would use a saturated variant: `(x | hi) - …`
// pre-sets each high bit, which absorbs the borrow within a lane and
// prevents it from crossing into the neighbour.)
//
// ----------------------------------------------------------------------
// How this function uses SWAR
// ----------------------------------------------------------------------
//
// The hot path scans 8 bytes at a time, building three byte-wise
// predicates in parallel — "byte < 0x20", "byte == '\"'", "byte == '\\'" —
// then OR-ing them and testing the eight high bits in a single branch.
// The tail (< 8 leftover bytes) falls back to the per-byte charFlags
// lookup. Crossover vs. the pure byte loop is at len ≈ 8; see
// BenchmarkHasSpecialSweep.
//
// Constants below are byte-broadcast masks: "lo" sets bit 0 of every
// byte, "hi" sets bit 7 of every byte; the others broadcast a single
// byte value (e.g. 0x22 = '"') across all eight lanes so we can test
// every byte in parallel.
const (
	hasSpecialLo     uint64 = 0x0101010101010101 // 1 in each byte (subtract pattern)
	hasSpecialHi     uint64 = 0x8080808080808080 // high bit of each byte (extract pattern)
	hasSpecialQuote  uint64 = 0x2222222222222222 // '"'  broadcast
	hasSpecialBSlash uint64 = 0x5C5C5C5C5C5C5C5C // '\\' broadcast
	hasSpecial0x20   uint64 = 0x2020202020202020 // 0x20 broadcast
)

func hasSpecialChars(s string) bool {
	i := 0
	// 8-byte SWAR loop. We process the string in aligned-by-8 chunks
	// from the head; the residual 0..7 bytes are handled by the tail
	// loop below.
	for i+8 <= len(s) {
		// Hoist the bounds check: a single panic-or-pass at s[i+7]
		// lets the compiler prove all the s[i..i+6] indexes below are
		// in range and elide their per-access bounds checks.
		_ = s[i+7]
		// Pack 8 bytes into a single uint64 in little-endian order.
		// On amd64/arm64 the compiler folds this whole expression into
		// one unaligned 64-bit load, so the cost is one MOV — not eight.
		// LE order is irrelevant to correctness because every predicate
		// below is byte-wise (no inter-byte arithmetic semantics relied on).
		v := uint64(s[i]) |
			uint64(s[i+1])<<8 |
			uint64(s[i+2])<<16 |
			uint64(s[i+3])<<24 |
			uint64(s[i+4])<<32 |
			uint64(s[i+5])<<40 |
			uint64(s[i+6])<<48 |
			uint64(s[i+7])<<56

		// --- Predicate 1: any byte < 0x20 (control character)?
		//
		// Per-byte we want b < 0x20. Subtracting 0x20 from every byte
		// in parallel underflows exactly the bytes where b < 0x20, and
		// underflow sets that byte's high bit. The "& ^v" step is what
		// makes this safe in the presence of high-byte values: for any
		// byte where the original was >= 0x80, ^v's high bit is 0, so
		// we can't get a spurious match from a byte that already had
		// its high bit set.
		//
		// Borrow caveat: subtraction is a real 64-bit subtract, so a
		// borrow can propagate from a lower byte into a higher one and
		// corrupt that higher byte's result. That can produce extra
		// "matches" — but a borrow only originates from a byte that
		// genuinely satisfied b < 0x20. So any cascaded false positive
		// already coexists with a true positive, and the OR-then-test
		// at the end still returns the correct boolean.
		ctrl := (v - hasSpecial0x20) & ^v

		// --- Predicate 2: any byte == '"' (0x22)?
		//
		// XOR with the broadcast pattern turns matching bytes into 0x00
		// and leaves all other bytes non-zero. Then the standard
		// haszero formula detects any zero byte. Same borrow caveat as
		// above applies — and is harmless for the same reason.
		q := v ^ hasSpecialQuote
		qmask := (q - hasSpecialLo) & ^q

		// --- Predicate 3: any byte == '\\' (0x5C)? Same shape as Predicate 2.
		sl := v ^ hasSpecialBSlash
		smask := (sl - hasSpecialLo) & ^sl

		// Combine predicates: each byte's high bit in (ctrl|qmask|smask)
		// is the disjunction of the three per-byte tests. ANDing with
		// the high-bit mask isolates just those eight result bits, and
		// a single non-zero test covers the whole 8-byte window in one
		// branch. This is what lets SWAR overtake the byte loop: per
		// byte we pay roughly one ALU op instead of one load + mask +
		// branch.
		if (ctrl|qmask|smask)&hasSpecialHi != 0 {
			return true
		}
		i += 8
	}
	// Tail: handle the 0..7 bytes that didn't fit into a full SWAR
	// chunk. Setting up another SWAR pass for ≤ 7 bytes (masking the
	// unread lanes) costs more than just doing the byte loop here, and
	// at this point we've already amortized the SWAR setup across the
	// head of the string anyway.
	for ; i < len(s); i++ {
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
	return s, "", false, errParseMissingCloseQuote
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
		return s, "", false, errParseMissingCloseQuote
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
			return ss, "", true, errParseMissingCloseQuote
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
		if charFlags[s[i]]&charNumChar != 0 {
			continue
		}
		if i == 0 || i == 1 && (s[0] == '-' || s[0] == '+') {
			if len(s[i:]) >= 3 {
				xs := s[i : i+3]
				if strings.EqualFold(xs, "inf") || strings.EqualFold(xs, "nan") {
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

// marshalToClean is a MarshalTo fast path for objects whose own keys are
// all escape-free. It skips the per-key escape check for THIS object, but
// recurses into children via [Value.MarshalTo] so each child re-checks
// its own noEscapeSubtree flag. That way a stale-true ancestor hint
// (left behind by mutation through a sub-handle) cannot cause a dirty
// descendant to be written out unescaped.
func (o *Object) marshalToClean(dst []byte) []byte {
	dst = append(dst, '{')
	for i, kv := range o.kvs {
		dst = append(dst, '"')
		dst = append(dst, kv.k...)
		dst = append(dst, '"', ':')
		dst = kv.v.MarshalTo(dst)
		if i != len(o.kvs)-1 {
			dst = append(dst, ',')
		}
	}
	return append(dst, '}')
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
	stringNeedsEscape bool // TypeString only: decoded string needs escaping on marshal
	// noEscapeSubtree is an advisory hint: when true, no key or string value
	// in this subtree required JSON escaping at the time of parse or the last
	// call to RecomputeEscapeHint. false is always safe; consumers that fast
	// path on true must tolerate stale-true on ancestors of mutations made
	// through a sub-handle. See MUTATION CORRECTNESS in package docs.
	noEscapeSubtree bool
	s               string
	a               []*Value
	o               Object
}

// MarshalTo appends marshaled v to dst and returns the result.
func (v *Value) MarshalTo(dst []byte) []byte {
	// Fast path: if the entire subtree is known to be escape-free, emit
	// each string/key literally without per-node escape checks. The hint
	// is advisory — callers who mutate through a sub-handle and want the
	// root hint to stay accurate must call [Value.RecomputeEscapeHint].
	if v.noEscapeSubtree && (v.t == TypeObject || v.t == TypeArray || v.t == TypeString) {
		v.debugVerifyEscapeHint()
		return v.marshalToClean(dst)
	}
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
		panic("BUG: unexpected Value type: " + strconv.Itoa(int(v.t)))
	}
}

// marshalToClean is a MarshalTo fast path for this node's own keys/string;
// see [Object.marshalToClean] for the ancestor-stale rationale. Children of
// objects and arrays are emitted via their own [Value.MarshalTo] so each
// subtree re-checks its own hint.
func (v *Value) marshalToClean(dst []byte) []byte {
	switch v.t {
	case TypeObject:
		return v.o.marshalToClean(dst)
	case TypeArray:
		dst = append(dst, '[')
		for i, vv := range v.a {
			dst = vv.MarshalTo(dst)
			if i != len(v.a)-1 {
				dst = append(dst, ',')
			}
		}
		return append(dst, ']')
	case TypeString:
		dst = append(dst, '"')
		dst = append(dst, v.s...)
		return append(dst, '"')
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
	valueTrue  = &Value{t: TypeTrue, noEscapeSubtree: true}
	valueFalse = &Value{t: TypeFalse, noEscapeSubtree: true}
	valueNull  = &Value{t: TypeNull, noEscapeSubtree: true}
)
