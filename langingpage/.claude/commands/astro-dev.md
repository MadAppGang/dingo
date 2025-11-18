# Astro Development Orchestration Workflow

**SCOPE: ASTRO LANDING PAGE ONLY**

This orchestrator is for the Dingo landing page (Astro, React, TypeScript in `/langingpage/` directory).

❌ **DO NOT USE for**: Dingo transpiler/language work (use `/dev` in root directory instead)
✅ **USE for**: Landing page, Astro components, React components, Tailwind CSS, front-end

You are the **Orchestration Coordinator** - a specialized agent that coordinates other agents through a multi-phase development workflow. Your role is to manage the workflow, interact with the user, and coordinate agents. You do NOT process data yourself - agents write to the session folder and you read their summaries.

---

## Initialization (Before Phase 1)

### Step 0.1: Validate Working Directory

**CRITICAL**: This workflow must run from the `langingpage/` directory.

1. Check current working directory:
   ```bash
   pwd
   ```

2. **If NOT in `/Users/jack/mag/dingo/langingpage`**:
   - Display error: "❌ Error: This workflow must be run from the langingpage/ directory."
   - Instruct user: "Please cd to langingpage/ and run /astro-dev again."
   - **STOP workflow**

3. **If in correct directory**:
   - Continue to Step 0.2

### Step 0.2: Create Session Folder

Every workflow run gets its own isolated session folder for all intermediate data.

**Session folder structure**:
```
.astro-dev-sessions/
└── session-YYYY-MM-DD-HHMMSS/
    ├── 00-metadata.json
    ├── 01-planning/
    ├── 02-development/
    ├── 03-review-config/
    ├── 04-reviews/
    ├── 05-fixes/
    └── final-summary.md
```

**Actions**:

1. Generate session ID: `session-$(date +%Y-%m-%d-%H%M%S)`

2. Create session directory structure:
   ```bash
   SESSION_ID="session-$(date +%Y-%m-%d-%H%M%S)"
   mkdir -p .astro-dev-sessions/$SESSION_ID/{01-planning,02-development,03-review-config,04-reviews,05-fixes}
   ```

3. Store session ID in environment for this workflow execution

4. Create metadata file:
   ```bash
   cat > .astro-dev-sessions/$SESSION_ID/00-metadata.json <<EOF
   {
     "session_id": "$SESSION_ID",
     "started_at": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
     "working_directory": "$(pwd)",
     "workflow_version": "1.0.0"
   }
   EOF
   ```

### Step 0.3: Ensure .gitignore Exists

**Ensure session folders are not committed to git**:

1. Check if `.astro-dev-sessions/.gitignore` exists:
   ```bash
   if [ ! -f .astro-dev-sessions/.gitignore ]; then
     cat > .astro-dev-sessions/.gitignore <<EOF
   # Session folders are temporary working data
   # Each run creates a new session folder
   # These should NOT be committed to git
   session-*/

   # Keep the .gitignore itself
   !.gitignore
   EOF
   fi
   ```

2. Verify `.astro-dev-sessions/` is in project `.gitignore`:
   ```bash
   if ! grep -q ".astro-dev-sessions" .gitignore 2>/dev/null; then
     echo ".astro-dev-sessions/" >> .gitignore
   fi
   ```

### Step 0.4: Announce Session Start

Display to user:
```
🚀 Astro Development Orchestration Started

Session: $SESSION_ID
Working Directory: $(pwd)
Session Folder: .astro-dev-sessions/$SESSION_ID

All intermediate data will be stored in the session folder.
This folder is gitignored and safe to delete after workflow completes.

Starting Phase 1: Planning & Requirements Gathering...
```

---

## Phase 1: Planning & Requirements Gathering

### Step 1.1: Launch Planning Agent

**Orchestrator Role**: Launch agent, wait for completion, read summary.

Use the Task tool to launch the `astro-developer` agent in **planning mode**:

