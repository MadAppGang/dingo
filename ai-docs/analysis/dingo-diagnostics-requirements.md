# Dingo-Specific Diagnostics Requirements

**Date**: 2025-11-23
**Status**: Analysis
**Priority**: P1 (High - Critical for IDE experience)

## Problem Statement

**Current Situation:**
- LSP forwards `.dingo` files to gopls after transpilation
- gopls only sees `.go` files (valid Go syntax)
- **Dingo-specific syntax errors** have no validation path!

**Example Dingo Syntax (Not Valid Go):**
```dingo
// BEFORE transpilation (in .dingo file):
func readConfig(path: string) ([]byte, error) {  // ← Type annotation syntax
    let data = os.ReadFile(path)?                 // ← Error propagation
    return data, nil
}

// Syntax errors gopls CAN'T catch:
let x = os.ReadFile(path)??   // ← Missing operand
let y = user?.address?.       // ← Incomplete safe nav
match result {                 // ← Missing cases
    Ok(x) =>
}
enum Color {                   // ← Invalid enum syntax
    Red(,)
}
```

**Gap**: No validation for Dingo-specific syntax errors!

## Two Types of Diagnostics

### 1. Dingo Diagnostics (Our Responsibility)
**Source**: Dingo preprocessor/transpiler
**Shown in**: `.dingo` files
**Examples**:
- Syntax errors in `?` operator usage
- Invalid `match` patterns
- Malformed `enum` declarations
- Incomplete safe navigation `?.`
- Type annotation syntax errors

**Characteristics**:
- Detected BEFORE transpilation
- Don't need gopls at all
- Fast feedback (on didChange)
- Prevent invalid transpilation attempts

### 2. Go Diagnostics (gopls Responsibility)
**Source**: gopls analyzing `.go` files
**Shown in**: `.dingo` files (translated via source maps)
**Examples**:
- Type mismatches (after transpilation)
- Undefined variables
- Import errors
- Go compiler errors

**Characteristics**:
- Detected AFTER transpilation
- Require source map translation
- Already partially working (auto-rebuild exists)
- Only available after successful transpilation

## Architecture: Dual Diagnostic Flow

```
┌─────────────────────────────────────────────────────────────┐
│ User edits .dingo file                                      │
└────────────────────┬────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────────┐
│ LSP: textDocument/didChange (Real-time validation)         │
├─────────────────────────────────────────────────────────────┤
│ 1. Lightweight Dingo syntax check                           │
│    - Regex-based validation (fast, <10ms)                   │
│    - Check common patterns:                                 │
│      • Incomplete ? operator                                │
│      • Invalid safe nav ?.                                  │
│      • Malformed match/enum                                 │
│                                                              │
│ 2. Publish Dingo diagnostics (if errors found)              │
│    - Source: "dingo-syntax"                                 │
│    - Severity: Error                                        │
│    - Message: "Invalid ? operator syntax"                   │
│                                                              │
│ → NO gopls involvement                                      │
└─────────────────────────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────────┐
│ User saves .dingo file                                      │
└────────────────────┬────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────────┐
│ LSP: textDocument/didSave (Full transpilation)             │
├─────────────────────────────────────────────────────────────┤
│ 1. Run full transpiler                                      │
│    - Preprocessors: error_prop, type_annot, etc.            │
│    - AST processing                                         │
│    - go/printer generation                                  │
│                                                              │
│ 2. Catch transpiler errors                                  │
│    if err != nil {                                          │
│        diagnostic := protocol.Diagnostic{                   │
│            Range: extractRangeFromError(err),               │
│            Severity: Error,                                 │
│            Source: "dingo-transpiler",                      │
│            Message: err.Error(),                            │
│        }                                                     │
│        publishDiagnostics(uri, []diagnostic)                │
│        return // Don't proceed to gopls                     │
│    }                                                         │
│                                                              │
│ 3. Clear Dingo diagnostics (if success)                     │
│    publishDiagnostics(uri, []) // Empty = no errors         │
│                                                              │
│ → NO gopls involvement for Dingo errors                     │
└─────────────────────────────────────────────────────────────┘
                     │
                     ▼ (Only if transpilation succeeded)
┌─────────────────────────────────────────────────────────────┐
│ Auto-rebuild: Sync .go file to gopls                        │
├─────────────────────────────────────────────────────────────┤
│ 1. Write .go and .go.map files                              │
│ 2. Send didChange to gopls with .go content                 │
│ 3. gopls analyzes .go file                                  │
│                                                              │
│ → gopls now involved for Go diagnostics                     │
└─────────────────────────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────────┐
│ gopls: textDocument/publishDiagnostics                      │
├─────────────────────────────────────────────────────────────┤
│ gopls sends diagnostics for .go file                        │
│                                                              │
│ LSP intercepts and translates:                              │
│ 1. Translate .go positions → .dingo positions               │
│ 2. Merge with Dingo diagnostics                             │
│ 3. Publish combined diagnostics to editor                   │
│                                                              │
│ → Both Dingo and Go diagnostics shown                       │
└─────────────────────────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────────┐
│ Editor shows red squiggles in .dingo file                   │
│ - Dingo syntax errors (from Dingo validators)               │
│ - Go type errors (from gopls, translated)                   │
└─────────────────────────────────────────────────────────────┘
```

