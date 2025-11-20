# Lambda Type Inference Fix Summary

## Problem
Lambda parameters missing type annotations in generated code:

```go
// Expected:
Map(func(x int) int { return x * 2 })

// Actual (before fix):
Map(func(x) { return x * 2 })
```

## Root Cause Analysis

### Issue 1: Go Parser Behavior
When preprocessor outputs `func(x)`, the Go parser interprets `x` as the **type**, not the parameter name:
- Input: `func(x)`
- AST: Field{Names: [], Type: Ident{Name: "x"}}
- This makes `x` the type, not the parameter!

### Issue 2: Type Information Unavailable
The lambda_type_inference plugin needs `go/types.Info` to look up method signatures (e.g., `Option.Map`). However:
1. Type checker runs on generated code
2. At that point, `Option` type methods are being **injected** by OptionTypePlugin
3. The type checker doesn't see `Map(fn func(int) int)` signature yet
4. Circular dependency: need types to infer lambda params, but types depend on plugins

## Solution Implemented

### Part 1: Preprocessor Marker
Modified lambda preprocessor to add `__TYPE_INFERENCE_NEEDED` marker:

**Before**:
```go
// x => x * 2
func(x) { return x * 2 }  // Parser sees 'x' as type!
```

**After**:
```go
// x => x * 2
func(x __TYPE_INFERENCE_NEEDED) { return x * 2 }  // Now x is name, marker is type
```

**Changes Made**:
- `pkg/preprocessor/lambda.go`:
  - `processSingleParamArrow()`: Add marker for untyped single params
  - `processParams()`: Add marker for untyped multi params
  - Now outputs: `func(x __TYPE_INFERENCE_NEEDED, y __TYPE_INFERENCE_NEEDED)` for `(x, y) => ...`

### Part 2: Plugin Detection
Updated lambda_type_inference plugin to recognize marker:

**Changes Made**:
- `pkg/plugin/builtin/lambda_type_inference.go`:
  - `hasUntypedParams()`: Check for `__TYPE_INFERENCE_NEEDED` marker
  - `applyInferredTypes()`: Replace marker with actual inferred type
  - Add return type inference from signature

### Part 3: Plugin Registration
Registered plugin in pipeline:

**Changes Made**:
- `pkg/generator/generator.go`:
  - Added `lambdaTypeInferencePlugin := builtin.NewLambdaTypeInferencePlugin()`
  - Registered before pattern match plugin

## Current Status

### What Works
✅ Preprocessor adds `__TYPE_INFERENCE_NEEDED` marker correctly
✅ Plugin is registered in pipeline
✅ Plugin detects untyped parameters with marker
✅ Plugin has logic to infer from go/types.Info

### What Doesn't Work Yet
❌ Type inference not actually happening in output
❌ Generated code still has `func(x __TYPE_INFERENCE_NEEDED)`

## Suspected Issue

The type checker runs **after** preprocessing but the `Option` type methods are being **injected** by plugins during the same transformation phase. This means:

1. Preprocessor runs → adds marker
2. Parser creates AST
3. Type checker runs → **doesn't see Option.Map yet** (it's added by plugin!)
4. Plugins run → OptionTypePlugin injects Map method
5. Plugins run → LambdaTypeInferencePlugin tries to look up Map type → **not found!**

**This is a plugin ordering issue.**

## Potential Solutions

### Option A: Two-Pass Type Checking (Recommended)
1. First pass: Run type-injecting plugins (ResultTypePlugin, OptionTypePlugin)
2. Run type checker on partially-transformed AST
3. Second pass: Run type-dependent plugins (LambdaTypeInferencePlugin, PatternMatchPlugin)

### Option B: Helper Method Signature Hardcoding
Hardcode signatures for known helper methods in plugin:
```go
// In lambda_type_inference plugin
knownMethods := map[string]*types.Signature{
    "Option.Map":     funcType(int, int),  // func(int) int
    "Option.AndThen": funcType(int, Option),
    "Result.Map":     funcType(T, U),
}
```

### Option C: Generate Type Stubs First
Before transformation:
1. Generate stub .go file with Result/Option type declarations
2. Run type checker with stubs
3. Transform with full type information
4. Replace stubs with actual generated types

## Files Modified

1. **pkg/preprocessor/lambda.go**
   - Line 268-272: Add `__TYPE_INFERENCE_NEEDED` marker for single params
   - Line 501-505: Add marker in `processParams()` for multi params
   - Line 468-509: Updated comment documentation

2. **pkg/plugin/builtin/lambda_type_inference.go**
   - Line 4-10: Removed unused `strings` import
   - Line 120-142: Updated `hasUntypedParams()` to check for marker
   - Line 280-324: Updated `applyInferredTypes()` to handle marker and add return types

3. **pkg/generator/generator.go**
   - Line 74-76: Registered `LambdaTypeInferencePlugin`

## Next Steps

To fully fix this issue:

1. **Investigate plugin execution order**
   - Check if type-injecting plugins run before type checker
   - Verify when `TypeInfo` is populated

2. **Implement two-pass approach**
   - Split plugins into "injection" and "inference" phases
   - Run type checker between phases

3. **Add detailed logging**
   - Log when each plugin runs
   - Log TypeInfo availability
   - Log method lookup results

4. **Test with simpler case**
   - Create test with explicit Option type defined (not injected)
   - Verify plugin works in that scenario
   - Isolates issue to timing vs. implementation

## Testing

**Test file**: `tests/golden/option_03_chaining.dingo`

**Expected output**:
```go
Map(func(x int) int { return x * 2 })
AndThen(func(x int) Option { ... })
```

**Actual output** (as of this fix):
```go
Map(func(x __TYPE_INFERENCE_NEEDED) { return x * 2 })
AndThen(func(x __TYPE_INFERENCE_NEEDED) { ... })
```

## Recommendation

This is a deeper architectural issue requiring a two-pass plugin system. For v1.0:

**SHORT-TERM FIX**: Require explicit type annotations in lambda parameters
- Update documentation: "Lambda parameters must have explicit types"
- Example: `opt.Map((x: int) => x * 2)` instead of `opt.Map(x => x * 2)`
- This is common in TypeScript too for complex cases

**LONG-TERM FIX** (v1.1): Implement two-pass transformation
- Phase 1: Type injection plugins
- Type checker run
- Phase 2: Type inference plugins
- This enables full type inference like TypeScript

