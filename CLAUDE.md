# Claude AI Agent Memory & Instructions

This file contains instructions and context for Claude AI agents working on the Dingo project.

## ⚠️ CRITICAL: Token Budget Enforcement (READ FIRST)

**EVERY action must pass this pre-check:**

### Token Budget Limits (HARD LIMITS)

| Operation | Limit | Violation Remedy |
|-----------|-------|------------------|
| File reads per message | 2 files OR 200 lines total | Delegate to agent |
| Bash output | 50 lines | Use `head -50` OR delegate |
| Grep results | 20 matches | Use `head_limit: 20` OR delegate |
| Agent response summary | 5 sentences max | Agent MUST compress |

**IF ANY LIMIT EXCEEDED → MUST delegate to agent instead**

### Pre-Check Decision Tree

```
┌─────────────────────────────────────────┐
│ Before EVERY action, ask:               │
└─────────────────────────────────────────┘
                   ↓
    [User wants multiple model perspectives?]
         ↓ YES
    [Create session folder]
         ↓
    [Write investigation prompt to file]
         ↓
    [Launch specialized agents in PARALLEL]
    (golang-architect for Go, etc.)
         ↓
    [Each agent invokes ONE external model via claudish]
         ↓
    [Results → files, Summaries → main chat (< 5 sentences)]
         ↓
    [Optional: Consolidation agent synthesizes]
                   ↓ NO
    [Will this exceed token budget?]
         /
       YES         NO
        │           │
        │           ↓
        │    [Is it multi-step task?]
        │         /
        │       YES         NO
        │        │           │
        │        │           ↓
        │        │    Execute directly
        │        │    (simple query/file op)
        │        │
        ↓        ↓
   Use Task tool (delegate to agent)
        │
        ↓
   Read ONLY summary (< 100 lines)
```

### Forbidden Patterns in Main Chat

**❌ NEVER DO THESE:**

1. **Reading Multiple Code Files**
   - ❌ Read 3+ files in one conversation turn
   - ✅ Delegate to agent → Read summary only

2. **Implementing Code**
   - ❌ Edit multiple files directly
   - ✅ Delegate to golang-developer → Read summary

3. **Running Tests**
   - ❌ Show full test output (>50 lines)
   - ✅ Delegate to golang-tester → Read summary

4. **Searching Codebase**
   - ❌ Multiple Grep calls, reading results
   - ✅ Delegate to Explore agent → Read summary

### Mandatory Pattern: Session Folders

For ANY multi-step task:

```bash
# Create session immediately
SESSION=$(date +%Y%m%d-%H%M%S)
mkdir -p ai-docs/sessions/$SESSION/{input,output}

# Write user request
echo "Request: ..." > ai-docs/sessions/$SESSION/input/request.md

# Delegate with file paths
Task → agent:
  Input: ai-docs/sessions/$SESSION/input/request.md
  Output: ai-docs/sessions/$SESSION/output/summary.txt

# Main chat reads ONLY summary
```

**Main chat NEVER reads detail files (unless presenting to user).**

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

### Critical Value Proposition: Dual Benefit (Personal + Collective)

**THE MOST IMPORTANT THING ABOUT DINGO:**

Dingo delivers **TWO revolutionary benefits simultaneously**:

**1. Immediate Personal Value (Why developers actually use it):**
- 67% less error handling boilerplate with `?` operator
- 78% code reduction with sum types/enums
- Zero nil pointer panics with Option types
- Same performance (transpiles to clean Go)
- Better code TODAY, zero waiting for proposals

**2. Collective Future Value (Automatic side effect):**
- Your usage generates real-world metrics
- Your bugs find edge cases theoretical debates miss
- Your production code validates ideas
- Go team gets evidence-based data for decisions

**This is EXACTLY what TypeScript did for JavaScript:**
- Developers adopted TypeScript selfishly (better codebases)
- Millions used features like async/await, optional chaining
- JavaScript saw proof it worked and adopted features
- Timeline: TypeScript feature → 1-2 years usage → JavaScript adoption

**Examples:**
- Async/await: TS 2015 → Millions used it → JS ES2017 (2 years)
- Optional chaining: TS 2019 → Widespread adoption → JS ES2020 (1 year)
- Nullish coalescing: TS 2019 → Standard in TS → JS ES2020 (1 year)

**Dingo enables the same for Go:**
- You use Dingo to make YOUR code better (selfish reason)
- 50,000 other developers do the same
- Go team sees 2 years of production validation
- Go proposals now have concrete evidence

