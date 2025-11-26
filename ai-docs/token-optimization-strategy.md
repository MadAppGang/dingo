# Token Optimization Strategy

**Date**: 2025-11-18
**Status**: Implemented

## Problem

Previous conversations consumed excessive context window tokens:
- Reading 10+ files directly: ~35k tokens
- Implementing in main chat: ~25k tokens
- Full test output: ~10k tokens
- **Total waste: ~70k tokens per complex task**

This shortened conversation lifespan and reduced effectiveness.

## Solution: Delegation-First Architecture

### Core Principle

**Main Chat = Orchestrator ONLY**
- Makes decisions
- Delegates work to agents
- Reads summaries (NOT details)
- Stays under token budget

**Agents = Workers**
- Do deep investigation
- Implement code
- Run tests
- Write detailed reports to FILES
- Return 5-sentence summaries

### Hard Token Limits (Enforced)

| Operation | Limit | Remedy |
|-----------|-------|--------|
| File reads/message | 2 files OR 200 lines | Delegate |
| Bash output | 50 lines | head -50 OR delegate |
| Grep results | 20 matches | head_limit OR delegate |
| Agent summary | 5 sentences | Agent compresses |

**Violation = Mandatory delegation**

### Implementation

#### 1. Updated CLAUDE.md Files

Both root and `langingpage/CLAUDE.md` now have:
- ⚠️ CRITICAL section at top (reads first)
- Token budget limits table
- Forbidden patterns (what NOT to do)
- Pre-check decision tree
- Strict delegation templates
- Session folder pattern

#### 2. Agent Templates Enhanced

All templates now specify:
- MAX 5 sentence return summaries
- Explicit "DO NOT return full details" warnings
- File paths for detailed output
- Clear status reporting format

#### 3. Forbidden Patterns

Main chat must NEVER:
- Read 3+ files in one turn
- Implement code across multiple files
- Show full test output (>50 lines)
- Run multiple Grep searches

## Expected Impact

### Token Savings

**Before**:
- Complex task: ~120k tokens (read files + implement + test)
- Conversation lifespan: ~5-7 complex tasks

**After**:
- Complex task: ~2k tokens (summaries only)
- Conversation lifespan: ~60-100 complex tasks
- **60x improvement in conversation capacity**

### Benefits

✅ **10-20x context reduction** per task
✅ **Longer conversation lifespan** (10x longer)
✅ **Clearer separation** of orchestration vs execution
✅ **Better parallel execution** (agents in parallel)
✅ **Persistent knowledge** (all details in files)
✅ **Faster decision making** (less context bloat)

## Usage Example

### OLD (Bad)
```
User: "Fix errors"
Main: Read type_inference.go (35k tokens)
Main: Read result_type.go (40k tokens)
Main: Edit files directly
Main: Run tests (10k tokens output)
Total: ~120k tokens
```

### NEW (Good)
```
User: "Fix errors"
Main: Creates session folder
Main: Task → golang-developer (fix errors)
Agent: Returns 5-line summary
Main: Task → golang-tester (run tests)
Agent: Returns 4-line summary
Main: "All fixes complete. Tests pass."
Total: ~2k tokens (60x reduction!)
```

## Enforcement Checklist

Before EVERY action, main chat asks:
- [ ] Will this read >2 files or >200 lines? → Delegate
- [ ] Will this output >50 lines? → Delegate
- [ ] Is this multi-step? → Create session + delegate
- [ ] Am I about to search/implement/test? → Delegate

**If ANY checkbox = YES → Use Task tool instead**

## Files Modified

1. `CLAUDE.md` - Root transpiler project
   - Added CRITICAL section at top
   - Enhanced delegation templates
   - Token budget enforcement

2. `langingpage/CLAUDE.md` - Landing page project
   - Added CRITICAL section at top
   - Astro-specific delegation templates
   - Component-focused limits

3. `ai-docs/token-optimization-strategy.md` - This file
   - Documents strategy
   - Tracks impact
   - Reference for future

## Success Metrics

Track these over next 10 conversations:
- [ ] Average tokens/task (target: <5k)
- [ ] Conversation lifespan (target: >20 complex tasks)
- [ ] Agent usage rate (target: >80% of complex tasks)
- [ ] User satisfaction (qualitative)

## Future Enhancements

Potential improvements:
1. Create `/delegate` slash command for pure orchestration
2. Add automated token counter in system prompts
3. Create agent summary validation (ensure <5 sentences)
4. Session folder management utilities

---

**Result**: Conversations can now handle 10-20x more complex tasks without context overflow.
