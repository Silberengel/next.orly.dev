# Effective Go - Key Points Summary

Source: https://go.dev/doc/effective_go

## Formatting

- Use `gofmt` to automatically format your code
- Indentation: use tabs
- Line length: no strict limit, but keep reasonable
- Parentheses: Go uses fewer parentheses than C/Java

## Commentary

- Every package should have a package comment
- Every exported name should have a doc comment
- Comments should be complete sentences
- Start comments with the name of the element being described

Example:
```go
// Package regexp implements regular expression search.
package regexp

// Compile parses a regular expression and returns, if successful,
// a Regexp object that can be used to match against text.
func Compile(str string) (*Regexp, error) {
```

## Names

### Package Names
- Short, concise, evocative
- Lowercase, single-word
- No underscores or mixedCaps
- Avoid stuttering (e.g., `bytes.Buffer` not `bytes.ByteBuffer`)

### Getters/Setters
- Getter: `Owner()` not `GetOwner()`
- Setter: `SetOwner()`

### Interface Names
- One-method interfaces use method name + -er suffix
- Examples: `Reader`, `Writer`, `Formatter`, `CloseNotifier`

### MixedCaps
- Use `MixedCaps` or `mixedCaps` rather than underscores

## Semicolons

- Lexer automatically inserts semicolons
- Never put opening brace on its own line

## Control Structures

### If
```go
if err := file.Chmod(0664); err != nil {
    log.Print(err)
    return err
}
```

### Redeclaration
```go
f, err := os.Open(name)
// err is declared here

d, err := f.Stat()
// err is redeclared here (same scope)
```

### For
```go
// Like a C for
for init; condition; post { }

// Like a C while
for condition { }

// Like a C for(;;)
for { }

// Range over array/slice/map/channel
for key, value := range oldMap {
    newMap[key] = value
}

// If you only need the key
for key := range m {
    // ...
}

// If you only need the value
for _, value := range array {
    // ...
}
```

### Switch
- No automatic fall through
- Cases can be expressions
- Can switch on no value (acts like if-else chain)

```go
switch {
case '0' <= c && c <= '9':
    return c - '0'
case 'a' <= c && c <= 'f':
    return c - 'a' + 10
case 'A' <= c && c <= 'F':
    return c - 'A' + 10
}
```

### Type Switch
```go
switch t := value.(type) {
case int:
    fmt.Printf("int: %d\n", t)
case string:
    fmt.Printf("string: %s\n", t)
default:
    fmt.Printf("unexpected type %T\n", t)
}
```

## Functions

### Multiple Return Values
```go
func (file *File) Write(b []byte) (n int, err error) {
    // ...
}
```

### Named Result Parameters
- Named results are initialized to zero values
- Can be used for documentation
- Enable naked returns

```go
func ReadFull(r Reader, buf []byte) (n int, err error) {
    for len(buf) > 0 && err == nil {
        var nr int
        nr, err = r.Read(buf)
        n += nr
        buf = buf[nr:]
    }
    return
}
```

### Defer
- Schedules function call to run after surrounding function returns
- LIFO order
- Arguments evaluated when defer executes

```go
func trace(s string) string {
    fmt.Println("entering:", s)
    return s
}

func un(s string) {
    fmt.Println("leaving:", s)
}

func a() {
    defer un(trace("a"))
    fmt.Println("in a")
}
```

## Data

### Allocation with new
- `new(T)` allocates zeroed storage for new item of type T
- Returns `*T`
- Returns memory address of newly allocated zero value

```go
p := new(int)   // p is *int, points to zeroed int
```

### Constructors and Composite Literals
```go
func NewFile(fd int, name string) *File {
    if fd < 0 {
        return nil
    }
    return &File{fd: fd, name: name}
}
```

### Allocation with make
- `make(T, args)` creates slices, maps, and channels only
- Returns initialized (not zeroed) value of type T (not *T)

```go
make([]int, 10, 100)   // slice: len=10, cap=100
make(map[string]int)   // map
make(chan int, 10)     // buffered channel
```

