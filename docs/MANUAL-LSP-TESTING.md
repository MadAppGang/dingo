# Manual LSP Testing Guide

**Purpose:** Test the Dingo Language Server with VSCode to verify Phase 3 + 4.1 features work correctly.

**Estimated Time:** 10-15 minutes

---

## Prerequisites

Before testing, ensure you have:

1. ✅ **dingo-lsp binary** built and available
2. ✅ **gopls** installed (`go install golang.org/x/tools/gopls@latest`)
3. ✅ **dingo transpiler** working
4. ✅ **VSCode** installed

---

## Step 1: Install the LSP Binary

```bash
# From the dingo project root
cd /Users/jack/mag/dingo

# Build dingo-lsp
go build -o dingo-lsp cmd/dingo-lsp/main.go

# Make it available in PATH (choose one option):

# Option A: Create symlink in /usr/local/bin (RECOMMENDED - standard location, always uses latest build)
sudo ln -sf $(pwd)/dingo-lsp /usr/local/bin/dingo-lsp

# Option B: Create symlink in $GOPATH/bin (if you prefer keeping Go tools together)
GOPATH="${GOPATH:-$(go env GOPATH)}"
mkdir -p "$GOPATH/bin"
ln -sf $(pwd)/dingo-lsp "$GOPATH/bin/dingo-lsp"
# Then ensure $GOPATH/bin is in PATH: export PATH=$PATH:$(go env GOPATH)/bin

# Option C: Use absolute path in VSCode settings (see Step 2)

# Ensure $GOPATH/bin is in your PATH (add to ~/.zshrc or ~/.bashrc if needed)
export PATH=$PATH:$(go env GOPATH)/bin

# Verify installation
which dingo-lsp
# Or test directly:
$GOPATH/bin/dingo-lsp  # Should start (will error about gopls, that's ok)
```

---

## Step 2: Install VSCode Extension

```bash
# From the dingo project root
cd /Users/jack/mag/dingo/editors/vscode

# Install the extension
code --install-extension dingo-0.2.0.vsix

# Or manually:
# 1. Open VSCode
# 2. View → Extensions (Cmd+Shift+X)
# 3. Click "..." menu → Install from VSIX
# 4. Select: editors/vscode/dingo-0.2.0.vsix
```

**Verify installation:**
1. Open VSCode
2. View → Extensions
3. Search for "Dingo"
4. Should show "Dingo Language Support" v0.2.0

---

## Step 3: Configure VSCode Settings

Open VSCode settings (Cmd+, or Code → Preferences → Settings) and add:

```json
{
  "dingo.lsp.path": "/Users/jack/mag/dingo/dingo-lsp",
  "dingo.transpileOnSave": true,
  "dingo.lsp.logLevel": "debug"
}
```

**If you installed dingo-lsp in PATH:**
```json
{
  "dingo.lsp.path": "dingo-lsp",
  "dingo.transpileOnSave": true,
  "dingo.lsp.logLevel": "debug"
}
```

---

## Step 4: Open Test Files in VSCode

1. **Open the dingo project folder:**
   ```bash
   code /Users/jack/mag/dingo
   ```

2. **Open the LSP demo folder in the file explorer:**
   - Navigate to `examples/lsp-demo/`
   - You should see:
     - `demo.dingo`
     - `pattern-matching-test.dingo` (NEW)
     - `README.md`

3. **Open `pattern-matching-test.dingo`** in the editor

---

## Step 5: Verify LSP is Running

### Check LSP Connection

1. **Look at the bottom-right corner of VSCode**
   - You should see a status indicator (if the extension provides one)

2. **Check the Output panel:**
   - View → Output (Cmd+Shift+U)
   - Select "Dingo Language Server" from the dropdown
   - You should see logs like:
     ```
     [Info] Starting Dingo LSP server...
     [Info] gopls client started
     [Info] Server initialized
     ```

3. **If you see errors:**
   - Check `dingo.lsp.path` is correct
   - Verify `gopls` is installed: `gopls version`
   - Check logs: `DINGO_LSP_LOG=debug code .`

### Restart LSP if Needed

- Command Palette (Cmd+Shift+P) → "Dingo: Restart Language Server"

