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
