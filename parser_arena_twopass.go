package astjson

import (
	"fmt"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/wundergraph/go-arena"
)

type arenaParsePlan struct {
	values      int
	kvs         int
	arrayElems  int
	stringBytes int

	objectSizes []int
	arraySizes  []int
	keySpans    []arenaStringSpan
	stringSpans []arenaStringSpan
}

type arenaStringSpan struct {
	rawLen    int
	hasEscape bool
}

type arenaPlanScratch struct {
	objectSizes []int
	arraySizes  []int
	keySpans    []arenaStringSpan
	stringSpans []arenaStringSpan

	deepCopyObjectSizes []int
	deepCopyArraySizes  []int
}

type arenaPlanner struct {
	plan arenaParsePlan
}

type arenaFillState struct {
	a    arena.Arena
	plan arenaParsePlan

	values []Value
	kvs    []kv

	objectRefs []*kv
	arrayRefs  []*Value
	strings    []byte

	valuePos     int
	kvPos        int
	objectRefPos int
	arrayRefPos  int
	stringPos    int
	objectPos    int
	arrayPos     int
	keySpanPos   int
	valueSpanPos int
}

func planArenaParse(s string) (arenaParsePlan, error) {
	return planArenaParseWithScratch(s, nil)
}

func planArenaParseWithScratch(s string, scratch *arenaPlanScratch) (arenaParsePlan, error) {
	s = skipWS(s)
	planner := newArenaPlanner(scratch)
	tail, err := planner.planValue(s, 0)
	if err != nil {
		return arenaParsePlan{}, err
	}
	tail = skipWS(tail)
	if len(tail) > 0 {
		return arenaParsePlan{}, fmt.Errorf("unexpected tail: %q", startEndString(tail))
	}
	return planner.finish(scratch), nil
}

func newArenaPlanner(scratch *arenaPlanScratch) arenaPlanner {
	plan := arenaParsePlan{}
	if scratch != nil {
		plan.objectSizes = scratch.objectSizes[:0]
		plan.arraySizes = scratch.arraySizes[:0]
		plan.keySpans = scratch.keySpans[:0]
		plan.stringSpans = scratch.stringSpans[:0]
	}
	return arenaPlanner{plan: plan}
}

func (p *arenaPlanner) finish(scratch *arenaPlanScratch) arenaParsePlan {
	if scratch != nil {
		scratch.objectSizes = p.plan.objectSizes[:0]
		scratch.arraySizes = p.plan.arraySizes[:0]
		scratch.keySpans = p.plan.keySpans[:0]
		scratch.stringSpans = p.plan.stringSpans[:0]
	}
	return p.plan
}

func (p *arenaPlanner) planValue(s string, depth int) (string, error) {
	if len(s) == 0 {
		return s, fmt.Errorf("cannot parse empty string")
	}
	depth++
	if depth > MaxDepth {
		return s, fmt.Errorf("too big depth for the nested JSON; it exceeds %d", MaxDepth)
	}

	switch s[0] {
	case '"':
		p.plan.values++
		raw, tail, hasEscape, err := parseRawStringInfo(s[1:])
		if err != nil {
			return tail, fmt.Errorf("cannot parse string: %s", err)
		}
		p.plan.stringSpans = append(p.plan.stringSpans, arenaStringSpan{
			rawLen:    len(raw),
			hasEscape: hasEscape,
		})
		return tail, nil
	case '{':
		p.plan.values++
		return p.planObject(s[1:], depth)
	case '[':
		p.plan.values++
		return p.planArray(s[1:], depth)
	case 't':
		if len(s) < len("true") || s[:len("true")] != "true" {
			return s, fmt.Errorf("unexpected value found: %q", s)
		}
		return s[len("true"):], nil
	case 'f':
		if len(s) < len("false") || s[:len("false")] != "false" {
			return s, fmt.Errorf("unexpected value found: %q", s)
		}
		return s[len("false"):], nil
	case 'n':
		if len(s) < len("null") || s[:len("null")] != "null" {
			if len(s) >= 3 && strings.EqualFold(s[:3], "nan") {
				p.plan.values++
				return s[3:], nil
			}
			return s, fmt.Errorf("unexpected value found: %q", s)
		}
		return s[len("null"):], nil
	default:
		p.plan.values++
		_, tail, err := parseRawNumber(s)
		if err != nil {
			return tail, fmt.Errorf("cannot parse number: %s", err)
		}
		return tail, nil
	}
}

