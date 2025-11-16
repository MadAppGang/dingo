# Dingo: Pure Go Implementation Guide

**Version:** 1.0
**Date:** 2025-11-16
**Language:** 100% Go (No Rust, No Dependencies)

---

## 🎯 Core Principle: Everything in Go

**IMPORTANT:** Dingo is implemented **entirely in Go**. We're only **studying Borgo's patterns**, not using Borgo's code.

**Why Pure Go:**
- ✅ Go developers already have Go installed
- ✅ No external toolchain dependencies (no Rust, no Node.js)
- ✅ Go developers can contribute immediately
- ✅ Faster iteration (no cargo build waits)
- ✅ Single binary distribution
- ✅ Native Go ecosystem integration

**What We Learn from Borgo:**
- ✅ Transpilation patterns (how to generate Go code)
- ✅ Feature designs (what Result/Option look like)
- ✅ Architectural patterns (Parser → TypeChecker → CodeGen)
- ❌ NOT using Borgo's Rust code
- ❌ NOT depending on Rust toolchain
- ❌ NOT reusing Borgo's parser

---

## Table of Contents

1. [Go-Native Architecture](#go-native-architecture)
2. [Borgo Patterns → Go Translation](#borgo-patterns--go-translation)
3. [Pure Go Tooling Stack](#pure-go-tooling-stack)
4. [Concrete Go Implementation](#concrete-go-implementation)
5. [Phase 1: Go Code Examples](#phase-1-go-code-examples)
6. [Build & Distribution](#build--distribution)

---

## Go-Native Architecture

### Project Structure (Pure Go)

```
dingo/
├── go.mod                  # Go modules (no Rust!)
├── go.sum
├── main.go                 # Entry point
│
├── cmd/
│   └── dingo/
│       ├── main.go        # CLI main
│       ├── build.go       # Build command
│       └── version.go     # Version info
│
├── pkg/
│   ├── ast/               # 100% Go AST
│   │   ├── ast.go
│   │   ├── walk.go
│   │   └── visitor.go
│   │
│   ├── parser/            # 100% Go parser
│   │   ├── parser.go      # Using participle (pure Go)
│   │   └── lexer.go       # Go tokenizer
│   │
│   ├── plugin/            # 100% Go plugin system
│   │   ├── interface.go
│   │   ├── registry.go
│   │   └── error_propagation/
│   │       └── plugin.go  # Pure Go implementation
│   │
│   ├── transform/         # 100% Go transformations
│   │   ├── transformer.go
│   │   └── visitor.go
│   │
│   ├── codegen/           # 100% Go code generation
│   │   ├── generator.go
│   │   ├── go_emitter.go
│   │   └── sourcemap.go
│   │
│   └── compiler/          # 100% Go compiler
│       ├── compiler.go
│       └── pipeline.go
│
└── internal/              # 100% Go internals
    ├── config/
    └── util/
```

**Dependencies (All Pure Go):**
```go
// go.mod
module github.com/yourusername/dingo

go 1.21

require (
    github.com/alecthomas/participle/v2 v2.1.0  // Pure Go parser
    github.com/spf13/cobra v1.8.0               // Pure Go CLI
    github.com/spf13/viper v1.18.0              // Pure Go config
    golang.org/x/tools v0.16.0                  // Official Go tools
    github.com/stretchr/testify v1.8.4          // Pure Go testing
)

// NO Rust dependencies!
// NO C dependencies!
// NO external compilers!
```

---

## Borgo Patterns → Go Translation

### Pattern 1: Tagged Union (Borgo → Go)

**What Borgo Does (Rust):**
```rust
// Borgo's Rust code (we DON'T use this)
enum Result<T, E> {
    Ok(T),
    Err(E),
}

// Transpiles to Go generics
```

**What Dingo Does (Pure Go):**
```go
// pkg/codegen/enum.go - Pure Go implementation

package codegen

import (
    "fmt"
    "go/ast"
    "go/token"
)

// GenerateEnum creates Go code for a Dingo enum
func (g *Generator) GenerateEnum(enum *dingoast.EnumDecl) []ast.Decl {
    var decls []ast.Decl

    // 1. Generate tag type
    tagType := g.generateTagType(enum)
    decls = append(decls, tagType)

    // 2. Generate tag constants
    tagConsts := g.generateTagConstants(enum)
    decls = append(decls, tagConsts)

    // 3. Generate struct
    structDecl := g.generateEnumStruct(enum)
    decls = append(decls, structDecl)

    // 4. Generate constructors
    for _, variant := range enum.Variants {
        constructor := g.generateConstructor(enum, variant)
        decls = append(decls, constructor)
    }

    return decls
}

func (g *Generator) generateTagType(enum *dingoast.EnumDecl) *ast.GenDecl {
    return &ast.GenDecl{
        Tok: token.TYPE,
        Specs: []ast.Spec{
            &ast.TypeSpec{
                Name: ast.NewIdent(enum.Name + "Tag"),
                Type: ast.NewIdent("int"),
            },
        },
    }
}

func (g *Generator) generateTagConstants(enum *dingoast.EnumDecl) *ast.GenDecl {
    specs := []ast.Spec{}

    for i, variant := range enum.Variants {
        var value ast.Expr
        if i == 0 {
            value = &ast.CallExpr{
                Fun: ast.NewIdent("iota"),
            }
        }

        specs = append(specs, &ast.ValueSpec{
            Names: []*ast.Ident{
                ast.NewIdent(enum.Name + "_" + variant.Name),
            },
            Type: ast.NewIdent(enum.Name + "Tag"),
            Values: []ast.Expr{value},
        })
    }

    return &ast.GenDecl{
        Tok: token.CONST,
        Specs: specs,
    }
}

func (g *Generator) generateEnumStruct(enum *dingoast.EnumDecl) *ast.GenDecl {
    fields := []*ast.Field{
        // Tag field
        {
            Names: []*ast.Ident{ast.NewIdent("tag")},
            Type:  ast.NewIdent(enum.Name + "Tag"),
        },
    }

    // Add fields for each variant's data
    for _, variant := range enum.Variants {
        for i, field := range variant.Fields {
            fieldName := fmt.Sprintf("%s%d", variant.Name, i)
            fields = append(fields, &ast.Field{
                Names: []*ast.Ident{ast.NewIdent(fieldName)},
                Type:  g.typeToGoType(field.Type),
            })
        }
    }

    return &ast.GenDecl{
        Tok: token.TYPE,
        Specs: []ast.Spec{
            &ast.TypeSpec{
                Name: ast.NewIdent(enum.Name),
                TypeParams: g.generateTypeParams(enum.TypeParams),
                Type: &ast.StructType{
                    Fields: &ast.FieldList{List: fields},
                },
            },
        },
    }
}
```

**Key Point:** This is **pure Go code** using Go's `go/ast` package. No Rust involved!

### Pattern 2: Error Propagation (Borgo → Go)

**What Borgo Does (Rust):**
```rust
// Borgo's Rust codegen (we DON'T use this)
fn emit_try_expr(&mut self, expr: &Expr) -> EmitResult {
    // Rust code for code generation
}
```

**What Dingo Does (Pure Go):**
```go
// pkg/plugin/error_propagation/transform.go - Pure Go

package error_propagation

import (
    "fmt"
    "go/ast"
    "go/token"

    dingoast "github.com/yourusername/dingo/pkg/ast"
)

type Plugin struct{}

func New() *Plugin {
    return &Plugin{}
}

// Transform converts expr? to Go error checking
func (p *Plugin) Transform(node dingoast.Node) (dingoast.Node, error) {
    return dingoast.Walk(node, func(n dingoast.Node) dingoast.Node {
        // Find ErrorPropagationExpr (expr?)
        if tryExpr, ok := n.(*dingoast.ErrorPropagationExpr); ok {
            return p.transformTryExpr(tryExpr)
        }
        return n
    })
}

func (p *Plugin) transformTryExpr(expr *dingoast.ErrorPropagationExpr) dingoast.Node {
    // Generate: __result0 := expr
    resultVar := p.genTempVar()

    // Generate:
    // __result0 := expr
    // if __result0.err != nil {
    //     return __result0.err
    // }
    // value := __result0.value

    return &dingoast.BlockStmt{
        List: []dingoast.Stmt{
            // Assignment
            &dingoast.AssignStmt{
                Lhs: []dingoast.Expr{resultVar},
                Tok: token.DEFINE,
                Rhs: []dingoast.Expr{expr.Expr},
            },

            // Error check
            &dingoast.IfStmt{
                Cond: &dingoast.BinaryExpr{
                    X: &dingoast.SelectorExpr{
                        X:   resultVar,
                        Sel: ast.NewIdent("err"),
                    },
                    Op: token.NEQ,
                    Y:  ast.NewIdent("nil"),
                },
                Body: &dingoast.BlockStmt{
                    List: []dingoast.Stmt{
                        &dingoast.ReturnStmt{
                            Results: []dingoast.Expr{
                                &dingoast.SelectorExpr{
                                    X:   resultVar,
                                    Sel: ast.NewIdent("err"),
                                },
                            },
                        },
                    },
                },
            },

            // Unwrap value
            &dingoast.AssignStmt{
                Lhs: []dingoast.Expr{ast.NewIdent(expr.VarName)},
                Tok: token.DEFINE,
                Rhs: []dingoast.Expr{
                    &dingoast.StarExpr{
                        X: &dingoast.SelectorExpr{
                            X:   resultVar,
                            Sel: ast.NewIdent("value"),
                        },
                    },
                },
            },
        },
    }
}

var tempVarCounter int

func (p *Plugin) genTempVar() *ast.Ident {
    tempVarCounter++
    return ast.NewIdent(fmt.Sprintf("__result%d", tempVarCounter))
}
```

**Key Point:** This is **pure Go**, using Go's standard library. Learning the pattern from Borgo, implementing in Go!

### Pattern 3: Parser (Borgo → Go)

**What Borgo Does (Rust):**
```rust
// Borgo reuses Rust's parser (we CAN'T do this)
// They use the 'syn' crate
```

**What Dingo Does (Pure Go):**
```go
// pkg/parser/parser.go - Pure Go with participle

package parser

import (
    "github.com/alecthomas/participle/v2"
    "github.com/alecthomas/participle/v2/lexer"

    "github.com/yourusername/dingo/pkg/ast"
)

// Parser is a pure Go parser for Dingo
type Parser struct {
    participle *participle.Parser[ast.File]
}

// NewParser creates a new parser (100% Go)
func NewParser() (*Parser, error) {
    // Define lexer (pure Go)
    dingoLexer := lexer.MustSimple([]lexer.SimpleRule{
        {Name: "Keyword", Pattern: `\b(func|let|return|if|else|match|enum)\b`},
        {Name: "Ident", Pattern: `[a-zA-Z_][a-zA-Z0-9_]*`},
        {Name: "Number", Pattern: `\d+`},
        {Name: "String", Pattern: `"[^"]*"`},
        {Name: "Operator", Pattern: `[+\-*/=!<>?:]`},
        {Name: "Punct", Pattern: `[(){}\[\],;.]`},
        {Name: "Whitespace", Pattern: `\s+`},
    })

    // Build parser (pure Go)
    parser, err := participle.Build[ast.File](
        participle.Lexer(dingoLexer),
        participle.Elide("Whitespace"),
    )
    if err != nil {
        return nil, err
    }

    return &Parser{participle: parser}, nil
}

// Parse parses Dingo source code to AST (pure Go)
func (p *Parser) Parse(source string) (*ast.File, error) {
    file, err := p.participle.ParseString("", source)
    if err != nil {
        return nil, err
    }
    return file, nil
}

// ParseFile parses a Dingo file (pure Go)
func (p *Parser) ParseFile(filename string) (*ast.File, error) {
    file, err := p.participle.ParseFile(filename)
    if err != nil {
        return nil, err
    }
    return file, nil
}
```

**Key Point:** Using **participle** (pure Go library), not Rust's parser!

---

## Pure Go Tooling Stack

### All Dependencies Are Pure Go

```go
// go.mod - Every dependency is 100% Go

module github.com/yourusername/dingo

go 1.21

require (
    // Parser - Pure Go parser generator
    github.com/alecthomas/participle/v2 v2.1.0

    // AST manipulation - Official Go tools
    golang.org/x/tools v0.16.0

    // CLI - Pure Go framework
    github.com/spf13/cobra v1.8.0

    // Config - Pure Go library
    github.com/spf13/viper v1.18.0

    // Testing - Pure Go
    github.com/stretchr/testify v1.8.4

    // Logging - Pure Go
    go.uber.org/zap v1.26.0
)

// ZERO non-Go dependencies
// ZERO CGO
// ZERO Rust
// ZERO Node.js
```

### Build Process (Pure Go)

```bash
# Build Dingo compiler (just Go!)
$ go build -o dingo ./cmd/dingo

# Install globally
$ go install ./cmd/dingo

# Run tests
$ go test ./...

# That's it! No cargo, no rustc, no npm!
```

### Distribution (Single Binary)

```bash
# Cross-compile for all platforms (pure Go!)
$ GOOS=linux GOARCH=amd64 go build -o dingo-linux-amd64
$ GOOS=darwin GOARCH=arm64 go build -o dingo-darwin-arm64
$ GOOS=windows GOARCH=amd64 go build -o dingo-windows-amd64.exe

# Single binary, no dependencies!
```

---

## Concrete Go Implementation

### Example: Complete Error Propagation Plugin (Pure Go)

```go
// pkg/plugin/error_propagation/plugin.go

package error_propagation

import (
    "github.com/yourusername/dingo/pkg/ast"
    "github.com/yourusername/dingo/pkg/config"
    "github.com/yourusername/dingo/pkg/plugin"
)

var _ plugin.Plugin = (*ErrorPropagationPlugin)(nil)

type ErrorPropagationPlugin struct {
    config *config.Config
}

func New() plugin.Plugin {
    return &ErrorPropagationPlugin{}
}

func (p *ErrorPropagationPlugin) Name() string {
    return "error-propagation"
}

func (p *ErrorPropagationPlugin) Description() string {
    return "Transforms `expr?` into early error returns"
}

func (p *ErrorPropagationPlugin) Priority() int {
    return 100 // Run early
}

func (p *ErrorPropagationPlugin) Enabled(cfg *config.Config) bool {
    return cfg.Features.ErrorPropagation.Enabled
}

func (p *ErrorPropagationPlugin) Transform(node ast.Node) (ast.Node, error) {
    // Walk AST and transform ? expressions
    visitor := &errorPropVisitor{}
    return ast.Walk(node, visitor.Visit), nil
}

func (p *ErrorPropagationPlugin) Validate(node ast.Node) []error {
    var errors []error

    // Check: ? can only be used in Result-returning functions
    ast.Inspect(node, func(n ast.Node) bool {
        if tryExpr, ok := n.(*ast.ErrorPropagationExpr); ok {
            if !p.isInResultFunction(tryExpr) {
                errors = append(errors, fmt.Errorf(
                    "%s: `?` can only be used in functions returning Result",
                    tryExpr.Pos(),
                ))
            }
        }
        return true
    })

    return errors
}

// Visitor for transforming ? expressions
type errorPropVisitor struct {
    counter int
}

func (v *errorPropVisitor) Visit(node ast.Node) ast.Node {
    if tryExpr, ok := node.(*ast.ErrorPropagationExpr); ok {
        return v.transformTry(tryExpr)
    }
    return node
}

func (v *errorPropVisitor) transformTry(expr *ast.ErrorPropagationExpr) ast.Node {
    v.counter++
    resultVar := fmt.Sprintf("__result%d", v.counter)

    // Build the transformation using Go's go/ast
    return &ast.BlockStmt{
        List: []ast.Stmt{
            // __resultN := expr
            &ast.AssignStmt{
                Lhs: []ast.Expr{ast.NewIdent(resultVar)},
                Tok: token.DEFINE,
                Rhs: []ast.Expr{expr.Expr},
            },

            // if __resultN.err != nil { return __resultN.err }
            &ast.IfStmt{
                Cond: &ast.BinaryExpr{
                    X: &ast.SelectorExpr{
                        X:   ast.NewIdent(resultVar),
                        Sel: ast.NewIdent("err"),
                    },
                    Op: token.NEQ,
                    Y:  ast.NewIdent("nil"),
                },
                Body: &ast.BlockStmt{
                    List: []ast.Stmt{
                        &ast.ReturnStmt{
                            Results: []ast.Expr{
                                &ast.SelectorExpr{
                                    X:   ast.NewIdent(resultVar),
                                    Sel: ast.NewIdent("err"),
                                },
                            },
                        },
                    },
                },
            },

            // value := *__resultN.value
            &ast.AssignStmt{
                Lhs: []ast.Expr{ast.NewIdent(expr.VarName)},
                Tok: token.DEFINE,
                Rhs: []ast.Expr{
                    &ast.StarExpr{
                        X: &ast.SelectorExpr{
                            X:   ast.NewIdent(resultVar),
                            Sel: ast.NewIdent("value"),
                        },
                    },
                },
            },
        },
    }
}
```

**This is 100% Go code!** Using only Go standard library and pure Go packages.

---

## Phase 1: Go Code Examples

### Main Compiler (Pure Go)

```go
// cmd/dingo/main.go

package main

import (
    "fmt"
    "os"

    "github.com/spf13/cobra"
    "github.com/yourusername/dingo/pkg/compiler"
    "github.com/yourusername/dingo/pkg/config"
)

func main() {
    var cfgFile string

    rootCmd := &cobra.Command{
        Use:   "dingo",
        Short: "Dingo - A better Go",
        Long:  "Dingo transpiles .dingo files to idiomatic Go code",
    }

    buildCmd := &cobra.Command{
        Use:   "build [files...]",
        Short: "Build Dingo source files",
        RunE: func(cmd *cobra.Command, args []string) error {
            // Load config (pure Go)
            cfg, err := config.Load(cfgFile)
            if err != nil {
                return err
            }

            // Create compiler (pure Go)
            comp, err := compiler.New(cfg)
            if err != nil {
                return err
            }

            // Compile each file (pure Go)
            for _, file := range args {
                if err := comp.CompileFile(file); err != nil {
                    return fmt.Errorf("compiling %s: %w", file, err)
                }
            }

            return nil
        },
    }

    buildCmd.Flags().StringVar(&cfgFile, "config", "dingo.yaml", "config file")
    rootCmd.AddCommand(buildCmd)

    if err := rootCmd.Execute(); err != nil {
        os.Exit(1)
    }
}
```

### Compiler Pipeline (Pure Go)

```go
// pkg/compiler/compiler.go

package compiler

import (
    "fmt"
    "os"

    "github.com/yourusername/dingo/pkg/ast"
    "github.com/yourusername/dingo/pkg/codegen"
    "github.com/yourusername/dingo/pkg/config"
    "github.com/yourusername/dingo/pkg/parser"
    "github.com/yourusername/dingo/pkg/plugin"
    "github.com/yourusername/dingo/pkg/transform"
)

type Compiler struct {
    parser      *parser.Parser
    transformer *transform.Transformer
    generator   *codegen.Generator
    config      *config.Config
}

func New(cfg *config.Config) (*Compiler, error) {
    // Initialize parser (pure Go)
    p, err := parser.NewParser()
    if err != nil {
        return nil, err
    }

    // Initialize transformer with plugins (pure Go)
    t := transform.NewTransformer(cfg)

    // Register Phase 1 plugins (all pure Go)
    t.RegisterPlugin(error_propagation.New())
    t.RegisterPlugin(null_coalescing.New())
    t.RegisterPlugin(ternary_operator.New())
    t.RegisterPlugin(functional_utilities.New())

    // Initialize generator (pure Go)
    g := codegen.NewGenerator()

    return &Compiler{
        parser:      p,
        transformer: t,
        generator:   g,
        config:      cfg,
    }, nil
}

func (c *Compiler) CompileFile(filename string) error {
    // 1. Parse (pure Go)
    file, err := c.parser.ParseFile(filename)
    if err != nil {
        return fmt.Errorf("parse error: %w", err)
    }

    // 2. Transform (pure Go plugins)
    transformed, err := c.transformer.Transform(file)
    if err != nil {
        return fmt.Errorf("transform error: %w", err)
    }

    // 3. Generate (pure Go)
    output, err := c.generator.Generate(transformed)
    if err != nil {
        return fmt.Errorf("codegen error: %w", err)
    }

    // 4. Write output (pure Go)
    outFile := filename + ".go"
    if err := os.WriteFile(outFile, []byte(output.GoSource), 0644); err != nil {
        return fmt.Errorf("write error: %w", err)
    }

    // 5. Write source map (pure Go)
    mapFile := filename + ".dingo.map"
    if err := os.WriteFile(mapFile, []byte(output.SourceMap), 0644); err != nil {
        return fmt.Errorf("write sourcemap error: %w", err)
    }

    fmt.Printf("✓ Compiled %s -> %s\n", filename, outFile)
    return nil
}
```

### AST Definition (Pure Go)

```go
// pkg/ast/ast.go

package ast

import (
    "go/token"
)

// File represents a Dingo source file
type File struct {
    Package  string       `parser:"'package' @Ident"`
    Imports  []*Import    `parser:"@@*"`
    Decls    []Decl       `parser:"@@*"`

    // Source mapping
    Filename string
    Source   string
}

// ErrorPropagationExpr represents `expr?`
type ErrorPropagationExpr struct {
    Pos     token.Pos
    Expr    Expr   `parser:"@@ '?'"`
    VarName string // Variable to bind result to
}

// NullCoalescingExpr represents `expr ?? default`
type NullCoalescingExpr struct {
    Pos     token.Pos
    Expr    Expr `parser:"@@"`
    Default Expr `parser:"'?' '?' @@"`
}

// TernaryExpr represents `cond ? true : false`
type TernaryExpr struct {
    Pos       token.Pos
    Condition Expr `parser:"@@"`
    TrueExpr  Expr `parser:"'?' @@"`
    FalseExpr Expr `parser:"':' @@"`
}

// ... more AST nodes (all pure Go structs)
```

---

## Build & Distribution

### Makefile (Pure Go Commands)

```makefile
# Makefile - Pure Go build

.PHONY: all build test clean install

# Build the Dingo compiler (pure Go)
build:
	go build -o bin/dingo ./cmd/dingo

# Run tests (pure Go)
test:
	go test -v ./...

# Install globally (pure Go)
install:
	go install ./cmd/dingo

# Clean build artifacts
clean:
	rm -rf bin/
	go clean

# Cross-compile for all platforms (pure Go!)
release:
	GOOS=linux GOARCH=amd64 go build -o dist/dingo-linux-amd64 ./cmd/dingo
	GOOS=darwin GOARCH=amd64 go build -o dist/dingo-darwin-amd64 ./cmd/dingo
	GOOS=darwin GOARCH=arm64 go build -o dist/dingo-darwin-arm64 ./cmd/dingo
	GOOS=windows GOARCH=amd64 go build -o dist/dingo-windows-amd64.exe ./cmd/dingo

# Run example (pure Go)
example:
	./bin/dingo build examples/hello.dingo
	go run examples/hello.dingo.go
```

### Installation (Zero Dependencies)

```bash
# Option 1: Install from source
$ git clone https://github.com/yourusername/dingo
$ cd dingo
$ go install ./cmd/dingo

# Option 2: Download binary (single file!)
$ curl -L https://github.com/yourusername/dingo/releases/latest/download/dingo-$(uname)-$(uname -m) -o dingo
$ chmod +x dingo
$ ./dingo build myfile.dingo

# That's it! No dependencies!
```

---

## Summary: Pure Go Implementation

### What We Learned from Borgo

| Learning | How We Use It (Pure Go) |
|----------|------------------------|
| **Tagged unions** | ✅ Implement in Go using structs + tag |
| **Error propagation pattern** | ✅ Generate Go error checks |
| **Expression contexts** | ✅ Track in Go transformer |
| **Type inference** | ✅ Implement in Go (Phase 2) |
| **Exhaustiveness checking** | ✅ Implement in Go (Phase 2) |

### What We DON'T Use from Borgo

| Borgo Has | Dingo Doesn't Use |
|-----------|------------------|
| ❌ Rust compiler code | ✅ We write our own in Go |
| ❌ Rust parser (`syn`) | ✅ We use `participle` (Go) |
| ❌ Rust dependencies | ✅ Only Go dependencies |
| ❌ Cargo build system | ✅ Just `go build` |
| ❌ Rust toolchain | ✅ Only Go toolchain |

### Technology Stack (100% Go)

```
┌─────────────────────────────────┐
│     Dingo Compiler (Go)         │
├─────────────────────────────────┤
│ Parser:       participle (Go)   │
│ AST:          go/ast (Go)       │
│ Transform:    Custom (Go)       │
│ CodeGen:      go/printer (Go)   │
│ CLI:          cobra (Go)        │
│ Config:       viper (Go)        │
│ Testing:      testify (Go)      │
└─────────────────────────────────┘
         ↓
    Single Go Binary
         ↓
    Works Anywhere!
```

---

## Next Steps: Start Coding (Pure Go!)

**Week 1: Project Setup**
```bash
$ mkdir dingo && cd dingo
$ go mod init github.com/yourusername/dingo
$ go get github.com/alecthomas/participle/v2
$ go get github.com/spf13/cobra
$ mkdir -p cmd/dingo pkg/{ast,parser,plugin,transform,codegen,compiler}
```

**Week 2: Parser (Pure Go)**
```bash
$ cd pkg/parser
$ touch parser.go
# Implement using participle (pure Go)
```

**Week 3: Plugins (Pure Go)**
```bash
$ cd pkg/plugin/error_propagation
$ touch plugin.go transform.go
# Implement error propagation (pure Go)
```

**Week 4: Code Generator (Pure Go)**
```bash
$ cd pkg/codegen
$ touch generator.go
# Implement using go/ast (pure Go)
```

**Week 5: Testing & Polish (Pure Go)**
```bash
$ go test ./...
# All tests in Go!
```

---

## Conclusion: Everything in Go!

**Dingo is 100% Go:**
- ✅ Written in Go
- ✅ Uses Go standard library
- ✅ Dependencies are pure Go
- ✅ Builds with `go build`
- ✅ Single binary output
- ✅ No external toolchains

**Borgo is a reference:**
- ✅ Shows what features to implement
- ✅ Proves transpilation is viable
- ✅ Demonstrates Go interop patterns
- ❌ We don't use Borgo's code
- ❌ We don't depend on Rust
- ❌ We implement everything ourselves in Go

**Let's build Dingo in pure Go!** 🚀
