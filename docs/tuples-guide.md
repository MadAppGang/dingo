# Tuples User Guide

**Quick Reference:** Lightweight product types for grouping values without defining structs.

---

## Table of Contents

1. [Getting Started](#getting-started)
2. [Syntax Overview](#syntax-overview)
3. [Common Patterns](#common-patterns)
4. [Type System](#type-system)
5. [Best Practices](#best-practices)
6. [Integration Examples](#integration-examples)
7. [Troubleshooting](#troubleshooting)
8. [Migration Guide](#migration-guide)

---

## Getting Started

### What Are Tuples?

Tuples are **fixed-size collections of heterogeneous values** that don't require struct definitions. They're perfect for:
- Multi-value function returns
- Temporary groupings in pattern matching
- Coordinate pairs, RGB colors, key-value pairs
- Quick prototyping without boilerplate

### Your First Tuple

```dingo
// Create a tuple
let point = (10, 20)

// Destructure it
let (x, y) = point

println("x:", x, "y:", y)  // Output: x: 10 y: 20
```

That's it! No struct definition needed.

### Quick Comparison

**Without tuples (traditional Go):**
```go
type Point struct {
    X int
    Y int
}

point := Point{X: 10, Y: 20}
x := point.X
y := point.Y
```

**With tuples (Dingo):**
```dingo
let point = (10, 20)
let (x, y) = point
```

**Savings:** 4 lines → 2 lines, no struct boilerplate.

---

## Syntax Overview

### Creating Tuples

```dingo
// Basic literals
let pair = (1, 2)
let triple = (1, 2, 3)
let colors = (255, 128, 64)

// Mixed types
let mixed = (42, "hello", true)
let user = ("Alice", 30, "alice@example.com")

// Nested tuples
let grid = ((0, 0), (100, 100))
let matrix = (((1, 2), (3, 4)), ((5, 6), (7, 8)))

// With complex types
let data = (Some(10), Ok("success"), []int{1, 2, 3})
```

### Destructuring Tuples

```dingo
// Simple destructuring
let (x, y) = (10, 20)

// Multi-value return
func swap(a int, b int) (int, int) {
    return (b, a)
}
let (x, y) = swap(1, 2)

// Nested destructuring
let ((x1, y1), (x2, y2)) = ((0, 0), (100, 100))

// Wildcard patterns (ignore values)
let (first, _, last) = (1, 2, 3)
let (name, _, _, isActive) = getUserData()
```

### Type Annotations

```dingo
// Explicit tuple types
let point: (int, int) = (10, 20)
let user: (string, int, bool) = ("Alice", 30, true)

// Function signatures
func divide(a int, b int) (int, int) {
    return (a / b, a % b)
}

// With generics
func parse(s string) Result<(int, int), string> {
    // Returns tuple wrapped in Result
    return Ok((10, 20))
}
```

---

## Common Patterns

### Multi-Value Returns

**Divmod Pattern:**
```dingo
func divmod(a int, b int) (int, int) {
    return (a / b, a % b)
}

let (quotient, remainder) = divmod(17, 5)
println(quotient)   // 3
println(remainder)  // 2
```

**Swap Pattern:**
```dingo
func swap(x int, y int) (int, int) {
    return (y, x)
}

let a = 10
let b = 20
let (a, b) = swap(a, b)  // a=20, b=10
```

**Bounds Pattern:**
```dingo
func getRange() ((int, int), (int, int)) {
    return ((0, 100), (0, 200))
}

let ((minX, maxX), (minY, maxY)) = getRange()
```

### Coordinate Handling

```dingo
type Position = (int, int)

func translate(pos Position, dx int, dy int) Position {
    let (x, y) = pos
    return (x + dx, y + dy)
}

let start = (100, 200)
let moved = translate(start, 10, -5)
let (finalX, finalY) = moved
```

### RGB Colors

```dingo
type Color = (int, int, int)

func lighten(color Color, amount int) Color {
    let (r, g, b) = color
    return (
        min(r + amount, 255),
        min(g + amount, 255),
        min(b + amount, 255),
    )
}

let red = (255, 0, 0)
let lightRed = lighten(red, 50)
```

### Partial Application

```dingo
// Store configuration as tuple
let config = ("localhost", 8080, true)

func connect(config (string, int, bool)) Result<Connection, string> {
    let (host, port, useTLS) = config
    // Connection logic...
}

let result = connect(config)
```

---

## Type System

### Type Names

Dingo generates **human-readable struct names** using CamelCase:

```dingo
let x = (10, "hello")
// Generates: Tuple2IntString

let y = (User{}, error(nil), true)
// Generates: Tuple3UserErrorBool
```

**Naming convention:**
- Pattern: `Tuple{N}{Type1}{Type2}...`
- Basic types: `Int`, `String`, `Bool`, `Error`
- Complex types: `PtrInt`, `SliceString`, `MapStringInt`
- User types: Keep as-is (`User`, `HttpRequest`)

### Type Inference

Dingo infers tuple types from context:

```dingo
// Inferred from literals
let point = (10, 20)
// Type: Tuple2IntInt

// Inferred from function return
func getCoords() (int, int) { ... }
let coords = getCoords()
// Type: Tuple2IntInt

// Explicit annotation
let data: (string, int) = getData()
// Type: Tuple2StringInt
```

### Arity Limits

**Supported:** 2 to 12 elements

```dingo
let valid2 = (1, 2)                                  // ✅ 2 elements
let valid12 = (1,2,3,4,5,6,7,8,9,10,11,12)          // ✅ 12 elements
```

**Not supported:**

```dingo
let empty = ()                                       // ❌ Error
let single = (42,)                                   // ❌ Error
let tooBig = (1,2,3,4,5,6,7,8,9,10,11,12,13)        // ❌ Error
```

**Why 12?**
- Matches Rust's limit (trait implementations)
- Covers 99%+ of real-world use cases
- Encourages better design (use structs for large data)

---

## Best Practices

### When to Use Tuples

✅ **Good:**
```dingo
// Multi-value returns
func divmod(a int, b int) (int, int)

// Coordinate pairs
let position = (x, y)

// Temporary pattern matching
match getStatus() {
    (200, Ok(data)) => ...,
    (404, _) => ...,
}

// Key-value pairs
let entry = ("user_id", 42)
```

❌ **Bad:**
```dingo
// Too many elements (use struct)
let user = (name, age, email, phone, address, city, zip, country)

// Named fields needed (use struct)
type Point = (int, int)  // What's X? What's Y?

// Public API (use struct for clarity)
func CreateUser(data (string, int, string)) User
```

### Tuples vs Structs

**Use tuples when:**
- Temporary, local scope
- 2-5 elements max
- Position meaning is clear
- Quick prototyping

**Use structs when:**
- Named fields improve clarity
- >5 elements
- Public API
- Long-lived data structures
- Need methods

**Example decision:**

```dingo
// ✅ Tuple (clear, temporary)
func getWindowSize() (int, int) {
    return (800, 600)
}

// ✅ Struct (clarity, public API)
type WindowConfig struct {
    Width  int
    Height int
    Title  string
    Resizable bool
}
```

### Wildcard Patterns

Use `_` to ignore unnecessary values:

```dingo
// Only need status code
let (status, _) = fetch(url)

// Only need first and last
let (first, _, _, last) = getData()

// Only need error
let (_, _, err) = complexOperation()
```

**Tip:** Wildcards make intent clearer than unused variables.

---

## Integration Examples

### With Result Types

**Pattern 1: Tuple inside Result**
```dingo
func parseCoords(s string) Result<(int, int), string> {
    if s == "" {
        return Err("empty input")
    }
    return Ok((10, 20))
}

match parseCoords("10,20") {
    Ok((x, y)) => println("Coords:", x, y),
    Err(msg) => println("Error:", msg),
}
```

**Pattern 2: Tuple of Results**
```dingo
func batchFetch(urls []string) (Result<Data, Error>, Result<Data, Error>) {
    let r1 = fetch(urls[0])
    let r2 = fetch(urls[1])
    return (r1, r2)
}

let (result1, result2) = batchFetch(urls)
```

### With Option Types

```dingo
func findPair(items []int) Option<(int, int)> {
    if len(items) < 2 {
        return None
    }
    return Some((items[0], items[1]))
}

match findPair(data) {
    Some((a, b)) => println("Found:", a, b),
    None => println("Not enough items"),
}
```

### With Pattern Matching

```dingo
enum Status {
    Success(int, string),
    Partial(int, string, error),
    Failed(error),
}

let status = getStatus()

match status {
    Status::Success(code, msg) => {
        println("Success:", code, msg)
    },
    Status::Partial(code, msg, err) => {
        println("Partial:", code, msg, "Error:", err)
    },
    Status::Failed(err) => {
        println("Failed:", err)
    },
}
```

### With Enums

```dingo
enum Point {
    TwoD(int, int),
    ThreeD(int, int, int),
}

let point2d = Point::TwoD(10, 20)
let point3d = Point::ThreeD(10, 20, 30)

match point2d {
    Point::TwoD(x, y) => println("2D:", x, y),
    Point::ThreeD(x, y, z) => println("3D:", x, y, z),
}
```

---

## Troubleshooting

### Common Errors

**Error: Empty tuple**
```dingo
let empty = ()
```
```
Error: empty tuples are not supported (line 1). Use 'struct{}' if you need a zero-size type
```
**Fix:** Use `struct{}` for zero-size types, or remove the tuple.

**Error: Single element**
```dingo
let single = (42,)
```
```
Error: single-element tuples are not supported (line 1). Remove parentheses
```
**Fix:** Remove parentheses: `let single = 42`

**Error: Too many elements**
```dingo
let huge = (1,2,3,4,5,6,7,8,9,10,11,12,13)
```
```
Error: tuple has 13 elements, maximum is 12 (line 1). Consider using a struct instead
```
**Fix:** Use a struct:
```dingo
type Data struct {
    field1 int
    field2 int
    // ...
}
```

### Ambiguous Syntax

**Problem: Grouping vs Tuple**
```dingo
let x = (10 + 20)      // Grouping (no comma)
let y = (10 + 20,)     // Error (single-element tuple)
let z = (10, 20)       // Tuple (has comma)
```

**Problem: Function call vs Tuple**
```dingo
foo(a, b)       // Function call
(a, b)          // Tuple literal
let x = (a, b)  // Clearly a tuple (assignment)
```

### Type Inference Issues

**Problem: Complex expressions**
```dingo
// May need explicit type
let result: (int, string) = compute()
```

**Problem: Nested generics**
```dingo
// Be explicit with nested types
let data: (Result<int, string>, Option<bool>) = getData()
```

---

## Migration Guide

### From Go Multi-Return

**Before (Go):**
```go
func divmod(a, b int) (int, int) {
    return a / b, a % b
}

q, r := divmod(17, 5)
```

**After (Dingo):**
```dingo
func divmod(a int, b int) (int, int) {
    return (a / b, a % b)
}

let (q, r) = divmod(17, 5)
```

**Change:** Add parentheses around return values, use `let` for destructuring.

### From Structs to Tuples

**Before (verbose):**
```dingo
type Pair struct {
    First  int
    Second int
}

func swap(p Pair) Pair {
    return Pair{First: p.Second, Second: p.First}
}

let p = Pair{First: 10, Second: 20}
let swapped = swap(p)
```

**After (concise):**
```dingo
func swap(p (int, int)) (int, int) {
    let (a, b) = p
    return (b, a)
}

let p = (10, 20)
let swapped = swap(p)
```

**Change:** Replace struct with tuple type, use destructuring instead of field access.

### From Arrays to Tuples

**Before (array):**
```dingo
func getCoords() []int {
    return []int{10, 20}
}

let coords = getCoords()
let x = coords[0]
let y = coords[1]
```

**After (tuple):**
```dingo
func getCoords() (int, int) {
    return (10, 20)
}

let (x, y) = getCoords()
```

**Advantages:**
- Type-safe (can't access wrong index)
- Cleaner syntax (destructuring vs indexing)
- No runtime bounds checks needed

---

## Quick Reference

### Syntax Cheatsheet

```dingo
// Creation
let tuple = (expr1, expr2, ...)

// Destructuring
let (var1, var2, ...) = tuple

// Wildcards
let (x, _, z) = tuple

// Nested
let ((a, b), (c, d)) = nested

// Type annotation
let tuple: (Type1, Type2) = (val1, val2)

// Function signature
func name(args) (Type1, Type2) { ... }
```

### Type Naming Reference

| Dingo Type | Generated Struct Name |
|------------|-----------------------|
| `(int, string)` | `Tuple2IntString` |
| `(bool, bool, bool)` | `Tuple3BoolBoolBool` |
| `(*int, []string)` | `Tuple2PtrIntSliceString` |
| `(map[string]int, chan bool)` | `Tuple2MapStringIntChanBool` |
| `(User, error)` | `Tuple2UserError` |

### Arity Cheatsheet

| Elements | Supported | Example |
|----------|-----------|---------|
| 0 | ❌ | `()` |
| 1 | ❌ | `(x,)` |
| 2-12 | ✅ | `(a, b)` to `(a,b,c,d,e,f,g,h,i,j,k,l)` |
| 13+ | ❌ | Use struct instead |

---

## Additional Resources

- **Feature Spec:** `features/tuples.md`
- **Implementation Plan:** `ai-docs/sessions/20251120-224222-tuples-phase8/`
- **Test Suite:** `tests/golden/tuples_*.dingo`
- **Related Features:**
  - [Pattern Matching](../features/pattern-matching.md)
  - [Result Type](../features/result.md)
  - [Option Type](../features/option.md)
  - [Sum Types](../features/sum-types.md)

---

**Last Updated:** 2025-11-20
**Status:** Production Ready (Phase 8 Complete)
