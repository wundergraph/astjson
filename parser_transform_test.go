package astjson

import (
	"testing"
	"unsafe"

	"github.com/stretchr/testify/require"
	"github.com/wundergraph/go-arena"
)

// ---------------------------------------------------------------------------
// Section 1 — Nil / no-op handling
// ---------------------------------------------------------------------------

func TestTransform_NilTransform_BehavesAsStructuralCopy(t *testing.T) {
	a := arena.NewMonotonicArena()
	var p Parser

	src, err := p.ParseWithArena(a, `{"id":"1","name":"Alice"}`)
	require.NoError(t, err)

	cp := p.StructuralCopyWithTransform(a, src, nil)

	require.Equal(t, `{"id":"1","name":"Alice"}`, string(cp.MarshalTo(nil)))
	require.NotSame(t, src, cp)
	require.Same(t, src.Get("id"), cp.Get("id"))
}

func TestTransform_NilValue_ReturnsNil(t *testing.T) {
	a := arena.NewMonotonicArena()
	var p Parser
	xform := &Transform{Entries: []TransformEntry{{InputKey: "id", OutputKey: "id"}}}
	require.Nil(t, p.StructuralCopyWithTransform(a, nil, xform))
}

func TestTransform_NilArena_AllocatesOnHeap(t *testing.T) {
	v := MustParse(`{"nickname":"Alice"}`)
	xform := &Transform{Entries: []TransformEntry{{InputKey: "nickname", OutputKey: "name"}}}
	var p Parser

	cp := p.StructuralCopyWithTransform(nil, v, xform)

	require.NotSame(t, v, cp)
	require.Equal(t, `{"name":"Alice"}`, string(cp.MarshalTo(nil)))
}

func TestTransform_EmptyTransform_OnObject_ProducesEmptyObject(t *testing.T) {
	a := arena.NewMonotonicArena()
	var p Parser

	src, err := p.ParseWithArena(a, `{"a":1,"b":2,"c":3}`)
	require.NoError(t, err)

	// Non-nil transform with no Entries and Passthrough=false → drops everything.
	cp := p.StructuralCopyWithTransform(a, src, &Transform{})

	require.Equal(t, `{}`, string(cp.MarshalTo(nil)))
}

func TestTransform_EmptyTransform_OnArray_CopiesArrayVerbatim(t *testing.T) {
	a := arena.NewMonotonicArena()
	var p Parser

	src, err := p.ParseWithArena(a, `[1,2,3]`)
	require.NoError(t, err)

	// Transform on array without ArrayItem → plain structural copy of elements.
	cp := p.StructuralCopyWithTransform(a, src, &Transform{})

	require.Equal(t, `[1,2,3]`, string(cp.MarshalTo(nil)))
	require.NotSame(t, src, cp)
}

// ---------------------------------------------------------------------------
// Section 2 — Scalar inputs pass through
// ---------------------------------------------------------------------------

func TestTransform_ScalarInputs_TransformIgnored_ValueAliased(t *testing.T) {
	cases := []struct {
		name string
		json string
	}{
		{"string", `"hello"`},
		{"number", `42`},
		{"true", `true`},
		{"false", `false`},
		{"null", `null`},
	}
	xform := &Transform{Entries: []TransformEntry{{InputKey: "x", OutputKey: "y"}}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := arena.NewMonotonicArena()
			var p Parser
			src, err := p.ParseWithArena(a, tc.json)
			require.NoError(t, err)

			cp := p.StructuralCopyWithTransform(a, src, xform)

			require.Same(t, src, cp, "scalar must be aliased (transform has no effect)")
		})
	}
}

// ---------------------------------------------------------------------------
// Section 3 — Entries: projection & renaming
// ---------------------------------------------------------------------------

func TestTransform_Entries_SingleRename(t *testing.T) {
	a := arena.NewMonotonicArena()
	var p Parser

	src, err := p.ParseWithArena(a, `{"nickname":"Alice"}`)
	require.NoError(t, err)

	cp := p.StructuralCopyWithTransform(a, src, &Transform{
		Entries: []TransformEntry{{InputKey: "nickname", OutputKey: "name"}},
	})

	require.Equal(t, `{"name":"Alice"}`, string(cp.MarshalTo(nil)))
}

