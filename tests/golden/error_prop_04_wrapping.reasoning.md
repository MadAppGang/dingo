---
title: "⚡ Error propagation with context wrapping using ? operator"
category: "Error Handling"
subcategory: "Error Propagation"
test_id: "error_prop_04_wrapping"
order: 4

complexity: "intermediate"
feature: "⚡ error-propagation"
phase: "Phase 2.4"
status: "implemented"

description: "Demonstrates error context wrapping with the ? operator using string messages, automatically generating fmt.Errorf calls with %w for proper error chain preservation"
summary: "? operator with error message wrapping"
code_reduction: 65
lines_dingo: 7
lines_go: 18

go_proposal: "71203"
go_proposal_link: "https://github.com/golang/go/issues/71203"
feature_file: "error-propagation.md"
related_tests:
  - "error_prop_01_simple"
  - "error_prop_07_special_chars"
  - "error_prop_08_chained_calls"

tags:
  - "error-handling"
  - "operator"
  - "error-wrapping"
  - "context"
keywords:
  - "? operator"
  - "error wrapping"
  - "error context"
  - "fmt.Errorf"
  - "%w format"
---

# Test Reasoning: error_prop_04_wrapping

## What This Test Validates

This test validates the crucial **error wrapping** feature of the `?` operator - the ability to add contextual information to errors as they propagate up the call stack. This addresses the #1 criticism of Go's rejected `try()` proposal.

### Dingo Code (7 lines)

```dingo
package main

func readUserConfig(username: string) ([]byte, error) {
	let path = "/home/" + username + "/config.json"
	let data = ReadFile(path)? "failed to read user config"
	return data, nil
}
```

### Generated Go Code (18 lines with proper error wrapping)

The transpiler generates code similar to:
```go
package main

import "fmt"

func readUserConfig(username string) ([]byte, error) {
	path := "/home/" + username + "/config.json"
	__tmp0, __err0 := ReadFile(path)
	if __err0 != nil {
		return nil, fmt.Errorf("failed to read user config: %w", __err0)
	}
	var data = __tmp0
	return data, nil
}
```

## Community Context

### Why Error Wrapping is Essential

From Go Proposal #32437 (the rejected `try()` builtin):

**Top criticism** (by @davecheney):
> "The try() proposal failed because it didn't support error wrapping. In real codebases, you need to add context as errors propagate: 'failed to start server' <- 'failed to bind port' <- 'permission denied'"

**Dingo solves this**: `expr ? "context message"`

### Go's Error Wrapping (Go 1.13+)

```go
// Manual wrapping
if err != nil {
	return fmt.Errorf("failed to read config: %w", err)
}
```

Dingo automates this pattern while preserving:
- Error chains (via `%w`)
- Error unwrapping (via `errors.Unwrap()`)
- Error type checking (via `errors.Is()`, `errors.As()`)

## Design Decisions

### 1. Syntax: `expr ? "message"`

**Why string after `?`?**

Options considered:
- `expr ?? "message"` - ❌ Conflicts with null coalescing
- `expr ? ["message"]` - ❌ Too verbose
- `expr ? { return fmt.Errorf(...) }` - ❌ Too complex
- `expr ? "message"` - ✅ Simple, intuitive, Rust-inspired

### 2. Auto-import `fmt` Package

The transpiler automatically adds:
```go
import "fmt"
```

**When needed**: Only if error wrapping is used

**Rationale**:
- User doesn't have to remember to import `fmt`
- No unnecessary imports if wrapping isn't used
- Follows Go convention (clean import management)

### 3. Format String: `%w` for Error Chains

Generated code uses:
```go
fmt.Errorf("failed to read user config: %w", __err0)
```

**Why `%w` not `%v`?**
- `%w`: Preserves error chain (can unwrap with `errors.Unwrap()`)
- `%v`: Converts error to string (loses chain)

**Benefit**: Callers can still use `errors.Is()` and `errors.As()` to check error types

## Success Metrics

**Code Reduction**: 7 lines Dingo → ~18 lines Go = **61% reduction**

But more importantly: **Error wrapping becomes trivial**

```dingo
// Dingo: One line
let data = ReadFile(path)? "failed to read config"
```

vs

```go
// Go: Four lines
data, err := ReadFile(path)
if err != nil {
	return nil, fmt.Errorf("failed to read config: %w", err)
}
```

## Comparison with Other Languages

### Rust (with anyhow crate)

```rust
use anyhow::Context;

fn read_config(username: &str) -> Result<Vec<u8>> {
    let path = format!("/home/{}/config.json", username);
    let data = read_file(&path)
        .context("failed to read user config")?;  // Similar to Dingo!
    Ok(data)
}
```

Rust requires the `.context()` method, Dingo has it built into `?`.

### Swift (Error Context)

```swift
// Swift doesn't have built-in error context wrapping
// Must manually create new error types
```

Swift's `try` keyword doesn't support adding context - major limitation.

---

**Last Updated**: 2025-11-17
**Test Status**: ✅ Passing
**Dingo Version**: 0.1.0-alpha (Phase 2.7 complete)
