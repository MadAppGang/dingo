# Investigation Skill

You are executing the **Investigation** pattern. This skill helps you delegate deep codebase investigation to specialized agents while keeping main chat context minimal.

## Your Task

The user wants to understand how something works in the codebase. Follow these steps:

### Step 1: Identify What to Investigate

Extract from user's request:
- What feature/concept to investigate?
- What specific questions to answer?
- What files/components are relevant?

### Step 2: Choose Appropriate Agent

Based on domain:
- **Go code** (parser, transpiler, AST, language features) → `golang-developer` or `Explore`
- **Astro/React** (landing page, components) → `astro-developer`
- **General codebase exploration** → `Explore` agent
- **Architecture/design** → `golang-architect` (for Go) or `astro-developer` (for Astro)

**Preference**:
- Use `Explore` for quick searches and understanding
- Use `golang-developer` for deep implementation analysis
- Use `golang-architect` for design/architecture questions

### Step 3: Create Output Location

```bash
# For quick investigations (single topic)
mkdir -p ai-docs/analysis/

# For complex investigations (multi-part)
SESSION=$(date +%Y%m%d-%H%M%S)
mkdir -p ai-docs/sessions/$SESSION/output/
```

### Step 4: Delegate to Agent

Use Task tool with appropriate agent:

```
Task tool → [agent-type]:

Investigate: [What to understand]

Questions to answer:
1. [Specific question 1]
2. [Specific question 2]
3. [Specific question 3]

Your Tasks:
1. Search codebase thoroughly (use Grep, Read, Glob as needed)
2. Analyze findings
3. Write detailed report to: [output-path]

Report should include:
- Overview: What this feature/concept is
- Location: Key files and line numbers
- Implementation: How it works
- Examples: Usage patterns
- Related: Connected components/features

Return to Main Chat (MAX 5 sentences):
What: [One-line description]
Where: [Key file locations with line numbers]
How: [Brief mechanism]
Details: [output-path]

DO NOT return full code or detailed analysis in response.
```

### Step 5: Present Summary to User

After receiving agent summary:
1. Show the brief summary (5 sentences)
2. Highlight key files with line numbers
3. Provide path to detailed report
4. Ask if user wants to see specific parts

### Example Execution

```
User: "How does the error propagation operator (?) work?"

You (main chat):
1. Identify: Error propagation implementation
2. Choose: golang-developer (deep implementation analysis)
3. Create: ai-docs/analysis/error-propagation-analysis.md
4. Delegate to golang-developer with specific questions
5. Receive summary:
   "Error propagation uses ErrorPropProcessor in pkg/preprocessor/preprocessor.go:156.
    Transforms x? to if err := x; err != nil { return err }.
    Integrated in plugin pipeline at pkg/plugin/plugin.go:89.
    Details: ai-docs/analysis/error-propagation-analysis.md"
6. Present to user with file paths

Total context: ~10 lines
Detailed report: In file
```

### Step 6: Optional Deep Dive

If user wants to see specific code:
1. Use Read tool to show specific files/sections
2. Keep output focused (use offset/limit if needed)
3. Explain what's shown

## Key Rules

1. ✅ **Always delegate investigation** to agents (not main chat)
2. ✅ **Use appropriate agent** (golang-developer for Go, etc.)
3. ✅ **Request file output** from agent (not inline response)
4. ✅ **Include line numbers** in file references
5. ✅ **Keep main chat minimal** (summaries only)
6. ❌ **Never read multiple files directly** (delegate to agent)
7. ❌ **Never show full analysis** in main chat (use files)

## Output Locations

**Quick investigations** (single topic):
- `ai-docs/analysis/[topic]-analysis.md`

**Complex investigations** (multi-part):
- `ai-docs/sessions/[YYYYMMDD-HHMMSS]/output/investigation-report.md`

## Success Metrics

- **Context saved**: 10-20x reduction (files vs inline)
- **Clarity**: File paths with line numbers for navigation
- **Speed**: Agent can search thoroughly without context limits

## What to Return to User

1. Brief summary (5 sentences max)
2. Key file paths with line numbers
3. Path to detailed report
4. Offer to show specific sections

**Keep it lean. Let agents do the heavy lifting!**
