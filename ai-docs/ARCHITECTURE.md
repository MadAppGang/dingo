# Dingo Transpiler Architecture

**Last Updated**: 2025-11-22
**Current Phase**: Phase 9 Complete - Post-AST Source Maps

---

## Overview

Dingo uses a **three-stage transpilation** architecture:

1. **Stage 1: Preprocessor** - Text-based transformations (Dingo syntax → valid Go) with unique markers
2. **Stage 2: AST Processing** - Structural transformations (Go AST → transformed Go AST)
3. **Stage 3: Post-AST Source Maps** - Accurate position mapping using FileSet and unique markers

This approach leverages Go's own parser while enabling Dingo-specific syntax that Go doesn't support, with 100% accurate source maps for full IDE integration.

---

## The Three-Stage Pipeline

```
┌─────────────────────────────────────────────────────────────┐
│                     .dingo Source File                       │
│  enum Color { Red, Green, Blue }                            │
│  func process(x: int) -> Result<int, Error> { ... }        │
└────────────────────┬────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────────┐
│            STAGE 1: PREPROCESSOR (Text-Based)                │
├─────────────────────────────────────────────────────────────┤
│  TypeAnnotProcessor:   param: Type  →  param Type           │
│  ErrorPropProcessor:   x?  →  if err != nil { return ... }  │
│  EnumProcessor:        enum Name {} → Go structs             │
│  KeywordProcessor:     Other Dingo keywords                  │
│                                                              │
│  NEW: Emits TransformMetadata with unique markers           │
│       Format: // dingo:X:N  (X=type, N=counter)            │
├─────────────────────────────────────────────────────────────┤
│  Tools: regexp, string manipulation                          │
│  Output: Valid Go syntax + unique markers                    │
└────────────────────┬────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────────┐
│             Valid Go Code + Markers                          │
│  type ColorTag uint8                                         │
│  func process(x int) Result_int_Error { ... }               │
│  tmp, err := foo() // dingo:E:1                             │
└────────────────────┬────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────────┐
│            STAGE 2: AST PROCESSING                           │
├─────────────────────────────────────────────────────────────┤
│  go/parser:        Parse preprocessed Go → AST              │
│                                                              │
│  Plugin Pipeline:  3-phase transformation                    │
│    Phase 1 - Discovery:   Find Ok()/Err() calls            │
│    Phase 2 - Transform:   Rewrite AST nodes                │
│    Phase 3 - Inject:      Add type declarations            │
├─────────────────────────────────────────────────────────────┤
│  Tools: go/parser, go/ast, astutil, go/printer              │
│  Output: Transformed AST → go/printer → final .go file      │
└────────────────────┬────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────────┐
│            STAGE 3: POST-AST SOURCE MAPS                     │
├─────────────────────────────────────────────────────────────┤
│  PostASTGenerator:                                           │
│    1. Reads final .go file (after go/printer formatting)   │
│    2. Uses go/token.FileSet for ground truth positions     │
│    3. Matches unique markers (// dingo:X:N) in comments    │
│    4. Maps positions: .dingo line/col → .go line/col       │
├─────────────────────────────────────────────────────────────┤
│  Tools: go/token, go/parser (for FileSet)                   │
│  Output: .sourcemap with 100% accurate positions            │
└─────────────────────────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────────┐
│      Final Output: .go file + .sourcemap (perfect LSP)       │
└─────────────────────────────────────────────────────────────┘
```

---

## Why This Architecture?

### The Problem We're Solving

Dingo needs to support syntax that **isn't valid Go**:
- `enum Color { Red, Green, Blue }` - enums don't exist in Go
- `param: Type` - Go uses `param Type` (space, not colon)
- `x?` - error propagation operator doesn't exist in Go

We have two options:
1. **Write a custom parser** - Complex, error-prone, hard to maintain
2. **Transform to valid Go first, then use go/parser** - Simple, reliable

We chose option 2.

### Why Not Use participle or Tree-sitter?

**Early planning docs mentioned `participle`** as the parser, but the implementation evolved:

**Why we moved away from custom parsers:**
- Go already has an excellent parser: `go/parser`
- Writing a custom parser duplicates existing work
- Maintaining grammar files adds complexity
- go/parser is battle-tested and always up-to-date

**What we do instead:**
- Preprocessors transform Dingo syntax → valid Go (simple regex)
- Then go/parser handles all the hard parsing work
- Much simpler, more maintainable

### The "Regex vs AST Parsing" Debate

**Some code reviewers flagged this:**
> "Enum preprocessor uses regex instead of AST parsing - this is fragile!"

**Why regex is CORRECT for preprocessors:**