---

## Step 6: Test Phase 3 Features

### Test 1: Type Annotations - Hover

**File:** `pattern-matching-test.dingo`, line 8

1. Hover over `User` in the struct definition
2. **Expected:** Tooltip shows:
   ```
   type User struct {
       ID:    int
       Name:  string
       Email: string
   }
   ```

### Test 2: Error Propagation - Autocomplete

**File:** `pattern-matching-test.dingo`, line 21

1. After `result := Ok[int, error](42)`, add a new line
2. Type: `result.`
3. **Expected:** Autocomplete dropdown shows:
   - `IsOk() bool`
   - `IsErr() bool`
   - `Unwrap() int`
   - `UnwrapOr(defaultValue int) int`
   - (other Result methods)

### Test 3: Go-to-Definition

**File:** `pattern-matching-test.dingo`, line 18

1. Place cursor on `User` type
2. Press **F12** (or Cmd+Click)
3. **Expected:** Jumps to `User` struct definition (line 8)

### Test 4: Diagnostics

**File:** `pattern-matching-test.dingo`, line 33

1. Change `user.Name` to `user.Age` (invalid field)
2. **Expected:** Red squiggle appears with error:
   ```
   user.Age undefined (type User has no field or method Age)
   ```

---

## Step 7: Test Phase 4.1 Pattern Matching Features

### Test 5: Pattern Matching - Autocomplete

**File:** `pattern-matching-test.dingo`, line 25

1. In the `match result {` block, start typing a new pattern
2. Type: `Ok(`
3. **Expected:** Autocomplete suggests `Ok(value)`
4. After completing: Type `) =>`
5. **Expected:** Pattern binding works

### Test 6: Hover on Pattern Variables

**File:** `pattern-matching-test.dingo`, line 26

1. Hover over `value` in the pattern `Ok(value) =>`
2. **Expected:** Tooltip shows: `value: int`

### Test 7: Nested Patterns - Type Inference

**File:** `pattern-matching-test.dingo`, line 46

1. Hover over `user` in pattern `Ok(Some(user)) =>`
2. **Expected:** Tooltip shows: `user: User` (NOT `Option[User]` - destructured!)

### Test 8: Exhaustiveness Checking

**File:** `pattern-matching-test.dingo`, line 62

1. **Current state:** Match has both `Some(x)` and `None` patterns (complete)
2. **Comment out the `None` pattern:**
   ```dingo
   match opt {
       Some(x) => fmt.Printf("Value: %d\n", x)
       // None => fmt.Println("No value")  // COMMENTED OUT
   }
   ```
3. **Expected:** Diagnostic error appears:
   ```
   Non-exhaustive match: missing pattern for None
   ```

### Test 9: None Context Inference

**File:** `pattern-matching-test.dingo`, line 73

1. Hover over `None` in `return None`
2. **Expected:** Tooltip shows: `None: Option[string]` (inferred from return type!)

### Test 10: Enum Pattern Matching

**File:** `pattern-matching-test.dingo`, line 97

1. In the enum match block, hover over `percent` in:
   ```dingo
   Status.InProgress{percent} => {
   ```
2. **Expected:** Tooltip shows: `percent: int`

### Test 11: Wildcard Pattern

**File:** `pattern-matching-test.dingo`, line 113

1. Hover over `_` in `Err(_) =>`
2. **Expected:** May show generic info or no hover (wildcards ignore values)

---

## Step 8: Test Auto-Transpile on Save

1. **Open:** `pattern-matching-test.dingo`
2. **Make a small change:** Add a comment or blank line
3. **Save:** Cmd+S
4. **Expected:**
   - File saves
   - Transpiler runs automatically (you may see a brief status message)
   - If transpilation succeeds: No errors
   - If transpilation fails: Diagnostic errors appear

**Verify transpilation:**
```bash
# Check that .go file was created/updated
ls -l examples/lsp-demo/pattern-matching-test.go

# Check timestamp is recent (just updated)
stat examples/lsp-demo/pattern-matching-test.go
```

---

## Step 9: Test with Complex Nested Patterns

**File:** `pattern-matching-test.dingo`, line 127