func TestTransform_Entries_MultipleRenamesPreserveOrder(t *testing.T) {
	a := arena.NewMonotonicArena()
	var p Parser

	// Source fields appear in c, a, b order; Entries list them as a, b, c.
	// Output must follow the Entries order.
	src, err := p.ParseWithArena(a, `{"c":3,"a":1,"b":2}`)
	require.NoError(t, err)

	cp := p.StructuralCopyWithTransform(a, src, &Transform{
		Entries: []TransformEntry{
			{InputKey: "a", OutputKey: "first"},
			{InputKey: "b", OutputKey: "second"},
			{InputKey: "c", OutputKey: "third"},
		},
	})

	require.Equal(t, `{"first":1,"second":2,"third":3}`, string(cp.MarshalTo(nil)))
}

func TestTransform_Entries_IdentityKey(t *testing.T) {
	a := arena.NewMonotonicArena()
	var p Parser

	src, err := p.ParseWithArena(a, `{"id":"1","name":"Alice"}`)
	require.NoError(t, err)

	cp := p.StructuralCopyWithTransform(a, src, &Transform{
		Entries: []TransformEntry{
			{InputKey: "id", OutputKey: "id"},
			{InputKey: "name", OutputKey: "name"},
		},
	})

	require.Equal(t, `{"id":"1","name":"Alice"}`, string(cp.MarshalTo(nil)))
}

func TestTransform_Entries_MissingSourceFieldSkipped(t *testing.T) {
	a := arena.NewMonotonicArena()
	var p Parser

	src, err := p.ParseWithArena(a, `{"id":"1"}`)
	require.NoError(t, err)

	cp := p.StructuralCopyWithTransform(a, src, &Transform{
		Entries: []TransformEntry{
			{InputKey: "id", OutputKey: "id"},
			{InputKey: "missing", OutputKey: "missing"},
		},
	})

	require.Equal(t, `{"id":"1"}`, string(cp.MarshalTo(nil)))
}

func TestTransform_Entries_UnlistedFieldsDroppedByDefault(t *testing.T) {
	a := arena.NewMonotonicArena()
	var p Parser

	src, err := p.ParseWithArena(a, `{"id":"1","secret":"hunter2","extra":"x"}`)
	require.NoError(t, err)

	cp := p.StructuralCopyWithTransform(a, src, &Transform{
		Entries: []TransformEntry{{InputKey: "id", OutputKey: "id"}},
	})

	require.Equal(t, `{"id":"1"}`, string(cp.MarshalTo(nil)))
	require.Nil(t, cp.Get("secret"))
	require.Nil(t, cp.Get("extra"))
}

// Documents current behavior: listing the same InputKey twice in Entries
// produces two entries in the output (no dedup). Callers must ensure keys
// are unique.
func TestTransform_Entries_DuplicateInputKey_EmitsDuplicates(t *testing.T) {
	a := arena.NewMonotonicArena()
	var p Parser

	src, err := p.ParseWithArena(a, `{"id":"1"}`)
	require.NoError(t, err)

	cp := p.StructuralCopyWithTransform(a, src, &Transform{
		Entries: []TransformEntry{
			{InputKey: "id", OutputKey: "a"},
			{InputKey: "id", OutputKey: "b"},
		},
	})

	require.Equal(t, `{"a":"1","b":"1"}`, string(cp.MarshalTo(nil)))
}

// ---------------------------------------------------------------------------
// Section 5 — Passthrough
// ---------------------------------------------------------------------------

func TestTransform_Passthrough_AloneCopiesAllVerbatim(t *testing.T) {
	a := arena.NewMonotonicArena()
	var p Parser

	src, err := p.ParseWithArena(a, `{"a":1,"b":"x","c":true}`)
	require.NoError(t, err)

	cp := p.StructuralCopyWithTransform(a, src, &Transform{Passthrough: true})

	require.Equal(t, `{"a":1,"b":"x","c":true}`, string(cp.MarshalTo(nil)))
	require.NotSame(t, src, cp)
}

