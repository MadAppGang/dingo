# Claude AI Agent Memory & Instructions

This file contains instructions and context for Claude AI agents working on the Dingo project.

## Project Structure Rules

### Root Directory (Minimal)
The root folder should **ONLY** contain:
- `README.md` - Main project documentation (user-facing)
- `CLAUDE.md` - This file: AI agent memory and instructions
- Standard project files: `go.mod`, `go.sum`, `.gitignore`, `LICENSE`, etc.
- Source code directories: `cmd/`, `internal/`, `pkg/`, etc.

**DO NOT create additional documentation files in the root!**

### AI Documentation (`ai-docs/`)
All AI-related research, context, and working documents go here:
- `claude-research.md` - Comprehensive implementation guide
- `gemini_research.md` - Technical blueprint and analysis
- Any future AI-generated research, design docs, or context files

**Purpose**: These files help AI agents understand the project context, architecture decisions, and current stage. They are NOT user-facing documentation.

### Other Documentation
- User-facing documentation goes in `docs/` (when created)
- API documentation, tutorials, examples go in appropriate subdirectories
- Keep root clean and minimal

## Project Context

### What is Dingo?
A meta-language for Go (like TypeScript for JavaScript) that:
- Transpiles `.dingo` files to idiomatic `.go` files
- Provides Result/Option types, pattern matching, and error propagation
- Maintains 100% Go ecosystem compatibility
- Offers full IDE support via gopls-wrapping language server

**Official Website**: https://dingolang.com (landing page domain)

### Architecture (Two Components)

1. **Transpiler** (`dingo build`)
   - Parses `.dingo` → AST
   - Transforms to Go AST
   - Generates `.go` + `.sourcemap` files
   - Tools: `participle`/Tree-sitter, `go/ast`, `go/printer`

2. **Language Server** (`dingo-lsp`)
   - Wraps gopls as proxy
   - Translates LSP requests using source maps
   - Provides IDE features (autocomplete, navigation, diagnostics)
   - Tools: `go.lsp.dev/protocol`, gopls subprocess

### Current Stage

**Phase 0: Research & Design** ✅ Complete

Completed comprehensive research on:
- Meta-language patterns (TypeScript, Borgo, templ)
- Go AST manipulation
- Language server proxy architecture
- Source mapping strategies

**Next: Phase 1 - Core Transpiler** (Starting Now)

Goals:
1. Define minimal Dingo syntax
2. Build parser (using `participle`)
3. Implement AST → Go transformation
4. Generate source maps
5. Create CLI tool
6. Achieve end-to-end: `.dingo` → `.go` → execution

### Key Research Findings

See `ai-docs/claude-research.md` and `ai-docs/gemini_research.md` for details:

- **Proven precedents**: Borgo (Go transpiler), templ (gopls proxy), TypeScript (architecture)
- **Critical technology**: Source maps for bidirectional position mapping
- **Recommended tools**:
  - Parser: `alecthomas/participle` → Tree-sitter
  - AST: `go/ast`, `golang.org/x/tools/go/ast/astutil`
  - LSP: `go.lsp.dev/protocol`
- **Timeline**: 12-15 months to v1.0

### Design Principles

1. **Zero Runtime Overhead**: Generate clean Go code, no runtime library
2. **Full Compatibility**: Interoperate with all Go packages and tools
3. **IDE-First**: Maintain gopls feature parity
4. **Simplicity**: Only add features that solve real pain points
5. **Readable Output**: Generated Go should look hand-written

### Planned Features (Priority Order)

1. `Result<T, E>` type (replaces `(T, error)`)
2. `?` operator for error propagation
3. `Option<T>` type (replaces nil checks)
4. Pattern matching (`match` expression)
5. Sum types (`enum` keyword)
6. Automatic Go interop (wrap `(T, error)` → `Result<T, E>`)

## Instructions for AI Agents

