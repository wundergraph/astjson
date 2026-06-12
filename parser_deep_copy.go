package astjson

import (
	"strings"
	"sync"

	"github.com/wundergraph/go-arena"
)

// ---------------------------------------------------------------------------
// Copy APIs: DeepCopy / StructuralCopy and their transform-aware variants.
//
// Every copy comes in four flavors spanning two axes:
//
//   - DeepCopy vs StructuralCopy: DeepCopy clones everything including scalar
//     string/number payloads; StructuralCopy clones only container nodes
//     (objects, arrays) and aliases scalar leaves from the source.
//   - plain vs WithTransform: the transform-aware variants rename/filter
//     object fields per [Transform] while copying.
//
// Arena-mode copies (a != nil) use the same plan+fill architecture as the
// two-pass arena parser (see parser_arena_twopass.go): a counting pass first
// computes exact slab sizes (plan*/count* functions building a deepCopyPlan),
// then a fill pass (deepCopyFillState) draws every node from bump-pointer
// slabs. The planner and the fill MUST skip and count exactly the same
// fields — see [passthroughSkipped] — otherwise sibling containers read
// misaligned objectSizes/arraySizes from the plan.
//
// Heap-mode copies (a == nil) skip planning entirely and allocate per node
// (copyValueHeap and friends at the bottom of this file).
//
// The APIs are exposed twice: as Parser methods (reusing the Parser's
// planner scratch, no synchronization) and as package-level functions
// (planner scratch from an internal sync.Pool).
// ---------------------------------------------------------------------------

// DeepCopy returns a fully independent deep copy of v. When a is non-nil the
// copy is allocated on the arena; when a is nil the copy is heap-allocated.
// In both cases the returned tree shares no mutable memory with v — scalar
// string payloads are cloned onto the destination.
//
// Use Parser.DeepCopy when inserting a heap-parsed *Value into an arena-allocated
// container (via [Object.Set], [Value.SetArrayItem], [AppendArrayItems], etc.)
// to prevent the GC from collecting the value while the arena container still
// references it. Example:
//
//	var parser Parser
//	heapVal, _ := Parse(`"hello"`)                       // heap-allocated
//	arenaObj.Set(a, "key", parser.DeepCopy(a, heapVal))  // safe: copy lives in a
//
// Prefer the package-level [DeepCopy] for ad-hoc calls; Parser.DeepCopy is
// the same API without the pool mutex, for callers already holding a Parser.
//
// Returns nil when v is nil. The immutable singletons (valueTrue, valueFalse,
// valueNull) are shared rather than cloned.
func (p *Parser) DeepCopy(a arena.Arena, v *Value) *Value {
	if v == nil {
		return nil
	}
	if a == nil {
		return copyValueHeap(v)
	}
	scratch := p.ensureArenaScratch()
	plan := planDeepCopyWithScratch(v, scratch)
	state := newDeepCopyFillState(a, plan)
	return state.copyValue(v)
}

// DeepCopyWithTransform is the transform-aware variant of [Parser.DeepCopy].
// Fields are renamed or filtered per t while every scalar payload surviving
// the transform is deep-copied. See [Transform] for t semantics.
//
// When t is nil, behaves identically to [Parser.DeepCopy].
func (p *Parser) DeepCopyWithTransform(a arena.Arena, v *Value, t *Transform) *Value {
	if v == nil {
		return nil
	}
	if t == nil {
		return p.DeepCopy(a, v)
	}
	if a == nil {
		return copyValueHeapWithTransform(v, t)
	}
	scratch := p.ensureArenaScratch()
	plan := planDeepCopyWithTransformScratch(v, t, scratch)
	state := newDeepCopyFillState(a, plan)
	return state.deepCopyWithTransformValue(v, t)
}

// StructuralCopy clones only the container structure of v. Object and array
// nodes are freshly allocated (on a when non-nil, else on the heap) while
// scalar leaves (strings, numbers, bools, nulls) and object key strings are
// aliased from the source tree unchanged.
//
// This is intended for trees whose leaves already have the same lifetime as the
// cloned structure, typically when both source and destination are used within
// the same request and reset together. It is not a safe replacement for
// DeepCopy when moving heap-allocated leaves into arena-owned containers.
func (p *Parser) StructuralCopy(a arena.Arena, v *Value) *Value {
	if v == nil {
		return nil
	}
	if a == nil {
		return structuralCopyValueHeap(v)
	}
	scratch := p.ensureArenaScratch()
	plan := planStructuralCopyWithScratch(v, scratch)
	state := newDeepCopyFillState(a, plan)
	return state.structuralCopyValue(v)
}

// StructuralCopyWithTransform clones v onto arena a, applying transform t
// to rename and filter object fields during the copy.
//
// Container nodes (objects, arrays) are reallocated on a. Leaf nodes
// (strings, numbers, bools, nulls) are aliased from the source unchanged,
// same as [Parser.StructuralCopy].
//
// Object key strings in the output are copied onto the arena from
// t.OutputKey to ensure GC safety — arena memory is noscan, so a heap
// string referenced only from an arena-allocated kv would be invisible
// to the GC.
//
// When t is nil, behaves identically to [Parser.StructuralCopy].
// When a is nil (heap mode), allocations fall back to the heap.
func (p *Parser) StructuralCopyWithTransform(a arena.Arena, v *Value, t *Transform) *Value {
	if v == nil {
		return nil
	}
	if t == nil {
		return p.StructuralCopy(a, v)
	}
	if a == nil {
		return structuralCopyValueHeapWithTransform(v, t)
	}
	scratch := p.ensureArenaScratch()
	plan := planStructuralCopyWithTransformScratch(v, t, scratch)
	state := newDeepCopyFillState(a, plan)
	return state.structuralCopyWithTransformValue(v, t)
}