func TestTransform_Passthrough_WithEntries_ListedRenamedRestVerbatim(t *testing.T) {
	a := arena.NewMonotonicArena()
	var p Parser

	src, err := p.ParseWithArena(a, `{"nickname":"Alice","age":30,"email":"a@b.com"}`)
	require.NoError(t, err)

	cp := p.StructuralCopyWithTransform(a, src, &Transform{
		Entries:     []TransformEntry{{InputKey: "nickname", OutputKey: "name"}},
		Passthrough: true,
	})

	// Renamed field appears first (from Entries), then unlisted fields in source order.
	require.Equal(t, `{"name":"Alice","age":30,"email":"a@b.com"}`, string(cp.MarshalTo(nil)))
}

func TestTransform_Passthrough_EntriesShadowSourceKey_NoDuplicate(t *testing.T) {
	a := arena.NewMonotonicArena()
	var p Parser

	src, err := p.ParseWithArena(a, `{"name":"Alice","age":30}`)
	require.NoError(t, err)

	// Entries renames "name" → "displayName". Passthrough copies unlisted fields.
	// The source "name" key must NOT also appear verbatim in the passthrough output
	// because it was already consumed by Entries.
	cp := p.StructuralCopyWithTransform(a, src, &Transform{
		Entries:     []TransformEntry{{InputKey: "name", OutputKey: "displayName"}},
		Passthrough: true,
	})

	require.Equal(t, `{"displayName":"Alice","age":30}`, string(cp.MarshalTo(nil)))
}

// Rename wins: if an Entries OutputKey collides with a separate unlisted
// source field, the rename is authoritative and the source field is dropped.
// Emitting both would produce JSON with duplicate keys, which most consumers
// collapse to "last wins" — silently shadowing the explicit rename.
func TestTransform_Passthrough_EntriesOutputKeyCollision_RenameWins(t *testing.T) {
	a := arena.NewMonotonicArena()
	var p Parser

	src, err := p.ParseWithArena(a, `{"name":"Alice","displayName":"old"}`)
	require.NoError(t, err)

	cp := p.StructuralCopyWithTransform(a, src, &Transform{
		Entries:     []TransformEntry{{InputKey: "name", OutputKey: "displayName"}},
		Passthrough: true,
	})

	require.Equal(t, `{"displayName":"Alice"}`, string(cp.MarshalTo(nil)))
}

// Key swap: Entries {A→B, B→A} on source {A, B} must produce a proper swap
// with no duplicates. Both slots are filled by Entries; Passthrough skips
// both source fields (A is consumed, B is consumed).
func TestTransform_Passthrough_SwapKeys(t *testing.T) {
	a := arena.NewMonotonicArena()
	var p Parser

	src, err := p.ParseWithArena(a, `{"a":1,"b":2}`)
	require.NoError(t, err)

	cp := p.StructuralCopyWithTransform(a, src, &Transform{
		Entries: []TransformEntry{
			{InputKey: "a", OutputKey: "b"},
			{InputKey: "b", OutputKey: "a"},
		},
		Passthrough: true,
	})

	require.Equal(t, `{"b":1,"a":2}`, string(cp.MarshalTo(nil)))
}

// When Entries reserves an OutputKey but can't emit it (source is missing
// the InputKey), the slot isn't claimed — so Passthrough is free to emit
// a source field of the same name verbatim. This avoids hiding data that
// the caller plainly expected to flow through.
func TestTransform_Passthrough_OutputKeyReservedButNotEmitted_SourcePassesThrough(t *testing.T) {
	a := arena.NewMonotonicArena()
	var p Parser

	src, err := p.ParseWithArena(a, `{"displayName":"old"}`)
	require.NoError(t, err)

	cp := p.StructuralCopyWithTransform(a, src, &Transform{
		// Source has no "name", so this entry emits nothing and does not
		// reserve the "displayName" slot.
		Entries:     []TransformEntry{{InputKey: "name", OutputKey: "displayName"}},
		Passthrough: true,
	})

	require.Equal(t, `{"displayName":"old"}`, string(cp.MarshalTo(nil)))
}