1. Navigate to the `testComplexNesting()` function
2. Hover over `percent` in the deeply nested pattern:
   ```dingo
   Event.UserAction{action: Status.InProgress{percent}} => {
   ```
3. **Expected:** Tooltip shows: `percent: int` (correctly destructured from nested enum!)

---

## Step 10: Performance Check

### Test Autocomplete Latency

1. In any file, type a variable name and `.`
2. **Expected:** Autocomplete appears in **<100ms** (should feel instant)
3. If slow: Check gopls is running (`ps aux | grep gopls`)

### Test Position Translation Accuracy

1. Use go-to-definition (F12) multiple times on different types
2. **Expected:** Always jumps to correct location in .dingo file (not .go file)

---

## Common Issues and Solutions

### Issue 1: "Language server not found"

**Symptoms:** No autocomplete, hover, or diagnostics

**Solutions:**
1. Verify `dingo-lsp` path in settings
2. Check Output panel for errors
3. Restart LSP: Cmd+Shift+P → "Dingo: Restart Language Server"

### Issue 2: "gopls not found"

**Symptoms:** LSP starts but no Go features work

**Solutions:**
1. Install gopls: `go install golang.org/x/tools/gopls@latest`
2. Add to PATH: `export PATH=$PATH:$(go env GOPATH)/bin`
3. Restart LSP

### Issue 3: Autocomplete shows .go positions

**Symptoms:** Go-to-definition jumps to .go files instead of .dingo

**Solutions:**
1. Check source maps exist: `ls examples/lsp-demo/*.go.map`
2. Transpile manually: `dingo build examples/lsp-demo/pattern-matching-test.dingo`
3. Restart LSP

### Issue 4: Pattern matching errors not showing

**Symptoms:** Non-exhaustive matches don't show diagnostics

**Solutions:**
1. Ensure transpilation on save is working
2. Check transpiler output for errors
3. Manually transpile to see errors: `dingo build file.dingo`

---

## Test Results Checklist

After completing all tests, verify:

- [ ] Autocomplete works (Phase 3 features)
- [ ] Go-to-definition jumps to .dingo files
- [ ] Hover shows correct types
- [ ] Diagnostics appear for errors
- [ ] Pattern matching autocomplete works (Phase 4.1)
- [ ] Nested pattern hover shows correct types (Phase 4.1)
- [ ] Exhaustiveness errors appear (Phase 4.1)
- [ ] None context inference works (Phase 4.1)
- [ ] Auto-transpile on save works
- [ ] Performance is acceptable (<100ms autocomplete)

---

## Advanced: Debugging LSP

### Enable Debug Logging

**Terminal 1: Start LSP with debug logs**
```bash
cd /Users/jack/mag/dingo
DINGO_LSP_LOG=debug ./dingo-lsp
```

**Terminal 2: Connect VSCode**
Update VSCode settings to use stdio instead of spawning:
```json
{
  "dingo.lsp.connect": "stdio"  // If supported by extension
}
```

### View LSP Communication

**Check logs:**
- VSCode Output panel → "Dingo Language Server"
- Look for request/response pairs:
  ```
  [Debug] Request: textDocument/completion
  [Debug] Translated position: .dingo:42:10 → .go:58:15
  [Debug] gopls response received
  [Debug] Translated back: .go:58:15 → .dingo:42:10
  [Debug] Response sent
  ```

---

## Success Criteria

**LSP is working correctly if:**

✅ All 11 tests pass
✅ Autocomplete feels instant (<100ms)
✅ Go-to-definition always jumps to .dingo files
✅ Pattern matching features work (hover, autocomplete, exhaustiveness)
✅ Auto-transpile on save works without errors

**If all criteria met:** Phase V LSP is production-ready! 🎉

---

## Next Steps After Testing

1. **Report results:** Document which tests passed/failed
2. **File bugs:** If any features don't work, note specific test cases
3. **Create git commit:** Commit Phase V implementation
4. **User testing:** Share with early adopters

---

**Questions or Issues?**
- Check `docs/lsp-debugging.md` for detailed troubleshooting
- Review `pkg/lsp/README.md` for architecture details
- See `examples/lsp-demo/README.md` for more test ideas
