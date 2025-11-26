# Claude AI Agent Instructions - Dingo Landing Page

This file contains instructions for Claude AI agents working on the Dingo landing page project.

## ⚠️ CRITICAL: Token Budget Enforcement (READ FIRST)

**EVERY action must pass this pre-check:**

### Token Budget Limits (HARD LIMITS)

| Operation | Limit | Violation Remedy |
|-----------|-------|------------------|
| File reads per message | 2 files OR 200 lines total | Delegate to astro-developer |
| Component reads | 2 components max | Delegate to astro-developer |
| Bash output | 50 lines | Use `head -50` OR delegate |
| Agent response summary | 5 sentences max | Agent MUST compress |

**IF ANY LIMIT EXCEEDED → MUST delegate to agent instead**

### Forbidden Patterns in Main Chat

**❌ NEVER DO THESE:**

1. **Reading Multiple Components**
   - ❌ Read 3+ component files
   - ✅ Delegate to astro-developer → Read summary

2. **Implementing Components**
   - ❌ Write/edit multiple components directly
   - ✅ Delegate to astro-developer → Read summary

3. **Visual QA**
   - ❌ Take multiple screenshots, analyze in main chat
   - ✅ Delegate to astro-reviewer → Read summary

4. **Searching Codebase**
   - ❌ Multiple Grep calls for components
   - ✅ Delegate to astro-developer → Read summary

### Delegation Templates for Astro

#### Template: Component Implementation
```
Task tool → astro-developer:

Implement: [Component description]

Your Tasks:
1. Create/modify components
2. Write summary to: ai-docs/sessions/[session]/output/summary.txt

Return to Main Chat (MAX 5 sentences):
Status: Success/Partial/Failed
Components: [count] modified
Details: ai-docs/sessions/[session]/output/summary.txt

DO NOT return full component code in response.
```

#### Template: Visual Review
```
Task tool → astro-reviewer:

Review: [What to review visually]

Your Tasks:
1. Take screenshots, analyze visually
2. Check against ai-docs best practices
3. Write report to: ai-docs/sessions/[session]/output/review.md

Return to Main Chat (MAX 5 sentences):
Status: APPROVED / NEEDS_FIXES
Critical: [count] | Medium: [count]
Top Issue: [one-liner]
Details: ai-docs/sessions/[session]/output/review.md

DO NOT return full review or screenshots in response.
```

### Agent Self-Awareness Rules (Anti-Recursion)

**CRITICAL FOR ALL AGENTS:**

#### Rule 1: Know Thyself

Every agent MUST be aware of its own type:
- If you are `astro-developer`, you cannot delegate to `astro-developer`
- If you are `astro-reviewer`, you cannot delegate to `astro-reviewer`
- If you are `code-reviewer`, you cannot delegate to `code-reviewer`

**Why:** You ARE the specialized agent. Delegating to yourself causes recursion and failures.

#### Rule 2: Delegation Decision Tree

```
Before using Task tool, ask:
│
├─ What is my agent type?
│  └─ I am: [agent-name]
│
├─ What agent type does this task need?
│  ├─ Same as me → ❌ DO NOT delegate. Implement directly.
│  └─ Different → ✅ CAN delegate to that different agent
│
└─ Why do I want to delegate?
   ├─ "To save context" → ❌ WRONG REASON. Just do the work.
   ├─ "Instructions say to" → ❌ Those are for CALLERS, not you.
   └─ "Need different expertise" → ✅ OK if it's a different agent type.
```

#### Rule 3: Proxy Mode Is Not Self-Delegation

**Proxy Mode Means:**
- Using `claudish` to consult external models (Grok, Gemini)
- Getting suggestions/implementations from those models
- Implementing their suggestions yourself

**Proxy Mode Does NOT Mean:**
- Using Task tool to invoke yourself
- Creating another instance of your own agent
- Delegating work you should do directly

#### Rule 4: Instructions Are Context-Dependent

When agents read their prompts and see:
- "Use the Task tool to invoke the astro-developer agent"
- "Delegate to astro-reviewer for visual validation"

**Understand:** These instructions are for MAIN CHAT and EXTERNAL MODELS to use when calling the agent.

**NOT** for the agent to use to call itself.

The agent is the **destination** of those calls, not the **source**.

#### Rule 5: When In Doubt, Implement

If an agent is uncertain whether to delegate:
1. Check if delegating to own agent type → If yes, DON'T
2. Check if it has the expertise to implement → If yes, DO IT
3. Check if trying to save context → NOT A VALID REASON

**Default action for agents: Implement directly.**

#### Examples

