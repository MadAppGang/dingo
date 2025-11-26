# Dingo LSP Demo Example

This example demonstrates the Dingo Language Server Protocol (LSP) integration with Phase 3 + 4.1 features.

## Features Demonstrated

### Phase 3 Features
1. **Type Annotations** (`: Type` syntax)
2. **Error Propagation** (`?` operator)
3. **Result<T,E> Types** with helper methods
4. **Option<T> Types** with helper methods
5. **Sum Types (Enums)** with tagged unions

### Phase 4.1 Features (NEW!)
6. **Pattern Matching** (`match` expressions with Rust-style syntax)
7. **Exhaustiveness Checking** (compile-time verification)
8. **Nested Patterns** (`Ok(Some(value))` destructuring)
9. **None Context Inference** (smart type inference for None)
10. **Enum Pattern Matching** with field destructuring

## Files

- `demo.dingo` - Main demo file showcasing LSP features (Phase 3 + 4.1)
- `pattern-matching-test.dingo` - Comprehensive pattern matching examples (Phase 4.1)
- `README.md` - This file

## LSP Features to Test

### In `demo.dingo`:
- **Autocomplete:** Type `result.` or `user.` to see available methods/fields
- **Go-to-definition:** Press F12 on `User` or any type to jump to definition
- **Hover:** Hover over variables to see their types
- **Diagnostics:** Change field names to see inline errors
- **Pattern matching:** Test autocomplete in match expressions

### In `pattern-matching-test.dingo`:
- **Basic patterns:** Test autocomplete for `Ok`/`Err` patterns
- **Nested patterns:** Hover over destructured variables to see inferred types
- **Exhaustiveness:** Remove a pattern to see exhaustiveness check error
- **None inference:** Hover over `None` in different contexts to see type inference
- **Enum patterns:** Test autocomplete for enum variant patterns
- **Wildcards:** Test `_` pattern behavior

## Setup

### Prerequisites

1. **Dingo transpiler:**
   ```bash
   # Ensure dingo is installed
   dingo version
   ```

2. **gopls (Go language server):**
   ```bash
   # Install gopls
   go install golang.org/x/tools/gopls@latest

   # Verify installation
   gopls version
   ```

3. **dingo-lsp (Dingo language server):**
   ```bash
   # Build from source (run from dingo project root)
   go build -o dingo-lsp cmd/dingo-lsp/main.go

   # Add to PATH or copy to $GOPATH/bin
   cp dingo-lsp $GOPATH/bin/
   ```

4. **VSCode extension:**
   ```bash
   # Install extension (run from dingo project root)
   code --install-extension editors/vscode/dingo-0.2.0.vsix
   ```

### Configuration

Create or update VSCode settings (`.vscode/settings.json`):

```json
{
  "dingo.lsp.path": "dingo-lsp",
  "dingo.transpileOnSave": true,
  "dingo.lsp.logLevel": "info"
}
```

## Testing LSP Features

### 1. Autocomplete

Open `demo.dingo` in VSCode.

**Test 1: Method Autocomplete**
1. Line 18: Type `result.` and press Ctrl+Space
2. Should see: `IsOk()`, `IsErr()`, `Unwrap()`, `UnwrapOr()`, etc.

**Test 2: Type Autocomplete**
1. Line 22: Type `opt := Option[` and press Ctrl+Space
2. Should see Go built-in types: `int`, `string`, `bool`, etc.

**Test 3: Variable Autocomplete**
1. Line 30: Type `use` and press Ctrl+Space
2. Should see: `user`, `userData`

### 2. Go-to-Definition

**Test 1: Type Definition**
1. Line 10: Right-click on `User` in `User{...}`
2. Select "Go to Definition" (or press F12)
3. Should jump to line 6 (type User struct definition)

**Test 2: Function Definition**
1. Line 18: Right-click on `fetchUserData` call
2. Select "Go to Definition"
3. Should jump to line 12 (function definition)

**Test 3: Field Definition**
1. Line 30: Right-click on `Name` in `user.Name`
2. Select "Go to Definition"
3. Should jump to line 7 (Name field in User struct)

### 3. Hover Information

**Test 1: Variable Type**
1. Line 18: Hover over `result`
2. Should show: `result: Result[User, error]`

**Test 2: Function Signature**
1. Line 12: Hover over `fetchUserData`
2. Should show: `func fetchUserData(id: int) Result[User, error]`

**Test 3: Method Documentation**
1. Line 19: Hover over `IsOk()`
2. Should show: `func (r Result[T, E]) IsOk() bool`

