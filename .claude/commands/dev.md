# Development Orchestrator

**SCOPE: DINGO TRANSPILER/LANGUAGE DEVELOPMENT ONLY**

This orchestrator is for the Dingo transpiler and language (Go code in `/cmd/`, `/pkg/`, `/internal/`).

❌ **DO NOT USE for**: Astro landing page work (use `/astro-dev` instead)
✅ **USE for**: Dingo transpiler, parser, AST, language features

You are now running the **Development Orchestrator**, a file-based workflow coordinator that manages planning, implementation, code review, and testing phases.

## Core Principles

### 1. File-Based Communication

**CRITICAL**: All agents communicate through files. The orchestrator's context should only contain:
- Brief status updates
- File paths
- Next action decisions
- User-facing summaries

**Never** pass large code blocks, detailed plans, or full reviews through the orchestrator context.

### 2. Parallel Execution by Default

**CRITICAL**: This orchestrator MAXIMIZES PARALLELISM to achieve 3-4x speedup:

**Implementation Phase**:
- Analyze plan for independent subtasks (different files, separate features)
- Group into parallel batches based on dependencies
- Launch ALL tasks in a batch with a SINGLE message (multiple Task tool calls)
- Example: 3 independent features → 3 golang-developer agents running simultaneously

**Code Review Phase**:
- Launch ALL reviewers (internal + external models) in parallel
- Single message with multiple Task tool calls to code-reviewer agents
- Example: internal + 3 external models → 4 reviews running simultaneously

**When to Parallelize**:
- ✅ Multiple features from `features/` directory
- ✅ Separate golden test files
- ✅ Independent package implementations
- ✅ Multiple code reviewers

**When to Sequence**:
- ❌ AST changes → transformer logic (dependency)
- ❌ Implementation → tests (dependency)
- ❌ Refactoring shared code (conflicts)

**Performance Target**: For N independent tasks, aim for near-linear speedup (N tasks in ~1.2x time instead of Nx time)

## Session Setup

### Initialize Session Directory
Create a timestamped session directory:
```bash
SESSION_DIR="ai-docs/sessions/$(date +%Y%m%d-%H%M%S)"
mkdir -p $SESSION_DIR/{01-planning,02-implementation,03-reviews,04-testing,session-logs}
echo $SESSION_DIR > /tmp/dingo-dev-session
```

Store the session path in: `/tmp/dingo-dev-session`

### Create Session State File
Create `$SESSION_DIR/session-state.json`:
```json
{
  "session_id": "{timestamp}",
  "phase": "planning",
  "iteration": 1,
  "review_iterations": 0,
  "test_iterations": 0,
  "status": "active"
}
```

Use this file to track workflow state between agent calls.

## Phase 1: Planning

### Step 1.1: Capture User Request
Write the user's request to: `$SESSION_DIR/01-planning/user-request.md`

### Step 1.2: Invoke golang-architect for Planning
Use Task tool with golang-architect:

**Prompt**:
```
You are architecting a solution for the Dingo project.

INPUT FILES:
- User request: $SESSION_DIR/01-planning/user-request.md

YOUR TASK:
1. Read the user request from the file above
2. Design a detailed architecture and implementation plan
3. Identify gaps and ambiguities in the requirements

OUTPUT FILES (you MUST write to these files):
- $SESSION_DIR/01-planning/initial-plan.md - Your complete architectural plan
- $SESSION_DIR/01-planning/gaps.json - JSON array of questions to ask user: [{"question": "...", "rationale": "..."}]
- $SESSION_DIR/01-planning/summary.txt - 2-3 sentence summary of your plan

Return ONLY a brief status message (max 3 sentences) confirming you've written the files.
```

After the agent completes, read ONLY `$SESSION_DIR/01-planning/summary.txt` to display to user.

### Step 1.3: Ask Clarification Questions
Read `$SESSION_DIR/01-planning/gaps.json` and extract up to 3 most important questions.

Use AskUserQuestion tool to ask these questions.

Write user's answers to: `$SESSION_DIR/01-planning/clarifications.md`

### Step 1.4: Finalize Plan
Invoke golang-architect again:

**Prompt**:
```
Finalize the implementation plan.

INPUT FILES:
- Initial plan: $SESSION_DIR/01-planning/initial-plan.md
- User clarifications: $SESSION_DIR/01-planning/clarifications.md

YOUR TASK:
Incorporate the clarifications and create the final plan.

OUTPUT FILES:
- $SESSION_DIR/01-planning/final-plan.md - Complete final plan
- $SESSION_DIR/01-planning/plan-summary.txt - 3-4 bullet point summary for user

Return ONLY a brief confirmation message.
```

Read and display ONLY `$SESSION_DIR/01-planning/plan-summary.txt` to user.

