---
title: "⚡ Error propagation with complex return types"
category: "Error Handling"
subcategory: "Error Propagation"
test_id: "error_prop_05_complex_types"
order: 5

complexity: "intermediate"
feature: "⚡ error-propagation"
phase: "Phase 2.4"
status: "implemented"

description: "Validates error propagation with complex Go types including pointers, slices, and structs, ensuring proper zero value generation for all type categories"
summary: "? operator with pointer and slice types"
code_reduction: 60
lines_dingo: 16
lines_go: 40

go_proposal: "71203"
go_proposal_link: "https://github.com/golang/go/issues/71203"
feature_file: "error-propagation.md"
related_tests:
  - "error_prop_01_simple"
  - "error_prop_02_multiple"

tags:
  - "error-handling"
  - "operator"
  - "complex-types"
  - "zero-values"
keywords:
  - "? operator"
  - "pointer types"
  - "slice types"
  - "struct types"
  - "zero values"
---

# Test Reasoning: error_prop_05_complex_types

## What This Test Validates

This test ensures the transpiler correctly handles error propagation with complex Go types: pointers (`*User`), slices (`[]string`), and structs, validating proper zero value generation for each type category.

### Dingo Code (16 lines)

```dingo
package main

type User struct {
	ID   int
	Name string
}

func fetchUser(id: int) (*User, error) {
	let data = ReadFile("user.json")?
	return &User{ID: id, Name: string(data)}, nil
}

func getNames() ([]string, error) {
	let data = ReadFile("names.txt")?
	return []string{string(data)}, nil
}
```

## Design Decisions

### Type-Specific Zero Values

The transpiler must generate appropriate zero values for early returns:

**Pointer types** (`*User`):
```go
if __err0 != nil {
	return nil, __err0  // Zero value for pointer is nil
}
```

**Slice types** (`[]string`):
```go
if __err0 != nil {
	return nil, __err0  // Zero value for slice is nil
}
```

**Struct types** (if returning `User` not `*User`):
```go
if __err0 != nil {
	return User{}, __err0  // Zero value for struct is {}
}
```

### Type System Integration

The transpiler uses Go's `types` package to determine correct zero values:
- Reference types → `nil`
- Value types → Zero-initialized structs
- Basic types → Language-defined zeros

## Success Metrics

**Code Reduction**: 16 lines Dingo → ~40 lines Go = **60% reduction**

**Type Safety**: Compiler validates zero values match return types at compile time.

---

**Last Updated**: 2025-11-17
**Test Status**: ✅ Passing
**Dingo Version**: 0.1.0-alpha (Phase 2.7 complete)
