---
title: "⚡ Error propagation in mixed statement and expression contexts"
category: "Error Handling"
category_order: 10
subcategory: "Error Propagation"
test_id: "error_prop_06_mixed_context"
order: 6

complexity: "advanced"
feature: "⚡ error-propagation"
phase: "Phase 2.4"
status: "implemented"

description: "Demonstrates error propagation mixing both statement context (let bindings) and expression context (return statements) in the same function, validating context-aware transformation"
summary: "Mixed statement and expression ? usage"
code_reduction: 63
lines_dingo: 6
lines_go: 16

go_proposal: "71203"
go_proposal_link: "https://github.com/golang/go/issues/71203"
feature_file: "error-propagation.md"
related_tests:
  - "error_prop_01_simple"
  - "error_prop_03_expression"
  - "error_prop_02_multiple"

tags:
  - "error-handling"
  - "operator"
  - "context-awareness"
  - "mixed-usage"
keywords:
  - "? operator"
  - "statement context"
  - "expression context"
  - "context-aware transformation"
---

# Test Reasoning: error_prop_06_mixed_context

## What This Test Validates

This test validates the transpiler's ability to handle `?` operators in **different contexts within the same function**: both statement context (let binding) and expression context (return statement).

### Dingo Code (6 lines)

```dingo
package main

func processData(path: string) (int, error) {
	let data = ReadFile(path)?        // Statement context
	return Atoi(string(data))?        // Expression context
}
```

## Design Decisions

### Context Detection

The transpiler must detect whether `?` appears in:

1. **Statement context**: `let x = expr?`
   - Generate error check before assignment
   - Assign unwrapped value to variable

2. **Expression context**: `return expr?`
   - Generate error check before return
   - Return unwrapped value directly

### Single Function, Dual Patterns

Generated code (conceptual):
```go
func processData(path string) (int, error) {
	// Statement context handling
	__tmp0, __err0 := ReadFile(path)
	if __err0 != nil {
		return 0, __err0
	}
	var data = __tmp0

	// Expression context handling
	__tmp1, __err1 := Atoi(string(data))
	if __err1 != nil {
		return 0, __err1
	}
	return __tmp1, nil
}
```

## Success Metrics

**Code Reduction**: 6 lines Dingo → 16 lines Go = **63% reduction**

**Context Awareness**: Proves the transpiler correctly identifies and transforms both statement and expression contexts.

---

**Last Updated**: 2025-11-17
**Test Status**: ✅ Passing
**Dingo Version**: 0.1.0-alpha (Phase 2.7 complete)
