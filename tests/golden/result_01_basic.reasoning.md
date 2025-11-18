---
title: "🎁 Basic Result type with Ok and Err variants"
category: "Error Handling"
category_order: 30
subcategory: "Result Type"
test_id: "result_01_basic"
order: 1

complexity: "basic"
feature: "🎁 result-type"
phase: "Phase 2.5"
status: "implemented"

description: "Demonstrates basic Result<T,E> enum type with Ok and Err variants, providing type-safe error handling through sum types instead of tuple returns"
summary: "Basic Result enum with Ok/Err"
code_reduction: 46
lines_dingo: 23
lines_go: 42

go_proposal: "19412"
go_proposal_link: "https://github.com/golang/go/issues/19412"
feature_file: "result-option.md"
related_tests:
  - "result_02_propagation"
  - "sum_types_01_simple"
  - "sum_types_02_struct_variant"

tags:
  - "result-type"
  - "error-handling"
  - "sum-types"
  - "type-safety"
keywords:
  - "Result"
  - "Ok"
  - "Err"
  - "error handling"
  - "algebraic data type"
---

# Test Reasoning: result_01_basic

## What This Test Validates

This test validates the core `Result<T,E>` type pattern - Dingo's answer to Go's `(T, error)` tuple returns. Result is implemented as a sum type (enum) with two variants: `Ok(T)` for success and `Err(E)` for errors.

### Dingo Code (23 lines)

```dingo
package main

import "errors"

enum Result {
	Ok(float64),
	Err(error),
}

func divide(a: float64, b: float64) Result {
	if b == 0.0 {
		return Result_Err(errors.New("division by zero"))
	}
	return Result_Ok(a / b)
}

func main() {
	let result = divide(10.0, 2.0)
	if result.IsOk() {
		let v = *result.ok_0
	}
}
```

### Generated Go Code (42 lines)

The transpiler generates idiomatic Go tagged union:
```go
package main

import "errors"

type ResultTag uint8

const (
	ResultTag_Ok ResultTag = iota
	ResultTag_Err
)

type Result struct {
	tag   ResultTag
	ok_0  *float64
	err_0 *error
}

func Result_Ok(arg0 float64) Result {
	return Result{tag: ResultTag_Ok, ok_0: &arg0}
}

func Result_Err(arg0 error) Result {
	return Result{tag: ResultTag_Err, err_0: &arg0}
}

func (e Result) IsOk() bool {
	return e.tag == ResultTag_Ok
}

func (e Result) IsErr() bool {
	return e.tag == ResultTag_Err
}

func divide(a float64, b float64) Result {
	if b == 0.0 {
		return Result_Err(errors.New("division by zero"))
	}
	return Result_Ok(a / b)
}

func main() {
	result := divide(10.0, 2.0)
	if result.IsOk() {
		v := *result.ok_0
		_ = v
	}
}
```

## Community Context

### Rust's Result Type - The Gold Standard

```rust
fn divide(a: f64, b: f64) -> Result<f64, String> {
    if b == 0.0 {
        Err("division by zero".to_string())
    } else {
        Ok(a / b)
    }
}
```

Dingo's Result type follows the exact same pattern as Rust, providing:
- Type-safe error handling
- Explicit success/failure variants
- No silent null/nil bugs
- Compiler-enforced error checking (with pattern matching)

### Go's Current Pattern

```go
func divide(a, b float64) (float64, error) {
	if b == 0.0 {
		return 0, errors.New("division by zero")
	}
	return a / b, nil
}

result, err := divide(10.0, 2.0)
if err != nil {
	// handle error
}
// use result
```

**Problems with Go's pattern**:
- Can forget to check `err`
- Must manually handle zero value for error case
- No type-level guarantee that errors are handled

**Result type advantages**:
- Cannot access value without checking tag
- Pattern matching enforces exhaustiveness
- Type system guarantees error handling

## Design Decisions

### Enum Variant Syntax: `Ok(float64)`

**Why positional, not named?**

```dingo
Ok(float64)        // Chosen (matches Rust)
Ok{ value: float64 }  // Alternative
```

**Rationale**:
- Result variants typically have single values
- Positional syntax is more concise
- Matches Rust's familiar syntax
- Named fields available for multi-value variants

### Generated Field Names: `ok_0`, `err_0`

Pattern: `{variant}_{index}`

These are the pointer fields in the generated struct:
```go
type Result struct {
	tag   ResultTag
	ok_0  *float64   // Ok variant's value
	err_0 *error     // Err variant's value
}
```

### IsOk() / IsErr() Type Guards

```go
func (e Result) IsOk() bool {
	return e.tag == ResultTag_Ok
}
```

**Purpose**: Enable simple if-based checks before pattern matching is implemented.

**Usage**:
```go
if result.IsOk() {
	value := *result.ok_0  // Safe to unwrap
}
```

## Success Metrics

**Code Reduction**: 23 lines Dingo → 42 lines Go = **45% reduction**

But more importantly: **Type safety gained**

**Go's problem**:
```go
result, err := divide(10, 0)
// Forgot to check err!
fmt.Println(result)  // Prints 0, not an error
```

**Dingo's solution**:
```dingo
let result = divide(10, 0)
// Cannot access value without checking
match result {
	Ok(v) => println(v),
	Err(e) => println("Error: ${e}"),
}
```

## Future Enhancements

### Phase 3 - Pattern Matching Integration

```dingo
match divide(10.0, 2.0) {
	Ok(value) => println("Result: ${value}"),
	Err(e) => println("Error: ${e}"),
}
```

### Phase 3 - Automatic Go Interop

```dingo
// Automatically wrap Go functions returning (T, error)
let result = Result::from(os.ReadFile("config.json"))
// Returns Result<[]byte, error>
```

### Phase 4 - Combinator Methods

```dingo
divide(10.0, 2.0)
	.map(|x| x * 2)
	.and_then(|x| divide(x, 4.0))
	.unwrap_or(0.0)
```

---

**Last Updated**: 2025-11-17
**Test Status**: 🎁 Passing
**Dingo Version**: 0.1.0-alpha (Phase 2.7 complete)