type deepCopyPlan struct {
	values      int
	kvs         int
	arrayElems  int
	stringBytes int

	objectSizes []int
	arraySizes  []int
}

type deepCopyFillState struct {
	a    arena.Arena
	plan deepCopyPlan

	values     []Value
	kvs        []kv
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
}

type arenaAllocatedSlabs struct {
	values     []Value
	kvs        []kv
	objectRefs []*kv
	arrayRefs  []*Value
	strings    []byte
}

func planDeepCopyWithScratch(v *Value, scratch *arenaPlanScratch) deepCopyPlan {
	var plan deepCopyPlan
	if scratch != nil {
		plan.objectSizes = scratch.deepCopyObjectSizes[:0]
		plan.arraySizes = scratch.deepCopyArraySizes[:0]
	}
	countDeepCopyValue(&plan, v)
	if scratch != nil {
		scratch.deepCopyObjectSizes = plan.objectSizes[:0]
		scratch.deepCopyArraySizes = plan.arraySizes[:0]
	}
	return plan
}

func planStructuralCopyWithScratch(v *Value, scratch *arenaPlanScratch) deepCopyPlan {
	var plan deepCopyPlan
	if scratch != nil {
		plan.objectSizes = scratch.deepCopyObjectSizes[:0]
		plan.arraySizes = scratch.deepCopyArraySizes[:0]
	}
	countStructuralCopyValue(&plan, v)
	if scratch != nil {
		scratch.deepCopyObjectSizes = plan.objectSizes[:0]
		scratch.deepCopyArraySizes = plan.arraySizes[:0]
	}
	return plan
}

func planStructuralCopyWithTransformScratch(v *Value, t *Transform, scratch *arenaPlanScratch) deepCopyPlan {
	var plan deepCopyPlan
	if scratch != nil {
		plan.objectSizes = scratch.deepCopyObjectSizes[:0]
		plan.arraySizes = scratch.deepCopyArraySizes[:0]
	}
	countStructuralCopyWithTransformValue(&plan, v, t)
	if scratch != nil {
		scratch.deepCopyObjectSizes = plan.objectSizes[:0]
		scratch.deepCopyArraySizes = plan.arraySizes[:0]
	}
	return plan
}

func planDeepCopyWithTransformScratch(v *Value, t *Transform, scratch *arenaPlanScratch) deepCopyPlan {
	var plan deepCopyPlan
	if scratch != nil {
		plan.objectSizes = scratch.deepCopyObjectSizes[:0]
		plan.arraySizes = scratch.deepCopyArraySizes[:0]
	}
	countDeepCopyWithTransformValue(&plan, v, t)
	if scratch != nil {
		scratch.deepCopyObjectSizes = plan.objectSizes[:0]
		scratch.deepCopyArraySizes = plan.arraySizes[:0]
	}
	return plan
}

// countDeepCopyWithTransformValue is the deep-copy analogue of
// countStructuralCopyWithTransformValue. It counts scalar string bytes (since
// a deep copy must duplicate them) while otherwise mirroring the transform's
// Entries + Passthrough + ArrayItem logic.
func countDeepCopyWithTransformValue(plan *deepCopyPlan, v *Value, t *Transform) {
	if v == nil {
		return
	}

	switch v.t {
	case TypeObject:
		if t == nil {
			// No transform at this level — count as plain deep copy.
			countDeepCopyValue(plan, v)
			return
		}
		plan.values++
		maxFields := len(t.Entries)
		if t.Passthrough {
			maxFields += len(v.o.kvs)
		}
		plan.objectSizes = append(plan.objectSizes, maxFields)
		plan.kvs += maxFields

		for i := range t.Entries {
			plan.stringBytes += len(t.Entries[i].OutputKey)
			child := v.Get(t.Entries[i].InputKey)
			if child == nil {
				continue
			}
			if t.Entries[i].Child != nil {
				countDeepCopyWithTransformValue(plan, child, t.Entries[i].Child)
			} else {
				countDeepCopyValue(plan, child)
			}
		}
		if t.Passthrough {
			for i, entry := range v.o.kvs {
				if passthroughSkipped(v, entry.k, t) {
					continue
				}
				// Duplicate source keys: the fill emits only the first
				// occurrence and refs-skips the rest. Planner must match.
				dup := false
				for j := 0; j < i; j++ {
					if v.o.kvs[j].k == entry.k {
						dup = true
						break
					}
				}
				if dup {
					continue
				}
				plan.stringBytes += len(entry.k)
				countDeepCopyValue(plan, entry.v)
			}
		}

	case TypeArray:
		plan.values++
		plan.arraySizes = append(plan.arraySizes, len(v.a))
		plan.arrayElems += len(v.a)
		if t != nil && t.ArrayItem != nil {
			for _, item := range v.a {
				countDeepCopyWithTransformValue(plan, item, t.ArrayItem)
			}
		} else {
			for _, item := range v.a {
				countDeepCopyValue(plan, item)
			}
		}

	default:
		// Scalar: transforms do not apply; count as plain deep copy.
		countDeepCopyValue(plan, v)
	}
}