**Perfect incentive alignment:**
- Developers: Better code today, zero waiting
- Go team: Real data for decisions, reduced risk
- Ecosystem: Faster evolution, battle-tested features

**When working on Dingo, remember:**
- Primary goal: Make developers' code better IMMEDIATELY
- Secondary effect: Generate data that could reshape Go's future
- Every feature should provide measurable value (track metrics!)
- We're not competing with Go—we're accelerating its evolution through real-world experimentation
- Emphasize: "Use Dingo selfishly. Help Go evolve as a bonus."

### Architecture (Two Components)

1. **Transpiler** (`dingo build`) - Two-Stage Approach
   - **Stage 1: Preprocessor** - Text-based transformation (Dingo syntax → valid Go)
     - TypeAnnotProcessor: `param: Type` → `param Type`
     - ErrorPropProcessor: `x?` → error handling code
     - EnumProcessor: `enum Name {}` → Go tagged unions
     - KeywordProcessor: Other Dingo keywords
   - **Stage 2: AST Processing** - Parse and transform
     - Uses native `go/parser` to parse preprocessed Go code
     - Plugin pipeline transforms AST (Result types, etc.)
     - Generates `.go` + `.sourcemap` files
   - Tools: Regex-based preprocessors, `go/parser`, `go/ast`, `go/printer`

2. **Language Server** (`dingo-lsp`)
   - Wraps gopls as proxy
   - Translates LSP requests using source maps
   - Provides IDE features (autocomplete, navigation, diagnostics)
   - Tools: `go.lsp.dev/protocol`, gopls subprocess

### Current Stage

**Phase VI: P0 Feature Completion** ✅ Complete (2025-11-20)

**Status: v1.0-BETA READY**

Dingo has completed critical P0 feature implementation sprint with 92.5% test passing rate (up from 51%), 100% compilation rate, and 5/6 P0 features production-ready. The transpiler generates valid Go code across all test scenarios.

**Key Features Implemented:**
- Two-stage transpilation (preprocessor + go/parser)
- Result<T,E> and Option<T> types with full helper methods (Map, AndThen)
- Error propagation (`?` operator) - 100% test coverage
- Lambda expressions (TypeScript & Rust syntax) - 100% test coverage
- Pattern matching with guards and tuple patterns - 92% test coverage
- Sum types/enums with exhaustiveness checking
- Null coalescing (`??`) - implementation complete, parser refinement needed
- Multi-package workspace builds
- Comprehensive developer documentation

**Quality Metrics:**
- 3/4 external model approval for v1.0 (Grok 4 Fast, Gemini 3 Pro, GPT-5, Claude Opus 4)
- Average scores: 8.9/10 Quality, 8.9/10 Completeness, 8.1/10 Production Readiness
- **92.5% test passing rate (124/134 tests)** ⬆️ up from 51%
- **100% compilation rate** - all generated Go code compiles
- **5/6 P0 features at 90%+** - Error prop, Lambdas, Option, Result, Pattern matching

**Recent Session (2025-11-20):**
- Fixed 6 critical P0 bugs (tuple naming, null coalesce comments, helper methods, IIFE indentation, etc.)
- Regenerated 17 golden files with new helper methods
- Improved pass rate by 41.5 percentage points (51% → 92.5%)
- Commit: `9cf49e3` - feat(p0): Complete P0 feature implementation sprint

See `ai-docs/sessions/20251120-p0-final/` for detailed session report and `CHANGELOG.md` for complete project history.

### Key Research Findings

See `ai-docs/claude-research.md` and `ai-docs/gemini_research.md` for details:

- **Proven precedents**: Borgo (Go transpiler), templ (gopls proxy), TypeScript (architecture)
- **Critical technology**: Source maps for bidirectional position mapping
- **Actual Implementation** (as of Phase 2.16):
  - **Preprocessor**: Regex-based text transformations (Dingo → valid Go)
  - **Parser**: Native `go/parser` (standard library)
  - **AST**: `go/ast`, `golang.org/x/tools/go/ast/astutil`
  - **Plugins**: Interface-based AST transformation pipeline
  - **LSP**: `go.lsp.dev/protocol` (future)
- **Timeline**: 12-15 months to v1.0

### Design Principles

1. **Zero Runtime Overhead**: Generate clean Go code, no runtime library
2. **Full Compatibility**: Interoperate with all Go packages and tools
3. **IDE-First**: Maintain gopls feature parity
4. **Simplicity**: Only add features that solve real pain points
5. **Readable Output**: Generated Go should look hand-written

