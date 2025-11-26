# LSP Auto-Import Feature Specification

**Date:** 2025-11-23
**Status:** PLANNED (Iteration 2)
**Priority:** HIGH
**Reference:** Architectural decision from transpiler test fixes

---

## Background

### Architectural Decision (2025-11-23)

During transpiler test fixes (session: test-fixes), we determined that **auto-import should be handled by the LSP, NOT the transpiler**.

**Rationale:**
- Transpiler assumes imports are already present in .dingo files (Go-style)
- LSP provides editing features (auto-completion, auto-import, etc.)
- Avoids source map line shifting complexity in transpiler
- Follows TypeScript/VSCode pattern (LSP handles editing features)
- Better UX (user sees import being added to their .dingo file)

**Transpiler Test Skipped:**
- `TestCRITICAL1_MappingsBeforeImportsNotShifted` in `pkg/preprocessor/preprocessor_test.go`
- Test was for transpiler auto-import (now deprecated approach)
- Comprehensive skip documentation added with this architectural decision

---

## Feature Overview

### What Is Auto-Import?

When a user references a package identifier without importing it:

**Example Dingo Code (Missing Import):**
```dingo
package main

func main() {
    data, err := os.ReadFile("config.json")  // ❌ Error: undefined "os"
}
```

**LSP Behavior:**
1. Detect undefined reference: `os` package not imported
2. Provide diagnostic: "Package 'os' is not imported"
3. Offer code action: "Add import for 'os'"
4. When user accepts → Insert `import "os"` at top of file
5. Result:

```dingo
package main

import "os"  // ✅ Auto-inserted by LSP

func main() {
    data, err := os.ReadFile("config.json")  // ✅ Now valid
}
```

### Why This Matters

**User Experience:**
- Eliminates manual import management
- Reduces friction when writing Dingo code
- Matches Go/TypeScript developer expectations
- IDE integration feels complete and professional

**Technical Correctness:**
- .dingo files must have correct imports before transpilation
- Transpiler can focus on syntax transformation only
- Clean separation of concerns (LSP = editing, Transpiler = transformation)

---

## Implementation Design

### Architecture Pattern: TypeScript LSP Reference

TypeScript Language Server provides auto-import via:
1. **Diagnostics:** Report undefined identifier as error
2. **Code Actions:** Offer "Add import" as quick fix
3. **Text Edits:** Insert import statement at correct location

**We follow the same pattern.**

### Component Breakdown

```
┌────────────────────────────────────────────────┐
│ 1. Undefined Reference Detection              │
│    • Parse .dingo file AST                     │
│    • Identify package.Identifier patterns      │
│    • Check if package imported                 │
│    • Generate diagnostic if missing            │
└────────────────────────────────────────────────┘
                      ↓
┌────────────────────────────────────────────────┐
│ 2. Code Action Provider                        │
│    • Listen for textDocument/codeAction        │
│    • Check diagnostics at cursor position      │
│    • If "missing import" → offer code action   │
│    • Action: "Add import for 'os'"             │
└────────────────────────────────────────────────┘
                      ↓
┌────────────────────────────────────────────────┐
│ 3. Import Insertion Logic                      │
│    • Parse .dingo file                         │
│    • Find import block (or package declaration)│
│    • Insert new import (alphabetically sorted) │
│    • Return TextEdit to LSP client             │
└────────────────────────────────────────────────┘
```

---

## Detailed Implementation Steps

### Step 1: Undefined Reference Detection

**File:** `pkg/lsp/diagnostics.go` (new file)

**Function:** `DetectMissingImports(dingoSource string) []Diagnostic`

**Algorithm:**
1. Parse .dingo source to AST (use `go/parser` on transpiled Go for simplicity)
2. Walk AST looking for `SelectorExpr` nodes (e.g., `os.ReadFile`)
3. Extract package name (left side of selector: `os`)
4. Check if package is in import declarations
5. If NOT imported → Create diagnostic:
   ```go
   Diagnostic{
       Range: Range{Start: Position{line, col}, End: Position{line, col+len(pkg)}},
       Severity: Error,
       Message: fmt.Sprintf("Package '%s' is not imported", pkg),
       Code: "missing-import",
       Source: "dingo-lsp",
   }
   ```