### When Adding Context/Research
- Save to `ai-docs/` directory
- Use descriptive filenames: `ai-docs/architecture-decisions.md`, `ai-docs/parser-research.md`
- Update this CLAUDE.md if adding important context

### When Creating Documentation
- **User-facing docs**: → `docs/` directory (when it exists)
- **AI context/research**: → `ai-docs/` directory
- **Root files**: Only README.md and CLAUDE.md
- **Never** create standalone docs in root

### When Implementing Code
- Follow the research recommendations in `claude-research.md` and `gemini_research.md`
- Start with minimal viable features (Result, ?, basic transpilation)
- Prioritize end-to-end functionality over completeness
- Generate idiomatic, readable Go code

### Agent Usage Guidelines

**CRITICAL**: This project has TWO separate development areas with different agents:

#### 1. **Dingo Transpiler/Language** (This Directory)
**Working Directory**: `/Users/jack/mag/dingo/` (root)
**Code**: `cmd/`, `pkg/`, `internal/`, `tests/golden/`
**Language**: Go

**Use these agents**:
- ✅ `golang-developer` - Implementation (transpiler, parser, AST, language features)
- ✅ `golang-architect` - Architecture and design
- ✅ `golang-tester` - Testing and golden tests
- ✅ `code-reviewer` - Code review

**Slash commands**:
- ✅ `/dev` - Development orchestrator for Dingo language

#### 2. **Landing Page** (Separate Directory)
**Working Directory**: `/Users/jack/mag/dingo/langingpage/`
**Code**: `src/`, Astro components, React components
**Language**: TypeScript, Astro, React

**Use these agents**:
- ✅ `astro-developer` - Implementation (landing page, components, styling)
- ✅ `astro-reviewer` - Code review and visual validation
- ⚠️ `code-reviewer` - Can review, but astro-reviewer preferred

**Slash commands**:
- ✅ `/astro-dev` - Development orchestrator for landing page
- ✅ `/astro-fix` - Visual fix orchestrator for landing page

#### ❌ **NEVER Mix Agents**

**WRONG Examples** (DO NOT DO THIS):
- ❌ Using `golang-developer` for Astro/landing page work
- ❌ Using `astro-developer` for transpiler/Go work
- ❌ Using `/dev` in `langingpage/` directory
- ❌ Using `/astro-dev` in root directory

**Correct Examples**:
- ✅ Root directory → Go work → `golang-developer`, `golang-architect`, `golang-tester`, `/dev`
- ✅ `langingpage/` directory → Astro work → `astro-developer`, `astro-reviewer`, `/astro-dev`, `/astro-fix`

#### Quick Decision Guide

**If working on**:
- Parser, AST, transpiler, language features → Use golang-* agents
- Landing page, components, styling, UI → Use astro-* agents
- In doubt? Check your working directory:
  - Root (`/Users/jack/mag/dingo/`) → golang-* agents
  - Langingpage (`/Users/jack/mag/dingo/langingpage/`) → astro-* agents

### Phase 1 Implementation Checklist

```
[ ] Define Dingo syntax spec (EBNF or struct tags)
[ ] Set up Go module structure
[ ] Implement parser with participle
[ ] Build AST → Go AST transformer
[ ] Implement source map generation
[ ] Create CLI tool (cmd/dingo)
[ ] Write golden file tests
[ ] Achieve: dingo build file.dingo → file.go → go run file.go
```

## Important References

### Research Documents
- `ai-docs/claude-research.md` - Comprehensive guide: tooling, architecture, TypeScript lessons
- `ai-docs/gemini_research.md` - Technical blueprint: transpiler, LSP proxy, implementation roadmap

### Key External Projects
- **Borgo** (github.com/borgo-lang/borgo) - Rust-like syntax → Go transpiler
- **templ** (github.com/a-h/templ) - gopls proxy architecture reference
- **TypeScript** - Meta-language architecture gold standard