// passthroughSkipped reports whether a source field key should be dropped
// during a transform's Passthrough pass — either because it matches an
// Entries InputKey (the rename owns that field) or because it collides
// with an OutputKey that an Entry would actually emit (rename-wins on
// collision). Both planner and fill must agree on this predicate, otherwise
// plan counts drift from fill consumption and subsequent sibling nodes
// read misaligned objectSizes / arraySizes.
func passthroughSkipped(src *Value, key string, t *Transform) bool {
	for i := range t.Entries {
		if key == t.Entries[i].InputKey {
			return true
		}
	}
	for i := range t.Entries {
		if key != t.Entries[i].OutputKey {
			continue
		}
		if src.Get(t.Entries[i].InputKey) != nil {
			return true
		}
	}
	return false
}

func countStructuralCopyWithTransformValue(plan *deepCopyPlan, v *Value, t *Transform) {
	if v == nil {
		return
	}

	switch v.t {
	case TypeObject:
		if t == nil {
			// No transform at this level — count as plain structural copy.
			countStructuralCopyValue(plan, v)
			return
		}
		plan.values++
		// Over-count: allocate slots for all Entries + source fields (passthrough).
		maxFields := len(t.Entries)
		if t.Passthrough {
			maxFields += len(v.o.kvs)
		}
		plan.objectSizes = append(plan.objectSizes, maxFields)
		plan.kvs += maxFields

		// Count children and OutputKey string bytes (copied onto the arena
		// for GC safety — see [Parser.StructuralCopyWithTransform]).
		for i := range t.Entries {
			plan.stringBytes += len(t.Entries[i].OutputKey)
			child := v.Get(t.Entries[i].InputKey)
			if child == nil {
				continue
			}
			if t.Entries[i].Child != nil {
				countStructuralCopyWithTransformValue(plan, child, t.Entries[i].Child)
			} else {
				countStructuralCopyValue(plan, child)
			}
		}
		if t.Passthrough {
			// Passthrough fields are structurally copied verbatim, except
			// for fields that rename-collide with an emitted OutputKey,
			// shadow an InputKey already handled above, or appear as a
			// duplicate of an earlier passthrough field (refs-deduped by
			// the fill). See [passthroughSkipped] for why the predicate
			// must match the fill's skip logic.
			for i, entry := range v.o.kvs {
				if passthroughSkipped(v, entry.k, t) {
					continue
				}
				dup := false
				for j := 0; j < i; j++ {
					if v.o.kvs[j].k == entry.k {
						dup = true
						break
					}
				}
				if dup {
					continue
				}
				countStructuralCopyValue(plan, entry.v)
			}
		}

	case TypeArray:
		plan.values++
		plan.arraySizes = append(plan.arraySizes, len(v.a))
		plan.arrayElems += len(v.a)
		if t != nil && t.ArrayItem != nil {
			for _, item := range v.a {
				countStructuralCopyWithTransformValue(plan, item, t.ArrayItem)
			}
		} else {
			for _, item := range v.a {
				countStructuralCopyValue(plan, item)
			}
		}
	}
	// Scalars: no allocation needed (aliased from source).
}

func countDeepCopyValue(plan *deepCopyPlan, v *Value) {
	if v == nil {
		return
	}

	switch v.t {
	case TypeTrue, TypeFalse, TypeNull:
		return
	}

	plan.values++
	switch v.t {
	case TypeString, TypeNumber:
		plan.stringBytes += len(v.s)
	case TypeObject:
		plan.objectSizes = append(plan.objectSizes, len(v.o.kvs))
		plan.kvs += len(v.o.kvs)
		for _, entry := range v.o.kvs {
			plan.stringBytes += len(entry.k)
			countDeepCopyValue(plan, entry.v)
		}
	case TypeArray:
		plan.arraySizes = append(plan.arraySizes, len(v.a))
		plan.arrayElems += len(v.a)
		for _, item := range v.a {
			countDeepCopyValue(plan, item)
		}
	}
}

func countStructuralCopyValue(plan *deepCopyPlan, v *Value) {
	if v == nil {
		return
	}

	switch v.t {
	case TypeObject:
		plan.values++
		plan.objectSizes = append(plan.objectSizes, len(v.o.kvs))
		plan.kvs += len(v.o.kvs)
		for _, entry := range v.o.kvs {
			countStructuralCopyValue(plan, entry.v)
		}
	case TypeArray:
		plan.values++
		plan.arraySizes = append(plan.arraySizes, len(v.a))
		plan.arrayElems += len(v.a)
		for _, item := range v.a {
			countStructuralCopyValue(plan, item)
		}
	}
}

func allocateArenaSlabs(a arena.Arena, values, kvs, arrayElems, stringBytes int) arenaAllocatedSlabs {
	var slabs arenaAllocatedSlabs
	if values > 0 {
		slabs.values = arena.AllocateSlice[Value](a, values, values)
	}
	if kvs > 0 {
		slabs.kvs = arena.AllocateSlice[kv](a, kvs, kvs)
		slabs.objectRefs = arena.AllocateSlice[*kv](a, kvs, kvs)
	}
	if arrayElems > 0 {
		slabs.arrayRefs = arena.AllocateSlice[*Value](a, arrayElems, arrayElems)
	}
	if stringBytes > 0 {
		slabs.strings = arena.AllocateSlice[byte](a, stringBytes, stringBytes)
	}
	return slabs
}

