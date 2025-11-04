# Go Common Patterns and Idioms

## Design Patterns

### Functional Options Pattern

Used for configuring objects with many optional parameters:

```go
type Server struct {
    host    string
    port    int
    timeout time.Duration
    maxConn int
}

type Option func(*Server)

func WithHost(host string) Option {
    return func(s *Server) {
        s.host = host
    }
}

func WithPort(port int) Option {
    return func(s *Server) {
        s.port = port
    }
}

func WithTimeout(timeout time.Duration) Option {
    return func(s *Server) {
        s.timeout = timeout
    }
}

func NewServer(opts ...Option) *Server {
    // Set defaults
    s := &Server{
        host:    "localhost",
        port:    8080,
        timeout: 30 * time.Second,
        maxConn: 100,
    }
    
    // Apply options
    for _, opt := range opts {
        opt(s)
    }
    
    return s
}

// Usage
srv := NewServer(
    WithHost("example.com"),
    WithPort(443),
    WithTimeout(60 * time.Second),
)
```

### Builder Pattern

For complex object construction:

```go
type HTTPRequest struct {
    method  string
    url     string
    headers map[string]string
    body    []byte
}

type RequestBuilder struct {
    request *HTTPRequest
}

func NewRequestBuilder() *RequestBuilder {
    return &RequestBuilder{
        request: &HTTPRequest{
            headers: make(map[string]string),
        },
    }
}

func (b *RequestBuilder) Method(method string) *RequestBuilder {
    b.request.method = method
    return b
}

func (b *RequestBuilder) URL(url string) *RequestBuilder {
    b.request.url = url
    return b
}

func (b *RequestBuilder) Header(key, value string) *RequestBuilder {
    b.request.headers[key] = value
    return b
}

func (b *RequestBuilder) Body(body []byte) *RequestBuilder {
    b.request.body = body
    return b
}

func (b *RequestBuilder) Build() *HTTPRequest {
    return b.request
}

// Usage
req := NewRequestBuilder().
    Method("POST").
    URL("https://api.example.com").
    Header("Content-Type", "application/json").
    Body([]byte(`{"key":"value"}`)).
    Build()
```

### Singleton Pattern

Thread-safe singleton using sync.Once:

```go
type Database struct {
    conn *sql.DB
}

var (
    instance *Database
    once     sync.Once
)

func GetDatabase() *Database {
    once.Do(func() {
        conn, err := sql.Open("postgres", "connection-string")
        if err != nil {
            log.Fatal(err)
        }
        instance = &Database{conn: conn}
    })
    return instance
}
```

### Factory Pattern

```go
type Animal interface {
    Speak() string
}

type Dog struct{}
func (d Dog) Speak() string { return "Woof!" }

type Cat struct{}
func (c Cat) Speak() string { return "Meow!" }

type AnimalFactory struct{}

func (f *AnimalFactory) CreateAnimal(animalType string) Animal {
    switch animalType {
    case "dog":
        return &Dog{}
    case "cat":
        return &Cat{}
    default:
        return nil
    }
}
```

### Strategy Pattern

```go
type PaymentStrategy interface {
    Pay(amount float64) error
}

type CreditCard struct {
    number string
}

func (c *CreditCard) Pay(amount float64) error {
    fmt.Printf("Paying %.2f using credit card %s\n", amount, c.number)
    return nil
}

type PayPal struct {
    email string
}

func (p *PayPal) Pay(amount float64) error {
    fmt.Printf("Paying %.2f using PayPal account %s\n", amount, p.email)
    return nil
}

type PaymentContext struct {
    strategy PaymentStrategy
}

func (pc *PaymentContext) SetStrategy(strategy PaymentStrategy) {
    pc.strategy = strategy
}

func (pc *PaymentContext) ExecutePayment(amount float64) error {
    return pc.strategy.Pay(amount)
}
```

## Concurrency Patterns

### Worker Pool