1. **Chicken-and-egg problem**: You can't use go/parser on `enum Color {}` because it's not valid Go syntax
2. **Preprocessors MUST use text manipulation**: That's their job - make invalid Go valid
3. **Proven pattern**: Other preprocessors work the same way:
   - TypeAnnotProcessor uses regex for `:` → space
   - ErrorPropProcessor uses regex for `?` operator
   - EnumProcessor uses regex for `enum` keyword

**AST parsing happens in Stage 2**, where it belongs:
- Plugin pipeline uses proper AST traversal (astutil.Apply)
- Type transformations use go/ast
- Declaration injection uses go/ast

---

## Stage 1: Preprocessor Pipeline

### Architecture

```go
type Preprocessor struct {
    processors []FeatureProcessor
}

type FeatureProcessor interface {
    Process(source []byte, mappings []Mapping) ([]byte, []Mapping, error)
}
```

### Current Preprocessors

| Preprocessor | Input | Output | Purpose |
|--------------|-------|--------|---------|
| **TypeAnnotProcessor** | `func add(a: int, b: int)` | `func add(a int, b int)` | Convert `:` to space |
| **ErrorPropProcessor** | `let x = readFile(path)?` | `x, err := readFile(path); if err != nil { return ..., err }` | Expand `?` operator |
| **EnumProcessor** | `enum Color { Red, Green }` | `type ColorTag uint8; const (...); type Color struct {...}` | Generate tagged unions |
| **KeywordProcessor** | `let x = 5` | `var x = 5` | Convert Dingo keywords |

### How Preprocessors Work

**Example: EnumProcessor**

Input (Dingo):
```go
enum Color {
    Red,
    Green,
    Blue
}
```

Transformation steps:
1. **Find** enum declarations using regex: `enum\s+(\w+)\s*\{([^}]*)\}`
2. **Parse** variant list by splitting on commas
3. **Generate** Go code:
   - Tag type: `type ColorTag uint8`
   - Constants: `const (ColorTag_Red ColorTag = iota; ...)`
   - Struct: `type Color struct { tag ColorTag }`
   - Constructors: `func Color_Red() Color { ... }`
   - Helper methods: `func (c Color) IsRed() bool { ... }`
4. **Replace** original enum block with generated code

Output (Valid Go):
```go
type ColorTag uint8

const (
    ColorTag_Red ColorTag = iota
    ColorTag_Green
    ColorTag_Blue
)

type Color struct {
    tag ColorTag
}

func Color_Red() Color {
    return Color{tag: ColorTag_Red}
}

// ... more constructors and helper methods
```

### Source Mapping (Post-AST Architecture)

**New Approach (Phase 9)**: Preprocessors emit metadata with unique markers for Post-AST position mapping:

```go
type TransformMetadata struct {
    OriginalLine   int    // Line in .dingo file
    OriginalCol    int    // Column in .dingo file
    UniqueMarker   string // "// dingo:X:N" for matching in final .go
    TransformType  string // "error_prop", "type_annot", etc.
}
```

**How it works**:
1. **Stage 1**: Preprocessors emit TransformMetadata with unique markers (e.g., `// dingo:E:1`)
2. **Stage 2**: AST processing preserves markers as comments
3. **Stage 3**: PostASTGenerator matches markers in final .go file using go/token.FileSet

**Benefits**:
- 100% accurate positions (FileSet is ground truth)
- Zero drift from go/printer reformatting
- No line offset math or cumulative tracking
- Full IDE integration support

This allows error messages and LSP features to point to the original `.dingo` file with perfect accuracy.

---

## Stage 2: Plugin Pipeline

### Architecture

```go
type Pipeline struct {
    plugins []Plugin
    ctx     *Context
}

type Plugin interface {
    Process(file *ast.File) error
}

// Optional interfaces plugins can implement:
type ContextAware interface {
    SetContext(*Context)
}

type Transformer interface {
    Transform(node ast.Node) (ast.Node, error)
}

type DeclarationProvider interface {
    GetPendingDeclarations() []ast.Decl
    ClearPendingDeclarations()
}
```

### Three-Phase Execution

**Phase 1: Discovery**
- Each plugin calls `Process(file)` to scan the AST
- Plugins discover what transformations are needed
- Example: ResultTypePlugin finds all `Ok()` and `Err()` calls

**Phase 2: Transform**
- Plugins implementing `Transformer` modify AST nodes
- Uses `astutil.Apply` for safe tree traversal
- Example: Replace `Ok(value)` with `Result{tag: ResultTag_Ok, ok_0: &value}`

**Phase 3: Inject**
- Plugins implementing `DeclarationProvider` add type declarations
- Example: Add `type Result_int_error struct {...}` to file

### Current Plugins

| Plugin | Purpose | Transforms |
|--------|---------|------------|
| **ResultTypePlugin** | Result<T,E> support | `Ok(x)` → struct literal, adds Result type defs |

