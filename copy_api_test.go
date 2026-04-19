package astjson

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/wundergraph/go-arena"
)

// TestDeepCopyHeapIndependence verifies that DeepCopy with a nil arena
// produces a fully independent heap-allocated tree — mutating either side
// must not affect the other.
func TestDeepCopyHeapIndependence(t *testing.T) {
	src := MustParse(`{"a":"hello","b":[1,"two",{"c":true}],"d":{"e":42}}`)
	cp := DeepCopy(nil, src)

	require.NotSame(t, src, cp)
	require.Equal(t, src.MarshalTo(nil), cp.MarshalTo(nil))

	// Mutate the copy — source must be untouched.
	cp.Set(nil, "a", StringValue(nil, "mutated"))
	cp.Get("d").Set(nil, "e", IntValue(nil, 999))
	require.Equal(t, `{"a":"hello","b":[1,"two",{"c":true}],"d":{"e":42}}`, string(src.MarshalTo(nil)))

	// Mutate the source — copy must be untouched.
	src.Set(nil, "a", StringValue(nil, "src-changed"))
	require.Equal(t, `{"a":"mutated","b":[1,"two",{"c":true}],"d":{"e":999}}`, string(cp.MarshalTo(nil)))
}

// TestDeepCopyArenaIndependence is the arena-mode counterpart: a copy on a
// fresh arena must not share backing memory with the source.
func TestDeepCopyArenaIndependence(t *testing.T) {
	srcArena := arena.NewMonotonicArena()
	var p Parser
	src, err := p.ParseWithArena(srcArena, `{"a":"hello","b":[1,2,3]}`)
	require.NoError(t, err)

	dstArena := arena.NewMonotonicArena()
	cp := DeepCopy(dstArena, src)

	require.Equal(t, src.MarshalTo(nil), cp.MarshalTo(nil))
	cp.Set(dstArena, "a", StringValue(dstArena, "changed"))
	require.Equal(t, `{"a":"hello","b":[1,2,3]}`, string(src.MarshalTo(nil)))
	require.Equal(t, `{"a":"changed","b":[1,2,3]}`, string(cp.MarshalTo(nil)))
}

// TestDeepCopySingletonsShared verifies the immutable true/false/null
// singletons are shared even under DeepCopy.
func TestDeepCopySingletonsShared(t *testing.T) {
	require.Same(t, valueTrue, DeepCopy(nil, valueTrue))
	require.Same(t, valueFalse, DeepCopy(nil, valueFalse))
	require.Same(t, valueNull, DeepCopy(nil, valueNull))
	a := arena.NewMonotonicArena()
	require.Same(t, valueTrue, DeepCopy(a, valueTrue))
}

// TestDeepCopyNil returns nil for nil input regardless of arena.
func TestDeepCopyNil(t *testing.T) {
	require.Nil(t, DeepCopy(nil, nil))
	a := arena.NewMonotonicArena()
	require.Nil(t, DeepCopy(a, nil))
}