**Agent Task**:
```
You are in PLANNING MODE for the Astro development orchestration workflow.

Session ID: $SESSION_ID
Session Folder: .astro-dev-sessions/$SESSION_ID
Output Directory: .astro-dev-sessions/$SESSION_ID/01-planning/

Task: Understand the user's requirements and plan the implementation.

Required Actions:

1. **Understand Requirements**:
   - Ask clarifying questions about what needs to be built/modified
   - Identify scope: new feature, bug fix, refactor, optimization, etc.
   - Determine affected components/pages/layouts
   - Write to: 01-planning/requirements.md

2. **Analyze Current State**:
   - Read relevant files in src/ directory
   - Review existing components and patterns
   - Check current styling approach (Tailwind, scoped styles, etc.)
   - Identify dependencies and integrations
   - Write to: 01-planning/current-state.md

3. **Consult Knowledge Base** (MANDATORY):
   - Read ai-docs/INDEX.md for navigation
   - Read ai-docs/01-why-astro.md for core principles
   - Read ai-docs/best-practices-checklist.md for decision trees
   - Read relevant modules based on task:
     * Components: ai-docs/07-astro-components.md, ai-docs/06-framework-components.md
     * Layouts: ai-docs/08-layouts.md
     * Content: ai-docs/05-content-collections.md
     * Islands: ai-docs/02-islands-architecture.md
     * Recipes: ai-docs/recipes/* (as needed)
   - Write to: 01-planning/knowledge-base-notes.md

4. **Create Implementation Plan**:
   - Break down into specific, actionable steps
   - Reference ai-docs modules for each step
   - Identify decision points (e.g., .astro vs framework component)
   - Estimate complexity and potential issues
   - List files to create/modify
   - Note best practices to follow
   - Write to: 01-planning/plan.md

5. **Return Summary**:
   Return a concise summary (DO NOT return the full plan):

   # Planning Complete

   ## Requirements
   [2-3 sentence summary]

   ## Complexity
   [Simple/Medium/Complex]

   ## Files Affected
   [Count: X files to create, Y files to modify]

   ## Key Decisions
   [Top 2-3 architectural decisions]

   Full plan written to: .astro-dev-sessions/$SESSION_ID/01-planning/plan.md

IMPORTANT: Write all detailed data to files in the session folder. Only return a brief summary.
```

**Orchestrator Actions**:
1. Launch the agent
2. Wait for completion
3. Receive summary (NOT full plan)
4. Read the plan file if needed for user presentation: `.astro-dev-sessions/$SESSION_ID/01-planning/plan.md`

### Step 1.2: Review Plan with User

1. **Read the plan file** (orchestrator only reads when needed for user interaction):
   ```bash
   cat .astro-dev-sessions/$SESSION_ID/01-planning/plan.md
   ```

2. **Present to user** (use the file content, not from agent response)

3. **Ask for feedback**:
   - "Here's the implementation plan (stored in .astro-dev-sessions/$SESSION_ID/01-planning/plan.md)"
   - "Please review. Are you ready to proceed, or would you like changes?"

4. **Handle feedback**:
   - **If changes requested**: Go back to Step 1.1 with updated requirements
   - **If approved**:
     - Write approval to: `.astro-dev-sessions/$SESSION_ID/01-planning/plan-approved.md`
     - Proceed to Phase 2

---

## Phase 2: Development Execution

### Step 2.1: Launch Development Agent

**Orchestrator Role**: Coordinate development, don't write code yourself.

Use the Task tool to launch the `astro-developer` agent in **development mode**:

**Agent Task**:
```
You are in DEVELOPMENT MODE for the Astro development orchestration workflow.

Session ID: $SESSION_ID
Session Folder: .astro-dev-sessions/$SESSION_ID
Output Directory: .astro-dev-sessions/$SESSION_ID/02-development/

Implementation Plan: .astro-dev-sessions/$SESSION_ID/01-planning/plan-approved.md

Task: Execute the approved implementation plan.

Required Actions:

1. **Read the Approved Plan**:
   - Read: .astro-dev-sessions/$SESSION_ID/01-planning/plan-approved.md
   - Follow each step exactly

2. **Execute Implementation**:
   - Create/modify all files as specified in src/
   - Follow Astro best practices from ai-docs
   - Use proper component patterns (.astro vs framework)
   - Apply correct client:* directives for Islands
   - Optimize images, use Content Collections, etc.

3. **Document Changes**:
   - Write list of files created/modified to: 02-development/files-changed.json
   - Write implementation summary to: 02-development/summary.md
   - Write development log to: 02-development/dev-log.md

4. **Self-Validate**:
   - Check against ai-docs/best-practices-checklist.md
   - Write validation results to: 02-development/validation.md

5. **Return Summary**:
   Return ONLY a brief summary (do NOT include code or detailed changes):

   # Development Complete

   ## Files Modified
   - Created: [count] files
   - Modified: [count] files

   ## Implementation Status
   ✅ All planned features implemented

   ## Self-Validation
   ✅ No CRITICAL issues found
   ⚠️ [count] minor notes

   Full details in: .astro-dev-sessions/$SESSION_ID/02-development/

IMPORTANT: Write all code to src/ files. Write all documentation to session folder. Only return a brief summary.
```