### Plugin Example: ResultTypePlugin

**Discovery phase:**
```go
func (p *ResultTypePlugin) Process(file *ast.File) error {
    ast.Inspect(file, func(n ast.Node) bool {
        if call, ok := n.(*ast.CallExpr); ok {
            if ident, ok := call.Fun.(*ast.Ident); ok {
                if ident.Name == "Ok" || ident.Name == "Err" {
                    // Record this call needs transformation
                    p.recordResultUsage(call)
                }
            }
        }
        return true
    })
    return nil
}
```

**Transform phase:**
```go
func (p *ResultTypePlugin) Transform(node ast.Node) (ast.Node, error) {
    return astutil.Apply(node,
        func(cursor *astutil.Cursor) bool {
            if call, ok := cursor.Node().(*ast.CallExpr); ok {
                if ident, ok := call.Fun.(*ast.Ident); ok {
                    if ident.Name == "Ok" {
                        replacement := p.transformOkConstructor(call)
                        cursor.Replace(replacement)
                    }
                }
            }
            return true
        },
        nil,
    )
}
```

**Inject phase:**
```go
func (p *ResultTypePlugin) GetPendingDeclarations() []ast.Decl {
    return []ast.Decl{
        // type Result_int_error struct { ... }
        // func (r Result_int_error) IsOk() bool { ... }
        // etc.
    }
}
```

---

## Code Generation

After plugin pipeline completes:

```go
func (g *Generator) Generate(file *ast.File) ([]byte, error) {
    // 1. Transform through plugin pipeline
    transformed, err := g.pipeline.Transform(file)
    if err != nil {
        return nil, err
    }

    // 2. Generate Go code using go/printer
    var buf bytes.Buffer
    if err := printer.Fprint(&buf, g.fset, transformed); err != nil {
        return nil, err
    }

    // 3. Format with gofmt
    formatted, err := format.Source(buf.Bytes())
    if err != nil {
        return buf.Bytes(), nil // Return unformatted on error
    }

    return formatted, nil
}
```

---

## Why This Works

### Separation of Concerns

**Preprocessors handle:**
- Dingo syntax → valid Go syntax
- Simple text transformations
- Syntax that go/parser can't handle

**Plugins handle:**
- Go AST → transformed Go AST
- Complex semantic transformations
- Type inference and code generation

### Leveraging Go's Tooling

We use Go's own tools wherever possible:
- `go/parser` - battle-tested, always up-to-date
- `go/ast` - comprehensive AST representation
- `go/printer` - correct code generation
- `go/format` - standard formatting

This means:
- Less code to maintain
- Fewer bugs
- Automatic support for new Go features

### Extensibility

**Adding a new Dingo syntax feature:**

1. If it's not valid Go: Add a preprocessor
   ```go
   type MyFeatureProcessor struct {}
   func (p *MyFeatureProcessor) Process(src []byte, ...) ([]byte, ..., error) {
       // Transform your syntax to valid Go
   }
   ```

2. If it transforms valid Go AST: Add a plugin
   ```go
   type MyFeaturePlugin struct {}
   func (p *MyFeaturePlugin) Process(file *ast.File) error {
       // Discover usage
   }
   func (p *MyFeaturePlugin) Transform(node ast.Node) (ast.Node, error) {
       // Transform AST
   }
   ```

3. Register it:
   ```go
   // For preprocessors:
   preprocessor.RegisterProcessor(NewMyFeatureProcessor())

   // For plugins:
   registry.Register("my-feature", NewMyFeaturePlugin())
   ```

---

## Known Limitations (Phase 2.16)

### Fix A4: Literal Address Issue

**Problem**: Generated code contains `&42`, `&"string"` (invalid Go)

**Example**:
```go
// Dingo:
x := Ok(42)

// Generated (INVALID):
x := Result{tag: ResultTag_Ok, ok_0: &42}  // Can't take address of literal
```

**Workaround**: Assign to variable first
```go
val := 42
x := Ok(val)  // Works
```

**Fix (Phase 3)**: Generate temporary variables
```go
tmp := 42
x := Result{tag: ResultTagOk, ok: &tmp}
```

### Fix A5: Type Inference Limitations

**Problem**: Falls back to `interface{}` without go/types integration

**Example**:
```go
func process() Result<int, error> {
    return Ok(42)  // Plugin infers types from context
}

// But this fails:
x := Ok(42)  // No context - falls back to interface{}
```

**Fix (Phase 3)**: Integrate go/types for full type inference

---

## File Organization

