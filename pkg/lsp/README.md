# Dingo Language Server Protocol (LSP) Implementation

**Package:** `github.com/MadAppGang/dingo/pkg/lsp`
**Purpose:** LSP proxy server for Dingo language, providing full IDE support via gopls integration
**Status:** Phase V Iteration 1 Complete (Phase 3 + 4.1 features supported)

## Architecture Overview

### Three-Layer Design

```
┌─────────────────────────────────────────────────────────────┐
│ Layer 1: IDE (VSCode, Neovim, etc.)                         │
│ • Speaks: LSP protocol                                      │
│ • Sees: .dingo files                                        │
│ • Users: Developers                                         │
└─────────────────────────────────────────────────────────────┘
                           ↕ LSP via stdin/stdout
┌─────────────────────────────────────────────────────────────┐
│ Layer 2: dingo-lsp (Proxy Server - This Implementation)     │
│ • Receives LSP requests for .dingo files                    │
│ • Translates positions: .dingo → .go (via source maps)      │
│ • Forwards translated requests → gopls                      │
│ • Receives gopls responses                                  │
│ • Translates positions back: .go → .dingo                   │
│ • Returns responses to IDE                                  │
│ • Auto-transpiles on save (configurable)                    │
└─────────────────────────────────────────────────────────────┘
                           ↕ LSP via stdin/stdout
┌─────────────────────────────────────────────────────────────┐
│ Layer 3: gopls (Native Go Language Server)                  │
│ • Speaks: LSP protocol                                      │
│ • Sees: .go files (transpiled output)                       │
│ • Provides: Type checking, autocomplete, definitions, etc.  │
└─────────────────────────────────────────────────────────────┘
```

### Core Insight: Zero Feature Reimplementation

We do NOT reimplement Go language features. We only:
1. Transpile `.dingo` → `.go` (already implemented in Phase 3 + 4.1)
2. Translate LSP request positions (`.dingo` coordinates → `.go` coordinates)
3. Forward requests to gopls (which analyzes `.go` files)
4. Translate LSP response positions (`.go` coordinates → `.dingo` coordinates)
5. Return responses to IDE

**Result:** Full IDE support (autocomplete, hover, go-to-definition, diagnostics) with minimal code.

## Components

### 1. LSP Server (`server.go`)

**Responsibilities:**
- LSP request/response handling
- Method routing (which requests need translation)
- Lifecycle management (initialize, shutdown)
- Workspace configuration

**Key Methods:**
- `NewServer()` - Create server with configuration
- `Serve()` - Start serving LSP requests via stdio
- `handleRequest()` - Route LSP method to appropriate handler
- `handleInitialize()` - Initialize workspace, start gopls
- `handleCompletion()` - Autocomplete with position translation
- `handleDefinition()` - Go-to-definition with position translation
- `handleHover()` - Hover information with position translation
- `handleDidSave()` - Auto-transpile on save (configurable)

**Example:**
```go
server, err := lsp.NewServer(lsp.ServerConfig{
    Logger:        logger,
    GoplsPath:     "gopls",
    AutoTranspile: true,
})
if err != nil {
    log.Fatal(err)
}

stream := jsonrpc2.NewStream(os.Stdin, os.Stdout)
conn := jsonrpc2.NewConn(stream)
server.Serve(context.Background(), conn)
```

### 2. gopls Client (`gopls_client.go`)

**Responsibilities:**
- Manage gopls subprocess lifecycle
- Forward LSP requests to gopls
- Handle crashes and auto-restart
- Initialization handshake

**Features:**
- Auto-restart on crash (max 3 attempts)
- Graceful shutdown
- Stderr logging (debug mode)

**Example:**
```go
client, err := lsp.NewGoplsClient("gopls", logger)
if err != nil {
    log.Fatal(err)
}
defer client.Shutdown(context.Background())

result, err := client.Completion(ctx, params)
```

### 3. Position Translator (`translator.go`)

**Responsibilities:**
- Translate LSP positions using source maps
- Handle different request/response types
- Bidirectional translation (Dingo ↔ Go)
- Edge case handling (unmapped positions, multi-line expansions)

