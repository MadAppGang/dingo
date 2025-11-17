---
title: "⚡ Error propagation with special characters in error messages"
category: "Error Handling"
subcategory: "Error Propagation"
test_id: "error_prop_07_special_chars"
order: 7

complexity: "intermediate"
feature: "⚡ error-propagation"
phase: "Phase 2.4"
status: "implemented"

description: "Validates proper handling and escaping of special characters (quotes, backslashes, etc.) in error wrapping messages to prevent syntax errors in generated Go code"
summary: "? operator with special chars in messages"
code_reduction: 58
lines_dingo: 6
lines_go: 14

go_proposal: "71203"
go_proposal_link: "https://github.com/golang/go/issues/71203"
feature_file: "error-propagation.md"
related_tests:
  - "error_prop_04_wrapping"
  - "error_prop_08_chained_calls"

tags:
  - "error-handling"
  - "operator"
  - "string-escaping"
  - "special-characters"
keywords:
  - "? operator"
  - "string escaping"
  - "quotes"
  - "special characters"
  - "error messages"
---

# Test Reasoning: error_prop_07_special_chars

## What This Test Validates

This test validates the transpiler's **string escaping and handling of special characters** in error wrapping messages, ensuring generated Go code is syntactically correct.

### Dingo Code (6 lines)

```dingo
package main

func readWithMessage(path: string) ([]byte, error) {
	let data = ReadFile(path)? "failed to read \"important\" file"
	return data, nil
}
```

## Design Decisions

### String Escaping Requirements

The error message contains **escaped quotes**: `"failed to read \"important\" file"`

**Challenge**: Transpiler must preserve escaping in generated Go code:
```go
fmt.Errorf("failed to read \"important\" file: %w", __err0)
```

### Edge Cases Handled

Special characters that must be escaped:
- `\"` - Quoted strings within messages
- `\\` - Backslashes
- `\n`, `\t` - Newlines and tabs
- `%` - Format string specifiers (must be escaped as `%%`)

### Implementation Strategy

The transpiler performs:
1. Parse error message string
2. Preserve existing escape sequences
3. Escape format specifiers if present
4. Generate valid Go string literal

## Success Metrics

**Code Reduction**: 6 lines Dingo → 14 lines Go = **58% reduction**

**Correctness**: Generated code compiles without syntax errors and produces correct error messages at runtime.

---

**Last Updated**: 2025-11-17
**Test Status**: ✅ Passing
**Dingo Version**: 0.1.0-alpha (Phase 2.7 complete)