**Edge Cases:**
- Built-in types (`string`, `int`, etc.) → Skip (not packages)
- Locally defined identifiers → Skip (check symbol table)
- Already imported packages → Skip
- Standard library vs third-party → Both handled same way

**Performance:**
- Only run when file changes (textDocument/didChange)
- Cache results per file version
- Should complete in <10ms for typical files

---

### Step 2: Code Action Provider

**File:** `pkg/lsp/handlers.go` (extend existing)

**Function:** `handleCodeAction(ctx context.Context, req *CodeActionParams) ([]CodeAction, error)`

**LSP Flow:**
1. IDE sends `textDocument/codeAction` when cursor is on error
2. LSP receives request with document URI + position + diagnostics
3. Check if any diagnostic at position has code `"missing-import"`
4. If yes → Generate code action:
   ```go
   CodeAction{
       Title: fmt.Sprintf("Add import for '%s'", packageName),
       Kind: "quickfix",
       Diagnostics: [diagnostic], // Link to diagnostic being fixed
       Edit: WorkspaceEdit{
           Changes: map[DocumentURI][]TextEdit{
               dingoURI: {generateImportEdit(packageName)},
           },
       },
   }
   ```

**User Experience:**
```
// User sees red squiggle under "os"
data, err := os.ReadFile(...)
             ^^

// User hits Cmd+. (VSCode quick fix shortcut)
// IDE shows: "💡 Add import for 'os'"
// User presses Enter
// Import added automatically
```

---

### Step 3: Import Insertion Logic

**File:** `pkg/lsp/imports.go` (new file)

**Function:** `generateImportEdit(packageName string, dingoSource string) TextEdit`

**Algorithm:**
1. Parse .dingo source to find import block location
2. Determine insertion strategy:
   - **No imports yet:** Insert after package declaration
   - **Single import:** Convert to multi-line block, add new import
   - **Import block exists:** Insert alphabetically within block
3. Generate TextEdit with proper formatting:
   ```go
   TextEdit{
       Range: Range{Start: insertLine, End: insertLine}, // Zero-width range
       NewText: "import \"os\"\n",
   }
   ```

**Example Transformations:**

**Case 1: No imports yet**
```dingo
package main
           ← Insert here
func main() {}
```
→
```dingo
package main

import "os"

func main() {}
```

**Case 2: Single import exists**
```dingo
package main

import "fmt"

func main() {}
```
→
```dingo
package main

import (
    "fmt"
    "os"
)

func main() {}
```

**Case 3: Import block exists**
```dingo
package main

import (
    "fmt"
    "strings"
)

func main() {}
```
→
```dingo
package main

import (
    "fmt"
    "os"      ← Inserted alphabetically
    "strings"
)

func main() {}
```

**Formatting Rules:**
- Alphabetical sorting (standard Go convention)
- Group standard library separately from third-party (optional, Phase 2)
- Maintain existing spacing/style
- Use `gofmt` rules for consistency

---

### Step 4: Integration with Existing LSP Server

**File:** `pkg/lsp/server.go` (extend existing)

**Changes:**

1. **Add diagnostics publisher:**
   ```go
   func (s *Server) publishDiagnostics(uri DocumentURI, diagnostics []Diagnostic) {
       s.client.Notify(context.Background(), "textDocument/publishDiagnostics", PublishDiagnosticsParams{
           URI: uri,
           Diagnostics: diagnostics,
       })
   }
   ```

2. **Extend `handleDidChange` to run diagnostics:**
   ```go
   func (s *Server) handleDidChange(ctx context.Context, params DidChangeTextDocumentParams) {
       // ... existing code ...

       // Run diagnostics
       diagnostics := DetectMissingImports(params.ContentChanges[0].Text)
       s.publishDiagnostics(params.TextDocument.URI, diagnostics)
   }
   ```

