[![Build Status](https://github.com/wundergraph/astjson/actions/workflows/ci.yml/badge.svg)](https://github.com/wundergraph/astjson/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/wundergraph/astjson.svg)](https://pkg.go.dev/github.com/wundergraph/astjson)

# astjson - fast JSON parser and validator for Go


## Features

  * Fast. Up to 15x faster than the standard [encoding/json](https://golang.org/pkg/encoding/json/).
    See [benchmarks](#benchmarks).
  * Parses arbitrary JSON without schema, reflection, struct magic and code generation.
  * Optional arena allocation for zero-GC-overhead parsing in high-throughput paths.
  * Outperforms [jsonparser](https://github.com/buger/jsonparser) and [gjson](https://github.com/tidwall/gjson)
    when accessing multiple unrelated fields, since `astjson` parses the input JSON only once.
  * Validates the parsed JSON unlike [jsonparser](https://github.com/buger/jsonparser)
    and [gjson](https://github.com/tidwall/gjson).
  * Extract, modify, and re-serialize parts of JSON with `Value.Get(...).MarshalTo`,
    [Del](https://godoc.org/github.com/wundergraph/astjson#Value.Del),
    and [Set](https://godoc.org/github.com/wundergraph/astjson#Value.Set).
  * Parses arrays containing values with distinct types (non-homogenous).
  * Preserves the original order of object keys when calling
    [Object.Visit](https://godoc.org/github.com/wundergraph/astjson#Object.Visit).


## Known limitations

  * Not safe for concurrent use. Use per-goroutine `Parser`, `Value`, and `Scanner` instances.
  * Cannot parse JSON from `io.Reader`. Use [Scanner](https://godoc.org/github.com/wundergraph/astjson#Scanner)
    for parsing a stream of JSON values from a string.


## Usage

### One-liner field access

For quick, single-field extraction from `[]byte` input:

```go
s := []byte(`{"foo": [123, "bar"]}`)
fmt.Printf("foo.0=%d\n", astjson.GetInt(s, "foo", "0"))
// Output: foo.0=123
```

Other one-liners: `GetString`, `GetBytes`, `GetFloat64`, `GetBool`, `Exists`.

### Parsing with a Parser

When you need multiple fields from the same JSON, parse once and query:

```go
var p astjson.Parser
v, err := p.Parse(`{
    "str": "bar",
    "int": 123,
    "float": 1.23,
    "bool": true,
    "arr": [1, "foo", {}]
}`)
if err != nil {
    log.Fatal(err)
}
fmt.Printf("str=%s\n", v.GetStringBytes("str"))
fmt.Printf("int=%d\n", v.GetInt("int"))
fmt.Printf("float=%f\n", v.GetFloat64("float"))
fmt.Printf("bool=%v\n", v.GetBool("bool"))
fmt.Printf("arr.1=%s\n", v.GetStringBytes("arr", "1"))
```

Use `Get` for deep path access. Array elements are accessed by index as a string key:

```go
v.Get("users", "0", "name")          // first user's name
v.GetInt("matrix", "1", "2")         // matrix[1][2] as int
```

### Arena-mode parsing

Arena mode allocates all parsed values on a monotonic arena instead of the heap,
eliminating GC pressure. See [GC & Arena Safety](#gc--arena-safety) for the rules.

```go
a := arena.NewMonotonicArena()
var p astjson.Parser
v, err := p.ParseWithArena(a, `{"name": "alice", "age": 30}`)
if err != nil {
    log.Fatal(err)
}
fmt.Printf("name=%s\n", v.GetStringBytes("name"))
// v is valid for the lifetime of a
```

The input string is copied onto the arena automatically — the caller can drop
the reference to it immediately after parsing.

### Creating values

Value constructors accept an `arena.Arena` — pass `nil` for heap allocation:

```go
a := arena.NewMonotonicArena()

s := astjson.StringValue(a, "hello")      // arena-allocated string
n := astjson.IntValue(a, 42)              // arena-allocated number
f := astjson.FloatValue(a, 3.14)          // arena-allocated float
b := astjson.TrueValue(a)                 // arena-allocated true
obj := astjson.ObjectValue(a)             // empty object
arr := astjson.ArrayValue(a)              // empty array

// Heap-allocated (pass nil):
heapStr := astjson.StringValue(nil, "heap")
```

Also available: `FalseValue`, `NumberValue` (raw numeric string), `StringValueBytes`.

### Modifying JSON

**Set object fields:**

```go
a := arena.NewMonotonicArena()
v, _ := p.ParseWithArena(a, `{"foo": 1}`)
v.Set(a, "foo", astjson.StringValue(a, "updated"))
v.Set(a, "bar", astjson.IntValue(a, 2))
fmt.Println(v) // {"foo":"updated","bar":2}
```

**Delete fields:**

```go
v.Del("foo")        // delete from object
v.Del("0")          // delete index 0 from array
```

**Set array items:**

```go
arr, _ := p.ParseWithArena(a, `[1, 2, 3]`)
arr.SetArrayItem(a, 1, astjson.StringValue(a, "two"))
fmt.Println(arr) // [1,"two",3]
```

**Append to array:**

```go
arr := astjson.ArrayValue(a)
astjson.AppendToArray(a, arr, astjson.IntValue(a, 1))
astjson.AppendToArray(a, arr, astjson.IntValue(a, 2))
fmt.Println(arr) // [1,2]
```

**Set deeply nested paths (creates intermediate objects):**

```go
v := astjson.MustParse(`{}`)
astjson.SetValue(nil, v, astjson.IntValue(nil, 42), "a", "b", "c")
fmt.Println(v) // {"a":{"b":{"c":42}}}
```

### Merging values

`MergeValues` recursively merges two values. For objects, keys from `b` are
added to or replace keys in `a`. For arrays, elements are merged pairwise
(arrays must have equal length). For scalars, `b` replaces `a` when they differ.

```go
a := arena.NewMonotonicArena()
var p astjson.Parser
base, _ := p.ParseWithArena(a, `{"name": "alice", "age": 30}`)
overlay, _ := p.ParseWithArena(a, `{"age": 31, "email": "alice@example.com"}`)

merged, changed, err := astjson.MergeValues(a, base, overlay)
fmt.Println(merged) // {"name":"alice","age":31,"email":"alice@example.com"}
```

`MergeValuesWithPath` wraps `b` in a nested object at the given path before merging:

```go
extra, _ := p.ParseWithArena(a, `"1.0"`)
merged, _, _ = astjson.MergeValuesWithPath(a, base, extra, "metadata", "version")
// equivalent to merging {"metadata":{"version":"1.0"}} into base
```

### Iterating objects

```go
v, _ := p.Parse(`{"a": 1, "b": "two", "c": [3]}`)
o, _ := v.Object()
o.Visit(func(key []byte, val *astjson.Value) {
    fmt.Printf("%s: %s\n", key, val)
})
// a: 1
// b: "two"
// c: [3]
```

Object keys are visited in their original order. The `key` slice and `val`
pointer must not be retained after the callback returns.

### Scanning a stream of JSON values

```go
var sc astjson.Scanner
sc.Init(`{"foo":"bar"} [1,2] "hello" true 42`)
for sc.Next() {
    fmt.Printf("%s\n", sc.Value())
}
if err := sc.Error(); err != nil {
    log.Fatal(err)
}
```

### Serialization

```go
// Serialize to a new byte slice:
data := v.MarshalTo(nil)

// Append to an existing buffer:
buf = v.MarshalTo(buf)

// Get a string representation:
s := v.String()
```

### DeepCopy

`DeepCopy` creates a complete copy of a value tree on the given arena. This is
the safe way to insert heap-allocated values into arena-allocated containers:

```go
a := arena.NewMonotonicArena()
obj := astjson.ObjectValue(a)

heapVal, _ := astjson.Parse(`{"nested": "data"}`)
obj.Set(a, "key", astjson.DeepCopy(a, heapVal))  // safe: copy lives in arena
```

When `a` is nil, `DeepCopy` returns the value unchanged (no-op in heap mode).


## GC & Arena Safety

This section is critical reading if you use arena mode. Misuse leads to
**silent use-after-free** bugs that are difficult to diagnose.

### Heap mode vs arena mode

| | Heap mode (`nil` arena) | Arena mode (non-nil arena) |
|---|---|---|
| **Allocation** | Standard Go heap | Monotonic arena bump allocator |
| **GC tracking** | All pointers traced normally | Arena memory is **invisible** to GC |
| **Lifetime** | Until GC collects (no live refs) | Until the arena is dropped/reset |
| **When to use** | Simple cases, low throughput | High throughput, request-scoped work |

### The noscan invariant

Arena buffers are allocated as `[]byte` slabs. Go's GC treats `[]byte` as
containing no pointers — it never scans inside them. This means **any pointer
stored in arena memory is invisible to the GC**. This includes:

  * `Value.a` (`[]*Value`) — array element pointers
  * `Object.kvs` (`[]*kv`) — key-value entry pointers
  * `kv.v` (`*Value`) — value pointers inside object entries
  * `Value.s` / `kv.k` (`string`) — string data pointers

The library maintains safety by copying all string data onto the arena when
a non-nil arena is provided. But **pointer fields to Value structs** are the
caller's responsibility when inserting values across allocation boundaries.

### The golden rule

> **Never store a heap-allocated `*Value` into an arena-allocated container
> unless another GC-visible reference keeps it alive.**

If the only reference to a heap Value lives in arena memory, the GC cannot see
it and may collect it, causing a use-after-free. Use `DeepCopy` to copy the
value onto the arena first.

**Unsafe:**
```go
arenaObj := astjson.ObjectValue(a)            // arena-allocated
heapVal := astjson.StringValue(nil, "hello")  // heap-allocated
arenaObj.Set(a, "key", heapVal)               // UNSAFE: GC can't see this ref
heapVal = nil                                 // GC may collect it
```

**Safe — use DeepCopy:**
```go
arenaObj := astjson.ObjectValue(a)
heapVal := astjson.StringValue(nil, "hello")
arenaObj.Set(a, "key", astjson.DeepCopy(a, heapVal))  // safe: copy lives in arena
```

**Also safe — all values from the same arena:**
```go
arenaObj := astjson.ObjectValue(a)
arenaVal := astjson.StringValue(a, "hello")
arenaObj.Set(a, "key", arenaVal)  // safe: both on same arena
```

### Affected APIs

These APIs store value pointers directly. When the container is arena-allocated,
the value must also be arena-allocated (same arena) or a package-level singleton:

  * `Object.Set` / `Value.Set` — stores the value argument directly
  * `Value.SetArrayItem` — stores the value argument directly
  * `AppendToArray` / `Value.AppendArrayItems` — stores elements directly
  * `MergeValues` / `MergeValuesWithPath` — may store values from `b` into `a`

Package-level singletons (`NullValue`, and the internal `valueTrue`, `valueFalse`,
`valueNull`) are always safe because they are GC-visible global variables.

### Arena lifetime

Keep the arena alive for as long as any values allocated on it are in use.
In performance-sensitive code where the compiler might not prove liveness,
use `runtime.KeepAlive`:

```go
a := arena.NewMonotonicArena()
v, _ := p.ParseWithArena(a, input)

// ... use v ...
result := v.MarshalTo(nil)

runtime.KeepAlive(a) // ensure arena outlives all uses of v
```

### Don't mix arenas

All values in a single tree should come from the same arena. If `MergeValues`
returns a value from `b` and `a`/`b` were allocated on different arenas,
resetting one arena while the other is still in use causes silent corruption.


## Best Practices

  * **One arena per unit of work.** Create an arena at the start of a request,
    parse and build values on it, serialize the result, then let the arena be
    collected. This gives you a clear, bounded lifetime.
  * **Use `DeepCopy` at boundaries.** When inserting a value from an unknown
    source (different arena, heap, parsed separately) into an arena container,
    wrap it in `DeepCopy(a, val)`. This is a no-op when `a` is nil.
  * **Prefer arena mode for hot paths.** Arena mode avoids per-Value heap
    allocations, reducing GC pause time in high-throughput services.
  * **Use heap mode for simplicity.** If GC pressure is not a concern, pass
    `nil` for the arena. The GC handles all lifetimes and there are no mixing
    pitfalls.
  * **Don't share values across goroutines.** `Parser`, `Value`, `Object`, and
    `Scanner` are not safe for concurrent use. Use per-goroutine instances.
  * **Use `runtime.KeepAlive(arena)` when in doubt.** If there is any chance
    the compiler could determine the arena is unused before you finish reading
    values from it, add a `KeepAlive` call after the last use.
  * **Deduplicate keys if merging from untrusted sources.**
    `DeduplicateObjectKeysRecursively(v)` removes duplicate object keys
    in place, keeping the first occurrence.


## Security

  * `astjson` shouldn't crash or panic when parsing input strings specially crafted
    by an attacker. It must return error on invalid input JSON.
  * `astjson` requires up to `sizeof(Value) * len(inputJSON)` bytes of memory
    for parsing `inputJSON` string. Limit the maximum size of the `inputJSON`
    before parsing it in order to limit the maximum memory usage.


## Performance optimization tips

  * Prefer calling `Value.Get*` on the value returned from `Parser`
    instead of calling `Get*` one-liners when multiple fields
    must be obtained from JSON, since each `Get*` one-liner re-parses
    the input JSON again.
  * Prefer calling once `Value.Get` for common prefix paths and then calling
    `Value.Get*` on the returned value for distinct suffix paths.
  * Prefer iterating over the array returned from `Value.GetArray`
    with a range loop instead of calling `Value.Get*` for each array item.

## Fuzzing
Install [go-fuzz](https://github.com/dvyukov/go-fuzz) & optionally the go-fuzz-corpus.

```bash
go get -u github.com/dvyukov/go-fuzz/go-fuzz github.com/dvyukov/go-fuzz/go-fuzz-build
```

Build using `go-fuzz-build` and run `go-fuzz` with an optional corpus.

```bash
mkdir -p workdir/corpus
cp $GOPATH/src/github.com/dvyukov/go-fuzz-corpus/json/corpus/* workdir/corpus
go-fuzz-build github.com/wundergraph/astjson
go-fuzz -bin=astjson-fuzz.zip -workdir=workdir
```

## Benchmarks

Go 1.12 has been used for benchmarking.

Legend:

  * `small` - parse [small.json](testdata/small.json) (190 bytes).
  * `medium` - parse [medium.json](testdata/medium.json) (2.3KB).
  * `large` - parse [large.json](testdata/large.json) (28KB).
  * `canada` - parse [canada.json](testdata/canada.json) (2.2MB).
  * `citm` - parse [citm_catalog.json](testdata/citm_catalog.json) (1.7MB).
  * `twitter` - parse [twitter.json](testdata/twitter.json) (617KB).

  * `stdjson-map` - parse into a `map[string]interface{}` using `encoding/json`.
  * `stdjson-struct` - parse into a struct containing
    a subset of fields of the parsed JSON, using `encoding/json`.
  * `stdjson-empty-struct` - parse into an empty struct using `encoding/json`.
    This is the fastest possible solution for `encoding/json`, may be used
    for json validation. See also benchmark results for json validation.
  * `fastjson` - parse using `fastjson` without fields access.
  * `fastjson-get` - parse using `fastjson` with fields access similar to `stdjson-struct`.

```
$ GOMAXPROCS=1 go test github.com/wundergraph/astjson -bench='Parse$'
goos: linux
goarch: amd64
pkg: github.com/wundergraph/astjson
BenchmarkParse/small/stdjson-map         	  200000	      7305 ns/op	  26.01 MB/s	     960 B/op	      51 allocs/op
BenchmarkParse/small/stdjson-struct      	  500000	      3431 ns/op	  55.37 MB/s	     224 B/op	       4 allocs/op
BenchmarkParse/small/stdjson-empty-struct         	  500000	      2273 ns/op	  83.58 MB/s	     168 B/op	       2 allocs/op
BenchmarkParse/small/fastjson                     	 5000000	       347 ns/op	 547.53 MB/s	       0 B/op	       0 allocs/op
BenchmarkParse/small/fastjson-get                 	 2000000	       620 ns/op	 306.39 MB/s	       0 B/op	       0 allocs/op
BenchmarkParse/medium/stdjson-map                 	   30000	     40672 ns/op	  57.26 MB/s	   10196 B/op	     208 allocs/op
BenchmarkParse/medium/stdjson-struct              	   30000	     47792 ns/op	  48.73 MB/s	    9174 B/op	     258 allocs/op
BenchmarkParse/medium/stdjson-empty-struct        	  100000	     22096 ns/op	 105.40 MB/s	     280 B/op	       5 allocs/op
BenchmarkParse/medium/fastjson                    	  500000	      3025 ns/op	 769.90 MB/s	       0 B/op	       0 allocs/op
BenchmarkParse/medium/fastjson-get                	  500000	      3211 ns/op	 725.20 MB/s	       0 B/op	       0 allocs/op
BenchmarkParse/large/stdjson-map                  	    2000	    614079 ns/op	  45.79 MB/s	  210734 B/op	    2785 allocs/op
BenchmarkParse/large/stdjson-struct               	    5000	    298554 ns/op	  94.18 MB/s	   15616 B/op	     353 allocs/op
BenchmarkParse/large/stdjson-empty-struct         	    5000	    268577 ns/op	 104.69 MB/s	     280 B/op	       5 allocs/op
BenchmarkParse/large/fastjson                     	   50000	     35210 ns/op	 798.56 MB/s	       5 B/op	       0 allocs/op
BenchmarkParse/large/fastjson-get                 	   50000	     35171 ns/op	 799.46 MB/s	       5 B/op	       0 allocs/op
BenchmarkParse/canada/stdjson-map                 	      20	  68147307 ns/op	  33.03 MB/s	12260502 B/op	  392539 allocs/op
BenchmarkParse/canada/stdjson-struct              	      20	  68044518 ns/op	  33.08 MB/s	12260123 B/op	  392534 allocs/op
BenchmarkParse/canada/stdjson-empty-struct        	     100	  17709250 ns/op	 127.11 MB/s	     280 B/op	       5 allocs/op
BenchmarkParse/canada/fastjson                    	     300	   4182404 ns/op	 538.22 MB/s	  254902 B/op	     381 allocs/op
BenchmarkParse/canada/fastjson-get                	     300	   4274744 ns/op	 526.60 MB/s	  254902 B/op	     381 allocs/op
BenchmarkParse/citm/stdjson-map                   	      50	  27772612 ns/op	  62.19 MB/s	 5214163 B/op	   95402 allocs/op
BenchmarkParse/citm/stdjson-struct                	     100	  14936191 ns/op	 115.64 MB/s	    1989 B/op	      75 allocs/op
BenchmarkParse/citm/stdjson-empty-struct          	     100	  14946034 ns/op	 115.56 MB/s	     280 B/op	       5 allocs/op
BenchmarkParse/citm/fastjson                      	    1000	   1879714 ns/op	 918.87 MB/s	   17628 B/op	      30 allocs/op
BenchmarkParse/citm/fastjson-get                  	    1000	   1881598 ns/op	 917.94 MB/s	   17628 B/op	      30 allocs/op
BenchmarkParse/twitter/stdjson-map                	     100	  11289146 ns/op	  55.94 MB/s	 2187878 B/op	   31266 allocs/op
BenchmarkParse/twitter/stdjson-struct             	     300	   5779442 ns/op	 109.27 MB/s	     408 B/op	       6 allocs/op
BenchmarkParse/twitter/stdjson-empty-struct       	     300	   5738504 ns/op	 110.05 MB/s	     408 B/op	       6 allocs/op
BenchmarkParse/twitter/fastjson                   	    2000	    774042 ns/op	 815.86 MB/s	    2541 B/op	       2 allocs/op
BenchmarkParse/twitter/fastjson-get               	    2000	    777833 ns/op	 811.89 MB/s	    2541 B/op	       2 allocs/op
```

Benchmark results for json validation:

```
$ GOMAXPROCS=1 go test github.com/wundergraph/astjson -bench='Validate$'
goos: linux
goarch: amd64
pkg: github.com/wundergraph/astjson
BenchmarkValidate/small/stdjson 	 2000000	       955 ns/op	 198.83 MB/s	      72 B/op	       2 allocs/op
BenchmarkValidate/small/fastjson         	 5000000	       384 ns/op	 493.60 MB/s	       0 B/op	       0 allocs/op
BenchmarkValidate/medium/stdjson         	  200000	     10799 ns/op	 215.66 MB/s	     184 B/op	       5 allocs/op
BenchmarkValidate/medium/fastjson        	  300000	      3809 ns/op	 611.30 MB/s	       0 B/op	       0 allocs/op
BenchmarkValidate/large/stdjson          	   10000	    133064 ns/op	 211.31 MB/s	     184 B/op	       5 allocs/op
BenchmarkValidate/large/fastjson         	   30000	     45268 ns/op	 621.14 MB/s	       0 B/op	       0 allocs/op
BenchmarkValidate/canada/stdjson         	     200	   8470904 ns/op	 265.74 MB/s	     184 B/op	       5 allocs/op
BenchmarkValidate/canada/fastjson        	     500	   2973377 ns/op	 757.07 MB/s	       0 B/op	       0 allocs/op
BenchmarkValidate/citm/stdjson           	     200	   7273172 ns/op	 237.48 MB/s	     184 B/op	       5 allocs/op
BenchmarkValidate/citm/fastjson          	    1000	   1684430 ns/op	1025.39 MB/s	       0 B/op	       0 allocs/op
BenchmarkValidate/twitter/stdjson        	     500	   2849439 ns/op	 221.63 MB/s	     312 B/op	       6 allocs/op
BenchmarkValidate/twitter/fastjson       	    2000	   1036796 ns/op	 609.10 MB/s	       0 B/op	       0 allocs/op
```

## FAQ

  * Q: _There are a ton of other high-perf packages for JSON parsing in Go. Why creating yet another package?_
    A: Because other packages require either rigid JSON schema via struct magic
       and code generation or perform poorly when multiple unrelated fields
       must be obtained from the parsed JSON.
       Additionally, `astjson` provides nicer [API](http://godoc.org/github.com/wundergraph/astjson).

  * Q: _What is the main purpose for `astjson`?_
    A: High-perf JSON parsing for [RTB](https://www.iab.com/wp-content/uploads/2015/05/OpenRTB_API_Specification_Version_2_3_1.pdf)
       and other [JSON-RPC](https://en.wikipedia.org/wiki/JSON-RPC) services.

  * Q: _Why doesn't astjson provide fast marshaling (serialization)?_
    A: It provides `Value.MarshalTo` for serializing parsed/constructed JSON trees.
       For high-performance templated JSON marshaling, consider
       [quicktemplate](https://github.com/valyala/quicktemplate#use-cases).

  * Q: _`astjson` crashes my program!_
    A: There is high probability of improper use.
       * Make sure you don't hold references to objects recursively returned by `Parser` / `Scanner`
         beyond the next `Parser.Parse` / `Scanner.Next` call.
       * Make sure you don't access `astjson` objects from concurrently running goroutines.
       * If using arena mode, read the [GC & Arena Safety](#gc--arena-safety) section carefully.
         Mixing heap and arena values without `DeepCopy` causes silent use-after-free.
       * Build and run your program with [-race](https://golang.org/doc/articles/race_detector.html) flag.
         Make sure the race detector detects zero races.
       * If your program continues crashing after fixing the issues above, [file a bug](https://github.com/wundergraph/astjson/issues/new).
