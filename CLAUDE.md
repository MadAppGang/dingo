# Claude AI Agent Instructions - Dingo Project

## 🚨🚨🚨 STOP: READ THIS BEFORE ANY IMPLEMENTATION 🚨🚨🚨

**We have FAILED to follow this rule MULTIPLE TIMES. This is non-negotiable.**

### The Architectural Principle (understand this FIRST)

> **Position information flows through the token system, NEVER through byte arithmetic.**

The rule isn't "avoid these 5 functions". The rule is: ALL position tracking must use `token.Pos` and `token.FileSet`, not byte offsets or string scanning.

### WHY This Rule Exists

Byte offsets are **FRAGILE** - they shift when go/printer reformats code:
```
Before go/printer: "x := foo()\n"    (offset 5 = 'f')
After go/printer:  "x := foo()\n"    (tabs→spaces, offset 5 = different char!)
```

`token.Pos` is **STABLE** because it's a logical reference into a FileSet, not a physical byte position. The Go toolchain uses this throughout for a reason.

### ❌ FORBIDDEN Patterns (ALL of these, not just the listed functions):

**Direct byte/string manipulation:**
- `bytes.Index()`, `bytes.HasPrefix()`, `bytes.Contains()`
- `strings.Index()`, `strings.Contains()`, `strings.Split(string(src), ...)`
- `regexp.MustCompile()`, `regexp.Match()`, `regexp.Find*()`

**Character scanning loops:**
- `for i := 0; i < len(src); i++ { if src[i] == '?' ... }`
- Any loop that counts newlines to calculate line numbers

**Byte slice extraction from source:**
- `src[start:end]` for extracting source content by offset
- `string(src[loc.Start:loc.End])`

**Output scanning:**
- Scanning generated Go code to extract position info
- Regex/string matching on output to find `//line` directives

### ✅ REQUIRED Approaches (the ONLY correct ways):

**For Dingo source positions:**
```go
// Positions come from AST nodes (parser already tracked them)
pos := dingoNode.Pos()  // This is token.Pos
position := dingoFset.Position(pos)  // Line, column
```