// ---------------------------------------------------------------------------
// Section 6 — Child (nested transforms)
// ---------------------------------------------------------------------------

func TestTransform_Child_NestedObjectRenamed(t *testing.T) {
	a := arena.NewMonotonicArena()
	var p Parser

	src, err := p.ParseWithArena(a, `{"usr":{"handle":"Alice","__typename":"User"}}`)
	require.NoError(t, err)

	cp := p.StructuralCopyWithTransform(a, src, &Transform{
		Entries: []TransformEntry{
			{InputKey: "usr", OutputKey: "user", Child: &Transform{
				Entries: []TransformEntry{
					{InputKey: "handle", OutputKey: "name"},
					{InputKey: "__typename", OutputKey: "__typename"},
				},
			}},
		},
	})

	require.Equal(t, `{"user":{"name":"Alice","__typename":"User"}}`, string(cp.MarshalTo(nil)))
	require.NotSame(t, src.Get("usr"), cp.Get("user"))
}

func TestTransform_Child_NilChild_PlainCopiesInner(t *testing.T) {
	a := arena.NewMonotonicArena()
	var p Parser

	src, err := p.ParseWithArena(a, `{"usr":{"id":"1","name":"Alice","email":"a@b.com"}}`)
	require.NoError(t, err)

	// Outer renames "usr" → "user" but Child is nil: inner object copied verbatim.
	cp := p.StructuralCopyWithTransform(a, src, &Transform{
		Entries: []TransformEntry{{InputKey: "usr", OutputKey: "user", Child: nil}},
	})

	require.Equal(t, `{"user":{"id":"1","name":"Alice","email":"a@b.com"}}`, string(cp.MarshalTo(nil)))
}

func TestTransform_Child_OnScalarField_Ignored(t *testing.T) {
	a := arena.NewMonotonicArena()
	var p Parser

	src, err := p.ParseWithArena(a, `{"id":"1"}`)
	require.NoError(t, err)

	// id is a string; a Child transform on a scalar has no effect.
	cp := p.StructuralCopyWithTransform(a, src, &Transform{
		Entries: []TransformEntry{
			{InputKey: "id", OutputKey: "id", Child: &Transform{
				Entries: []TransformEntry{{InputKey: "x", OutputKey: "y"}},
			}},
		},
	})

	require.Equal(t, `{"id":"1"}`, string(cp.MarshalTo(nil)))
}

func TestTransform_Child_DeepNestingThreeLevels(t *testing.T) {
	a := arena.NewMonotonicArena()
	var p Parser

	src, err := p.ParseWithArena(a, `{"a":{"b":{"c":{"old":"v"}}}}`)
	require.NoError(t, err)

	cp := p.StructuralCopyWithTransform(a, src, &Transform{
		Entries: []TransformEntry{
			{InputKey: "a", OutputKey: "a", Child: &Transform{
				Entries: []TransformEntry{
					{InputKey: "b", OutputKey: "b", Child: &Transform{
						Entries: []TransformEntry{
							{InputKey: "c", OutputKey: "c", Child: &Transform{
								Entries: []TransformEntry{{InputKey: "old", OutputKey: "new"}},
							}},
						},
					}},
				},
			}},
		},
	})

	require.Equal(t, `{"a":{"b":{"c":{"new":"v"}}}}`, string(cp.MarshalTo(nil)))
}

// ---------------------------------------------------------------------------
// Section 7 — ArrayItem
// ---------------------------------------------------------------------------

func TestTransform_ArrayItem_TransformsEachElement(t *testing.T) {
	a := arena.NewMonotonicArena()
	var p Parser

	src, err := p.ParseWithArena(a, `{"users":[{"handle":"Alice"},{"handle":"Bob"}]}`)
	require.NoError(t, err)

	cp := p.StructuralCopyWithTransform(a, src, &Transform{
		Entries: []TransformEntry{
			{InputKey: "users", OutputKey: "users", Child: &Transform{
				ArrayItem: &Transform{
					Entries: []TransformEntry{{InputKey: "handle", OutputKey: "name"}},
				},
			}},
		},
	})

	require.Equal(t, `{"users":[{"name":"Alice"},{"name":"Bob"}]}`, string(cp.MarshalTo(nil)))
}