func newDeepCopyFillState(a arena.Arena, plan deepCopyPlan) deepCopyFillState {
	state := deepCopyFillState{a: a, plan: plan}
	slabs := allocateArenaSlabs(a, plan.values, plan.kvs, plan.arrayElems, plan.stringBytes)
	state.values = slabs.values
	state.kvs = slabs.kvs
	state.objectRefs = slabs.objectRefs
	state.arrayRefs = slabs.arrayRefs
	state.strings = slabs.strings
	return state
}

func (f *deepCopyFillState) allocValue() *Value {
	v := &f.values[f.valuePos]
	f.valuePos++
	return v
}

func (f *deepCopyFillState) allocKV() *kv {
	entry := &f.kvs[f.kvPos]
	f.kvPos++
	return entry
}

func (f *deepCopyFillState) allocObjectRefs(n int) []*kv {
	start := f.objectRefPos
	f.objectRefPos += n
	return f.objectRefs[start:f.objectRefPos:f.objectRefPos]
}

func (f *deepCopyFillState) allocArrayRefs(n int) []*Value {
	start := f.arrayRefPos
	f.arrayRefPos += n
	return f.arrayRefs[start:f.arrayRefPos:f.arrayRefPos]
}

func (f *deepCopyFillState) allocString(n int) []byte {
	start := f.stringPos
	f.stringPos += n
	return f.strings[start:f.stringPos]
}

func (f *deepCopyFillState) nextObjectSize() int {
	size := f.plan.objectSizes[f.objectPos]
	f.objectPos++
	return size
}

func (f *deepCopyFillState) nextArraySize() int {
	size := f.plan.arraySizes[f.arrayPos]
	f.arrayPos++
	return size
}

func (f *deepCopyFillState) copyString(s string) string {
	if len(s) == 0 {
		return ""
	}
	if len(f.strings) == 0 {
		return arenaString(f.a, s)
	}
	buf := f.allocString(len(s))
	copy(buf, s)
	return b2s(buf)
}

func (f *deepCopyFillState) copyValue(v *Value) *Value {
	if v == nil {
		return nil
	}

	switch v.t {
	case TypeTrue:
		return valueTrue
	case TypeFalse:
		return valueFalse
	case TypeNull:
		return valueNull
	}

	cp := f.allocValue()
	cp.t = v.t
	cp.stringNeedsEscape = v.stringNeedsEscape
	cp.s = ""
	cp.a = nil
	cp.o.reset()

	switch v.t {
	case TypeString:
		cp.s = f.copyString(v.s)
		cp.noEscapeSubtree = !cp.stringNeedsEscape
	case TypeNumber:
		cp.s = f.copyString(v.s)
		cp.noEscapeSubtree = true
	case TypeObject:
		count := f.nextObjectSize()
		if count == 0 {
			cp.noEscapeSubtree = true
			return cp
		}
		cp.o.kvs = f.allocObjectRefs(count)
		clean := true
		for i, entry := range v.o.kvs {
			newKV := f.allocKV()
			newKV.k = f.copyString(entry.k)
			newKV.keyUnescaped = true
			newKV.keyNeedsEscape = entry.keyNeedsEscape
			newKV.v = f.copyValue(entry.v)
			cp.o.kvs[i] = newKV
			clean = clean && !newKV.keyNeedsEscape && valueIsEscapeFree(newKV.v)
		}
		cp.noEscapeSubtree = clean
	case TypeArray:
		count := f.nextArraySize()
		if count == 0 {
			cp.noEscapeSubtree = true
			return cp
		}
		cp.a = f.allocArrayRefs(count)
		clean := true
		for i, item := range v.a {
			cp.a[i] = f.copyValue(item)
			clean = clean && valueIsEscapeFree(cp.a[i])
		}
		cp.noEscapeSubtree = clean
	}

	return cp
}

func (f *deepCopyFillState) structuralCopyValue(v *Value) *Value {
	if v == nil {
		return nil
	}

	switch v.t {
	case TypeObject:
		cp := f.allocValue()
		cp.t = TypeObject
		cp.s = ""
		cp.a = nil
		cp.o.reset()

		count := f.nextObjectSize()
		if count == 0 {
			cp.noEscapeSubtree = true
			return cp
		}
		cp.o.kvs = f.allocObjectRefs(count)
		clean := true
		for i, entry := range v.o.kvs {
			newKV := f.allocKV()
			newKV.k = entry.k
			newKV.keyUnescaped = entry.keyUnescaped
			newKV.keyNeedsEscape = entry.keyNeedsEscape
			newKV.v = f.structuralCopyValue(entry.v)
			cp.o.kvs[i] = newKV
			clean = clean && !newKV.keyNeedsEscape && valueIsEscapeFree(newKV.v)
		}
		cp.noEscapeSubtree = clean
		return cp
	case TypeArray:
		cp := f.allocValue()
		cp.t = TypeArray
		cp.s = ""
		cp.a = nil
		cp.o.reset()

		count := f.nextArraySize()
		if count == 0 {
			cp.noEscapeSubtree = true
			return cp
		}
		cp.a = f.allocArrayRefs(count)
		clean := true
		for i, item := range v.a {
			cp.a[i] = f.structuralCopyValue(item)
			clean = clean && valueIsEscapeFree(cp.a[i])
		}
		cp.noEscapeSubtree = clean
		return cp
	default:
		// Scalars: alias from source. Safe for concurrent reads because
		// strings are always eagerly decoded during parsing — no lazy
		// mutation can race.
		return v
	}
}

