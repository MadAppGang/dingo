# Claudish Timeout Configuration Update

**Date**: 2025-11-18
**Status**: Documentation
**Requested**: 15-minute timeout
**Maximum Available**: 10 minutes (Bash tool limit)

## Background

When agents use claudish in PROXY MODE, they execute claudish commands via the Bash tool. The Bash tool has timeout limits:

- **Default**: 120000ms (2 minutes)
- **Current (estimated)**: 300000ms (5 minutes)
- **Requested**: 900000ms (15 minutes)
- **Maximum**: 600000ms (10 minutes) ⚠️

**IMPORTANT**: The Bash tool has a hard maximum of 10 minutes (600000ms). We cannot set a 15-minute timeout with the current tool. **We can increase to 10 minutes maximum.**

## Recommendation

**Set claudish timeouts to 10 minutes (600000ms)** - the maximum available.

This provides:
- Current: 5 minutes → New: 10 minutes (2x increase)
- Sufficient for most external model review tasks
- Within Bash tool limits

If 15 minutes is truly needed, we would need to:
1. Request an increase to Bash tool maximum timeout, OR
2. Use a different execution mechanism for long-running claudish tasks

## Where to Update

### 1. Agent Instructions for Proxy Mode

**Files to Update**:
- `.claude/agents/code-reviewer.md`
- `.claude/agents/golang-developer.md`
- `langingpage/.claude/agents/astro-reviewer.md`
- `langingpage/.claude/agents/astro-developer.md`

**Pattern to Add**:

When agents execute claudish via Bash tool in proxy mode, they should specify the timeout:

```markdown
### Proxy Mode Timeout Configuration

When executing claudish commands for external model delegation:

**IMPORTANT**: Always specify timeout parameter for Bash tool:
- **Timeout**: 600000ms (10 minutes maximum)
- **Why**: External model reviews can take 5-10 minutes for complex code

Example:
```bash
Bash tool with timeout:
- command: claudish --model x-ai/grok-code-fast-1 "..."
- timeout: 600000  # 10 minutes
```

**Never** use default timeout (2 minutes) for claudish proxy tasks - they will fail mid-execution.
```

### 2. Slash Command Documentation

**Files to Update**:
- `.claude/commands/dev.md`
- `langingpage/.claude/commands/astro-dev.md`

**Section to Add** (in External Review instructions):

```markdown
### External Review Timeout

When code-reviewer or astro-reviewer agents execute claudish in proxy mode:

**Bash Tool Configuration**:
```json
{
  "command": "claudish --model [model-id] ...",
  "timeout": 600000,
  "description": "External review via [model-name]"
}
```

**Why 10 minutes**:
- External models may take 5-10 minutes for thorough code review
- Covers model processing time + network latency
- Maximum allowed by Bash tool
```

### 3. Documentation Updates

**File**: `CLAUDE.md` - Delegation Strategy Section

Add note about timeout best practices:

```markdown
### Timeout Configuration for External Tools

When delegating to external models via claudish:

**Bash Tool Timeout**:
- Default: 2 minutes (too short for external reviews)
- Recommended: 10 minutes (600000ms) - maximum available
- Use case: External model code reviews, complex analysis

**Example**:
```python
# In agent using Bash tool for claudish
Bash(
    command='claudish --model x-ai/grok-code-fast-1 "Review task"',
    timeout=600000,  # 10 minutes
    description='External code review via Grok'
)
```

**Note**: If tasks require more than 10 minutes, consider:
- Breaking into smaller subtasks
- Using async/background processing
- Requesting Bash tool timeout increase
```

## Implementation Steps

### Step 1: Update code-reviewer.md

Location: `.claude/agents/code-reviewer.md`

Find the "Proxy Mode" section (around line 100-150) and add:

```markdown
### Proxy Mode Execution with Timeout

When executing claudish in proxy mode, ALWAYS specify timeout:

**CRITICAL**: Use Bash tool with timeout parameter:
```python
Bash(
    command='claudish --model [model-id] << \'EOF\'\n[prompt]\nEOF',
    timeout=600000,  # 10 minutes (maximum)
    description='External review via [model-name]'
)
```

**Why 10 minutes**:
- External models process slowly (3-8 minutes typical)
- Complex reviews may require full 10 minutes
- Default 2-minute timeout will fail mid-review

**Example**:
```bash
# DON'T: Uses default 2-minute timeout (will fail)
claudish --model x-ai/grok-code-fast-1 "Review code..."