```go
func worker(id int, jobs <-chan Job, results chan<- Result) {
    for job := range jobs {
        result := processJob(job)
        results <- result
    }
}

func WorkerPool(numWorkers int, jobs []Job) []Result {
    jobsChan := make(chan Job, len(jobs))
    results := make(chan Result, len(jobs))
    
    // Start workers
    for w := 1; w <= numWorkers; w++ {
        go worker(w, jobsChan, results)
    }
    
    // Send jobs
    for _, job := range jobs {
        jobsChan <- job
    }
    close(jobsChan)
    
    // Collect results
    var output []Result
    for range jobs {
        output = append(output, <-results)
    }
    
    return output
}
```

### Pipeline Pattern

```go
func generator(nums ...int) <-chan int {
    out := make(chan int)
    go func() {
        for _, n := range nums {
            out <- n
        }
        close(out)
    }()
    return out
}

func square(in <-chan int) <-chan int {
    out := make(chan int)
    go func() {
        for n := range in {
            out <- n * n
        }
        close(out)
    }()
    return out
}

func main() {
    // Create pipeline
    c := generator(2, 3, 4)
    out := square(c)
    
    // Consume output
    for result := range out {
        fmt.Println(result)
    }
}
```

### Fan-Out, Fan-In

```go
func fanOut(in <-chan int, n int) []<-chan int {
    channels := make([]<-chan int, n)
    for i := 0; i < n; i++ {
        channels[i] = worker(in)
    }
    return channels
}

func worker(in <-chan int) <-chan int {
    out := make(chan int)
    go func() {
        for n := range in {
            out <- expensiveOperation(n)
        }
        close(out)
    }()
    return out
}

func fanIn(channels ...<-chan int) <-chan int {
    out := make(chan int)
    var wg sync.WaitGroup
    
    wg.Add(len(channels))
    for _, c := range channels {
        go func(ch <-chan int) {
            defer wg.Done()
            for n := range ch {
                out <- n
            }
        }(c)
    }
    
    go func() {
        wg.Wait()
        close(out)
    }()
    
    return out
}
```

### Timeout Pattern

```go
func DoWithTimeout(timeout time.Duration) (result string, err error) {
    done := make(chan struct{})
    
    go func() {
        result = expensiveOperation()
        close(done)
    }()
    
    select {
    case <-done:
        return result, nil
    case <-time.After(timeout):
        return "", fmt.Errorf("operation timed out after %v", timeout)
    }
}
```

### Graceful Shutdown

```go
func main() {
    server := &http.Server{Addr: ":8080"}
    
    // Start server in goroutine
    go func() {
        if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            log.Fatalf("listen: %s\n", err)
        }
    }()
    
    // Wait for interrupt signal
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit
    log.Println("Shutting down server...")
    
    // Graceful shutdown with timeout
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    if err := server.Shutdown(ctx); err != nil {
        log.Fatal("Server forced to shutdown:", err)
    }
    
    log.Println("Server exiting")
}
```

### Rate Limiting

```go
func rateLimiter(rate time.Duration) <-chan time.Time {
    return time.Tick(rate)
}

func main() {
    limiter := rateLimiter(200 * time.Millisecond)
    
    for req := range requests {
        <-limiter  // Wait for rate limiter
        go handleRequest(req)
    }
}
```

### Circuit Breaker

```go
type CircuitBreaker struct {
    maxFailures int
    timeout     time.Duration
    failures    int
    lastFail    time.Time
    state       string
    mu          sync.Mutex
}

func (cb *CircuitBreaker) Call(fn func() error) error {
    cb.mu.Lock()
    defer cb.mu.Unlock()
    
    if cb.state == "open" {
        if time.Since(cb.lastFail) > cb.timeout {
            cb.state = "half-open"
        } else {
            return fmt.Errorf("circuit breaker is open")
        }
    }
    
    err := fn()
    if err != nil {
        cb.failures++
        cb.lastFail = time.Now()
        if cb.failures >= cb.maxFailures {
            cb.state = "open"
        }
        return err
    }
    
    cb.failures = 0
    cb.state = "closed"
    return nil
}
```