### 4. Diagnostics (Error Detection)

**Test 1: Syntax Error**
1. Line 30: Remove closing `}` from `if` statement
2. Save file (auto-transpile)
3. Should see red squiggly line with error message

**Test 2: Type Error**
1. Line 31: Change `user.Name` to `user.Age` (non-existent field)
2. Save file
3. Should see error: "undefined: Age"

**Test 3: Unused Variable**
1. Line 22: Comment out line 23 (where `opt` is used)
2. Save file
3. Should see warning: "opt declared but not used"

### 5. Auto-Transpile

**Test 1: File Created**
1. Make any edit to `demo.dingo`
2. Save file (Cmd+S)
3. Check that `demo.go` and `demo.go.map` are created/updated
4. Timestamp should match save time

**Test 2: Auto-Transpile Disabled**
1. VSCode settings: `"dingo.transpileOnSave": false`
2. Edit and save `demo.dingo`
3. `demo.go` should NOT update
4. Manual transpile: Command Palette → "Dingo: Transpile Current File"

### 6. Error Propagation (`?` Operator)

**Test 1: Position Translation**
1. Line 18: Place cursor on `?` in `fetchUserData(123)?`
2. Request hover (move mouse over `?`)
3. Should show type information for expanded error handling code

**Test 2: Multi-line Expansion**
1. Line 18: Right-click on `?`
2. "Go to Definition" on any variable inside
3. Check transpiled `demo.go` to see expanded 7-line error handling
4. All lines should map back to same Dingo position

## Commands

### Transpile Commands

**Transpile Current File:**
```
Command Palette → "Dingo: Transpile Current File"
```

**Transpile Workspace:**
```
Command Palette → "Dingo: Transpile All Files in Workspace"
```

### LSP Commands

**Restart Language Server:**
```
Command Palette → "Dingo: Restart Language Server"
```
Use this if LSP becomes unresponsive or after updating dingo-lsp binary.

## Debugging

### Enable Debug Logging

VSCode settings:
```json
{
  "dingo.lsp.logLevel": "debug"
}
```

View logs:
1. Open Output panel (View → Output)
2. Select "Dingo Language Server" from dropdown
3. Look for translation messages:
   ```
   [DEBUG] Position translated: Dingo{18:25} → Go{45:22}
   [INFO]  Source map loaded: demo.go.map (version 1)
   ```

### Check Transpiled Files

```bash
# View transpiled Go code
cat demo.go

# View source map (pretty-printed)
cat demo.go.map | jq .

# Verify Go code compiles
go build demo.go
```

### Test gopls Directly

```bash
# Check gopls sees transpiled file
gopls check demo.go

# Should show no errors if transpilation succeeded
```

## Expected Results

When LSP is working correctly:

✅ **Autocomplete:** Suggestions appear instantly (<100ms)
✅ **Go-to-Definition:** Jumps to correct line in .dingo file (not .go)
✅ **Hover:** Shows accurate type information
✅ **Diagnostics:** Errors appear inline with red squiggly lines
✅ **Auto-Transpile:** .go file updates within 500ms of save
✅ **Performance:** No lag when typing or navigating

## Troubleshooting

See [LSP Debugging Guide](../../docs/lsp-debugging.md) for detailed troubleshooting.

**Quick fixes:**

**No autocomplete?**
- Ensure file is transpiled: `dingo build demo.dingo`
- Check gopls is installed: `gopls version`
- Restart LSP: Command Palette → "Dingo: Restart Language Server"

**Wrong positions?**
- Source map may be stale
- Save file to re-transpile
- Check `demo.go.map` exists

**LSP crashes?**
- Check logs: Output panel → "Dingo Language Server"
- Enable debug logging
- Check gopls is running: `ps aux | grep gopls`

## Next Steps

After testing this demo:

1. **Create your own .dingo files** in this directory
2. **Test more complex features:** Nested Result types, multiple `?` operators, enum pattern matching
3. **Try different editors:** Neovim, Sublime (LSP-compatible)
4. **Report bugs:** If LSP behaves unexpectedly, file an issue with this demo as minimal reproduction

## Performance Benchmarks

Expected performance (run benchmarks with `go test ./pkg/lsp/... -bench=.`):

- **Position Translation:** <1ms per translation
- **Autocomplete Latency:** <100ms total (IDE → IDE)
- **Source Map Load:** <5ms (first load), <1μs (cached)
- **File Watcher CPU:** <5% idle

## License

Part of the Dingo project - see root LICENSE file.
