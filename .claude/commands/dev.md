# Development Orchestrator

You are now running the **Development Orchestrator**, a file-based workflow coordinator that manages planning, implementation, code review, and testing phases.

## Core Principle: File-Based Communication

**CRITICAL**: All agents communicate through files. The orchestrator's context should only contain:
- Brief status updates
- File paths
- Next action decisions
- User-facing summaries

**Never** pass large code blocks, detailed plans, or full reviews through the orchestrator context.

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

### Step 2.1: Invoke golang-system-architect
Use Task tool with golang-system-architect:

**Prompt**:
```
You are implementing the solution for the Dingo project.

INPUT FILES:
- Implementation plan: $SESSION_DIR/01-planning/final-plan.md
- User request: $SESSION_DIR/01-planning/user-request.md

YOUR TASK:
Implement the complete solution according to the plan. Make all necessary code changes.

OUTPUT FILES (you MUST write to these):
- $SESSION_DIR/02-implementation/changes-made.md - List of files created/modified with brief descriptions
- $SESSION_DIR/02-implementation/implementation-notes.md - Any important decisions or deviations from plan
- $SESSION_DIR/02-implementation/status.txt - Single line: "SUCCESS" or "PARTIAL: {reason}"

Return ONLY a 2-3 sentence summary of what you implemented.
```

Read ONLY `$SESSION_DIR/02-implementation/status.txt` and `$SESSION_DIR/02-implementation/changes-made.md` (just the list, not full content).

Display brief summary to user.

Update session state: `"phase": "code_review"`

## Phase 3: Code Review

### Step 3.1: Ask Review Preferences
Use AskUserQuestion (multiSelect: true):

Options:
- "Internal code-reviewer agent"
- "Gemini (via claudish)"
- "GPT-4 (via claudish)"
- "Claude Opus (via claudish)"
- "Skip code review"

If "Skip" selected, jump to Phase 5.

Write selections to: `$SESSION_DIR/03-reviews/reviewers.json`

### Step 3.2: Create Review Iteration Directory
```bash
REVIEW_ITER=$SESSION_DIR/03-reviews/iteration-$(printf "%02d" $REVIEW_ITERATION)
mkdir -p $REVIEW_ITER
```

### Step 3.3: Run Reviews in Parallel

**For internal code-reviewer**:
Use Task tool:

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
For each selected model, use Bash:

```bash
claudish -m {model} "$(cat <<'EOF'
You are conducting a code review for the Dingo project.

CONTEXT FILES TO READ:
- Implementation plan: $SESSION_DIR/01-planning/final-plan.md
- Changes made: $SESSION_DIR/02-implementation/changes-made.md
- Use 'find' and 'cat' commands to examine the actual code files

YOUR TASK:
Review the code changes. Focus on: correctness, Go best practices, performance, maintainability.

CATEGORIZE ISSUES:
- CRITICAL: Must fix (bugs, security, correctness)
- IMPORTANT: Should fix (code quality, best practices)
- MINOR: Nice to have (style, optimizations)

OUTPUT FORMAT:
Write your review to: $REVIEW_ITER/{model}-review.md

At the END of the file, add:
---
STATUS: APPROVED or CHANGES_NEEDED
CRITICAL_COUNT: N
IMPORTANT_COUNT: N
MINOR_COUNT: N

After writing the file, output only: "Review written to {model}-review.md"
EOF
)"
```

Run all external reviews in parallel using multiple Bash calls in one message.

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

### Step 4.2: Invoke golang-system-architect for Fixes
Use Task tool:

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

### Step 5.1: Invoke test-scenario-architect
Use Task tool:

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

1. Invoke golang-system-architect:
```
Fix failing tests.

INPUT FILES:
- Test results: $SESSION_DIR/04-testing/test-results.md
- Test plan: $SESSION_DIR/04-testing/test-plan.md

OUTPUT FILES:
- $SESSION_DIR/04-testing/fixes-iteration-{N}.md

Return brief status.
```

2. Re-run test-scenario-architect (update test-results and test-summary)

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
6. **Parallel execution**: Run external reviews in one message with multiple Bash calls
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
