# AI Documentation

This directory contains AI agent working documents, research, and historical records.

## Directory Structure

```
ai-docs/
├── README.md                      # This file
├── ARCHITECTURE.md                # Technical architecture overview
├── delegation-strategy.md         # AI agent delegation rules
├── research/                      # Consolidated research
│   ├── compiler/
│   │   └── claude-research.md     # Compiler architecture (canonical)
│   ├── golang_missing/
│   │   └── claud.md               # Go missing features analysis (canonical)
│   └── enum-naming-recommendations.md
├── language/                      # Language design docs
│   ├── SYNTAX_DESIGN.md
│   └── UI_IMPLEMENTATION.md
├── sessions/                      # Current session logs (last 7 days)
│   └── 20251119-*/                # Only keep recent sessions
└── archive/                       # Historical documents
    ├── sessions/                  # Old session logs (Nov 16-18)
    ├── investigations/            # Resolved bug investigations  
    └── research/                  # Superseded research docs
```

## Documentation Policy

### Keep in ai-docs/ (Active)
- Current architecture documentation
- Active research (not superseded)
- Session logs from last 7 days
- Language design specs

### Move to ai-docs/archive/ (Historical)
- Session logs older than 7 days
- Resolved bug investigations
- Superseded research versions
- Completed project phases

### Delete (Redundant)
- Exact duplicates
- Empty/stub files
- Obsolete without historical value

## Source of Truth

For current project status, always defer to:
1. **`CLAUDE.md`** (project root) - Current phase, test results, implementation status
2. **`features/INDEX.md`** - Feature-by-feature status
3. **`CHANGELOG.md`** - Historical timeline

This directory contains supporting context and research, NOT the source of truth for project status.

## Maintenance

Run documentation consolidation quarterly or when files exceed 200.