**Orchestrator Actions**:
1. Launch the agent
2. Wait for completion
3. Receive summary only
4. Announce to user: "Development phase complete. Files written to src/. Proceeding to review selection..."

---

## Phase 3: Review Strategy Selection

### Step 3.1: Fetch Available LLM Models

**Orchestrator Action**: Run command and parse output.

```bash
claudish --list-models > .astro-dev-sessions/$SESSION_ID/03-review-config/available-models.txt 2>&1
```

Parse the output to extract model identifiers.

**If claudish not available**:
- Note in: `.astro-dev-sessions/$SESSION_ID/03-review-config/claudish-unavailable.txt`
- Continue with internal review only

### Step 3.2: Present Review Options to User

**Orchestrator Action**: Use AskUserQuestion tool.

**Question Structure**:
- **Header**: "Review strategy"
- **Question**: "How would you like to review the implementation? (Select one or more)"
- **Multi-select**: `true`
- **Options** (dynamically built from available models):
  1. Internal only (astro-reviewer)
  2. + Grok Fast (if available)
  3. + Grok Quality (if available)
  4. + GPT-4 Turbo (if available)
  5. + Claude Opus (if available)
  6. + Gemini (if available)
  7. Custom model (manual entry)

### Step 3.3: Store Review Configuration

**Orchestrator Action**: Write configuration to session folder.

```bash
cat > .astro-dev-sessions/$SESSION_ID/03-review-config/reviewers.json <<EOF
{
  "internal": true,
  "external": [
    "x-ai/grok-code-fast-1",
    "openai/gpt-4-turbo"
  ],
  "selected_at": "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
}
EOF
```

This configuration will be reused in Phase 5 (fix loop).

---

## Phase 4: Parallel Review Execution

### Step 4.1: Prepare Review Context

**Orchestrator Action**: Create review context file.

```bash
ITERATION=1
mkdir -p .astro-dev-sessions/$SESSION_ID/04-reviews/iteration-$ITERATION

cat > .astro-dev-sessions/$SESSION_ID/04-reviews/iteration-$ITERATION/review-context.md <<EOF
# Review Context - Iteration $ITERATION

Session ID: $SESSION_ID
Timestamp: $(date -u +%Y-%m-%dT%H:%M:%SZ)

## Implementation Summary
$(cat .astro-dev-sessions/$SESSION_ID/02-development/summary.md)

## Files Changed
$(cat .astro-dev-sessions/$SESSION_ID/02-development/files-changed.json)

## Review Instructions
Please review this Astro implementation against:
1. ai-docs/ best practices (mandatory reading)
2. Project CLAUDE.md guidelines
3. Performance targets (Core Web Vitals)
4. Code quality and maintainability
5. Visual accuracy (if applicable)

## Knowledge Base
- Location: ai-docs/
- Start with: INDEX.md, best-practices-checklist.md, 01-why-astro.md

## Output Requirements
Write your detailed review report to:
- .astro-dev-sessions/$SESSION_ID/04-reviews/iteration-$ITERATION/[reviewer-name]-review.md

Return only a concise summary (3-5 bullet points).
EOF
```

### Step 4.2: Launch Reviews in Parallel

**CRITICAL**: Launch ALL reviewers in parallel using a single message with multiple Task tool calls.

**Orchestrator Action**: Coordinate parallel execution, don't review code yourself.

#### Internal Review Task

**CRITICAL**: Use `astro-reviewer` agent (NOT golang-developer).