**For Go source analysis (e.g., finding //line directives):**
```go
// Use go/scanner - it tokenizes with proper position tracking
fset := token.NewFileSet()
file := fset.AddFile("", fset.Base(), len(goSource))
var s scanner.Scanner
s.Init(file, goSource, nil, scanner.ScanComments)

for {
    pos, tok, lit := s.Scan()
    if tok == token.EOF { break }
    if tok == token.COMMENT && strings.HasPrefix(lit, "//line ") {
        goLine := fset.Position(pos).Line  // Token-based!
    }
}
```

**For byte offset → line:col conversion:**
```go
// Use token.FileSet, NOT manual newline counting
fset := token.NewFileSet()
file := fset.AddFile("", fset.Base(), len(src))
file.SetLinesForContent(src)  // FileSet scans internally
position := fset.Position(file.Pos(offset))
return position.Line, position.Column
```

**Key insight: Track during generation, don't scan output**
```go
// ❌ WRONG: Scan output after generation
lineMappings := extractFromOutput(generatedCode)

// ✅ RIGHT: Track during generation
for _, transform := range transforms {
    emit(transform.Code)
    mappings = append(mappings, LineMapping{
        DingoLine: transform.SourceLine,  // Already known from AST!
    })
}
```

### Pipeline Architecture
```
Source → pkg/tokenizer/ → []Token → pkg/parser/ → AST → pkg/ast/*_codegen.go
         ↑                                                 ↑
   ONLY place that                                   ONLY accepts
   reads raw bytes                                   AST nodes
```

### Pre-Implementation Checklist

Before writing ANY transform code, ask:
- [ ] Am I about to read raw bytes? → Only `pkg/tokenizer/` should
- [ ] Am I about to scan for a pattern? → Use `go/scanner` instead
- [ ] Am I calculating line:col from offset? → Use `token.FileSet`
- [ ] Am I extracting info from output? → Track during generation instead
- [ ] Would this break if go/printer reformats? → It's wrong

### Before implementing ANY feature:
1. Check if parser already handles it: `pkg/parser/`
2. Check if codegen exists: `pkg/ast/*_codegen.go`
3. If adding new syntax: extend parser FIRST, then codegen from AST

### Verification (run before ANY PR):
```bash
grep -rn "bytes\.Index\|bytes\.HasPrefix\|strings\.Index\|strings\.Split.*src\|regexp\." \
  pkg/transpiler/*.go pkg/codegen/*.go pkg/ast/*_codegen.go 2>/dev/null \
  | grep -v "_test\.go" | grep -v "// OK:"
# Must return NOTHING
```

### Post-Edit Hook

A verification hook runs automatically after code changes to catch violations.
See `.claude/hooks/README.md` for details.

---

## What is Dingo?
A meta-language for Go (like TypeScript for JavaScript):
- Transpiles `.dingo` files to idiomatic `.go` files
- Provides Result/Option types, pattern matching, error propagation (`?`)
- 100% Go ecosystem compatibility via gopls proxy for IDE support

## Project Structure
```
cmd/dingo/          # CLI entry point
pkg/
├── ast/            # Code generators (*_codegen.go) - FROM AST ONLY
├── parser/         # Dingo parser (Pratt-based) - PRODUCES AST
├── tokenizer/      # Tokenizer - ONLY place reading raw bytes
├── goparser/       # Go parser wrapper + transforms
├── feature/        # Pluggable feature system
├── transpiler/     # Main pipeline
└── typechecker/    # go/types integration
tests/golden/       # Golden test files
examples/           # Example .dingo files
```

## Architecture

```
.dingo → tokenizer → parser → AST → *_codegen.go → .go file → gopls
```

**Key insight**: Dingo is syntax sugar, NOT a new type system. We use gopls for all type checking.

## Features (10 plugins in pkg/feature/builtin/)

| Feature | Priority | Syntax |
|---------|----------|--------|
| enum | 10 | `enum Name { Variant }` |
| match | 20 | `match expr { Pat => val }` |
| error_prop | 40 | `expr?` |
| tuples | 50 | `(a, b) := func()` |
| safe_nav | 60 | `x?.y` |
| null_coalesce | 70 | `a ?? b` |
| lambdas | 80 | `\|x\| expr` or `x => expr` |
| generics | 110 | Uses Go's native `[T]` syntax directly |

## Option/Result API (dgo package)

**Option[T]** methods:
- `.IsSome()` / `.IsNone()` - check state
- `.MustSome()` - extract value (panics if None)
- `.SomeOr(defaultVal)` - extract with default
- `.SomeOrElse(func() T)` - extract with lazy default

**Result[T, E]** methods:
- `.IsOk()` / `.IsErr()` - check state
- `.MustOk()` - extract value (panics if Err)
- `.MustErr()` - extract error (panics if Ok)
- `.OkOr(defaultVal)` - extract with default

**Constructors:**
- `Some(val)`, `None[T]()` - for Option
- `Ok[T, E](val)`, `Err[T, E](err)` - for Result

## Two Enum Patterns

1. **Generic types (dgo package):** `Option[T]`, `Result[T, E]`
   - Methods: `.IsSome()`, `.MustSome()`, `.IsOk()`, `.MustOk()`
   - Constructors: `Some(x)`, `None[T]()`, `Ok[T,E](x)`, `Err[T,E](e)`

2. **Interface-based enums:** `enum Option { Some(T), None }`
   - Generates Go interfaces + struct variants
   - Constructors: `NewOptionSome(x)`, `NewOptionNone()`
   - Use type switches: `switch v := opt.(type) { case OptionSome: ... }`

**Don't mix these patterns** - they have different APIs.

## Code Generation Standards

Variable naming:
- ✅ `tmp`, `tmp1`, `tmp2` (camelCase, no leading number)
- ❌ `__tmp0`, `_err_0` (no underscores, no zero-based)

## Test Policy

**NEVER exclude tests to hide bugs.** Fix the underlying issue instead.

- If a test is failing, fix the bug - don't exclude the test
- If an example doesn't compile, fix the transpiler - don't skip the example
- CI exclusions should only be temporary during active development
- Document any temporary exclusions with specific bug tracking
- **Features in `features/` directory have NO limitations** - all documented features must work

## Agent Selection

| Task | Agent |
|------|-------|
| Implementation | golang-developer |
| Architecture | golang-architect |
| Testing | golang-tester |
| Code review | code-reviewer |
| Codebase search | Explore |

**Landing page** (`landingpage/` dir): Use astro-* agents instead.

## Agent Skills

Project-specific skills are available in `.claude/skills/`:

| Skill | Description | When to Use |
|-------|-------------|-------------|
| `lsp-hover-testing` | Automated LSP hover validation | After sourcemap changes, debugging hover issues |

### LSP Hover Testing

Automated headless testing of hover functionality. Use instead of manual VS Code checks.

```bash
# Build required tools
go build -o dingo ./cmd/dingo
go build -o editors/vscode/server/bin/dingo-lsp ./cmd/dingo-lsp
go build -o lsp-hovercheck ./cmd/lsp-hovercheck

# Run hover tests
./lsp-hovercheck --spec "ai-docs/hover-specs/*.yaml"

# Verbose for debugging
./lsp-hovercheck --spec ai-docs/hover-specs/http_handler.yaml --verbose
```

**Test specs** are in `ai-docs/hover-specs/*.yaml`. See `.claude/skills/lsp-hover-testing/SKILL.md` for full documentation.

**Available test specs:**
- `http_handler.yaml` - Token-based tests for all error propagation patterns (16 tests)
- `column_precision.yaml` - Explicit character position tests validating column mapping (5 tests)
- `column_validation.yaml` - Basic character position validation (2 tests)

**LSP Position Specification:**
- LSP uses **0-indexed** positions for both `line` and `character`
- VS Code UI shows **1-indexed** columns (Col 15 in status bar = `character: 14` in LSP)
- Test specs use LSP's 0-indexed `character` field

**When to run hover tests:**
- After ANY changes to `pkg/lsp/`, `pkg/sourcemap/`, or `pkg/transpiler/`
- Before committing position tracking changes
- When debugging user-reported hover issues

## Sourcemap Architecture (v3)

Position tracking uses `token.Pos` from Dingo AST, NOT byte offsets.
The pipeline emits `//line file.dingo:line:col` directives so gopls
reports errors directly at Dingo positions.

Key files:
- `pkg/sourcemap/position_tracker.go` - New token.Pos-based tracker
- `pkg/sourcemap/dmap/format.go` - v3 format with column mappings
- `pkg/transpiler/pure_pipeline.go` - Main pipeline (see header comment)

## LSP Semantic Map Architecture

**Core Principle: Build entities from Dingo source, not Go position mapping.**

The semantic map provides hover/completion for Dingo. The correct architecture:

1. **Dingo positions from Dingo source** - Scan with go/scanner (CLAUDE.md compliant)
2. **Type info from Go type checker** - After transformation
3. **NO position mapping from Go → Dingo** - This breaks with transformations

Why Go→Dingo position mapping is fundamentally broken:
- Match expressions: 1 Dingo arm → 3+ Go lines (case + bindings)
- Error propagation: 1 Dingo line → 5 Go lines (if block)
- Enums: 1 Dingo enum → 50+ Go lines (interface + structs + methods)

**Anti-pattern** (what NOT to do):
```go
// WRONG: Try to compute Dingo line from Go line using offsets
dingoLine := goLine - regionOffset - cumulativeExpansion
// This is fundamentally fragile because transformations change line structure
```

**Correct pattern** (what TO do):
```go
// RIGHT: Scan Dingo source for identifiers, use Go types for enrichment
// See: detectEnumVariantOccurrences, detectOperators, detectLambdaParams
for {
    pos, tok, lit := scanner.Scan()
    if tok == token.IDENT && isKnownIdentifier(lit) {
        entities = append(entities, SemanticEntity{
            Line: fset.Position(pos).Line,  // Dingo position (always correct)
            Type: lookupGoType(lit),        // Go type info (for hover)
        })
    }
}
```

## Key Files

- Entry: `pkg/transpiler/pure_pipeline.go` → `PureASTTranspile()`
- Transform: `pkg/ast/transform.go` → `TransformSource()`
- Parser: `pkg/parser/` → Pratt-based expression parsing
- Features: `pkg/feature/builtin/plugins.go`

## Testing

- **Unit tests**: `go test ./...`
- **Golden tests**: `tests/golden/` - see `GOLDEN_TEST_GUIDELINES.md`
- **LSP hover tests**: `./lsp-hovercheck --spec "ai-docs/hover-specs/*.yaml"` (see Agent Skills section)

## CLI Commands

Dingo CLI mirrors Go's compiler structure:

| Command | Description | Go Equivalent |
|---------|-------------|---------------|
| `dingo build` | Transpile + compile to binary | `go build` |
| `dingo run` | Transpile + run immediately | `go run` |
| `dingo go` | Transpile to .go files only | N/A |

All `go build/run` flags are passed through (e.g., `-o`, `-race`, `-ldflags`).

**Dingo-specific flags:**
- `--verbose` - Show the go build/run command
- `--no-mascot` - Disable mascot animation (silent output)

Note: `dingo run` always runs in silent mode (no mascot) to give the running program full CLI access.

## Running Dingo in Claude Code

Always use `--no-mascot` flag when running dingo build in Claude Code terminal:
```bash
./dingo build --no-mascot examples/03_option/user_settings.dingo
```
This disables animation which doesn't render properly in Claude Code.

For `dingo run`, the mascot is automatically disabled (no flag needed):
```bash
./dingo run examples/03_option/user_settings.dingo
```

## Building Binaries

### ⚠️ CRITICAL: LSP Binary Location

**VS Code uses `dingo-lsp` from `$PATH`, NOT from `editors/vscode/server/bin/`!**

When rebuilding the LSP server, you MUST update the binary in `$PATH`:

```bash
# Build AND install to $PATH (REQUIRED for VS Code to use it)
go build -o /Users/jack/go/bin/dingo-lsp ./cmd/dingo-lsp

# Or build locally then copy
go build -o editors/vscode/server/bin/dingo-lsp ./cmd/dingo-lsp
cp editors/vscode/server/bin/dingo-lsp /Users/jack/go/bin/dingo-lsp
```

**After updating the binary, restart VS Code** to pick up the new version.

To verify VS Code is using the correct binary:
```bash
# Check which binary is in PATH
which dingo-lsp
md5 $(which dingo-lsp)

# Compare with local build
md5 editors/vscode/server/bin/dingo-lsp
```

### All Binaries

| Binary | Build Command | Install Location |
|--------|---------------|------------------|
| `dingo` | `go build -o dingo ./cmd/dingo` | Local or `$PATH` |
| `dingo-lsp` | `go build -o /Users/jack/go/bin/dingo-lsp ./cmd/dingo-lsp` | **Must be in `$PATH`** |
| `lsp-hovercheck` | `go build -o lsp-hovercheck ./cmd/lsp-hovercheck` | Local (test tool) |

### Version Management

All Dingo binaries share the same version defined in `pkg/version/version.go`:

```go
const Version = "0.4.1"
```

When releasing a new version, update this single file. Both `dingo` and `dingo-lsp` will use it:
- `dingo version` - shows version
- `dingo-lsp --version` - shows version

## Editor Plugins

| Editor | Repository | Installation |
|--------|------------|--------------|
| **Neovim** | [MadAppGang/dingo.nvim](https://github.com/MadAppGang/dingo.nvim) | `{ "MadAppGang/dingo.nvim" }` |
| **VS Code** | `editors/vscode/` (this repo) | See `editors/vscode/INSTALL.md` |
| **GoLand** | `editors/goland/` (this repo) | See `editors/goland/README.md` |

**Note**: The Neovim plugin lives in a **separate repository** at `github.com/MadAppGang/dingo.nvim`.
Local development path: `/Users/jack/mag/dingo.nvim`

## References

- Research: `ai-docs/claude-research.md`, `ai-docs/gemini_research.md`
- Architecture: `ai-docs/dingo-vs-borgo.md`

## Learned Preferences

### Tools & Commands
<!-- learned: 2026-03-28 session: 9f136fc8 source: repeated_pattern -->
- Use `claudish` CLI for external model invocation and multi-model review sessions
<!-- learned: 2026-03-28 session: 83851fd3 source: repeated_pattern -->
- Use `CI=true` prefix when running Go tests to suppress interactive prompts

### Project Structure
<!-- learned: 2026-03-28 session: 9f136fc8 source: explicit_rule -->
- Session artifacts go in `ai-docs/sessions/{command}-{YYYYMMDD}-{HHMMSS}-{hash}/`

---
**Last Updated**: 2026-03-28