func (p *arenaPlanner) planArray(s string, depth int) (string, error) {
	s = skipWS(s)
	if len(s) == 0 {
		return "", fmt.Errorf("missing ']'")
	}

	idx := len(p.plan.arraySizes)
	p.plan.arraySizes = append(p.plan.arraySizes, 0)
	if s[0] == ']' {
		return s[1:], nil
	}

	count := 0
	for {
		var err error
		s = skipWS(s)
		s, err = p.planValue(s, depth)
		if err != nil {
			return s, fmt.Errorf("cannot parse array value: %s", err)
		}
		p.plan.arrayElems++
		count++

		s = skipWS(s)
		if len(s) == 0 {
			return s, fmt.Errorf("unexpected end of array")
		}
		if s[0] == ',' {
			s = s[1:]
			continue
		}
		if s[0] == ']' {
			p.plan.arraySizes[idx] = count
			return s[1:], nil
		}
		return s, fmt.Errorf("missing ',' after array value")
	}
}

func (p *arenaPlanner) planObject(s string, depth int) (string, error) {
	s = skipWS(s)
	if len(s) == 0 {
		return "", fmt.Errorf("missing '}'")
	}

	idx := len(p.plan.objectSizes)
	p.plan.objectSizes = append(p.plan.objectSizes, 0)
	if s[0] == '}' {
		return s[1:], nil
	}

	count := 0
	for {
		var err error
		s = skipWS(s)
		if len(s) == 0 || s[0] != '"' {
			return s, fmt.Errorf(`cannot find opening '"" for object key`)
		}
		rawKey, tail, hasEscape, err := parseRawKey(s[1:])
		if err != nil {
			return tail, fmt.Errorf("cannot parse object key: %s", err)
		}
		p.plan.keySpans = append(p.plan.keySpans, arenaStringSpan{
			rawLen:    len(rawKey),
			hasEscape: hasEscape,
		})
		if hasEscape {
			p.plan.stringBytes += decodedStringBestEffortLen(rawKey)
		}
		s = skipWS(tail)
		if len(s) == 0 || s[0] != ':' {
			return s, fmt.Errorf("missing ':' after object key")
		}
		s = s[1:]
		s = skipWS(s)
		s, err = p.planValue(s, depth)
		if err != nil {
			return s, fmt.Errorf("cannot parse object value: %s", err)
		}
		p.plan.kvs++
		count++

		s = skipWS(s)
		if len(s) == 0 {
			return s, fmt.Errorf("unexpected end of object")
		}
		if s[0] == ',' {
			s = s[1:]
			continue
		}
		if s[0] == '}' {
			p.plan.objectSizes[idx] = count
			return s[1:], nil
		}
		return s, fmt.Errorf("missing ',' after object value")
	}
}

func parseArenaTwoPass(p *Parser, a arena.Arena, s string) (*Value, error) {
	s = skipWS(s)
	scratch := p.ensureArenaScratch()
	planner := newArenaPlanner(scratch)
	tail, err := planner.planValue(s, 0)
	if err != nil {
		return nil, NewParseError(fmt.Errorf("cannot parse JSON: %s; unparsed tail: %q", err, startEndString(tail)))
	}
	plan := planner.finish(scratch)
	if scratch != nil {
		defer func() {
			scratch.objectSizes = plan.objectSizes[:0]
			scratch.arraySizes = plan.arraySizes[:0]
			scratch.keySpans = plan.keySpans[:0]
			scratch.stringSpans = plan.stringSpans[:0]
		}()
	}
	tail = skipWS(tail)
	if len(tail) > 0 {
		return nil, NewParseError(fmt.Errorf("unexpected tail: %q", startEndString(tail)))
	}

	filler := newArenaFillState(a, plan)
	v, tail, err := filler.parseValue(s, 0)
	if err != nil {
		return nil, NewParseError(fmt.Errorf("cannot parse JSON: %s; unparsed tail: %q", err, startEndString(tail)))
	}
	tail = skipWS(tail)
	if len(tail) > 0 {
		return nil, NewParseError(fmt.Errorf("unexpected tail: %q", startEndString(tail)))
	}
	if err := filler.finish(); err != nil {
		return nil, NewParseError(err)
	}
	return v, nil
}

