# Go Reference Type Simplification - Revised Proposal

## Executive Summary

Keep Go's convenient syntax (slicing, `<-`, `for range`) while making reference semantics **explicit through pointer types**. This reduces cognitive load and improves safety without sacrificing ergonomics.

## Core Principle: Explicit Pointers, Convenient Syntax

**The Key Insight:**
- Make slices/maps/channels explicitly `*[]T`, `*map[K]V`, `*chan T`
- Keep convenient operators (auto-dereference like struct methods do)
- Eliminate special allocation functions (`make()`)
- Add explicit control where it matters (grow, clone)

## Proposed Changes

### 1. Slices Become `*[]T` (Explicit Pointers)

**Current Problem:**
```go
s := []int{1, 2, 3}        // Looks like value, is reference
s2 := s                    // Copies reference - HIDDEN SHARING
s2[0] = 99                 // Mutates s too! Not obvious
```

**Proposed:**
```go
s := &[]int{1, 2, 3}       // Explicit pointer allocation
s2 := s                    // Copies pointer - OBVIOUS SHARING
s2[0] = 99                 // Mutates s too - but now obvious!

// Slicing still works (auto-dereference)
sub := s[1:3]              // Returns *[]int (new slice header, same backing)
sub := s[1:3:5]            // Full slicing with capacity still works

// To copy data, be explicit
s3 := s.Clone()            // Deep copy
s3 := &[]int(*s)           // Alternative: copy via literal

// Append works as before
s.Append(4, 5, 6)          // Implicit grow if needed (fine!)
s.Grow(100)                // Explicit capacity increase
```

**What Changes:**
- ✅ Allocation: `&[]T{}` instead of `make([]T, len, cap)`
- ✅ Type: `*[]int` instead of `[]int`
- ✅ Explicit clone: Must call `.Clone()` to copy data
- ✅ Explicit grow: `.Grow(n)` for pre-allocation
- ❌ Slicing syntax: **KEEP IT** - `s[i:j]` still works
- ❌ Append behavior: **KEEP IT** - implicit growth is fine
- ❌ Auto-dereference: Like methods, `s[i]` auto-derefs

**Benefits:**
- Assignment `s2 := s` is obviously pointer copy
- Function parameters `func f(s *[]int)` show mutation potential
- Still convenient: slicing and indexing work as before

### 2. Maps Become `*map[K]V` (Explicit Pointers)

**Current Problem:**
```go
m := make(map[string]int)  // Special make() function
m2 := m                    // HIDDEN reference sharing

var m3 map[string]int      // nil map
v := m3["key"]             // OK - returns zero value
m3["key"] = 42             // PANIC! Nil map write trap
```

**Proposed:**
```go
m := &map[string]int{}     // Explicit pointer allocation
m := &map[string]int{      // Literal initialization
    "key": 42,
}

m2 := m                    // Obviously copies pointer

// Map operations auto-dereference
m["key"] = 42              // Auto-deref (like s[i] for slices)
v := m["key"]
v, ok := m["key"]

// Nil pointer is consistent
var m3 *map[string]int     // nil pointer
v := m3["key"]             // PANIC - nil pointer deref (consistent!)
m3 = &map[string]int{}     // Must allocate
m3["key"] = 42             // Now OK

// Copying requires explicit clone
m4 := m.Clone()            // Deep copy
```

**What Changes:**
- ✅ Allocation: `&map[K]V{}` instead of `make(map[K]V)`
- ✅ Type: `*map[K]V` instead of `map[K]V`
- ✅ Nil behavior: Consistent nil pointer panic
- ✅ Explicit clone: Must call `.Clone()`
- ❌ Map syntax: **KEEP IT** - `m[k]` auto-derefs

**Benefits:**
- Obvious pointer semantics
- No special nil-map read-only trap
- Clear when data is shared

### 3. Channels Become `*chan T` (Explicit Pointers)

