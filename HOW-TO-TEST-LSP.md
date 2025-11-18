# How to Test the Dingo LSP

**Quick Start:** 5 minutes to get LSP running in VSCode

---

## Option 1: Automated Quick Test (Recommended)

Run the automated setup script:

```bash
cd /Users/jack/mag/dingo
./scripts/lsp-quicktest.sh
```

This will:
- ✅ Check prerequisites (VSCode, gopls)
- ✅ Build dingo-lsp binary
- ✅ Install VSCode extension
- ✅ Open test file in VSCode

Then follow the on-screen instructions.

---

## Option 2: Manual Setup (5 steps)

### Step 1: Build LSP Binary
```bash
cd /Users/jack/mag/dingo
go build -o dingo-lsp cmd/dingo-lsp/main.go

# Create symlink in /usr/local/bin (recommended - always uses latest build!)
sudo ln -sf $(pwd)/dingo-lsp /usr/local/bin/dingo-lsp
```

### Step 2: Install VSCode Extension
```bash
code --install-extension editors/vscode/dingo-0.2.0.vsix
```

### Step 3: Configure VSCode
Open VSCode settings (Cmd+,) and add:
```json
{
  "dingo.lsp.path": "/Users/jack/mag/dingo/dingo-lsp",
  "dingo.transpileOnSave": true
}
```

### Step 4: Open Test File
```bash
code examples/lsp-demo/pattern-matching-test.dingo
```

### Step 5: Test Features

**Quick Tests:**
1. **Hover:** Line 26 - Hover over `value` → should show `value: int`
2. **Autocomplete:** Line 25 - Type `result.` → should show methods
3. **Go-to-definition:** Line 18 - F12 on `User` → jumps to definition
4. **Exhaustiveness:** Line 62 - Comment out `None` pattern → error appears

---

## What to Test

### Phase 3 Features
- ✅ Type annotations autocomplete
- ✅ Error propagation (`?` operator)
- ✅ Result/Option types hover
- ✅ Go-to-definition on types

### Phase 4.1 Features (NEW!)
- ✅ Pattern matching autocomplete
- ✅ Nested pattern type inference
- ✅ Exhaustiveness checking diagnostics
- ✅ None context inference

---

## Expected Results

| Test | Location | Action | Expected Result |
|------|----------|--------|-----------------|
| **Hover on pattern variable** | Line 26 | Hover over `value` | Shows `value: int` |
| **Nested pattern** | Line 46 | Hover over `user` | Shows `user: User` (destructured!) |
| **Exhaustiveness** | Line 62 | Remove `None` pattern | Error: "Non-exhaustive match" |
| **None inference** | Line 73 | Hover over `None` | Shows `None: Option[string]` |
| **Autocomplete** | Line 25 | Type `result.` | Shows `IsOk()`, `IsErr()`, etc. |

---

## Troubleshooting

### LSP Not Working?

1. **Check Output Panel:**
   - View → Output → Select "Dingo Language Server"
   - Look for errors

2. **Restart LSP:**
   - Cmd+Shift+P → "Dingo: Restart Language Server"

3. **Verify gopls:**
   ```bash
   gopls version
   # If not found: go install golang.org/x/tools/gopls@latest
   ```

### Need More Help?

- **Full Guide:** `docs/MANUAL-LSP-TESTING.md` (comprehensive 11-test suite)
- **Debugging:** `docs/lsp-debugging.md`
- **Architecture:** `pkg/lsp/README.md`

---

## Quick Verification Checklist

After 5 minutes, you should have:
- [ ] VSCode extension installed
- [ ] LSP server running (check Output panel)
- [ ] Hover shows types on pattern variables
- [ ] Autocomplete works in match expressions
- [ ] Exhaustiveness errors appear

**All checked?** LSP is working! 🎉

**Some failed?** See troubleshooting above or check full guide.

---

## Performance Expectations

- **Autocomplete:** <100ms (instant feel)
- **Hover:** <50ms (instant)
- **Go-to-definition:** <100ms
- **Diagnostics:** <500ms after save

---

**Last Updated:** 2025-11-18 (Phase V complete)