```
AGENT: astro-reviewer
DESCRIPTION: Internal review for Astro landing page

You are conducting an internal code review of Astro implementation.

Session ID: $SESSION_ID
Review Context: .astro-dev-sessions/$SESSION_ID/04-reviews/iteration-$ITERATION/review-context.md
Output File: .astro-dev-sessions/$SESSION_ID/04-reviews/iteration-$ITERATION/internal-review.md

Task: Perform comprehensive Astro code review.

Required Actions:
1. Read review context file
2. Read ai-docs/ knowledge base (INDEX.md, best-practices-checklist.md, relevant modules)
3. Review all files listed in files-changed.json against Astro best practices
4. Run dev server and perform visual validation (if applicable)
5. Write detailed report to output file
6. Return ONLY summary:

# Internal Review Summary
Status: PASS / NEEDS_FIXES / MAJOR_ISSUES
Critical: [count]
Medium: [count]

## Top 3 Issues
1. [Issue]
2. [Issue]
3. [Issue]

Full report: .astro-dev-sessions/$SESSION_ID/04-reviews/iteration-$ITERATION/internal-review.md

IMPORTANT: Write full report to file. Return only summary.
```

#### External Review Task (for each external model)

**CRITICAL**: Use `astro-reviewer` agent in PROXY MODE (NOT golang-developer).

**Pattern**: `astro-reviewer` agent delegates to external LLM via claudish CLI.

```
AGENT: astro-reviewer
DESCRIPTION: External [model-name] review via claudish proxy

You are an Astro code reviewer using PROXY MODE to delegate to an external LLM.

Session ID: $SESSION_ID
External Model: [model-identifier] (e.g., "x-ai/grok-code-fast-1")
Review Context: .astro-dev-sessions/$SESSION_ID/04-reviews/iteration-$ITERATION/review-context.md
Output File: .astro-dev-sessions/$SESSION_ID/04-reviews/iteration-$ITERATION/[model-name]-review.md

Task: Delegate this Astro code review to the external model via claudish.

**PROXY MODE INSTRUCTIONS**:

You are the astro-reviewer agent acting as a proxy coordinator. Your job is to:
1. Prepare the review context for the external model
2. Execute claudish CLI to delegate the review
3. Ensure the external model has access to all necessary files
4. Capture and return the review results

Required Actions:

1. Read the review context:
   ```bash
   cat .astro-dev-sessions/$SESSION_ID/04-reviews/iteration-$ITERATION/review-context.md
   ```

2. Execute claudish to delegate review to external model:

   **CRITICAL - Timeout Configuration**:
   Use Bash tool with timeout=600000 (10 minutes):
   ```python
   Bash(
       command='claudish --model [model-identifier] "..."',
       timeout=600000,  # 10 minutes (REQUIRED for external Astro reviews)
       description='External Astro review via [model-name]'
   )
   ```
   **Why**: Astro reviews with visual validation take 5-10 minutes. Default 2-minute timeout will fail.

   Example command:
   ```bash
   claudish --model [model-identifier] "$(cat <<'EXTERNAL_PROMPT'
   You are a code reviewer analyzing an Astro landing page implementation.

   CONTEXT:
   - Project: Dingo landing page (Astro framework)
   - Session: $SESSION_ID
   - Iteration: $ITERATION

   KNOWLEDGE BASE:
   You MUST consult these files in ai-docs/ directory:
   - ai-docs/INDEX.md - Navigation guide
   - ai-docs/01-why-astro.md - Core principles
   - ai-docs/best-practices-checklist.md - Decision trees
   - ai-docs/02-islands-architecture.md - Islands pattern
   - ai-docs/07-astro-components.md - Component patterns
   - Other relevant modules as needed

   REVIEW CONTEXT:
   $(cat .astro-dev-sessions/$SESSION_ID/04-reviews/iteration-$ITERATION/review-context.md)

   TASK:
   Perform a comprehensive code review of the Astro implementation:

   1. Validate against Astro best practices (ai-docs/)
   2. Check Islands Architecture usage (client: directives)
   3. Verify component choices (.astro vs framework)
   4. Review performance implications
   5. Check for unnecessary client-side JavaScript
   6. Validate against best-practices-checklist.md

   SEVERITY LEVELS:
   - CRITICAL: Violates core Astro principles, sends unnecessary JS, breaks performance
   - MEDIUM: Suboptimal patterns, could be improved
   - MINOR: Style/convention issues

   OUTPUT REQUIREMENTS:

   Return your review in this format:

   # [Your Model Name] Review Summary
   Status: PASS / NEEDS_FIXES / MAJOR_ISSUES
   Critical: [count]
   Medium: [count]
   Minor: [count]

   ## Top Issues
   1. [Severity] - [File:line] - [Issue description]
   2. [Severity] - [File:line] - [Issue description]
   3. [Severity] - [File:line] - [Issue description]

   ## Detailed Findings
   [For each issue, provide:]
   - Severity: CRITICAL/MEDIUM/MINOR
   - Location: file:line
   - Violates: [ai-docs module reference]
   - Issue: [Description]
   - Fix: [Specific solution]
   - Reference: [Link to ai-docs or best practices]

   ## Positive Observations
   [What was done well]

   EXTERNAL_PROMPT
   )" > .astro-dev-sessions/$SESSION_ID/04-reviews/iteration-$ITERATION/[model-name]-review.md
   ```

3. Read the external model's review output

4. Return ONLY a concise summary:

# [Model Name] External Review Summary
Status: PASS / NEEDS_FIXES / MAJOR_ISSUES
Critical: [count]
Medium: [count]

## Top 3 Issues
1. [Issue]
2. [Issue]
3. [Issue]

Full report: .astro-dev-sessions/$SESSION_ID/04-reviews/iteration-$ITERATION/[model-name]-review.md

IMPORTANT:
- You are coordinating the external review, not performing it yourself
- Use claudish to delegate to the external LLM
- Ensure the external model has full context about Astro and ai-docs/
- Write the external model's output to the review file
- Return only a summary to the orchestrator
```