### Step 1.5: Get User Approval
Use AskUserQuestion with options:
- "Proceed with implementation"
- "I want to suggest changes"

If changes requested, write to `$SESSION_DIR/01-planning/user-feedback.md` and repeat Step 1.4 with updated prompt.

Update session state: `"phase": "implementation"`

## Phase 2: Implementation

### Step 2.1: Analyze Implementation Plan for Parallelization

Read `$SESSION_DIR/01-planning/final-plan.md` and analyze for parallelization opportunities:

1. Identify independent subtasks (different files, separate features, isolated components)
2. Identify sequential dependencies (AST changes → transformers, implementation → tests)
3. Create parallel execution plan

Write to: `$SESSION_DIR/02-implementation/execution-plan.json`
```json
{
  "parallel_batches": [
    {
      "batch_id": 1,
      "tasks": [
        {"task_id": "A", "description": "...", "files": ["..."], "agent": "golang-developer"},
        {"task_id": "B", "description": "...", "files": ["..."], "agent": "golang-developer"}
      ]
    },
    {
      "batch_id": 2,
      "depends_on": [1],
      "tasks": [
        {"task_id": "C", "description": "...", "files": ["..."], "agent": "golang-developer"}
      ]
    }
  ]
}
```

### Step 2.2: Execute Implementation in Parallel Batches

For each batch in execution plan:

**Execute ALL tasks in a batch in PARALLEL** (single message with multiple Task tool calls):

For each task in the batch, use Task tool with golang-developer:

**Prompt template** (substitute {TASK_ID}, {TASK_DESCRIPTION}, {TASK_FILES}):
```
You are implementing subtask {TASK_ID} for the Dingo project.

INPUT FILES:
- Implementation plan: $SESSION_DIR/01-planning/final-plan.md
- User request: $SESSION_DIR/01-planning/user-request.md

YOUR SPECIFIC SUBTASK:
{TASK_DESCRIPTION}

TARGET FILES:
{TASK_FILES}

YOUR TASK:
Implement ONLY this specific subtask. Stay focused on the files and scope listed above.

OUTPUT FILES (you MUST write to these):
- $SESSION_DIR/02-implementation/task-{TASK_ID}-changes.md - Files you created/modified
- $SESSION_DIR/02-implementation/task-{TASK_ID}-notes.md - Decisions or deviations
- $SESSION_DIR/02-implementation/task-{TASK_ID}-status.txt - "SUCCESS" or "PARTIAL: {reason}"

Return ONLY: "Task {TASK_ID} complete: {one-line summary}"
```

**CRITICAL**:
- Launch ALL tasks in a batch with a SINGLE message containing multiple Task tool calls
- Do NOT wait between tasks in the same batch
- Wait for ALL tasks in a batch to complete before starting the next batch

### Step 2.3: Consolidate Implementation Results

After all batches complete, consolidate results:

Read all `task-*-status.txt` and `task-*-changes.md` files.

Create consolidated files:
- `$SESSION_DIR/02-implementation/changes-made.md` - All files modified across all tasks
- `$SESSION_DIR/02-implementation/implementation-notes.md` - Combined notes
- `$SESSION_DIR/02-implementation/status.txt` - Overall status

Display brief summary to user: "Implementation complete: {N} parallel tasks across {M} batches"

Update session state: `"phase": "code_review"`

## Phase 3: Code Review

### Step 3.1: Fetch Available Models and Ask Review Preferences

First, get available models from claudish:
```bash
claudish --list-models > $SESSION_DIR/03-reviews/available-models.txt
```

Parse the output to extract model names (lines containing model IDs before the description).

Then use AskUserQuestion (multiSelect: true):

Options:
- "Internal code-reviewer agent"
- {Each model from claudish --list-models as separate option}
- "Skip code review"

Example options based on current claudish output:
- "Internal code-reviewer agent"
- "x-ai/grok-code-fast-1 (Grok Code Fast)"
- "openai/gpt-5-codex (GPT-5 Codex)"
- "minimax/minimax-m2 (MiniMax M2)"
- "z-ai/glm-4.6 (GLM-4.6)"
- "qwen/qwen3-vl-235b-a22b-instruct (Qwen3 VL)"
- "anthropic/claude-sonnet-4.5 (Claude Sonnet 4.5)"
- "Skip code review"

If "Skip" selected, jump to Phase 5.

Write selections to: `$SESSION_DIR/03-reviews/reviewers.json` in format:
```json
{
  "internal": true/false,
  "external_models": ["model-id-1", "model-id-2"]
}
```

### Step 3.2: Create Review Iteration Directory
```bash
REVIEW_ITER=$SESSION_DIR/03-reviews/iteration-$(printf "%02d" $REVIEW_ITERATION)
mkdir -p $REVIEW_ITER
```

### Step 3.3: Run Reviews in Parallel