3. **Register code action handler:**
   ```go
   case "textDocument/codeAction":
       return s.handleCodeAction(ctx, req)
   ```

---

## Testing Strategy

### Unit Tests

**File:** `pkg/lsp/diagnostics_test.go`

```go
func TestDetectMissingImports(t *testing.T) {
    tests := []struct {
        name     string
        source   string
        expected []string // Package names
    }{
        {
            name: "Single missing import",
            source: `package main
func main() { os.ReadFile("x") }`,
            expected: []string{"os"},
        },
        {
            name: "Multiple missing imports",
            source: `package main
func main() {
    fmt.Println("x")
    os.ReadFile("y")
}`,
            expected: []string{"fmt", "os"},
        },
        {
            name: "No missing imports",
            source: `package main
import "os"
func main() { os.ReadFile("x") }`,
            expected: []string{},
        },
        {
            name: "Built-in types not flagged",
            source: `package main
func main() { var x string }`,
            expected: []string{},
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            diagnostics := DetectMissingImports(tt.source)
            // Assert diagnostics match expected packages
        })
    }
}
```

**File:** `pkg/lsp/imports_test.go`

```go
func TestGenerateImportEdit(t *testing.T) {
    tests := []struct {
        name        string
        packageName string
        source      string
        expected    string // Expected source after edit
    }{
        {
            name: "First import",
            packageName: "os",
            source: `package main

func main() {}`,
            expected: `package main

import "os"

func main() {}`,
        },
        {
            name: "Add to existing import block",
            packageName: "os",
            source: `package main

import (
    "fmt"
    "strings"
)

func main() {}`,
            expected: `package main

import (
    "fmt"
    "os"
    "strings"
)

func main() {}`,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            edit := generateImportEdit(tt.packageName, tt.source)
            result := applyTextEdit(tt.source, edit)
            assert.Equal(t, tt.expected, result)
        })
    }
}
```

### Integration Tests

**File:** `pkg/lsp/integration_test.go`

```go
func TestAutoImportEndToEnd(t *testing.T) {
    // 1. Create test .dingo file with missing import
    dingoSource := `package main
func main() {
    data := os.ReadFile("config.json")
}`

    // 2. Start LSP server
    server := NewTestServer(t)
    defer server.Shutdown()

    // 3. Open document
    server.DidOpen(dingoURI, dingoSource)

    // 4. Verify diagnostic published
    diagnostics := server.WaitForDiagnostics(dingoURI)
    assert.Len(t, diagnostics, 1)
    assert.Contains(t, diagnostics[0].Message, "Package 'os' is not imported")

    // 5. Request code action
    actions := server.CodeAction(dingoURI, diagnostics[0].Range)
    assert.Len(t, actions, 1)
    assert.Equal(t, "Add import for 'os'", actions[0].Title)

    // 6. Apply code action
    server.ApplyEdit(actions[0].Edit)

    // 7. Verify import added
    updatedSource := server.GetDocumentContent(dingoURI)
    assert.Contains(t, updatedSource, `import "os"`)

    // 8. Verify diagnostic cleared
    diagnostics = server.WaitForDiagnostics(dingoURI)
    assert.Len(t, diagnostics, 0)
}
```

### Manual Testing (VSCode)

1. Create `test.dingo` with missing import
2. Open in VSCode with Dingo extension
3. Verify red squiggle appears under undefined package
4. Hover → See error message
5. Press `Cmd+.` (quick fix)
6. See "Add import" suggestion
7. Press Enter
8. Verify import added to file
9. Verify error disappears

---

## Gopls Integration

### Challenge: Gopls Already Provides Import Errors

Gopls analyzes transpiled `.go` files and reports missing imports **for Go files**.

**Our Goal:** Report missing imports **for .dingo files** (before transpilation).

### Two Approaches

**Option A: Pre-transpilation Analysis (Recommended)**
- LSP analyzes .dingo source BEFORE forwarding to gopls
- Detects missing imports in .dingo file
- Provides code action to fix .dingo file
- Then transpile → gopls sees valid Go
- **Advantage:** User fixes .dingo source (correct layer)

