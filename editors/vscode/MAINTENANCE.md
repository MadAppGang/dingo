# VS Code Extension Maintenance Guide

This guide explains how to maintain and update the Dingo VS Code extension when adding new language features.

## Quick Reference

```bash
# Build grammar from YAML
npm run build-grammar

# Package extension
npm run package

# Install locally
code --install-extension dingo-*.vsix
```

## Workflow: Adding a New Feature

### 1. Read the Feature Spec

Before updating the extension, review the feature documentation in `/features/`:

```bash
cat ../features/your-feature.md
```

Identify:
- New keywords (e.g., `match`, `enum`)
- New operators (e.g., `?`, `??`)
- New type syntax (e.g., `Result<T, E>`)
- Example code snippets

### 2. Update the Grammar

Edit `syntaxes/dingo.tmLanguage.yaml` (YAML is easier to maintain than JSON).

#### Example: Adding Ternary Operator

From `features/ternary-operator.md`:
```dingo
result := condition ? valueIfTrue : valueIfFalse
```

Add to `syntaxes/dingo.tmLanguage.yaml`:

```yaml
repository:
  operators:
    patterns:
      # ... existing patterns ...

      - name: keyword.operator.ternary.dingo
        match: \?(?!\?)
        comment: Ternary operator (? but not ??)
```

**Important**: Place specific patterns BEFORE general ones (e.g., `??` before `?`).

### 3. Build the Grammar

```bash
npm run build-grammar
```

This converts `dingo.tmLanguage.yaml` → `dingo.tmLanguage.json`.

### 4. Create Example File

Add a test file to `examples/`:

```bash
# examples/ternary.dingo
package main

func main() {
    age := 25
    category := age >= 18 ? "adult" : "minor"
    println(category)
}
```

### 5. Test in VS Code

#### Option A: Extension Development Host

1. Open `editors/vscode` in VS Code
2. Press `F5` to launch Extension Development Host
3. Open `examples/your-feature.dingo`
4. Verify syntax highlighting

#### Option B: Install Locally

```bash
npm run package
code --install-extension dingo-*.vsix
```

### 6. Use Scope Inspector

**Command Palette** → `Developer: Inspect Editor Tokens and Scopes`

Click on a token to see:
- Token type
- Scope name
- Applied theme rules

**Common Issues:**
- Token not highlighted → Check regex pattern
- Wrong color → Check scope name against TextMate conventions
- Scope not found → Pattern isn't matching, test regex separately

### 7. Update Version and Changelog

Edit `package.json`:
```json
{
  "version": "0.2.0"
}
```

Add to main project `CHANGELOG.md`:
```markdown
## [0.2.0] - 2025-XX-XX
### Added
- VS Code extension: Ternary operator syntax highlighting
```

### 8. Commit Changes

```bash
git add editors/vscode/
git commit -m "feat(vscode): add ternary operator syntax highlighting"
```

## Grammar Patterns Cheat Sheet

### Keywords

```yaml
- name: keyword.control.dingo
  match: \b(if|else|for|while)\b
```

### Operators

```yaml
- name: keyword.operator.something.dingo
  match: \+\+|--
```

### Types

```yaml
- name: entity.name.type.custom.dingo
  match: \bMyType\s*<
```

### Functions

```yaml
- name: meta.function.declaration.dingo
  begin: \b(func)\s+([a-zA-Z_]\w*)\s*\(
  beginCaptures:
    1: { name: keyword.declaration.function.dingo }
    2: { name: entity.name.function.dingo }
  end: \)
```

### Complex Patterns (begin/end)

```yaml
- name: string.quoted.double.dingo
  begin: "\""
  end: "\""
  patterns:
    - name: constant.character.escape.dingo
      match: \\.
```

## Scope Naming Conventions

Use standard TextMate scopes for maximum theme compatibility:

| Element | Scope |
|---------|-------|
| Keywords | `keyword.control.*`, `keyword.declaration.*` |
| Operators | `keyword.operator.*` |
| Types | `entity.name.type.*`, `storage.type.*` |
| Functions | `entity.name.function.*` |
| Variables | `variable.other.*`, `variable.parameter.*` |
| Strings | `string.quoted.*` |
| Numbers | `constant.numeric.*` |
| Comments | `comment.line.*`, `comment.block.*` |
| Constants | `constant.language.*` |

