# Multi-Model Consultation Skill

You are executing the **Multi-Model Consultation** pattern. This skill helps you consult multiple external LLMs in parallel to get diverse perspectives on architectural decisions, design choices, or complex analysis.

## Your Task

The user wants perspectives from multiple external models. Follow these steps EXACTLY:

### Step 1: Create Session Folder

```bash
SESSION=$(date +%Y%m%d-%H%M%S)
mkdir -p ai-docs/sessions/$SESSION/{input,output}
```

### Step 2: Write Investigation Prompt

Extract the user's question/investigation topic and write a comprehensive prompt to:
`ai-docs/sessions/$SESSION/input/investigation-prompt.md`

The prompt should:
- Clearly state the question/problem
- Provide necessary context about Dingo project
- Ask for specific analysis or recommendations
- Be self-contained (model won't have conversation history)

### Step 3: Identify Models to Consult

**Default models** (if user doesn't specify):
- `openai/gpt-5.1-codex` - Software engineering specialist
- `google/gemini-2.5-flash` - Advanced reasoning + fast
- `x-ai/grok-code-fast-1` - Ultra-fast practical insights

**Available models** (via `claudish --list-models`):
- `openai/gpt-5` - Most advanced reasoning
- `openai/gpt-5.1-codex` - Software engineering specialist
- `google/gemini-2.5-flash` - Advanced reasoning + fast
- `x-ai/grok-code-fast-1` - Ultra-fast coding
- `qwen/qwen3-vl-235b-a22b-instruct` - Multimodal
- `openrouter/polaris-alpha` - FREE experimental
- `minimax/minimax-m2` - Compact high-efficiency

### Step 4: Select Appropriate Agent Type

Based on the domain:
- **Go project questions** (parser, AST, transpiler) → `golang-architect`
- **Astro/landing page questions** → `astro-developer`
- **General code review** → `code-reviewer`
- **Multi-language** → `general-purpose` (last resort)

### Step 5: Launch Agents in Parallel

**CRITICAL**: Launch ALL agents in a **SINGLE MESSAGE** with multiple Task tool calls.

For each model, create a Task call like this:

```
Task tool → [agent-type]:

You are consulting external model: [model-name]

Your task:
1. Read the investigation prompt from: ai-docs/sessions/$SESSION/input/investigation-prompt.md
2. Invoke the external model via claudish:
   cat ai-docs/sessions/$SESSION/input/investigation-prompt.md | \
     claudish --model [model-id] > \
     ai-docs/sessions/$SESSION/output/[model-name]-analysis.md
3. Return a MAX 5 sentence summary to main chat

Return format (MAX 5 sentences):
Model: [model-name]
Key insight: [one-liner]
Recommendation: [brief]
Details: ai-docs/sessions/$SESSION/output/[model-name]-analysis.md

DO NOT return the full analysis in your response.
```

**Example** (3 models in parallel):
- Launch 3 Task calls in ONE message
- Each Task uses same agent type (e.g., golang-architect)
- Each Task invokes different model
- Each Task saves to different output file

### Step 6: Aggregate Results

After receiving all summaries:
1. Present brief overview to user (which models were consulted)
2. Show 1-sentence key finding from each model
3. Provide file paths for detailed analyses
4. Ask if user wants a consolidation analysis

### Step 7: Optional Consolidation

If user wants synthesis:
- Launch ONE final agent (same type)
- Agent reads ALL analysis files
- Agent synthesizes consensus + disagreements
- Agent writes: `ai-docs/sessions/$SESSION/output/CONSOLIDATED.md`
- Agent returns brief summary

## Key Rules

1. ✅ **Always use specialized agents** (golang-architect for Go, etc.)
2. ✅ **Always launch in parallel** (single message, multiple Task calls)
3. ✅ **Each agent = one model** (1:1 mapping)
4. ✅ **Communication via files** (full analysis → files, brief summary → response)
5. ✅ **Agent uses Bash** to invoke claudish (not Task tool)
6. ❌ **Never use general-purpose** unless no specialized agent exists

## Example Execution

```
User: "Should we use regex preprocessor or migrate to tree-sitter?"

You (main chat):
1. Create session: ai-docs/sessions/20251118-160000/
2. Write investigation-prompt.md with detailed question
3. Launch 3 golang-architect agents IN PARALLEL (one message):
   - Task 1: gpt-5.1-codex
   - Task 2: gemini-2.5-flash
   - Task 3: grok-code-fast-1
4. Receive 3 summaries
5. Present to user:
   "Consulted 3 models:
    - GPT-5.1-Codex: Recommends regex for simplicity, tree-sitter for future
    - Gemini-2.5-Flash: Suggests hybrid approach
    - Grok: Advocates staying with regex

    Details: ai-docs/sessions/20251118-160000/output/
    Want me to synthesize these perspectives?"
```

## Success Metrics

- **Speed**: 2-3 minutes (parallel) vs 5-10 minutes (sequential)
- **Context**: 50-100 lines (files) vs 500-1000 lines (inline)
- **Quality**: Diverse perspectives + domain expertise

## What to Return to User

After execution completes:
1. Brief summary of what models were consulted
2. One-line key insight from each model
3. Session folder path
4. Ask if consolidation needed

**Total output**: < 20 lines in main chat
**Detailed analyses**: In session files

---

**Remember**: You are the orchestrator. Delegate the actual model invocations to specialized agents. Keep main chat lean!
