# Parallel Multi-Model Code Review - Execution Guide

**Purpose**: Execute code reviews with multiple AI models in parallel (one-shot execution)

**Session**: Based on 20251119-235621 (Phase 6 Lambda Functions)

---

## Problem: Why One-Shot Execution Failed

**User Request**: "Run internal and external reviewers in parallel, then consolidate"

**What Happened**: Required 2 user prompts (reviewers → consolidate)

**What Should Happen**: Complete workflow with 1 user prompt

---

## Root Causes Identified

### Issue #1: Mixed Tool Types
- **Wrong**: Bash + Task in same message → Sequential execution
- **Correct**: Separate prep (Bash) from execution (Task only)

### Issue #2: No Auto-Consolidation
- **Wrong**: Wait for user to say "consolidate"
- **Correct**: Automatically consolidate after N reviews

### Issue #3: Background Claudish
- **Wrong**: Spawn claudish as background process → Returns too early
- **Correct**: Block on claudish execution → Wait for response

---

## Solution: State Machine Protocol

```
State 1: PREP
  Action: mkdir (Bash only)
  Transition: → State 2

State 2: PARALLEL_REVIEW
  Action: Launch N reviewers (Task only, single message)
  Transition: → State 3 when all summaries received

State 3: AUTO_CONSOLIDATE
  Action: Launch consolidation (NO user prompt)
  Transition: → State 4

State 4: PRESENT
  Action: Show results
  Transition: → DONE
```

**Critical**: State 2 → State 3 is AUTOMATIC

---

## Implementation Checklist

### Before Execution
- [ ] User requests multiple reviewers
- [ ] Identify models needed (internal + external)
- [ ] Determine session directory path

### Execution
- [ ] Message 1: Create directories (Bash mkdir)
- [ ] Message 2: Launch ALL reviewers (Task only)
- [ ] Message 3: Auto-consolidate (no user prompt)
- [ ] Message 4: Present results

### After Execution
- [ ] Verify all review files created
- [ ] Verify consolidated review exists
- [ ] Verify working tree clean

---

## Testing: One-Shot Execution Validation

**Test Case**:
```
User: "Run internal and external reviewers (grok, minimax, codex)"
```

**Expected Behavior**:
- Total messages: 4 (prep, parallel, consolidate, present)
- User prompts: 1 (initial request only)
- Reviewers: 4 (internal + 3 external)
- Consolidated review: Automatic

**Success Criteria**:
- ✅ User makes ONE request
- ✅ System completes full workflow
- ✅ Consolidation happens automatically
- ✅ Results presented without additional prompts

---

## Session Reference

**Full analysis**: `ai-docs/sessions/20251119-235621/execution-analysis.md`

**Key metrics**:
- Reviewers: 5 (1 internal + 4 external)
- Reviews: Parallel execution (single message)
- Consolidation: Manual (should be automatic)
- Execution: Mixed tools (should separate prep from execution)
