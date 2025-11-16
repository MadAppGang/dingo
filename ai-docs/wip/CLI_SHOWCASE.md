# Dingo CLI - Beautiful Terminal Output ✨

Built with [lipgloss](https://github.com/charmbracelet/lipgloss) by Charm, the Dingo CLI provides a beautiful, colorful developer experience.

## 🎨 Features

- **🌈 Full color support** - Carefully chosen color palette for readability
- **📊 Clear progress tracking** - See exactly what's happening at each step
- **⚡ Performance metrics** - Timing for each build stage
- **🎯 Helpful error messages** - Clear, actionable error output
- **🦕 Branded design** - Consistent Dingo branding throughout

## 📸 Screenshots

### Version Command

```bash
$ dingo version
```

```
╭────────────╮
│  🦕 Dingo  │
╰────────────╯

  Version: 0.1.0-alpha
  Runtime: Go
  Website: https://dingo-lang.org
```

### Single File Build (Success)

```bash
$ dingo build hello.dingo
```

```
╭─────────────────────╮
│  🦕 Dingo Compiler  │
╰─────────────────────╯
                        v0.1.0-alpha

📦 Building 1 file

  examples/hello.dingo → examples/hello.go

  ✓ Parse       Done (497µs)
  ○ Transform   Skipped
    no plugins enabled
  ✓ Generate    Done (163µs)
  ✓ Write       Done (152µs)
    132 bytes written

────────────────────────

✨ Success! Built in 1ms
```

### Multiple Files Build

```bash
$ dingo build examples/*.dingo
```

```
╭─────────────────────╮
│  🦕 Dingo Compiler  │
╰─────────────────────╯
                        v0.1.0-alpha

📦 Building 3 files

  examples/hello.dingo → examples/hello.go

  ✓ Parse       Done (419µs)
  ○ Transform   Skipped
    no plugins enabled
  ✓ Generate    Done (64µs)
  ✓ Write       Done (150µs)
    132 bytes written

  examples/math.dingo → examples/math.go

  ✓ Parse       Done (715µs)
  ○ Transform   Skipped
    no plugins enabled
  ✓ Generate    Done (53µs)
  ✓ Write       Done (147µs)
    192 bytes written

  examples/utils.dingo → examples/utils.go

  ✓ Parse       Done (322µs)
  ○ Transform   Skipped
    no plugins enabled
  ✓ Generate    Done (23µs)
  ✓ Write       Done (84µs)
    97 bytes written

────────────────────────

✨ Success! Built in 2ms
```

### Build Error

```bash
$ dingo build broken.dingo
```

```
╭─────────────────────╮
│  🦕 Dingo Compiler  │
╰─────────────────────╯
                        v0.1.0-alpha

📦 Building 1 file

  examples/broken.dingo → examples/broken.go

  ✗ Parse       Failed (385µs)
  ✗ Error: parse error: unexpected token "return" (expected Block)

─────────────────────────────────────────────────────────

💥 Build failed
   Error: parse error: unexpected token "return"
```

## 🎨 Color Palette

The Dingo CLI uses a carefully chosen color palette optimized for readability in both light and dark terminals:

| Element | Color | Hex |
|---------|-------|-----|
| **Primary (Purple)** | Dingo brand color | `#7D56F4` |
| **Secondary (Cyan)** | Section headers | `#56C3F4` |
| **Success (Green)** | Successful operations | `#5AF78E` |
| **Warning (Yellow)** | Warnings, skipped steps | `#F7DC6F` |
| **Error (Pink/Red)** | Errors, failures | `#FF6B9D` |
| **Muted (Gray)** | Secondary information | `#6C7086` |
| **Text (Light)** | Primary text | `#CDD6F4` |
| **Highlight** | File paths, links | `#F5E0DC` |

## 🏗️ Build Steps

Each build shows 4 clear steps:

1. **✓ Parse** - Parsing Dingo source to AST
   - Status: Success ✓ / Failed ✗
   - Shows timing in µs/ms

2. **○ Transform** - Running plugin transformations
   - Status: Success ✓ / Skipped ○ / Failed ✗
   - Shows which plugins are active

3. **✓ Generate** - Generating Go source code
   - Status: Success ✓ / Failed ✗
   - Shows timing

4. **✓ Write** - Writing output file
   - Status: Success ✓ / Failed ✗
   - Shows bytes written

## 📦 Status Icons

- `✓` Success
- `✗` Error/Failed
- `○` Skipped
- `⚠` Warning
- `📦` Package/Build
- `🦕` Dingo branding
- `✨` Success summary
- `💥` Failure summary
- `ℹ` Information

## 🚀 Design Principles

1. **Non-interactive** - Clean, scrollable output for CI/CD
2. **Scannable** - Easy to find what you're looking for
3. **Informative** - All relevant information at a glance
4. **Beautiful** - Professional, polished appearance
5. **Performant** - Minimal overhead from styling

## 🛠️ Implementation

Built using:
- **[lipgloss](https://github.com/charmbracelet/lipgloss)** - Terminal styling and layout
- **[cobra](https://github.com/spf13/cobra)** - CLI framework
- **Custom UI package** - `pkg/ui/styles.go` (~365 lines)

## 📚 Future Enhancements

Planned improvements:
- [ ] File size comparisons (Dingo vs Go)
- [ ] Plugin activity visualization
- [ ] Build statistics (lines of code, etc.)
- [ ] Watch mode with live updates
- [ ] Color theme customization
- [ ] JSON output mode for CI/CD