// TestDeepCopyWithTransformBasics covers the matrix of Transform features:
// rename via Entries, Passthrough, nested Child, and ArrayItem — in both
// arena and heap modes.
func TestDeepCopyWithTransformBasics(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		xform    *Transform
		expected string
	}{
		{
			name:  "rename only",
			input: `{"a":1,"b":2}`,
			xform: &Transform{
				Entries: []TransformEntry{
					{InputKey: "a", OutputKey: "alpha"},
				},
			},
			expected: `{"alpha":1}`,
		},
		{
			name:  "rename with passthrough",
			input: `{"a":1,"b":2,"c":3}`,
			xform: &Transform{
				Entries: []TransformEntry{
					{InputKey: "a", OutputKey: "alpha"},
				},
				Passthrough: true,
			},
			expected: `{"alpha":1,"b":2,"c":3}`,
		},
		{
			name:  "nested child",
			input: `{"user":{"name":"Alice","email":"a@b.c"}}`,
			xform: &Transform{
				Entries: []TransformEntry{
					{
						InputKey: "user", OutputKey: "u",
						Child: &Transform{
							Entries: []TransformEntry{
								{InputKey: "name", OutputKey: "n"},
							},
						},
					},
				},
			},
			expected: `{"u":{"n":"Alice"}}`,
		},
		{
			name:  "array item transform",
			input: `{"users":[{"a":1,"b":2},{"a":3,"b":4}]}`,
			xform: &Transform{
				Entries: []TransformEntry{
					{
						InputKey: "users", OutputKey: "users",
						Child: &Transform{
							ArrayItem: &Transform{
								Entries: []TransformEntry{
									{InputKey: "a", OutputKey: "a"},
								},
							},
						},
					},
				},
			},
			expected: `{"users":[{"a":1},{"a":3}]}`,
		},
		{
			name:  "empty entries, no passthrough filters everything",
			input: `{"a":1,"b":2}`,
			xform: &Transform{},
			expected: `{}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name+"/heap", func(t *testing.T) {
			src := MustParse(tc.input)
			cp := DeepCopyWithTransform(nil, src, tc.xform)
			require.Equal(t, tc.expected, string(cp.MarshalTo(nil)))
			// Source must be untouched.
			require.Equal(t, tc.input, string(src.MarshalTo(nil)))
		})
		t.Run(tc.name+"/arena", func(t *testing.T) {
			a := arena.NewMonotonicArena()
			var p Parser
			src, err := p.ParseWithArena(a, tc.input)
			require.NoError(t, err)
			cp := DeepCopyWithTransform(a, src, tc.xform)
			require.Equal(t, tc.expected, string(cp.MarshalTo(nil)))
		})
	}
}

// TestDeepCopyWithTransformIndependence verifies that a transform deep copy
// in heap mode is fully independent — mutating the copy must not reach back
// through a shared scalar to the source.
func TestDeepCopyWithTransformIndependence(t *testing.T) {
	src := MustParse(`{"name":"Alice","age":30}`)
	xform := &Transform{
		Entries:     []TransformEntry{{InputKey: "name", OutputKey: "n"}},
		Passthrough: true,
	}
	cp := DeepCopyWithTransform(nil, src, xform)
	cp.Set(nil, "n", StringValue(nil, "Bob"))
	cp.Set(nil, "age", IntValue(nil, 40))

	require.Equal(t, `{"name":"Alice","age":30}`, string(src.MarshalTo(nil)))
	require.Equal(t, `{"n":"Bob","age":40}`, string(cp.MarshalTo(nil)))
}

// TestDeepCopyWithTransformNilTransform delegates to DeepCopy semantics.
func TestDeepCopyWithTransformNilTransform(t *testing.T) {
	src := MustParse(`{"a":1,"b":2}`)
	cp := DeepCopyWithTransform(nil, src, nil)
	require.NotSame(t, src, cp)
	require.Equal(t, string(src.MarshalTo(nil)), string(cp.MarshalTo(nil)))
}

// TestStructuralCopyHeapAliasesScalars confirms that heap-mode structural
// copy still aliases scalar leaves — matching the arena-mode contract.
// Container nodes are independent (Set on copy does not mutate source).
func TestStructuralCopyHeapAliasesScalars(t *testing.T) {
	src := MustParse(`{"s":"leaf","n":42,"b":true,"o":{"k":"v"}}`)
	cp := StructuralCopy(nil, src)

	require.NotSame(t, src, cp)

	// Scalar leaves are aliased — same pointer in source and copy.
	require.Same(t, src.Get("s"), cp.Get("s"))
	require.Same(t, src.Get("n"), cp.Get("n"))
	require.Same(t, src.Get("b"), cp.Get("b"))

	// Container children are not aliased — they were freshly allocated.
	require.NotSame(t, src.Get("o"), cp.Get("o"))

	// Restructuring the copy must not touch the source.
	cp.Del("s")
	require.NotNil(t, src.Get("s"))
	require.Nil(t, cp.Get("s"))
}

// TestStructuralCopyWithTransformHeapAliasesScalars mirrors the test above
// but through the transform entry point.
func TestStructuralCopyWithTransformHeapAliasesScalars(t *testing.T) {
	src := MustParse(`{"a":"val","b":"other"}`)
	xform := &Transform{
		Entries: []TransformEntry{
			{InputKey: "a", OutputKey: "alpha"},
		},
		Passthrough: true,
	}
	cp := StructuralCopyWithTransform(nil, src, xform)

	require.NotSame(t, src, cp)
	require.Equal(t, `{"alpha":"val","b":"other"}`, string(cp.MarshalTo(nil)))

	// The scalar value under "alpha" in the copy is the SAME pointer as
	// the scalar under "a" in the source (aliased by structural semantics).
	require.Same(t, src.Get("a"), cp.Get("alpha"))
}

// TestDeepCopyWithTransformPassthroughCollision covers the case where a
// rename's OutputKey also exists as a source field. The passthrough pass
// must drop that source field (rename-wins) AND the planner must not
// over-reserve slab positions for it — otherwise sibling subtrees read
// misaligned objectSizes/arraySizes and panic.
func TestDeepCopyWithTransformPassthroughCollision(t *testing.T) {
	// Case A: simple collision at a single object level.
	t.Run("simple", func(t *testing.T) {
		src := MustParse(`{"a":{"x":1},"b":{"y":2},"c":3}`)
		xform := &Transform{
			Entries:     []TransformEntry{{InputKey: "a", OutputKey: "b"}},
			Passthrough: true,
		}
		expected := `{"b":{"x":1},"c":3}`
		// Heap.
		cpHeap := DeepCopyWithTransform(nil, src, xform)
		require.Equal(t, expected, string(cpHeap.MarshalTo(nil)))
		// Arena.
		a := arena.NewMonotonicArena()
		var p Parser
		srcA, err := p.ParseWithArena(a, `{"a":{"x":1},"b":{"y":2},"c":3}`)
		require.NoError(t, err)
		cpArena := DeepCopyWithTransform(a, srcA, xform)
		require.Equal(t, expected, string(cpArena.MarshalTo(nil)))
	})

	// Case B: collision inside a nested child transform, with a sibling
	// under the outer object. This is the exact scenario codex flagged:
	// the dropped passthrough subtree used to leave objectSizes entries
	// that misaligned the sibling copy.
	t.Run("nested_with_sibling", func(t *testing.T) {
		input := `{"x":{"a":{"aa":1},"b":{"drop":1},"c":{"w1":1,"w2":2,"w3":3}},"y":{"later1":1,"later2":2}}`
		xform := &Transform{
			Entries: []TransformEntry{
				{
					InputKey: "x", OutputKey: "x",
					Child: &Transform{
						Entries:     []TransformEntry{{InputKey: "a", OutputKey: "b"}},
						Passthrough: true,
					},
				},
				{InputKey: "y", OutputKey: "y"},
			},
		}
		// Dropped: x.b (collides with emitted x.b from x.a rename).
		// Preserved via passthrough: x.c.
		// Sibling y fully deep-copied.
		expected := `{"x":{"b":{"aa":1},"c":{"w1":1,"w2":2,"w3":3}},"y":{"later1":1,"later2":2}}`
		t.Run("heap", func(t *testing.T) {
			src := MustParse(input)
			cp := DeepCopyWithTransform(nil, src, xform)
			require.Equal(t, expected, string(cp.MarshalTo(nil)))
		})
		t.Run("arena", func(t *testing.T) {
			a := arena.NewMonotonicArena()
			var p Parser
			src, err := p.ParseWithArena(a, input)
			require.NoError(t, err)
			cp := DeepCopyWithTransform(a, src, xform)
			require.Equal(t, expected, string(cp.MarshalTo(nil)))
		})
	})
}

// TestDeepCopyWithTransformPassthroughDuplicateKeys covers the case where
// a source object has two kvs with the same key and Passthrough is on. The
// fill drops the later duplicate via refs lookup; the planner must not
// over-count it, or sibling-slab positions drift and marshal panics.
func TestDeepCopyWithTransformPassthroughDuplicateKeys(t *testing.T) {
	input := `{"y":false,"y":[1,2,3],"b":[4]}`
	xform := &Transform{
		Entries:     []TransformEntry{{InputKey: "name", OutputKey: "age"}},
		Passthrough: true,
	}
	// Only the first "y" survives (first-wins on duplicates, matches fill).
	expected := `{"y":false,"b":[4]}`

	t.Run("heap", func(t *testing.T) {
		cp := DeepCopyWithTransform(nil, MustParse(input), xform)
		require.Equal(t, expected, string(cp.MarshalTo(nil)))
	})
	t.Run("arena", func(t *testing.T) {
		a := arena.NewMonotonicArena()
		var p Parser
		src, err := p.ParseWithArena(a, input)
		require.NoError(t, err)
		cp := DeepCopyWithTransform(a, src, xform)
		require.Equal(t, expected, string(cp.MarshalTo(nil)))
	})
	t.Run("structural arena", func(t *testing.T) {
		a := arena.NewMonotonicArena()
		var p Parser
		src, err := p.ParseWithArena(a, input)
		require.NoError(t, err)
		cp := StructuralCopyWithTransform(a, src, xform)
		require.Equal(t, expected, string(cp.MarshalTo(nil)))
	})
}

// TestStructuralCopyWithTransformPassthroughCollision mirrors the above for
// the structural copy path; the same planner-vs-fill mismatch existed
// there before the fix.
func TestStructuralCopyWithTransformPassthroughCollision(t *testing.T) {
	input := `{"x":{"a":{"aa":1},"b":{"drop":1},"c":{"w1":1,"w2":2,"w3":3}},"y":{"later1":1,"later2":2}}`
	xform := &Transform{
		Entries: []TransformEntry{
			{
				InputKey: "x", OutputKey: "x",
				Child: &Transform{
					Entries:     []TransformEntry{{InputKey: "a", OutputKey: "b"}},
					Passthrough: true,
				},
			},
			{InputKey: "y", OutputKey: "y"},
		},
	}
	expected := `{"x":{"b":{"aa":1},"c":{"w1":1,"w2":2,"w3":3}},"y":{"later1":1,"later2":2}}`
	t.Run("heap", func(t *testing.T) {
		src := MustParse(input)
		cp := StructuralCopyWithTransform(nil, src, xform)
		require.Equal(t, expected, string(cp.MarshalTo(nil)))
	})
	t.Run("arena", func(t *testing.T) {
		a := arena.NewMonotonicArena()
		var p Parser
		src, err := p.ParseWithArena(a, input)
		require.NoError(t, err)
		cp := StructuralCopyWithTransform(a, src, xform)
		require.Equal(t, expected, string(cp.MarshalTo(nil)))
	})
}

// TestPackageLevelParity ensures the package-level funcs produce byte-
// identical output to the Parser methods for a representative fixture —
// they must share internals.
func TestPackageLevelParity(t *testing.T) {
	src := MustParse(`{"data":{"user":{"id":"u1","name":"Alice","tags":["a","b"]}}}`)
	var p Parser

	t.Run("DeepCopy heap", func(t *testing.T) {
		a := p.DeepCopy(nil, src)
		b := DeepCopy(nil, src)
		require.Equal(t, string(a.MarshalTo(nil)), string(b.MarshalTo(nil)))
	})
	t.Run("DeepCopy arena", func(t *testing.T) {
		ar := arena.NewMonotonicArena()
		a := p.DeepCopy(ar, src)
		b := DeepCopy(ar, src)
		require.Equal(t, string(a.MarshalTo(nil)), string(b.MarshalTo(nil)))
	})
	t.Run("StructuralCopy heap", func(t *testing.T) {
		a := p.StructuralCopy(nil, src)
		b := StructuralCopy(nil, src)
		require.Equal(t, string(a.MarshalTo(nil)), string(b.MarshalTo(nil)))
	})
}
