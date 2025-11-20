# Code Generation Naming Convention Guide

**Date**: 2025-11-20
**Status**: MANDATORY - All code generators must follow these rules
**Updated**: 2025-11-20 (Initial standardization)

## Overview

This document defines the **mandatory** naming convention for ALL generated temporary variables in the Dingo compiler. This ensures consistency, readability, and adherence to Go's coding standards.

## The Rules

### Rule 1: Use camelCase (No Underscores)

**Rationale**: Go's standard style guide uses camelCase for local variables, not underscore prefixes.

✅ **Correct**:
```go
tmp1, err1 := fetchUser(id)
coalesce1 := getValue()
val1 := user.Name
```

❌ **Wrong**:
```go
__tmp0, __err0 := fetchUser(id)  // Double underscores violate Go convention
__coalesce0 := getValue()        // Looks like internal/private (wrong signal)
__val0 := user.Name              // Not idiomatic Go
```

### Rule 2: One-Based Indexing (Start at 1)

**Rationale**: More natural and human-readable. First variable is `1`, not `0`.

✅ **Correct**:
```go
tmp1, err1 := fetchUser(id)
tmp2, err2 := fetchProfile(user.ID)
tmp3, err3 := fetchPosts(profile.ID)
```

❌ **Wrong**:
```go
tmp0, err0 := fetchUser(id)      // Zero-based is unnatural
tmp1, err1 := fetchProfile(user.ID)
tmp2, err2 := fetchPosts(profile.ID)
```

### Rule 3: Counter Initialization Must Start at 1

**Rationale**: Ensures all generators produce one-based numbering consistently.

✅ **Correct**:
```go
// In preprocessors
func (e *ErrorPropProcessor) Process(source []byte) ([]byte, []Mapping, error) {
    e.tryCounter = 1  // Start at 1
    // ...
}

// In plugins
func (ctx *Context) NextTempVar() string {
    if ctx.TempVarCounter == 0 {
        ctx.TempVarCounter = 1  // Initialize to 1
    }
    varName := fmt.Sprintf("tmp%d", ctx.TempVarCounter)
    ctx.TempVarCounter++
    return varName
}
```

❌ **Wrong**:
```go
e.tryCounter = 0  // Wrong! Start at 1
ctx.TempVarCounter = 0  // Wrong! Initialize to 1
```

## Component-Specific Naming

### Error Propagation (`pkg/preprocessor/error_prop.go`)

**Purpose**: Handle `?` operator for error propagation
**Variables**: `tmp%d`, `err%d`

```go
// Input: let user = fetchUser(id)?
// Output:
tmp1, err1 := fetchUser(id)
if err1 != nil {
    return User{}, err1
}
user := tmp1
```

### Null Coalescing (`pkg/preprocessor/null_coalesce.go`)

**Purpose**: Handle `??` operator for null coalescing
**Variables**: `coalesce%d`

```go
// Input: result := a ?? b ?? c
// Output:
func() string {
    coalesce1 := a
    if coalesce1.IsSome() { return coalesce1.Unwrap() }
    coalesce2 := b
    if coalesce2.IsSome() { return coalesce2.Unwrap() }
    return c
}()
```

### Safe Navigation (`pkg/preprocessor/safe_nav.go`)

**Purpose**: Handle `?.` operator for safe navigation
**Variables**: `{base}%d` (e.g., `val1`, `user1`)

```go
// Input: user?.profile?.name
// Output:
func() Option_string {
    if user.IsNone() { return None_string() }
    val1 := user.Unwrap()
    if val1.profile.IsNone() { return None_string() }
    val2 := val1.profile.Unwrap()
    return Some_string(val2.name)
}()
```

### Plugin Temps (`pkg/plugin/plugin.go`)

**Purpose**: Generate unique temp vars for AST transformations
**Variables**: `tmp%d`

```go
// Input: Ok(42) -> needs IIFE for addressability
// Output:
Ok(func() *int {
    tmp1 := 42
    return &tmp1
}())
```

## Implementation Checklist

When creating a new code generator, ensure:

- [ ] Counter initializes to `1`, not `0`
- [ ] Variable names use `camelCase` (e.g., `tmp%d`, `err%d`, `coalesce%d`)
- [ ] No double underscores (`__`) in generated names
- [ ] Comments and documentation use new naming convention
- [ ] Tests expect new naming convention
- [ ] Golden tests regenerated if generator is modified

## Testing

All tests must expect the new naming convention:

```go
// Test expectations
func TestErrorPropagation(t *testing.T) {
    result := transpile("let x = foo()?")

    // ✅ Correct assertions
    assert.Contains(result, "tmp1, err1 :=")
    assert.NotContains(result, "__tmp0")

    // ❌ Wrong assertions
    assert.Contains(result, "__tmp0, __err0 :=")  // Will fail!
}
```

## Documentation References

When documenting generated code, use the new naming:

✅ **Correct**:
```markdown
Error propagation generates the following code:
\`\`\`go
tmp1, err1 := fetchUser(id)
if err1 != nil {
    return nil, err1
}
\`\`\`
```

❌ **Wrong**:
```markdown
Error propagation generates the following code:
\`\`\`go
__tmp0, __err0 := fetchUser(id)
if __err0 != nil {
    return nil, __err0
}
\`\`\`
```

## Historical Context

**Before (Pre-2025-11-20)**:
- Used double underscores: `__tmp0`, `__err0`, `__coalesce0`
- Zero-based indexing: Started at `0`
- Inconsistent across generators

**After (2025-11-20 onwards)**:
- No underscores: `tmp1`, `err1`, `coalesce1`
- One-based indexing: Start at `1`
- Consistent across ALL generators

**Migration**: All existing code and tests were updated on 2025-11-20. See `CHANGELOG.md` entry "Generated Variable Naming Convention (2025-11-20)" for details.

## Verification

To verify naming convention compliance:

```bash
# Check for violations in source code
rg '__tmp\d+|__err\d+|__coalesce\d+|tmpCounter.*=.*0|counter.*=.*0' pkg/

# Should return: No matches found

# Check for violations in generated golden tests
rg '__tmp\d+|__err\d+|__coalesce\d+' tests/golden/*.go

# Should return: No matches found
```

## References

- **CLAUDE.md**: "Code Generation Standards" section
- **CHANGELOG.md**: "Generated Variable Naming Convention (2025-11-20)" entry
- **docs/features/error-propagation.md**: Updated examples
- **Source code**: All generators follow this convention

## Enforcement

**This is a MANDATORY standard**. All code reviews should verify:
1. No double underscores in generated variable names
2. Counters initialize to `1`, not `0`
3. Documentation uses new naming convention
4. Tests expect new naming convention

Violations should be rejected in code review.