**Reference:** [TextMate Scope Naming](https://macromates.com/manual/en/language_grammars)

## Common Patterns

### Feature: Default Parameters

From `features/default-parameters.md`:
```dingo
func greet(name: string = "World") {
    println("Hello, ${name}!")
}
```

Grammar update:
```yaml
parameters:
  patterns:
    - match: ([a-z_]\w*)\s*(:)\s*(\w+)\s*(=)
      captures:
        1: { name: variable.parameter.dingo }
        2: { name: keyword.operator.type-annotation.dingo }
        3: { name: storage.type.dingo }
        4: { name: keyword.operator.assignment.dingo }
```

### Feature: Tuple Destructuring

From `features/tuples.md`:
```dingo
let (x, y) = getPoint()
```

Grammar update:
```yaml
tuples:
  patterns:
    - name: meta.tuple.destructuring.dingo
      begin: \(
      end: \)
      patterns:
        - include: '#variables'
```

## Testing Checklist

Before committing:

- [ ] Grammar builds without errors (`npm run build-grammar`)
- [ ] All example files have correct highlighting
- [ ] Tested across themes: Dark+, Light+, Monokai, Material
- [ ] Scope Inspector shows expected scopes
- [ ] No regressions in existing highlighting
- [ ] Version bumped in `package.json`
- [ ] Changes documented in main `CHANGELOG.md`

## Regex Tips

### Escaping in YAML

Characters that need escaping: `\ . * + ? [ ] ( ) { } ^ $ |`

```yaml
# Match literal ? (error propagation)
match: \?

# Match literal | (lambda params)
match: \|

# Match word boundary
match: \bfunc\b
```

### Lookahead/Lookbehind

```yaml
# Positive lookahead: match "func" only if followed by "("
match: \bfunc\b(?=\s*\()

# Negative lookahead: match "?" but not "??"
match: \?(?!\?)

# Positive lookbehind: match type after ":"
match: (?<=:\s*)\w+
```

### Common Patterns

```yaml
# Identifier: [a-zA-Z_][a-zA-Z0-9_]*
match: \b[a-zA-Z_]\w*\b

# Type (PascalCase): [A-Z][a-zA-Z0-9_]*
match: \b[A-Z]\w*\b

# Number with underscores: 1_000_000
match: \b[0-9][0-9_]*\b
```

## Troubleshooting

### Pattern Not Matching

1. Test regex at [regex101.com](https://regex101.com) (use JavaScript flavor)
2. Check pattern order - specific before general
3. Use Scope Inspector to see what scope is applied
4. Check for conflicting patterns

### Wrong Color

1. Scope name doesn't follow TextMate conventions
2. Theme doesn't support your custom scope
3. Check theme's `tokenColors` in settings

### Performance Issues

1. Avoid backtracking regexes (use atomic groups)
2. Don't use `.*` without end anchors
3. Limit `begin`/`end` nesting depth
4. Test with large files (>1000 lines)

## Resources

- [VS Code Language Extension Guide](https://code.visualstudio.com/api/language-extensions/overview)
- [TextMate Grammar Guide](https://macromates.com/manual/en/language_grammars)
- [Oniguruma Regex](https://github.com/kkos/oniguruma/blob/master/doc/RE) (engine used by TextMate)
- [Example: Go Grammar](https://github.com/dannymcgee/vscode-go-grammar)
- [Scope Inspector](https://code.visualstudio.com/api/language-extensions/syntax-highlight-guide#scope-inspector)

## FAQ

**Q: Should I edit JSON or YAML?**
A: Always edit YAML (`dingo.tmLanguage.yaml`), then build to JSON. YAML is easier to maintain with comments and multiline strings.

**Q: How do I test a specific pattern?**
A: Use regex101.com with JavaScript flavor, or create a minimal test file in `examples/`.

**Q: What if a theme doesn't highlight correctly?**
A: Use standard TextMate scopes (see table above). Custom scopes won't work with most themes.

**Q: Can I nest patterns?**
A: Yes, use `begin`/`end` with `patterns` array. See `strings` for an example.

**Q: How often should I bump the version?**
A: For each feature addition, bump minor version (0.1.0 → 0.2.0). For bug fixes, bump patch (0.1.0 → 0.1.1).
