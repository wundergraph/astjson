package astjson

// Transform describes how to map object fields during a structural copy.
// A nil Transform means "copy verbatim" (no renaming or filtering).
//
// For objects: only fields listed in Entries are copied from the source.
// Each is renamed from InputKey to OutputKey. Unlisted fields are dropped.
// Callers that need to preserve an invariant field (e.g., __typename) beyond
// the query's selection set must append it to Entries themselves.
//
// For arrays: when ArrayItem is non-nil, it is applied to each element.
// Entries must be empty when ArrayItem is set.
//
// InputKey is never stored in the output — it is only used to look up
// the source field. OutputKey is copied onto the arena for GC safety:
// arena memory is noscan, so a heap string referenced only from an
// arena-allocated kv would be invisible to the GC and could be collected.
type Transform struct {
	// Entries maps input fields to output fields.
	// When Passthrough is false, only fields listed here are included in
	// the output. When Passthrough is true, unlisted fields are also
	// copied verbatim (no rename).
	Entries []TransformEntry

	// ArrayItem is applied to each element of a JSON array.
	// Mutually exclusive with Entries (a Transform is either
	// object-level or array-level).
	ArrayItem *Transform

	// Passthrough, when true, copies source fields not listed in Entries
	// verbatim (same key, no child transform). This enables
	// rename-without-projection: listed fields are renamed, unlisted
	// fields pass through unchanged.
	//
	// Rename wins on collision: if an Entries OutputKey was emitted and
	// a source field of the same name exists, the source field is dropped
	// rather than producing JSON with duplicate keys. If the rename could
	// not emit (source missing the InputKey), the slot is not claimed and
	// the source field passes through normally.
	Passthrough bool
}

// TransformEntry maps one source field to one destination field.
type TransformEntry struct {
	// InputKey is the field name to read from the source object.
	InputKey string

	// OutputKey is the field name to write in the destination object.
	// May differ from InputKey (alias renaming) or include a hash
	// suffix (argument-aware cache keys).
	OutputKey string

	// Child, when non-nil, is applied recursively to this field's value.
	// nil means copy the value with plain StructuralCopy semantics.
	Child *Transform
}
