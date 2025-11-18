# Dingo LSP Debugging Guide

This guide helps you troubleshoot issues with the Dingo Language Server (dingo-lsp) and IDE integration.

## Quick Diagnostics

### Is LSP Running?

**VSCode:**
1. Open Output panel (View → Output)
2. Select "Dingo Language Server" from dropdown
3. Look for `LSP server started` message

**Command Line:**
```bash
# Check if dingo-lsp binary exists
which dingo-lsp

# Test manually (type Ctrl+C to exit)
dingo-lsp
```

### Is gopls Installed?

```bash
# Check gopls is installed
gopls version

# If not installed
go install golang.org/x/tools/gopls@latest
```

### Is File Transpiled?

```bash
# Check if .go and .go.map files exist
ls -la myfile.go myfile.go.map

# If not, transpile manually
dingo build myfile.dingo
```

## Enabling Debug Logging

### VSCode

**Method 1: Settings UI**
1. Open Settings (Cmd+,)
2. Search "dingo.lsp.logLevel"
3. Set to "debug"
4. Restart LSP: Command Palette → "Dingo: Restart Language Server"

**Method 2: settings.json**
```json
{
  "dingo.lsp.logLevel": "debug"
}
```

### Command Line

```bash
# Set environment variable
export DINGO_LSP_LOG=debug

# Run dingo-lsp
dingo-lsp
```

### Reading Logs

**VSCode:**
- Output panel → "Dingo Language Server"
- Look for messages like:
  ```
  [DEBUG] Position translated: Dingo{10:15} → Go{18:22}
  [INFO]  Source map loaded: /path/file.go.map (version 1)
  [WARN]  gopls crashed, restarting (attempt 1/3)
  [ERROR] Transpilation failed: syntax error at line 42
  ```

**Command Line:**
- Logs printed to stderr
- Redirect to file: `dingo-lsp 2> lsp.log`

## Common Issues & Solutions

### 1. Autocomplete Not Working

**Symptom:** No suggestions appear when typing

**Possible Causes:**

**A. File Not Transpiled**
```bash
# Check if .go file exists
ls myfile.go myfile.go.map

# Solution: Transpile manually
dingo build myfile.dingo

# Or enable auto-transpile in VSCode settings
{
  "dingo.transpileOnSave": true
}
```

**B. gopls Not Installed**
```bash
# Check gopls
gopls version

# Solution: Install gopls
go install golang.org/x/tools/gopls@latest
```

**C. LSP Not Started**
- Check Output panel for errors
- Try: Command Palette → "Dingo: Restart Language Server"
- Check dingo-lsp binary exists: `which dingo-lsp`

**D. Syntax Error in .dingo File**
- LSP may be waiting for valid transpilation
- Fix syntax errors, save file
- Check for red squiggly lines (diagnostics)

### 2. Go-to-Definition Jumps to Wrong Line

**Symptom:** F12 jumps to incorrect position

**Possible Causes:**

**A. Stale Source Map**
```bash
# Source map may not match current .dingo file
# Solution: Re-transpile
dingo build myfile.dingo

# Or save file if auto-transpile enabled
```

**B. Multi-line Expansion Position**
- Some Dingo features expand to multiple Go lines (e.g., `?` operator)
- Source map may point to expanded code block start
- **Normal behavior** - gopls sees expanded Go code

**C. Source Map Version Mismatch**
- Check LSP logs for version errors
- Update dingo-lsp if source map version unsupported

### 3. Hover Shows Wrong Type Information

**Symptom:** Hovering over variable shows incorrect type

**Possible Causes:**

**A. Stale .go File**
- .go file may not match current .dingo file
- Solution: Save .dingo file (auto-transpile)

**B. gopls Cache Stale**
- gopls may have cached old type information
- Solution: Restart gopls via LSP restart

**C. Type Inference Limitation**
- gopls infers types from .go code
- If transpilation generates unexpected Go code, types may differ
- Check transpiled .go file: `cat myfile.go`

### 4. Diagnostics (Errors) Not Appearing

**Symptom:** No red squiggly lines for syntax errors

**Possible Causes:**

**A. Transpilation Succeeds (No Errors)**
- Check if code actually has errors
- Try: `dingo build myfile.dingo` manually

**B. LSP Not Forwarding Diagnostics**
- Check LSP logs for diagnostic messages
- Enable debug logging: `DINGO_LSP_LOG=debug`