**Current Problem:**
```go
ch := make(chan int, 10)   // Special make() function
ch2 := ch                  // HIDDEN reference sharing

var ch3 chan int           // nil channel
ch3 <- 42                  // BLOCKS FOREVER! Silent deadlock trap
```

**Proposed:**
```go
ch := &chan int{cap: 10}   // Explicit pointer allocation
ch := &chan int{}          // Unbuffered (cap: 0)

ch2 := ch                  // Obviously copies pointer

// Channel operations auto-dereference
ch <- 42                   // KEEP <- syntax!
v := <-ch
v, ok := <-ch

// for range still works
for v := range ch {        // KEEP for range!
    process(v)
}

// select still works
select {                   // KEEP select!
case v := <-ch:
    handle(v)
case ch2 <- 42:
    sent()
}

// Nil pointer is consistent
var ch3 *chan int          // nil pointer
ch3 <- 42                  // PANIC - nil pointer deref (consistent!)

// Directional channels as type aliases or interfaces
type SendOnly[T any] = *chan T   // Could restrict at type level
func send(ch *chan int) {}       // Or just document convention
```

**What Changes:**
- ✅ Allocation: `&chan T{cap: n}` instead of `make(chan T, n)`
- ✅ Type: `*chan T` instead of `chan T`
- ✅ Nil behavior: Consistent nil pointer panic
- ❌ Send/receive: **KEEP `<-` syntax**
- ❌ Select: **KEEP `select` statement**
- ❌ For range: **KEEP `for range ch`**

**Benefits:**
- Obvious pointer semantics
- No silent nil-channel blocking trap
- Keep all the convenient syntax
- Directional types could be interfaces if needed

### 4. Unified Allocation: Eliminate `make()`

**Before (Three Allocation Primitives):**
```go
new(T)                     // Returns *T (zero value)
make([]T, len, cap)        // Returns []T (special)
make(map[K]V, hint)        // Returns map[K]V (special)
make(chan T, buf)          // Returns chan T (special)
```

**After (One Allocation Syntax):**
```go
new(T)                     // Returns *T (zero value)
&T{}                       // Returns *T (composite literal)
&[]T{}                     // Returns *[]T (empty slice)
&[n]T{}                    // Returns *[n]T (array)
&map[K]V{}                 // Returns *map[K]V (empty map)
&chan T{}                  // Returns *chan T (unbuffered)
&chan T{cap: 10}           // Returns *chan T (buffered)
```

**Eliminate:**
- ❌ `make()` entirely
- ❌ Special capacity/hint parameters (use methods instead)

### 5. Type System Unification

**Before:**
```
Value types: int, float, bool, struct, [N]T
Reference types: []T, map[K]V, chan T (SPECIAL SEMANTICS)
Pointer types: *T
```

**After:**
```
Value types: int, float, bool, struct, [N]T
Pointer types: *T (including *[]T, *map[K]V, *chan T - UNIFIED)
```

All pointer types have consistent semantics:
- Assignment copies the pointer
- Nil pointer dereference panics consistently
- Auto-dereference for convenient syntax
- Explicit `.Clone()` for deep copy

## Syntax Comparison

### Slices

**Before:**
```go
// Many ways to create
var s []int                          // nil slice
s = []int{}                          // empty slice
s = make([]int, 10)                  // len=10, cap=10
s = make([]int, 10, 20)              // len=10, cap=20
s = []int{1, 2, 3}                   // literal

// Slicing
sub := s[1:3]                        // subslice
sub = s[:3]                          // from start
sub = s[1:]                          // to end
sub = s[:]                           // full slice
sub = s[1:3:5]                       // with capacity

// Append
s = append(s, 4)                     // might reallocate
s = append(s, items...)              // spread

// Copy (manual)
s2 := make([]int, len(s))
copy(s2, s)
```