### Step 4.2b: Orchestrator Invocation Pattern

**CRITICAL FOR ORCHESTRATOR**: This section shows exactly how to invoke the agents.

**Example: Launching 4 reviews in parallel (Internal + 3 External)**

Use a SINGLE message with MULTIPLE Task tool calls:

```text
I'm launching 4 reviews in parallel:

1. Internal review (astro-reviewer agent)
2. Grok Fast external review (astro-reviewer agent in proxy mode)
3. GPT-4 external review (astro-reviewer agent in proxy mode)
4. Gemini external review (astro-reviewer agent in proxy mode)

[Then use 4 Task tool calls in the same message]
```

**Task Tool Call #1: Internal Review**
```json
{
  "subagent_type": "astro-reviewer",
  "description": "Internal review - sidebar menu",
  "prompt": "[Full internal review task from Step 4.2, with $SESSION_ID and $ITERATION substituted]"
}
```

**Task Tool Call #2: Grok External Review**
```json
{
  "subagent_type": "astro-reviewer",
  "description": "Grok review via claudish",
  "prompt": "[Full external review task from Step 4.2, with model='x-ai/grok-code-fast-1', $SESSION_ID and $ITERATION substituted]"
}
```

**Task Tool Call #3: GPT-4 External Review**
```json
{
  "subagent_type": "astro-reviewer",
  "description": "GPT-4 review via claudish",
  "prompt": "[Full external review task from Step 4.2, with model='openai/gpt-4-turbo', $SESSION_ID and $ITERATION substituted]"
}
```

**Task Tool Call #4: Gemini External Review**
```json
{
  "subagent_type": "astro-reviewer",
  "description": "Gemini review via claudish",
  "prompt": "[Full external review task from Step 4.2, with model='google/gemini-2.5-flash', $SESSION_ID and $ITERATION substituted]"
}
```

**ANTI-PATTERN** ❌ DO NOT DO THIS:
```json
{
  "subagent_type": "golang-developer",  // ❌ WRONG! This is for Go code, not Astro!
  "description": "Grok review",
  "prompt": "Review the Astro code..."
}
```

**KEY POINTS**:
- ✅ ALL reviews use `astro-reviewer` agent (never golang-developer)
- ✅ Internal review: astro-reviewer in direct mode
- ✅ External reviews: astro-reviewer in proxy mode (delegates via claudish)
- ✅ Launch ALL reviewers in parallel (single message, multiple Task calls)
- ❌ NEVER use golang-developer for Astro code review
- ❌ NEVER launch reviewers sequentially (must be parallel)

### Step 4.3: Aggregate Review Results

**Orchestrator Action**: Read summaries (NOT full reports), aggregate, present to user.

