# Dingo Compiler Implementation Plan

**Version:** 1.0
**Date:** 2025-11-16
**Focus:** Compiler with Plugin Architecture (Phase 1)

---

## Executive Summary

This document outlines the implementation plan for the **Dingo Compiler**, the first of three core tools:

1. **✅ Compiler** (This Document) - Transpiles `.dingo` → `.go`
2. **⏭️ Language Server** (Future) - IDE support via gopls proxy
3. **⏭️ Translate** (Future) - Backwards compiler: `.go` → `.dingo` (code improvement)

### Core Philosophy

**Plugin Architecture:** Every feature is a self-contained, independently toggleable plugin. This enables:
- ✅ Incremental development (build one feature at a time)
- ✅ User control (enable only features they want)
- ✅ Easy testing (test each plugin in isolation)
- ✅ Future extensibility (community can contribute plugins)

### Phase 1 Goal

Build a **working compiler** with **4 quick-win plugins**:
1. Error Propagation (`?`) - 🟢 Low complexity, 1-2 weeks
2. Null Coalescing (`??`) - 🟢 Low complexity, 2-3 days
3. Ternary Operator (`?:`) - 🟢 Low complexity, 2-3 days
4. Functional Utilities - 🟢 Low complexity, 1 week

**Timeline:** ~3-4 weeks for fully functional Phase 1 compiler

---

## Table of Contents

