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

**Phase 0: Research & Design** ✅ Complete

Completed comprehensive research on:
- Meta-language patterns (TypeScript, Borgo, templ)
- Go AST manipulation
- Language server proxy architecture
- Source mapping strategies

**Phase 3: Fix A4/A5 + Complete Result/Option** ✅ Complete

Implemented:
1. Two-stage transpilation: Preprocessor + go/parser ✅
2. Enum preprocessor for sum types ✅
3. Plugin pipeline for AST transformations ✅
4. Result<T,E> type with complete helper methods ✅
5. Option<T> type with complete helper methods ✅
6. Fix A5: go/types integration for type inference (>90% accuracy) ✅
7. Fix A4: IIFE pattern for literal handling (Ok(42), Some("hello")) ✅
8. Comprehensive test suite (261/267 passing, 97.8%) ✅
9. End-to-end: `.dingo` → preprocessor → `.go` → compile ✅

**Phase 4.1: Pattern Matching + None Inference** ✅ Complete (2025-11-18)

Implemented:
1. Configuration system (dingo.toml) for pattern matching syntax ✅
2. AST parent tracking for context-aware inference (<10ms) ✅
3. Rust pattern match syntax (`match result { Ok(x) => ... }`) ✅
4. Strict exhaustiveness checking (compile-time errors) ✅
5. Pattern transformation with tag-based dispatch ✅
6. None context inference (5 context types) ✅
7. 57/57 Phase 4 tests passing, 9 critical fixes applied ✅

**Current: Phase 4.2 - Pattern Matching Enhancements** 🚧 In Progress

Objectives:
1. Pattern guards (`pattern if condition => expr`)
2. Swift pattern syntax (`switch { case .Variant(let x): }`)
3. Tuple destructuring (`(pattern1, pattern2)`)
4. Enhanced error messages (rustc-style source snippets)

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

## 🎯 Delegation Strategy & Context Economy

### Core Principle: Main Chat = Orchestration Only

The main conversation context should be **minimal and lean**, containing ONLY:
- High-level decisions and next steps
- Brief agent summaries (2-5 sentences max)
- User interactions and approvals
- File paths (NOT file contents)

**All detailed work happens in agent threads and files.**

### The Problem We're Solving

**BAD (Context Bloat)**:
```
Main Chat:
User: "Implement feature X"
Assistant: [Reads 5 files, 500 lines total]
Assistant: [Pastes full implementation plan, 200 lines]
Assistant: [Shows full code review, 150 lines]
Assistant: [Displays all test results, 100 lines]
Result: 950 lines in main context, can't remember earlier decisions
```

**GOOD (Context Economy)**:
```
Main Chat:
User: "Implement feature X"
Assistant: "Delegating to golang-developer agent..."
Agent returns: "Feature X implemented. 3 files modified. Summary: ai-docs/session-123/summary.txt"
Assistant: "Implementation complete. Ready for review?"
Result: <20 lines in main context, crisp decision-making
```

### Delegation Pattern: The Three-Layer Architecture

```
┌─────────────────────────────────────────────────────────┐
│ Layer 1: MAIN CHAT (Orchestrator)                      │
│ Role: Strategy, decisions, user interaction            │
│ Context: Minimal (summaries only)                      │
│ Output: Next action, delegation instructions           │
└─────────────────────────────────────────────────────────┘
                           ↓ Delegates via Task tool
┌─────────────────────────────────────────────────────────┐
│ Layer 2: AGENTS (Specialized Workers)                  │
│ Role: Deep investigation, implementation, analysis     │
│ Context: Full access to codebase                       │
│ Output: Files + Brief summary (3-5 sentences)          │
└─────────────────────────────────────────────────────────┘
                           ↓ Writes to
┌─────────────────────────────────────────────────────────┐
│ Layer 3: FILES (Persistent Storage)                    │
│ Role: Detailed reports, code, analysis                 │
│ Context: Complete implementation details               │
│ Output: Referenced by path, read only when needed      │
└─────────────────────────────────────────────────────────┘
```

### Communication Protocol

#### Main Chat → Agent (Delegation)

