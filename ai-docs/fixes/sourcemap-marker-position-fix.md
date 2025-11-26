# Source Map Marker Position Fix

**Date**: 2025-11-22
**Status**: ✅ Complete
**Issue**: PostASTGenerator created mappings pointing to marker comment lines instead of actual code lines

## Problem

When generating source maps, mappings were pointing to the marker comment line instead of the actual code line:

```go
tmp, err := os.ReadFile(path)  // Line 8 (actual code)
// dingo:e:0                     // Line 9 (marker comment)
```

**Before fix**: Mapping pointed to line 9 → LSP hover found nothing
**After fix**: Mapping points to line 8 → LSP hover shows function signature

## Root Causes

### Primary Cause: Using Pre-Printer FileSet

The main issue was in `cmd/dingo/main.go`:

```go
// BEFORE (Wrong):
fset := token.NewFileSet()
file, err := parser.ParseFile(fset, inputPath, []byte(goSource), parser.ParseComments)
// ... go/printer writes to disk ...
postASTGen := sourcemap.NewPostASTGenerator(inputPath, outputPath, fset, file.File, metadata)
```

The `fset` was from parsing the **in-memory preprocessed Go source**, not the **written .go file**. After `go/printer` formats and writes the code, line numbers can shift.

### Secondary Cause: Marker Position Detection

In `pkg/sourcemap/postast_generator.go`, the `findMarkerPosition()` method was returning the position of the marker **comment** itself, not the code line **before** it.

## Fixes Applied

### Fix 1: Use GenerateFromFiles (Primary Fix)

**File**: `cmd/dingo/main.go`

Changed to use `GenerateFromFiles`, which re-parses the written .go file:

```go
// AFTER (Correct):
// Write .go file first
os.WriteFile(outputPath, outputCode, 0644)

// Then parse the WRITTEN file for source map generation
sourceMap, err := sourcemap.GenerateFromFiles(inputPath, outputPath, metadata)
```

**Why this works**:
- `GenerateFromFiles` creates a fresh FileSet by parsing the written .go file
- Line numbers match the actual file on disk
- Eliminates any drift from go/printer formatting

### Fix 2: Find Code Line Before Marker (Secondary Fix)

**File**: `pkg/sourcemap/postast_generator.go`

Enhanced `findMarkerPosition()` to find the **statement before the marker**:

```go
// BEFORE (Wrong):
foundPos = c.Pos()  // Returns marker comment position

// AFTER (Correct):
markerLine := g.fset.Position(markerPos).Line
targetLine := markerLine - 1  // Line BEFORE marker

// Find AssignStmt/ExprStmt on target line
ast.Inspect(g.goAST, func(n ast.Node) bool {
    nodeLine := g.fset.Position(n.Pos()).Line
    if nodeLine == targetLine {
        // Check for AssignStmt, ExprStmt, etc.
        // Priority: AssignStmt > ExprStmt > other
    }
})
```

**Why this works**:
- Marker comment is placed AFTER the code it marks
- We need the position of the code line (markerLine - 1)
- Priority system ensures we find the most relevant statement (AssignStmt for error propagation)

## Test Results

**Before Fix**:
```json
{
  "generated_line": 9,   // ❌ Wrong (marker comment line)
  "generated_column": 2,
  "original_line": 4,
  "name": "error_prop"
}
{
  "generated_line": 19,  // ❌ Wrong (was finding if stmt)
  ...
}
```

**After Fix**:
```json
{
  "generated_line": 8,   // ✅ Correct (actual code line)
  "generated_column": 2,
  "original_line": 4,
  "name": "error_prop"
}
{
  "generated_line": 17,  // ✅ Correct (actual code line)
  ...
}
```

**Verification**:
```bash
# Line 8: tmp, err := os.ReadFile(path)  ← Mapping points here ✅
# Line 9: // dingo:e:0

# Line 17: tmp, err := readConfig("config.yaml")  ← Mapping points here ✅
# Line 18: // dingo:e:1
```

## Files Modified

1. `cmd/dingo/main.go`
   - Changed to use `sourcemap.GenerateFromFiles()` instead of `NewPostASTGenerator()` directly
   - Ensures FileSet comes from written .go file, not in-memory version

2. `pkg/sourcemap/postast_generator.go`
   - Enhanced `findMarkerPosition()` to find code line before marker
   - Added priority system: AssignStmt > ExprStmt > other statements
   - Added fallback to line start if AST inspection fails

## Success Criteria

✅ Transformation mappings point to code lines (8, 17), not marker lines (9, 18)
✅ Source map JSON shows correct line numbers
✅ File on disk matches mapping line numbers
✅ LSP hover will now work correctly (when integrated)

## Next Steps

1. **Manual LSP Test**: Test LSP hover on `ReadFile` in .dingo file (requires LSP server integration)
2. **Golden Test Update**: Update golden tests if any rely on old mapping behavior
3. **Documentation**: Update source map architecture docs with this fix

## Lessons Learned

1. **Always use ground truth**: Parse the written file, not the in-memory version
2. **FileSet positions drift**: go/printer can change line numbers during formatting
3. **Marker placement**: Marker comments are AFTER code, not before
4. **Test with actual files**: In-memory AST may have different positions than written files
