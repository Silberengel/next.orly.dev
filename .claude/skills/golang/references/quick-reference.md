# Go Quick Reference Cheat Sheet

## Basic Syntax

### Hello World
```go
package main

import "fmt"

func main() {
    fmt.Println("Hello, World!")
}
```

### Variables
```go
var name string = "John"
var age int = 30
var height = 5.9  // type inference

// Short declaration (inside functions only)
count := 42
```

### Constants
```go
const Pi = 3.14159
const (
    Sunday = iota  // 0
    Monday         // 1
    Tuesday        // 2
)
```

## Data Types

### Basic Types
```go
bool                        // true, false
string                      // "hello"
int  int8  int16  int32  int64
uint uint8 uint16 uint32 uint64
byte                        // alias for uint8
rune                        // alias for int32 (Unicode)
float32 float64
complex64 complex128
```

### Composite Types
```go
// Array (fixed size)
var arr [5]int

// Slice (dynamic)
slice := []int{1, 2, 3}
slice = append(slice, 4)

// Map
m := make(map[string]int)
m["key"] = 42

// Struct
type Person struct {
    Name string
    Age  int
}
p := Person{Name: "Alice", Age: 30}

// Pointer
ptr := &p
```

## Functions

```go
// Basic function
func add(a, b int) int {
    return a + b
}

// Named returns (preferred)
func divide(a, b float64) (result float64, err error) {
    if b == 0 {
        err = errors.New("division by zero")
        return
    }
    result = a / b
    return
}

// Variadic
func sum(nums ...int) int {
    total := 0
    for _, n := range nums {
        total += n
    }
    return total
}

// Multiple returns
func swap(a, b int) (int, int) {
    return b, a
}
```

## Control Flow

### If/Else
```go
if x > 0 {
    // positive
} else if x < 0 {
    // negative
} else {
    // zero
}

// With initialization
if err := doSomething(); err != nil {
    return err
}
```

### For Loops
```go
// Traditional for
for i := 0; i < 10; i++ {
    fmt.Println(i)
}

// While-style
for condition {
}

// Infinite
for {
}

// Range
for i, v := range slice {
    fmt.Printf("%d: %v\n", i, v)
}

for key, value := range myMap {
    fmt.Printf("%s: %v\n", key, value)
}
```

### Switch
```go
switch x {
case 1:
    fmt.Println("one")
case 2, 3:
    fmt.Println("two or three")
default:
    fmt.Println("other")
}

// Type switch
switch v := i.(type) {
case int:
    fmt.Printf("int: %d\n", v)
case string:
    fmt.Printf("string: %s\n", v)
}
```

## Methods & Interfaces

### Methods
```go
type Rectangle struct {
    Width, Height float64
}

// Value receiver
func (r Rectangle) Area() float64 {
    return r.Width * r.Height
}

// Pointer receiver
func (r *Rectangle) Scale(factor float64) {
    r.Width *= factor
    r.Height *= factor
}
```

### Interfaces
```go
type Shape interface {
    Area() float64
    Perimeter() float64
}

// Empty interface (any type)
var x interface{}  // or: var x any
```

## Concurrency

### Goroutines
```go
go doSomething()

go func() {
    fmt.Println("In goroutine")
}()
```

### Channels
```go
// Create
ch := make(chan int)        // unbuffered
ch := make(chan int, 10)    // buffered

// Send & Receive
ch <- 42        // send
value := <-ch   // receive

// Close
close(ch)

// Check if closed
value, ok := <-ch
```

### Select
```go
select {
case msg := <-ch1:
    fmt.Println("ch1:", msg)
case msg := <-ch2:
    fmt.Println("ch2:", msg)
case <-time.After(1 * time.Second):
    fmt.Println("timeout")
default:
    fmt.Println("no channel ready")
}
```

### Sync Package
```go
// Mutex
var mu sync.Mutex
mu.Lock()
defer mu.Unlock()

// RWMutex
var mu sync.RWMutex
mu.RLock()
defer mu.RUnlock()

// WaitGroup
var wg sync.WaitGroup
wg.Add(1)
go func() {
    defer wg.Done()
    // work
}()
wg.Wait()
```

## Error Handling