**Template**:
```
Use Task tool with [agent-name]:

Task: [Specific, actionable task]

Input Files:
- [file1]
- [file2]

Your Job:
1. [Action 1]
2. [Action 2]
3. Write detailed results to: [output-path]

Return to Main Chat:
ONLY a 2-5 sentence summary in this format:

# [Task Name] Complete

Status: [Success/Partial/Failed]
Key Finding: [One-liner]
Changed: [N] files
Details: [output-path]
```

#### Agent → Main Chat (Return)

**Agents MUST return concise summaries:**
```
# Error Propagation Implementation Complete

Status: Success
Implemented: ? operator in 3 files (preprocessor, transformer, tests)
Tests: 15/15 passing
Details: ai-docs/sessions/20251118-143022/implementation-summary.md
```

**Agents MUST NOT return:**
- ❌ Full implementation code
- ❌ Complete test output
- ❌ Detailed file listings
- ❌ Long explanations (save for files)

#### Main Chat Uses Summary

**Main chat reads agent summary and decides:**
- ✅ Success? → Move to next phase
- ⚠️ Partial? → Ask user for guidance
- ❌ Failed? → Investigate or retry

**Main chat does NOT:**
- ❌ Read full implementation files (unless needed for user presentation)
- ❌ Parse detailed logs
- ❌ Store large data in context

### Agent Self-Awareness Rules (Anti-Recursion)

**CRITICAL FOR ALL AGENTS:**

#### Rule 1: Know Thyself

Every agent MUST be aware of its own type:
- If you are `golang-developer`, you cannot delegate to `golang-developer`
- If you are `astro-developer`, you cannot delegate to `astro-developer`
- If you are `code-reviewer`, you cannot delegate to `code-reviewer`
- If you are `golang-tester`, you cannot delegate to `golang-tester`
- If you are `golang-architect`, you cannot delegate to `golang-architect`
- If you are `astro-reviewer`, you cannot delegate to `astro-reviewer`

**Why:** You ARE the specialized agent. Delegating to yourself causes recursion and failures.

#### Rule 2: Delegation Decision Tree

```
Before using Task tool, ask:
│
├─ What is my agent type?
│  └─ I am: [agent-name]
│
├─ What agent type does this task need?
│  ├─ Same as me → ❌ DO NOT delegate. Implement directly.
│  └─ Different → ✅ CAN delegate to that different agent
│
└─ Why do I want to delegate?
   ├─ "To save context" → ❌ WRONG REASON. Just do the work.
   ├─ "Instructions say to" → ❌ Those are for CALLERS, not you.
   └─ "Need different expertise" → ✅ OK if it's a different agent type.
```

#### Rule 3: Proxy Mode Is Not Self-Delegation

**Proxy Mode Means:**
- Using `claudish` to consult external models (Grok, Gemini, Codex)
- Getting suggestions/implementations from those models
- Implementing their suggestions yourself

**Proxy Mode Does NOT Mean:**
- Using Task tool to invoke yourself
- Creating another instance of your own agent
- Delegating work you should do directly

#### Rule 4: Instructions Are Context-Dependent

When agents read their prompts and see:
- "Use the Task tool to invoke the golang-developer agent"
- "Delegate to astro-developer for implementation"

**Understand:** These instructions are for MAIN CHAT and EXTERNAL MODELS to use when calling the agent.

**NOT** for the agent to use to call itself.

The agent is the **destination** of those calls, not the **source**.

#### Rule 5: When In Doubt, Implement

If an agent is uncertain whether to delegate:
1. Check if delegating to own agent type → If yes, DON'T
2. Check if it has the expertise to implement → If yes, DO IT
3. Check if trying to save context → NOT A VALID REASON

**Default action for agents: Implement directly.**

#### Rule 6: External Model Invocation

When a specialized agent (golang-architect, astro-developer, etc.) is asked to invoke an external model:

✅ **Correct approach:**
- Use Bash tool to invoke claudish
- Read input prompt from file
- Save full response to file
- Return brief summary (MAX 5 sentences)

❌ **Incorrect approaches:**
- Delegating to another agent to invoke claudish
- Trying to invoke claudish via Task tool
- Returning full analysis in response (context bloat)