```
pkg/
├── preprocessor/
│   ├── preprocessor.go      # Pipeline coordinator
│   ├── typeannotation.go    # : → space
│   ├── errorprop.go          # ? → error handling
│   ├── enum.go               # enum → structs
│   ├── keyword.go            # let → var, etc.
│   └── mapping.go            # Source map utilities
│
├── plugin/
│   ├── plugin.go             # Pipeline & interfaces
│   ├── context.go            # Shared context
│   ├── registry.go           # Plugin registration
│   └── builtin/
│       └── result_type.go    # Result<T,E> plugin
│
└── generator/
    └── generator.go          # Code generation orchestration
```

---

## Testing Strategy

### Preprocessor Tests

- Unit tests for each processor
- Validate with `go/parser` (generated code must parse)
- Test edge cases: nested braces, comments, Unicode

Example:
```go
func TestEnumProcessor(t *testing.T) {
    input := `enum Color { Red, Green, Blue }`

    processor := NewEnumProcessor()
    output, _, err := processor.Process([]byte(input), nil)
    require.NoError(t, err)

    // Validate generated Go compiles
    _, err = parser.ParseFile(token.NewFileSet(), "", output, 0)
    require.NoError(t, err, "Generated code must be valid Go")
}
```

### Plugin Tests

- Unit tests for each plugin
- AST comparison tests
- Integration tests with full pipeline

### Golden Tests

- End-to-end: `.dingo` → `.go` → compile
- Both files committed to repo
- Test realistic examples

---

## Performance Characteristics

### Preprocessor Stage

- **Time Complexity**: O(n) where n = source file size
- **Space Complexity**: O(n) for source + O(m) for mappings
- **Bottlenecks**: Regex matching, string concatenation

### Plugin Stage

- **Time Complexity**: O(n * p) where n = AST nodes, p = plugins
- **Space Complexity**: O(n) for AST
- **Bottlenecks**: Multiple AST traversals (one per plugin)

### Future Optimizations

- **Preprocessor**: Use `strings.Builder` for concatenation
- **Plugins**: Combine traversals when possible
- **Caching**: Source map caching for repeated builds

---

## Design Decisions

### Why Not Use a Custom Parser Generator?

**Considered**: participle, Tree-sitter, ANTLR

**Rejected because**:
- Adds dependency and complexity
- Requires maintaining grammar files
- go/parser already exists and works great
- Preprocessor approach is simpler

### Why Regex in Preprocessors?

**Some reviewers questioned this** - "regex is fragile!"

**But regex is CORRECT because**:
- Preprocessors transform text that ISN'T valid Go
- You CAN'T use go/parser on `enum Color {}`
- Regex is the right tool for text transformation
- All preprocessors use this pattern (proven approach)

### Why Not Transpile to a Different Target?

**Could we transpile to Rust/C/LLVM instead of Go?**

No - the entire value proposition is **staying in the Go ecosystem**:
- Use all Go packages
- Deploy to Go infrastructure
- Leverage Go's tooling
- Zero runtime overhead

Transpiling to anything else loses these benefits.

---

## Comparison to Other Transpilers

### TypeScript → JavaScript

- Adds types to untyped language
- Removes type annotations at compile time
- Dingo does the same: adds features, removes at compile time

### Borgo → Go

- Full Rust-like syntax
- Written in Rust
- Monolithic transpiler
- **Dingo differs**: Go-like syntax, written in Go, modular architecture

### templ → Go

- HTML templates as Go functions
- gopls proxy for IDE support
- **Dingo adopts**: Their LSP proxy pattern (future)

---

## Future Architecture

### Phase 3: Type Inference

Integrate go/types for better type inference:
```go
import "go/types"

func (p *ResultTypePlugin) inferType(expr ast.Expr) types.Type {
    return p.typeInfo.TypeOf(expr)
}
```

### Phase 4: Language Server

gopls proxy using templ's pattern:
```go
type Server struct {
    gopls      *subprocess
    sourceMaps map[string]*SourceMap
}

func (s *Server) Handle(req *protocol.Request) (*protocol.Response, error) {
    // Translate request positions using source maps
    translated := s.translateRequest(req)

    // Forward to gopls
    resp, err := s.gopls.Handle(translated)

    // Translate response positions back
    return s.translateResponse(resp), err
}
```

---

## Conclusion

Dingo's two-stage architecture is:

✅ **Simple** - Leverages go/parser instead of custom parsing
✅ **Correct** - Preprocessors use regex because they MUST transform non-Go syntax
✅ **Maintainable** - Clear separation: preprocessors handle syntax, plugins handle semantics
✅ **Extensible** - Easy to add new features as preprocessors or plugins
✅ **Proven** - Similar to TypeScript, Borgo, templ approaches

The architecture correctly addresses the core challenge: **How do you add syntax to Go without forking the Go parser?**

Answer: Transform to valid Go first, then use go/parser.

---

**Questions or concerns about the architecture?**
Open an issue: https://github.com/MadAppGang/dingo/issues