**Translation Direction:**
```go
const (
    DingoToGo Direction = iota  // .dingo → .go
    GoToDingo                    // .go → .dingo
)
```

**Example:**
```go
translator := lsp.NewTranslator(mapCache)

// Translate request
goParams, err := translator.TranslateCompletionParams(dingoParams, DingoToGo)

// Forward to gopls
goResult, err := gopls.Completion(ctx, goParams)

// Translate response
dingoResult, err := translator.TranslateCompletionList(goResult, GoToDingo)
```

**Supported Translations:**
- Completion requests/responses
- Definition requests/responses
- Hover requests/responses
- Diagnostics (gopls errors → Dingo positions)

### 4. Source Map Cache (`sourcemap_cache.go`)

**Responsibilities:**
- Load `.go.map` files on demand
- In-memory caching (avoid repeated disk I/O)
- Version validation (Phase 4 compatibility)
- Cache invalidation on file changes
- Graceful degradation (missing/invalid maps)

**Features:**
- **Version checking:** Supports source map version 1, fails gracefully on unsupported versions
- **Concurrency-safe:** Uses RWMutex for thread-safe access
- **Cache management:** `Get()`, `Invalidate()`, `InvalidateAll()`

**Example:**
```go
cache, _ := lsp.NewSourceMapCache(logger)

// Load source map (cached)
sm, err := cache.Get("/path/to/file.go")

// After transpilation, invalidate cache
cache.Invalidate("/path/to/file.go")
```

### 5. File Watcher (`watcher.go`)

**Responsibilities:**
- Watch workspace for `.dingo` file changes
- Trigger auto-transpilation on save
- Debounce rapid changes (500ms)
- Respect ignore patterns (`.gitignore`, `node_modules`, etc.)

**Features:**
- **Hybrid strategy:** Watch workspace root, filter for `.dingo` files only
- **Debouncing:** 500ms to batch rapid changes
- **Ignore patterns:** Skips `node_modules`, `vendor`, `.git`, `.dingo_cache`, `dist`, `build`
- **Recursive watching:** Catches multi-file refactoring

**Example:**
```go
watcher, err := lsp.NewFileWatcher(workspaceRoot, logger, func(dingoPath string) {
    // Auto-transpile triggered
    transpileDingoFile(dingoPath)
})
defer watcher.Close()
```

### 6. Transpiler Integration (`transpiler.go`)

**Responsibilities:**
- Execute `dingo build` for transpilation
- Parse transpilation errors into LSP diagnostics
- Publish errors to IDE

**Example:**
```go
transpiler := lsp.NewTranspiler(logger)

// Transpile file
if err := transpiler.Transpile(dingoPath); err != nil {
    // Parse error into diagnostic
    diagnostic := lsp.ParseTranspileError(err)
    // Publish to IDE
}
```

### 7. Logger (`logger.go`)

**Responsibilities:**
- Configurable log levels (debug, info, warn, error)
- Structured logging for debugging
- Environment variable configuration

**Example:**
```go
logger := lsp.NewLogger("debug")  // Or: "info", "warn", "error"

logger.Debugf("Position translated: %v → %v", dingoPos, goPos)
logger.Infof("LSP server started")
logger.Errorf("gopls crashed: %v", err)
```

## Position Translation Mechanics

### Source Map Format (Version 1)

```json
{
  "version": 1,
  "dingo_file": "/path/example.dingo",
  "go_file": "/path/example.go",
  "mappings": [
    {
      "generated_line": 18,
      "generated_column": 22,
      "original_line": 10,
      "original_column": 15,
      "length": 3,
      "name": "error_prop"
    }
  ]
}
```

### Translation Example

**Dingo Source (line 10, column 15):**
```dingo
result := getData()?  // ? operator at column 15
```

**Generated Go (lines 15-21):**
```go
_tmp0, _err0 := getData()
if _err0 != nil {
    return nil, _err0  // Line 18, column 22
}
result := _tmp0
```

**Translation Flow:**
1. User requests autocomplete at Dingo position `{line: 10, col: 15}`
2. Translator looks up source map, finds mapping to Go position `{line: 18, col: 22}`
3. LSP forwards request to gopls with Go position
4. gopls returns completions with Go positions
5. Translator maps Go positions back to Dingo positions
6. IDE displays completions at correct Dingo position

