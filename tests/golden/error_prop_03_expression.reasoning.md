---
title: "⚡ Error propagation in return expressions with ? operator"
category: "Error Handling"
category_order: 10
subcategory: "Error Propagation"
test_id: "error_prop_03_expression"
order: 3

complexity: "intermediate"
feature: "⚡ error-propagation"
phase: "Phase 2.4"
status: "implemented"

description: "Demonstrates error propagation using the ? operator directly in return expressions, showing how errors can be propagated inline without intermediate variable assignment"
summary: "? operator in return expression"
code_reduction: 58
lines_dingo: 5
lines_go: 11

go_proposal: "71203"
go_proposal_link: "https://github.com/golang/go/issues/71203"
feature_file: "error-propagation.md"
related_tests:
  - "error_prop_01_simple"
  - "error_prop_02_multiple"
  - "error_prop_06_mixed_context"

tags:
  - "error-handling"
  - "operator"
  - "expression-context"
  - "return-statement"
keywords:
  - "? operator"
  - "return expression"
  - "inline propagation"
  - "expression context"
---

# Test Reasoning: error_prop_03_expression

## What This Test Validates

This test validates the `?` operator in **expression context** - specifically within a return statement. This is more complex than statement context because the transpiler must inline the error check within the expression itself.

### Dingo Code (5 lines)

```dingo
package main

func parseInt(s: string) (int, error) {
	return Atoi(s)?
}
```

### Generated Go Code (11 lines)

```go
package main

func parseInt(s string) (int, error) {
	__tmp0, __err0 := Atoi(s)
	// dingo:s:1
	if __err0 != nil {
		return 0, __err0
	}
	// dingo:e:1
	return __tmp0, nil
}
```

## Design Decisions

### Expression vs Statement Context

**Statement Context** (test 01):
```dingo
let data = ReadFile(path)?  // Assignment statement
```
Transpiler lifts error check before the assignment.

**Expression Context** (this test):
```dingo
return Atoi(s)?  // Return expression
```
Transpiler must unwrap the result and return it directly.

### Zero Value: `return 0, __err0`

For `int` return type, the zero value is `0`. The transpiler correctly generates type-specific zero values:
- `int` → `0`
- `[]byte` → `nil`
- `string` → `""`
- `bool` → `false`

## Success Metrics

**Code Reduction**: 5 lines Dingo → 11 lines Go = **55% reduction**

More importantly: **Conciseness without sacrificing clarity**. The return statement makes it obvious that if parsing fails, the error propagates immediately.

## Comparison with Rust

```rust
fn parse_int(s: &str) -> Result<i32, ParseIntError> {
    Ok(s.parse()?)  // Identical pattern to Dingo!
}
```

Rust's `?` works exactly the same way in expression context.

---

**Last Updated**: 2025-11-17
**Test Status**: ✅ Passing
**Dingo Version**: 0.1.0-alpha (Phase 2.7 complete)