### Arrays
- Arrays are values, not pointers
- Passing array to function copies the entire array
- Array size is part of its type

### Slices
- Hold references to underlying array
- Can grow dynamically with `append`
- Passing slice passes reference

### Maps
- Hold references to underlying data structure
- Passing map passes reference
- Zero value is `nil`

### Printing
- `%v` - default format
- `%+v` - struct with field names
- `%#v` - Go syntax representation
- `%T` - type
- `%q` - quoted string

## Initialization

### Constants
- Created at compile time
- Can only be numbers, characters, strings, or booleans

### init Function
- Each source file can have `init()` function
- Called after package-level variables initialized
- Used for setup that can't be expressed as declarations

```go
func init() {
    // initialization code
}
```

## Methods

### Pointers vs. Values
- Value methods can be invoked on pointers and values
- Pointer methods can only be invoked on pointers

Rule: Value methods can be called on both values and pointers, but pointer methods should only be called on pointers (though Go allows calling on addressable values).

```go
type ByteSlice []byte

func (slice ByteSlice) Append(data []byte) []byte {
    // ...
}

func (p *ByteSlice) Append(data []byte) {
    slice := *p
    // ...
    *p = slice
}
```

## Interfaces and Other Types

### Interfaces
- A type implements an interface by implementing its methods
- No explicit declaration of intent

### Type Assertions
```go
value, ok := str.(string)
```

### Type Switches
```go
switch v := value.(type) {
case string:
    // v is string
case int:
    // v is int
}
```

### Generality
- If a type exists only to implement an interface and will never have exported methods beyond that interface, there's no need to export the type itself

## The Blank Identifier

### Unused Imports and Variables
```go
import _ "net/http/pprof"  // Import for side effects
```

### Interface Checks
```go
var _ json.Marshaler = (*RawMessage)(nil)
```

## Embedding

### Composition, not Inheritance
```go
type ReadWriter struct {
    *Reader  // *bufio.Reader
    *Writer  // *bufio.Writer
}
```

## Concurrency

### Share by Communicating
- Don't communicate by sharing memory; share memory by communicating
- Use channels to pass ownership

### Goroutines
- Cheap: small initial stack
- Multiplexed onto OS threads
- Prefix function call with `go` keyword

### Channels
- Allocate with `make`
- Unbuffered: synchronous
- Buffered: asynchronous up to buffer size

```go
ci := make(chan int)            // unbuffered
cj := make(chan int, 0)         // unbuffered
cs := make(chan *os.File, 100)  // buffered
```

### Channels of Channels
```go
type Request struct {
    args        []int
    f           func([]int) int
    resultChan  chan int
}
```

### Parallelization
```go
const numCPU = runtime.NumCPU()
runtime.GOMAXPROCS(numCPU)
```

## Errors

### Error Type
```go
type error interface {
    Error() string
}
```

### Custom Errors
```go
type PathError struct {
    Op   string
    Path string
    Err  error
}

func (e *PathError) Error() string {
    return e.Op + " " + e.Path + ": " + e.Err.Error()
}
```

### Panic
- Use for unrecoverable errors
- Generally avoid in library code

### Recover
- Called inside deferred function
- Stops panic sequence
- Returns value passed to panic

```go
func server(workChan <-chan *Work) {
    for work := range workChan {
        go safelyDo(work)
    }
}

func safelyDo(work *Work) {
    defer func() {
        if err := recover(); err != nil {
            log.Println("work failed:", err)
        }
    }()
    do(work)
}
```

## A Web Server Example

```go
package main

import (
    "fmt"
    "log"
    "net/http"
)

type Counter struct {
    n int
}

func (ctr *Counter) ServeHTTP(w http.ResponseWriter, req *http.Request) {
    ctr.n++
    fmt.Fprintf(w, "counter = %d\n", ctr.n)
}

func main() {
    ctr := new(Counter)
    http.Handle("/counter", ctr)
    log.Fatal(http.ListenAndServe(":8080", nil))
}
```

