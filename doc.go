/*
Package astjson provides fast JSON parsing with optional arena allocation.

Arbitrary JSON may be parsed without creating structs or generating Go code.
Just parse JSON and get the required fields with Get* functions.

# Heap Mode vs Arena Mode

The library operates in two memory modes depending on whether an [arena.Arena]
is provided:

Heap mode (nil arena): All Value structs and their backing data are allocated
on the Go heap. The garbage collector tracks all references normally. Use
[Parse], [ParseBytes], or [Parser.Parse] / [Parser.ParseBytes] for this mode.
Parsed values remain valid until the next call to any Parse method on the same
Parser, or indefinitely when using the package-level convenience functions.

Arena mode (non-nil arena): All Value structs, string backing bytes, and slice
backing arrays are allocated on the arena. The Go GC does not scan arena memory,
so the library copies all input data onto the arena before parsing. This means
the caller can drop references to the input string/bytes immediately after
parsing. Parsed values remain valid for the lifetime of the arena. Use
[ParseWithArena], [ParseBytesWithArena], or [Parser.ParseWithArena] /
[Parser.ParseBytesWithArena] for this mode.

Arena mode is useful for high-throughput parsing where GC pressure from millions
of small Value allocations would degrade performance. Instead of creating heap
garbage, all allocations go into a contiguous arena buffer that can be reset and
reused.

# GC Safety Invariant

When using arena mode, the following invariant is maintained automatically:

Arena-allocated Values never reference heap-allocated string data.

This is critical because the GC does not scan arena memory for pointers. If an
arena-allocated Value.s field pointed to a heap string's backing array, the GC
could collect that string (since it cannot see the reference in arena memory),
causing a use-after-free. The library prevents this by:

  - Copying the entire input string/bytes onto the arena before parsing, so all
    parsed substrings (number literals, string values, object keys) point into
    arena memory.
  - Copying string arguments onto the arena in value constructors like
    [StringValue], [IntValue], etc.
  - Copying object keys onto the arena in [Object.Set].

When using heap mode (nil arena), all Values live on the heap where the GC can
see them, so heap string references are safe.

# Mixing Arena and Heap Values

The noscan property extends beyond string data to all pointer fields within
arena-allocated structs. Because arena buffers are raw []byte allocations, the
GC does not trace pointer fields such as Value.a ([]*Value), Object.kvs ([]*kv),
or kv.v (*Value) that live inside arena memory.

This means storing a heap-allocated *Value into an arena-allocated container is
unsafe if no other GC-visible reference to that heap Value exists. The GC may
collect the heap Value since it cannot see the reference in arena memory,
resulting in a use-after-free.

Affected APIs (when the container is arena-allocated and the value is
heap-allocated):

  - [Object.Set] / [Value.Set]: the value argument is stored directly.
  - [Value.SetArrayItem]: the value argument is stored directly.
  - [AppendToArray] / [Value.AppendArrayItems]: elements are stored directly.
  - [MergeValues] / [MergeValuesWithPath]: values from b may be stored in a.

Safe patterns:

  - All values from a single arena: always safe.
  - All values on the heap (nil arena): always safe.
  - Package-level singletons (valueTrue, valueFalse, valueNull, [NullValue]):
    always safe because they are GC-visible global variables.
  - Using [DeepCopy] to copy a heap value onto the arena before storing it:
    always safe because the copy lives entirely in arena memory.

Unsafe pattern:

	arenaObj := ObjectValue(a)            // arena-allocated
	heapVal := StringValue(nil, "hello")  // heap-allocated
	arenaObj.Set(a, "key", heapVal)       // UNSAFE if heapVal has no other reference
	heapVal = nil                         // GC may now collect the heap Value

Safe pattern using DeepCopy:

	arenaObj := ObjectValue(a)
	heapVal := StringValue(nil, "hello")  // heap-allocated
	arenaObj.Set(a, "key", DeepCopy(a, heapVal))  // safe: copy lives in a
	heapVal = nil                                  // GC collects original, copy is in a

# Value Constructors

Use [StringValue], [IntValue], [FloatValue], [NumberValue], [TrueValue],
[FalseValue], [ObjectValue], and [ArrayValue] to create new values. These
accept an [arena.Arena] parameter: pass nil for heap allocation or a non-nil
arena for arena allocation. All string data is automatically copied onto the
arena when a non-nil arena is provided.

# Concurrency

[Parser], [Value], [Object], and [Scanner] cannot be used from concurrent
goroutines. Use per-goroutine instances.
*/
package astjson