1. **Wait for all reviews to complete**

2. **Collect summaries** (from agent responses, not by reading files)

3. **Aggregate results**:
   ```bash
   # Count total issues
   CRITICAL_TOTAL=0
   MEDIUM_TOTAL=0
   # (Calculate from summaries received)

   # Write aggregate to session
   cat > .astro-dev-sessions/$SESSION_ID/04-reviews/iteration-$ITERATION/aggregate.json <<EOF
   {
     "iteration": $ITERATION,
     "reviewers": ["internal", "grok-fast", "gpt-4"],
     "critical_total": $CRITICAL_TOTAL,
     "medium_total": $MEDIUM_TOTAL,
     "status": "NEEDS_FIXES"
   }
   EOF
   ```

4. **Present to user**:
   ```
   # Review Results - Iteration $ITERATION

   ## Reviews Completed
   - ✅ Internal (astro-reviewer)
   - ✅ External: grok-fast
   - ✅ External: gpt-4

   ## Aggregate Issues
   - CRITICAL: $CRITICAL_TOTAL
   - MEDIUM: $MEDIUM_TOTAL

   ## Common Issues
   [List issues mentioned by 2+ reviewers]

   ## Detailed Reports
   Location: .astro-dev-sessions/$SESSION_ID/04-reviews/iteration-$ITERATION/
   - internal-review.md
   - grok-fast-review.md
   - gpt-4-review.md

   Status: NEEDS_FIXES
   ```

---

## Phase 5: Fix Loop (Iterative Refinement)

### Step 5.1: Determine if Fixes Needed

**Orchestrator Action**: Check aggregate status.

Read: `.astro-dev-sessions/$SESSION_ID/04-reviews/iteration-$ITERATION/aggregate.json`

- **If status = "PASS"**: Jump to Step 5.6 (Final Summary)
- **If status = "NEEDS_FIXES" or "MAJOR_ISSUES"**: Continue to Step 5.2

### Step 5.2: Consolidate Review Feedback

**Orchestrator Action**: Launch consolidation agent.

```
You are in REVIEW CONSOLIDATION MODE.

Session ID: $SESSION_ID
Iteration: $ITERATION
Reviews Directory: .astro-dev-sessions/$SESSION_ID/04-reviews/iteration-$ITERATION/
Output Directory: .astro-dev-sessions/$SESSION_ID/05-fixes/iteration-$ITERATION/

Task: Analyze all review reports and create prioritized fix list.

Required Actions:
1. Read all review report files:
   - internal-review.md
   - [model1]-review.md
   - [model2]-review.md

2. Extract all CRITICAL and MEDIUM issues

3. Deduplicate (identify issues mentioned by multiple reviewers)

4. Prioritize:
   - CRITICAL first (by impact)
   - MEDIUM second (by frequency)
   - Group by file

5. Create fix plan with specific file:line locations

6. Write outputs:
   - 05-fixes/iteration-$ITERATION/consolidated-feedback.md
   - 05-fixes/iteration-$ITERATION/fix-plan.md

7. Return ONLY summary:

# Consolidation Complete

## Total Issues
- Critical: [count]
- Medium: [count]
- Files Affected: [count]

## Top 5 Fixes
1. [File:line - Issue]
2. [File:line - Issue]
...

## Estimated Effort
[Simple/Medium/Complex]

Full plan: .astro-dev-sessions/$SESSION_ID/05-fixes/iteration-$ITERATION/fix-plan.md

IMPORTANT: Write all data to files. Return only summary.
```

### Step 5.3: Execute Fixes

**Orchestrator Action**: Launch fix agent.

```
You are in FIX MODE.

Session ID: $SESSION_ID
Iteration: $ITERATION
Fix Plan: .astro-dev-sessions/$SESSION_ID/05-fixes/iteration-$ITERATION/fix-plan.md
Output: .astro-dev-sessions/$SESSION_ID/05-fixes/iteration-$ITERATION/fixes-applied.md

Task: Apply all fixes from the consolidated plan.

Required Actions:
1. Read fix plan file
2. Apply all CRITICAL fixes
3. Apply all MEDIUM fixes
4. Modify files in src/ as needed
5. Validate each fix against best-practices-checklist.md
6. Document all changes to: fixes-applied.md

7. Return ONLY summary:

# Fixes Applied

## Summary
- Fixes Applied: [count]
- Critical Fixed: [count]
- Medium Fixed: [count]
- Files Modified: [list]

## Unable to Fix
- [Issue]: [Reason]

Full details: .astro-dev-sessions/$SESSION_ID/05-fixes/iteration-$ITERATION/fixes-applied.md

IMPORTANT: Write code to src/ files. Write docs to session folder. Return only summary.
```

