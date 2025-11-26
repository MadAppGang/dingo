# Astro AI Knowledge Base - Index

This directory contains comprehensive documentation for AI agents working on Astro-based projects. The knowledge base is organized into numbered modules for sequential learning.

## Purpose

This knowledge base helps AI agents understand:
- Astro's philosophy and design principles
- Core concepts and architecture
- Best practices and patterns
- Common tasks and workflows
- Integration strategies

## Documentation Structure

### Quick Reference

- **[best-practices-checklist.md](./best-practices-checklist.md)** - ⭐ Quick validation checklist for AI agents
  - Core principles validation
  - Islands Architecture patterns
  - Component development checklist
  - QA testing severity guide
  - Common anti-patterns
  - Decision trees for quick reference
  - Pre-deployment checklist

### Current Modules

1. **[01-why-astro.md](./01-why-astro.md)** - Philosophy and Design Principles
   - Why Astro exists
   - Core features and benefits
   - Design principles (content-driven, server-first, fast by default, easy to use, developer-focused)
   - Comparison with SPA frameworks
   - When to choose Astro

2. **[02-islands-architecture.md](./02-islands-architecture.md)** - Islands Architecture
   - Concept and historical context
   - Client Islands vs Server Islands
   - Loading strategies (`client:load`, `client:idle`, `client:visible`, etc.)
   - Framework flexibility and mixing
   - State sharing between islands
   - Best practices and common patterns
   - Performance impact

3. **[03-project-structure.md](./03-project-structure.md)** - Project Structure
   - Core directories (`src/`, `public/`, `src/pages/`)
   - Common subdirectories (components, layouts, styles, content, assets)
   - Configuration files (package.json, astro.config.mjs, tsconfig.json)
   - File naming conventions
   - Best practices for organization

4. **[04-development-workflow.md](./04-development-workflow.md)** - Development Workflow
   - Development server (dev mode, HMR, features)
   - Build process (production builds, optimization)
   - Preview mode (testing builds locally)
   - CLI commands and integrations
   - Environment variables
   - Debugging and troubleshooting

5. **[05-content-collections.md](./05-content-collections.md)** - Content Collections
   - Content Layer API and loaders (glob, file, custom)
   - Schemas and Zod validation
   - TypeScript type safety
   - Querying collections (getCollection, getEntry)
   - References between collections
   - Rendering Markdown content
   - Generating routes from collections
   - JSON schemas for editor support

6. **[06-framework-components.md](./06-framework-components.md)** - Framework Components
   - Official integrations (React, Vue, Svelte, Solid, Alpine)
   - Installing and configuring frameworks
   - Static vs hydrated components
   - Client directives (load, idle, visible, media, only)
   - Passing props and children
   - Named slots
   - Mixing multiple frameworks
   - Nesting components
   - Framework-specific examples

7. **[07-astro-components.md](./07-astro-components.md)** - Astro Components
   - Component structure (script and template)
   - Props and TypeScript
   - Slots (default and named)
   - Fallback content
   - Transferring slots
   - HTML components
   - Component patterns

8. **[08-layouts.md](./08-layouts.md)** - Layouts
   - What layouts are and how to use them
   - TypeScript with layouts
   - Markdown layouts and frontmatter
   - Markdown layout props
   - MDX layouts (frontmatter and manual import)
   - Nesting layouts
   - Layout patterns (SEO, multi-slot, conditional)

9. **[09-markdown.md](./09-markdown.md)** - Markdown in Astro
   - Markdown vs MDX
   - File imports and content collections
   - Frontmatter (YAML/TOML)
   - Rendering with `<Content />` component
   - Heading IDs and anchor generation
   - Remark and rehype plugins
   - Syntax highlighting integration

10. **[10-data-fetching.md](./10-data-fetching.md)** - Data Fetching
   - fetch() in Astro components and framework components
   - Top-level await support
   - GraphQL queries
   - Headless CMS integration (Contentful, WordPress, GraphCMS)
   - Dynamic routes with fetched data
   - Environment variables for API keys
   - Caching and performance optimization