**✅ CORRECT Delegation (Different Agent Types):**
- `astro-developer` delegates to `astro-reviewer` (different agent ✅)
- `astro-reviewer` delegates to `astro-developer` (different agent ✅)
- `astro-developer` delegates to `Explore` (different agent ✅)

**❌ WRONG Delegation (Recursion - Same Agent Type):**
- `astro-developer` delegates to `astro-developer` (same agent ❌)
- `astro-reviewer` delegates to `astro-reviewer` (same agent ❌)

## Project Overview

This is the **landing page** for the Dingo project, built with **Astro** - a modern web framework optimized for content-driven websites.

### Technology Stack
- **Framework**: Astro
- **Package Manager**: pnpm
- **Language**: TypeScript
- **Styling**: (To be determined - Tailwind CSS likely)

### Project Purpose
Create a fast, SEO-optimized landing page that:
- Explains what Dingo is (a meta-language for Go)
- Showcases Dingo's features and benefits
- Provides clear calls-to-action
- Links to documentation and GitHub repository
- Maintains excellent performance and accessibility

## Key Rules for AI Agents

### CRITICAL: Agent Scope Separation

**This landing page project uses DIFFERENT agents than the main Dingo transpiler.**

#### ✅ **For Landing Page Work** (This Directory)
**Working Directory**: `langingpage/` (this directory)
**Code**: `src/`, Astro components, React components, TypeScript, Tailwind CSS

**Use these agents**:
- ✅ `astro-developer` - Implementation (components, layouts, pages, styling)
- ✅ `astro-reviewer` - Code review and visual validation with chrome-devtools
- ⚠️ `code-reviewer` - Can review, but astro-reviewer preferred (Astro-specific knowledge)

**Slash commands**:
- ✅ `/astro-dev` - Development orchestrator for landing page
- ✅ `/astro-fix` - Visual fix orchestrator with iterative refinement

#### ❌ **NOT For Landing Page Work**
**These agents are for the Dingo transpiler ONLY** (parent directory):
- ❌ `golang-developer` - For Go/transpiler code only
- ❌ `golang-architect` - For Go architecture only
- ❌ `golang-tester` - For Go testing only
- ❌ `/dev` - For transpiler development only

#### Quick Decision Guide

**If you're working in `/langingpage/` directory**:
- Astro components, React components, styling → `astro-developer`
- Code review, visual validation → `astro-reviewer`
- Development workflow → `/astro-dev`
- Visual bug fixes → `/astro-fix`

**If someone asks you to work on**:
- Transpiler, parser, AST, language features → **WRONG DIRECTORY!** → Use parent directory with golang-* agents
- Landing page, UI, components, styling → **CORRECT!** → Use astro-* agents

### 1. Consult the AI Documentation First
**IMPORTANT**: Before working on Astro-related tasks, consult the `ai-docs/` directory:

- **Start with**: `ai-docs/INDEX.md` - Overview and navigation guide
- **Core concepts**: `ai-docs/01-why-astro.md` - Philosophy and principles
- **Specific topics**: Reference the appropriate module based on your task

### 2. Follow Astro Best Practices

Always adhere to Astro's core principles:
- **Server-first**: Default to server rendering
- **Zero JS by default**: Only add client-side JavaScript when necessary
- **Islands Architecture**: Use for interactive components
- **Content-focused**: Prioritize fast content delivery
- **Progressive enhancement**: Build with HTML/CSS, enhance with JS

### 3. Project Structure

```
/
├── ai-docs/           # AI agent knowledge base (CONSULT THIS!)
├── public/            # Static assets (favicon, images, etc.)
├── src/
│   ├── assets/        # Optimized assets (images, etc.)
│   ├── components/    # Astro/framework components
│   ├── layouts/       # Page layouts
│   └── pages/         # File-based routing
├── astro.config.mjs   # Astro configuration
├── CLAUDE.md          # This file
└── package.json       # Dependencies and scripts
```

### 4. Development Workflow

```bash
# Start development server
pnpm dev

# Build for production
pnpm build

# Preview production build
pnpm preview

# Run Astro CLI commands
pnpm astro [command]
```

### 5. Documentation Guidelines

**DO NOT** create documentation files in the root directory. Instead:
- **AI agent docs**: → `ai-docs/` directory
- **User-facing docs**: → `docs/` directory (when needed)
- **Keep root clean**: Only essential project files

## Current Project Status

**Status**: Initial setup phase
**Next Steps**:
1. Design landing page structure
2. Create component library
3. Implement content sections
4. Add styling system (likely Tailwind CSS)
5. Optimize for performance and SEO

## AI Documentation Modules

The `ai-docs/` directory contains comprehensive Astro knowledge:

1. **Why Astro** - Philosophy and design principles ✅
2. **Core Concepts** - Islands, routing, rendering (planned)
3. **Components** - Component development patterns (planned)
4. **Content** - Content collections and management (planned)
5. **Styling** - Styling strategies and best practices (planned)
6. **Integrations** - Adding functionality with integrations (planned)
7. **Deployment** - Build and deployment strategies (planned)
8. **Best Practices** - Performance, SEO, accessibility (planned)

## AI Agent Usage Guidelines

### CRITICAL: Always Consult ai-docs First

Before performing ANY Astro-related task, AI agents MUST:

1. **Start with the INDEX**: Read `ai-docs/INDEX.md` to understand the knowledge base structure
2. **Read core principles**: Review `ai-docs/01-why-astro.md` to understand Astro's philosophy
3. **Consult relevant modules**: Reference specific ai-docs modules for the task at hand
4. **Use the checklist**: Validate against `ai-docs/best-practices-checklist.md`
5. **Document references**: Note which ai-docs modules influenced your decisions

### AI Agent Workflow Pattern

#### Phase 1: Planning
1. Read `ai-docs/INDEX.md` and `ai-docs/01-why-astro.md`
2. Identify relevant modules for the task (components, content, layouts, etc.)
3. Review applicable recipes from `ai-docs/recipes/`
4. Consult `ai-docs/best-practices-checklist.md` for decision trees
5. Create plan with explicit ai-docs references

**Example Plan Documentation**:
```markdown
## Architecture Decision: Component Strategy
Following ai-docs/07-astro-components.md and ai-docs/02-islands-architecture.md:
- Hero section: .astro component (static content)
- Contact form: React component with client:visible (interactivity needed)
- Features grid: .astro component (static)
```

#### Phase 2: Implementation
1. Keep `ai-docs/best-practices-checklist.md` open during coding
2. Read specific modules on-demand:
   - Components: `ai-docs/07-astro-components.md` or `ai-docs/06-framework-components.md`
   - Images: `ai-docs/recipes/images.md`
   - Layouts: `ai-docs/08-layouts.md`
   - Content: `ai-docs/05-content-collections.md`
   - Scripts: `ai-docs/recipes/scripts-and-events.md`
3. Validate each file against the checklist as you write
4. Document which ai-docs modules you consulted

**Example Implementation Documentation**:
```markdown
## Implementation: Blog Post Component
Referenced ai-docs modules:
- ai-docs/08-layouts.md - Used nested layout pattern
- ai-docs/05-content-collections.md - Implemented Content Layer with glob loader
- ai-docs/09-markdown.md - Configured remark plugins for heading IDs
- ai-docs/recipes/syntax-highlighting.md - Set up Shiki for code blocks
```

#### Phase 3: QA Testing
1. Systematically validate against `ai-docs/best-practices-checklist.md`
2. Check Core Principles section (CRITICAL issues)
3. Verify Islands Architecture usage (if applicable)
4. Validate project structure
5. Review component patterns
6. Check applicable recipes
7. For each issue found:
   - Classify severity (CRITICAL/MEDIUM/MINOR)
   - Reference specific ai-docs module violated
   - Include ai-docs link in report

**Example QA Report Format**:
```markdown
### CRITICAL Issues

1. **Client-side JS sent unnecessarily**
   - Severity: CRITICAL
   - Violates: ai-docs/01-why-astro.md - Principle #2 (Zero JS by default)
   - Location: src/components/Hero.tsx
   - Issue: Using React component with client:load for static content
   - Fix: Convert to .astro component per ai-docs/07-astro-components.md
   - Reference: ai-docs/best-practices-checklist.md - "Should I use a framework component?"

### MEDIUM Issues

2. **Suboptimal Island directive**
   - Severity: MEDIUM
   - Violates: ai-docs/02-islands-architecture.md - "Which client: directive?"
   - Location: src/components/ContactForm.tsx
   - Issue: Using client:load when client:visible would work (below fold)
   - Fix: Change to client:visible per ai-docs/02-islands-architecture.md
```

#### Phase 4: Fixes
1. Read the ai-docs module referenced in the issue
2. Understand the correct pattern from the ai-docs
3. Apply the fix exactly as documented
4. Re-validate against checklist
5. Document which ai-docs module was used

