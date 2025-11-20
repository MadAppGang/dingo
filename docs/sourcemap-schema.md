# Dingo Source Map Schema Documentation

**Version**: 1
**Last Updated**: 2025-11-19

## Overview

Dingo source maps provide bidirectional position mapping between original `.dingo` source files and transpiled `.go` files. This enables accurate error reporting, IDE navigation, and debugging support.

## Format Specification

Dingo uses a **simplified JSON source map format** (version 1) optimized for transpiler use cases. This format is distinct from the [Source Map v3 specification](https://sourcemaps.info/spec.html) used by web browsers.

### File Extension

Source maps are stored with the `.go.golden.map` extension for golden test files:
- Source: `error_prop_01_simple.dingo`
- Transpiled: `error_prop_01_simple.go.golden`
- Source map: `error_prop_01_simple.go.golden.map`

For regular builds:
- Source: `main.dingo`
- Transpiled: `main.go`
- Source map: `main.go.map`

## JSON Schema

```json
{
  "version": 1,
  "dingo_file": "path/to/source.dingo",
  "go_file": "path/to/output.go",
  "mappings": [
    {
      "generated_line": 10,
      "generated_column": 5,
      "original_line": 8,
      "original_column": 3,
      "length": 15,
      "name": "identifier_name"
    }
  ]
}
```

### Field Descriptions

#### Top-Level Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `version` | integer | ✅ Yes | Source map format version. Must be `1`. |
| `dingo_file` | string | ⚠️ Optional | Path to original `.dingo` source file (relative or absolute). |
| `go_file` | string | ⚠️ Optional | Path to generated `.go` file (relative or absolute). |
| `mappings` | array | ✅ Yes | Array of position mappings (can be empty). |

**Note**: While `dingo_file` and `go_file` are optional for schema validity, they are **highly recommended** for debugging and LSP integration.

#### Mapping Object Fields

Each mapping object in the `mappings` array describes a single position correspondence:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `generated_line` | integer | ✅ Yes | Line number in generated `.go` file (1-indexed). Must be >= 1. |
| `generated_column` | integer | ✅ Yes | Column number in generated `.go` file (0-indexed). Must be >= 0. |
| `original_line` | integer | ✅ Yes | Line number in original `.dingo` file (1-indexed). Must be >= 1. |
| `original_column` | integer | ✅ Yes | Column number in original `.dingo` file (0-indexed). Must be >= 0. |
| `length` | integer | ✅ Yes | Length of the mapped segment in characters. Must be >= 0. |
| `name` | string | ⚠️ Optional | Semantic name for this mapping (e.g., `"error_prop"`, `"expr_mapping"`). |

### Position Indexing

**IMPORTANT**: Dingo uses **mixed indexing** to align with Go's standard library conventions:

- **Lines**: 1-indexed (first line is 1)
- **Columns**: 0-indexed (first column is 0)

This matches `go/token.Position` and `go/ast` behavior.

#### Example

```dingo
// File: example.dingo (line numbers shown)
1: package main
2:
3: func main() {
4:     data := ReadFile("config.json")?
5: }
```

```go
// File: example.go (generated)
1: package main
2:
3: func main() {
4:     data, err1 := ReadFile("config.json")
5:     if err1 != nil {
6:         return
7:     }
8: }
```

The `?` operator on Dingo line 4, column 36 expands to error handling code on Go lines 4-7. The source map contains:

```json
{
  "mappings": [
    {
      "generated_line": 4,
      "generated_column": 20,
      "original_line": 4,
      "original_column": 13,
      "length": 14,
      "name": "expr_mapping"
    },
    {
      "generated_line": 5,
      "generated_column": 1,
      "original_line": 4,
      "original_column": 37,
      "length": 1,
      "name": "error_prop"
    }
  ]
}
```

## Mapping Rules

### Length Calculation

The `length` field represents the **character span** of the mapping in the **generated** code:

- **For direct mappings**: Length equals the token/identifier length
- **For transformations**: Length covers the entire generated construct
- **Zero length**: Valid but should be avoided (indicates a point mapping)

### Overlapping Mappings

Mappings on the same generated line **may overlap** when:
1. Multiple original constructs map to the same generated code
2. Nested transformations occur (e.g., error propagation + type annotation)

**Example**: `ReadFile("test")?` generates both `expr_mapping` and `error_prop` mappings at overlapping positions.

### Round-Trip Accuracy

A valid source map must support **round-trip position mapping**:

```
Dingo Position → Go Position → Dingo Position  (must match original)
Go Position → Dingo Position → Go Position      (must match original)
```

**Validation target**: >99.9% round-trip accuracy across all mappings.

## Semantic Names

The `name` field provides semantic context for mappings. Common names used in Dingo:

| Name | Description |
|------|-------------|
| `expr_mapping` | Maps an expression (e.g., function call before `?`) |
| `error_prop` | Maps the error propagation operator `?` |
| `unqualified:FuncName` | Maps an unqualified import (e.g., `ReadFile` → `os.ReadFile`) |
| `type_annotation` | Maps a Dingo type annotation (`:` syntax) |
| `enum_variant` | Maps an enum variant declaration |
| `pattern_match` | Maps a pattern matching construct |

These names are used by:
- Error reporting (to provide context-specific messages)
- LSP navigation (to jump to the correct original position)
- Debugging tools (to display semantic information)

## Usage Examples

### Example 1: Simple Error Propagation

**Dingo Source (`example.dingo`)**:
```dingo
data := ReadFile("config.json")?
```

**Generated Go (`example.go`)**:
```go
data, err1 := ReadFile("config.json")
if err1 != nil {
    return
}
```

**Source Map (`example.go.map`)**:
```json
{
  "version": 1,
  "dingo_file": "example.dingo",
  "go_file": "example.go",
  "mappings": [
    {
      "generated_line": 1,
      "generated_column": 16,
      "original_line": 1,
      "original_column": 9,
      "length": 8,
      "name": "expr_mapping"
    },
    {
      "generated_line": 2,
      "generated_column": 1,
      "original_line": 1,
      "original_column": 34,
      "length": 1,
      "name": "error_prop"
    }
  ]
}
```

### Example 2: Multiple Mappings

**Dingo Source**:
```dingo
result: Result<int, error> = Ok(42)
```

**Generated Go**:
```go
result := ResultIntError{tag: ResultTagOk, ok0: 42}
```

**Source Map**:
```json
{
  "mappings": [
    {
      "generated_line": 1,
      "generated_column": 0,
      "original_line": 1,
      "original_column": 0,
      "length": 6,
      "name": "identifier"
    },
    {
      "generated_line": 1,
      "generated_column": 11,
      "original_line": 1,
      "original_column": 8,
      "length": 17,
      "name": "type_annotation"
    },
    {
      "generated_line": 1,
      "generated_column": 33,
      "original_line": 1,
      "original_column": 33,
      "length": 5,
      "name": "result_constructor"
    }
  ]
}
```

## Validation Rules

### Schema Validation

✅ **MUST**:
- Have `version` field set to `1`
- Have `mappings` array (can be empty)
- All mappings have required fields

⚠️ **SHOULD**:
- Include `dingo_file` path
- Include `go_file` path

### Position Validation

✅ **MUST**:
- `generated_line >= 1`
- `generated_column >= 0`
- `original_line >= 1`
- `original_column >= 0`
- `length >= 0`

⚠️ **SHOULD WARN**:
- `length == 0` (zero-length mapping)
- `length > 1000` (unusually large mapping)
- Duplicate `(generated_line, generated_column)` positions
- Overlapping mappings on the same line (may be intentional)

### Consistency Validation

✅ **MUST**:
- Support round-trip mapping (>99.9% accuracy)
- Map generated positions back to valid original positions
- Map original positions to valid generated positions

## Implementation Notes

### Parser Integration

Preprocessors (e.g., `ErrorPropProcessor`, `TypeAnnotProcessor`) generate mappings during transformation:

```go
sm := preprocessor.NewSourceMap()
sm.AddMapping(preprocessor.Mapping{
    GeneratedLine:   outputLine,
    GeneratedColumn: outputCol,
    OriginalLine:    sourceLine,
    OriginalColumn:  sourceCol,
    Length:          tokenLength,
    Name:            "error_prop",
})
```

### LSP Integration (Future)

The LSP server will use source maps to:
1. Translate editor positions (Dingo) to compiler positions (Go)
2. Translate compiler errors (Go) to editor positions (Dingo)
3. Support "Go to Definition" across transpilation boundary

```go
// LSP request: user clicks at Dingo position 4:37
dingoPos := Position{Line: 4, Column: 37}

// Translate to Go position
goPos := sourceMap.MapToGenerated(dingoPos.Line, dingoPos.Column)

// Request gopls for definition at Go position
definition := gopls.Definition(goPos)

// Translate definition back to Dingo position
dingoDefinition := sourceMap.MapToOriginal(definition.Line, definition.Column)
```

### Error Reporting

When transpiler encounters Go compilation errors:

```go
// Go error at position 5:10
goError := CompileError{Line: 5, Column: 10, Message: "undefined: x"}

// Map back to Dingo source
dingoLine, dingoCol := sourceMap.MapToOriginal(goError.Line, goError.Column)

// Report to user
fmt.Printf("%s:%d:%d: %s\n", dingoFile, dingoLine, dingoCol, goError.Message)
```

## Validation Tool

Use the `pkg/sourcemap` validator to check source map correctness:

```go
import "github.com/jackMort/dingo/pkg/sourcemap"

// Load and validate
validator, err := sourcemap.NewValidatorFromFile("main.go.map")
if err != nil {
    log.Fatal(err)
}

result := validator.Validate()
if !result.Valid {
    for _, e := range result.Errors {
        fmt.Printf("[%s] %s\n", e.Type, e.Message)
    }
}

fmt.Printf("Accuracy: %.2f%%\n", result.Accuracy)
```

### Validation Output

```
✓ Source map is VALID

Statistics:
  Total mappings: 42
  Round-trip tests: 84
  Passed tests: 84
  Accuracy: 100.00%

Warnings (2):
  [schema] missing dingo_file field (optional but recommended)
  [consistency] overlapping mappings on line 10: [5-15] and [12-20]
```

## Future Extensions

### Version 2 Considerations

Potential enhancements for future versions:

1. **VLQ Encoding**: Base64-encoded mappings (like Source Map v3)
2. **Source Content**: Embed original source inline
3. **Sections**: Support multi-file mappings
4. **Extended Metadata**: Transpiler version, timestamp, options

### Backward Compatibility

Version 1 source maps will remain supported. Future versions will:
- Use distinct `version` field values
- Maintain `mappings` array structure
- Provide migration tools

## References

- [Source Map v3 Specification](https://sourcemaps.info/spec.html) - Web source map standard (for comparison)
- [Dingo Preprocessor](../pkg/preprocessor/sourcemap.go) - Implementation
- [Validation Suite](../pkg/sourcemap/validator.go) - Validation tool
- [Golden Tests](../tests/golden/README.md) - Example source maps

## Change Log

### Version 1 (2025-11-19)

- Initial source map format
- Support for preprocessor transformations
- Simple JSON structure (no VLQ encoding)
- Bidirectional position mapping
- Semantic `name` field for context