**Example (golang-architect agent)**:
```bash
# Read prompt
cat /path/to/investigation-prompt.md | \
  claudish --model openai/gpt-5.1-codex > \
  /path/to/output/gpt-5.1-codex-analysis.md

# Return brief summary only
```

#### Examples

**✅ CORRECT Delegation (Different Agent Types):**
- `golang-developer` delegates to `golang-tester` (different agent ✅)
- `astro-developer` delegates to `astro-reviewer` (different agent ✅)
- `golang-developer` delegates to `Explore` (different agent ✅)
- `golang-architect` delegates to `golang-developer` (different agent ✅)

**❌ WRONG Delegation (Recursion - Same Agent Type):**
- `golang-developer` delegates to `golang-developer` (same agent ❌)
- `astro-developer` delegates to `astro-developer` (same agent ❌)
- `code-reviewer` delegates to `code-reviewer` (same agent ❌)

### When to Delegate vs. Handle Directly

#### ✅ DELEGATE (Use Skills or Task Tool)

**Use Skills for Common Patterns**:
- Multi-model consultation → `multi-model-consult` skill
- Codebase investigation → `investigate` skill
- Feature implementation → `implement` skill
- Testing tasks → `test` skill

**Use Task Tool Directly** (without skill):
- One-off agent tasks
- Simple code review
- Quick analysis
- Tasks not covered by skills

#### ⚠️ HANDLE DIRECTLY (Main Chat)

**Only handle these yourself**:
- User interaction (questions, approvals, summaries)
- Single file read (known path)
- Single line fix
- Git status check
- Coordination (launching agents, deciding next phase)

**Rule of thumb**: If it takes >3 steps or >2 files → Delegate!

### File-Based Communication Patterns

#### Session Folders (Recommended for Workflows)

For orchestrated workflows (like `/dev`), use session folders:

```
ai-docs/sessions/YYYYMMDD-HHMMSS/
├── 01-planning/
│   ├── requirements.md       # Agent writes
│   ├── plan.md              # Agent writes
│   └── plan-summary.txt     # Agent returns this to main chat
├── 02-implementation/
│   ├── changes.md           # Agent writes
│   └── summary.txt          # Agent returns this to main chat
├── 03-review/
│   ├── review-full.md       # Agent writes
│   └── summary.txt          # Agent returns this to main chat
└── session-state.json       # Orchestrator writes
```

**Orchestrator** (main chat):
- Reads ONLY: `*-summary.txt` files and `session-state.json`
- Writes: `session-state.json` (tracks workflow state)
- Presents summaries to user

**Agents**:
- Write: Full detailed reports to `.md` files
- Write: Summary to `*-summary.txt`
- Return: "Summary written to: [path]"

#### Quick Tasks (Ad-hoc Delegation)

For one-off tasks, use simpler paths:

```
Main Chat: "Understand how Result<T,E> works"

Delegates to golang-developer:
- Investigates codebase
- Writes: ai-docs/analysis/result-type-analysis.md
- Returns: "Result<T,E> is implemented via AST transformation.
           Uses IIFE pattern for constructors.
           Details: ai-docs/analysis/result-type-analysis.md"

Main Chat:
- Receives summary
- Decides: Good enough? Or need more detail?
- If need detail: Reads the file
- Otherwise: Continues with next task
```

### Parallel Execution

**When agents can run in parallel**, launch ALL of them in a **single message**:

```
I'm launching 3 agents in parallel:

[Task tool call 1: golang-developer - Implement feature A]
[Task tool call 2: golang-developer - Implement feature B]
[Task tool call 3: golang-tester - Create tests]

(All 3 tool calls in ONE message)
```

**Benefits**:
- 3x speedup (tasks run simultaneously)
- All summaries come back together
- Can aggregate results efficiently

**Rules**:
- ✅ Parallel: Independent tasks (different files, separate features)
- ❌ Sequential: Dependencies (tests need implementation first)

### Example: Full Workflow with Delegation

**User Request**: "Implement lambda syntax for Dingo"

