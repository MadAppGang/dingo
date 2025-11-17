# Preprocessor Package

## Purpose

The preprocessor transforms Dingo-specific syntax into valid Go syntax before AST parsing. It operates on raw text, performing line-level transformations that are simpler and faster than AST manipulation.

## Responsibilities

### Primary Features

1. **Error Propagation (`?` operator)** - Primary implementation (693 lines, production-ready)
   - Transforms `expr?` into proper error checking patterns
   - Generates unique temporary variables and error returns
   - Supports error message wrapping: `expr? "context message"`
   - Maintains accurate source maps for bidirectional position tracking

2. **Type Annotations** - Transforms parameter syntax
   - Converts `func max(a: int, b: int)` to `func max(a int, b int)`
   - Simplifies parser by normalizing to Go syntax

3. **Keyword Transformations** - Simple syntax sugar
   - Converts `let x = value` to `var x = value`
   - Additional keyword mappings as needed

4. **Automatic Import Detection** - Tracks function calls
   - Detects standard library functions: `ReadFile`, `WriteFile`, `Marshal`, `Unmarshal`, `Atoi`, `ParseInt`, etc.
   - Automatically injects missing imports after all transformations
   - Uses `golang.org/x/tools/go/ast/astutil` for safe import management
   - Deduplicates and sorts imports for consistency

5. **Source Mapping** - Position tracking
   - Maintains bidirectional mappings: Dingo source ↔ Generated Go
   - Automatically adjusts line numbers when imports are added
   - Critical for IDE features (go-to-definition, error reporting)

## Why Preprocessor vs Transformer?

Error propagation and simple syntax transformations belong in the preprocessor because:

### Advantages of Text-Based Processing

1. **Simplicity**: Regex and line-based transformations are easier to understand and maintain
2. **Performance**: Text processing is faster than AST parsing/printing cycles
3. **Source Mapping**: Line-level transforms make it trivial to maintain accurate position mappings
4. **Independence**: Doesn't require type information or AST structure
5. **Proven**: 693 lines of battle-tested, production-ready code

### When to Use Transformer Instead

Complex features requiring semantic analysis belong in the AST transformer:
- Lambda function transformations (need type inference)
- Pattern matching (need exhaustiveness checking)
- Safe navigation with method chaining (need type context)

## Architecture

### Pipeline Position

```
.dingo file
    ↓
[Preprocessor] → Go source text + source maps
    ↓           (? operator expanded, imports added)
[Parser] → AST
    ↓
[Transformer] → Modified AST
    ↓           (lambdas, pattern matching, safe nav)
[Generator] → Final .go file
```

### Processing Flow

1. **Sequential Feature Processing**
   - Type annotations (must be first)
   - Error propagation
   - Keywords (after error prop to avoid interference)
   - Future: Lambdas, sum types, pattern matching, operators

2. **Import Collection**
   - Each processor implementing `ImportProvider` reports needed imports
   - Deduplication across all processors
   - Automatic filtering of already-present imports

3. **Import Injection**
   - After all transformations complete
   - Uses `go/parser` + `astutil.AddImport` for correctness
   - Generates properly formatted import block

4. **Source Map Adjustment**
   - Calculate line offset from added imports
   - Adjust all mapping positions to maintain accuracy

## Implementation Details

### Error Propagation Example

**Dingo Input:**
```go
func readConfig() error {
    let data = ReadFile("config.json")?
    return nil
}
```

**Preprocessor Output:**
```go
import "os"

func readConfig() error {
    data, __err0 := ReadFile("config.json")
    if __err0 != nil {
        return __err0
    }
    return nil
}
```

**Source Map:**
```
Dingo line 2 → Go line 4 (accounting for import)
Dingo line 3 → Go line 8
```

### Import Detection Example

**Detected Functions:**
- `ReadFile` → `"os"`
- `Marshal` → `"encoding/json"`
- `Atoi` → `"strconv"`

**Auto-Injected Import Block:**
```go
import (
    "encoding/json"
    "os"
    "strconv"
)
```

## Key Files

- `preprocessor.go` - Main orchestrator, import injection
- `error_prop.go` - Error propagation (`?`) transformation (693 lines)
- `type_annot.go` - Type annotation syntax normalization
- `keyword.go` - Keyword transformations (`let` → `var`)
- `sourcemap.go` - Position mapping infrastructure

## Testing

Run preprocessor tests:
```bash
go test ./pkg/preprocessor/... -v
```

Test import detection:
```bash
go test ./pkg/preprocessor/... -run TestImport -v
```

## Future Enhancements

- [ ] Smart import grouping (stdlib, third-party, local)
- [ ] Import alias detection and preservation
- [ ] Configurable import style (grouped vs single-line)
- [ ] Performance optimization for large files
- [ ] Incremental processing for IDE integration

## Contributing

When adding new preprocessor features:

1. Implement `FeatureProcessor` interface
2. Add to processor list in `New()` (order matters!)
3. Implement `ImportProvider` if imports are needed
4. Write comprehensive unit tests
5. Update source map generation if line counts change
6. Document the feature in this README

## Related Packages

- `pkg/transform` - AST-level transformations (lambdas, pattern matching)
- `pkg/parser` - Go parser wrapper
- `pkg/generator` - Final code generation and formatting
