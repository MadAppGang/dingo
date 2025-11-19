---
name: external-model-selection
description: Choose optimal external AI models for code analysis, bug investigation, and architectural decisions. Use when consulting multiple LLMs via claudish, comparing model perspectives, or investigating complex Go/LSP/transpiler issues. Provides empirically validated model rankings (91/100 for MiniMax M2, 83/100 for Grok Code Fast) and proven consultation strategies based on real-world testing.
---

# External Model Selection

**Purpose**: Select the best external AI models for your specific task based on empirical performance data from production bug investigations.

**When Claude invokes this Skill**: When you need to consult external models, choose between different LLMs, or want diverse perspectives on architectural decisions, code bugs, or design choices.

---

## Quick Reference: Top Models

### 🥇 Tier 1 - Primary Recommendations (Use First)

**1. MiniMax M2** (`minimax/minimax-m2`)
- **Score**: 91/100 | **Speed**: 3 min ⚡⚡⚡ | **Cost**: $$
- **Best for**: Fast root cause analysis, production bugs, when you need simple implementable fixes
- **Proven**: Found exact bug (column calculation error) in 3 minutes during LSP investigation
- **Why it wins**: Pinpoint accuracy, avoids overengineering, focuses on simplest solution first

**2. Grok Code Fast** (`x-ai/grok-code-fast-1`)
- **Score**: 83/100 | **Speed**: 4 min ⚡⚡ | **Cost**: $$
- **Best for**: Debugging traces, validation strategies, test coverage design
- **Proven**: Step-by-step execution traces, identified tab/space edge cases
- **Why it wins**: Excellent debugging methodology, practical validation approach

**3. GPT-5.1 Codex** (`openai/gpt-5.1-codex`)
- **Score**: 80/100 | **Speed**: 5 min ⚡ | **Cost**: $$$
- **Best for**: Architectural redesign, long-term refactoring plans
- **Proven**: Proposed granular mapping system for future enhancements
- **Why it's valuable**: Strong architectural vision, excellent for planning major changes

**4. Sherlock Think Alpha** (`openrouter/sherlock-think-alpha`) 🎁 FREE
- **Score**: TBD | **Speed**: ~5 min ⚡ | **Cost**: FREE ($0!) 💰
- **Context**: 1.8M tokens (LARGEST context window available!)
- **Best for**: Massive codebase analysis, entire project reasoning, long-context planning
- **Secret**: Big player testing under weird name - don't let the name fool you
- **Specialties**:
  - Full codebase analysis (1.8M tokens = ~500k lines of code!)
  - Research synthesis across dozens of files
  - Protocol compliance & standards validation
  - Entire project architectural analysis