func newArenaFillState(a arena.Arena, plan arenaParsePlan) arenaFillState {
	state := arenaFillState{a: a, plan: plan}
	slabs := allocateArenaSlabs(a, plan.values, plan.kvs, plan.arrayElems, plan.stringBytes)
	state.values = slabs.values
	state.kvs = slabs.kvs
	state.objectRefs = slabs.objectRefs
	state.arrayRefs = slabs.arrayRefs
	state.strings = slabs.strings
	return state
}

func (f *arenaFillState) finish() error {
	if f.valuePos != len(f.values) {
		return fmt.Errorf("BUG: values slab mismatch: used %d of %d", f.valuePos, len(f.values))
	}
	if f.kvPos != len(f.kvs) {
		return fmt.Errorf("BUG: kv slab mismatch: used %d of %d", f.kvPos, len(f.kvs))
	}
	if f.objectRefPos != len(f.objectRefs) {
		return fmt.Errorf("BUG: object ref slab mismatch: used %d of %d", f.objectRefPos, len(f.objectRefs))
	}
	if f.arrayRefPos != len(f.arrayRefs) {
		return fmt.Errorf("BUG: array ref slab mismatch: used %d of %d", f.arrayRefPos, len(f.arrayRefs))
	}
	if f.stringPos != len(f.strings) {
		return fmt.Errorf("BUG: string slab mismatch: used %d of %d", f.stringPos, len(f.strings))
	}
	if f.objectPos != len(f.plan.objectSizes) {
		return fmt.Errorf("BUG: object count mismatch: used %d of %d", f.objectPos, len(f.plan.objectSizes))
	}
	if f.arrayPos != len(f.plan.arraySizes) {
		return fmt.Errorf("BUG: array count mismatch: used %d of %d", f.arrayPos, len(f.plan.arraySizes))
	}
	if f.keySpanPos != len(f.plan.keySpans) {
		return fmt.Errorf("BUG: key span count mismatch: used %d of %d", f.keySpanPos, len(f.plan.keySpans))
	}
	if f.valueSpanPos != len(f.plan.stringSpans) {
		return fmt.Errorf("BUG: string span count mismatch: used %d of %d", f.valueSpanPos, len(f.plan.stringSpans))
	}
	return nil
}

func (f *arenaFillState) allocValue() *Value {
	v := &f.values[f.valuePos]
	f.valuePos++
	v.stringNeedsEscape = false
	return v
}

func (f *arenaFillState) allocKV() *kv {
	entry := &f.kvs[f.kvPos]
	f.kvPos++
	return entry
}

func (f *arenaFillState) allocObjectRefs(n int) []*kv {
	start := f.objectRefPos
	f.objectRefPos += n
	return f.objectRefs[start:f.objectRefPos:f.objectRefPos]
}

func (f *arenaFillState) allocArrayRefs(n int) []*Value {
	start := f.arrayRefPos
	f.arrayRefPos += n
	return f.arrayRefs[start:f.arrayRefPos:f.arrayRefPos]
}

func (f *arenaFillState) allocString(n int) []byte {
	start := f.stringPos
	f.stringPos += n
	return f.strings[start:f.stringPos]
}

func (f *arenaFillState) nextObjectSize() int {
	n := f.plan.objectSizes[f.objectPos]
	f.objectPos++
	return n
}

func (f *arenaFillState) nextArraySize() int {
	n := f.plan.arraySizes[f.arrayPos]
	f.arrayPos++
	return n
}

func (f *arenaFillState) nextKeySpan() arenaStringSpan {
	span := f.plan.keySpans[f.keySpanPos]
	f.keySpanPos++
	return span
}

func (f *arenaFillState) nextStringSpan() arenaStringSpan {
	span := f.plan.stringSpans[f.valueSpanPos]
	f.valueSpanPos++
	return span
}

func consumePlannedString(s string, span arenaStringSpan) (string, string, error) {
	end := 1 + span.rawLen
	if len(s) <= end || s[0] != '"' || s[end] != '"' {
		return "", s, fmt.Errorf("BUG: planned string span mismatch")
	}
	return s[1:end], s[end+1:], nil
}

