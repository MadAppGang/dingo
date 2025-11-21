# Tuples

**Priority:** P2 (Medium - Convenience feature)
**Status:** ✅ Implemented (Phase 8 Complete)
**Community Demand:** ⭐⭐⭐ (Proposal #63221)
**Inspiration:** Python, Swift, Rust
**Implemented:** 2025-11-20

---

## Overview

Tuples provide lightweight, unnamed product types for grouping related values without defining a struct. They're ideal for temporary value grouping, multi-value returns, and pattern matching.

## Syntax

### Tuple Literals

Create tuples using parentheses with comma-separated values:

```dingo
// Basic tuple literals
let pair = (10, 20)
let triple = (true, "hello", 42)
let coords = (100.5, 200.3)

// Nested tuples
let nested = ((1, 2), (3, 4))
let deep = (((1, 2), (3, 4)), ((5, 6), (7, 8)))

// Complex types
let mixed = (Some(10), Err("fail"), true)
let data = (User{name: "Alice"}, 200, true)
```

### Tuple Destructuring

Extract tuple elements using pattern matching:

```dingo
// Basic destructuring
let (x, y) = (10, 20)
let (name, age, active) = getUserInfo()

// Nested destructuring
let ((minX, maxX), (minY, maxY)) = getRange()

// Wildcard patterns (ignore elements)
let (x, _, z) = (1, 2, 3)
let (name, _, active) = getUserInfo()
```

### Type Annotations

Explicitly specify tuple types when needed:

```dingo
// Explicit types
let point: (int, int) = (10, 20)
let person: (string, int, bool) = ("Alice", 30, true)

// Function signatures
func divmod(a int, b int) (int, int) {
    return (a / b, a % b)
}

// With Result types
func parseCoord(s string) Result<(int, int), string> {
    if s == "" {
        return Err("empty string")
    }
    return Ok((10, 20))
}
```

## Type Naming Convention

Dingo generates human-readable struct type names using CamelCase:

### Naming Rules

1. **Pattern:** `Tuple{N}{Type1}{Type2}...{TypeN}` (no underscores)
2. **Basic types:** Capitalize standard names (`Int`, `String`, `Bool`, `Error`)
3. **Complex types:** Use Go-style CamelCase
   - Pointers: `*int` → `PtrInt`
   - Slices: `[]string` → `SliceString`
   - Maps: `map[string]int` → `MapStringInt`
   - Channels: `chan int` → `ChanInt`
   - Interface: `interface{}` → `Any`
4. **User types:** Use full type name as-is (`User`, `HttpRequest`)
5. **Package prefixes:** Removed (`pkg.User` → `User`)

### Examples

```go
// Simple types
type Tuple2IntString struct {
    _0 int
    _1 string
}

// Mixed complexity
type Tuple3UserErrorBool struct {
    _0 User
    _1 error
    _2 bool
}

// Complex types
type Tuple4PtrIntSliceStringMapStringIntBool struct {
    _0 *int
    _1 []string
    _2 map[string]int
    _3 bool
}
```

**Why this approach:**
- **Readable:** Clear what each tuple contains
- **Debuggable:** Type names appear in stack traces and error messages
- **Idiomatic:** Follows Go's preference for explicit over clever
- **Maintainable:** No hash collisions or lookup tables

## Limitations

### Arity Constraints

Tuples support **2 to 12 elements** (Tuple2 through Tuple12):

```dingo
// Valid
let pair = (1, 2)                                  // 2 elements ✅
let max = (1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12)  // 12 elements ✅

// Invalid
let empty = ()                                      // Error: empty tuple ❌
let single = (42,)                                  // Error: single-element ❌
let huge = (1,2,3,4,5,6,7,8,9,10,11,12,13)         // Error: >12 elements ❌
```

**Rationale:**
- Matches **Rust's practical limit** (12 elements for trait implementations)
- Covers **>99% of real-world use cases**
- Encourages good design: Larger tuples suggest using a struct instead

### Error Messages

```
Empty tuple error:
  "empty tuples are not supported (line 5). Use 'struct{}' if you need a zero-size type"

Single-element error:
  "single-element tuples are not supported (line 8). Remove parentheses"

Too many elements:
  "tuple has 13 elements, maximum is 12 (line 12). Consider using a struct instead"
```

## Transpilation

### Generated Code

Tuples transpile to numbered-field structs:

```dingo
// Input
let point = (10, 20)
let (x, y) = point
```

```go
// Generated
type Tuple2IntInt struct {
    _0 int
    _1 int
}

func main() {
    point := Tuple2IntInt{_0: 10, _1: 20}
    x, y := point._0, point._1
}
```

### Destructuring Expansion

```dingo
// Input
let (x, y) = getCoords()
```

```go
// Generated
tmp := getCoords()
x, y := tmp._0, tmp._1
```

### Nested Tuples

```dingo
// Input
let ((a, b), (c, d)) = nested()
```

```go
// Generated
type Tuple2IntInt struct {
    _0 int
    _1 int
}

type Tuple2Tuple2IntIntTuple2IntInt struct {
    _0 Tuple2IntInt
    _1 Tuple2IntInt
}

func main() {
    tmp1 := nested()
    tmp2, tmp3 := tmp1._0, tmp1._1
    a, b := tmp2._0, tmp2._1
    c, d := tmp3._0, tmp3._1
}
```

## Integration with Other Features

### Pattern Matching

Tuples work seamlessly with pattern matching:

```dingo
let pair = (Ok(10), Err("fail"))

match pair {
    (Ok(x), Ok(y)) => println("both ok:", x, y),
    (Ok(x), Err(e)) => println("partial:", x, e),
    (Err(e1), Err(e2)) => println("both failed:", e1, e2),
}
```

### Result and Option Types

Tuples can contain or be contained in Result/Option:

```dingo
// Tuple of Results
func getResults() (Result<int, string>, Result<int, string>) {
    return (Ok(10), Err("fail"))
}

let (r1, r2) = getResults()

// Result of tuple
func divmod(a int, b int) Result<(int, int), string> {
    if b == 0 {
        return Err("division by zero")
    }
    return Ok((a / b, a % b))
}

let result = divmod(17, 5)
match result {
    Ok((q, r)) => println("Quotient:", q, "Remainder:", r),
    Err(msg) => println("Error:", msg),
}
```

### Enum Tuple Variants

Enums can have tuple-typed variants:

```dingo
enum Message {
    Point(int, int),
    Color(int, int, int),
}

let msg = Message::Point(10, 20)
match msg {
    Message::Point(x, y) => println(x, y),
    Message::Color(r, g, b) => println(r, g, b),
}
```

## Best Practices

### When to Use Tuples

✅ **Good use cases:**
- Multi-value returns (divmod, swap, coordinates)
- Temporary grouping in pattern matching
- Fixed-size homogeneous collections (pairs, triples)
- Return types from external functions

❌ **Avoid tuples when:**
- You need named fields (use struct instead)
- More than 5-6 elements (readability suffers)
- Values are semantically different (struct is clearer)
- Data structure is long-lived or public API

### Tuples vs Structs

**Use tuples for:**
```dingo
// Quick multi-return
func swap(x int, y int) (int, int) {
    return (y, x)
}

// Temporary pattern matching
match getStatus() {
    (200, Ok(data)) => handleSuccess(data),
    (_, Err(e)) => handleError(e),
}
```

**Use structs for:**
```dingo
// Public API with named fields
type Point struct {
    X int
    Y int
}

// Complex data structures
type UserProfile struct {
    Name      string
    Age       int
    Email     string
    Active    bool
    CreatedAt time.Time
}
```

### Wildcard Patterns

Use `_` to ignore elements you don't need:

```dingo
// Only need first and last
let (first, _, _, _, last) = getData()

// Only need middle value
let (_, value, _) = getTriple()
```

## Real-World Examples

### Coordinate Handling

```dingo
func translatePoint(point (int, int), dx int, dy int) (int, int) {
    let (x, y) = point
    return (x + dx, y + dy)
}

let pos = (100, 200)
let newPos = translatePoint(pos, 10, -5)
let (x, y) = newPos
println("New position:", x, y)
```

### Multi-Value Parsing

```dingo
func parseIPPort(addr string) Result<(string, int), string> {
    // Parse logic...
    return Ok(("127.0.0.1", 8080))
}

let result = parseIPPort("127.0.0.1:8080")
match result {
    Ok((ip, port)) => println("Connecting to", ip, "on port", port),
    Err(msg) => println("Parse error:", msg),
}
```

### Batch Operations

```dingo
func batchProcess(items []Item) ((int, int), []error) {
    let success = 0
    let failed = 0
    let errors = []error{}

    for item in items {
        match process(item) {
            Ok(_) => success++,
            Err(e) => {
                failed++
                errors.append(e)
            },
        }
    }

    return ((success, failed), errors)
}

let ((successCount, failCount), errors) = batchProcess(items)
println("Success:", successCount, "Failed:", failCount)
```

## Implementation Details

### Architecture

Tuples use a **two-stage transpilation pipeline**:

**Stage 1: Preprocessor** (Text-based)
- Detects tuple literals: `(expr, expr, ...)`
- Detects destructuring: `let (x, y) = expr`
- Validates arity (2-12 elements)
- Emits markers: `__TUPLE_{N}__LITERAL__{hash}(...)`

**Stage 2: AST Plugin** (go/parser-based)
- Discovers markers in AST
- Infers element types using go/types
- Generates struct declarations
- Transforms markers to struct literals
- Injects type declarations at package level

### Type Deduplication

Multiple identical tuples generate only **one struct type**:

```dingo
let a = (1, 2)
let b = (3, 4)
let c = (5, 6)
```

```go
// Only ONE type generated
type Tuple2IntInt struct {
    _0 int
    _1 int
}

func main() {
    a := Tuple2IntInt{_0: 1, _1: 2}
    b := Tuple2IntInt{_0: 3, _1: 4}
    c := Tuple2IntInt{_0: 5, _1: 6}
}
```

## Benefits

✅ **Avoids single-use struct definitions** - No boilerplate for temporary groupings
✅ **Natural for multi-return values** - Clear syntax for divmod, swap, etc.
✅ **Pattern matching support** - Destructure in match arms
✅ **Zero runtime overhead** - Transpiles to standard Go structs
✅ **Type-safe** - Full compile-time type checking
✅ **IDE-friendly** - Works with gopls, hover, go-to-definition

## Implementation Complexity

**Effort:** Medium
**Timeline:** 1.5-2 weeks
**Status:** ✅ Complete (2025-11-20)

**Components:**
- `pkg/preprocessor/tuples.go` - Literal and destructuring detection (510 LOC)
- `pkg/plugin/builtin/tuples.go` - Type inference and AST transformation (507 LOC)
- `tests/golden/tuples_*.dingo` - Comprehensive test suite (9 files)

---

## References

### Internal Documentation
- Implementation plan: `ai-docs/sessions/20251120-224222-tuples-phase8/01-planning/final-plan.md`
- Existing tuple pattern logic: `pkg/preprocessor/rust_match.go`
- Test suite: `tests/golden/tuples_*.dingo`

### External References
- **Rust Tuples:** https://doc.rust-lang.org/book/ch03-02-data-types.html#the-tuple-type
- **Swift Tuples:** https://docs.swift.org/swift-book/documentation/the-swift-programming-language/thebasics/
- **TypeScript Tuples:** https://www.typescriptlang.org/docs/handbook/2/objects.html#tuple-types
- **Go Proposal #63221:** "First-class tuple support"

### Related Features
- [Pattern Matching](pattern-matching.md) - Tuple destructuring in match arms
- [Result Type](result.md) - Result types containing tuples
- [Option Type](option.md) - Option types containing tuples
- [Sum Types](sum-types.md) - Enum variants with tuple payloads