**After:**
```go
// One way to create
var s *[]int                         // nil pointer
s = &[]int{}                         // empty slice
s = &[10]int{}[:]                    // len=10 from array
s = &[]int{1, 2, 3}                  // literal

// Slicing (UNCHANGED)
sub := s[1:3]                        // auto-deref, returns *[]int
sub = s[:3]
sub = s[1:]
sub = s[:]
sub = s[1:3:5]

// Append (UNCHANGED)
s.Append(4)                          // might reallocate (fine!)
s.Append(items...)                   // spread

// Explicit operations
s.Grow(100)                          // pre-allocate capacity
s2 := s.Clone()                      // explicit deep copy
```

### Maps

**Before:**
```go
// Many ways to create
var m map[K]V                        // nil map
m = map[K]V{}                        // empty map
m = make(map[K]V)                    // empty map
m = make(map[K]V, 100)               // with hint
m = map[K]V{k: v}                    // literal

// Access
m[k] = v
v = m[k]
v, ok = m[k]

// Copy (manual)
m2 := make(map[K]V, len(m))
for k, v := range m {
    m2[k] = v
}
```

**After:**
```go
// One way to create
var m *map[K]V                       // nil pointer
m = &map[K]V{}                       // empty map
m = &map[K]V{k: v}                   // literal

// Access (UNCHANGED)
m[k] = v                             // auto-deref
v = m[k]
v, ok = m[k]

// Explicit operations
m2 := m.Clone()                      // explicit deep copy
```

### Channels

**Before:**
```go
// Create
ch := make(chan int)                 // unbuffered
ch := make(chan int, 10)             // buffered

// Operations
ch <- 42                             // send
v := <-ch                            // receive
v, ok := <-ch                        // receive with closed check
close(ch)

// for range
for v := range ch {
    process(v)
}

// select
select {
case v := <-ch:
    handle(v)
case <-timeout:
    timeout()
}
```

**After:**
```go
// Create
ch := &chan int{}                    // unbuffered
ch := &chan int{cap: 10}             // buffered

// Operations (UNCHANGED)
ch <- 42                             // auto-deref
v := <-ch
v, ok := <-ch
ch.Close()                           // method instead of builtin

// for range (UNCHANGED)
for v := range ch {
    process(v)
}

// select (UNCHANGED)
select {
case v := <-ch:
    handle(v)
case <-timeout:
    timeout()
}
```

## Grammar Simplification

### Eliminated Syntax

1. **`make()` builtin** - 3 different forms → 0
   - `make([]T, n, cap)` → `&[]T{}` + `.Grow(cap)`
   - `make(map[K]V, hint)` → `&map[K]V{}`
   - `make(chan T, buf)` → `&chan T{cap: buf}`

2. **Dual allocation semantics** - 2 primitives → 1
   - `new(T)` and `make(T)` → just `new(T)` or `&T{}`

### Preserved Syntax

1. ✅ Slice expressions: `s[i:j]`, `s[i:j:k]`
2. ✅ Channel operators: `<-ch`, `ch<-`
3. ✅ Select statement: `select { case ... }`
4. ✅ Range over channels: `for v := range ch`
5. ✅ Map/slice indexing: `m[k]`, `s[i]`
6. ✅ Auto-dereference: Like methods on `*T`

## New Built-in Methods

### Slices (`*[]T`)

```go
s := &[]int{1, 2, 3}

// Capacity management
s.Grow(n int)              // Ensure capacity for n more elements
s.Cap() int                // Current capacity
s.Len() int                // Current length

// Modification
s.Append(items ...T)       // Append items (implicit grow OK)
s.Insert(i int, items ...T) // Insert at index
s.Delete(i, j int)         // Delete s[i:j]
s.Clear()                  // Set length to 0

// Copying
s.Clone() *[]T             // Deep copy
s.Slice(i, j int) *[]T     // Alternative to s[i:j]
```

### Maps (`*map[K]V`)