func (f *deepCopyFillState) structuralCopyWithTransformValue(v *Value, t *Transform) *Value {
	if v == nil {
		return nil
	}

	switch v.t {
	case TypeObject:
		if t == nil {
			return f.structuralCopyValue(v)
		}
		cp := f.allocValue()
		cp.t = TypeObject
		cp.s = ""
		cp.a = nil
		cp.o.reset()

		maxFields := f.nextObjectSize()
		if maxFields == 0 {
			cp.noEscapeSubtree = true
			return cp
		}
		refs := f.allocObjectRefs(maxFields)

		n := 0
		clean := true
		for i := range t.Entries {
			child := v.Get(t.Entries[i].InputKey)
			if child == nil {
				continue
			}
			newKV := f.allocKV()
			// OutputKey is copied onto the arena for GC safety — see
			// [Parser.StructuralCopyWithTransform].
			newKV.k = f.copyString(t.Entries[i].OutputKey)
			newKV.keyUnescaped = true
			newKV.keyNeedsEscape = hasSpecialChars(t.Entries[i].OutputKey)
			if t.Entries[i].Child != nil {
				newKV.v = f.structuralCopyWithTransformValue(child, t.Entries[i].Child)
			} else {
				newKV.v = f.structuralCopyValue(child)
			}
			refs[n] = newKV
			n++
			clean = clean && !newKV.keyNeedsEscape && valueIsEscapeFree(newKV.v)
		}
		if t.Passthrough {
			// Copy source fields not already handled by Entries.
			// Linear scans are intentional: typical JSON objects have
			// few fields, and map-based lookups would cost two heap
			// allocations per call that beat the O(n*m) scan only at
			// sizes we don't expect to see here.
			for _, entry := range v.o.kvs {
				handled := false
				for i := range t.Entries {
					if entry.k == t.Entries[i].InputKey {
						handled = true
						break
					}
				}
				// Rename wins: if an Entries OutputKey already emitted a field
				// matching this source field, skip it to avoid producing JSON
				// with duplicate keys. refs[0:n] holds the emitted kvs.
				if !handled {
					for i := 0; i < n; i++ {
						if entry.k == refs[i].k {
							handled = true
							break
						}
					}
				}
				if handled {
					continue
				}
				newKV := f.allocKV()
				newKV.k = entry.k
				newKV.keyUnescaped = entry.keyUnescaped
				newKV.keyNeedsEscape = entry.keyNeedsEscape
				newKV.v = f.structuralCopyValue(entry.v)
				refs[n] = newKV
				n++
				clean = clean && !newKV.keyNeedsEscape && valueIsEscapeFree(newKV.v)
			}
		}
		cp.o.kvs = refs[:n]
		cp.noEscapeSubtree = clean
		return cp

	case TypeArray:
		cp := f.allocValue()
		cp.t = TypeArray
		cp.s = ""
		cp.a = nil
		cp.o.reset()

		count := f.nextArraySize()
		if count == 0 {
			cp.noEscapeSubtree = true
			return cp
		}
		cp.a = f.allocArrayRefs(count)
		clean := true
		if t != nil && t.ArrayItem != nil {
			for i, item := range v.a {
				cp.a[i] = f.structuralCopyWithTransformValue(item, t.ArrayItem)
				clean = clean && valueIsEscapeFree(cp.a[i])
			}
		} else {
			for i, item := range v.a {
				cp.a[i] = f.structuralCopyValue(item)
				clean = clean && valueIsEscapeFree(cp.a[i])
			}
		}
		cp.noEscapeSubtree = clean
		return cp

	default:
		// Scalars: alias from source. Safe for concurrent reads because
		// strings are always eagerly decoded — no lazy mutation can race.
		return v
	}
}

// deepCopyWithTransformValue is the deep-copy analogue of
// structuralCopyWithTransformValue. Produces an independent tree — scalar
// string and number payloads are copied onto the arena, not aliased.
func (f *deepCopyFillState) deepCopyWithTransformValue(v *Value, t *Transform) *Value {
	if v == nil {
		return nil
	}

	switch v.t {
	case TypeObject:
		if t == nil {
			return f.copyValue(v)
		}
		cp := f.allocValue()
		cp.t = TypeObject
		cp.stringNeedsEscape = false
		cp.s = ""
		cp.a = nil
		cp.o.reset()

		maxFields := f.nextObjectSize()
		if maxFields == 0 {
			cp.noEscapeSubtree = true
			return cp
		}
		refs := f.allocObjectRefs(maxFields)

		n := 0
		clean := true
		for i := range t.Entries {
			child := v.Get(t.Entries[i].InputKey)
			if child == nil {
				continue
			}
			newKV := f.allocKV()
			newKV.k = f.copyString(t.Entries[i].OutputKey)
			newKV.keyUnescaped = true
			newKV.keyNeedsEscape = hasSpecialChars(t.Entries[i].OutputKey)
			if t.Entries[i].Child != nil {
				newKV.v = f.deepCopyWithTransformValue(child, t.Entries[i].Child)
			} else {
				newKV.v = f.copyValue(child)
			}
			refs[n] = newKV
			n++
			clean = clean && !newKV.keyNeedsEscape && valueIsEscapeFree(newKV.v)
		}
		if t.Passthrough {
			for _, entry := range v.o.kvs {
				handled := false
				for i := range t.Entries {
					if entry.k == t.Entries[i].InputKey {
						handled = true
						break
					}
				}
				if !handled {
					for i := 0; i < n; i++ {
						if entry.k == refs[i].k {
							handled = true
							break
						}
					}
				}
				if handled {
					continue
				}
				newKV := f.allocKV()
				newKV.k = f.copyString(entry.k)
				newKV.keyUnescaped = true
				newKV.keyNeedsEscape = entry.keyNeedsEscape
				newKV.v = f.copyValue(entry.v)
				refs[n] = newKV
				n++
				clean = clean && !newKV.keyNeedsEscape && valueIsEscapeFree(newKV.v)
			}
		}
		cp.o.kvs = refs[:n]
		cp.noEscapeSubtree = clean
		return cp

	case TypeArray:
		cp := f.allocValue()
		cp.t = TypeArray
		cp.stringNeedsEscape = false
		cp.s = ""
		cp.a = nil
		cp.o.reset()

		count := f.nextArraySize()
		if count == 0 {
			cp.noEscapeSubtree = true
			return cp
		}
		cp.a = f.allocArrayRefs(count)
		clean := true
		if t != nil && t.ArrayItem != nil {
			for i, item := range v.a {
				cp.a[i] = f.deepCopyWithTransformValue(item, t.ArrayItem)
				clean = clean && valueIsEscapeFree(cp.a[i])
			}
		} else {
			for i, item := range v.a {
				cp.a[i] = f.copyValue(item)
				clean = clean && valueIsEscapeFree(cp.a[i])
			}
		}
		cp.noEscapeSubtree = clean
		return cp

	default:
		// Scalar: transforms do not apply; delegate to the deep-copy path.
		return f.copyValue(v)
	}
}