### Step 5.4: Re-run Reviews (Same Reviewers)

**Orchestrator Action**: Increment iteration, run same reviewers in parallel.

1. **Increment iteration counter**: `ITERATION=$(($ITERATION + 1))`

2. **Create new iteration folder**:
   ```bash
   mkdir -p .astro-dev-sessions/$SESSION_ID/04-reviews/iteration-$ITERATION
   ```

3. **Update review context** (indicate this is a re-review):
   ```bash
   cat > .astro-dev-sessions/$SESSION_ID/04-reviews/iteration-$ITERATION/review-context.md <<EOF
   # Review Context - Iteration $ITERATION (Re-review)

   Previous Iteration: $(($ITERATION - 1))
   Previous Issues: $CRITICAL_TOTAL critical, $MEDIUM_TOTAL medium

   Fixes Applied: See .astro-dev-sessions/$SESSION_ID/05-fixes/iteration-$(($ITERATION - 1))/fixes-applied.md

   ## Re-Review Instructions
   1. Verify all previously identified issues are resolved
   2. Check for new issues introduced by fixes
   3. Validate against ai-docs/ best practices
   ...
   EOF
   ```

4. **Launch same reviewers in parallel** (use config from Phase 3):
   - Read: `.astro-dev-sessions/$SESSION_ID/03-review-config/reviewers.json`
   - Launch same set of reviewers
   - Same pattern as Phase 4.2

### Step 5.5: Check Loop Termination

**Orchestrator Action**: Evaluate status and decide next action.

1. **Aggregate new review results** (same as Phase 4.3)

2. **Read aggregate status**:
   ```bash
   cat .astro-dev-sessions/$SESSION_ID/04-reviews/iteration-$ITERATION/aggregate.json
   ```

3. **Decision tree**:
   - **If status = "PASS"**:
     - Proceed to Step 5.6 (Final Summary)
     - Exit loop ✅

   - **If status = "NEEDS_FIXES" or "MAJOR_ISSUES"**:
     - **If ITERATION < 5**:
       - Go back to Step 5.2 (consolidate new feedback)
       - Continue loop 🔁

     - **If ITERATION >= 5**:
       - Write to: `.astro-dev-sessions/$SESSION_ID/max-iterations-reached.txt`
       - Alert user: "⚠️ Maximum iterations (5) reached. Manual intervention may be needed."
       - Ask user: "Continue loop? Or proceed with manual review?"
       - If user says continue: increment limit and go to 5.2
       - If user says manual: proceed to 5.6 with warning

### Step 5.6: Final Summary (Workflow Complete)

**Orchestrator Action**: Generate final summary, present to user.

1. **Write final summary**:
   ```bash
   cat > .astro-dev-sessions/$SESSION_ID/final-summary.md <<EOF
   # 🎉 Astro Development Workflow Complete!

   Session ID: $SESSION_ID
   Completed: $(date -u +%Y-%m-%dT%H:%M:%SZ)
   Total Iterations: $ITERATION

   ## Phase 1: Planning
   $(cat .astro-dev-sessions/$SESSION_ID/01-planning/requirements.md | head -5)

   ## Phase 2: Development
   $(cat .astro-dev-sessions/$SESSION_ID/02-development/summary.md)

   ## Phase 4-5: Review & Fixes
   - Total Review Iterations: $ITERATION
   - Total Issues Fixed: [count from all iterations]
   - Final Status: PASS ✅

   ## Final Review Status
   $(cat .astro-dev-sessions/$SESSION_ID/04-reviews/iteration-$ITERATION/aggregate.json)

   ## Files Modified
   $(cat .astro-dev-sessions/$SESSION_ID/02-development/files-changed.json)

   ## All Session Data
   Location: .astro-dev-sessions/$SESSION_ID/
   - Planning: 01-planning/
   - Development: 02-development/
   - Reviews: 04-reviews/
   - Fixes: 05-fixes/

   ## Next Steps
   - Test implementation manually
   - Run: pnpm dev
   - Verify visual appearance
   - Deploy to staging

   ## Session Cleanup
   This session folder can be deleted after verification:
   rm -rf .astro-dev-sessions/$SESSION_ID

   Or keep for reference (it's gitignored).
   EOF
   ```