func (f *arenaFillState) parseValue(s string, depth int) (*Value, string, error) {
	if len(s) == 0 {
		return nil, s, fmt.Errorf("cannot parse empty string")
	}
	depth++
	if depth > MaxDepth {
		return nil, s, fmt.Errorf("too big depth for the nested JSON; it exceeds %d", MaxDepth)
	}

	switch s[0] {
	case '"':
		span := f.nextStringSpan()
		raw, tail, err := consumePlannedString(s, span)
		if err != nil {
			return nil, tail, fmt.Errorf("cannot parse string: %s", err)
		}
		v := f.allocValue()
		v.t = TypeString
		if span.hasEscape {
			v.s, v.stringNeedsEscape = unescapeStringBestEffortInfo(f.a, raw)
		} else {
			v.s = raw
		}
		v.a = nil
		v.o.reset()
		return v, tail, nil
	case '{':
		return f.parseObject(s[1:], depth)
	case '[':
		return f.parseArray(s[1:], depth)
	case 't':
		if len(s) < len("true") || s[:len("true")] != "true" {
			return nil, s, fmt.Errorf("unexpected value found: %q", s)
		}
		return valueTrue, s[len("true"):], nil
	case 'f':
		if len(s) < len("false") || s[:len("false")] != "false" {
			return nil, s, fmt.Errorf("unexpected value found: %q", s)
		}
		return valueFalse, s[len("false"):], nil
	case 'n':
		if len(s) < len("null") || s[:len("null")] != "null" {
			if len(s) >= 3 && strings.EqualFold(s[:3], "nan") {
				v := f.allocValue()
				v.t = TypeNumber
				v.s = s[:3]
				v.a = nil
				v.o.reset()
				return v, s[3:], nil
			}
			return nil, s, fmt.Errorf("unexpected value found: %q", s)
		}
		return valueNull, s[len("null"):], nil
	default:
		ns, tail, err := parseRawNumber(s)
		if err != nil {
			return nil, tail, fmt.Errorf("cannot parse number: %s", err)
		}
		v := f.allocValue()
		v.t = TypeNumber
		v.s = ns
		v.a = nil
		v.o.reset()
		return v, tail, nil
	}
}

func (f *arenaFillState) parseArray(s string, depth int) (*Value, string, error) {
	s = skipWS(s)
	if len(s) == 0 {
		return nil, s, fmt.Errorf("missing ']'")
	}

	v := f.allocValue()
	v.t = TypeArray
	v.s = ""
	v.o.reset()
	count := f.nextArraySize()
	if count == 0 {
		v.a = nil
		return v, s[1:], nil
	}

	v.a = f.allocArrayRefs(count)
	for i := 0; i < count; i++ {
		var err error
		s = skipWS(s)
		v.a[i], s, err = f.parseValue(s, depth)
		if err != nil {
			return nil, s, fmt.Errorf("cannot parse array value: %s", err)
		}
		s = skipWS(s)
		if i == count-1 {
			if len(s) == 0 || s[0] != ']' {
				return nil, s, fmt.Errorf("missing ',' after array value")
			}
			return v, s[1:], nil
		}
		if len(s) == 0 {
			return nil, s, fmt.Errorf("unexpected end of array")
		}
		if s[0] != ',' {
			return nil, s, fmt.Errorf("missing ',' after array value")
		}
		s = s[1:]
	}
	return nil, s, fmt.Errorf("BUG: unreachable array parse")
}

func (f *arenaFillState) parseObject(s string, depth int) (*Value, string, error) {
	s = skipWS(s)
	if len(s) == 0 {
		return nil, s, fmt.Errorf("missing '}'")
	}

	v := f.allocValue()
	v.t = TypeObject
	v.s = ""
	v.a = nil
	count := f.nextObjectSize()
	if count == 0 {
		v.o.reset()
		return v, s[1:], nil
	}

	v.o.kvs = f.allocObjectRefs(count)
	for i := 0; i < count; i++ {
		var err error
		entry := f.allocKV()
		v.o.kvs[i] = entry

		s = skipWS(s)
		if len(s) == 0 || s[0] != '"' {
			return nil, s, fmt.Errorf(`cannot find opening '"" for object key`)
		}
		keySpan := f.nextKeySpan()
		entry.k, s, err = consumePlannedString(s, keySpan)
		if err != nil {
			return nil, s, fmt.Errorf("cannot parse object key: %s", err)
		}
		entry.k, entry.keyNeedsEscape = f.storeString(entry.k, keySpan.hasEscape)
		entry.keyUnescaped = true

		s = skipWS(s)
		if len(s) == 0 || s[0] != ':' {
			return nil, s, fmt.Errorf("missing ':' after object key")
		}
		s = s[1:]

		s = skipWS(s)
		entry.v, s, err = f.parseValue(s, depth)
		if err != nil {
			return nil, s, fmt.Errorf("cannot parse object value: %s", err)
		}
		s = skipWS(s)
		if i == count-1 {
			if len(s) == 0 || s[0] != '}' {
				return nil, s, fmt.Errorf("missing ',' after object value")
			}
			return v, s[1:], nil
		}
		if len(s) == 0 {
			return nil, s, fmt.Errorf("unexpected end of object")
		}
		if s[0] != ',' {
			return nil, s, fmt.Errorf("missing ',' after object value")
		}
		s = s[1:]
	}
	return nil, s, fmt.Errorf("BUG: unreachable object parse")
}