**For internal code-reviewer**:
Use Task tool with code-reviewer agent:

**Prompt**:
```
You are conducting a code review.

INPUT FILES:
- Changes made: $SESSION_DIR/02-implementation/changes-made.md
- Implementation plan: $SESSION_DIR/01-planning/final-plan.md
- Previous review feedback (if exists): $SESSION_DIR/03-reviews/iteration-{N-1}/consolidated.md

YOUR TASK:
Review all code changes. If this is a re-review, verify previous issues were fixed.

OUTPUT FILES:
- $REVIEW_ITER/internal-review.md - Detailed review with categorized issues (CRITICAL, IMPORTANT, MINOR)
- $REVIEW_ITER/internal-summary.txt - Format: "STATUS: APPROVED" or "STATUS: CHANGES_NEEDED\nCRITICAL_COUNT: N\nIMPORTANT_COUNT: N"

Return ONLY a one-line status.
```

**For external models** (run in parallel):
For each selected external model, use Task tool with **code-reviewer** agent in PROXY MODE:

**Prompt template** (substitute {MODEL_ID} and {MODEL_NAME}):
```
You are operating in PROXY MODE to conduct a code review using the external model {MODEL_NAME}.

INPUT FILES:
- Changes made: $SESSION_DIR/02-implementation/changes-made.md
- Implementation plan: $SESSION_DIR/01-planning/final-plan.md
- Previous review feedback (if exists): $SESSION_DIR/03-reviews/iteration-{N-1}/consolidated.md

YOUR TASK (PROXY MODE):
1. Read the input files above to gather context
2. Use the claudish CLI tool to delegate this review to {MODEL_NAME} (model ID: {MODEL_ID})
3. Provide the external model with:
   - Context from the input files
   - The list of changed files from changes-made.md
   - Request to review focusing on:
     * Correctness and bug-free implementation
     * Go best practices and idioms
     * Performance considerations
     * Code maintainability and readability
     * Architecture alignment with the plan
   - Request to categorize issues as CRITICAL/IMPORTANT/MINOR
4. Receive the response from the external model
5. Format and write the review to the output file

OUTPUT FILES (you MUST write):
- $REVIEW_ITER/{MODEL_ID}-review.md - Complete review with:
  * Categorized issues (CRITICAL, IMPORTANT, MINOR)
  * Specific file locations and line numbers
  * At the END, include this summary section:
    ---
    STATUS: APPROVED or CHANGES_NEEDED
    CRITICAL_COUNT: N
    IMPORTANT_COUNT: N
    MINOR_COUNT: N

Return ONLY: "Review by {MODEL_NAME} complete: {STATUS}"
```

**CRITICAL - Agent Selection and Execution**:
- **ALWAYS use code-reviewer agent for ALL code reviews** (internal AND external)
- **NEVER use golang-developer for code reviews** - it's for implementation, not review
- **Run ALL reviews in parallel**: Submit multiple Task tool calls in a SINGLE message (one per reviewer)
- **NEVER use Bash tool for external reviews**: The code-reviewer agent handles claudish internally in PROXY MODE
- External reviewers receive "PROXY MODE" instruction with model ID/name
- All agents (internal and external) return file paths and brief status only

### Step 3.4: Collect Review Status
Read ONLY the summary/status from each review file (last few lines):
- Extract STATUS and issue counts
- Count approvals vs changes needed

Display to user: "Reviews complete: {N} approved, {M} need changes"

### Step 3.5: Consolidate Feedback (if changes needed)
If ANY review needs changes, invoke code-reviewer:

**Prompt**:
```
You are consolidating multiple code reviews.

INPUT FILES:
- Review files: $REVIEW_ITER/*-review.md

YOUR TASK:
1. Read ALL review files from the directory above
2. Consolidate feedback, removing duplicates
3. Identify conflicts between reviewers
4. Prioritize issues by severity and frequency

OUTPUT FILES:
- $REVIEW_ITER/consolidated.md - Organized consolidated feedback with priority sections
- $REVIEW_ITER/action-items.md - Numbered list of specific fixes needed (just the critical/important ones)
- $REVIEW_ITER/consolidated-summary.txt - Format: "TOTAL_ISSUES: N\nCRITICAL: N\nIMPORTANT: N"

Return ONLY a one-sentence confirmation.
```

Read and display ONLY `$REVIEW_ITER/consolidated-summary.txt`.

## Phase 4: Fix Loop

### Step 4.1: Check if Fixes Needed
Read all review status files. If ALL say "APPROVED", skip to Phase 5.

### Step 4.2: Invoke golang-developer for Fixes
Use Task tool with golang-developer:

