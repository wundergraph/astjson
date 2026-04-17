package astjson

import (
	"unsafe"

	"github.com/wundergraph/go-arena"
)

func b2s(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

func s2b(s string) (b []byte) {
	return unsafe.Slice(unsafe.StringData(s), len(s))
}

// arenaString copies the string bytes onto the arena when a is non-nil.
// When a is nil, returns s unchanged (heap string in heap struct is GC-safe).
func arenaString(a arena.Arena, s string) string {
	if a == nil {
		return s
	}
	b := arena.AllocateSlice[byte](a, len(s), len(s))
	copy(b, s)
	return b2s(b)
}

const maxStartEndStringLen = 80

func startEndString(s string) string {
	if len(s) <= maxStartEndStringLen {
		return s
	}
	start := s[:40]
	end := s[len(s)-40:]
	return start + "..." + end
}

// NullValue is a heap-allocated JSON null singleton. It is safe to use from
// any context (heap or arena) since it is a package-level global that is always
// reachable by the GC.
var (
	NullValue = MustParse(`null`)
)

// AppendToArray appends value to the end of array. Does nothing if array is
// not of TypeArray. The arena a is used to grow the array's backing slice.
//
// GC safety: when array is arena-allocated (a is non-nil), value must also be
// arena-allocated from the same arena, or be a package-level singleton.
// Use [Parser.DeepCopy] on value before calling if value is heap-allocated.
// See the package documentation section "Mixing Arena and Heap Values".
func AppendToArray(a arena.Arena, array, value *Value) {
	if array.Type() != TypeArray {
		return
	}
	items, _ := array.Array()
	array.SetArrayItem(a, len(items), value)
}

// SetValue sets value at the nested key path within v. Intermediate object
// nodes are created on the arena as needed when they don't exist. The path
// must have at least one element.
//
// Object keys created along the path are copied onto the arena when a is
// non-nil, ensuring GC safety. The same arena/heap mixing rules as
// [Object.Set] apply to the value argument.
func SetValue(a arena.Arena, v *Value, value *Value, path ...string) {
	for i := 0; i < len(path)-1; i++ {
		parent := v
		v = v.Get(path[i])
		if v == nil {
			child := ObjectValue(a)
			parent.Set(a, path[i], child)
			v = parent.Get(path[i])
		}
	}
	v.Set(a, path[len(path)-1], value)
}

// SetNull sets a null value at the nested key path within v. The null Value is
// allocated on the arena when a is non-nil. See [SetValue] for path behavior.
func SetNull(a arena.Arena, v *Value, path ...string) {
	null := arena.Allocate[Value](a)
	null.t = TypeNull
	SetValue(a, v, null, path...)
}

// ValueIsNonNull reports whether v is non-nil and not TypeNull.
func ValueIsNonNull(v *Value) bool {
	if v == nil {
		return false
	}
	if v.Type() == TypeNull {
		return false
	}
	return true
}

// DeepCopy returns a deep copy of v allocated on arena a.
// All string data, slice backing arrays, object keys, and non-literal child
// Values are arena-allocated, making the result self-contained within a except
// for the immutable package-level singleton nodes used for true, false, and null.
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
// When a is nil (heap mode), Parser.DeepCopy returns v unchanged. In heap mode the GC
// traces all references normally, so no copy is needed.
func (p *Parser) DeepCopy(a arena.Arena, v *Value) *Value {
	if v == nil || a == nil {
		return v
	}

	scratch := p.ensureArenaScratch()
	plan := planDeepCopyWithScratch(v, scratch)
	state := newDeepCopyFillState(a, plan)
	return state.copyValue(v)
}

// StructuralCopy clones only the container structure of v onto arena a.
// Object and array nodes are reallocated on a, while all leaf nodes and object
// key strings are aliased from the source tree unchanged.
//
// This is intended for trees whose leaves already have the same lifetime as the
// cloned structure, typically when both source and destination are used within
// the same request and reset together. It is not a safe replacement for
// DeepCopy when moving heap-allocated leaves into arena-owned containers.
//
// When a is nil (heap mode), StructuralCopy returns v unchanged.
func (p *Parser) StructuralCopy(a arena.Arena, v *Value) *Value {
	if v == nil || a == nil {
		return v
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
	if t == nil {
		return p.StructuralCopy(a, v)
	}
	if v == nil {
		return v
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
			// Passthrough fields are structurally copied verbatim.
			for _, entry := range v.o.kvs {
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
	case TypeString, TypeNumber:
		cp.s = f.copyString(v.s)
	case TypeObject:
		count := f.nextObjectSize()
		if count == 0 {
			return cp
		}
		cp.o.kvs = f.allocObjectRefs(count)
		for i, entry := range v.o.kvs {
			newKV := f.allocKV()
			newKV.k = f.copyString(entry.k)
			newKV.keyUnescaped = true
			newKV.keyNeedsEscape = entry.keyNeedsEscape
			newKV.v = f.copyValue(entry.v)
			cp.o.kvs[i] = newKV
		}
	case TypeArray:
		count := f.nextArraySize()
		if count == 0 {
			return cp
		}
		cp.a = f.allocArrayRefs(count)
		for i, item := range v.a {
			cp.a[i] = f.copyValue(item)
		}
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
			return cp
		}
		cp.o.kvs = f.allocObjectRefs(count)
		for i, entry := range v.o.kvs {
			newKV := f.allocKV()
			newKV.k = entry.k
			newKV.keyUnescaped = entry.keyUnescaped
			newKV.keyNeedsEscape = entry.keyNeedsEscape
			newKV.v = f.structuralCopyValue(entry.v)
			cp.o.kvs[i] = newKV
		}
		return cp
	case TypeArray:
		cp := f.allocValue()
		cp.t = TypeArray
		cp.s = ""
		cp.a = nil
		cp.o.reset()

		count := f.nextArraySize()
		if count == 0 {
			return cp
		}
		cp.a = f.allocArrayRefs(count)
		for i, item := range v.a {
			cp.a[i] = f.structuralCopyValue(item)
		}
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
			return cp
		}
		refs := f.allocObjectRefs(maxFields)

		n := 0
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
			newKV.keyNeedsEscape = false
			if t.Entries[i].Child != nil {
				newKV.v = f.structuralCopyWithTransformValue(child, t.Entries[i].Child)
			} else {
				newKV.v = f.structuralCopyValue(child)
			}
			refs[n] = newKV
			n++
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
			}
		}
		cp.o.kvs = refs[:n]
		return cp

	case TypeArray:
		cp := f.allocValue()
		cp.t = TypeArray
		cp.s = ""
		cp.a = nil
		cp.o.reset()

		count := f.nextArraySize()
		if count == 0 {
			return cp
		}
		cp.a = f.allocArrayRefs(count)
		if t != nil && t.ArrayItem != nil {
			for i, item := range v.a {
				cp.a[i] = f.structuralCopyWithTransformValue(item, t.ArrayItem)
			}
		} else {
			for i, item := range v.a {
				cp.a[i] = f.structuralCopyValue(item)
			}
		}
		return cp

	default:
		// Scalars: alias from source. Safe for concurrent reads because
		// strings are always eagerly decoded — no lazy mutation can race.
		return v
	}
}