func (f *arenaFillState) storeString(raw string, hasEscape bool) (string, bool) {
	if !hasEscape {
		return raw, false
	}
	n := decodedStringBestEffortLen(raw)
	var buf []byte
	if len(f.strings) > 0 {
		buf = f.allocString(n)
	} else {
		buf = arena.AllocateSlice[byte](f.a, n, n)
	}
	_, needsEscape := decodeStringBestEffort(buf, raw)
	return b2s(buf), needsEscape
}

func decodedStringBestEffortLen(s string) int {
	n, _ := decodeStringBestEffort(nil, s)
	return n
}

func decodeStringBestEffort(dst []byte, s string) (int, bool) {
	out := 0
	needsEscape := false
	write := func(bs ...byte) {
		if dst != nil {
			copy(dst[out:], bs)
		}
		out += len(bs)
	}
	writeString := func(ss string) {
		if dst != nil {
			copy(dst[out:], ss)
		}
		out += len(ss)
	}

	n := strings.IndexByte(s, '\\')
	if n < 0 {
		writeString(s)
		return out, hasSpecialChars(s)
	}

	writeString(s[:n])
	s = s[n+1:]

	for len(s) > 0 {
		ch := s[0]
		s = s[1:]
		switch ch {
		case '"':
			write('"')
			needsEscape = true
		case '\\':
			write('\\')
			needsEscape = true
		case '/':
			write('/')
		case 'b':
			write('\b')
			needsEscape = true
		case 'f':
			write('\f')
			needsEscape = true
		case 'n':
			write('\n')
			needsEscape = true
		case 'r':
			write('\r')
			needsEscape = true
		case 't':
			write('\t')
			needsEscape = true
		case 'u':
			if len(s) < 4 {
				write('\\', 'u')
				needsEscape = true
				break
			}
			xs := s[:4]
			x, err := parseUint16Hex(xs)
			if err != nil {
				write('\\', 'u')
				needsEscape = true
				break
			}
			s = s[4:]
			if !utf16.IsSurrogate(rune(x)) {
				var buf [utf8.UTFMax]byte
				r := rune(x)
				n := utf8.EncodeRune(buf[:], r)
				write(buf[:n]...)
				needsEscape = needsEscape || runeNeedsEscaping(r)
				break
			}

			if len(s) < 6 || s[0] != '\\' || s[1] != 'u' {
				write('\\', 'u')
				writeString(xs)
				needsEscape = true
				break
			}
			x1, err := parseUint16Hex(s[2:6])
			if err != nil {
				write('\\', 'u')
				writeString(xs)
				needsEscape = true
				break
			}
			r := utf16.DecodeRune(rune(x), rune(x1))
			var buf [utf8.UTFMax]byte
			n := utf8.EncodeRune(buf[:], r)
			write(buf[:n]...)
			needsEscape = needsEscape || runeNeedsEscaping(r)
			s = s[6:]
		default:
			write('\\', ch)
			needsEscape = true
		}

		n = strings.IndexByte(s, '\\')
		if n < 0 {
			writeString(s)
			break
		}
		writeString(s[:n])
		s = s[n+1:]
	}

	return out, needsEscape
}

func parseUint16Hex(s string) (uint16, error) {
	var n uint16
	for i := 0; i < len(s); i++ {
		n <<= 4
		switch ch := s[i]; {
		case ch >= '0' && ch <= '9':
			n |= uint16(ch - '0')
		case ch >= 'a' && ch <= 'f':
			n |= uint16(ch-'a') + 10
		case ch >= 'A' && ch <= 'F':
			n |= uint16(ch-'A') + 10
		default:
			return 0, fmt.Errorf("invalid hex")
		}
	}
	return n, nil
}