**Prompt**:
```
You are fixing issues found in code review.

INPUT FILES:
- Action items: $REVIEW_ITER/action-items.md
- Consolidated feedback: $REVIEW_ITER/consolidated.md
- Original plan: $SESSION_DIR/01-planning/final-plan.md

YOUR TASK:
Fix all CRITICAL and IMPORTANT issues. Do NOT break existing functionality.

OUTPUT FILES:
- $REVIEW_ITER/fixes-applied.md - List what you fixed
- $REVIEW_ITER/fix-status.txt - Single line: "ALL_FIXED" or "PARTIAL: {reason}"

Return ONLY a brief one-line status.
```

Read ONLY `$REVIEW_ITER/fix-status.txt`.

### Step 4.3: Increment and Re-review
Increment review iteration counter in session state.

Go back to Step 3.2 with new iteration number.

### Step 4.4: Safety Limit
After 5 iterations, ask user:
- "Continue fix loop"
- "Proceed to testing despite issues"
- "Stop and review manually"

Update session state: `"phase": "testing"`

## Phase 5: Testing

### Step 5.1: Invoke golang-tester
Use Task tool with golang-tester:

**Prompt**:
```
You are designing and running tests for the implementation.

INPUT FILES:
- Implementation plan: $SESSION_DIR/01-planning/final-plan.md
- Changes made: $SESSION_DIR/02-implementation/changes-made.md
- User request: $SESSION_DIR/01-planning/user-request.md

YOUR TASK:
1. Design comprehensive test scenarios
2. Implement tests
3. Run tests and capture results

OUTPUT FILES:
- $SESSION_DIR/04-testing/test-plan.md - What you're testing and why
- $SESSION_DIR/04-testing/test-results.md - Detailed test output
- $SESSION_DIR/04-testing/test-summary.txt - Format: "STATUS: PASS" or "STATUS: FAIL\nFAILED_TESTS: N\nTOTAL_TESTS: N"

Return ONLY a brief status message.
```

Read ONLY `$SESSION_DIR/04-testing/test-summary.txt`.

### Step 5.2: Handle Test Failures
If tests fail:

1. Invoke golang-developer:
```
Fix failing tests.

INPUT FILES:
- Test results: $SESSION_DIR/04-testing/test-results.md
- Test plan: $SESSION_DIR/04-testing/test-plan.md

OUTPUT FILES:
- $SESSION_DIR/04-testing/fixes-iteration-{N}.md

Return brief status.
```

2. Re-run golang-tester (update test-results and test-summary)

3. If still failing after 3 iterations, also run code-reviewer to check for issues introduced by fixes

4. Safety limit: After 3 test fix iterations, ask user for guidance

Update session state: `"phase": "complete", "status": "success"`

## Phase 6: Completion

### Step 6.1: Generate Session Report
Read these summary files ONLY:
- `$SESSION_DIR/01-planning/plan-summary.txt`
- `$SESSION_DIR/02-implementation/changes-made.md`
- `$SESSION_DIR/03-reviews/iteration-*/consolidated-summary.txt` (latest)
- `$SESSION_DIR/04-testing/test-summary.txt`
- `$SESSION_DIR/session-state.json`

Create brief completion report:
```
Development Session Complete
============================
Plan: {one-line from plan-summary}
Implementation: {count} files changed
Code Review: {iterations} iterations, final status: {status}
Testing: {status}

All session files: $SESSION_DIR/
```

### Step 6.2: Offer Next Steps
Ask user:
- "Create git commit"
- "Generate documentation"
- "Start new dev session"
- "Done"

## Critical Rules for Orchestrator

1. **Never read full content**: Only read summary/status files (*.txt, *-summary.*, *.json)
2. **Always pass file paths**: Agents read their own inputs from files
3. **Brief confirmations only**: Agents return max 3 sentence confirmations
4. **Update session state**: After each phase, update session-state.json
5. **Use TodoWrite**: Create todos for phases, not individual agent steps
6. **Parallel execution**: Run all reviews in parallel with multiple Task tool calls in a SINGLE message
   - Internal review: Task tool → code-reviewer agent (direct mode)
   - External reviews: Task tool → code-reviewer agent (PROXY MODE with model ID)
   - **NEVER use Bash tool for reviews** - agents handle their own tools
7. **Preserve session dir**: Never delete session directory, it's the audit trail

## TodoWrite Structure

Create todos at phase level:
- "Planning phase" (with sub-states: gathering requirements, creating plan, getting approval)
- "Implementation phase"
- "Code review phase (iteration {N})"
- "Testing phase"
- "Completion"

Update status as you progress through phases.

## Error Handling

- If agent doesn't write expected file: Re-run with explicit reminder about OUTPUT FILES
- If file read fails: Check path, inform user, ask to proceed or retry
- If external tool unavailable: Skip gracefully and note in session log
- Always log errors to: `$SESSION_DIR/session-logs/errors.log`

---

**Now begin: Initialize Session and Start Phase 1**

First, create the session directory structure and begin the planning phase.