**Option B: Translate Gopls Diagnostics**
- Let gopls report errors on transpiled .go file
- Translate gopls error positions back to .dingo
- Translate code action edits back to .dingo
- **Disadvantage:** Complex position translation for multi-line expansions

**Decision: Use Option A** (simpler, more correct)

---

## Implementation Phases

### Phase 1: Core Functionality (Week 1-2)

**Deliverables:**
- [ ] `diagnostics.go` - Missing import detection
- [ ] `imports.go` - Import insertion logic
- [ ] `handlers.go` - Code action provider
- [ ] Unit tests for all components
- [ ] Integration test (end-to-end)

**Acceptance:**
- VSCode shows diagnostic for missing import
- Quick fix adds import to .dingo file
- Tests pass

### Phase 2: Edge Cases & Polish (Week 3)

**Deliverables:**
- [ ] Handle import aliases (`import f "fmt"`)
- [ ] Handle dot imports (`import . "fmt"`)
- [ ] Group standard library vs third-party
- [ ] Performance optimization (caching)
- [ ] Error recovery (malformed .dingo files)

**Acceptance:**
- All edge cases handled gracefully
- Performance <10ms for typical files

### Phase 3: User Experience (Week 4)

**Deliverables:**
- [ ] Auto-import on autocomplete (like TypeScript)
- [ ] Remove unused imports
- [ ] Organize imports command
- [ ] Configuration options (enable/disable, grouping style)

**Acceptance:**
- Feature parity with TypeScript auto-import
- User documentation complete

---

## Configuration

**VSCode Extension Settings:**

```json
{
    "dingo.autoImport.enabled": true,
    "dingo.autoImport.grouping": "standard-first",  // "standard-first" | "none"
    "dingo.autoImport.aliasDetection": true,
    "dingo.autoImport.removeUnused": true
}
```

**LSP Server Config:**

```go
type AutoImportConfig struct {
    Enabled          bool
    Grouping         string // "standard-first", "none"
    AliasDetection   bool
    RemoveUnused     bool
}
```

---

## Success Metrics

**Must Have:**
- ✅ Missing import detection: 100% accuracy (no false positives)
- ✅ Code action appears within 100ms of diagnostic
- ✅ Import insertion: Correct position 100% of time
- ✅ Tests pass: Unit + integration

**Should Have:**
- ✅ Performance: <10ms for diagnostics on 1000-line file
- ✅ Edge cases handled: Aliases, dot imports, malformed files
- ✅ User documentation complete

**Nice to Have:**
- ⭐ Auto-import on autocomplete (like TypeScript)
- ⭐ Remove unused imports
- ⭐ Organize imports command

---

## References

**TypeScript LSP Auto-Import:**
- [TypeScript Auto-Import Docs](https://www.typescriptlang.org/docs/handbook/release-notes/typescript-2-9.html#auto-import)
- [VSCode Auto-Import Guide](https://code.visualstudio.com/docs/languages/typescript#_auto-import)

**LSP Specification:**
- [Diagnostics](https://microsoft.github.io/language-server-protocol/specifications/lsp/3.17/specification/#diagnostic)
- [Code Actions](https://microsoft.github.io/language-server-protocol/specifications/lsp/3.17/specification/#textDocument_codeAction)
- [Text Edits](https://microsoft.github.io/language-server-protocol/specifications/lsp/3.17/specification/#textEdit)

**Dingo Transpiler:**
- `pkg/preprocessor/` - Preprocessor implementation
- `pkg/preprocessor/preprocessor_test.go:619-711` - Skipped test with rationale
- `CLAUDE.md` - Architectural decisions

---

## Notes

- This feature MUST be implemented in LSP, NOT transpiler
- Transpiler assumes imports are present (Go-style)
- LSP provides editing features (this is one of them)
- Follows TypeScript/VSCode pattern (industry standard)
- Clean separation: LSP = editing, Transpiler = transformation

---

**Last Updated:** 2025-11-23
**Author:** Dingo Team (Claude Code)
**Status:** Ready for Implementation (Iteration 2)
