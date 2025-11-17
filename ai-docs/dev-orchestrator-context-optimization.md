# /dev Orchestrator Context Optimization

**Date**: 2025-11-18
**Status**: Implemented

## Problem

The `/dev` orchestrator was flooding its context window by reading full agent output files:

```
# OLD WORKFLOW (BAD)
1. Agent writes full review → file (5000 lines)
2. Orchestrator reads file → loads 5000 lines into context
3. Repeat for 3 reviewers → 15,000 lines in orchestrator context
4. Context filled with content instead of coordination logic
```

**Impact**: Wasted tokens, slower performance, limited orchestrator reasoning capacity.

## Solution

**File-based communication with message-based summaries**:

```
# NEW WORKFLOW (GOOD)
1. Agent writes full review → file (5000 lines)
2. Agent returns brief summary → message (3 lines)
3. Orchestrator uses summary only, never reads file
4. Full content stays in files for audit trail
```

## Implementation

### Agent Prompt Pattern

All agent prompts now follow this structure:

```markdown
YOUR TASK:
[Detailed task description]

OUTPUT FILES (write full details here):
- path/to/detailed-output.md - Complete detailed content

RETURN MESSAGE (keep this brief - max 3 lines):
Return ONLY this format:
STATUS: [result]
METRICS: [key metrics]
Full details: path/to/detailed-output.md
```

### Orchestrator Rules

**Critical Rule #1**: NEVER read agent output files

```markdown
❌ DO NOT use Read tool on:
  - Review files (*-review.md)
  - Test results (test-results.md)
  - Implementation notes (task-*-notes.md)

✅ DO use Read tool for:
  - Session state (session-state.json)
  - User input files
  - Plan summaries (written by orchestrator itself)

**Exception**: Only read files written by YOU (the orchestrator), never by agents.
```

**Critical Rule #2**: Agents return brief summaries

All agents MUST return max 3-line summaries in their final message. Full details go to files.

## Examples

### Code Review

**Agent Prompt** (`/dev` Step 3.3):
```
OUTPUT FILES (write full details here):
- $REVIEW_ITER/internal-review.md - Detailed review

RETURN MESSAGE (keep this brief - max 3 lines):
Return ONLY this format:
STATUS: [APPROVED or CHANGES_NEEDED]
CRITICAL: N | IMPORTANT: N | MINOR: N
Full review: $REVIEW_ITER/internal-review.md
```

**Agent Returns** (in final message):
```
STATUS: CHANGES_NEEDED
CRITICAL: 2 | IMPORTANT: 5 | MINOR: 8
Full review: ai-docs/sessions/20251117-233209/03-reviews/iteration-01/internal-review.md
```

**Orchestrator** (Step 3.4):
- Parses the 3-line return message above
- Does NOT read the review file
- Displays summary to user
- Continues workflow

### Testing

**Agent Prompt** (`/dev` Step 5.1):
```
OUTPUT FILES (write full details here):
- $SESSION_DIR/04-testing/test-results.md - Detailed test output

RETURN MESSAGE (keep this brief - max 3 lines):
Return ONLY this format:
Tests: [PASS or FAIL]
Results: Passed N/M tests
Full details: $SESSION_DIR/04-testing/test-results.md
```

**Agent Returns**:
```
Tests: FAIL
Results: Passed 35/38 tests
Full details: ai-docs/sessions/20251117-233209/04-testing/test-results.md
```

**Orchestrator**:
- Sees FAIL status from message
- Does NOT read test-results.md
- Invokes fix agent with file path
- Fix agent reads the file (not orchestrator)

## Benefits

### Context Window Efficiency
- **Before**: 15k-50k tokens of agent outputs in orchestrator context
- **After**: ~100-500 tokens of brief summaries in orchestrator context
- **Savings**: 95-99% reduction in orchestrator context usage

### Parallelization Capacity
- More context available for parallel agent coordination
- Can run more agents simultaneously
- Faster overall workflow execution

### Audit Trail
- Full details preserved in session directory
- Easy to review detailed outputs later
- Clean separation between coordination and content

### Agent Independence
- Agents read/write their own files
- Orchestrator never touches agent content
- Clear boundaries and responsibilities

## Pattern Summary

**Golden Rule**: Agents communicate through TWO channels:

1. **Files** (detailed content) - For audit, human review, agent-to-agent handoff
2. **Messages** (brief summaries) - For orchestrator coordination only

**Orchestrator Role**: Coordination and workflow, not content processing.

**Agent Role**: Read inputs from files, write outputs to files, return brief status.

## Related Files

- `/Users/jack/mag/dingo/.claude/commands/dev.md` - Updated orchestrator workflow
- `/Users/jack/mag/dingo/.claude/agents/code-reviewer.md` - Updated agent instructions
- Similar updates needed for: `golang-developer`, `golang-architect`, `golang-tester`

## Next Steps

1. ✅ Update `/dev` orchestrator (done)
2. ✅ Update `code-reviewer` agent (done)
3. ⏳ Update `golang-developer` agent (if needed)
4. ⏳ Update `golang-architect` agent (if needed)
5. ⏳ Update `golang-tester` agent (if needed)
6. ⏳ Test with real workflow
7. ⏳ Monitor context usage metrics

## Metrics to Track

When testing this new workflow, measure:
- Orchestrator context usage (tokens)
- Total workflow completion time
- Agent parallelization effectiveness
- User satisfaction with summaries vs full content