```go
m := &map[string]int{}

// Capacity
m.Len() int                // Number of keys

// Modification
m.Clear()                  // Remove all keys
m.Delete(k K)              // Delete key

// Copying
m.Clone() *map[K]V         // Deep copy

// Bulk operations
m.Keys() *[]K              // All keys
m.Values() *[]V            // All values
m.Merge(other *map[K]V)    // Merge other into m
```

### Channels (`*chan T`)

```go
ch := &chan int{cap: 10}

// Metadata
ch.Len() int               // Items in buffer
ch.Cap() int               // Buffer capacity

// Control
ch.Close()                 // Close channel (method vs builtin)
```

## Auto-Dereference Rules

Like struct methods today, pointer types auto-dereference:

```go
type Person struct { name string }
func (p *Person) Name() string { return p.name }

p := &Person{name: "Alice"}
n := p.Name()              // Auto-deref: (*p).Name()

// Same for new pointer types
s := &[]int{1, 2, 3}
v := s[0]                  // Auto-deref: (*s)[0]
sub := s[1:3]              // Auto-deref: (*s)[1:3]

m := &map[K]V{}
v = m[k]                   // Auto-deref: (*m)[k]

ch := &chan int{}
ch <- 42                   // Auto-deref: (*ch) <- 42
v = <-ch                   // Auto-deref: <-(*ch)
```

**Rule:** Pointer to slice/map/channel auto-derefs for indexing, slicing, and channel ops.

## Concurrency Safety

### Before: Implicit Sharing

```go
func worker(s []int, wg *sync.WaitGroup) {
    defer wg.Done()
    s[0] = 99                // RACE - not obvious from signature
}

s := []int{1, 2, 3}
var wg sync.WaitGroup
wg.Add(2)
go worker(s, &wg)            // Sharing not obvious
go worker(s, &wg)            // Two goroutines mutate same slice
wg.Wait()
```

### After: Explicit Sharing

```go
func worker(s *[]int, wg *sync.WaitGroup) {
    defer wg.Done()
    (*s)[0] = 99             // RACE - but obvious from *[]int
}

s := &[]int{1, 2, 3}
var wg sync.WaitGroup
wg.Add(2)
go worker(s, &wg)            // OBVIOUS pointer sharing
go worker(s, &wg)            // Clear that both access same data
wg.Wait()
```

**Benefits:**
- Function signature shows mutation: `func f(s *[]int)` vs `func f(s []int)`
- Pointer copy is obvious: `s2 := s` (copies pointer)
- Value copy requires explicit clone: `s2 := s.Clone()`

### Pattern: Immutable by Default

```go
// Current Go - unclear if mutation happens
func ProcessSlice(s []int) []int {
    s[0] = 99                // Mutates caller's slice!
    return s
}

// Proposed - explicit mutation
func ProcessSlice(s *[]int) {
    (*s)[0] = 99             // Clear mutation
}

// Or value semantics (copy)
func ProcessSlice(s []int) []int {  // Note: NOT pointer
    result := &[]int(s)      // Explicit copy from value
    (*result)[0] = 99        // Mutate copy
    return result
}
```

## Migration Path

### Phase 1: Allow Both (Backward Compatible)

```go
// Old style still works
s := []int{1, 2, 3}
s = append(s, 4)

// New style also works (same runtime behavior)
s := &[]int{1, 2, 3}
s.Append(4)

// Add deprecation warnings
make([]int, 10)              // WARNING: Use &[]int{} or &[10]int{}[:]
```

### Phase 2: Deprecate Old Forms

```go
// Compiler warnings
[]int{1, 2, 3}               // WARNING: Use &[]int{1, 2, 3}
make([]int, 10)              // WARNING: Use &[]int{} with .Grow(10)
make(map[K]V)                // WARNING: Use &map[K]V{}
make(chan T, 10)             // WARNING: Use &chan T{cap: 10}
```

