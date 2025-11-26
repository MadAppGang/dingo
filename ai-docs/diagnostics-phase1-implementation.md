# Phase 1 Dingo Diagnostics Implementation

## Overview
Implemented didSave error reporting for Dingo-specific syntax errors in the LSP server.

## Changes Made

### 1. AutoTranspiler (pkg/lsp/transpiler.go)
- Added `server *Server` field to store server reference
- Updated `NewAutoTranspiler()` to accept server parameter
- Modified `OnFileChange()` to:
  - Publish diagnostic on transpilation error
  - Clear diagnostics on successful transpilation
  - Use existing `ParseTranspileError()` for error parsing

### 2. Server (pkg/lsp/server.go)
- Added `publishDingoDiagnostics()` method:
  - Publishes Dingo-specific diagnostics to IDE
  - Thread-safe (uses `GetConn()`)
  - Separate from gopls diagnostics (which are translated)
  - Logs diagnostic activity for debugging
- Updated `NewServer()` initialization:
  - Create server first without transpiler
  - Pass server to transpiler constructor
  - Avoids circular dependency

### 3. Tests (pkg/lsp/transpiler_test.go)
- Updated test to pass `nil` for server parameter
- Test still validates transpilation logic

## Architecture

```
.dingo file saved
    ↓
AutoTranspiler.OnFileChange()
    ↓
TranspileFile() fails
    ↓
ParseTranspileError() → Diagnostic
    ↓
Server.publishDingoDiagnostics()
    ↓
IDE shows error at line:col
```

## Error Flow

**On transpilation error:**
1. `OnFileChange()` catches error from `TranspileFile()`
2. `ParseTranspileError()` parses error message (format: `file.dingo:10:5: message`)
3. `publishDingoDiagnostics()` sends diagnostic to IDE via LSP protocol
4. IDE displays red squiggle at error location

**On successful transpilation:**
1. `OnFileChange()` succeeds
2. `publishDingoDiagnostics()` called with empty array
3. IDE clears any previous Dingo diagnostics
4. gopls diagnostics continue working (separate flow)

## Testing

**Build Status:**
- ✅ `go build ./pkg/lsp/...` - compiles successfully
- ✅ `go test ./pkg/lsp/...` - all tests pass (3.387s)
- ✅ `go build ./cmd/dingo-lsp/...` - LSP binary builds

**Test Coverage:**
- Existing transpiler test updated to pass new parameter
- All existing functionality preserved
- No regression in position translation or diagnostic forwarding

## Implementation Notes

**Circular Dependency Resolution:**
- Server needs AutoTranspiler
- AutoTranspiler needs Server (for diagnostics)
- Solution: Create server first, then transpiler, then assign

**URI Creation:**
- Used `protocol.DocumentURI(lspuri.File(path))` pattern
- Matches existing codebase conventions
- Consistent with other URI usage in server.go

**Thread Safety:**
- Uses existing `GetConn()` method (thread-safe)
- Follows same pattern as `handlePublishDiagnostics()`
- No new concurrency issues introduced

## Next Steps (Future Phases)

**Phase 2 - didChange live validation:**
- Validate on every keystroke (not just save)
- Debounce validation to avoid excessive processing
- Cache validation results

**Phase 3 - Enhanced error messages:**
- Add suggested fixes (quickfix actions)
- Show error context (surrounding code)
- Link to documentation for Dingo syntax

**Phase 4 - Integration with gopls:**
- Merge Dingo and gopls diagnostics
- Prioritize Dingo errors over Go errors
- Clear Go errors when Dingo syntax invalid

## Files Modified

```
pkg/lsp/transpiler.go      - Added server field, diagnostic publishing
pkg/lsp/server.go          - Added publishDingoDiagnostics method
pkg/lsp/transpiler_test.go - Updated test to pass nil server
```

## Verification Steps

To test this implementation:

1. Start dingo-lsp server in an editor
2. Open a .dingo file with invalid syntax (e.g., `match x { }` missing cases)
3. Save the file
4. Verify error appears in editor at correct line/column
5. Fix the syntax error
6. Save the file
7. Verify error disappears

## Error Format Examples

**Input (transpiler error):**
```
test.dingo:10:5: syntax error: unexpected token
```

**Output (LSP diagnostic):**
```json
{
  "uri": "file:///path/to/test.dingo",
  "diagnostics": [{
    "range": {
      "start": {"line": 9, "character": 4},
      "end": {"line": 9, "character": 4}
    },
    "severity": 1,
    "source": "dingo",
    "message": "syntax error: unexpected token"
  }]
}
```

**Note:** Line/column are 0-based in LSP but 1-based in error messages.
