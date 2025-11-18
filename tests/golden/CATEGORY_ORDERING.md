# Category Ordering System

This document defines how examples are ordered on the landing page.

## Overview

Examples are sorted by TWO levels:
1. **Primary Sort:** `category_order` - Controls which category appears first
2. **Secondary Sort:** `order` - Controls example order within each category

## Category Order Values

Categories are ordered to maximize impact on landing page visitors:

| Category | Order | Rationale |
|----------|-------|-----------|
| **Showcase** | 0 | 🎪 THE flagship - comprehensive feature demonstration |
| **Error Handling** | 10 | ⚡ Core feature, immediate practical value |
| **Type System (Sum Types)** | 20 | 🎭 Massive boilerplate elimination (85.7%!) |
| **Result Type** | 30 | 📦 Related to error handling, high value |
| **Option Type** | 40 | 🔍 Null safety, common pain point |
| **Functions (Lambdas)** | 50 | 🎯 Functional programming patterns |
| **Control Flow (Pattern Matching)** | 60 | 🔀 Advanced control flow |
| **Operators (Ternary)** | 70 | ⚡ Simple, familiar operators |
| **Operators (Null Coalescing)** | 80 | ?? Simple operators |
| **Operators (Safe Navigation)** | 90 | ?. Simple operators |
| **Data Structures (Tuples)** | 100 | 📊 Data handling |
| **Functional Utilities** | 110 | 🔧 Higher-order functions |

**Note:** Values increment by 10 to allow inserting new categories (e.g., order 15, 25, etc.)

---

## Implementation

### 1. Content Schema (`langingpage/src/content.config.ts`)

```typescript
category_order: z.number().optional(), // Controls category sorting
order: z.number(),                      // Controls example order within category
```

### 2. Golden Loader (`langingpage/src/lib/goldenLoader.ts`)

Extracts `category_order` from reasoning file frontmatter:

```typescript
category_order: frontmatter.category_order,
```

### 3. Index Page (`langingpage/src/pages/index.astro`)

Sorts examples with two-level sorting:

```typescript
const sortedExamples = allExamples.sort((a, b) => {
  // Primary: category_order (showcase=0 first)
  const categoryOrderA = a.data.category_order ?? 999;
  const categoryOrderB = b.data.category_order ?? 999;

  if (categoryOrderA !== categoryOrderB) {
    return categoryOrderA - categoryOrderB;
  }

  // Secondary: order within category
  return a.data.order - b.data.order;
});
```

### 4. Reasoning Files

Each `.reasoning.md` file has frontmatter:

```yaml
---
title: "Example Title"
category: "Category Name"
category_order: 10        # ← Controls category position
order: 1                  # ← Controls position within category
---
```

---

## Examples by Category

### Showcase (order: 0)
- `showcase_01_api_server` - Complete feature demonstration

### Error Handling (order: 10)
- `error_prop_01_simple` through `error_prop_09_multi_value`
- `01_simple_statement` (legacy name)

### Type System / Sum Types (order: 20)
- `sum_types_01_simple_enum` through `sum_types_05_nested`

### Result Type (order: 30)
- `result_01_basic` through `result_05_go_interop`

### Option Type (order: 40)
- `option_01_basic` through `option_04_go_interop`

### Functions / Lambdas (order: 50)
- `lambda_01_basic` through `lambda_04_higher_order`

### Control Flow / Pattern Matching (order: 60)
- `pattern_match_01_basic` through `pattern_match_04_exhaustive`

### Operators / Ternary (order: 70)
- `ternary_01_basic` through `ternary_03_complex`

### Operators / Null Coalescing (order: 80)
- `null_coalesce_01_basic` through `null_coalesce_03_with_option`

### Operators / Safe Navigation (order: 90)
- `safe_nav_01_basic` through `safe_nav_03_with_methods`

### Data Structures / Tuples (order: 100)
- `tuples_01_basic` through `tuples_03_nested`

### Functional Utilities (order: 110)
- `func_util_01_map` through `func_util_04_chaining`

---

## Adding New Categories

1. **Choose order value:**
   - Between existing categories? Use intermediate value (e.g., 15, 25)
   - After all categories? Use next increment (e.g., 120)

2. **Update all examples in category:**
   ```bash
   # Add to frontmatter of each .reasoning.md file:
   category_order: XX
   ```

3. **Update this documentation** with the new category

---

## Visitor Journey (Landing Page)

When visitors arrive at dingolang.com, they see examples in this order:

1. **🎪 Showcase** - Immediate "wow" with complete API server
2. **⚡ Error Propagation** - "This solves my daily pain!"
3. **🎭 Sum Types** - "85.7% code reduction? Amazing!"
4. **📦 Result/Option** - "Type-safe error handling!"
5. **🎯 Advanced Features** - Lambdas, pattern matching, etc.

This ordering maximizes **engagement** and **conversion** by showing:
- Most impressive features first
- Most practical features early
- Complexity gradually increases

---

## Maintenance

**When adding new examples:**
- Add `category_order` to frontmatter
- Use appropriate value from table above
- Update this documentation if adding new category

**When reordering categories:**
- Update `category_order` values in reasoning files
- Update table above
- Consider visitor journey impact

---

**Last Updated:** 2025-11-18
**Total Categories:** 12
**Total Examples:** 47 (46 feature-specific + 1 showcase)