// ---------------------------------------------------------------------------
// Heap-mode copy primitives
//
// These are used when the destination arena is nil. They produce heap-
// allocated Value trees with the same semantics as the arena path:
//   - DeepCopy  : independent tree, all scalar payloads cloned.
//   - Structural: structure-only clone; scalar leaves aliased from source.
// The transform-aware variants apply rename/filter logic while following
// the same cloning semantics as their plain siblings.
// ---------------------------------------------------------------------------

// copyValueHeap returns a fully independent heap-allocated deep copy of v.
// Strings and numbers are cloned so the result shares no backing memory
// with the source; the valueTrue / valueFalse / valueNull singletons are
// returned as-is because they are immutable.
func copyValueHeap(v *Value) *Value {
	if v == nil {
		return nil
	}
	switch v.t {
	case TypeTrue:
		return valueTrue
	case TypeFalse:
		return valueFalse
	case TypeNull:
		return valueNull
	}
	cp := &Value{
		t:                 v.t,
		stringNeedsEscape: v.stringNeedsEscape,
	}
	switch v.t {
	case TypeString:
		cp.s = strings.Clone(v.s)
		cp.noEscapeSubtree = !cp.stringNeedsEscape
	case TypeNumber:
		cp.s = strings.Clone(v.s)
		cp.noEscapeSubtree = true
	case TypeObject:
		if len(v.o.kvs) == 0 {
			cp.noEscapeSubtree = true
			return cp
		}
		cp.o.kvs = make([]*kv, len(v.o.kvs))
		clean := true
		for i, entry := range v.o.kvs {
			child := copyValueHeap(entry.v)
			newKV := &kv{
				k:              strings.Clone(entry.k),
				keyUnescaped:   true,
				keyNeedsEscape: entry.keyNeedsEscape,
				v:              child,
			}
			cp.o.kvs[i] = newKV
			clean = clean && !newKV.keyNeedsEscape && valueIsEscapeFree(child)
		}
		cp.noEscapeSubtree = clean
	case TypeArray:
		if len(v.a) == 0 {
			cp.noEscapeSubtree = true
			return cp
		}
		cp.a = make([]*Value, len(v.a))
		clean := true
		for i, item := range v.a {
			cp.a[i] = copyValueHeap(item)
			clean = clean && valueIsEscapeFree(cp.a[i])
		}
		cp.noEscapeSubtree = clean
	}
	return cp
}

// structuralCopyValueHeap clones only the container structure onto the heap.
// Scalar leaves (including the kv key strings, matching the arena path) are
// aliased from the source. See the caveat in [StructuralCopy] about leaf
// lifetimes when mixing arena and heap values.
func structuralCopyValueHeap(v *Value) *Value {
	if v == nil {
		return nil
	}
	switch v.t {
	case TypeObject:
		cp := &Value{t: TypeObject}
		if len(v.o.kvs) == 0 {
			cp.noEscapeSubtree = true
			return cp
		}
		cp.o.kvs = make([]*kv, len(v.o.kvs))
		clean := true
		for i, entry := range v.o.kvs {
			newKV := &kv{
				k:              entry.k, // aliased
				keyUnescaped:   entry.keyUnescaped,
				keyNeedsEscape: entry.keyNeedsEscape,
				v:              structuralCopyValueHeap(entry.v),
			}
			cp.o.kvs[i] = newKV
			clean = clean && !newKV.keyNeedsEscape && valueIsEscapeFree(newKV.v)
		}
		cp.noEscapeSubtree = clean
		return cp
	case TypeArray:
		cp := &Value{t: TypeArray}
		if len(v.a) == 0 {
			cp.noEscapeSubtree = true
			return cp
		}
		cp.a = make([]*Value, len(v.a))
		clean := true
		for i, item := range v.a {
			cp.a[i] = structuralCopyValueHeap(item)
			clean = clean && valueIsEscapeFree(cp.a[i])
		}
		cp.noEscapeSubtree = clean
		return cp
	default:
		// Scalars: alias from source.
		return v
	}
}