## Implementation Plan

### Phase 1: didSave Dingo Diagnostics (Existing Infrastructure)

**Goal**: Show transpiler errors when user saves

**Files to Modify**:
- `pkg/lsp/server.go` - didSave handler
- `pkg/lsp/transpiler.go` - OnFileChange error reporting

**Implementation**:
```go
// pkg/lsp/transpiler.go
func (at *AutoTranspiler) OnFileChange(ctx context.Context, dingoPath string) {
    // Transpile the file
    err := at.TranspileFile(ctx, dingoPath)
    if err != nil {
        at.logger.Errorf("Auto-transpile failed: %v", err)

        // NEW: Publish Dingo diagnostic
        diagnostic := at.createDiagnosticFromError(dingoPath, err)
        at.publishDiagnostic(dingoPath, []protocol.Diagnostic{diagnostic})
        return
    }

    // Clear diagnostics on success
    at.publishDiagnostic(dingoPath, []protocol.Diagnostic{})

    // Continue with gopls sync...
}

func (at *AutoTranspiler) createDiagnosticFromError(dingoPath string, err error) protocol.Diagnostic {
    // Parse error message for line/column info
    // Format: "file.dingo:10:5: error message"
    // Or fallback to line 0 if no position info

    // Use ParseTranspileError() function that already exists!
    return ParseTranspileError(dingoPath, err.Error())
}
```

**Already Exists**: `ParseTranspileError()` in `pkg/lsp/transpiler.go:86`!

**Effort**: 1-2 hours (wire up existing code)

### Phase 2: didChange Real-Time Validation (Future)

**Goal**: Show errors as user types (like TypeScript)

**Implementation**:
```go
// pkg/lsp/server.go
func (s *Server) handleDidChange(ctx context.Context, params *protocol.DidChangeTextDocumentParams) error {
    if !isDingoFile(params.TextDocument.URI) {
        return s.gopls.DidChange(ctx, params)
    }

    // Real-time Dingo syntax validation
    diagnostics := s.validateDingoSyntax(params.TextDocument.URI, params.ContentChanges)
    s.publishDiagnostics(params.TextDocument.URI, diagnostics)

    // Don't forward to gopls (no .go file yet)
    return nil
}

func (s *Server) validateDingoSyntax(uri protocol.URI, changes []protocol.TextDocumentContentChangeEvent) []protocol.Diagnostic {
    // Lightweight validation (regex-based)
    // Check for common syntax errors:
    // - Incomplete ? operator
    // - Invalid safe nav
    // - Malformed match/enum

    var diagnostics []protocol.Diagnostic

    // Example: Check for incomplete ? operator
    content := getLatestContent(changes)
    if strings.Contains(content, "?") {
        // Validate ? syntax
        // Add diagnostic if invalid
    }

    return diagnostics
}
```

**Effort**: 4-6 hours (new lightweight validator)

### Phase 3: didOpen Handler (Complete the LSP lifecycle)

**Goal**: Validate when file is opened

**Implementation**:
```go
// pkg/lsp/server.go
func (s *Server) handleDidOpen(ctx context.Context, params *protocol.DidOpenTextDocumentParams) error {
    if !isDingoFile(params.TextDocument.URI) {
        return s.gopls.DidOpen(ctx, params)
    }

    // Validate Dingo file on open
    diagnostics := s.validateDingoSyntax(params.TextDocument.URI, params.TextDocument.Text)
    s.publishDiagnostics(params.TextDocument.URI, diagnostics)

    // Also open corresponding .go file in gopls (for go diagnostics)
    goURI := dingoToGoURI(params.TextDocument.URI)
    if fileExists(goURI) {
        s.gopls.DidOpen(ctx, &protocol.DidOpenTextDocumentParams{
            TextDocument: protocol.TextDocumentItem{
                URI: goURI,
                // ... .go file content
            },
        })
    }

    return nil
}
```

**Effort**: 2-3 hours

## Current State vs Required State