```
┌─────────────────────────────────────────────────────────┐
│ MAIN CHAT (Orchestrator)                                │
└─────────────────────────────────────────────────────────┘

User: "Implement lambda syntax for Dingo"

Main Chat:
1. Creates session: ai-docs/sessions/20251118-150000/
2. Delegates to golang-architect: "Plan lambda implementation"

┌─────────────────────────────────────────────────────────┐
│ GOLANG-ARCHITECT AGENT                                  │
└─────────────────────────────────────────────────────────┘

Agent:
- Reads features/lambdas.md
- Designs implementation approach
- Writes: 01-planning/plan.md (5 pages, detailed)
- Writes: 01-planning/summary.txt (3 sentences)
- Returns: "Lambda plan complete. 3-phase approach: preprocessor,
           parser, codegen. Details: 01-planning/plan.md"

┌─────────────────────────────────────────────────────────┐
│ MAIN CHAT (Orchestrator)                                │
└─────────────────────────────────────────────────────────┘

Main Chat:
- Receives summary (3 sentences)
- Reads plan.md for user presentation
- Shows plan to user
- User approves

Main Chat:
3. Launches 2 parallel golang-developer agents:
   - Agent A: Implement preprocessor
   - Agent B: Implement parser

┌─────────────────────────────────────────────────────────┐
│ GOLANG-DEVELOPER AGENTS (Parallel)                      │
└─────────────────────────────────────────────────────────┘

Agent A:
- Implements preprocessor in pkg/generator/preprocessor/
- Writes: 02-implementation/preprocessor-changes.md
- Returns: "Preprocessor complete. 2 files modified.
           Details: 02-implementation/preprocessor-changes.md"

Agent B (simultaneously):
- Implements parser patterns
- Writes: 02-implementation/parser-changes.md
- Returns: "Parser complete. 3 files modified.
           Details: 02-implementation/parser-changes.md"

┌─────────────────────────────────────────────────────────┐
│ MAIN CHAT (Orchestrator)                                │
└─────────────────────────────────────────────────────────┘

Main Chat:
- Receives both summaries
- Aggregates: "5 files modified total"
- Announces to user: "Implementation phase complete"

Main Chat:
4. Launches golang-tester: "Create tests for lambda syntax"

┌─────────────────────────────────────────────────────────┐
│ GOLANG-TESTER AGENT                                      │
└─────────────────────────────────────────────────────────┘

Agent:
- Creates golden tests
- Runs test suite
- Writes: 04-testing/test-results.md (full output, 200 lines)
- Writes: 04-testing/summary.txt (pass/fail counts)
- Returns: "Tests complete. 45/45 passing.
           Details: 04-testing/test-results.md"

┌─────────────────────────────────────────────────────────┐
│ MAIN CHAT (Orchestrator)                                │
└─────────────────────────────────────────────────────────┘

Main Chat:
- Receives summary: "45/45 passing"
- Announces to user: "Lambda implementation complete! All tests pass."
- Total context used: <50 lines (not 500+ lines)
```

### Metrics: Context Savings

**Traditional Approach** (no delegation):
- User request: 1 line
- Read 10 files: 500 lines
- Implementation code shown: 200 lines
- Test output shown: 150 lines
- Review discussion: 100 lines
- **Total: ~950 lines in main context** ❌

**Delegation Approach**:
- User request: 1 line
- Agent delegation: 3 lines × 4 agents = 12 lines
- Agent summaries: 3 lines × 4 agents = 12 lines
- Orchestrator decisions: 10 lines
- User announcements: 5 lines
- **Total: ~40 lines in main context** ✅

**Result**: **23x reduction in context usage** while achieving the same outcome!

### Key Rules for All Participants

#### Main Chat (Orchestrator) Rules:
1. ✅ Delegate complex tasks to agents
2. ✅ Read only summaries from agents
3. ✅ Read full files only when presenting to user
4. ✅ Launch parallel agents in single message
5. ✅ Keep context minimal (summaries + decisions)
6. ❌ Don't read full implementation files into context
7. ❌ Don't store detailed data in conversation
8. ❌ Don't do deep analysis yourself (delegate it)

#### Agent Rules:
1. ✅ Do deep investigation and implementation
2. ✅ Write ALL details to files
3. ✅ Return ONLY 2-5 sentence summary
4. ✅ Include file paths in summary
5. ✅ Use session folders for workflow tasks
6. ❌ Don't return full code in response
7. ❌ Don't return detailed logs in response
8. ❌ Don't return multi-page explanations in response