**C. VSCode Cache Issue**
- Close and reopen file
- Restart VSCode

### 5. "File Not Transpiled" Error

**Symptom:** LSP shows "File not transpiled. Run 'dingo build' or enable auto-transpile."

**Solutions:**

**A. Manual Transpile**
```bash
dingo build myfile.dingo
```

**B. Enable Auto-Transpile**
```json
{
  "dingo.transpileOnSave": true
}
```

**C. Transpile Workspace**
- Command Palette → "Dingo: Transpile All Files in Workspace"

### 6. gopls Crashes Repeatedly

**Symptom:** LSP logs show "gopls crashed, restarting..."

**Possible Causes:**

**A. Invalid .go File**
- Transpiled .go file may have syntax errors
- Check: `go build myfile.go`
- If errors, report as Dingo transpiler bug

**B. gopls Version Issue**
- Update gopls: `go install golang.org/x/tools/gopls@latest`
- Dingo supports gopls v0.11+

**C. Large Workspace**
- gopls may run out of memory on huge workspaces
- Reduce workspace size or increase memory limit

**D. gopls Binary Corrupted**
- Reinstall gopls: `go install golang.org/x/tools/gopls@latest`

### 7. Unsupported Source Map Version

**Symptom:** Error: "Unsupported source map version 2 (max: 1)"

**Cause:** Transpiler (Phase 4) generated newer source map format

**Solution:**
1. Update dingo-lsp to latest version
2. If latest version still incompatible, file a bug report
3. Temporary workaround: Use older transpiler version

### 8. Auto-Transpile Not Working

**Symptom:** Saving .dingo file doesn't trigger transpilation

**Checks:**

**A. Setting Enabled?**
```json
{
  "dingo.transpileOnSave": true  // ← Check this
}
```

**B. File Watcher Running?**
- Check LSP logs: Should see "File watcher started"
- If not, LSP may have failed to initialize watcher

**C. File Not Detected?**
- File may be outside workspace root
- File may be in ignored directory (node_modules, vendor, .git)

**D. Debounce Delay**
- Auto-transpile has 500ms debounce
- Wait 1 second after save to see .go file update

### 9. High CPU Usage

**Symptom:** LSP or gopls using excessive CPU

**Possible Causes:**

**A. File Watcher on Large Directory**
- Watching too many files
- Check: `DINGO_LSP_LOG=debug` shows excessive file events
- Solution: Add directories to .gitignore or move .dingo files to smaller workspace

**B. gopls Indexing**
- gopls may be indexing workspace on first run
- Wait a few minutes for indexing to complete

**C. Rapid Auto-Transpile**
- Auto-save plugin triggering transpiles too frequently
- Solution: Increase debounce (code change needed) or disable auto-save

### 10. Position Off by 1 Line/Column

**Symptom:** Autocomplete or definition is off by exactly 1 line or column

**Cause:** LSP vs Source Map Indexing Difference
- LSP uses 0-based indexing (line 0 = first line)
- Source maps use 1-based indexing (line 1 = first line)
- Translator should handle conversion automatically

**If this happens:**
1. File a bug report with example .dingo file
2. Include source map (.go.map) in report
3. Specify exact Dingo position and expected vs actual Go position

## Debugging Workflow

### Step 1: Reproduce Issue

1. Create minimal .dingo file that reproduces issue
2. Note exact steps to trigger issue (e.g., "Type 'x.' at line 10")
3. Note expected vs actual behavior

### Step 2: Enable Debug Logging

```bash
# VSCode settings.json
{
  "dingo.lsp.logLevel": "debug"
}

# Restart LSP
Command Palette → "Dingo: Restart Language Server"
```

### Step 3: Capture Logs

**VSCode:**
1. Output panel → "Dingo Language Server"
2. Reproduce issue
3. Copy log output

**Command Line:**
```bash
DINGO_LSP_LOG=debug dingo-lsp 2> lsp-debug.log
# Trigger issue
# Check lsp-debug.log
```

### Step 4: Check Intermediate Files

```bash
# Check transpiled .go file
cat myfile.go

# Check source map
cat myfile.go.map | jq .  # jq for pretty-printing

# Check .go file compiles
go build myfile.go
```

### Step 5: Test gopls Directly

```bash
# Test gopls on transpiled .go file
gopls check myfile.go

# Test gopls completion at specific position
echo '{"line": 10, "character": 15}' | gopls completion myfile.go
```

### Step 6: Report Bug