### Edge Cases Handled

**Unmapped Position:**
- Input position has no source map entry
- **Behavior:** Pass through unchanged (1:1 mapping)
- **Use case:** Comments, whitespace

**Multi-line Expansion:**
- Example: `x?` → 7 lines of Go code
- All 7 Go lines map back to same Dingo line/col
- gopls errors on any line → translated to original `?` position

**Missing Source Map:**
- `.dingo` file never transpiled
- **Behavior:** Return empty responses, show diagnostic: "File not transpiled"
- **Recovery:** Save file (auto-transpile) → Works normally

**Unsupported Source Map Version:**
- Source map version > 1 (future Phase 4 changes)
- **Behavior:** Fail gracefully with clear error message
- **Message:** "Unsupported source map version 2 (max: 1). Update dingo-lsp."

## Performance Characteristics

### Latency Budget

**Target:** <100ms for autocomplete (VSCode → VSCode)

**Breakdown:**
- VSCode → dingo-lsp IPC: ~5ms (local stdio)
- Position translation (Dingo → Go): **<1ms** ✅ (hash map lookup)
- dingo-lsp → gopls IPC: ~5ms (local stdio)
- gopls type checking: ~50ms (Go compiler)
- gopls → dingo-lsp IPC: ~5ms
- Position translation (Go → Dingo): **<1ms** ✅
- dingo-lsp → VSCode IPC: ~5ms
- **Total:** ~72ms ✅ Within budget

### Optimization Strategies

**1. Source Map Lookup**
- **Current:** Linear scan through mappings (O(n))
- **Optimization (if >10ms):** Binary search by line number (O(log n))
- **Future:** Pre-compute line→mapping index (O(1))

**2. Source Map Cache**
- **Strategy:** In-memory cache (no eviction in iteration 1)
- **Invalidation:** On file change (fsnotify event)
- **Hit Rate:** >95% (most edits in same files)

**3. gopls Connection**
- **Strategy:** Single long-lived subprocess (no restarts)
- **Benefit:** Avoids initialization overhead (~500ms per restart)
- **Recovery:** Auto-restart on crash (max 3 attempts)

**4. File Watcher Debouncing**
- **Duration:** 500ms
- **Benefit:** Batch rapid saves (e.g., auto-save plugins)
- **Trade-off:** Slight delay, but prevents 10x transpilations

## Extending with New Features

### Adding a New LSP Method

1. **Add handler to `server.go`:**
```go
func (s *Server) handleNewMethod(ctx context.Context, req *jsonrpc2.Request) (*NewMethodResult, error) {
    var params NewMethodParams
    json.Unmarshal(req.Params, &params)

    // Check if Dingo file
    if !isDingoFile(params.TextDocument.URI) {
        return s.gopls.NewMethod(ctx, params)
    }

    // Translate Dingo → Go
    goParams, err := s.translator.TranslateNewMethodParams(params, DingoToGo)

    // Forward to gopls
    goResult, err := s.gopls.NewMethod(ctx, goParams)

    // Translate Go → Dingo
    dingoResult, err := s.translator.TranslateNewMethodResult(goResult, GoToDingo)

    return dingoResult, nil
}
```

2. **Add translation methods to `translator.go`:**
```go
func (t *Translator) TranslateNewMethodParams(params NewMethodParams, dir Direction) (NewMethodParams, error) {
    uri, pos, err := t.translatePosition(params.TextDocument.URI, params.Position, dir)
    return NewMethodParams{
        TextDocument: TextDocumentIdentifier{URI: uri},
        Position:     pos,
    }, nil
}

func (t *Translator) TranslateNewMethodResult(result *NewMethodResult, dir Direction) (*NewMethodResult, error) {
    // Translate any positions in result
    return result, nil
}
```

3. **Add gopls method to `gopls_client.go`:**
```go
func (c *GoplsClient) NewMethod(ctx context.Context, params NewMethodParams) (*NewMethodResult, error) {
    var result NewMethodResult
    err := c.conn.Call(ctx, "textDocument/newMethod", params, &result)
    return &result, err
}
```