func TestTransform_ArrayItem_EmptyArrayRemainsEmpty(t *testing.T) {
	a := arena.NewMonotonicArena()
	var p Parser

	src, err := p.ParseWithArena(a, `{"users":[]}`)
	require.NoError(t, err)

	cp := p.StructuralCopyWithTransform(a, src, &Transform{
		Entries: []TransformEntry{
			{InputKey: "users", OutputKey: "users", Child: &Transform{
				ArrayItem: &Transform{
					Entries: []TransformEntry{{InputKey: "handle", OutputKey: "name"}},
				},
			}},
		},
	})

	require.Equal(t, `{"users":[]}`, string(cp.MarshalTo(nil)))
}

func TestTransform_ArrayItem_NestedArrayOfArrays(t *testing.T) {
	a := arena.NewMonotonicArena()
	var p Parser

	src, err := p.ParseWithArena(a, `{"grid":[[{"v":1},{"v":2}],[{"v":3}]]}`)
	require.NoError(t, err)

	cp := p.StructuralCopyWithTransform(a, src, &Transform{
		Entries: []TransformEntry{
			{InputKey: "grid", OutputKey: "grid", Child: &Transform{
				ArrayItem: &Transform{
					ArrayItem: &Transform{
						Entries: []TransformEntry{{InputKey: "v", OutputKey: "value"}},
					},
				},
			}},
		},
	})

	require.Equal(t, `{"grid":[[{"value":1},{"value":2}],[{"value":3}]]}`, string(cp.MarshalTo(nil)))
}

func TestTransform_ArrayItem_OnRootArray(t *testing.T) {
	a := arena.NewMonotonicArena()
	var p Parser

	src, err := p.ParseWithArena(a, `[{"handle":"Alice"},{"handle":"Bob"}]`)
	require.NoError(t, err)

	cp := p.StructuralCopyWithTransform(a, src, &Transform{
		ArrayItem: &Transform{
			Entries: []TransformEntry{{InputKey: "handle", OutputKey: "name"}},
		},
	})

	require.Equal(t, `[{"name":"Alice"},{"name":"Bob"}]`, string(cp.MarshalTo(nil)))
}

// ---------------------------------------------------------------------------
// Section 8 — Realistic GraphQL scenarios
// ---------------------------------------------------------------------------

// A GraphQL query with argument-aware aliases: the cache stores `name` under
// a hashed key so concurrent queries with different arguments don't collide.
// StructuralCopyWithTransform maps cache keys back to output aliases. Callers
// that need __typename preserved append an identity entry themselves.
func TestTransform_GraphQL_EntityCacheWithHashedAliases(t *testing.T) {
	a := arena.NewMonotonicArena()
	var p Parser

	src, err := p.ParseWithArena(a,
		`{"id":"u-1","name_a6b3c":"Alice","name_f19d2":"alice@example.com","__typename":"User"}`)
	require.NoError(t, err)

	cp := p.StructuralCopyWithTransform(a, src, &Transform{
		Entries: []TransformEntry{
			{InputKey: "id", OutputKey: "id"},
			{InputKey: "name_a6b3c", OutputKey: "displayName"},
			{InputKey: "name_f19d2", OutputKey: "email"},
			{InputKey: "__typename", OutputKey: "__typename"},
		},
	})

	require.Equal(t,
		`{"id":"u-1","displayName":"Alice","email":"alice@example.com","__typename":"User"}`,
		string(cp.MarshalTo(nil)))
}

