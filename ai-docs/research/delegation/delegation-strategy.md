# Delegation Strategy & Context Economy Guide

**Status**: Active
**Version**: 1.0.0
**Last Updated**: 2025-11-18

This guide provides comprehensive instructions for efficient delegation and context management in the Dingo project.

## Table of Contents

1. [Core Philosophy](#core-philosophy)
2. [The Three-Layer Architecture](#the-three-layer-architecture)
3. [Communication Protocols](#communication-protocols)
4. [Agent Responsibilities](#agent-responsibilities)
5. [Orchestrator Patterns](#orchestrator-patterns)
6. [File Organization](#file-organization)
7. [Best Practices](#best-practices)
8. [Anti-Patterns](#anti-patterns)
9. [Examples](#examples)

---

## Core Philosophy

### Main Principle: Context is Precious

Every token in the main conversation context costs:
- **Memory**: Finite context window (200K tokens)
- **Clarity**: More context = harder to make decisions
- **Speed**: Processing large context is slower

**Solution**: Keep main chat minimal by delegating deep work to agents.

### The 90/10 Rule

- **90% of work** happens in agent threads and files
- **10% of communication** happens in main chat (summaries, decisions)

**Example**:
- Agent investigates 50 files, 2000 lines of code
- Agent returns: "Found 3 patterns. Details: analysis.md" (1 line)
- Main chat stays lean, agent thread contains everything

---

## The Three-Layer Architecture

```
┌──────────────────────────────────────────────────────────────┐
│ Layer 1: MAIN CHAT (Orchestrator)                           │
├──────────────────────────────────────────────────────────────┤
│ Role:          Strategy, decisions, user interaction         │
│ Context Size:  Minimal (<50 lines per task)                  │
│ Reads:         Agent summaries only                          │
│ Writes:        Delegation instructions, decisions            │
│ Tools:         Task, AskUserQuestion, TodoWrite              │
└──────────────────────────────────────────────────────────────┘
                             ↓
                    Delegates via Task tool
                             ↓
┌──────────────────────────────────────────────────────────────┐
│ Layer 2: AGENTS (Specialized Workers)                       │
├──────────────────────────────────────────────────────────────┤
│ Role:          Investigation, implementation, analysis       │
│ Context Size:  Full (can read entire codebase)              │
│ Reads:         Files, code, documentation                    │
│ Writes:        Detailed reports to files + brief summaries   │
│ Tools:         Read, Write, Edit, Grep, Glob, Bash          │
└──────────────────────────────────────────────────────────────┘
                             ↓
                      Writes to files
                             ↓
┌──────────────────────────────────────────────────────────────┐
│ Layer 3: FILES (Persistent Storage)                         │
├──────────────────────────────────────────────────────────────┤
│ Role:          Long-term storage of detailed work            │
│ Context Size:  Unlimited (filesystem)                        │
│ Contains:      Full reports, analysis, code changes          │
│ Read by:       Orchestrator (when presenting to user)        │
│                Agents (as inputs for next phase)             │
└──────────────────────────────────────────────────────────────┘
```

### Layer Responsibilities

| Layer | Reads | Writes | Returns | Context Size |
|-------|-------|--------|---------|--------------|
| Main Chat | Summaries | Decisions | User messages | <50 lines/task |
| Agents | Full files | Files + summaries | 2-5 sentence summary | Unlimited |
| Files | N/A | N/A | Full content | Unlimited |

---

## Communication Protocols

### Protocol 1: Main Chat → Agent (Delegation)

**Format**:
```
Task tool → [agent-name]:

Task: [One-line description]

Context:
- [Background info]
- [Key constraints]

Input Files:
- [file1]
- [file2]

Your Job:
1. [Specific action 1]
2. [Specific action 2]
3. Write detailed results to: [output-file]

Return to Main Chat (BRIEF SUMMARY ONLY):
# [Task Name] Complete

Status: [Success/Partial/Failed]
[One-liner key finding]
Files: [count] modified/created
Details: [file-path]
```

**Example**:
```
Task tool → golang-developer:

Task: Implement error propagation preprocessor

Context:
- Error propagation uses ? operator (like Rust)
- Transforms x? → if err != nil { return err }
- Session: ai-docs/sessions/20251118-150000/

Input Files:
- Plan: ai-docs/sessions/20251118-150000/01-planning/plan.md
- Existing: pkg/generator/preprocessor/type_annot.go (for pattern)

Your Job:
1. Read plan.md for requirements
2. Create pkg/generator/preprocessor/error_prop.go
3. Implement ErrorPropProcessor following TypeAnnotProcessor pattern
4. Add unit tests in error_prop_test.go
5. Document regex patterns and edge cases
6. Write implementation summary to: ai-docs/sessions/20251118-150000/02-implementation/error-prop-summary.md

Return to Main Chat (BRIEF SUMMARY ONLY):
# Error Propagation Preprocessor Complete

Status: Success
Implemented: ? operator transformation in error_prop.go
Tests: 12/12 passing
Files: 2 created (error_prop.go, error_prop_test.go)
Details: ai-docs/sessions/20251118-150000/02-implementation/error-prop-summary.md
```

### Protocol 2: Agent → Main Chat (Return)

**Required Format**:
```
# [Task Name] Complete

Status: [Success/Partial/Failed]
[1-2 sentence key finding or result]
[Metrics: files, tests, etc.]
Details: [file-path]
```

**Rules**:
- ✅ Maximum 5 sentences total
- ✅ Include status (Success/Partial/Failed)
- ✅ Include metrics (files changed, tests passed, etc.)
- ✅ Include file path to full details
- ❌ NO code snippets
- ❌ NO full file listings
- ❌ NO multi-paragraph explanations
- ❌ NO detailed logs

**Good Example**:
```
# Lambda Syntax Implementation Complete

Status: Success
Implemented preprocessor pattern for fn(x) => expr syntax.
Tests: 18/18 passing (tests/golden/lambda_*.dingo)
Files: 3 modified (preprocessor/lambda.go, generator.go, lambda_test.go)
Details: ai-docs/sessions/20251118-150000/02-implementation/lambda-summary.md
```

**Bad Example** ❌:
```
# Lambda Syntax Implementation

I've successfully implemented the lambda syntax feature for Dingo! Here's what I did:

First, I analyzed the requirements in the plan.md file and determined that we needed to support the following syntax patterns:
1. Single expression: fn(x) => x + 1
2. Block syntax: fn(x) => { return x + 1 }
3. Multiple parameters: fn(x, y) => x + y
4. No parameters: fn() => 42

Then I created the LambdaProcessor in pkg/generator/preprocessor/lambda.go:

```go
type LambdaProcessor struct {
    pattern *regexp.Regexp
}

func (p *LambdaProcessor) Process(input string) string {
    // Transform fn(params) => expr to func(params) Type { return expr }
    return p.pattern.ReplaceAllStringFunc(input, func(match string) string {
        // ... 50 more lines
    })
}
```

[... 200 more lines of detailed explanation ...]

All tests are passing! Here's the test output:

=== RUN   TestLambdaProcessor
=== RUN   TestLambdaProcessor/single_expression
    lambda_test.go:15: PASS
=== RUN   TestLambdaProcessor/block_syntax
    lambda_test.go:25: PASS
[... 50 more lines of test output ...]
```
^ This is WAY TOO MUCH for main chat! Save all details to files.

### Protocol 3: Main Chat Uses Summary

**Decision Tree**:
```
Receive agent summary
    ↓
Status = Success?
├─ YES → Announce to user, proceed to next phase
│        (Don't read detailed files unless user asks)
│
├─ PARTIAL → Read details file, decide:
│            - Can we continue? → Proceed with note
│            - Need fix? → Delegate fix task to agent
│
└─ FAILED → Read details file, understand error
            Ask user for guidance or retry with adjustments
```

**Orchestrator Rules**:
1. Always trust the agent's status (Success/Partial/Failed)
2. Read details file ONLY if:
   - Status is Partial/Failed (to understand what happened)
   - User explicitly asks to see details
   - Need to present specific findings to user
3. Never copy full details into main chat
4. Reference files by path when discussing them

---

## Agent Responsibilities

### What Agents MUST Do

#### 1. Write Detailed Results to Files

**Example Structure**:
```
ai-docs/sessions/20251118-150000/02-implementation/feature-x-summary.md

# Feature X Implementation

## Overview
[Full description of what was implemented]

## Files Changed
- pkg/feature/x.go (created)
  - New functions: ProcessX(), ValidateX()
  - Dependencies: pkg/common, pkg/util

- pkg/feature/x_test.go (created)
  - 15 test cases covering happy path and edge cases
  - All tests passing

## Implementation Details
[Detailed explanation of approach, decisions, patterns used]

## Challenges & Solutions
[Any difficulties encountered and how they were resolved]

## Testing Results
```
=== RUN   TestProcessX
=== RUN   TestProcessX/happy_path
    x_test.go:20: PASS
[Full test output]
```

## Next Steps / Notes for Future Work
[Anything that needs follow-up or consideration]
```

#### 2. Return Brief Summary

After writing the detailed file, return ONLY:
```
# Feature X Implementation Complete

Status: Success
Implemented ProcessX() and ValidateX() with full test coverage.
Tests: 15/15 passing
Files: 2 created (x.go, x_test.go)
Details: ai-docs/sessions/20251118-150000/02-implementation/feature-x-summary.md
```

#### 3. Use Session Folders for Workflows

For orchestrated workflows (invoked by `/dev` or `/astro-dev`):
- Always write to the session folder provided
- Follow the folder structure:
  - `01-planning/` - Plans, requirements, architecture
  - `02-implementation/` - Code changes, implementation notes
  - `03-review/` - Review reports, feedback
  - `04-testing/` - Test results, coverage reports
  - `05-fixes/` - Bug fixes, refinements
- Write both detailed `.md` files AND `*-summary.txt` files

#### 4. Use ai-docs/ for Ad-hoc Tasks

For one-off investigations outside workflows:
- `ai-docs/analysis/` - Code analysis, investigations
- `ai-docs/research/` - Research notes, comparisons
- `ai-docs/reports/` - One-time reports

### What Agents MUST NOT Do

❌ **Return full code in response**
❌ **Return complete test output in response**
❌ **Return multi-page explanations in response**
❌ **Return detailed file listings in response**
❌ **Assume orchestrator will read files automatically**

---

## Orchestrator Patterns

### Pattern 1: Simple Task Delegation

**Use Case**: Single, independent task

```
User: "Understand how Result<T,E> works"

Orchestrator:
1. Delegates to golang-developer with investigation task
2. Receives summary: "Result<T,E> uses AST transformation..."
3. If summary sufficient: Done
4. If need details: Reads file and presents to user
```

**Code**:
```
// Step 1: Delegate
Task tool → golang-developer:
  Investigate: How Result<T,E> type is implemented
  Output: ai-docs/analysis/result-type.md
  Return: Brief summary with file path

// Step 2: Receive summary (automatic)
Agent returns: "Result<T,E> implemented via AST..."

// Step 3: Decision
If user satisfied: Continue
If need detail: Read ai-docs/analysis/result-type.md and show user
```

### Pattern 2: Multi-Phase Workflow

**Use Case**: Plan → Implement → Review → Test

```
User: "Add feature X"

Orchestrator:
Phase 1: Planning
  - Delegate to golang-architect
  - Receive plan summary
  - Present to user, get approval

Phase 2: Implementation
  - Delegate to golang-developer
  - Receive implementation summary
  - Announce completion to user

Phase 3: Review
  - Delegate to code-reviewer
  - Receive review summary
  - If issues: goto Fix Loop
  - If approved: Continue

Phase 4: Testing
  - Delegate to golang-tester
  - Receive test summary
  - If fail: goto Fix Loop
  - If pass: Done

Fix Loop:
  - Delegate fixes to golang-developer
  - Re-run review/tests
  - Max 5 iterations, then ask user
```

### Pattern 3: Parallel Execution

**Use Case**: Multiple independent tasks

```
User: "Implement features A, B, and C"

Orchestrator:
1. Analyzes: Are A, B, C independent? → YES
2. Launches 3 agents IN PARALLEL (single message):
   - Task call 1: golang-developer → Feature A
   - Task call 2: golang-developer → Feature B
   - Task call 3: golang-developer → Feature C
3. Waits for all 3 to complete
4. Receives 3 summaries
5. Aggregates: "3 features implemented, X files changed"
6. Announces to user

Speedup: 3x (vs sequential execution)
```

**Critical**: All parallel agents MUST be invoked in a SINGLE message with multiple Task tool calls.

---

## File Organization

### Session Folder Structure (Workflows)

For `/dev` and `/astro-dev` workflows:

```
ai-docs/sessions/YYYYMMDD-HHMMSS/
├── 00-metadata.json               # Orchestrator writes (session info)
├── 01-planning/
│   ├── requirements.md            # Agent writes (full requirements)
│   ├── plan.md                    # Agent writes (detailed plan)
│   ├── plan-summary.txt           # Agent writes, Orchestrator reads
│   └── plan-approved.md           # Orchestrator writes after user approval
├── 02-implementation/
│   ├── changes.md                 # Agent writes (all files changed)
│   ├── implementation-notes.md    # Agent writes (decisions, challenges)
│   ├── summary.txt                # Agent writes, Orchestrator reads
│   └── task-[A|B|C]-*.md          # Parallel task results
├── 03-review/
│   ├── iteration-01/
│   │   ├── internal-review.md     # code-reviewer agent writes
│   │   ├── grok-review.md         # external review via proxy
│   │   ├── consolidated.md        # consolidation agent writes
│   │   └── summary.txt            # Orchestrator reads
│   └── iteration-02/
│       └── [same structure]
├── 04-testing/
│   ├── test-plan.md               # golang-tester writes
│   ├── test-results.md            # golang-tester writes (full output)
│   └── summary.txt                # Orchestrator reads
├── 05-fixes/
│   ├── iteration-01/
│   │   ├── fixes-applied.md       # Agent writes
│   │   └── summary.txt
│   └── iteration-02/
│       └── [same structure]
└── final-summary.md               # Orchestrator writes at end
```

### Ad-hoc Task Storage

For one-off tasks outside workflows:

```
ai-docs/
├── analysis/                      # Code investigations
│   ├── result-type-analysis.md
│   ├── error-handling-patterns.md
│   └── performance-bottlenecks.md
├── research/                      # Research notes
│   ├── parser-comparison.md
│   ├── delegation/                # This guide!
│   └── type-inference.md
└── reports/                       # One-time reports
    ├── code-review-20251118.md
    └── test-coverage-analysis.md
```

---

## Best Practices

### For Orchestrators (Main Chat)

✅ **DO**:
1. Create session folders for multi-phase workflows
2. Delegate complex tasks to specialized agents
3. Launch parallel agents in single message
4. Read only summaries from agents
5. Read full files only when presenting to user
6. Keep context minimal (<50 lines per task)
7. Use TodoWrite to track high-level phases
8. Trust agent status (Success/Partial/Failed)

❌ **DON'T**:
1. Read full implementation files into context
2. Copy detailed reports into conversation
3. Do deep code analysis yourself (delegate it)
4. Launch agents sequentially when they can run parallel
5. Store large data in conversation
6. Skip session folder creation for workflows
7. Read files "just to know" (only when needed)

### For Agents

✅ **DO**:
1. Write ALL detailed work to files
2. Return ONLY brief summaries (2-5 sentences)
3. Include file paths in summaries
4. Use provided session folders for workflows
5. Follow return format exactly (Status, metrics, details path)
6. Write both detailed `.md` AND `*-summary.txt`
7. Include metrics (files changed, tests passed, etc.)

❌ **DON'T**:
1. Return full code in response
2. Return complete test output in response
3. Return multi-page explanations in response
4. Assume orchestrator read your detailed file
5. Write summary to main chat AND file (file only, return path)
6. Use ambiguous status (use Success/Partial/Failed)

### For Users

✅ **Expect**:
1. Main chat shows high-level progress
2. Detailed results available in files
3. Can request to see specific details
4. Quick, clear decision points
5. Agents working in parallel when possible

❌ **Don't**:
1. Expect to see all code in chat
2. Expect to see all test output in chat
3. Expect instant responses (agents need time)

---

## Anti-Patterns

### Anti-Pattern 1: Context Bloat

**Bad**:
```
User: "Implement feature X"

Orchestrator:
- Reads 10 files (500 lines) into context
- Shows full plan (200 lines) to user
- Delegates to agent
- Agent returns full code (300 lines)
- Orchestrator shows full test output (150 lines)
- Total: 1150 lines in context ❌
```

**Good**:
```
User: "Implement feature X"

Orchestrator:
- Delegates to golang-architect with task
- Receives summary: "Plan ready. Details: plan.md"
- Reads plan.md only to show user
- Delegates to golang-developer
- Receives summary: "Feature done. 5 files. Details: summary.md"
- Total: <50 lines in context ✅
```

### Anti-Pattern 2: Sequential Execution

**Bad**:
```
User: "Implement A, B, C"

Orchestrator:
Message 1: Task → golang-developer (Feature A)
[wait for response]
Message 2: Task → golang-developer (Feature B)
[wait for response]
Message 3: Task → golang-developer (Feature C)
[wait for response]
Total time: 3x ❌
```

**Good**:
```
User: "Implement A, B, C"

Orchestrator (SINGLE message):
Task call 1 → golang-developer (Feature A)
Task call 2 → golang-developer (Feature B)
Task call 3 → golang-developer (Feature C)
[all run in parallel]
Total time: 1.2x ✅ (3x speedup!)
```

### Anti-Pattern 3: Agent Verbal Diarrhea

**Bad**:
```
Agent returns:
"I've implemented the feature! Here's what I did:
[10 paragraphs of explanation]
[Full code listing: 200 lines]
[Complete test output: 100 lines]
[Architecture diagrams in ASCII art]
..." ❌

Orchestrator context bloated with 500+ lines
```

**Good**:
```
Agent returns:
"# Feature X Complete

Status: Success
Implemented in x.go with 12 tests passing.
Files: 2 created, 1 modified
Details: ai-docs/sessions/.../summary.md"

Orchestrator context: 4 lines ✅
```

---

## Examples

### Example 1: Investigation Task

**Scenario**: User asks "How does error propagation work?"

**Main Chat**:
```
User: "How does error propagation work in Dingo?"

Orchestrator:
"I'll investigate the error propagation implementation."

[Task tool → golang-developer]

Task: Investigate error propagation mechanism

Output File: ai-docs/analysis/error-propagation.md

Your Job:
1. Find all code related to ? operator
2. Understand preprocessor transformation
3. Document the complete flow
4. Include examples from golden tests
5. Write detailed analysis to output file

Return Brief Summary:
- What: [mechanism description]
- Where: [key files]
- Details: [file path]

[Agent executes, returns]:
# Error Propagation Investigation Complete

Status: Success
Error propagation uses ? operator, transformed by ErrorPropProcessor.
Pattern: x? → if err != nil { return nil, err }
Key files: pkg/generator/preprocessor/error_prop.go, tests/golden/error_prop_*.dingo
Details: ai-docs/analysis/error-propagation.md

[Orchestrator to user]:
"Error propagation in Dingo uses the ? operator (like Rust). The ErrorPropProcessor
transforms `x?` into Go error handling code.

Full analysis: ai-docs/analysis/error-propagation.md

Would you like me to show you specific parts?"
```

**Context Used**:
- Delegation: 3 lines
- Agent summary: 4 lines
- User response: 3 lines
- **Total: 10 lines** ✅

### Example 2: Multi-Phase Workflow

**Scenario**: User asks "/dev implement lambda syntax"

**Main Chat** (orchestrator throughout):

```
Phase 1: Planning

User: "/dev implement lambda syntax"

[Session created: ai-docs/sessions/20251118-150000/]

Orchestrator: "Creating session and planning..."

[Task → golang-architect]
Returns: "Plan complete. 3-phase implementation. Details: 01-planning/plan.md"

Orchestrator: [Reads plan.md, shows to user]
"Here's the implementation plan:
1. Preprocessor for fn() => syntax
2. AST transformation
3. Golden tests

Proceed?"

User: "Yes"

---

Phase 2: Implementation (Parallel)

Orchestrator: "Launching 2 agents in parallel..."

[Single message, 2 Task calls]:
  Task 1 → golang-developer: Implement preprocessor
  Task 2 → golang-developer: Create golden tests

Returns:
  Agent 1: "Preprocessor done. 2 files. Details: ..."
  Agent 2: "Tests created. 6 files. Details: ..."

Orchestrator: "Implementation complete. 8 files modified."

---

Phase 3: Review

Orchestrator: "Launching code review..."

[Task → code-reviewer]
Returns: "Review complete. 2 minor issues. Details: ..."

Orchestrator: [Reads issues]
"Review found 2 minor style issues. Fix them?"

User: "Yes"

---

Phase 4: Fixes

[Task → golang-developer: Fix issues]
Returns: "Fixed. Details: ..."

[Task → code-reviewer: Re-review]
Returns: "Approved. No issues."

Orchestrator: "All issues resolved! Implementation approved."

---

Phase 5: Testing

[Task → golang-tester]
Returns: "Tests: 24/24 passing. Details: ..."

Orchestrator: "All tests passing! Lambda syntax implementation complete.

Session: ai-docs/sessions/20251118-150000/
Files: 10 modified/created
Tests: 24/24 passing

Summary: ai-docs/sessions/20251118-150000/final-summary.md"
```

**Context Used**:
- Phase 1: 8 lines
- Phase 2: 6 lines
- Phase 3: 8 lines
- Phase 4: 6 lines
- Phase 5: 6 lines
- **Total: 34 lines** ✅

Without delegation: Would be 800+ lines ❌

---

## Summary

### Key Takeaways

1. **Main Chat = Orchestration**
   - Keep context minimal
   - Delegate complex tasks
   - Read only summaries

2. **Agents = Execution**
   - Do deep work
   - Write to files
   - Return brief summaries

3. **Files = Persistence**
   - Store all details
   - Referenced by path
   - Read when needed

4. **Parallel > Sequential**
   - Launch independent agents together
   - 2-4x speedup
   - Single message, multiple Task calls

5. **Context Economy**
   - 10-20x reduction
   - Clearer decisions
   - Scalable to large projects

### Quick Reference

**When delegating**:
```
Task → agent:
  What: [task]
  Input: [files]
  Output: [file]
  Return: Brief summary only
```

**Agent returns**:
```
# Task Complete
Status: Success/Partial/Failed
[1-liner result]
Details: [path]
```

**Orchestrator uses**:
```
If Success: Announce, proceed
If Partial: Investigate, decide
If Failed: Read details, retry/ask user
```

---

**Remember**: The goal is to **maximize clarity and efficiency** by keeping the main conversation focused on **strategy and decisions**, while **agents handle the detailed work** and **files preserve everything**.

This is how we handle complex multi-month projects without losing context!