### Code Generation Standards

**CRITICAL: Variable Naming Convention (Enforced 2025-11-20)**

All code generators MUST follow these naming rules:

1. **No Underscores - Use camelCase**
   - ✅ Correct: `tmp`, `tmp1`, `err`, `err1`, `coalesce`
   - ❌ Wrong: `__tmp0`, `__err0`, `__coalesce0`

2. **No-Number-First Pattern**
   - ✅ Correct: First `tmp`, then `tmp1`, `tmp2`
   - ✅ Correct: First `err`, then `err1`, `err2`
   - ❌ Wrong: `tmp1`, `tmp2`, `tmp3` (all numbered)
   - ❌ Wrong: `tmp0`, `tmp1`, `tmp2` (zero-based)

3. **Counter Initialization**
   - ✅ Correct: `counter = 1` or `counter := 1`
   - ❌ Wrong: `counter = 0` or `counter := 0`

**Affected Components:**
- `pkg/preprocessor/error_prop.go` - Error propagation (`tmp`, `err` → `tmp1`, `err1`)
- `pkg/preprocessor/null_coalesce.go` - Null coalescing (`coalesce` → `coalesce1`)
- `pkg/preprocessor/safe_nav.go` - Safe navigation (`user` → `user1`, `__user_tmp` → `__user_tmp1`)
- `pkg/plugin/plugin.go` - Plugin temp vars (`tmp` → `tmp1`)

**Rationale:**
- Go convention: camelCase for local variables
- Readability: No visual clutter from underscores
- Consistency: All generators follow same pattern
- Human-like: Generated code looks hand-written

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

### Common Delegation Patterns (Skills)

For complex delegation workflows, use these **skills** (detailed instructions loaded only when invoked):

**1. Multi-Model Consultation** → Use skill `multi-model-consult`
- **When**: Need perspectives from multiple LLMs (gpt-5, gemini, grok, etc.)
- **Triggers**: "run multiple models", "get perspectives from different models"
- **How**: Skill orchestrates parallel external model consultation via claudish
- **Result**: 2-3x faster, 10x less context, diverse expert opinions

**2. Deep Investigation** → Use skill `investigate`
- **When**: Need to understand how codebase works
- **Triggers**: "how does X work?", "find all usages of Y"
- **How**: Skill delegates to appropriate agent (Explore, golang-developer, etc.)
- **Result**: 10-20x less context, file paths with line numbers

**3. Feature Implementation** → Use skill `implement`
- **When**: Multi-file feature implementation needed
- **Triggers**: "implement feature X", "add support for Y"
- **How**: Skill orchestrates planning → implementation → testing
- **Result**: Structured workflow, parallel execution, tracked progress

**4. Testing** → Use skill `test`
- **When**: Run tests, create tests, fix failing tests
- **Triggers**: "run tests", "create golden tests", "fix failing tests"
- **How**: Skill delegates to golang-tester with appropriate scope
- **Result**: Pass/fail summary, detailed results in files

**Why Skills?**
- **Context Economy**: Detailed patterns loaded ONLY when needed
- **Consistency**: Standardized execution across all delegation tasks
- **Maintainability**: Update patterns in one place, all uses benefit

## 🚨 MANDATORY DELEGATION POLICY

**CRITICAL RULE: Main chat is STRICTLY PROHIBITED from doing detailed work. ALL multi-step tasks, code analysis, implementation, and testing MUST be delegated to specialized agents.**

### What Main Chat CAN Do (Orchestration Only)

✅ **ALLOWED** - High-level orchestration:
- User interaction (questions, approvals, presenting summaries)
- Single git status check
- Single file read for user presentation (NOT for analysis)
- Launching agents via Task tool or Skills
- Coordinating workflow and deciding next steps

❌ **FORBIDDEN** - Any detailed work:
- Reading multiple files (>2 files OR >200 lines total)
- Implementing code or editing files
- Running tests or analyzing output
- Searching codebase (multiple Grep calls)
- Deep analysis or investigation
- Writing detailed documentation

### Mandatory Delegation Triggers

**IF any of these conditions are true → MUST delegate immediately:**

| Condition | Delegate To |
|-----------|-------------|
| Reading 3+ files | Explore or golang-developer agent |
| Implementing any code | golang-developer agent |
| Running tests | golang-tester agent |
| Analyzing architecture | golang-architect agent |
| Code review | code-reviewer agent |
| Multi-step task (>3 steps) | Appropriate specialized agent |
| Codebase investigation | Explore agent (via Skill or Task) |

