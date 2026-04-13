package astjson

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wundergraph/go-arena"
)

func TestArenaTwoPassPlanCounts(t *testing.T) {
	t.Run("nested structures without escapes", func(t *testing.T) {
		plan, err := planArenaParse(`{"a":1,"b":[true,false],"c":{"d":"x"}}`)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if plan.values != 5 {
			t.Fatalf("unexpected value count: got %d want 5", plan.values)
		}
		if plan.kvs != 4 {
			t.Fatalf("unexpected kv count: got %d want 4", plan.kvs)
		}
		if plan.arrayElems != 2 {
			t.Fatalf("unexpected array element count: got %d want 2", plan.arrayElems)
		}
		if plan.stringBytes != 0 {
			t.Fatalf("unexpected decoded string byte count: got %d want 0", plan.stringBytes)
		}
	})

	t.Run("escaped value strings no longer reserve decoded bytes", func(t *testing.T) {
		plan, err := planArenaParse(`{"a":"\n","b":"plain","c":"\u263A"}`)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if plan.values != 4 {
			t.Fatalf("unexpected value count: got %d want 4", plan.values)
		}
		if plan.kvs != 3 {
			t.Fatalf("unexpected kv count: got %d want 3", plan.kvs)
		}
		if plan.stringBytes != 0 {
			t.Fatalf("unexpected decoded string byte count: got %d want 0", plan.stringBytes)
		}
	})

	t.Run("singleton literals do not reserve value slab entries", func(t *testing.T) {
		plan, err := planArenaParse(`{"t":true,"f":false,"n":null}`)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if plan.values != 1 {
			t.Fatalf("unexpected value count: got %d want 1", plan.values)
		}
		if plan.kvs != 3 {
			t.Fatalf("unexpected kv count: got %d want 3", plan.kvs)
		}
	})
}

func TestParseRawKeyReportsEscapes(t *testing.T) {
	raw, tail, hasEscape, err := parseRawKey(`escaped\u006bey":1}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasEscape {
		t.Fatalf("expected escape to be reported")
	}
	if raw != `escaped\u006bey` {
		t.Fatalf("raw key = %q", raw)
	}
	if tail != `:1}` {
		t.Fatalf("tail = %q", tail)
	}

	raw, tail, hasEscape, err = parseRawKey(`plain":1}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasEscape {
		t.Fatalf("did not expect escape to be reported")
	}
	if raw != "plain" {
		t.Fatalf("raw key = %q", raw)
	}
	if tail != `:1}` {
		t.Fatalf("tail = %q", tail)
	}
}

func TestParserArenaPlannerScratchReuse(t *testing.T) {
	var parser Parser
	a := arena.NewMonotonicArena()

	if _, err := parser.ParseBytesWithArena(a, []byte(`{"a":{"b":"x"},"c":[1,2,3],"d":{"e":{"f":true}}}`)); err != nil {
		t.Fatalf("first parse failed: %v", err)
	}
	if parser.arenaScratch == nil {
		t.Fatalf("expected parser arena scratch to be initialized")
	}
	objectCap := cap(parser.arenaScratch.objectSizes)
	arrayCap := cap(parser.arenaScratch.arraySizes)
	if objectCap == 0 {
		t.Fatalf("expected object scratch capacity to be retained")
	}
	if arrayCap == 0 {
		t.Fatalf("expected array scratch capacity to be retained")
	}
	keyCap := cap(parser.arenaScratch.keySpans)
	valueStringCap := cap(parser.arenaScratch.stringSpans)
	if keyCap == 0 {
		t.Fatalf("expected key span scratch capacity to be retained")
	}
	if valueStringCap == 0 {
		t.Fatalf("expected string span scratch capacity to be retained")
	}

	a.Reset()
	if _, err := parser.ParseBytesWithArena(a, []byte(`{"x":1}`)); err != nil {
		t.Fatalf("second parse failed: %v", err)
	}
	if cap(parser.arenaScratch.objectSizes) != objectCap {
		t.Fatalf("object scratch capacity changed: got %d want %d", cap(parser.arenaScratch.objectSizes), objectCap)
	}
	if cap(parser.arenaScratch.arraySizes) != arrayCap {
		t.Fatalf("array scratch capacity changed: got %d want %d", cap(parser.arenaScratch.arraySizes), arrayCap)
	}
	if cap(parser.arenaScratch.keySpans) != keyCap {
		t.Fatalf("key span scratch capacity changed: got %d want %d", cap(parser.arenaScratch.keySpans), keyCap)
	}
	if cap(parser.arenaScratch.stringSpans) != valueStringCap {
		t.Fatalf("string span scratch capacity changed: got %d want %d", cap(parser.arenaScratch.stringSpans), valueStringCap)
	}
}