// copyValueHeapWithTransform is the heap-mode variant of
// deepCopyWithTransformValue. Applies rename/filter/passthrough while
// producing an independent tree with all scalar payloads cloned.
func copyValueHeapWithTransform(v *Value, t *Transform) *Value {
	if v == nil {
		return nil
	}
	switch v.t {
	case TypeObject:
		if t == nil {
			return copyValueHeap(v)
		}
		cp := &Value{t: TypeObject}
		maxFields := len(t.Entries)
		if t.Passthrough {
			maxFields += len(v.o.kvs)
		}
		if maxFields == 0 {
			cp.noEscapeSubtree = true
			return cp
		}
		refs := make([]*kv, 0, maxFields)
		clean := true
		for i := range t.Entries {
			child := v.Get(t.Entries[i].InputKey)
			if child == nil {
				continue
			}
			outKey := strings.Clone(t.Entries[i].OutputKey)
			var newChild *Value
			if t.Entries[i].Child != nil {
				newChild = copyValueHeapWithTransform(child, t.Entries[i].Child)
			} else {
				newChild = copyValueHeap(child)
			}
			newKV := &kv{
				k:              outKey,
				keyUnescaped:   true,
				keyNeedsEscape: hasSpecialChars(outKey),
				v:              newChild,
			}
			refs = append(refs, newKV)
			clean = clean && !newKV.keyNeedsEscape && valueIsEscapeFree(newChild)
		}
		if t.Passthrough {
			for _, entry := range v.o.kvs {
				handled := false
				for i := range t.Entries {
					if entry.k == t.Entries[i].InputKey {
						handled = true
						break
					}
				}
				if !handled {
					for _, existing := range refs {
						if entry.k == existing.k {
							handled = true
							break
						}
					}
				}
				if handled {
					continue
				}
				newChild := copyValueHeap(entry.v)
				newKV := &kv{
					k:              strings.Clone(entry.k),
					keyUnescaped:   true,
					keyNeedsEscape: entry.keyNeedsEscape,
					v:              newChild,
				}
				refs = append(refs, newKV)
				clean = clean && !newKV.keyNeedsEscape && valueIsEscapeFree(newChild)
			}
		}
		cp.o.kvs = refs
		cp.noEscapeSubtree = clean
		return cp

	case TypeArray:
		cp := &Value{t: TypeArray}
		if len(v.a) == 0 {
			cp.noEscapeSubtree = true
			return cp
		}
		cp.a = make([]*Value, len(v.a))
		clean := true
		if t != nil && t.ArrayItem != nil {
			for i, item := range v.a {
				cp.a[i] = copyValueHeapWithTransform(item, t.ArrayItem)
				clean = clean && valueIsEscapeFree(cp.a[i])
			}
		} else {
			for i, item := range v.a {
				cp.a[i] = copyValueHeap(item)
				clean = clean && valueIsEscapeFree(cp.a[i])
			}
		}
		cp.noEscapeSubtree = clean
		return cp

	default:
		// Scalar: transforms do not apply; delegate to plain deep copy.
		return copyValueHeap(v)
	}
}

// structuralCopyValueHeapWithTransform is the heap-mode variant of
// structuralCopyWithTransformValue. Rename/filter logic with aliased
// scalar leaves.
func structuralCopyValueHeapWithTransform(v *Value, t *Transform) *Value {
	if v == nil {
		return nil
	}
	switch v.t {
	case TypeObject:
		if t == nil {
			return structuralCopyValueHeap(v)
		}
		cp := &Value{t: TypeObject}
		maxFields := len(t.Entries)
		if t.Passthrough {
			maxFields += len(v.o.kvs)
		}
		if maxFields == 0 {
			cp.noEscapeSubtree = true
			return cp
		}
		refs := make([]*kv, 0, maxFields)
		clean := true
		for i := range t.Entries {
			child := v.Get(t.Entries[i].InputKey)
			if child == nil {
				continue
			}
			outKey := t.Entries[i].OutputKey
			var newChild *Value
			if t.Entries[i].Child != nil {
				newChild = structuralCopyValueHeapWithTransform(child, t.Entries[i].Child)
			} else {
				newChild = structuralCopyValueHeap(child)
			}
			newKV := &kv{
				k:              outKey,
				keyUnescaped:   true,
				keyNeedsEscape: hasSpecialChars(outKey),
				v:              newChild,
			}
			refs = append(refs, newKV)
			clean = clean && !newKV.keyNeedsEscape && valueIsEscapeFree(newChild)
		}
		if t.Passthrough {
			for _, entry := range v.o.kvs {
				handled := false
				for i := range t.Entries {
					if entry.k == t.Entries[i].InputKey {
						handled = true
						break
					}
				}
				if !handled {
					for _, existing := range refs {
						if entry.k == existing.k {
							handled = true
							break
						}
					}
				}
				if handled {
					continue
				}
				newChild := structuralCopyValueHeap(entry.v)
				newKV := &kv{
					k:              entry.k,
					keyUnescaped:   entry.keyUnescaped,
					keyNeedsEscape: entry.keyNeedsEscape,
					v:              newChild,
				}
				refs = append(refs, newKV)
				clean = clean && !newKV.keyNeedsEscape && valueIsEscapeFree(newChild)
			}
		}
		cp.o.kvs = refs
		cp.noEscapeSubtree = clean
		return cp

	case TypeArray:
		cp := &Value{t: TypeArray}
		if len(v.a) == 0 {
			cp.noEscapeSubtree = true
			return cp
		}
		cp.a = make([]*Value, len(v.a))
		clean := true
		if t != nil && t.ArrayItem != nil {
			for i, item := range v.a {
				cp.a[i] = structuralCopyValueHeapWithTransform(item, t.ArrayItem)
				clean = clean && valueIsEscapeFree(cp.a[i])
			}
		} else {
			for i, item := range v.a {
				cp.a[i] = structuralCopyValueHeap(item)
				clean = clean && valueIsEscapeFree(cp.a[i])
			}
		}
		cp.noEscapeSubtree = clean
		return cp

	default:
		return v
	}
}

