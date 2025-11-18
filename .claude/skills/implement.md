# Implementation Skill

You are executing the **Implementation** pattern. This skill helps you delegate feature implementation to specialized agents while orchestrating progress and maintaining context economy.

## Your Task

The user wants to implement a feature. Follow these steps:

### Step 1: Understand Requirements

Extract from user's request:
- What feature to implement?
- What are the requirements?
- Are there constraints or preferences?
- Which files/components affected?

If unclear, use AskUserQuestion tool to clarify BEFORE delegating.

### Step 2: Create Session Folder

```bash
SESSION=$(date +%Y%m%d-%H%M%S)
mkdir -p ai-docs/sessions/$SESSION/{input,output}
```

### Step 3: Write Requirements (Optional but Recommended)

For complex features, write requirements to file:
```
ai-docs/sessions/$SESSION/input/requirements.md
```

Include:
- Feature description
- Acceptance criteria
- Implementation constraints
- Affected components

### Step 4: Choose Appropriate Agent

Based on domain:
- **Go code** (transpiler, parser, AST) → `golang-developer`
- **Astro/React** (landing page) → `astro-developer`
- **Tests** → `golang-tester` (for Go) or included in implementation
- **Architecture planning** → `golang-architect` (plan first) then `golang-developer` (implement)

**For complex features**:
1. First: `golang-architect` to create implementation plan
2. Then: `golang-developer` to implement based on plan
3. Finally: `golang-tester` to create/run tests

### Step 5: Delegate Implementation

#### Simple Feature (Single Agent)

```
Task tool → golang-developer:

Implement: [Feature description]

Requirements:
- [Requirement 1]
- [Requirement 2]
- [Requirement 3]

Your Tasks:
1. Implement the feature
2. Make necessary code changes
3. Ensure code compiles and follows project conventions
4. Write summary to: ai-docs/sessions/$SESSION/output/implementation-summary.md

Return to Main Chat (MAX 5 sentences):
Status: Success/Partial/Failed
Files modified: [count] files
Key changes: [brief description]
Tests: [status if run]
Details: ai-docs/sessions/$SESSION/output/implementation-summary.md

DO NOT return code or detailed changes in response.
```

#### Complex Feature (Multi-Phase)

**Phase 1: Planning**
```
Task tool → golang-architect:

Plan implementation for: [Feature description]

Your Tasks:
1. Design implementation approach
2. Identify files to modify/create
3. Plan implementation phases
4. Write plan to: ai-docs/sessions/$SESSION/output/implementation-plan.md

Return brief summary (MAX 5 sentences).
```

**Phase 2: Implementation** (after plan approval)
```
Task tool → golang-developer:

Implement: [Feature] following plan at ai-docs/sessions/$SESSION/output/implementation-plan.md

[Rest similar to simple feature...]
```

**Phase 3: Testing** (optional)
```
Task tool → golang-tester:

Create tests for: [Feature]

[See test.md skill for details]
```

### Step 6: Parallel Implementation (Independent Components)

If feature has **independent parts** that can be implemented in parallel:

**Launch multiple golang-developer agents in SINGLE MESSAGE**:
```
I'm launching 2 golang-developer agents in parallel:

Task 1 → golang-developer: Implement component A
Task 2 → golang-developer: Implement component B

(Both tasks in ONE message)
```

### Step 7: Track Progress

Use TodoWrite tool to track implementation phases:
```
- Planning: in_progress
- Implementation: pending
- Testing: pending
- Review: pending
```

Update status as phases complete.

### Step 8: Present Results to User

After implementation completes:
1. Show brief summary from agent
2. List files modified
3. Mention test status (if run)
4. Provide path to detailed summary
5. Ask if user wants to review changes or run tests

### Example Execution

```
User: "Implement lambda syntax support"

You (main chat):
1. Create session: ai-docs/sessions/20251118-161000/
2. Use AskUserQuestion if needed to clarify syntax
3. Delegate to golang-architect for plan
4. Present plan to user for approval
5. Launch 2 parallel golang-developer agents:
   - Agent A: Implement preprocessor
   - Agent B: Implement AST transformer
6. Receive summaries: "5 files modified"
7. Delegate to golang-tester: "Create golden tests"
8. Present to user:
   "Lambda syntax implemented!
    - 5 files modified (preprocessor + transformer)
    - Golden tests created and passing
    - Details: ai-docs/sessions/20251118-161000/output/
    Ready for code review?"

Total context: ~30 lines
Detailed changes: In session files
```

## Key Rules

1. ✅ **Clarify requirements** before implementing (use AskUserQuestion)
2. ✅ **Plan complex features** (golang-architect first)
3. ✅ **Use parallel execution** for independent components
4. ✅ **Track progress** with TodoWrite
5. ✅ **File-based communication** (detailed changes in files)
6. ❌ **Never implement directly** in main chat (multi-file changes)
7. ❌ **Never skip planning** for complex features
8. ❌ **Never show full code** in response (use file paths)

## When to Implement Directly (Skip Skill)

**Only** for trivial changes:
- Single-line fixes
- Adding one comment
- Updating one config value

**Everything else**: Use this skill (delegate to agent)

## Success Metrics

- **Speed**: 2-4x faster with parallel execution
- **Context**: 20-50 lines vs 500+ lines
- **Quality**: Planned, reviewed, tested

## What to Return to User

1. Brief implementation summary (5 sentences)
2. Files modified count
3. Test status
4. Session folder path
5. Next step offer (review, test, deploy)

**Orchestrate, don't implement. Let agents handle the details!**