1. [Architecture Overview](#architecture-overview)
2. [Plugin System Design](#plugin-system-design)
3. [Three-Stage Pipeline](#three-stage-pipeline)
4. [Technology Stack](#technology-stack)
5. [Project Structure](#project-structure)
6. [Phase 1 Features](#phase-1-features)
7. [Implementation Roadmap](#implementation-roadmap)
8. [Testing Strategy](#testing-strategy)
9. [Success Criteria](#success-criteria)

---

## Architecture Overview

### Three Core Components

```
┌─────────────────────────────────────────────────────────┐
│                    DINGO COMPILER                        │
│                                                          │
│  ┌──────────┐      ┌───────────┐      ┌─────────────┐  │
│  │          │      │           │      │             │  │
│  │  PARSER  │ ───> │ TRANSFORM │ ───> │  GENERATOR  │  │
│  │          │      │           │      │             │  │
│  └──────────┘      └───────────┘      └─────────────┘  │
│       │                  │                    │         │
│       │                  │                    │         │
│       v                  v                    v         │
│  ┌──────────┐      ┌───────────┐      ┌─────────────┐  │
│  │          │      │           │      │             │  │
│  │ Dingo    │      │ Plugin    │      │  Go Code +  │  │
│  │   AST    │      │  System   │      │ Source Map  │  │
│  │          │      │           │      │             │  │
│  └──────────┘      └───────────┘      └─────────────┘  │
│                                                          │
└─────────────────────────────────────────────────────────┘
```

### Data Flow

```
Input: program.dingo
    ↓
┌─────────────────┐
│ 1. PARSER       │ ──> Dingo AST (with source positions)
└─────────────────┘
    ↓
┌─────────────────┐
│ 2. TRANSFORMER  │ ──> Modified AST (plugins apply transformations)
│    - Plugin 1   │
│    - Plugin 2   │
│    - Plugin N   │
└─────────────────┘
    ↓
┌─────────────────┐
│ 3. GENERATOR    │ ──> program.go + program.dingo.map
│    - Go AST     │
│    - Source Map │
└─────────────────┘
    ↓
Output: program.go (idiomatic Go) + program.dingo.map (positions)
```

---

## Plugin System Design

### Core Principle: Everything is a Plugin

**Design Goals:**
1. **Modularity** - Each feature is self-contained
2. **Toggleable** - Users can enable/disable features via config
3. **Independent** - Plugins don't depend on each other
4. **Composable** - Plugins can chain transformations
5. **Testable** - Each plugin tested in isolation

### Plugin Interface

```go
// pkg/plugin/interface.go

package plugin

import (
    "github.com/jack/dingo/pkg/ast"
    "github.com/jack/dingo/pkg/config"
)

// Plugin represents a feature that can transform Dingo AST
type Plugin interface {
    // Name returns the plugin name (e.g., "error-propagation")
    Name() string

    // Description returns human-readable description
    Description() string

    // Enabled returns whether this plugin is active
    Enabled(cfg *config.Config) bool

    // Priority returns execution order (lower = earlier)
    Priority() int

    // Transform modifies the AST
    Transform(node ast.Node) (ast.Node, error)

    // Validate checks if AST is valid for this plugin
    Validate(node ast.Node) []error
}

// Registry manages all plugins
type Registry struct {
    plugins []Plugin
}

// Register adds a plugin to the registry
func (r *Registry) Register(p Plugin) {
    r.plugins = append(r.plugins, p)
}

// Apply runs all enabled plugins on the AST
func (r *Registry) Apply(node ast.Node, cfg *config.Config) (ast.Node, error) {
    // Sort by priority
    sort.Slice(r.plugins, func(i, j int) bool {
        return r.plugins[i].Priority() < r.plugins[j].Priority()
    })

    // Apply each enabled plugin
    for _, p := range r.plugins {
        if !p.Enabled(cfg) {
            continue
        }

        transformed, err := p.Transform(node)
        if err != nil {
            return nil, fmt.Errorf("plugin %s failed: %w", p.Name(), err)
        }
        node = transformed
    }

    return node, nil
}
```

### Plugin Configuration

```yaml
# dingo.yaml (user config file)

# Global settings
version: "1.0"
output_dir: "./generated"
source_maps: true

# Feature toggles (plugins)
features:
  error_propagation:
    enabled: true

  null_coalescing:
    enabled: true

  ternary_operator:
    enabled: true
    max_nesting: 2  # Plugin-specific config

  functional_utilities:
    enabled: true
    functions: ["map", "filter", "reduce"]  # Which functions to enable

  # Future plugins (disabled for now)
  result_type:
    enabled: false

  pattern_matching:
    enabled: false
```

### Plugin Example: Error Propagation

```go
// pkg/plugin/error_propagation/plugin.go

package error_propagation

import (
    "github.com/jack/dingo/pkg/ast"
    "github.com/jack/dingo/pkg/config"
    "github.com/jack/dingo/pkg/plugin"
)

type ErrorPropagationPlugin struct{}

func New() plugin.Plugin {
    return &ErrorPropagationPlugin{}
}

func (p *ErrorPropagationPlugin) Name() string {
    return "error-propagation"
}

func (p *ErrorPropagationPlugin) Description() string {
    return "Transforms `expr?` into early error returns"
}

func (p *ErrorPropagationPlugin) Enabled(cfg *config.Config) bool {
    return cfg.Features.ErrorPropagation.Enabled
}

func (p *ErrorPropagationPlugin) Priority() int {
    return 100  // Run early (before code gen plugins)
}

func (p *ErrorPropagationPlugin) Transform(node ast.Node) (ast.Node, error) {
    // Walk AST looking for ErrorPropagationExpr nodes
    ast.Walk(node, func(n ast.Node) bool {
        if expr, ok := n.(*ast.ErrorPropagationExpr); ok {
            // Transform: expr? → if err != nil { return err }
            return p.transformErrorProp(expr)
        }
        return true
    })

    return node, nil
}

func (p *ErrorPropagationPlugin) transformErrorProp(expr *ast.ErrorPropagationExpr) ast.Node {
    // Create: __result := expr
    resultVar := ast.NewTempVar("__result")
    assign := &ast.AssignStmt{
        Lhs: resultVar,
        Rhs: expr.Expr,
    }

    // Create: if __result.err != nil { return __result.err }
    errorCheck := &ast.IfStmt{
        Cond: &ast.BinaryExpr{
            Left: &ast.SelectorExpr{X: resultVar, Sel: "err"},
            Op: token.NEQ,
            Right: &ast.Ident{Name: "nil"},
        },
        Body: &ast.BlockStmt{
            List: []ast.Stmt{
                &ast.ReturnStmt{
                    Results: []ast.Expr{
                        &ast.SelectorExpr{X: resultVar, Sel: "err"},
                    },
                },
            },
        },
    }

    // Create: value := *__result.value
    valueAssign := &ast.AssignStmt{
        Lhs: expr.Ident,
        Rhs: &ast.StarExpr{
            X: &ast.SelectorExpr{X: resultVar, Sel: "value"},
        },
    }

    // Return sequence of statements
    return &ast.BlockStmt{
        List: []ast.Stmt{assign, errorCheck, valueAssign},
    }
}

func (p *ErrorPropagationPlugin) Validate(node ast.Node) []error {
    var errors []error

    // Check: `?` can only be used in functions returning Result
    ast.Walk(node, func(n ast.Node) bool {
        if expr, ok := n.(*ast.ErrorPropagationExpr); ok {
            if !p.isInResultReturningFunction(expr) {
                errors = append(errors, fmt.Errorf(
                    "%s: `?` can only be used in functions returning Result",
                    expr.Pos(),
                ))
            }
        }
        return true
    })

    return errors
}
```

---

## Three-Stage Pipeline

### Stage 1: Parser

**Goal:** Convert `.dingo` source text → Dingo AST

**Technology:**
- **Phase 1:** `alecthomas/participle` (rapid prototyping)
- **Phase 2+:** Tree-sitter (incremental parsing for LSP)

**Input:** `program.dingo` (text)
**Output:** `ast.File` (Dingo AST with source positions)

```go
// Parser interface
type Parser interface {
    // Parse converts source text to AST
    Parse(source string, filename string) (*ast.File, error)

    // ParseFile reads and parses a file
    ParseFile(filename string) (*ast.File, error)
}

// Participle-based implementation
type ParticipleParser struct {
    parser *participle.Parser
}

func NewParser() Parser {
    p, err := participle.Build(&ast.File{})
    if err != nil {
        panic(err)
    }
    return &ParticipleParser{parser: p}
}
```

**AST Design:**

```go
// pkg/ast/ast.go

// File represents a complete Dingo source file
type File struct {
    Package  *PackageDecl `parser:"@@"`
    Imports  []*ImportDecl `parser:"@@*"`
    Decls    []Decl       `parser:"@@*"`

    // Source mapping
    Filename string
    Source   string
}

// Expressions (Phase 1 plugins need these)

// ErrorPropagationExpr represents `expr?`
type ErrorPropagationExpr struct {
    Pos  token.Pos
    Expr Expr  `parser:"@@ '?'"`
}

// NullCoalescingExpr represents `expr ?? default`
type NullCoalescingExpr struct {
    Pos     token.Pos
    Expr    Expr `parser:"@@"`
    Default Expr `parser:"'??' '??' @@"`
}

// TernaryExpr represents `cond ? true : false`
type TernaryExpr struct {
    Pos       token.Pos
    Condition Expr `parser:"@@"`
    TrueExpr  Expr `parser:"'?' @@"`
    FalseExpr Expr `parser:"':' @@"`
}

// CallExpr represents function calls (for functional utilities)
type CallExpr struct {
    Pos  token.Pos
    Func Expr   `parser:"@@"`
    Args []Expr `parser:"'(' (@@ (',' @@)*)? ')'"`
}
```

### Stage 2: Transformer

**Goal:** Apply plugin transformations to AST

**Process:**
1. Load config (`dingo.yaml`)
2. Initialize plugin registry
3. Register all available plugins
4. Apply enabled plugins in priority order
5. Validate transformed AST

```go
// pkg/transform/transformer.go

type Transformer struct {
    registry *plugin.Registry
    config   *config.Config
}

func NewTransformer(cfg *config.Config) *Transformer {
    t := &Transformer{
        registry: plugin.NewRegistry(),
        config:   cfg,
    }

    // Register Phase 1 plugins
    t.registry.Register(error_propagation.New())
    t.registry.Register(null_coalescing.New())
    t.registry.Register(ternary_operator.New())
    t.registry.Register(functional_utilities.New())

    // Future plugins (registered but disabled by default)
    // t.registry.Register(result_type.New())
    // t.registry.Register(pattern_matching.New())

    return t
}

func (t *Transformer) Transform(file *ast.File) (*ast.File, error) {
    // Apply all enabled plugins
    transformed, err := t.registry.Apply(file, t.config)
    if err != nil {
        return nil, err
    }

    // Validate result
    errors := t.registry.Validate(transformed)
    if len(errors) > 0 {
        return nil, fmt.Errorf("validation failed: %v", errors)
    }

    return transformed.(*ast.File), nil
}
```

### Stage 3: Generator

**Goal:** Generate idiomatic Go code + source maps

**Technology:**
- `go/ast` - Go's AST representation
- `go/printer` - Pretty-print Go code
- Custom source map generator

**Output:**
- `program.go` - Generated Go code
- `program.dingo.map` - Source position mappings

```go
// pkg/codegen/generator.go

type Generator struct {
    sourceMap *SourceMap
}

func NewGenerator() *Generator {
    return &Generator{
        sourceMap: NewSourceMap(),
    }
}

func (g *Generator) Generate(file *ast.File) (*GeneratedOutput, error) {
    // Convert Dingo AST → Go AST
    goFile := g.dingoToGoAST(file)

    // Generate Go source code
    var buf bytes.Buffer
    if err := printer.Fprint(&buf, token.NewFileSet(), goFile); err != nil {
        return nil, err
    }

    // Build source map
    sourceMapJSON := g.sourceMap.ToJSON()

    return &GeneratedOutput{
        GoSource:  buf.String(),
        SourceMap: sourceMapJSON,
    }, nil
}

func (g *Generator) dingoToGoAST(dingoFile *ast.File) *goast.File {
    // Transform each declaration
    var decls []goast.Decl
    for _, decl := range dingoFile.Decls {
        goDecl := g.transformDecl(decl)
        decls = append(decls, goDecl)

        // Record source mapping
        g.sourceMap.Add(decl.Pos(), goDecl.Pos())
    }

    return &goast.File{
        Name:  goast.NewIdent(dingoFile.Package.Name),
        Decls: decls,
    }
}
```

**Source Map Format:**

```json
{
  "version": 1,
  "file": "program.go",
  "sourceRoot": "",
  "sources": ["program.dingo"],
  "mappings": [
    {
      "dingoLine": 5,
      "dingoCol": 10,
      "goLine": 12,
      "goCol": 4,
      "name": "fetchUser"
    }
  ]
}
```

---

## Technology Stack

### Core Dependencies

```go
// go.mod

module github.com/jack/dingo

go 1.21

require (
    // Parser (Phase 1)
    github.com/alecthomas/participle/v2 v2.1.0

    // Go AST manipulation
    golang.org/x/tools v0.16.0

    // Testing
    github.com/stretchr/testify v1.8.4

    // CLI
    github.com/spf13/cobra v1.8.0
    github.com/spf13/viper v1.18.0

    // Logging
    go.uber.org/zap v1.26.0

    // Future: Tree-sitter (Phase 2+)
    // github.com/tree-sitter/go-tree-sitter v0.20.0
)
```

### Tooling Rationale

| Tool | Purpose | Why Chosen |
|------|---------|------------|
| **participle** | Parser generator | Idiomatic Go, struct-tag grammar, rapid prototyping |
| **x/tools/go/ast** | Go AST manipulation | Official Go tool, battle-tested |
| **cobra** | CLI framework | Industry standard for Go CLIs |
| **viper** | Configuration | Supports YAML/JSON/env vars |
| **testify** | Testing assertions | Clean API, widely used |
| **zap** | Structured logging | High performance, structured |

---

## Project Structure

```
dingo/
├── cmd/
│   └── dingo/                 # CLI entry point
│       ├── main.go           # CLI setup
│       ├── build.go          # `dingo build` command
│       └── version.go        # Version info
│
├── pkg/
│   ├── ast/                  # Dingo AST definitions
│   │   ├── ast.go           # Node types
│   │   ├── walk.go          # AST traversal
│   │   └── print.go         # Debug printing
│   │
│   ├── parser/               # Stage 1: Parser
│   │   ├── parser.go        # Parser interface
│   │   ├── participle.go    # Participle implementation
│   │   └── errors.go        # Parse error handling
│   │
│   ├── plugin/               # Plugin system
│   │   ├── interface.go     # Plugin interface
│   │   ├── registry.go      # Plugin registry
│   │   │
│   │   ├── error_propagation/    # Plugin: `?` operator
│   │   │   ├── plugin.go
│   │   │   ├── transform.go
│   │   │   └── validate.go
│   │   │
│   │   ├── null_coalescing/      # Plugin: `??` operator
│   │   │   ├── plugin.go
│   │   │   └── transform.go
│   │   │
│   │   ├── ternary_operator/     # Plugin: `? :`
│   │   │   ├── plugin.go
│   │   │   ├── transform.go
│   │   │   └── linter.go         # Check nesting depth
│   │   │
│   │   └── functional_utilities/  # Plugin: map/filter/reduce
│   │       ├── plugin.go
│   │       ├── map.go
│   │       ├── filter.go
│   │       └── reduce.go
│   │
│   ├── transform/            # Stage 2: Transformer
│   │   ├── transformer.go   # Main transformer
│   │   └── validate.go      # AST validation
│   │
│   ├── codegen/              # Stage 3: Generator
│   │   ├── generator.go     # Main generator
│   │   ├── dingo_to_go.go   # AST conversion
│   │   └── sourcemap.go     # Source map generation
│   │
│   ├── config/               # Configuration
│   │   ├── config.go        # Config types
│   │   └── loader.go        # Load from YAML
│   │
│   └── compiler/             # Orchestration
│       ├── compiler.go      # Main compiler
│       └── pipeline.go      # 3-stage pipeline
│
├── test/                     # Test data
│   ├── fixtures/            # Test input files
│   │   ├── phase1/
│   │   │   ├── error_prop.dingo
│   │   │   ├── null_coalesce.dingo
│   │   │   ├── ternary.dingo
│   │   │   └── functional.dingo
│   │   └── golden/          # Expected outputs
│   │       ├── error_prop.go
│   │       └── ...
│   └── integration/         # End-to-end tests
│
├── docs/                    # Documentation
│   ├── architecture.md
│   ├── plugin_guide.md
│   └── user_guide.md
│
├── features/                # Feature specs (existing)
├── ai-docs/                 # Research (existing)
│
├── go.mod
├── go.sum
├── dingo.yaml.example       # Example config
├── Makefile                 # Build automation
└── README.md
```

---

## Phase 1 Features

### Feature 1: Error Propagation (`?`)

**Complexity:** 🟢 Low
**Timeline:** 1-2 weeks
**Priority:** P0

**Syntax:**
```dingo
let user = fetchUser(id)?  // Auto-return on error
```

**Transpiles to:**
```go
__result0 := fetchUser(id)
if __result0.err != nil {
    return Result{err: __result0.err}
}
user := *__result0.value
```

**Plugin Tasks:**
- [ ] Define `ErrorPropagationExpr` AST node
- [ ] Parse `expr?` syntax
- [ ] Transform to if/err check + early return
- [ ] Validate: must be in Result-returning function
- [ ] Unit tests (20+ cases)
- [ ] Integration tests

**Test Cases:**
```dingo
// Basic
let x = foo()?

// Chained
let y = foo()?.bar()?.baz()

// In expression
return processUser(fetchUser(id)?)

// Error: not in Result function
func main() {
    let x = foo()?  // ERROR
}
```

### Feature 2: Null Coalescing (`??`)

**Complexity:** 🟢 Low
**Timeline:** 2-3 days
**Priority:** P0

**Syntax:**
```dingo
let name = user?.name ?? "Anonymous"
```

**Transpiles to:**
```go
var name string
if __opt.isSet {
    name = *__opt.value
} else {
    name = "Anonymous"
}
```

**Plugin Tasks:**
- [ ] Define `NullCoalescingExpr` AST node
- [ ] Parse `expr ?? default` syntax
- [ ] Transform to unwrapOr
- [ ] Type checking (both sides must match)
- [ ] Unit tests
- [ ] Integration tests

### Feature 3: Ternary Operator (`? :`)

**Complexity:** 🟢 Low
**Timeline:** 2-3 days
**Priority:** P3

**Syntax:**
```dingo
let max = a > b ? a : b
```

**Transpiles to:**
```go
var max int
if a > b {
    max = a
} else {
    max = b
}
```

**Plugin Tasks:**
- [ ] Define `TernaryExpr` AST node
- [ ] Parse `cond ? true : false` syntax
- [ ] Transform to if/else
- [ ] Type checking (branches must match)
- [ ] Linter: warn on >2 levels nesting
- [ ] Unit tests
- [ ] Integration tests

### Feature 4: Functional Utilities

**Complexity:** 🟢 Low
**Timeline:** 1 week
**Priority:** P2

**Syntax:**
```dingo
let doubled = numbers.map(|x| x * 2)
let evens = numbers.filter(|x| x % 2 == 0)
let sum = numbers.reduce(0, |acc, x| acc + x)
```

**Transpiles to:**
```go
var doubled []int
for _, x := range numbers {
    doubled = append(doubled, x * 2)
}

var evens []int
for _, x := range numbers {
    if x % 2 == 0 {
        evens = append(evens, x)
    }
}

sum := 0
for _, x := range numbers {
    sum = sum + x
}
```

**Plugin Tasks:**
- [ ] Recognize `.map()`, `.filter()`, `.reduce()` calls
- [ ] Transform to explicit for loops
- [ ] Type inference for lambda parameters
- [ ] Unit tests
- [ ] Integration tests

---

## Implementation Roadmap

### Week 1: Foundation

**Goals:**
- ✅ Project setup (go mod, directory structure)
- ✅ Core AST definitions
- ✅ Parser interface + participle implementation
- ✅ Basic CLI (`dingo build`)

**Deliverables:**
- `pkg/ast/` - AST node definitions
- `pkg/parser/` - Parser implementation
- `cmd/dingo/` - CLI skeleton
- Parse simple Dingo programs (no transformations yet)

### Week 2: Plugin System

**Goals:**
- ✅ Plugin interface design
- ✅ Plugin registry
- ✅ Configuration system (dingo.yaml)
- ✅ Transformer orchestration

**Deliverables:**
- `pkg/plugin/interface.go` - Plugin interface
- `pkg/plugin/registry.go` - Plugin registry
- `pkg/config/` - Config loading
- `pkg/transform/` - Transformer
- Plugin enable/disable working

### Week 3: Phase 1 Plugins (Part 1)

**Goals:**
- ✅ Error Propagation plugin
- ✅ Null Coalescing plugin

**Deliverables:**
- `pkg/plugin/error_propagation/` - Complete plugin
- `pkg/plugin/null_coalescing/` - Complete plugin
- Unit tests for both
- Integration tests

### Week 4: Phase 1 Plugins (Part 2) + Generator

**Goals:**
- ✅ Ternary Operator plugin
- ✅ Functional Utilities plugin
- ✅ Code generator (Go AST + source maps)

**Deliverables:**
- `pkg/plugin/ternary_operator/` - Complete plugin
- `pkg/plugin/functional_utilities/` - Complete plugin
- `pkg/codegen/` - Generator with source maps
- End-to-end: `.dingo` → `.go` working!

### Week 5: Polish & Documentation

**Goals:**
- ✅ Comprehensive testing (>80% coverage)
- ✅ Golden file tests
- ✅ Documentation
- ✅ Example programs

**Deliverables:**
- `test/` - Full test suite
- `docs/` - Architecture, plugin guide, user guide
- `examples/` - Sample Dingo programs
- README with quickstart

---

## Testing Strategy

### Unit Tests

**Per Plugin:**
```go
// pkg/plugin/error_propagation/transform_test.go

func TestErrorPropagation_Basic(t *testing.T) {
    input := `let user = fetchUser(id)?`

    expected := &ast.BlockStmt{
        List: []ast.Stmt{
            // __result := fetchUser(id)
            // if __result.err != nil { return __result.err }
            // user := *__result.value
        },
    }

    plugin := New()
    result, err := plugin.Transform(parse(input))

    assert.NoError(t, err)
    assert.Equal(t, expected, result)
}
```

### Integration Tests

**Golden File Testing:**
```go
// test/integration/golden_test.go

func TestGoldenFiles(t *testing.T) {
    fixtures := []string{
        "error_prop",
        "null_coalesce",
        "ternary",
        "functional",
    }

    for _, name := range fixtures {
        t.Run(name, func(t *testing.T) {
            // Compile fixture
            input := filepath.Join("fixtures/phase1", name+".dingo")
            output := compile(input)

            // Compare with golden file
            golden := filepath.Join("fixtures/golden", name+".go")
            expected := readFile(golden)

            assert.Equal(t, expected, output)
        })
    }
}
```

### End-to-End Tests

**Real Programs:**
```dingo
// test/fixtures/phase1/full_program.dingo

package main

func fetchUser(id: string) -> Result<User, Error> {
    let data = db.query(id)?
    let user = parseUser(data)?
    return Ok(user)
}

func main() {
    let user = fetchUser("123") ?? User.guest()
    let greeting = user.isActive ? "Welcome!" : "Account inactive"

    let numbers = [1, 2, 3, 4, 5]
    let doubled = numbers.map(|x| x * 2)

    println("${greeting}, ${user.name}")
    println("Doubled: ${doubled}")
}
```

---

## Success Criteria

### Phase 1 Complete When:

- [ ] **All 4 plugins working:**
  - [ ] Error Propagation (`?`)
  - [ ] Null Coalescing (`??`)
  - [ ] Ternary Operator (`? :`)
  - [ ] Functional Utilities (map/filter/reduce)

- [ ] **Compiler functionality:**
  - [ ] `dingo build` CLI command works
  - [ ] Generates idiomatic Go code
  - [ ] Generates source maps
  - [ ] Config-based plugin enable/disable
  - [ ] Handles parse errors gracefully

- [ ] **Testing:**
  - [ ] >80% code coverage
  - [ ] 50+ unit tests
  - [ ] 10+ integration tests (golden files)
  - [ ] 5+ end-to-end programs compile and run

- [ ] **Documentation:**
  - [ ] Architecture doc
  - [ ] Plugin development guide
  - [ ] User guide (getting started)
  - [ ] API docs (godoc)

- [ ] **Quality:**
  - [ ] No panics on invalid input
  - [ ] Helpful error messages
  - [ ] Generated Go passes `go vet`
  - [ ] Generated Go is readable (hand-written quality)

### Demo Program

**Goal:** Build a real program using all Phase 1 features

```dingo
// demo/http_server.dingo

package main

import "net/http"

func fetchUserData(userID: string) -> Result<UserData, Error> {
    let resp = http.Get("/api/users/" + userID)?
    let user = parseUser(resp.Body)?
    let posts = fetchPosts(user.ID)?
    return Ok(UserData{user, posts})
}

func handler(w: http.ResponseWriter, r: *http.Request) {
    let userID = r.URL.Query().Get("id") ?? "default"

    let data = fetchUserData(userID)
    let message = data.isOk() ? "Success" : "Error"

    let statuses = [200, 404, 500]
    let successCodes = statuses.filter(|code| code < 400)

    w.Write([]byte(message))
}

func main() {
    http.HandleFunc("/user", handler)
    http.ListenAndServe(":8080", nil)
}
```

**Acceptance:** This compiles to valid Go and runs without errors.

---

## Next Steps (Post-Phase 1)

### Phase 2: Core Type System (8-10 weeks)

Plugins to build:
1. Sum Types
2. Result<T, E> type
3. Option<T> type
4. Pattern Matching
5. Enums

### Phase 3: Language Server (4-6 weeks)

**Tool #2:** `dingo-lsp`
- LSP proxy architecture
- gopls integration
- Source map translation
- Real-time transpilation

### Phase 4: Translate Tool (2-3 weeks)

**Tool #3:** `dingo translate`
- Backwards compiler: Go → Dingo
- Code improvement suggestions
- Automated refactoring

---

## Conclusion

This implementation plan provides a **clear, phased approach** to building the Dingo compiler with a **plugin architecture** that enables:

✅ **Incremental development** - Build one feature at a time
✅ **User control** - Enable only desired features
✅ **Easy testing** - Test each plugin independently
✅ **Future extensibility** - Community can add plugins

**Phase 1 delivers 4 high-impact, low-complexity features in 4-5 weeks**, proving the architecture and providing immediate value to users.

Let's build it! 🚀