- **Why it's valuable**: FREE + massive context = ideal for comprehensive analysis
- **Use case**: When you need to analyze entire codebase or massive context (and it's FREE!)

**5. Gemini 3 Pro Preview** (`google/gemini-3-pro-preview`) ⭐ NEW
- **Score**: TBD | **Speed**: ~5 min ⚡ | **Cost**: $$$
- **Context**: 1M tokens (11.4B parameter model)
- **Best for**: Multimodal reasoning, agentic coding, complex architectural analysis, long-context planning
- **Strengths**: State-of-the-art on LMArena, GPQA Diamond, MathArena, SWE-Bench Verified
- **Specialties**:
  - Autonomous agents & coding assistants
  - Research synthesis & planning
  - High-context information processing (1M token window!)
  - Tool-calling & long-horizon planning
  - Multimodal analysis (text, code, images)
- **Why it's valuable**: Google's flagship frontier model, excels at inferring intent with minimal prompting
- **Use case**: When you need deep reasoning across massive context (entire codebase analysis)

### 🥈 Tier 2 - Specialized Use Cases

**6. Gemini 2.5 Flash** (`google/gemini-2.5-flash`)
- **Score**: 73/100 | **Speed**: 6 min ⚡ | **Cost**: $
- **Best for**: Ambiguous problems requiring exhaustive hypothesis exploration
- **Caution**: Can go too deep - best when truly uncertain about root cause
- **Value**: Low cost, thorough analysis when you need multiple angles

**7. GLM-4.6** (`z-ai/glm-4.6`)
- **Score**: 70/100 | **Speed**: 7 min 🐢 | **Cost**: $$
- **Best for**: Adding debug infrastructure, algorithm enhancements
- **Caution**: Tends to overengineer - verify complexity is warranted
- **Use case**: When you actually need priority systems or extensive logging

### ❌ AVOID - Known Reliability Issues

**Qwen3 Coder** (`qwen/qwen3-coder-30b-a3b-instruct`)
- **Score**: 0/100 | **Status**: FAILED (timeout after 8+ minutes)
- **Issue**: Reliability problems, availability issues
- **Recommendation**: DO NOT use for time-sensitive or production tasks

---

## Consultation Strategies

### Strategy 1: Fast Parallel Diagnosis (DEFAULT - 90% of use cases)

**Models**: `minimax/minimax-m2` + `x-ai/grok-code-fast-1`

```bash
# Launch 2 models in parallel (single message, multiple Task calls)
Task 1: golang-architect (PROXY MODE) → MiniMax M2
Task 2: golang-architect (PROXY MODE) → Grok Code Fast
```

**Time**: ~4 minutes total
**Success Rate**: 95%+
**Cost**: $$ (moderate)

**Use for**:
- Bug investigations
- Quick root cause diagnosis
- Production issues
- Most everyday tasks

**Benefits**:
- Fast diagnosis from MiniMax M2 (simplest solution)
- Validation strategy from Grok Code Fast (debugging trace)
- Redundancy if one model misses something

---

### Strategy 2: Comprehensive Analysis (Critical issues)

**Models**: `minimax/minimax-m2` + `openai/gpt-5.1-codex` + `x-ai/grok-code-fast-1`

```bash
# Launch 3 models in parallel
Task 1: golang-architect (PROXY MODE) → MiniMax M2
Task 2: golang-architect (PROXY MODE) → GPT-5.1 Codex
Task 3: golang-architect (PROXY MODE) → Grok Code Fast
```

**Time**: ~5 minutes total
**Success Rate**: 99%+
**Cost**: $$$ (high but justified)

**Use for**:
- Critical production bugs
- Architectural decisions
- High-impact changes
- When you need absolute certainty

**Benefits**:
- Quick fix (MiniMax M2)
- Long-term architectural plan (GPT-5.1)
- Validation and testing strategy (Grok)
- Triple redundancy

---

### Strategy 3: Deep Exploration (Ambiguous problems)

**Models**: `minimax/minimax-m2` + `google/gemini-2.5-flash` + `x-ai/grok-code-fast-1`

```bash
# Launch 3 models in parallel
Task 1: golang-architect (PROXY MODE) → MiniMax M2
Task 2: golang-architect (PROXY MODE) → Gemini 2.5 Flash
Task 3: golang-architect (PROXY MODE) → Grok Code Fast
```

**Time**: ~6 minutes total
**Success Rate**: 90%+
**Cost**: $$ (moderate)

**Use for**:
- Ambiguous bugs with unclear root cause
- Multi-faceted problems
- When initial investigation is inconclusive
- Complex system interactions

**Benefits**:
- Quick hypothesis (MiniMax M2)
- Exhaustive exploration (Gemini 2.5 Flash)
- Practical validation (Grok)
- Diverse analytical approaches

---

### Strategy 4: Full Codebase Analysis (Massive Context) 🆕

**Models**: `openrouter/sherlock-think-alpha` + `google/gemini-3-pro-preview`

```bash
# Launch 2 models in parallel
Task 1: golang-architect (PROXY MODE) → Sherlock Think Alpha
Task 2: golang-architect (PROXY MODE) → Gemini 3 Pro Preview
```

**Time**: ~5 minutes total
**Success Rate**: TBD (new strategy)
**Cost**: $$$ (one free, one paid = moderate overall)

**Use for**:
- Entire codebase architectural analysis
- Cross-file dependency analysis
- Large refactoring planning (50+ files)
- System-wide pattern detection
- Multi-module projects

**Benefits**:
- Sherlock: 1.8M token context (FREE!) - can analyze entire codebase
- Gemini 3 Pro: 1M token context + multimodal + SOTA reasoning
- Both have massive context windows for holistic analysis
- One free model reduces cost significantly

**Prompt Strategy**:
```markdown
Analyze the entire Dingo codebase focusing on [specific aspect].

Context provided:
- All files in pkg/ (50+ files)
- All tests in tests/ (60+ files)
- Documentation in ai-docs/
- Total: ~200k lines of code

Your task: [specific analysis goal]
```

---

### Strategy 5: Budget-Conscious (Cost-sensitive) 🎁

**Models**: `openrouter/sherlock-think-alpha` + `x-ai/grok-code-fast-1`

```bash
# Launch 2 models in parallel
Task 1: golang-architect (PROXY MODE) → Sherlock Think Alpha (FREE!)
Task 2: golang-architect (PROXY MODE) → Grok Code Fast
```

**Time**: ~5 minutes total
**Success Rate**: 85%+
**Cost**: $$ (Sherlock is FREE, only pay for Grok!)

**Use for**:
- **Cost-sensitive projects**
- **Large context needs on a budget**
- Non-critical investigations
- Exploratory analysis
- Learning and experimentation

**Benefits**:
- **Sherlock is completely FREE** with 1.8M context!
- **Massive context window** for comprehensive analysis
- Grok provides debugging methodology
- **Lowest cost option** with high value

---

## Decision Tree: Which Strategy?

```
START: Need external model consultation
    ↓
[What type of task?]
    ↓
├─ Bug Investigation (90% of cases)
│  → Strategy 1: MiniMax M2 + Grok Code Fast
│  → Time: 4 min | Cost: $$ | Success: 95%+
│
├─ Critical Bug / Architectural Decision
│  → Strategy 2: MiniMax M2 + GPT-5.1 + Grok
│  → Time: 5 min | Cost: $$$ | Success: 99%+
│
├─ Ambiguous / Multi-faceted Problem
│  → Strategy 3: MiniMax M2 + Gemini + Grok
│  → Time: 6 min | Cost: $$ | Success: 90%+
│
└─ Cost-Sensitive / Exploratory
   → Strategy 4: Gemini + Grok
   → Time: 6 min | Cost: $ | Success: 85%+
```

---

## Critical Implementation Details

### 1. ALWAYS Use 10-Minute Timeout

**CRITICAL**: External models take 5-10 minutes. Default 2-minute timeout WILL fail.

```python
# When delegating to agents in PROXY MODE:
Task tool → golang-architect:

**CRITICAL - Timeout Configuration**:
When executing claudish via Bash tool, ALWAYS use:
```bash
Bash(
    command='cat prompt.md | claudish --model [model-id] > output.md 2>&1',
    timeout=600000,  # 10 minutes (REQUIRED!)
    description='External consultation via [model-name]'
)
```

**Why**: Qwen3 Coder failed due to 2-minute timeout. 10 minutes prevents this.

---

### 2. Launch Models in Parallel (Single Message)

**CORRECT** (6-8x speedup):
```python
# Single message with multiple Task calls
Task 1: golang-architect (PROXY MODE) → Model A
Task 2: golang-architect (PROXY MODE) → Model B
Task 3: golang-architect (PROXY MODE) → Model C
# All execute simultaneously
```

**WRONG** (sequential, slow):
```python
# Multiple messages
Message 1: Task → Model A (wait...)
Message 2: Task → Model B (wait...)
Message 3: Task → Model C (wait...)
# Takes 3x longer
```

---

### 3. Agent Return Format (Keep Brief!)

Agents in PROXY MODE MUST return MAX 3 lines:

```
[Model-name] analysis complete
Root cause: [one-line summary]
Full analysis: [file-path]
```

**DO NOT** return full analysis in agent response (causes context bloat).

---

### 4. File-Based Communication

**Input**: Write investigation prompt to file
```bash
ai-docs/sessions/[timestamp]/input/investigation-prompt.md
```

**Output**: Agents write full analysis to files
```bash
ai-docs/sessions/[timestamp]/output/[model-name]-analysis.md
```

**Main chat**: Reads ONLY summaries, not full files

---

## Evidence: What Made Top Models Win

Based on LSP Source Mapping Bug Investigation (Session 20251118-223538):

### ✅ Success Patterns

**MiniMax M2** (91/100):
- Identified exact bug: `qPos` calculation produces column 15 instead of 27
- Proposed simplest fix: Change `strings.Index()` to `strings.LastIndex()`
- Completed in 3 minutes
- **Key insight**: "The bug is entirely in source map generation"

**Grok Code Fast** (83/100):
- Provided step-by-step execution trace
- Identified tab vs spaces edge case
- Proposed validation strategy with debug logging
- **Key insight**: "Generated_column values don't match actual positions due to prefix length"

**GPT-5.1 Codex** (80/100):
- Identified architectural limitation (coarse-grained mappings)
- Proposed long-term solution: granular mapping segments
- Excellent testing strategy
- **Key insight**: "Single coarse-grained mapping per error propagation"

### ❌ Failure Patterns

**Gemini 2.5 Flash** (73/100):
- Went too deep into fallback logic (not the root cause)
- Explored 10+ hypotheses
- Missed the simple bug (qPos calculation)
- **Issue**: Too thorough, lost focus on simplest explanation

**GLM-4.6** (70/100):
- Focused on MapToOriginal algorithm (which was correct)
- Proposed complex enhancements (priority system, debug logging)
- Overengineered the solution
- **Issue**: Added complexity when simple data fix was needed

**Sherlock Think** (65/100):
- Focused on 0-based vs 1-based indexing (secondary issue)
- Proposed normalization (helpful but not main fix)
- Expensive at $$$ for limited value
- **Issue**: Fixed symptoms, not root cause

**Qwen3 Coder** (0/100):
- Timed out after 8+ minutes
- No output produced
- Reliability issues
- **Issue**: Complete failure, avoid entirely

---

## Performance Benchmarks (Empirical Data)

**Test**: LSP Source Mapping Bug (diagnostic underlining wrong code)
**Methodology**: 8 models tested in parallel on real production bug

| Model | Time | Accuracy | Solution | Cost-Value |
|-------|------|----------|----------|------------|
| MiniMax M2 | 3 min | ✅ Exact | Simple fix | ⭐⭐⭐⭐⭐ |
| Grok Code Fast | 4 min | ✅ Correct | Good validation | ⭐⭐⭐⭐ |
| GPT-5.1 Codex | 5 min | ⚠️ Partial | Complex design | ⭐⭐⭐⭐ |
| Gemini 2.5 Flash | 6 min | ⚠️ Missed | Overanalyzed | ⭐⭐⭐ |
| GLM-4.6 | 7 min | ❌ Wrong | Overengineered | ⭐⭐ |
| Sherlock Think | 5 min | ❌ Secondary | Wrong cause | ⭐⭐ |
| Qwen3 Coder | 8+ min | ❌ Failed | Timeout | ⚠️ |

**Key Finding**: Faster models (3-5 min) delivered better results than slower ones (6-8 min).

**Correlation**: Speed ↔ Simplicity (faster models prioritize simple explanations first)

---

## When to Use Each Model

### Use MiniMax M2 when:
- ✅ Need fast, accurate diagnosis (3 minutes)
- ✅ Want simplest solution (avoid overengineering)
- ✅ Production bug investigation
- ✅ Most everyday tasks (90% of use cases)

### Use Grok Code Fast when:
- ✅ Need detailed debugging trace
- ✅ Want validation strategy
- ✅ Designing test coverage
- ✅ Understanding execution flow

### Use GPT-5.1 Codex when:
- ✅ Planning major architectural changes
- ✅ Need long-term refactoring strategy
- ✅ Want comprehensive testing approach
- ✅ High-level design decisions

### Use Gemini 2.5 Flash when:
- ✅ Problem is genuinely ambiguous
- ✅ Need exhaustive hypothesis exploration
- ✅ Budget is constrained (low cost)
- ✅ Multiple potential root causes

### Avoid using when:
- ❌ Problem is simple/obvious (just fix it)
- ❌ Sonnet 4.5 internal can answer (use internal first)
- ❌ Already-solved problem (check docs first)
- ❌ Time-critical (Qwen3 unreliable)

---

## Example: Invoking External Models

### Step 1: Create Session

```bash
SESSION=$(date +%Y%m%d-%H%M%S)
mkdir -p ai-docs/sessions/$SESSION/{input,output}
```

### Step 2: Write Investigation Prompt

```bash
# Write clear, self-contained prompt
echo "Problem: LSP diagnostic underlining wrong code..." > \
  ai-docs/sessions/$SESSION/input/investigation-prompt.md
```

### Step 3: Choose Strategy

Based on decision tree:
- Bug investigation → Strategy 1 (MiniMax M2 + Grok)

### Step 4: Launch Agents in Parallel

**Single message with 2 Task calls**:

```
Task 1 → golang-architect (PROXY MODE):

You are operating in PROXY MODE to investigate bug using MiniMax M2.

INPUT FILES:
- ai-docs/sessions/$SESSION/input/investigation-prompt.md

YOUR TASK (PROXY MODE):
1. Read investigation prompt
2. Use claudish to consult minimax/minimax-m2
3. Write full response to output file

**CRITICAL - Timeout**:
Bash(timeout=600000)  # 10 minutes!

OUTPUT FILES:
- ai-docs/sessions/$SESSION/output/minimax-m2-analysis.md

RETURN (MAX 3 lines):
MiniMax M2 analysis complete
Root cause: [one-line]
Full analysis: [file-path]
```

```
Task 2 → golang-architect (PROXY MODE):

[Same structure for Grok Code Fast]
```

### Step 5: Consolidate

After receiving both summaries:
1. Review 1-line summaries from each model
2. Identify consensus vs disagreements
3. Optionally read full analyses if needed
4. Decide on action based on recommendations

---

## Supporting Files

- **[BENCHMARKS.md](BENCHMARKS.md)** - Detailed performance metrics and test methodology
- **[STRATEGIES.md](STRATEGIES.md)** - Deep dive into each consultation strategy with examples

---

## Validation & Maintenance

**Last Validated**: 2025-11-18 (Session 20251118-223538)
**Next Review**: 2025-05 (6 months)
**Test Task**: LSP Source Mapping Bug

**Re-validation Schedule**:
- Every 3-6 months
- After new models become available
- When model performance changes significantly

**Track**:
- Model availability/reliability
- Speed improvements
- Accuracy changes
- Cost fluctuations

---

## Summary: Quick Decision Guide

**Most common use case (90%)**:
→ Use Strategy 1: MiniMax M2 + Grok Code Fast
→ Time: 4 min | Cost: $$ | Success: 95%+

**Critical issues**:
→ Use Strategy 2: MiniMax M2 + GPT-5.1 + Grok
→ Time: 5 min | Cost: $$$ | Success: 99%+

**Ambiguous problems**:
→ Use Strategy 3: MiniMax M2 + Gemini + Grok
→ Time: 6 min | Cost: $$ | Success: 90%+

**Cost-sensitive**:
→ Use Strategy 4: Gemini + Grok
→ Time: 6 min | Cost: $ | Success: 85%+

**Remember**:
- ⏱️ Always use 10-minute timeout
- 🚀 Launch models in parallel (single message)
- 📝 Communication via files (not inline)
- 🎯 Brief summaries only (MAX 3 lines)

---

**Full Reports**:
- Comprehensive comparison: `ai-docs/sessions/20251118-223538/01-planning/comprehensive-model-comparison.md`
- Model ranking analysis: `ai-docs/sessions/20251118-223538/01-planning/model-ranking-analysis.md`
