# Transform Package

## Purpose

The transformer handles AST-based transformations for complex Dingo features that require semantic analysis, type information, or deep structural changes to the syntax tree.

## Responsibilities

### Current Features (Planned)

1. **Lambda Functions** - Transform lambda syntax to Go anonymous functions
   - Multiple syntax styles: Rust-style `|x| x * 2`, arrow-style `(x) => x * 2`
   - Type inference from context
   - Capture analysis and closure generation

2. **Pattern Matching** - Transform match expressions to Go type switches
   - Exhaustiveness checking
   - Variable binding in patterns
   - Guard clauses

3. **Safe Navigation** - Transform `?.` operator to nil checks
   - Method call chaining: `user?.getProfile()?.getName()`
   - Field access: `user?.profile?.email`
   - Type-aware transformation

### What This Does NOT Handle

The following are handled by the preprocessor (`pkg/preprocessor`):

- **Error Propagation (`?` operator)** - Text-based transformation in preprocessor
- **Type Annotations (`:`)** - Simple syntax normalization in preprocessor
- **Keywords (`let`, etc.)** - Simple keyword replacement in preprocessor
- **Import Detection** - Managed by preprocessor during text transformation

## Why Transformer vs Preprocessor?

Complex features belong in the AST transformer when they require:

### Semantic Analysis Requirements

1. **Type Information**: Lambdas need inferred types, safe navigation needs nil-safety context
2. **Exhaustiveness Checking**: Pattern matching must verify all cases are covered
3. **Structural Complexity**: Features that can't be expressed with line-level regex
4. **Context Awareness**: Need to know parent/sibling AST nodes
5. **Deep Transformations**: Changes that affect multiple AST levels

### When to Use Preprocessor Instead

Simple features that can be expressed as text transformations:
- Syntax sugar that maps 1:1 to Go (error propagation, keywords)
- Features that don't need type information
- Transformations that preserve line structure

## Architecture

### Pipeline Position

```
Preprocessor → Parser → [Transformer] → Generator
                         ↑
                    You are here
```

The transformer receives:
- **Input**: AST of valid Go code (with `?` already expanded by preprocessor)
- **Output**: Modified AST with complex features transformed

### Transformation Strategy

1. **Type Checking Phase** (when needed)
   - Use `go/types` to populate type information
   - Required for: lambdas, pattern matching exhaustiveness

2. **AST Walking Phase**
   - Use `golang.org/x/tools/go/ast/astutil.Apply` for safe traversal
   - Detect placeholder function calls: `__dingo_lambda_N__`, `__dingo_match_N__`, etc.
   - Transform nodes in-place or replace with new subtrees

3. **Context Analysis**
   - Determine expression context: assignment, return, standalone, condition
   - Generate appropriate Go code for each context

### Placeholder Pattern

The preprocessor may insert placeholder calls that the transformer expands:

**Example - Lambda (Planned):**
```go
// Preprocessor output (placeholder):
__dingo_lambda_0__(x, x * 2)

// Transformer output (final):
func(x int) int { return x * 2 }
```

**Example - Pattern Match (Planned):**
```go
// Preprocessor output (placeholder):
__dingo_match_0__(result, ...)

// Transformer output (type switch):
switch v := result.(type) {
case Ok: ...
case Err: ...
}
```

## Current Status

**Phase**: Skeleton implementation

- `transformer.go`: Core infrastructure complete
- `transformLambda()`: TODO - Placeholder for lambda transformation
- `transformMatch()`: TODO - Placeholder for pattern matching
- `transformSafeNav()`: TODO - Placeholder for safe navigation

**Error Propagation**: Fully removed (see note below)

### Important Note: Error Propagation Removed

Error propagation was **intentionally removed** from this package because:

1. **Duplicate Implementation**: Preprocessor has 693 lines of production-ready error propagation code
2. **Architectural Clarity**: Text-based transformation is simpler and faster for this feature
3. **Source Mapping**: Line-level transforms in preprocessor maintain better position accuracy
4. **No Type Information Needed**: `?` operator doesn't require semantic analysis

See `pkg/preprocessor/error_prop.go` for the complete, working implementation.

## Implementation Details

### Placeholder Detection

```go
func (t *Transformer) handlePlaceholderCall(cursor *astutil.Cursor, ident *ast.Ident, call *ast.CallExpr) bool {
    name := ident.Name

    switch {
    case strings.HasPrefix(name, "__dingo_lambda_"):
        return t.transformLambda(cursor, call)
    case strings.HasPrefix(name, "__dingo_match_"):
        return t.transformMatch(cursor, call)
    case strings.HasPrefix(name, "__dingo_safe_nav_"):
        return t.transformSafeNav(cursor, call)
    }

    return true // Continue traversal
}
```

### Context Analysis

```go
type ExprContext int

const (
    ContextUnknown ExprContext = iota
    ContextAssignment   // x = expr
    ContextReturn       // return expr
    ContextStandalone   // expr; (statement position)
    ContextCondition    // if expr { ... }
)
```

Different contexts require different transformation strategies.

## Key Files

- `transformer.go` - Main transformer infrastructure (157 lines)
- Future: `lambda.go`, `pattern_match.go`, `safe_nav.go` for specific features

## Testing

Run transformer tests:
```bash
go test ./pkg/transform/... -v
```

## Future Implementation Plan

### Phase 1: Lambda Transformation
1. Parse lambda placeholder arguments
2. Infer parameter and return types from context
3. Generate `func(...) { ... }` AST nodes
4. Handle capture analysis for closures
5. Support multiple syntax styles

### Phase 2: Pattern Matching
1. Parse match placeholder structure
2. Type check scrutinee (matched value)
3. Generate type switch or if/else chain
4. Implement exhaustiveness checking
5. Handle variable binding in patterns

### Phase 3: Safe Navigation
1. Parse chained navigation placeholders
2. Detect method calls vs field access
3. Generate nil checks with early returns
4. Preserve type safety
5. Optimize for common patterns

## Contributing

When adding new AST transformations:

1. Create dedicated file for the feature (e.g., `lambda.go`)
2. Add placeholder detection in `handlePlaceholderCall()`
3. Implement transformation function
4. Write comprehensive unit tests
5. Document transformation rules and edge cases
6. Update this README with feature details

### Guidelines

- Use `astutil.Apply` for safe AST traversal
- Preserve position information for error reporting
- Handle all expression contexts appropriately
- Add type checking when semantic analysis is needed
- Write clear error messages with source locations

## Related Packages

- `pkg/preprocessor` - Text-based transformations (error propagation, syntax sugar)
- `pkg/parser` - Go parser wrapper
- `pkg/generator` - Final code generation and formatting
- `golang.org/x/tools/go/ast/astutil` - AST manipulation utilities

## References

- [Go AST Package](https://pkg.go.dev/go/ast)
- [Go Types Package](https://pkg.go.dev/go/types)
- [AST Utilities](https://pkg.go.dev/golang.org/x/tools/go/ast/astutil)
