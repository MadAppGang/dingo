# UnqualifiedImportProcessor Method Declaration Fix

**Date**: 2025-11-20
**Status**: Complete

## Problem

The `UnqualifiedImportProcessor` was incorrectly flagging method declarations and method calls as unqualified function calls, causing false positives for:

1. Method declarations: `func (r Result) Map(f func(interface{}) interface{}) Result`
2. Method calls: `result.Map(transform)`
3. Stdlib method calls: `strings.Map(fn, s)`, `bytes.Map(fn, b)`

This caused errors when implementing helper methods in `pkg/plugin/builtin/option_type.go` and `pkg/plugin/builtin/result_type.go` because the processor treated `Map`, `Filter`, etc. as ambiguous unqualified stdlib calls instead of recognizing them as method declarations.

## Root Cause

The processor used a simple regex pattern `\b([A-Z][a-zA-Z0-9]*)\s*\(` to match capitalized identifiers followed by `(`, which matched:
- ✅ Standalone calls: `ReadFile(...)`, `Printf(...)`
- ❌ Method declarations: `func (r Result) Map(...)`
- ❌ Method calls: `result.Map(...)`

It only checked if identifiers were "already qualified" (preceded by `.`), but didn't distinguish between method calls (`obj.Method`) and method declarations (`func (r Type) Method`).

## Solution

Added `isMethodDeclaration()` helper that detects the pattern:
```go
func (receiver Type) MethodName(...)
```

The fix works by:
1. Looking backwards from the method name
2. Finding the closing `)` of the receiver declaration
3. Tracking parenthesis depth to find the matching opening `(`
4. Verifying the `func` keyword appears before the receiver

**Implementation**: Added 65 lines to `pkg/preprocessor/unqualified_imports.go`

## Changes

### Modified Files

1. **pkg/preprocessor/unqualified_imports.go**
   - Added `isMethodDeclaration()` helper method (lines 193-260)
   - Integrated check in `Process()` method (line 73-75)
   - Now skips method declarations before checking qualification

2. **pkg/preprocessor/unqualified_imports_method_test.go** (NEW)
   - `TestUnqualifiedTransform_MethodDeclaration`: 4 test cases
   - `TestUnqualifiedTransform_ComplexMethodDeclarations`: Tests various receiver patterns
   - Total: 165 lines of comprehensive test coverage

## Test Results

All 10 unqualified import tests pass:
- ✅ TestUnqualifiedTransform_MethodDeclaration (4 subcases)
- ✅ TestUnqualifiedTransform_ComplexMethodDeclarations
- ✅ TestUnqualifiedTransform_Basic
- ✅ TestUnqualifiedTransform_LocalFunction
- ✅ TestUnqualifiedTransform_Ambiguous
- ✅ TestUnqualifiedTransform_MultipleImports
- ✅ TestUnqualifiedTransform_AlreadyQualified
- ✅ TestUnqualifiedTransform_MixedQualifiedUnqualified
- ✅ TestUnqualifiedTransform_NoStdlib
- ✅ TestUnqualifiedTransform_OnlyLocalFunctions

## Behavior

### Before Fix
```go
// Input
func (r Result) Map(f func(interface{}) interface{}) Result {
    return r
}

// Error: "Map is ambiguous: could be strings.Map or bytes.Map"
```

### After Fix
```go
// Input
func (r Result) Map(f func(interface{}) interface{}) Result {
    return r
}

// Output: Unchanged (correctly recognized as method declaration)
```

## Examples

| Pattern | Transformed? | Reason |
|---------|-------------|--------|
| `ReadFile("path")` | ✅ Yes → `os.ReadFile("path")` | Standalone stdlib call |
| `func (r Result) Map(...)` | ❌ No | Method declaration |
| `result.Map(fn)` | ❌ No | Method call (qualified with `.`) |
| `strings.Map(fn, s)` | ❌ No | Already qualified |
| `Printf("hello")` | ✅ Yes → `fmt.Printf("hello")` | Standalone stdlib call |

## Impact

This fix eliminates false positives when:
- Implementing helper methods on Option/Result types
- Using method names that conflict with stdlib (Map, Filter, etc.)
- Writing enum methods with common names

The processor now correctly distinguishes:
1. **Standalone calls** (transform to qualified)
2. **Method declarations** (ignore)
3. **Method calls** (ignore via existing `isAlreadyQualified`)

## Technical Details

### isMethodDeclaration Algorithm

```
1. Look backward from method name position
2. Skip whitespace
3. Find ')' (closing paren of receiver)
4. Track paren depth to find matching '('
5. Skip whitespace before '('
6. Verify 'func' keyword precedes
7. Ensure 'func' is complete word (not part of identifier)
```

### Edge Cases Handled

- Pointer receivers: `func (r *Result) Method(...)`
- Generic-style receivers: `func (r Result[T]) Method(...)`
- Whitespace variations: `func  (  r   Result  )  Method(...)`
- Multiple receivers in same file
- Nested parentheses in receiver type

## Files Created

- `pkg/preprocessor/unqualified_imports_method_test.go` (165 lines)
- `ai-docs/fixes/unqualified-imports-method-fix.md` (this file)

## Related Issues

This fix resolves the blocking issue for completing helper method implementations in:
- `pkg/plugin/builtin/option_type.go`
- `pkg/plugin/builtin/result_type.go`
- `pkg/preprocessor/enum.go`