// AppendArrayItems appends all elements from right into v. Both v and right
// must be TypeArray; does nothing otherwise. The arena a is used to grow v's
// backing slice.
//
// GC safety: when v is arena-allocated (a is non-nil), right and its elements
// must also be arena-allocated from the same arena. Use [Parser.DeepCopy] on right
// before calling if right is heap-allocated. See the package documentation
// section "Mixing Arena and Heap Values".
func (v *Value) AppendArrayItems(a arena.Arena, right *Value) {
	if v.t != TypeArray || right.t != TypeArray {
		return
	}
	for _, item := range right.a {
		v.a = arena.SliceAppend(a, v.a, item)
	}
}

// ValueIsNull reports whether v is nil or TypeNull.
func ValueIsNull(v *Value) bool {
	return !ValueIsNonNull(v)
}

// DeduplicateObjectKeysRecursively removes duplicate object keys from v and
// all nested objects and arrays, keeping the first occurrence of each key.
// This modifies v in place and does not require an arena.
func DeduplicateObjectKeysRecursively(v *Value) {
	if v.Type() == TypeArray {
		a := v.GetArray()
		for _, e := range a {
			DeduplicateObjectKeysRecursively(e)
		}
	}
	if v.Type() != TypeObject {
		return
	}
	o, _ := v.Object()
	// Heap-allocated: maps cannot be placed on the arena. The allocation is
	// bounded by the number of unique keys at each object level.
	seen := make(map[string]struct{})
	n := 0
	for _, kv := range o.kvs {
		if _, ok := seen[kv.k]; ok {
			continue
		}
		seen[kv.k] = struct{}{}
		o.kvs[n] = kv
		n++
		DeduplicateObjectKeysRecursively(kv.v)
	}
	for i := n; i < len(o.kvs); i++ {
		o.kvs[i] = nil // clear trailing slots for GC
	}
	o.kvs = o.kvs[:n]
}