### Essential Go Tools
- `go/ast`, `go/parser`, `go/printer` - Standard library AST
- `golang.org/x/tools/go/ast/astutil` - AST manipulation
- `go.lsp.dev/protocol` - LSP implementation
- `alecthomas/participle` - Parser generator (recommended for prototyping)

## Current Priorities

1. **Finalize syntax**: Define minimal Dingo syntax for Phase 1
2. **Project setup**: Initialize Go modules, directory structure, CI
3. **Parser prototype**: Get basic `.dingo` parsing working
4. **Simple transpiler**: Result type → (T, error) transformation
5. **End-to-end test**: Write → transpile → compile → run

## Questions to Resolve

- [ ] Final decision: `participle` vs Tree-sitter for initial parser?
- [ ] Syntax: Rust-like, Go-like, or hybrid?
- [ ] Source map format: JSON, binary, or custom?
- [ ] Monorepo vs separate repos for transpiler/LSP?
- [ ] License choice?

---

**Last Updated**: 2025-11-17
**Current Phase**: Phase 2.7 Complete (Functional Utilities)
**Next Milestone**: Phase 3 - Result/Option Integration

### Additional Project Information

- All feature proposals are located in `features/` folder (split per file, e.g., `features/lambdas.md`)
- No backward compatibility needed (pre-release), keep everything simple and clean
- Do not write progress files - update `CHANGELOG.md` instead
- Official domain: **dingolang.com** (landing page)

### Golden Test Guidelines

**IMPORTANT**: When writing or modifying golden tests in `tests/golden/`, you MUST follow the comprehensive guidelines in:

📖 **`tests/golden/GOLDEN_TEST_GUIDELINES.md`**

**Quick Rules:**
- **Naming:** `{feature}_{NN}_{description}.dingo` (e.g., `error_prop_01_simple.dingo`)
- **Feature prefixes:** `error_prop_`, `result_`, `option_`, `sum_types_`, `lambda_`, `ternary_`, `null_coalesce_`, `safe_nav_`, `pattern_match_`, `tuples_`, `func_util_`, `immutable_`
- **Files required:** Both `.dingo` and `.go.golden` for each test
- **Content:** Realistic examples, 10-50 lines, one feature per test
- **Quality:** Idiomatic Go output, compilable, no external deps
- **Progression:** 01=basic, 02-03=intermediate, 04+=advanced

See the full guidelines document for detailed rules, examples, and best practices.

### Golden Test Documentation Structure

**Main Documentation** (`tests/golden/README.md`):
- Quick start guide for running tests
- Complete test catalog (46 tests across 11 categories)
- Naming conventions and file structure
- Writing new tests workflow
- **Reasoning Documentation Section** - Explains the "why" behind tests:
  - Links to Go community proposals and discussions
  - Design rationale for implementation choices
  - External references (Rust, Swift, TypeScript, Kotlin)
  - Code reduction metrics (sum types: 78-79%, error propagation: 65%, etc.)
  - Go Proposal Reference Map with community voting data

**Individual Reasoning Files** (`.reasoning.md`):
Each test can have an optional reasoning file (e.g., `sum_types_01_simple_enum.reasoning.md`) that provides:
- Community context and Go proposals
- Design decisions and alternatives
- Implementation highlights
- Success metrics and future enhancements

**Completed Reasoning Docs:**
- `sum_types_01_simple_enum.reasoning.md` - Basic enum (79% code reduction, Go Proposal #19412)
- `sum_types_02_struct_variant.reasoning.md` - Enum with data (78% code reduction)
- `01_simple_statement.reasoning.md` - Error propagation suite (covers all 8 tests, Go Proposal #71203)

**Key Community References:**
- Go Proposal #19412 - Sum types (996+ 👍, highest-voted proposal ever)
- Go Proposal #71203 - Error `?` operator (Active 2025, 200+ comments)
- Go Proposal #21498 - Lambda functions (750+ 👍)

See `tests/golden/README.md` for the complete reasoning documentation index and metrics.