### Phase 3: Breaking Change

```go
// Only new syntax allowed
&[]int{1, 2, 3}              // OK
&map[K]V{}                   // OK
&chan T{cap: 10}             // OK

[]int{1, 2, 3}               // ERROR: Use &[]int{1, 2, 3}
make([]int, 10)              // ERROR: Removed
```

## Implementation Impact

### Compiler Changes

**New:**
- Auto-dereference for `*[]T`, `*map[K]V`, `*chan T`
- Built-in methods (`.Append()`, `.Clone()`, `.Grow()`, etc.)
- Composite literal fields: `&chan T{cap: 10}`

**Removed:**
- `make()` builtin (3 forms)
- Special case type checking for reference types

**Preserved:**
- Slice expressions `s[i:j:k]`
- Channel operators `<-`
- Select statement
- Range over channels
- All runtime implementations

### Runtime Changes

**Minimal:**
- Same memory layout for slices/maps/channels
- Same GC behavior
- Same scheduler
- No performance impact

**API:**
- Add runtime functions for `.Clone()`, `.Grow()`, etc.
- These can be compiler intrinsics for performance

## Complexity Reduction

| Metric | Before | After | Reduction |
|--------|--------|-------|-----------|
| **Allocation primitives** | 2 (`new`, `make`) | 1 (`&T{}`) | **50%** |
| **make() forms** | 3 (slice, map, chan) | 0 | **100%** |
| **Reference type special cases** | 3 types | 0 (unified) | **100%** |
| **Nil traps** | 2 (nil map write, nil chan) | 0 (consistent panic) | **100%** |
| **Type system categories** | 3 (value, ref, ptr) | 2 (value, ptr) | **33%** |
| **Syntax variants preserved** | Slicing, `<-`, select, range | All kept | **0%** |

**Total complexity reduction: ~30%** while keeping ergonomic syntax.

## Real-World Example: ORLY Codebase

### Before

```go
// pkg/database/query-events.go
func QueryEvents(db *badger.DB, filter *filter.T) ([]uint64, error) {
    results := make([]uint64, 0, 1000)
    // ... query logic
    return results, nil
}

// Caller must handle returned slice
events, err := QueryEvents(db, f)
if err != nil {
    return err
}
events = append(events, moreEvents...)  // Might copy
```

### After

```go
// pkg/database/query-events.go
func QueryEvents(db *badger.DB, filter *filter.T) (results *[]uint64, err error) {
    results = &[]uint64{}
    results.Grow(1000)           // Explicit capacity
    // ... query logic
    return
}

// Caller gets explicit pointer
events, err := QueryEvents(db, f)
if chk.E(err) {
    return
}
events.Append(moreEvents...)     // Clear mutation
```

**Benefits in ORLY:**
- Clear which functions mutate vs return new data
- Obvious when slices are shared across goroutines
- Explicit capacity management for performance-critical code
- No hidden allocations from append

## Conclusion

### What We Keep
✅ Slice expressions: `s[1:3:5]`
✅ Channel operators: `<-`
✅ Select statement
✅ For range channels
✅ Implicit append growth
✅ Convenient auto-dereference

### What We Gain
✅ Explicit pointer semantics
✅ Obvious data sharing
✅ Consistent nil behavior
✅ Unified type system
✅ Simpler language (no `make()`)
✅ Better concurrency safety

### What We Lose
❌ `make()` function (replaced by `&T{}`)
❌ Implicit reference types (now explicit `*[]T`)
❌ Zero-value usability for maps/slices (must allocate)

### Recommendation

This revision strikes the right balance:
- **Keep** Go's ergonomic syntax that makes it productive
- **Add** explicit semantics that make code safer and clearer
- **Remove** only the truly confusing parts (`make()`, implicit references)
- **Gain** ~30% complexity reduction without sacrificing convenience

The migration is straightforward and could be done gradually with good tooling support.