// ---------------------------------------------------------------------------
// Package-level copy API (with pooled planner scratch for arena paths)
// ---------------------------------------------------------------------------

var copyScratchPool = sync.Pool{
	New: func() any { return &arenaPlanScratch{} },
}

// acquireCopyScratch gets a scratch buffer from the pool, trimming any
// planner slice that grew beyond maxArenaScratchCap on a prior call so a
// single outlier document cannot permanently bloat the pool entries.
func acquireCopyScratch() *arenaPlanScratch {
	s := copyScratchPool.Get().(*arenaPlanScratch)
	capBoundInts(&s.objectSizes)
	capBoundInts(&s.arraySizes)
	capBoundSpans(&s.keySpans)
	capBoundSpans(&s.stringSpans)
	capBoundInts(&s.deepCopyObjectSizes)
	capBoundInts(&s.deepCopyArraySizes)
	return s
}

func releaseCopyScratch(s *arenaPlanScratch) {
	copyScratchPool.Put(s)
}

// DeepCopy returns a fully independent deep copy of v. When a is non-nil the
// copy is allocated on the arena; when a is nil the copy is heap-allocated.
// In both cases the returned tree shares no mutable memory with v — scalar
// string payloads are cloned onto the destination.
//
// Planner scratch buffers are reused via an internal [sync.Pool] so repeated
// calls avoid allocation. Callers that already hold a [Parser] in scope may
// prefer [Parser.DeepCopy] to skip the pool's internal mutex.
//
// Returns nil when v is nil. The immutable singletons (valueTrue, valueFalse,
// valueNull) are shared rather than cloned.
func DeepCopy(a arena.Arena, v *Value) *Value {
	if v == nil {
		return nil
	}
	if a == nil {
		return copyValueHeap(v)
	}
	scratch := acquireCopyScratch()
	defer releaseCopyScratch(scratch)
	plan := planDeepCopyWithScratch(v, scratch)
	state := newDeepCopyFillState(a, plan)
	return state.copyValue(v)
}

// DeepCopyWithTransform is the transform-aware variant of [DeepCopy]. Fields
// are renamed or filtered according to t while every scalar payload that
// survives the transform is deep-copied. See [Transform] for t semantics.
//
// When t is nil, behaves identically to [DeepCopy].
func DeepCopyWithTransform(a arena.Arena, v *Value, t *Transform) *Value {
	if v == nil {
		return nil
	}
	if t == nil {
		return DeepCopy(a, v)
	}
	if a == nil {
		return copyValueHeapWithTransform(v, t)
	}
	scratch := acquireCopyScratch()
	defer releaseCopyScratch(scratch)
	plan := planDeepCopyWithTransformScratch(v, t, scratch)
	state := newDeepCopyFillState(a, plan)
	return state.deepCopyWithTransformValue(v, t)
}

// StructuralCopy clones only the container structure of v. Object and array
// nodes are freshly allocated (on a when non-nil, else on the heap) while
// scalar leaves (strings, numbers, bools, nulls) and object key strings are
// aliased from the source tree unchanged.
//
// This is intended for trees whose leaves already have the same lifetime as
// the cloned structure — typically when both source and destination live
// within the same request and reset together. It is not a safe replacement
// for [DeepCopy] when moving heap-allocated leaves into arena-owned
// containers.
func StructuralCopy(a arena.Arena, v *Value) *Value {
	if v == nil {
		return nil
	}
	if a == nil {
		return structuralCopyValueHeap(v)
	}
	scratch := acquireCopyScratch()
	defer releaseCopyScratch(scratch)
	plan := planStructuralCopyWithScratch(v, scratch)
	state := newDeepCopyFillState(a, plan)
	return state.structuralCopyValue(v)
}

// StructuralCopyWithTransform clones v's container structure onto a (or the
// heap when a is nil), applying t to rename and filter object fields during
// the copy. Scalar leaves are aliased from the source exactly as in
// [StructuralCopy]; only object structure, array structure, and newly
// introduced OutputKey strings are freshly allocated.
//
// When t is nil, behaves identically to [StructuralCopy].
func StructuralCopyWithTransform(a arena.Arena, v *Value, t *Transform) *Value {
	if v == nil {
		return nil
	}
	if t == nil {
		return StructuralCopy(a, v)
	}
	if a == nil {
		return structuralCopyValueHeapWithTransform(v, t)
	}
	scratch := acquireCopyScratch()
	defer releaseCopyScratch(scratch)
	plan := planStructuralCopyWithTransformScratch(v, t, scratch)
	state := newDeepCopyFillState(a, plan)
	return state.structuralCopyWithTransformValue(v, t)
}
