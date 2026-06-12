package astjson

import (
	"testing"
	"unsafe"

	"github.com/wundergraph/go-arena"
)

// TestValueSizeUnchanged guards against accidental struct growth from the
// noEscapeSubtree field addition. Value must still fit its padding budget.
func TestValueSizeUnchanged(t *testing.T) {
	got := unsafe.Sizeof(Value{})
	// Current layout (amd64):
	//   t(1) + stringNeedsEscape(1) + noEscapeSubtree(1) + pad(5) +
	//   s(16) + a(24) + o(24) = 72
	// Accept 72 (new padding) or 80 (old padding preserved); reject growth above.
	if got > 80 {
		t.Fatalf("Value struct grew unexpectedly: got %d bytes", got)
	}
}

func TestNoEscapeSubtreeParseFixtures(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		wantOK bool
	}{
		{"empty object", `{}`, true},
		{"empty array", `[]`, true},
		{"plain scalars", `{"a":1,"b":true,"c":null,"d":"hello"}`, true},
		{"graphql-like response", `{"data":{"user":{"id":"u1","name":"Alice","tags":["a","b"]}}}`, true},
		{"nested clean", `{"a":{"b":{"c":[1,2,3]}}}`, true},
		{"string with newline", `{"a":"b\nc"}`, false},
		{"string with quote", `{"a":"b\"c"}`, false},
		{"key with newline", `{"a\nb":1}`, false},
		{"key with quote", `{"a\"b":1}`, false},
		{"nested dirty propagates", `{"a":{"b":"x\ny"}}`, false},
		{"dirty value in array", `{"a":[1,"x\ny",3]}`, false},
		{"array of plain strings", `["a","b","c"]`, true},
		{"array of numbers", `[1,2,3]`, true},
		{"int at top level", `42`, true},
		{"string at top level", `"hello"`, true},
		{"string with escape at top level", `"hi\n"`, false},
	}
	for _, tc := range tests {
		t.Run(tc.name+"/heap", func(t *testing.T) {
			var p Parser
			v, err := p.Parse(tc.input)
			if err != nil {
				t.Fatalf("Parse: %s", err)
			}
			if got := valueIsEscapeFree(v); got != tc.wantOK {
				t.Fatalf("heap parse %q: got noEscape=%v want %v", tc.input, got, tc.wantOK)
			}
		})
		t.Run(tc.name+"/twopass", func(t *testing.T) {
			a := arena.NewMonotonicArena()
			var p Parser
			v, err := p.ParseWithArena(a, tc.input)
			if err != nil {
				t.Fatalf("ParseWithArena: %s", err)
			}
			if got := valueIsEscapeFree(v); got != tc.wantOK {
				t.Fatalf("two-pass parse %q: got noEscape=%v want %v", tc.input, got, tc.wantOK)
			}
		})
	}
}

func TestNoEscapeSingletons(t *testing.T) {
	if !valueTrue.noEscapeSubtree {
		t.Fatal("valueTrue should be noEscape=true")
	}
	if !valueFalse.noEscapeSubtree {
		t.Fatal("valueFalse should be noEscape=true")
	}
	if !valueNull.noEscapeSubtree {
		t.Fatal("valueNull should be noEscape=true")
	}
}

// TestStructuralCopyWithTransformOutputKeyEscaping is a regression for the
// latent bug where newKV.keyNeedsEscape was hardcoded to false in
// structuralCopyWithTransformValue, causing MarshalTo to emit invalid JSON
// when the user-supplied OutputKey contained characters that need escaping.
func TestStructuralCopyWithTransformOutputKeyEscaping(t *testing.T) {
	a := arena.NewMonotonicArena()
	var p Parser
	src, err := p.ParseWithArena(a, `{"input":"hello"}`)
	if err != nil {
		t.Fatalf("parse: %s", err)
	}

	transform := &Transform{
		Entries: []TransformEntry{
			{InputKey: "input", OutputKey: `has"quote`},
		},
	}
	cp := p.StructuralCopyWithTransform(a, src, transform)

	out := string(cp.MarshalTo(nil))
	// Must be valid JSON — the OutputKey's `"` must be escaped.
	expected := `{"has\"quote":"hello"}`
	if out != expected {
		t.Fatalf("got %q want %q", out, expected)
	}
}

// TestMarshalFastPathEquivalence verifies the clean-subtree fast path in
// MarshalTo produces identical bytes to the slow path for escape-free
// payloads. For payloads that contain escapes, the fast path must not
// be taken at the top level (the flag will be false).
func TestMarshalFastPathEquivalence(t *testing.T) {
	fixtures := []string{
		`{}`,
		`[]`,
		`null`,
		`true`,
		`false`,
		`42`,
		`"hello"`,
		`{"a":1,"b":true,"c":null,"d":"hello"}`,
		`{"data":{"user":{"id":"u1","name":"Alice","tags":["a","b"]}}}`,
		`[{"k":"v"},{"k":"v2"},{"k":"v3"}]`,
		`{"a":"b\nc"}`,
		`{"a\"b":1}`,
		`{"a":{"b":{"c":"need\tescape"}}}`,
	}
	for _, fx := range fixtures {
		t.Run(fx, func(t *testing.T) {
			var p Parser
			v, err := p.Parse(fx)
			if err != nil {
				t.Fatalf("Parse: %s", err)
			}
			fast := string(v.MarshalTo(nil))
			// Force slow path by clearing the flag.
			origHint := v.noEscapeSubtree
			v.noEscapeSubtree = false
			slow := string(v.MarshalTo(nil))
			v.noEscapeSubtree = origHint
			if fast != slow {
				t.Fatalf("fast vs slow diverged\n  fast: %q\n  slow: %q", fast, slow)
			}
		})
	}
}