```go
// Create errors
err := errors.New("error message")
err := fmt.Errorf("failed: %w", originalErr)

// Check errors
if err != nil {
    return err
}

// Custom error type
type MyError struct {
    Msg string
}

func (e *MyError) Error() string {
    return e.Msg
}

// Error checking (Go 1.13+)
if errors.Is(err, os.ErrNotExist) {
    // handle
}

var pathErr *os.PathError
if errors.As(err, &pathErr) {
    // handle
}
```

## Standard Library Snippets

### fmt - Formatting
```go
fmt.Print("text")
fmt.Println("text with newline")
fmt.Printf("Name: %s, Age: %d\n", name, age)
s := fmt.Sprintf("formatted %v", value)
```

### strings
```go
strings.Contains(s, substr)
strings.HasPrefix(s, prefix)
strings.Join([]string{"a", "b"}, ",")
strings.Split(s, ",")
strings.ToLower(s)
strings.TrimSpace(s)
```

### strconv
```go
i, _ := strconv.Atoi("42")
s := strconv.Itoa(42)
f, _ := strconv.ParseFloat("3.14", 64)
```

### io
```go
io.Copy(dst, src)
data, _ := io.ReadAll(r)
io.WriteString(w, "data")
```

### os
```go
file, _ := os.Open("file.txt")
defer file.Close()
os.Getenv("PATH")
os.Exit(1)
```

### net/http
```go
// Server
http.HandleFunc("/", handler)
http.ListenAndServe(":8080", nil)

// Client
resp, _ := http.Get("https://example.com")
defer resp.Body.Close()
```

### encoding/json
```go
// Encode
data, _ := json.Marshal(obj)

// Decode
json.Unmarshal(data, &obj)
```

### time
```go
now := time.Now()
time.Sleep(5 * time.Second)
t.Format("2006-01-02 15:04:05")
time.Parse("2006-01-02", "2024-01-01")
```

## Testing

### Basic Test
```go
// mycode_test.go
package mypackage

import "testing"

func TestAdd(t *testing.T) {
    result := Add(2, 3)
    if result != 5 {
        t.Errorf("got %d, want 5", result)
    }
}
```

### Table-Driven Test
```go
func TestAdd(t *testing.T) {
    tests := []struct {
        name     string
        a, b     int
        expected int
    }{
        {"positive", 2, 3, 5},
        {"negative", -1, -1, -2},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := Add(tt.a, tt.b)
            if result != tt.expected {
                t.Errorf("got %d, want %d", result, tt.expected)
            }
        })
    }
}
```

### Benchmark
```go
func BenchmarkAdd(b *testing.B) {
    for i := 0; i < b.N; i++ {
        Add(2, 3)
    }
}
```

## Go Commands

```bash
# Run
go run main.go

# Build
go build
go build -o myapp

# Test
go test
go test -v
go test -cover
go test -race

# Format
go fmt ./...
gofmt -s -w .

# Lint
go vet ./...

# Modules
go mod init module-name
go mod tidy
go get package@version
go get -u ./...

# Install
go install

# Documentation
go doc package.Function
```

## Common Patterns

### Defer
```go
file, err := os.Open("file.txt")
if err != nil {
    return err
}
defer file.Close()
```

### Error Wrapping
```go
if err != nil {
    return fmt.Errorf("failed to process: %w", err)
}
```

### Context
```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
```

### Options Pattern
```go
type Option func(*Config)

func WithPort(port int) Option {
    return func(c *Config) {
        c.port = port
    }
}

func New(opts ...Option) *Server {
    cfg := &Config{port: 8080}
    for _, opt := range opts {
        opt(cfg)
    }
    return &Server{cfg: cfg}
}
```

## Format Verbs

```go
%v   // default format
%+v  // struct with field names
%#v  // Go-syntax representation
%T   // type
%t   // bool
%d   // decimal integer
%b   // binary
%o   // octal
%x   // hex (lowercase)
%X   // hex (uppercase)
%f   // float
%e   // scientific notation
%s   // string
%q   // quoted string
%p   // pointer address
%w   // error wrapping
```

## Best Practices

1. Use `gofmt` to format code
2. Always check errors
3. Use named return values
4. Prefer composition over inheritance
5. Use defer for cleanup
6. Keep functions small and focused
7. Write table-driven tests
8. Document exported names
9. Use interfaces for flexibility
10. Follow Effective Go guidelines