// Composite scenario exercising Entries × Child × ArrayItem together,
// matching the shape of a batched entity resolution in the GraphQL resolver.
// __typename preservation is owned by the caller: it appends an identity
// Entries entry at each level where the invariant must hold.
func TestTransform_GraphQL_BatchOfEntitiesWithNestedFields(t *testing.T) {
	a := arena.NewMonotonicArena()
	var p Parser

	src, err := p.ParseWithArena(a, `{
		"data": {
			"user": {
				"nickname": "alice",
				"posts": [
					{"title": "hello", "body": "lorem", "__typename": "Post"},
					{"title": "world", "body": "ipsum", "__typename": "Post"}
				],
				"__typename": "User"
			},
			"__typename": "Query"
		}
	}`)
	require.NoError(t, err)

	typename := TransformEntry{InputKey: "__typename", OutputKey: "__typename"}

	cp := p.StructuralCopyWithTransform(a, src, &Transform{
		Entries: []TransformEntry{
			{InputKey: "data", OutputKey: "data", Child: &Transform{
				Entries: []TransformEntry{
					{InputKey: "user", OutputKey: "user", Child: &Transform{
						Entries: []TransformEntry{
							{InputKey: "nickname", OutputKey: "name"},
							{InputKey: "posts", OutputKey: "posts", Child: &Transform{
								ArrayItem: &Transform{
									Entries: []TransformEntry{
										{InputKey: "title", OutputKey: "title"},
										typename,
									},
								},
							}},
							typename,
						},
					}},
					typename,
				},
			}},
		},
	})

	require.Equal(t,
		`{"data":{"user":{"name":"alice","posts":[{"title":"hello","__typename":"Post"},{"title":"world","__typename":"Post"}],"__typename":"User"},"__typename":"Query"}}`,
		string(cp.MarshalTo(nil)))
}

// Union member caching: the resolver doesn't know the concrete shape of each
// union entity, so it uses Passthrough=true to forward all fields. The caller
// still appends an identity __typename entry so the slot is claimed first and
// type discrimination is available even when the source is missing __typename.
func TestTransform_GraphQL_UnionTypePassthrough(t *testing.T) {
	a := arena.NewMonotonicArena()
	var p Parser

	src, err := p.ParseWithArena(a,
		`{"id":"m-1","title":"A Post","author":"Alice","__typename":"Post"}`)
	require.NoError(t, err)

	cp := p.StructuralCopyWithTransform(a, src, &Transform{
		Entries:     []TransformEntry{{InputKey: "__typename", OutputKey: "__typename"}},
		Passthrough: true,
	})

	require.Equal(t,
		`{"__typename":"Post","id":"m-1","title":"A Post","author":"Alice"}`,
		string(cp.MarshalTo(nil)))
}

// ---------------------------------------------------------------------------
// Section 9 — Memory & aliasing semantics
// ---------------------------------------------------------------------------

func TestTransform_LeafValuesAliasedFromSource(t *testing.T) {
	a := arena.NewMonotonicArena()
	var p Parser

	src, err := p.ParseWithArena(a, `{"nickname":"Alice","age":30,"__typename":"User"}`)
	require.NoError(t, err)

	cp := p.StructuralCopyWithTransform(a, src, &Transform{
		Entries: []TransformEntry{
			{InputKey: "nickname", OutputKey: "name"},
			{InputKey: "age", OutputKey: "age"},
			{InputKey: "__typename", OutputKey: "__typename"},
		},
	})

	require.Same(t, src.Get("nickname"), cp.Get("name"))
	require.Same(t, src.Get("age"), cp.Get("age"))
	require.Same(t, src.Get("__typename"), cp.Get("__typename"))
}

func TestTransform_ContainerValuesAreNewInstances(t *testing.T) {
	a := arena.NewMonotonicArena()
	var p Parser

	src, err := p.ParseWithArena(a, `{"user":{"id":"1"},"posts":[{"id":"p1"}]}`)
	require.NoError(t, err)

	cp := p.StructuralCopyWithTransform(a, src, &Transform{
		Entries: []TransformEntry{
			{InputKey: "user", OutputKey: "user", Child: &Transform{
				Entries: []TransformEntry{{InputKey: "id", OutputKey: "id"}},
			}},
			{InputKey: "posts", OutputKey: "posts", Child: &Transform{
				ArrayItem: &Transform{
					Entries: []TransformEntry{{InputKey: "id", OutputKey: "id"}},
				},
			}},
		},
	})

	require.NotSame(t, src, cp)
	require.NotSame(t, src.Get("user"), cp.Get("user"))
	require.NotSame(t, src.Get("posts"), cp.Get("posts"))
	require.NotSame(t, src.GetArray("posts")[0], cp.GetArray("posts")[0])
}