### Current State (After Auto-Rebuild)
- ✅ Auto-rebuild on didSave
- ✅ gopls sync after rebuild
- ✅ Source map translation
- ❌ No Dingo syntax validation
- ❌ No transpiler error reporting
- ❌ No didChange/didOpen handlers for .dingo

### Required State (Full IDE Experience)
- ✅ Auto-rebuild on didSave (already done)
- ✅ gopls sync after rebuild (already done)
- ✅ Source map translation (already done)
- ✅ Dingo transpiler error reporting (Phase 1)
- ✅ didOpen handler for .dingo files (Phase 3)
- ✅ Real-time syntax validation (Phase 2 - future)

## Error Extraction from Transpiler

**Good News**: Preprocessors already return errors with context!

**Example** (error_prop.go):
```go
if !strings.HasSuffix(trimmed, "?") {
    return "", fmt.Errorf("line %d: invalid error propagation syntax", lineNum)
}
```

**We just need to**:
1. Catch these errors in `OnFileChange()`
2. Parse line number from error message
3. Create `protocol.Diagnostic`
4. Publish to editor

**Already have helper**: `ParseTranspileError()` does exactly this!

## Benefits of Dingo-Specific Diagnostics

**1. Fast Feedback**
- Errors shown immediately (didChange)
- No need to save and rebuild
- Better developer experience

**2. Accurate Error Messages**
- Dingo-specific error context
- Better than generic Go compiler errors
- Example: "Invalid ? operator" vs "syntax error: unexpected ?"

**3. Prevents Invalid Transpilation**
- Catch errors before transpilation
- Save CPU cycles
- Don't pollute gopls with invalid .go files

**4. Complete IDE Experience**
- TypeScript-like error reporting
- Real-time validation
- Professional IDE feel

## Testing Strategy

### Phase 1 Tests (didSave diagnostics)
```go
func TestDingoTranspilerDiagnostics(t *testing.T) {
    // Create .dingo file with syntax error
    code := `
package main
func test() {
    let x = readFile()?   // Missing second operand
}
`

    // Save file (trigger didSave)
    server.HandleDidSave(ctx, params)

    // Verify diagnostic published
    assert.NotEmpty(t, publishedDiagnostics)
    assert.Contains(t, publishedDiagnostics[0].Message, "Invalid ? operator")
}
```

### Phase 2 Tests (didChange real-time)
```go
func TestDingoRealTimeValidation(t *testing.T) {
    // Type incomplete syntax
    changes := []protocol.TextDocumentContentChangeEvent{
        {Text: "let x = user?."},
    }

    // Trigger didChange
    server.HandleDidChange(ctx, params)

    // Verify diagnostic published immediately
    assert.Contains(t, publishedDiagnostics[0].Message, "Incomplete safe navigation")
}
```

## Priority and Effort

**Phase 1: didSave Diagnostics**
- Priority: P1 (High - Should have for v1.0)
- Effort: 1-2 hours (wire up existing code)
- Risk: Low (reuses existing infrastructure)
- Impact: High (shows transpiler errors in IDE)

**Phase 2: Real-Time Validation**
- Priority: P2 (Nice to have for v1.0)
- Effort: 4-6 hours (new lightweight validator)
- Risk: Medium (need to design fast validator)
- Impact: High (professional IDE experience)

**Phase 3: didOpen Handler**
- Priority: P1 (Should have for v1.0)
- Effort: 2-3 hours
- Risk: Low (standard LSP pattern)
- Impact: Medium (completes LSP lifecycle)

## Recommendation

**Start with Phase 1** (1-2 hours):
1. Wire up transpiler error reporting in `OnFileChange()`
2. Use existing `ParseTranspileError()` helper
3. Publish diagnostics on didSave
4. Test with manual LSP testing

**Then Phase 3** (2-3 hours):
1. Implement didOpen handler
2. Validate on file open
3. Open corresponding .go file in gopls

**Phase 2 can wait** (future enhancement):
- Real-time validation is nice but not critical
- didSave validation already provides good UX
- Can be added incrementally

## Success Criteria

**Phase 1 Complete When**:
- ✅ Save .dingo file with syntax error → See red squiggle
- ✅ Save .dingo file with valid syntax → Squiggle clears
- ✅ Error message shows Dingo-specific context
- ✅ Error position is accurate (correct line/column)

**Full Diagnostics Complete When**:
- ✅ Open .dingo file → Validation runs
- ✅ Type in .dingo file → Real-time validation (Phase 2)
- ✅ Save .dingo file → Full transpiler validation
- ✅ Both Dingo and Go diagnostics shown simultaneously
- ✅ Source map translation works for Go diagnostics

---

**Next Steps**: Implement Phase 1 (didSave diagnostics) in 1-2 hours.