### Quick Reference: Agent Selection

- **Investigation/Search** → Explore agent (fast, optimized for codebase exploration)
- **Implementation** → golang-developer agent
- **Testing** → golang-tester agent
- **Architecture/Design** → golang-architect agent
- **Code Review** → code-reviewer agent
- **Multi-model consultation** → Use `claudish-usage` skill

### Response Format: Agents Return Summaries Only

Agents MUST return **2-5 sentence summaries** in this format:

```
# [Task Name] Complete

Status: [Success/Partial/Failed]
Key Finding: [One-liner]
Changed: [N] files
Details: [file-path]
```

**Detailed work ALWAYS goes to files. Main chat reads ONLY summaries.**

### Parallel Execution

When tasks are independent, launch agents in **parallel** (single message with multiple Task tool calls):

```
✅ CORRECT: Single message with 3 Task tool calls
❌ WRONG: 3 separate messages with 1 Task tool call each
```

**Benefits**: 2-3x faster execution, all summaries return together.

### Architecture: Three-Layer Pattern

```
Main Chat → Orchestrates, delegates, presents to user
    ↓
Agents → Investigate, implement, analyze (write to files)
    ↓
Files → Detailed results, code, analysis (persistent storage)
```

**Main chat never stores detailed data in conversation context.**

### Full Detailed Guide

For complete delegation patterns, templates, examples, and anti-recursion rules, see:

📖 **`ai-docs/delegation-strategy.md`**

**Key sections in detailed guide:**
- Communication protocols
- Agent self-awareness rules (anti-recursion)
- File-based communication patterns
- Session folder structure
- Complete workflow examples
- Context savings metrics (23x reduction)

## Parallel Multi-Model Review Protocol

When user requests multiple code reviewers (internal + external models):

### Execution Pattern (One-Shot)

**User Request**: "Run internal and external reviewers (grok, minimax, codex, gemini)"

**Required Pattern**:

```
Message 1 (Preparation):
  - Create directories ONLY (Bash mkdir)
  - NO other operations

Message 2 (Parallel Execution):
  - Launch ALL reviewers in SINGLE message
  - ONLY Task tool calls (no Bash, no TodoWrite)
  - Each Task call is independent

Message 3 (Automatic Consolidation):
  - DO NOT wait for user to request consolidation
  - Automatically launch consolidation agent
  - Pass all review file paths

Message 4 (Results):
  - Present consolidated review to user
```

### State Machine

```
PREP → PARALLEL_REVIEW → AUTO_CONSOLIDATE → PRESENT
         ↑ Single message      ↑ Automatic (no user prompt)
```

### Critical Rules

**DO**:
- Separate directory creation from parallel execution
- Use only Task tool in parallel execution message
- Auto-consolidate after N reviews (N ≥ 2)
- Present consolidated results

**DON'T**:
- Mix Bash and Task in same message
- Wait for user to request consolidation
- Launch reviewers sequentially
- Include TodoWrite in parallel execution message

### Example: Correct One-Shot Execution

```
User: "Run internal and 4 external reviewers in parallel"

Assistant Message 1:
  [Bash] mkdir -p ai-docs/sessions/XXX/reviews

Assistant Message 2:
  [Task] Internal review → summary
  [Task] Grok review → summary
  [Task] MiniMax review → summary
  [Task] Codex review → summary
  [Task] Gemini review → summary

Assistant Message 3 (AUTOMATIC - no user prompt):
  [Task] Consolidate reviews → summary

Assistant Message 4:
  "Consolidated review complete: 5 reviewers analyzed..."
```

### Proxy Mode for External Models

When code-reviewer agent uses external models via claudish:

**Required**: Blocking execution
```bash
# CORRECT (blocking):
REVIEW=$(claudish --model openai/gpt-5.1-codex <<'EOF'
Review prompt...
EOF
)

# Write to file
echo "$REVIEW" > review.md

# Return summary (2-5 sentences)
```

**NEVER**: Background execution
```bash
# WRONG (returns too early):
claudish --model ... &
```

---

### Implementation Architecture (Actual)

**Two-Stage Transpilation Pipeline**:

