# Dingo Syntax Design Decisions

## 🎯 Philosophy

Dingo is a **meta-language for Go**, not a replacement. Our guiding principle:

> **Minimize syntax changes except where they add real value.**

We only diverge from Go when there's a clear benefit to developer experience.

---

## ✅ Function Syntax

### Parameter Types: Use `:` (Different from Go)

**Dingo:**
```dingo
func max(a: int, b: int) int {
    return a
}
```

**Go:**
```go
func max(a int, b int) int {
    return a
}
```

**Why the change?**
- ✅ **Clearer**: Name THEN type is more intuitive
- ✅ **Consistent**: Matches TypeScript, Rust, Kotlin, Swift
- ✅ **Readable**: `name: type` reads like "name is of type"
- ✅ **Valuable**: Real improvement over Go's `name type` order

### Return Types: Use Go's approach (Same as Go)

**Dingo (Recommended):**
```dingo
func max(a: int, b: int) int {
    return a
}
```

**Dingo (Also Valid - for backward compatibility):**
```dingo
func max(a: int, b: int) -> int {
    return a
}
```

**Why no arrow by default?**
- ❌ Arrow doesn't add clarity
- ✅ Closer to Go = easier adoption
- ✅ Less syntax to learn
- ✅ Return position is already obvious

**Result:** Arrow (`->`) is **optional** for return types. We recommend omitting it.

---

## 📊 Syntax Comparison

| Feature | Go | Dingo | Why Different? |
|---------|----|----|---------|
| **Parameters** | `name type` | `name: type` | ✅ Clearer, more intuitive |
| **Return Type** | `type` | `type` or `-> type` | Arrow optional (Go-style recommended) |
| **Variables** | `var name type` | `let name: type` | ✅ `let` is immutable by default |
| **Type inference** | `x := 5` | `let x = 5` | ✅ Explicit `let` keyword |

---

## 🎨 Design Principles

### 1. **Minimize Divergence**
Only change syntax when there's clear value.

**Example - Parameters:**
- Go: `func add(a int, b int) int`
- Dingo: `func add(a: int, b: int) int`
- **Justification:** Colon makes type relationship clearer

**Example - Return types:**
- Go: `int` (no arrow)
- Dingo: `int` (arrow optional)
- **Justification:** Arrow adds no value, so make it optional

### 2. **Learn from Modern Languages**
Adopt proven patterns from TypeScript, Rust, Kotlin, Swift.

**Adopted:**
- `:` for type annotations (TypeScript, Rust, Kotlin, Swift)
- `let` for variables (JavaScript, Rust, Swift)
- `Result<T, E>` for errors (Rust)

**Not Adopted:**
- `->` for return types (not necessary)
- Semicolons (Go doesn't use them)
- Braces position (follow Go style)

### 3. **Progressive Enhancement**
Dingo should feel like "Go with superpowers", not a different language.

**Goal:** A Go developer should be productive in Dingo within minutes.

---

## 🔄 Evolution

### Initial Design (Week 1 - v0.1.0)
```dingo
func max(a: int, b: int) -> int {
    return a
}
```

### Revised Design (After User Feedback)
```dingo
// Recommended (Go-style)
func max(a: int, b: int) int {
    return a
}

// Also valid (backward compatible)
func max(a: int, b: int) -> int {
    return a
}
```

**Why the change?**
- User feedback: "Why the arrow? It should inherit Go's approach"
- Analysis: Arrow doesn't add value
- Decision: Make arrow optional, recommend Go-style

---

## 📝 Syntax Guide

### Functions

```dingo
// Basic function (recommended)
func greet(name: string) string {
    return "Hello, " + name
}

// With arrow (also valid)
func greet(name: string) -> string {
    return "Hello, " + name
}

// Multiple parameters
func add(a: int, b: int) int {
    return a + b
}

// No return type
func main() {
    println("Hello")
}

// Multiple return values (future - not yet implemented)
func divide(a: int, b: int) (int, error) {
    return a / b, nil
}
```

### Variables

```dingo
// Immutable (recommended)
let x = 42
let name: string = "Dingo"

// Mutable
var count = 0
var message: string = "Hello"
```

### Type Annotations

```dingo
// Optional when type can be inferred
let x = 42              // inferred as int
let name = "Dingo"       // inferred as string

// Required when type cannot be inferred
let result: int
let data: []byte
```

---

## 🎯 Future Syntax

### Result Types (Planned)
```dingo
func fetchUser(id: string) Result<User, Error> {
    let user = db.query(id)?
    return Ok(user)
}
```

### Pattern Matching (Planned)
```dingo
match result {
    Ok(user) => println(user.name),
    Err(e) => println("Error: " + e),
}
```

### Lambdas (Planned)
```dingo
// Rust-style
let double = |x| x * 2

// Arrow-style
let add = (a, b) => a + b
```

---

## 💡 Rationale Summary

| Syntax Element | Decision | Rationale |
|---------------|----------|-----------|
| Parameter types | `name: type` | Clearer than `name type` |
| Return types | `type` (no arrow) | Arrow adds no value |
| Arrow optional | Yes | Backward compatibility |
| `let` keyword | Yes | Explicit immutability |
| `var` keyword | Yes | Explicit mutability |
| Type inference | Yes | Reduce boilerplate |

---

## 🎓 Key Takeaway

**Dingo syntax = Go syntax + minimal enhancements**

We only change what needs changing. Everything else stays familiar.

This makes Dingo:
- ✅ **Easy to learn** (if you know Go, you know 90% of Dingo)
- ✅ **Easy to read** (looks like Go with small improvements)
- ✅ **Easy to adopt** (minimal migration cost)

---

**Last Updated:** 2025-11-16
**Based on:** User feedback about arrow syntax