#### User Expectations:
- Main chat shows high-level progress
- Detailed results available in files
- Can request to see specific files
- Quick, clear decision points
- No context overwhelm

### Quick Delegation Templates

**For common patterns**, use skills (investigate, implement, test, multi-model-consult).

**For ad-hoc tasks**, delegate directly with Task tool:

**Basic Template**:
```
Task tool → [agent-type]:

[Task description]

Your Tasks:
1. [Action 1]
2. [Action 2]
3. Write detailed results to: [output-path]

Return to Main Chat (MAX 5 sentences):
[Brief summary format]
Details: [output-path]

DO NOT return full details in response.
```

**Agent MUST**:
- Write ALL details to files
- Return ONLY 2-5 sentence summary
- Include file path in response

See skills (`investigate.md`, `implement.md`, `test.md`) for detailed templates.

### Summary: Delegation Strategy Benefits

✅ **Context Economy**: 10-20x reduction in main chat context usage
✅ **Clarity**: Clear separation between orchestration and execution
✅ **Parallel Speedup**: 2-4x faster via concurrent agent execution
✅ **Persistence**: All work saved to files, nothing lost
✅ **Scalability**: Can handle large, complex multi-phase projects
✅ **Focus**: Main chat stays focused on decisions, not details

**Remember**:
- **Main chat = Strategy and decisions**
- **Agents = Investigation and execution**
- **Files = Detailed persistent storage**

This is the key to handling complex projects without context overload!

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

**Last Updated**: 2025-11-18 (Phase 4.1 complete, Phase 4.2 started)
**Current Phase**: Phase 4.2 - Pattern Matching Enhancements (In Progress)
**Previous Phase**: Phase 4.1 Complete - Pattern Matching + None Inference (57/57 tests passing)
**Session**: 20251118-173201

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

### 🎪 Showcase Example - THE Comprehensive Demo

**CRITICAL**: `tests/golden/showcase_01_api_server.dingo` is THE flagship example that demonstrates ALL currently implemented Dingo features in one realistic, production-like scenario.

**Purpose:**
1. **Landing Page Hero** - First example visitors see at dingolang.com
2. **Value Proposition** - Dramatic before/after comparison showing code reduction
3. **Integration Test** - Ensures all features work together harmoniously
4. **Marketing Asset** - Concrete proof of Dingo's productivity benefits

**Maintenance Rules (CRITICAL):**

✅ **ALWAYS update when implementing new features:**
- New feature implemented? → Add it to showcase immediately
- Feature modified? → Update showcase to reflect changes
- Feature removed? → Remove from showcase

✅ **Keep it comprehensive:**
- Should demonstrate EVERY major feature currently working
- Currently includes: Type annotations (`:` syntax), error propagation (`?` operator), `let` bindings
- Future: Add enums, Result<T,E>, Option<T>, pattern matching as they stabilize

✅ **Keep it realistic:**
- Real-world scenario (currently: user registration API)
- Production-quality code patterns
- Demonstrates features in practical context

✅ **Maintain documentation:**
- `showcase_01_api_server.reasoning.md` must explain ALL features
- Update metrics when code changes (line counts, reduction %)
- Keep comparison accurate between Dingo and Go versions

**Files to Update:**
1. `tests/golden/showcase_01_api_server.dingo` - Dingo source
2. `tests/golden/showcase_01_api_server.go.golden` - Transpiled Go (auto-generated via test)
3. `tests/golden/showcase_01_api_server.reasoning.md` - Comprehensive documentation
4. `tests/golden/README.md` - Reference in catalog

**Test Status Check:**
```bash
go test ./tests -run TestGoldenFiles/showcase_01_api_server -v
```
Must ALWAYS pass. If it fails after adding a feature, fix immediately.

**Landing Page Usage:**
This example should be the FIRST code sample visitors see. It demonstrates:
- How short and clean Dingo code is (left side)
- How verbose the equivalent Go is (right side)
- Immediate "wow factor" of code reduction
- Practical, relatable scenario (not toy examples)

**When in doubt:** If you implement ANY new feature, ask yourself: "Should this be in the showcase?" If it works and provides value, the answer is probably YES.