4. **Add unit tests:**
```go
func TestTranslateNewMethodParams(t *testing.T) {
    // Test Dingo → Go translation
}

func TestTranslateNewMethodResult(t *testing.T) {
    // Test Go → Dingo translation
}
```

### Supported Dingo Features (Iteration 1)

**Phase 3 Features:**
✅ **Type Annotations:** `: Type` syntax
✅ **Error Propagation:** `?` operator
✅ **Result<T,E> Types:** Sum types with helpers
✅ **Option<T> Types:** Optional values with helpers
✅ **Sum Types (Enums):** Tagged unions

**Phase 4.1 Features:**
✅ **Pattern Matching:** `match` expressions with Rust-style syntax
✅ **Exhaustiveness Checking:** Compile-time verification of all patterns
✅ **None Context Inference:** Smart type inference for None values
✅ **Nested Patterns:** `Ok(Some(value))` destructuring

**Deferred to Iteration 2 (after Phase 4.2):**
⏳ Pattern guards (`if` conditions in patterns)
⏳ Swift syntax (`switch/case` with `.Variant`)
⏳ Tuple destructuring in patterns
⏳ Enhanced error messages (rustc-style)

**Deferred to Iteration 3 (after Phase 4.3+):**
⏳ Lambda functions
⏳ Ternary operator
⏳ Null coalescing

## Environment Variables

**DINGO_LSP_LOG**
- **Values:** `debug`, `info`, `warn`, `error`
- **Default:** `info`
- **Purpose:** Control log verbosity

**DINGO_AUTO_TRANSPILE**
- **Values:** `true`, `false`
- **Default:** `true` (via VSCode extension setting)
- **Purpose:** Enable/disable auto-transpile on save

## Dependencies

**Required Go Packages:**
```go
require (
    go.lsp.dev/protocol v0.12.0
    go.lsp.dev/jsonrpc2 v0.10.0
    github.com/fsnotify/fsnotify v1.6.0
)
```

**Required External Tools:**
- **gopls:** Go language server (`go install golang.org/x/tools/gopls@latest`)
- **dingo:** Transpiler binary (from Dingo installation)

## Testing

**Run all tests:**
```bash
go test ./pkg/lsp/... -v
```

**Run with coverage:**
```bash
go test ./pkg/lsp/... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

**Run benchmarks:**
```bash
go test ./pkg/lsp/... -bench=. -benchmem
```

**Current Coverage:** >80% (unit + integration)

## Common Issues & Debugging

See [`docs/lsp-debugging.md`](../../docs/lsp-debugging.md) for detailed troubleshooting.

**Quick Checks:**
1. **Autocomplete not working?** Ensure `.dingo` file is transpiled (`dingo build file.dingo`)
2. **gopls errors?** Check gopls is installed: `gopls version`
3. **Position off by a few lines?** Source map may be stale, save file to re-transpile
4. **LSP crashes?** Check logs: `DINGO_LSP_LOG=debug dingo-lsp`

## Future Enhancements

**Iteration 2 (Post-Phase IV):**
- [ ] Document symbols (Ctrl+Shift+O)
- [ ] Find references (Shift+F12)
- [ ] Rename refactoring (F2)
- [ ] Code actions (quick fixes)
- [ ] Formatting (`dingo fmt`)
- [ ] Support for Phase IV features (lambdas, ternary, etc.)

**Performance:**
- [ ] Binary search for source map lookups (if needed)
- [ ] Pre-computed line→mapping index
- [ ] LRU cache eviction (if memory usage becomes issue)

**Distribution:**
- [ ] Neovim plugin (LSP-compatible)
- [ ] Other editors (Sublime, Emacs, etc.)

## License

Part of the Dingo project - see root LICENSE file.

## References

- [LSP Specification](https://microsoft.github.io/language-server-protocol/)
- [gopls Documentation](https://github.com/golang/tools/tree/master/gopls)
- [VSCode Extension Guide](../../editors/vscode/README.md)
- [Debugging Guide](../../docs/lsp-debugging.md)