**Example Fix Documentation**:
```markdown
## Fix Applied: Hero Component

### Issue
CRITICAL: Client-side JS sent unnecessarily (from internal-qa.md #1)

### ai-docs Consulted
- ai-docs/07-astro-components.md - Component structure and patterns
- ai-docs/best-practices-checklist.md - "Should I use a framework component?" decision tree

### Pattern Applied
Converted from React component to .astro component:
- Removed client:load directive (wasn't needed)
- Moved to .astro syntax for server-side rendering
- Kept scoped styles
- Result: 47KB JavaScript eliminated from bundle

### Verification
✅ No client-side JS sent for static content
✅ Follows ai-docs/01-why-astro.md principle #2 (Zero JS by default)
✅ Matches pattern from ai-docs/07-astro-components.md section "When to use .astro"
```

### Common Tasks for AI Agents

#### Creating Components
1. **Decision Tree** (from `ai-docs/best-practices-checklist.md`):
   - Is interactivity needed? → NO: Use `.astro` component
   - Is it simple (click, toggle)? → YES: Try `<script>` or Web Component
   - Complex state needed? → YES: Use framework component
2. **Consult**: `ai-docs/07-astro-components.md` for `.astro` patterns
3. **Or consult**: `ai-docs/06-framework-components.md` for framework integration
4. **Validate**: Against checklist - "Component Development" section

#### Managing Content
1. **Always use Content Collections** (Astro 5.x Content Layer API)
2. **Consult**: `ai-docs/05-content-collections.md` for setup
3. **Pattern**: Define schema with Zod, use appropriate loader (glob, file, custom)
4. **Validate**: Against checklist - "Content Management" section

#### Adding Layouts
1. **Consult**: `ai-docs/08-layouts.md` for layout patterns
2. **Pattern**: Base layout with HTML structure, nested layouts for inheritance
3. **Include**: SEO meta tags, Open Graph, structured data
4. **Validate**: Against checklist - "Layouts" section

#### Optimizing Images
1. **Rule**: Images in `src/assets/` (NOT `public/`)
2. **Consult**: `ai-docs/recipes/images.md`
3. **Use**: `<Image />` component for optimization
4. **Validate**: Against checklist - "Recipes & Common Patterns → Images"

#### Adding Interactivity
1. **Decision**: Which `client:*` directive?
   - `client:visible` - Preferred (loads when scrolled into view)
   - `client:idle` - After page load
   - `client:load` - Immediately (rare, critical only)
2. **Consult**: `ai-docs/02-islands-architecture.md` for directive guide
3. **Or consult**: `ai-docs/recipes/scripts-and-events.md` for `<script>` patterns
4. **Validate**: Against checklist - "Islands Architecture" section

#### Styling Components
1. **Default**: Scoped styles in `.astro` components
2. **For Markdown**: Use Tailwind Typography plugin
3. **Consult**: `ai-docs/recipes/tailwind-typography.md`
4. **Fonts**: See `ai-docs/recipes/custom-fonts.md`
5. **Validate**: Against checklist - "Recipes" section

#### Adding Syntax Highlighting
1. **Built-in**: Shiki (default, recommended)
2. **Consult**: `ai-docs/recipes/syntax-highlighting.md`
3. **Configure**: Theme in `astro.config.mjs`
4. **Use**: `<Code />` component for inline code

### Performance Optimization
1. **Targets**: LCP < 2.5s, FID < 100ms, CLS < 0.1
2. **Checklist**: Review "Performance Checklist" in `ai-docs/best-practices-checklist.md`
3. **Key tactics**:
   - Minimize client-side JavaScript (principle #1)
   - Use `client:visible` for below-fold (Islands Architecture)
   - Optimize images with `<Image />` (recipes/images.md)
   - Use content collections (ai-docs/05-content-collections.md)
4. **Validate**: Build size, Core Web Vitals in browser

## Key Principles

1. **Server-first rendering**: Astro renders on the server by default
2. **Islands for interactivity**: Use Islands Architecture for dynamic components
3. **Framework agnostic**: Can integrate React, Vue, Svelte, etc., when needed
4. **Zero JS by default**: No JavaScript sent to client unless explicitly needed
5. **Content-driven**: Optimized for content delivery and SEO

## External Resources

- [Astro Documentation](https://docs.astro.build)
- [Astro Discord](https://astro.build/chat)
- [Dingo Project Main Repo](../README.md)

## Questions to Resolve

- [ ] Styling approach: Tailwind CSS? Vanilla CSS? CSS framework?
- [ ] CTA strategy: Newsletter? GitHub stars? Documentation link?
- [ ] Content structure: Single page? Multiple pages?
- [ ] Integrations needed: Analytics? Contact forms?
- [ ] Deployment target: Vercel? Netlify? Cloudflare Pages?

---

**Last Updated**: 2025-11-17
**Current Phase**: Initial Setup
**Next Milestone**: Design and build landing page structure