11. **[11-astro-db.md](./11-astro-db.md)** - Astro DB
   - Fully-managed SQL database with libSQL
   - Table definitions and column types
   - Table relationships and foreign keys
   - Seeding development data
   - Drizzle ORM queries (select, insert, update, delete)
   - Remote database setup with Turso
   - Schema migrations and batch transactions

### Planned Modules

12. **12-styling.md** - Styling Strategies
   - Scoped styles
   - Global styles
   - CSS frameworks integration
   - Tailwind CSS setup

13. **13-integrations.md** - Integrations & Add-ons
    - Official integrations
    - Community integrations
    - Custom integrations
    - Configuration

14. **14-deployment.md** - Deployment
    - Build process
    - Static vs SSR
    - Deployment platforms
    - Performance optimization

15. **15-best-practices.md** - Best Practices
    - Project structure
    - Performance optimization
    - SEO optimization
    - Accessibility

### Recipes (Practical Guides)

- **[recipes/bun-integration.md](./recipes/bun-integration.md)** - Using Bun with Astro
  - Creating projects with Bun
  - Installing dependencies
  - Development workflow
  - Testing with Bun's built-in runner
  - Migration from npm/pnpm/yarn

- **[recipes/custom-fonts.md](./recipes/custom-fonts.md)** - Custom Fonts
  - Local font files with @font-face
  - Fontsource NPM packages
  - Tailwind CSS integration
  - Font display strategies
  - Performance optimization
  - Preloading critical fonts

- **[recipes/tailwind-typography.md](./recipes/tailwind-typography.md)** - Tailwind Typography Plugin
  - Installing and configuring
  - Creating Prose components
  - Element modifiers and customization
  - Dark mode support
  - Responsive typography

- **[recipes/syntax-highlighting.md](./recipes/syntax-highlighting.md)** - Syntax Highlighting
  - Markdown code blocks with Shiki
  - `<Code />` component usage
  - Themes and customization
  - Light/dark mode support
  - Prism alternative
  - Transformers for advanced features

- **[recipes/scripts-and-events.md](./recipes/scripts-and-events.md)** - Scripts & Event Handling
  - Client-side scripts in Astro
  - Script processing vs inline
  - Event handling patterns
  - Web Components (Custom Elements)
  - Passing data server-to-client
  - Common interactivity patterns

- **[recipes/images.md](./recipes/images.md)** - Images in Astro
  - Image storage (src/ vs public/)
  - `<Image />` and `<Picture />` components
  - Responsive images and formats
  - SVG components
  - Remote images and authorization
  - Images in content collections
  - Performance optimization

- **[recipes/markdoc.md](./recipes/markdoc.md)** - Markdoc Integration
  - Markdoc vs MDX vs Markdown
  - Custom tags for Astro components
  - Variables and conditionals
  - Partials for reusable content
  - Syntax highlighting with Shiki/Prism
  - Custom nodes and headings
  - Advanced configuration

- **[recipes/mdx.md](./recipes/mdx.md)** - MDX Integration
  - MDX with content collections
  - Exported variables and frontmatter
  - Using Astro and framework components
  - Custom component mapping to HTML elements
  - Remark and rehype plugins
  - JavaScript expressions in content
  - Optimization options

- **[recipes/deploy-aws.md](./recipes/deploy-aws.md)** - Deploy to AWS
  - AWS Amplify (static and SSR)
  - S3 static website hosting
  - CloudFront CDN integration
  - Custom domains and SSL
  - CI/CD with GitHub Actions
  - Cost estimation and best practices

- **[recipes/deploy-github-pages.md](./recipes/deploy-github-pages.md)** - Deploy to GitHub Pages
  - Official Astro GitHub Action
  - Base URL configuration
  - Custom domain setup
  - Environment variables
  - Deployment workflows
  - Troubleshooting and optimization

## How to Use This Knowledge Base

### For AI Agents

When working on Astro projects:

1. **Start with INDEX.md** (this file) to understand the knowledge base structure
2. **Review best-practices-checklist.md** for quick validation and decision trees
3. **Read 01-why-astro.md** to understand the framework's philosophy
4. **Consult specific modules** on-demand based on the task at hand
5. **Follow the principles** outlined in each document
6. **Validate continuously** against the checklist during planning, implementation, and QA
7. **Document ai-docs references** in your plans, code, and reports

**CRITICAL**: Always consult ai-docs before making architectural decisions. See `CLAUDE.md` in the langingpage directory for detailed AI agent workflow patterns.

### For Specific Tasks

- **Quick validation**: See best-practices-checklist.md ⭐
- **Making decisions** (component type, client directive, image location): See best-practices-checklist.md decision trees
- **QA testing**: See best-practices-checklist.md for systematic validation
- **Understanding Astro philosophy**: See 01-why-astro.md
- **Understanding Islands**: See 02-islands-architecture.md
- **Project organization**: See 03-project-structure.md
- **Dev/build workflow**: See 04-development-workflow.md
- **Managing content**: See 05-content-collections.md
- **Using framework components**: See 06-framework-components.md
- **Creating Astro components**: See 07-astro-components.md
- **Building layouts**: See 08-layouts.md
- **Working with Markdown/MDX**: See 09-markdown.md
- **Fetching data from APIs**: See 10-data-fetching.md
- **Using Astro DB**: See 11-astro-db.md
- **Styling decisions**: See 12-styling.md (planned)
- **Adding integrations**: See 13-integrations.md (planned)
- **Deployment help**: See 14-deployment.md (planned)
- **Performance issues**: See 15-best-practices.md (planned)

### For Specific Recipes

- **Using Bun runtime**: See recipes/bun-integration.md
- **Adding custom fonts**: See recipes/custom-fonts.md
- **Styling Markdown**: See recipes/tailwind-typography.md
- **Code highlighting**: See recipes/syntax-highlighting.md
- **Client-side interactivity**: See recipes/scripts-and-events.md
- **Optimizing images**: See recipes/images.md
- **Using Markdoc**: See recipes/markdoc.md
- **Using MDX**: See recipes/mdx.md
- **Deploy to AWS**: See recipes/deploy-aws.md
- **Deploy to GitHub Pages**: See recipes/deploy-github-pages.md

## Project Context

This knowledge base is specifically for the **Dingo Landing Page** project, an Astro-based marketing website.

### Project Info
- **Location**: `langingpage`
- **Framework**: Astro
- **Package Manager**: pnpm
- **Purpose**: Landing page for the Dingo meta-language project

### Key Commands
```bash
pnpm dev       # Start dev server at localhost:4321
pnpm build     # Build for production
pnpm preview   # Preview production build
```

## Contributing to This Knowledge Base

When adding new documentation:

1. **Number the file** sequentially (e.g., `09-new-topic.md`)
2. **Update this INDEX.md** with the new module
3. **Keep formatting consistent** with existing docs
4. **Include practical examples** where relevant
5. **Add "Last Updated" timestamp** at the bottom of each document

## Key Principles for AI Agents

When working with Astro, always remember:

1. **Server-first**: Default to server rendering, opt-in to client-side JavaScript
2. **Content-focused**: Prioritize content delivery and performance
3. **Zero JS by default**: Only add JavaScript when necessary
4. **Progressive enhancement**: Build base functionality with HTML/CSS, enhance with JS
5. **Use Islands**: For interactive components, use the Islands architecture

## External Resources

- [Official Astro Docs](https://docs.astro.build)
- [Astro Discord](https://astro.build/chat)
- [Astro GitHub](https://github.com/withastro/astro)
- [Astro Integrations](https://astro.build/integrations)

---

**Last Updated**: 2025-11-17
**Status**: Modules 1-11 complete, modules 12-15 planned, 10 recipes complete
**Next**: Add styling strategies module (12-styling.md)
**Recipes**: Bun, Custom Fonts, Tailwind Typography, Syntax Highlighting, Scripts & Events, Images, Markdoc, MDX, AWS Deployment, GitHub Pages Deployment