## Error Handling Patterns

### Error Wrapping

```go
func processFile(filename string) (err error) {
    data, err := readFile(filename)
    if err != nil {
        return fmt.Errorf("failed to process file %s: %w", filename, err)
    }
    
    if err := validate(data); err != nil {
        return fmt.Errorf("validation failed for %s: %w", filename, err)
    }
    
    return nil
}
```

### Sentinel Errors

```go
var (
    ErrNotFound     = errors.New("not found")
    ErrUnauthorized = errors.New("unauthorized")
    ErrInvalidInput = errors.New("invalid input")
)

func FindUser(id int) (*User, error) {
    user, exists := users[id]
    if !exists {
        return nil, ErrNotFound
    }
    return user, nil
}

// Check error
user, err := FindUser(123)
if errors.Is(err, ErrNotFound) {
    // Handle not found
}
```

### Custom Error Types

```go
type ValidationError struct {
    Field string
    Value interface{}
    Err   error
}

func (e *ValidationError) Error() string {
    return fmt.Sprintf("validation failed for field %s with value %v: %v", 
        e.Field, e.Value, e.Err)
}

func (e *ValidationError) Unwrap() error {
    return e.Err
}

// Usage
var validErr *ValidationError
if errors.As(err, &validErr) {
    fmt.Printf("Field: %s\n", validErr.Field)
}
```

## Resource Management Patterns

### Defer for Cleanup

```go
func processFile(filename string) error {
    file, err := os.Open(filename)
    if err != nil {
        return err
    }
    defer file.Close()
    
    // Process file
    return nil
}
```

### Context for Cancellation

```go
func fetchData(ctx context.Context, url string) ([]byte, error) {
    req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
    if err != nil {
        return nil, err
    }
    
    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    return io.ReadAll(resp.Body)
}
```

### Sync.Pool for Object Reuse

```go
var bufferPool = sync.Pool{
    New: func() interface{} {
        return new(bytes.Buffer)
    },
}

func process() {
    buf := bufferPool.Get().(*bytes.Buffer)
    defer bufferPool.Put(buf)
    
    buf.Reset()
    // Use buffer
}
```

## Testing Patterns

### Table-Driven Tests

```go
func TestAdd(t *testing.T) {
    tests := []struct {
        name     string
        a, b     int
        expected int
    }{
        {"positive numbers", 2, 3, 5},
        {"negative numbers", -1, -1, -2},
        {"mixed signs", -5, 10, 5},
        {"zeros", 0, 0, 0},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := Add(tt.a, tt.b)
            if result != tt.expected {
                t.Errorf("Add(%d, %d) = %d; want %d", 
                    tt.a, tt.b, result, tt.expected)
            }
        })
    }
}
```

### Mock Interfaces

```go
type Database interface {
    Get(key string) (string, error)
    Set(key, value string) error
}

type MockDB struct {
    data map[string]string
}

func (m *MockDB) Get(key string) (string, error) {
    val, ok := m.data[key]
    if !ok {
        return "", errors.New("not found")
    }
    return val, nil
}

func (m *MockDB) Set(key, value string) error {
    m.data[key] = value
    return nil
}

func TestUserService(t *testing.T) {
    mockDB := &MockDB{data: make(map[string]string)}
    service := NewUserService(mockDB)
    // Test service
}
```

### Test Fixtures

```go
func setupTestDB(t *testing.T) (*sql.DB, func()) {
    db, err := sql.Open("sqlite3", ":memory:")
    if err != nil {
        t.Fatal(err)
    }
    
    // Setup schema
    _, err = db.Exec(schema)
    if err != nil {
        t.Fatal(err)
    }
    
    cleanup := func() {
        db.Close()
    }
    
    return db, cleanup
}

func TestDatabase(t *testing.T) {
    db, cleanup := setupTestDB(t)
    defer cleanup()
    
    // Run tests
}
```