// TestMutationInvalidation confirms Value.Set / SetArrayItem / AppendArrayItems
// flip noEscapeSubtree to false on the directly-mutated node when the new
// content is not escape-free.
func TestMutationInvalidation(t *testing.T) {
	t.Run("Set with dirty key", func(t *testing.T) {
		a := arena.NewMonotonicArena()
		var p Parser
		v, _ := p.ParseWithArena(a, `{"a":1}`)
		if !v.noEscapeSubtree {
			t.Fatal("expected clean after parse")
		}
		v.Set(a, "k\nbad", IntValue(a, 2))
		if v.noEscapeSubtree {
			t.Fatal("expected dirty after Set with escape-requiring key")
		}
	})
	t.Run("Set with dirty value", func(t *testing.T) {
		a := arena.NewMonotonicArena()
		var p Parser
		v, _ := p.ParseWithArena(a, `{"a":1}`)
		v.Set(a, "k", StringValue(a, "v\nbad"))
		if v.noEscapeSubtree {
			t.Fatal("expected dirty after Set with escape-requiring value")
		}
	})
	t.Run("Set stays clean", func(t *testing.T) {
		a := arena.NewMonotonicArena()
		var p Parser
		v, _ := p.ParseWithArena(a, `{"a":1}`)
		v.Set(a, "b", IntValue(a, 2))
		if !v.noEscapeSubtree {
			t.Fatal("expected still clean after clean Set")
		}
	})
	t.Run("SetArrayItem with dirty value", func(t *testing.T) {
		a := arena.NewMonotonicArena()
		var p Parser
		v, _ := p.ParseWithArena(a, `[1,2,3]`)
		v.SetArrayItem(a, 0, StringValue(a, "v\nbad"))
		if v.noEscapeSubtree {
			t.Fatal("expected dirty after SetArrayItem with escape-requiring value")
		}
	})
	t.Run("AppendArrayItems with dirty", func(t *testing.T) {
		a := arena.NewMonotonicArena()
		var p Parser
		v, _ := p.ParseWithArena(a, `[1,2]`)
		rhs, _ := p.ParseWithArena(a, `[3,"x\ny"]`)
		v.AppendArrayItems(a, rhs)
		if v.noEscapeSubtree {
			t.Fatal("expected dirty after AppendArrayItems with escape-requiring element")
		}
	})
	t.Run("Del keeps clean", func(t *testing.T) {
		a := arena.NewMonotonicArena()
		var p Parser
		v, _ := p.ParseWithArena(a, `{"a":1,"b":2}`)
		v.Del("a")
		if !v.noEscapeSubtree {
			t.Fatal("Del should not dirty a clean subtree")
		}
	})
}

// TestMergeValuesAggregatesFlag verifies that MergeValues computes a correct
// post-merge flag — clean merges stay clean, dirty inputs make the result
// dirty.
func TestMergeValuesAggregatesFlag(t *testing.T) {
	t.Run("clean+clean=clean", func(t *testing.T) {
		a := MustParse(`{"a":1}`)
		b := MustParse(`{"b":2}`)
		m, err := MergeValues(nil, a, b)
		if err != nil {
			t.Fatalf("merge: %s", err)
		}
		if !m.noEscapeSubtree {
			t.Fatal("expected clean merge result")
		}
	})
	t.Run("clean+dirty=dirty", func(t *testing.T) {
		a := MustParse(`{"a":1}`)
		b := MustParse(`{"b":"has\nescape"}`)
		m, err := MergeValues(nil, a, b)
		if err != nil {
			t.Fatalf("merge: %s", err)
		}
		if m.noEscapeSubtree {
			t.Fatal("expected dirty merge result")
		}
	})
}

func TestRecomputeEscapeHint(t *testing.T) {
	var p Parser
	v, err := p.Parse(`{"a":{"b":"clean"},"c":[1,2,3]}`)
	if err != nil {
		t.Fatalf("parse: %s", err)
	}
	if !v.noEscapeSubtree {
		t.Fatal("expected clean subtree after parse")
	}
	// Tamper with a descendant string to mimic what a caller who mutated
	// through a sub-handle would leave behind.
	inner := v.Get("a", "b")
	inner.s = "a\nb"
	inner.stringNeedsEscape = true
	inner.noEscapeSubtree = false
	// Root's flag is stale-true.
	if !v.noEscapeSubtree {
		t.Fatal("root should still be stale-true before recompute")
	}
	if v.RecomputeEscapeHint() {
		t.Fatal("recompute should return false (dirty descendant)")
	}
	if v.noEscapeSubtree {
		t.Fatal("root flag should now be false")
	}
}