# DO: Specifies 10-minute timeout
Bash tool:
  command: claudish --model x-ai/grok-code-fast-1 "Review code..."
  timeout: 600000
```
```

### Step 2: Update golang-developer.md

Location: `.claude/agents/golang-developer.md`

Find "Proxy Delegation" section (around line 90-140) and add similar timeout guidance:

```markdown
### Timeout for Proxy Delegation

When using claudish in proxy mode for complex tasks:

**Bash Tool Configuration**:
- **timeout**: 600000 (10 minutes maximum)
- **Why**: External models need 5-10 minutes for implementation tasks

**Example claudish invocation**:
```bash
# Via Bash tool with timeout
Bash(
    command='claudish --model google/gemini-pro "Implement feature X"',
    timeout=600000,
    description='External implementation via Gemini'
)
```

**Tasks requiring long timeout**:
- Code generation (>100 lines)
- Architecture design
- Complex refactoring
- Performance optimization analysis
```

### Step 3: Update Astro Agents

**Files**:
- `langingpage/.claude/agents/astro-developer.md`
- `langingpage/.claude/agents/astro-reviewer.md`

Add similar timeout guidance for proxy mode execution.

### Step 4: Update Slash Commands

**File**: `.claude/commands/dev.md`

In the "External Review Task" section (around line 280-340), add:

```markdown
**CRITICAL - Timeout Configuration**:

When code-reviewer agent executes claudish in PROXY MODE:
```python
Bash(
    command='claudish --model [model-id] ...',
    timeout=600000,  # 10 minutes maximum
    description='External review via [model-name]'
)
```

This timeout is REQUIRED because:
- External reviews take 5-10 minutes
- Default 2-minute timeout will fail
- 10 minutes is maximum available
```

**File**: `langingpage/.claude/commands/astro-dev.md`

Similar update in external review section (around line 440-550).

## Quick Reference: Timeout Values

| Scenario | Timeout (ms) | Timeout (min) | Rationale |
|----------|--------------|---------------|-----------|
| Simple bash commands | 120000 | 2 min | Default (usually sufficient) |
| Git operations | 120000 | 2 min | Fast local operations |
| Test suite execution | 300000 | 5 min | Moderate test suites |
| **Claudish external review** | **600000** | **10 min** | **External model processing** |
| **Claudish implementation** | **600000** | **10 min** | **Complex code generation** |
| Build processes | 300000-600000 | 5-10 min | Depends on project size |

## Testing After Update

### Test 1: External Review Timeout

```bash
# Should complete without timeout (even if takes 8 minutes)
Task → code-reviewer (proxy mode):
  Model: x-ai/grok-code-fast-1
  Task: Review complex file (500+ lines)
  Expected: Completes in 6-9 minutes
  With 600000ms timeout: ✅ PASS
  With 300000ms timeout: ❌ FAIL (timeout)
```

### Test 2: Multiple External Reviews in Parallel

```bash
# All should complete even if each takes 7-8 minutes
Parallel launch (single message):
  - Grok review (timeout: 600000)
  - Gemini review (timeout: 600000)
  - GPT-4 review (timeout: 600000)

Expected: All complete in ~8-9 minutes (parallel)
```

## Summary

**Action Items**:
1. ✅ Document maximum available timeout (10 minutes, not 15)
2. ⏳ Update all agent files with timeout guidance
3. ⏳ Update slash commands with timeout requirements
4. ⏳ Add to CLAUDE.md delegation strategy
5. ⏳ Test with actual external reviews

**Key Points**:
- **Requested**: 15 minutes
- **Maximum Available**: 10 minutes (Bash tool limit)
- **Recommendation**: Use 10 minutes (600000ms) for all claudish proxy tasks
- **Current Default**: 2 minutes (too short, causes failures)
- **Increase**: 2 min → 10 min (5x increase in available time)

**If 15 minutes truly needed**:
- File feature request for Bash tool timeout increase
- Consider alternative execution mechanisms
- Break long tasks into smaller subtasks

---

**Next Step**: Update all agent files and slash commands with explicit timeout=600000 for claudish proxy mode execution.