```
.dingo file
    ↓
┌─────────────────────────────────────┐
│ Stage 1: Preprocessor (Text-based) │
├─────────────────────────────────────┤
│ • TypeAnnotProcessor                │  param: Type → param Type
│ • ErrorPropProcessor                │  x? → if err != nil...
│ • EnumProcessor                     │  enum Name {} → structs
│ • KeywordProcessor                  │  Other Dingo keywords
└─────────────────────────────────────┘
    ↓ (Valid Go syntax)
┌─────────────────────────────────────┐
│ Stage 2: AST Processing             │
├─────────────────────────────────────┤
│ • go/parser (native)                │  Parse to AST
│ • Plugin Pipeline:                  │
│   - Discovery phase                 │  Find Ok/Err calls
│   - Transform phase                 │  Rewrite AST nodes
│   - Inject phase                    │  Add type declarations
└─────────────────────────────────────┘
    ↓
.go file + .sourcemap
```

**Why This Approach?**
- Preprocessors transform Dingo syntax (not valid Go) to valid Go
- Then go/parser handles all parsing (no custom parser needed)
- Plugins transform AST for features that need Go semantics
- Simpler, leverages Go's own parser, easier to maintain

## Important References

### Research Documents
- `ai-docs/claude-research.md` - Comprehensive guide: tooling, architecture, TypeScript lessons
- `ai-docs/gemini_research.md` - Technical blueprint: transpiler, LSP proxy, implementation roadmap

### Key External Projects
- **Borgo** (github.com/borgo-lang/borgo) - Rust-like syntax → Go transpiler
- **templ** (github.com/a-h/templ) - gopls proxy architecture reference
- **TypeScript** - Meta-language architecture gold standard

### Essential Go Tools (Actually Used)
- `go/parser` - Native Go parser for preprocessed code
- `go/ast`, `go/printer` - Standard library AST manipulation
- `golang.org/x/tools/go/ast/astutil` - Advanced AST utilities
- `regexp` - Preprocessor pattern matching
- `go.lsp.dev/protocol` - LSP implementation (future)

## Current Status (Phase 3 Complete)

✅ **Completed**:
1. Two-stage architecture (preprocessor + go/parser)
2. Enum syntax support (`enum Name { Variant }`)
3. Plugin pipeline (Discovery → Transform → Inject)
4. Result<T,E> complete with 13 helper methods
5. Option<T> complete with 13 helper methods
6. Fix A5: go/types type inference (>90% accuracy)
7. Fix A4: IIFE pattern for literals (Ok(42), Some("hello"))
8. Error infrastructure with compile-time reporting
9. Comprehensive test suite (261/267 passing, 97.8%)

🎯 **Next (Phase 4)**:
1. Pattern matching implementation
2. Full go/types context integration (AST parent tracking)
3. None constant context inference
4. Enhanced error messages with suggestions

## Architecture Decisions (Resolved)

✅ **Parser Approach**: Two-stage (Preprocessor + go/parser)
  - Preprocessors handle Dingo-specific syntax via regex
  - Native go/parser handles standard Go parsing
  - Avoids need for custom parser generators

✅ **Syntax Style**: Rust-like with Go compatibility
  - `enum Name { Variant }` for sum types
  - `Result<T,E>`, `Option<T>` generic types
  - `?` operator for error propagation

⏳ **To Resolve**:
- [ ] Source map format: JSON, binary, or custom?
- [ ] Monorepo vs separate repos for transpiler/LSP?
- [ ] License choice?

---

**Last Updated**: 2025-11-20 (Naming Convention Standardization - All naming converted to Go camelCase)
**Recent Changes**:
- 2025-11-20: Naming Convention Standardization (underscore → camelCase, 4/4 reviewers approved)
- 2025-11-19: Phase V Complete - Infrastructure & Developer Experience (3/4 external model approval)
**Previous Phase**: Phase 4.2 Complete - Pattern Matching Enhancements
**Latest Session**: 20251120-120544 (Naming Convention Fix)
**Previous Session**: 20251119-150114 (Phase V)

### Additional Project Information

- All feature proposals are located in `features/` folder (split per file, e.g., `features/lambdas.md`)
- No backward compatibility needed (pre-release), keep everything simple and clean
- Do not write progress files - update `CHANGELOG.md` instead
- Official domain: **dingolang.com** (landing page)

### Golden Test Guidelines

**IMPORTANT**: When writing or modifying golden tests in `tests/golden/`, follow the comprehensive guidelines in `tests/golden/GOLDEN_TEST_GUIDELINES.md` and `tests/golden/README.md`.

The showcase example `tests/golden/showcase_01_api_server.dingo` is the flagship demo that must be updated whenever new features are implemented.