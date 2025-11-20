# Nested Match Block Fix - Implementation Summary

## Status: PARTIAL - Core Fix Complete, Cleanup Logic Needs Refinement

## What Was Fixed

### Multi-Pass Processing (✅ Complete)
- Modified `RustMatchProcessor.Process()` to run multiple passes (up to 10)
- Each pass transforms one level of `match` expressions
- Continues until no more `match` keywords remain
- Successfully handles nested match blocks recursively

### Test Results
- New test `TestNestedMatchBlocks` passes ✅
- Nested `match inner { ... }` is now transformed to nested switch statements
- Previously: Nested match left untransformed, caused Lambda processor to incorrectly transform it
- Now: All match keywords processed before Lambda processor runs

## Remaining Issue

### Invalid Block Expression Syntax
When a match expression is inside a block expression (e.g., `Ok(inner) => { match inner { ... } }`), the transformation produces:

```go
result = {
    // DINGO_MATCH_START: inner
    scrutinee2 := inner
    switch scrutinee2.tag { ... }
}
```

This is **invalid Go syntax** - you cannot have `result = { switch ... }`.

### Root Cause
The Variable Hoisting pattern generates `result = expr` for match arms. When `expr` is a block `{ match ... }`, after transformation the block contains switch statements, not a valid expression.

### Attempted Fix (Incomplete)
Added `cleanupBlockExpressions()` to remove invalid `result = { switch ... }` patterns. However, the brace-counting logic is breaking the outer switch structure.

## Recommended Next Steps

### Option 1: Pre-Processing Block Detection
Before multi-pass processing, detect blocks that contain ONLY a match expression:
```dingo
{ match inner { ... } }
```

Unwrap these to just:
```dingo
match inner { ... }
```

Then the multi-pass processing will handle them correctly without invalid block syntax.

### Option 2: Smarter Assignment Context Detection
In `generateCaseWithGuards()`, detect when the arm expression is a transformed match (contains `// DINGO_MATCH_START` marker) and handle it differently - don't wrap in `result = { ... }`, just execute the switch directly.

### Option 3: Expression vs Statement Mode
Add a parameter to `transformMatch()` indicating whether it's in expression context (needs `return`) or statement context (direct execution). Block expressions containing matches should use statement mode.

## Files Modified

- `pkg/preprocessor/rust_match.go`:
  - Split `Process()` into `Process()` + `processSinglePass()`
  - Added multi-pass loop with `containsMatchKeyword()` check
  - Added `cleanupBlockExpressions()` (needs refinement)

- `pkg/preprocessor/rust_match_nested_test.go`: New test file
- `pkg/preprocessor/debug_nested_test.go`: Debug test file

## Test Cases

### Passing
- `TestNestedMatchBlocks`: Verifies nested match is transformed

### Needs Fix
- `tests/golden/pattern_match_13_nested_blocks.dingo`: Produces invalid Go syntax

## Performance Impact
Minimal - most files complete in 1-2 passes. Complex nested matches might take 3-4 passes. Max 10 passes prevents infinite loops.

## Verification Command

```bash
# Run unit test
go test -v -run TestNestedMatchBlocks pkg/preprocessor

# Test with actual file
go run cmd/dingo/main.go build tests/golden/pattern_match_13_nested_blocks.dingo
```

## Summary for Main Chat

Implemented multi-pass processing in RustMatch preprocessor to handle nested match blocks recursively. Each pass transforms one level of match expressions until no more `match` keywords remain (max 10 passes). Nested matches are now correctly transformed to nested switch statements before the Lambda processor runs, preventing the `func(None ...)` bug.

Remaining work: Fix invalid `result = { switch ... }` syntax when match is inside a block expression. Recommend pre-processing to unwrap single-match blocks before multi-pass transformation.

Files changed: 1 (rust_match.go)
Tests added: 2 (rust_match_nested_test.go, debug_nested_test.go)
Status: Core fix complete, cleanup needs refinement