If issue persists, file a bug report with:
- Minimal .dingo file that reproduces issue
- LSP debug logs
- Transpiled .go file
- Source map (.go.map)
- Expected vs actual behavior
- dingo-lsp version: `dingo-lsp --version`
- gopls version: `gopls version`
- Editor: VSCode version, or other editor

## Advanced Debugging

### Inspect LSP Communication

**VSCode:**
1. Settings → `"dingo.lsp.trace.server": "verbose"`
2. Output panel shows all LSP messages (JSON-RPC)

**Example LSP Request:**
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "textDocument/completion",
  "params": {
    "textDocument": {"uri": "file:///path/myfile.dingo"},
    "position": {"line": 9, "character": 14}
  }
}
```

**Example LSP Response:**
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "items": [
      {"label": "String", "kind": 5, "detail": "func() string"}
    ]
  }
}
```

### Attach Debugger to dingo-lsp

**Using Delve:**
```bash
# Install delve
go install github.com/go-delve/delve/cmd/dlv@latest

# Build dingo-lsp with debug symbols
go build -gcflags="all=-N -l" -o dingo-lsp cmd/dingo-lsp/main.go

# Start debugger
dlv exec ./dingo-lsp

# Set breakpoint
(dlv) break translator.go:123
(dlv) continue
```

### Test Position Translation in Isolation

```go
package main

import (
    "fmt"
    "github.com/MadAppGang/dingo/pkg/lsp"
    "github.com/MadAppGang/dingo/pkg/preprocessor"
)

func main() {
    sm := &preprocessor.SourceMap{
        Version: 1,
        Mappings: []preprocessor.Mapping{
            {OriginalLine: 10, OriginalColumn: 15, GeneratedLine: 18, GeneratedColumn: 22},
        },
    }

    cache := &mockCache{sm: sm}
    translator := lsp.NewTranslator(cache)

    uri, pos, err := translator.translatePosition(
        protocol.URIFromPath("test.dingo"),
        protocol.Position{Line: 9, Character: 14},  // 0-based
        lsp.DingoToGo,
    )

    fmt.Printf("Translated: %v → %v (err: %v)\n",
        protocol.Position{Line: 9, Character: 14}, pos, err)
}
```

## Performance Profiling

### Profile Position Translation

```bash
# Run benchmark
go test ./pkg/lsp -bench=BenchmarkPositionTranslation -benchmem

# Expected: <1ms per translation
```

### Profile Source Map Loading

```bash
go test ./pkg/lsp -bench=BenchmarkSourceMapLoad -benchmem

# Expected: <5ms per load
```

### Profile CPU Usage

```bash
# Run LSP with CPU profiling
go run -cpuprofile=cpu.prof cmd/dingo-lsp/main.go

# Analyze profile
go tool pprof cpu.prof
(pprof) top10
(pprof) list translatePosition
```

### Profile Memory Usage

```bash
# Run LSP with memory profiling
go run -memprofile=mem.prof cmd/dingo-lsp/main.go

# Analyze profile
go tool pprof mem.prof
(pprof) top10
(pprof) list SourceMapCache.Get
```

## Environment Variables Reference

| Variable | Values | Default | Purpose |
|----------|--------|---------|---------|
| `DINGO_LSP_LOG` | debug, info, warn, error | info | Log level |
| `DINGO_AUTO_TRANSPILE` | true, false | true | Auto-transpile on save |
| `GOPLS_LOG` | verbose, info, warn, error | - | gopls log level (passed through) |

## VSCode Settings Reference

| Setting | Type | Default | Purpose |
|---------|------|---------|---------|
| `dingo.lsp.path` | string | "dingo-lsp" | Path to LSP binary |
| `dingo.transpileOnSave` | boolean | true | Auto-transpile on save |
| `dingo.showTranspileNotifications` | boolean | false | Show transpile success/fail notifications |
| `dingo.lsp.logLevel` | enum | "info" | LSP log level (debug/info/warn/error) |
| `dingo.lsp.trace.server` | enum | "off" | LSP message tracing (off/messages/verbose) |

## Getting Help

**GitHub Issues:** https://github.com/MadAppGang/dingo/issues
**Documentation:** https://dingolang.com/docs/lsp
**Community:** (Discord/Slack link when available)

**When reporting issues, include:**
1. Minimal reproducible .dingo file
2. LSP debug logs
3. dingo-lsp version
4. gopls version
5. Editor and version
6. OS and version