func TestTransform_MutatingCopyDoesNotAffectSource(t *testing.T) {
	a := arena.NewMonotonicArena()
	var p Parser

	src, err := p.ParseWithArena(a, `{"nickname":"Alice","__typename":"User"}`)
	require.NoError(t, err)

	cp := p.StructuralCopyWithTransform(a, src, &Transform{
		Entries: []TransformEntry{
			{InputKey: "nickname", OutputKey: "name"},
			{InputKey: "__typename", OutputKey: "__typename"},
		},
	})

	cp.Set(a, "extra", IntValue(a, 42))

	require.Nil(t, src.Get("extra"))
	require.Equal(t, `{"nickname":"Alice","__typename":"User"}`, string(src.MarshalTo(nil)))
}

// OutputKey bytes must be copied onto the arena, not aliased from the caller's
// string. Arena memory is noscan — a heap string referenced only from an
// arena-allocated kv would be invisible to the GC and could be collected.
func TestTransform_OutputKeyCopiedOntoArena(t *testing.T) {
	a := arena.NewMonotonicArena()
	var p Parser

	src, err := p.ParseWithArena(a, `{"x":"hello","y":"world"}`)
	require.NoError(t, err)

	// Force heap allocation for the OutputKey strings (compile-time constants
	// could live in rodata and compare equal to arena copies by accident).
	outputKeyX := "renamed" + "_x"
	outputKeyY := "renamed" + "_y"
	origPtrX := stringDataPtr(outputKeyX)
	origPtrY := stringDataPtr(outputKeyY)

	cp := p.StructuralCopyWithTransform(a, src, &Transform{
		Entries: []TransformEntry{
			{InputKey: "x", OutputKey: outputKeyX},
			{InputKey: "y", OutputKey: outputKeyY},
		},
	})

	require.Equal(t, `{"renamed_x":"hello","renamed_y":"world"}`, string(cp.MarshalTo(nil)))

	o, _ := cp.Object()
	for _, kv := range o.kvs {
		kvPtr := stringDataPtr(kv.k)
		switch kv.k {
		case "renamed_x":
			require.NotEqual(t, origPtrX, kvPtr,
				"key %q must be copied onto arena, not aliased from Transform", kv.k)
		case "renamed_y":
			require.NotEqual(t, origPtrY, kvPtr,
				"key %q must be copied onto arena, not aliased from Transform", kv.k)
		default:
			t.Fatalf("unexpected key: %q", kv.k)
		}
	}
}

func TestTransform_ScratchSlabReuseAcrossCalls(t *testing.T) {
	a := arena.NewMonotonicArena()
	var p Parser

	xform := &Transform{Entries: []TransformEntry{{InputKey: "a", OutputKey: "x"}}}

	src1, err := p.ParseWithArena(a, `{"a":"1"}`)
	require.NoError(t, err)
	cp1 := p.StructuralCopyWithTransform(a, src1, xform)
	require.Equal(t, `{"x":"1"}`, string(cp1.MarshalTo(nil)))

	// Second call reuses the parser's scratch slabs.
	src2, err := p.ParseWithArena(a, `{"a":"2"}`)
	require.NoError(t, err)
	cp2 := p.StructuralCopyWithTransform(a, src2, xform)
	require.Equal(t, `{"x":"2"}`, string(cp2.MarshalTo(nil)))
}

// stringDataPtr returns the pointer to the underlying data of a Go string.
// Used to verify that arena-copied strings have distinct backing memory
// from the caller-supplied originals.
func stringDataPtr(s string) uintptr {
	return uintptr(unsafe.Pointer(unsafe.StringData(s)))
}