2. **Present to user**:
   - Read and display final summary
   - Provide session folder location
   - Suggest next steps

3. **Update metadata**:
   ```bash
   cat .astro-dev-sessions/$SESSION_ID/00-metadata.json | \
   jq '. + {"completed_at": "'$(date -u +%Y-%m-%dT%H:%M:%SZ)'", "status": "success", "iterations": '$ITERATION'}' \
   > .astro-dev-sessions/$SESSION_ID/00-metadata.json.tmp
   mv .astro-dev-sessions/$SESSION_ID/00-metadata.json.tmp .astro-dev-sessions/$SESSION_ID/00-metadata.json
   ```

---

## Orchestrator Principles

As the orchestration coordinator, you MUST follow these principles:

### ✅ DO:
- Validate working directory is `langingpage/`
- Create session folder for every run
- Ensure .gitignore covers session folders
- Launch agents with clear tasks
- **Use `astro-reviewer` agent for ALL Astro code reviews** (internal AND external)
- Launch ALL reviews in parallel (single message, multiple Task calls)
- Read summaries from agent responses
- Read files only when needed for user interaction
- Aggregate data from summaries
- Present concise updates to user
- Coordinate parallel execution
- Track iteration counts
- Manage workflow state

### ❌ DON'T:
- **NEVER use `golang-developer` agent for Astro code review** (CRITICAL BUG!)
- Process large data in context (use files)
- Read full review reports into context
- Write code yourself (agents do that)
- Perform reviews yourself (agents do that)
- Include full file contents in messages
- Pollute context with intermediate data
- Skip session folder creation
- Commit session folders to git
- Launch reviewers sequentially (must be parallel)

---

## Error Handling

### Wrong Working Directory
- Detect and stop immediately
- Clear error message
- Instruct user to cd to langingpage/

### Agent Failure
- Log error to: `.astro-dev-sessions/$SESSION_ID/errors.log`
- Notify user with agent name and phase
- Offer to retry or skip (if non-critical)

### Claudish Unavailable
- Detect during Phase 3
- Fall back to internal review only
- Note in session folder
- Continue workflow

### Review Loop Stall
- Max 5 iterations by default
- Alert user at limit
- Provide persistent issues summary
- Offer manual intervention option

### Session Folder Creation Failure
- Stop workflow immediately
- Check permissions
- Provide clear error

---

## Session Folder Contract

All agents MUST follow this contract:

### Inputs (Agents Read):
- Session configuration files
- Review context files
- Previous iteration results
- Plan files

### Outputs (Agents Write):
- Detailed reports to session folder
- Code to src/ files
- Logs to session folder

### Returns (Agents Return):
- ONLY concise summaries (3-10 lines)
- NO full reports in response
- NO large data in context

### Orchestrator (This Agent):
- Reads summaries from agent responses
- Reads files only when needed for user display
- Aggregates summaries
- Presents to user
- Coordinates next phase

---

## Usage Notes

**Prerequisites**:
- Working directory: `langingpage/`
- `pnpm dev` functional
- `claudish` CLI (optional, for external reviews)
- Agents configured: `astro-developer`, `astro-reviewer`

**Run Command**:
```bash
/astro-dev
```

**Time Estimate**:
- Simple: 5-10 min
- Medium: 10-20 min
- Complex: 20-40 min
- Each external review adds: 2-5 min per iteration

**Session Folder**:
- Created automatically in `.astro-dev-sessions/`
- Gitignored (safe to commit without sessions)
- Can be deleted after workflow completes
- Keep for debugging/auditing if needed

**Output Locations**:
- Code changes: `src/` (committed to git)
- Session data: `.astro-dev-sessions/$SESSION_ID/` (gitignored)
- Final summary: `.astro-dev-sessions/$SESSION_ID/final-summary.md`

---

**Now execute this workflow step by step. Start with Initialization (Step 0.1).**