func TestParseBytesWithArenaMatchesHeapFixtures(t *testing.T) {
	repoRoot := "."
	fixtures := []string{"small.json", "medium.json", "twitter.json"}

	for _, fixture := range fixtures {
		t.Run(fixture, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(repoRoot, "testdata", fixture))
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}

			var heapParser Parser
			heapValue, err := heapParser.ParseBytes(data)
			if err != nil {
				t.Fatalf("heap parse failed: %v", err)
			}

			var arenaParser Parser
			a := arena.NewMonotonicArena()
			arenaValue, err := arenaParser.ParseBytesWithArena(a, data)
			if err != nil {
				t.Fatalf("arena parse failed: %v", err)
			}

			heapJSON := string(heapValue.MarshalTo(nil))
			arenaJSON := string(arenaValue.MarshalTo(nil))
			if heapJSON != arenaJSON {
				t.Fatalf("parsed JSON mismatch")
			}
		})
	}
}

func TestParseBytesWithArenaRejectsMalformedStrings(t *testing.T) {
	input := `{"a":"unterminated}`

	var parser Parser
	a := arena.NewMonotonicArena()
	if _, err := parser.ParseBytesWithArena(a, []byte(input)); err == nil {
		t.Fatalf("expected parse error for %q", input)
	}
}

func TestParseBytesWithArenaLazilyDecodesStringValues(t *testing.T) {
	input := []byte(`{"msg":"hello\nworld","plain":"ok"}`)

	var parser Parser
	a := arena.NewMonotonicArena()
	v, err := parser.ParseBytesWithArena(a, input)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	if got := string(v.MarshalTo(nil)); got != string(input) {
		t.Fatalf("marshal mismatch: got %q want %q", got, string(input))
	}

	msg := v.Get("msg")
	b, err := msg.StringBytes()
	if err != nil {
		t.Fatalf("StringBytes failed: %v", err)
	}
	if got := string(b); got != "hello\nworld" {
		t.Fatalf("decoded string = %q, want %q", got, "hello\nworld")
	}

	if got := string(v.MarshalTo(nil)); got != string(input) {
		t.Fatalf("marshal after decode mismatch: got %q want %q", got, string(input))
	}
}

func TestParseBytesWithArenaTracksStringEscapeFlags(t *testing.T) {
	input := []byte(`{"plain":"ok","escaped":"he said \"hi\"\n"}`)

	var parser Parser
	a := arena.NewMonotonicArena()
	v, err := parser.ParseBytesWithArena(a, input)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	plain := v.Get("plain")
	if !plain.stringRaw {
		t.Fatalf("expected plain string to stay raw before first decode")
	}
	if plain.stringHasEscapes {
		t.Fatalf("expected plain string to report no raw escapes")
	}
	if plain.stringNeedsEscape {
		t.Fatalf("expected plain string to report no marshal escaping")
	}

	escaped := v.Get("escaped")
	if !escaped.stringRaw {
		t.Fatalf("expected escaped string to stay raw before first decode")
	}
	if !escaped.stringHasEscapes {
		t.Fatalf("expected escaped string to report raw escapes")
	}
	if escaped.stringNeedsEscape {
		t.Fatalf("expected escaped string marshal flag to remain unset while raw")
	}

	sb, err := escaped.StringBytes()
	if err != nil {
		t.Fatalf("StringBytes failed: %v", err)
	}
	if string(sb) != "he said \"hi\"\n" {
		t.Fatalf("decoded string = %q", string(sb))
	}
	if escaped.stringRaw {
		t.Fatalf("expected escaped string to be decoded after StringBytes")
	}
	if escaped.stringHasEscapes {
		t.Fatalf("expected raw escape flag to be cleared after decode")
	}
	if !escaped.stringNeedsEscape {
		t.Fatalf("expected decoded string to still require escaping on marshal")
	}
}

func TestParseBytesWithArenaReusesLiteralSingletons(t *testing.T) {
	input := []byte(`{"t":true,"f":false,"n":null,"arr":[true,false,null]}`)

	var parser Parser
	a := arena.NewMonotonicArena()
	v, err := parser.ParseBytesWithArena(a, input)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	if got := v.Get("t"); got != valueTrue {
		t.Fatalf("true node = %p want singleton %p", got, valueTrue)
	}
	if got := v.Get("f"); got != valueFalse {
		t.Fatalf("false node = %p want singleton %p", got, valueFalse)
	}
	if got := v.Get("n"); got != valueNull {
		t.Fatalf("null node = %p want singleton %p", got, valueNull)
	}

	arr := v.GetArray("arr")
	if len(arr) != 3 {
		t.Fatalf("unexpected array length: got %d want 3", len(arr))
	}
	if arr[0] != valueTrue || arr[1] != valueFalse || arr[2] != valueNull {
		t.Fatalf("array literal singletons were not reused")
	}

	if got := string(v.MarshalTo(nil)); got != string(input) {
		t.Fatalf("marshal mismatch: got %q want %q", got, string(input))
	}
}

func TestParseBytesWithArenaDoesNotHeapAllocatePerParse(t *testing.T) {
	input := []byte(`{"a":[1,2,3],"b":{"c":"x"}}`)

	var parser Parser
	a := arena.NewMonotonicArena()

	allocs := testing.AllocsPerRun(1000, func() {
		a.Reset()
		v, err := parser.ParseBytesWithArena(a, input)
		if err != nil {
			t.Fatalf("parse failed: %v", err)
		}
		_ = v.Type()
	})

	if allocs != 0 {
		t.Fatalf("allocs per run = %v, want 0", allocs)
	}
}
