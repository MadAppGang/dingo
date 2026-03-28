package transpiler

import (
	"bytes"
	"fmt"
	"go/scanner"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/MadAppGang/dingo/pkg/ast"
	"github.com/MadAppGang/dingo/pkg/codegen"
	"github.com/MadAppGang/dingo/pkg/parser"
	"github.com/MadAppGang/dingo/pkg/sourcemap"
	"github.com/MadAppGang/dingo/pkg/tokenizer"
	"github.com/MadAppGang/dingo/pkg/typechecker"
	"github.com/MadAppGang/dingo/pkg/typeloader"
)

// extractLineMappingsFromGoAST extracts line mappings using Go's scanner.
// This uses proper token-based positioning - the scanner tokenizes the source
// and provides token.Pos for each token, which we convert to line numbers.
//
// //line directives appear as COMMENT tokens in Go's scanner.
//
// Error propagation generates a fixed pattern of 5 lines after the directive.
func extractLineMappingsFromGoAST(goSource []byte, kind string) []sourcemap.LineMapping {
	// Create file set and add file for position tracking
	fset := token.NewFileSet()
	file := fset.AddFile("", fset.Base(), len(goSource))

	// Initialize scanner with comment handling enabled
	var s scanner.Scanner
	s.Init(file, goSource, nil, scanner.ScanComments)

	var mappings []sourcemap.LineMapping

	// Scan all tokens looking for //line directive comments
	for {
		pos, tok, lit := s.Scan()
		if tok == token.EOF {
			break
		}

		// //line directives are COMMENT tokens
		if tok != token.COMMENT {
			continue
		}

		// Check if this comment is a //line directive
		// Use strings.HasPrefix for clean prefix check
		if !strings.HasPrefix(lit, "//line ") {
			continue
		}

		// Extract the directive content after "//line "
		directiveContent := lit[7:] // len("//line ") == 7

		// Parse the Dingo line number from the directive
		dingoLine := parseLineNumberFromDirective(directiveContent)
		if dingoLine <= 0 {
			continue
		}

		// Get the Go line number from the token position
		goLineNum := fset.Position(pos).Line

		// Create mapping with fixed expansion size based on error prop pattern
		// GoLineStart is the NEXT line (after directive), GoLineEnd is +4 lines (5 total)
		mappings = append(mappings, sourcemap.LineMapping{
			DingoLine:   dingoLine,
			GoLineStart: goLineNum + 1, // Code starts on next line after directive
			GoLineEnd:   goLineNum + 5, // Error prop is always 5 lines
			Kind:        kind,
		})
	}

	return mappings
}

// parseLineNumberFromDirective extracts the line number from a //line directive.
// Input format: "path/file.dingo:LINE" or "path/file.dingo:LINE:COL"
// Uses strings.Split for safe parsing without byte index manipulation.
func parseLineNumberFromDirective(directive string) int {
	// Split by colon - the line number is always present
	// Format: path/to/file.dingo:LINE or path/to/file.dingo:LINE:COL
	parts := strings.Split(directive, ":")

	if len(parts) < 2 {
		return 0
	}

	// Try parsing from the end - last part might be line or column
	lastPart := parts[len(parts)-1]
	if num, err := strconv.Atoi(lastPart); err == nil && num > 0 {
		// Check if there's a column (3+ parts with numeric second-to-last)
		if len(parts) >= 3 {
			secondLast := parts[len(parts)-2]
			if lineNum, err := strconv.Atoi(secondLast); err == nil && lineNum > 0 {
				return lineNum // This was line:col format
			}
		}
		return num // This was just line format
	}

	// Try second-to-last (handles edge cases)
	if len(parts) >= 3 {
		secondLast := parts[len(parts)-2]
		if num, err := strconv.Atoi(secondLast); err == nil && num > 0 {
			return num
		}
	}

	return 0
}

// fixColumnMappingGoLines correlates column mappings with line mappings to set correct GoLine.
//
// Column mappings are created during transformation with GoLine = DingoLine (placeholder).
// Line mappings are extracted after transformation with accurate GoLineStart values.
//
// For each column mapping with DingoLine = X:
//   - Find the line mapping with DingoLine = X
//   - Set GoLine = GoLineStart + 1 (the assignment line, not the directive line)
//
// This is necessary because the final Go line numbers depend on all transforms,
// which aren't known until after the complete transformation pass.
func fixColumnMappingGoLines(colMappings []sourcemap.ColumnMapping, lineMappings []sourcemap.LineMapping) []sourcemap.ColumnMapping {
	// Build lookup map: DingoLine -> GoLineStart
	dingoToGoLine := make(map[int]int)
	for _, lm := range lineMappings {
		dingoToGoLine[lm.DingoLine] = lm.GoLineStart
	}

	// Fix each column mapping's GoLine
	result := make([]sourcemap.ColumnMapping, len(colMappings))
	for i, cm := range colMappings {
		result[i] = cm
		if goLineStart, found := dingoToGoLine[cm.DingoLine]; found {
			// GoLine = GoLineStart + 1 because:
			// - GoLineStart is the //line directive line
			// - GoLineStart + 1 is the "tmp, err := expr" line where the function call lives
			result[i].GoLine = goLineStart + 1
		}
	}

	return result
}

// transformASTExpressions finds and transforms all Dingo expressions (match, lambda)
// to Go code using the AST-based parser and codegen pipeline.
//
// Process:
//  1. Find all match/lambda expression locations using FindDingoExpressions
//  2. Sort by position descending (transform from end to avoid offset shifts)
//  3. For each expression:
//     a. Parse the expression using pkg/parser
//     b. Set IsExpr on MatchExpr based on context (Assignment/Return/Argument = true)
//     c. Generate Go code using pkg/codegen
//     d. Splice generated code back into result
//  4. Return transformed source
//
// enumRegistry provides enum name resolution for match expressions (legacy format).
// fullEnumRegistry provides full registry with value enum metadata for match expressions.
// originalSrc is the original Dingo source (before transforms) for //line directives.
// filename is used to generate //line directives for accurate error reporting.
//
// Returns transformed source and line mappings for multi-line transforms.
// Line mappings enable the semantic builder to map Go positions back to Dingo
// for constructs that expand to multiple lines (safe nav, match, null coalesce).
// No machine comments are added to the generated code - mappings are pure metadata.
func transformASTExpressions(src []byte, enumRegistry map[string]string, fullEnumRegistry *ast.EnumRegistry, originalSrc []byte, filename string) ([]byte, []sourcemap.LineMapping, error) {
	// Find all Dingo expressions in the (potentially transformed) source
	locations, err := ast.FindDingoExpressions(src)
	if err != nil {
		return nil, nil, fmt.Errorf("find expressions: %w", err)
	}

	// If no expressions found, return source unchanged
	if len(locations) == 0 {
		return src, nil, nil
	}

	// Also find expressions in original source for accurate mapping positions
	var originalLocations []ast.ExprLocation
	if originalSrc != nil {
		originalLocations, _ = ast.FindDingoExpressions(originalSrc)
	}

	// Filter out only expressions that are nested inside ternary expressions
	// These will be handled by the ternary's codegen via GenerateExpr
	// Other expressions (e.g., standalone safe nav) should still be processed
	locations = filterExprNestedInTernary(locations)

	// Sort by position descending (highest offset first)
	// This allows transformation from end to beginning, avoiding offset shifts
	sort.Slice(locations, func(i, j int) bool {
		return locations[i].Start > locations[j].Start
	})

	result := src

	// Create FileSet for line number calculation (CLAUDE.md compliant)
	// Used to map byte offsets to line numbers for line mapping metadata
	srcFset := token.NewFileSet()
	srcFile := srcFset.AddFile("", srcFset.Base(), len(src))
	srcFile.SetLinesForContent(src)

	// Create FileSet for ORIGINAL source line calculation (CLAUDE.md compliant)
	// This is critical: src may be post-enum-expansion with different line numbers.
	// We need originalSrc to get accurate Dingo line numbers for line mappings.
	var originalSrcFile *token.File
	if len(originalSrc) > 0 {
		origFset := token.NewFileSet()
		originalSrcFile = origFset.AddFile("", origFset.Base(), len(originalSrc))
		originalSrcFile.SetLinesForContent(originalSrc)
	}

	// Collect line mappings for multi-line transforms
	// These enable the semantic builder to map Go positions back to Dingo
	var lineMappings []sourcemap.LineMapping

	// Track cumulative line delta from splices.
	// Since we process end-to-start, each splice may add lines that affect
	// the CURRENT position's line number (but not earlier positions we haven't processed yet).
	cumulativeLineDelta := 0

	// Shared counter for unique temp var names across all expressions
	// Counter starts at 0 and increments, so first temp var has no suffix
	tempCounter := 0

	// Transform each expression from end to beginning
	for _, loc := range locations {
		// Skip error propagation expressions - they're handled at statement level
		if loc.Kind == ast.ExprErrorProp {
			continue
		}

		// Extract expression source from original src, not result.
		// loc.Start/End are offsets into the original src, and result changes after each splice.
		exprSrc := src[loc.Start:loc.End]

		// Calculate original position ONCE at the beginning.
		// This is reused for both //line directives and line mappings.
		// Don't call findOriginalPosition multiple times as it marks locations as used.
		originalPos := -1
		if originalSrc != nil {
			originalPos = findOriginalPosition(originalLocations, loc, exprSrc, originalSrc)
		}

		// Handle expression types based on kind
		switch loc.Kind {
		case ast.ExprMatch, ast.ExprLambdaRust, ast.ExprLambdaTS, ast.ExprNullCoalesce, ast.ExprSafeNav, ast.ExprTernary:
			// Expression-level transformation (supported types)
		default:
			// Unknown expression kind - skip
			continue
		}

		// Parse the expression
		fset := token.NewFileSet()
		dingoNode, parseErr := parser.ParseExpr(fset, string(exprSrc))
		if parseErr != nil {
			// Use original source position when available (src may have enum expansion that shifts lines)
			var errLine, errCol int
			if originalPos >= 0 && originalSrcFile != nil {
				origPos := originalSrcFile.Position(originalSrcFile.Pos(originalPos))
				errLine = origPos.Line
				errCol = origPos.Column
			} else {
				pos := srcFset.Position(srcFile.Pos(loc.Start))
				errLine = pos.Line
				errCol = pos.Column
			}

			// Provide helpful error messages for common mistakes
			errMsg := parseErr.Error()
			if loc.Kind == ast.ExprNullCoalesce {
				// Check if this looks like a double ?? or missing right operand
				exprText := string(exprSrc)
				if len(exprText) > 2 && exprText[len(exprText)-2:] == "??" {
					errMsg = "null coalescing operator '??' requires a default value (e.g., x ?? defaultValue)"
				}
			}

			// Return structured error with position info for LSP diagnostics
			return nil, nil, &TranspileError{
				File:    filename,
				Line:    errLine,
				Col:     errCol,
				Message: errMsg,
			}
		}

		// Extract the actual Expr from DingoNode wrapper
		var expr ast.Expr
		if wrapped, ok := dingoNode.(*ast.ExprWrapper); ok {
			expr = wrapped.DingoExpr
		} else if astExpr, ok := dingoNode.(ast.Expr); ok {
			expr = astExpr
		} else {
			// Use original source position when available (src may have enum expansion that shifts lines)
			var errLine, errCol int
			if originalPos >= 0 && originalSrcFile != nil {
				origPos := originalSrcFile.Position(originalSrcFile.Pos(originalPos))
				errLine = origPos.Line
				errCol = origPos.Column
			} else {
				pos := srcFset.Position(srcFile.Pos(loc.Start))
				errLine = pos.Line
				errCol = pos.Column
			}
			return nil, nil, &TranspileError{
				File:    filename,
				Line:    errLine,
				Col:     errCol,
				Message: fmt.Sprintf("unexpected node type: %T", dingoNode),
			}
		}

		// Set IsExpr flag on MatchExpr based on context
		if matchExpr, ok := expr.(*ast.MatchExpr); ok {
			// Context determines if match needs IIFE wrapping
			matchExpr.IsExpr = loc.Context == ast.ContextAssignment ||
				loc.Context == ast.ContextReturn ||
				loc.Context == ast.ContextArgument
		}

		// Generate Go code with context for null coalesce/safe nav/match/ternary (human-like output)
		var genResult ast.CodeGenResult
		if (loc.Kind == ast.ExprNullCoalesce || loc.Kind == ast.ExprSafeNav || loc.Kind == ast.ExprMatch || loc.Kind == ast.ExprTernary) &&
			(loc.Context == ast.ContextReturn || loc.Context == ast.ContextAssignment || loc.Context == ast.ContextArgument) {
			// Create context for human-like code generation
			ctx := &codegen.GenContext{
				Context:        loc.Context,
				VarName:        loc.VarName,
				StatementStart: loc.StatementStart,
				StatementEnd:   loc.StatementEnd,
				EnumRegistry:   enumRegistry,     // Pass enum registry for match pattern resolution
				ValueEnumReg:   fullEnumRegistry, // Pass full registry for value enum detection in match
				TempCounter:    &tempCounter,     // Share counter for unique temp var names
			}

			// For assignments and arguments, try to infer the type
			if loc.Context == ast.ContextAssignment || loc.Context == ast.ContextArgument {
				switch loc.Kind {
				case ast.ExprSafeNav:
					// Infer type using go/types
					varType := inferSafeNavType(result, exprSrc)
					ctx.VarType = varType
				case ast.ExprMatch:
					// Infer type from match arm bodies
					if matchExpr, ok := expr.(*ast.MatchExpr); ok {
						varType := typechecker.InferMatchResultType(matchExpr, result)
						ctx.VarType = varType
					}
				case ast.ExprTernary:
					// Infer type from ternary branches
					if ternaryExpr, ok := expr.(*ast.TernaryExpr); ok {
						varType := inferTernaryType(ternaryExpr, result)
						ctx.VarType = varType
					}
				}
			}

			genResult = codegen.GenerateExprWithContext(expr, ctx)
		} else {
			// For non-context generation, still pass registry and counter for match expressions
			if loc.Kind == ast.ExprMatch {
				ctx := &codegen.GenContext{
					EnumRegistry: enumRegistry,
					ValueEnumReg: fullEnumRegistry,
					TempCounter:  &tempCounter,
				}
				genResult = codegen.GenerateExprWithContext(expr, ctx)
			} else {
				genResult = codegen.GenerateExpr(expr)
			}
		}

		if len(genResult.Output) == 0 && len(genResult.StatementOutput) == 0 {
			return nil, nil, fmt.Errorf("codegen produced no output for expression at byte %d", loc.Start)
		}

		// Determine what to replace and what to use as replacement
		var replaceStart, replaceEnd int
		var replacement []byte

		// Handle HoistedCode for argument context (variable declaration before statement)
		if len(genResult.HoistedCode) > 0 && loc.StatementEnd > loc.StatementStart {
			// Insert hoisted code before the statement
			// Then replace the expression with the temp variable name
			replaceStart = loc.Start
			replaceEnd = loc.End

			// Build replacement: hoisted code at statement start, temp var at expression location
			hoistedInsertPos := loc.StatementStart

			// Calculate original position for //line directive
			// Reuse pre-calculated originalPos from start of loop iteration
			dingoPos := loc.Start
			if originalPos >= 0 {
				dingoPos = originalPos
			}

			// Find line start and indentation for proper //line directive placement.
			// IMPORTANT: //line directives MUST start at column 1 (not indented).
			// Go's parser ignores //line directives that don't start at column 1.
			lineStart, indent := findLineStartAndIndent(result, hoistedInsertPos)

			// Build final hoisted code with //line directive before EACH line.
			// This ensures all lines of multi-line generated code (like error propagation)
			// map back to the same Dingo source line, preventing Go's //line auto-increment
			// from causing incorrect position mapping.
			var finalHoistedCode []byte
			if filename != "" && originalSrc != nil && len(originalSrc) > 0 {
				line, col := offsetToLineCol(originalSrc, dingoPos)
				if line > 0 && col > 0 {
					// Insert //line directive before each line of hoisted code
					finalHoistedCode = ast.InsertLineDirectivesForEachLine(filename, line, col, indent, genResult.HoistedCode)
				} else {
					finalHoistedCode = append(indent, genResult.HoistedCode...)
				}
			} else {
				finalHoistedCode = append(indent, genResult.HoistedCode...)
			}

			// Insert hoisted code at line start (column 1 for //line directive)
			newResult := make([]byte, 0, len(result)+len(finalHoistedCode)+len(genResult.Output)+len(indent))
			newResult = append(newResult, result[:lineStart]...)
			newResult = append(newResult, finalHoistedCode...)
			newResult = append(newResult, []byte("\n")...)

			// Re-add indentation for the statement line (hoistedInsertPos is after indent)
			newResult = append(newResult, indent...)
			// Replace expression with temp variable
			newResult = append(newResult, result[hoistedInsertPos:loc.Start]...)
			newResult = append(newResult, genResult.Output...)
			newResult = append(newResult, result[loc.End:]...)
			result = newResult
			continue
		}

		if len(genResult.StatementOutput) > 0 && loc.StatementEnd > loc.StatementStart {
			// Statement-level replacement (human-like output)
			replaceStart = loc.StatementStart
			replaceEnd = loc.StatementEnd

			// Calculate original position for //line directive
			// Reuse pre-calculated originalPos from start of loop iteration
			dingoPos := loc.Start
			if originalPos >= 0 {
				dingoPos = originalPos
			}

			// Emit single //line directive before statement output
			if filename != "" && originalSrc != nil && len(originalSrc) > 0 {
				line, col := offsetToLineCol(originalSrc, dingoPos)
				if line > 0 && col > 0 {
					// Find indentation at statement start
					_, indent := findLineStartAndIndent(result, loc.StatementStart)
					directive := ast.FormatLineDirective(filename, line, col)
					// Build replacement: directive + indent + statement output
					replacement = append([]byte(directive), indent...)
					replacement = append(replacement, genResult.StatementOutput...)
				} else {
					replacement = genResult.StatementOutput
				}
			} else {
				replacement = genResult.StatementOutput
			}
		} else {
			// Expression-level replacement (IIFE fallback)
			replaceStart = loc.Start
			replaceEnd = loc.End
			replacement = genResult.Output
		}

		// NOTE: Do NOT emit //line directives for expression-level (IIFE) replacements.
		// These get spliced into the middle of statements (e.g., "return expr")
		// and a //line directive would break the syntax: "return //line ...\nfunc()..."
		//
		// //line directives ARE emitted for:
		// 1. HoistedCode (standalone statements before the expression)
		// 2. StatementOutput (match/safe_nav/null_coalesce/ternary statement transforms)
		// 3. Error propagation (separate path in transformErrorPropStatements)
		//
		// For expression-level (IIFE) transforms, gopls uses the nearby //line directives.

		// Splice generated code into result
		oldLen := replaceEnd - replaceStart
		newResult := make([]byte, 0, len(result)-oldLen+len(replacement))
		newResult = append(newResult, result[:replaceStart]...)
		newResult = append(newResult, replacement...)
		newResult = append(newResult, result[replaceEnd:]...)

		// Track line mapping for multi-line transforms (safe nav, match, null coalesce)
		// This enables the semantic builder to map Go positions back to Dingo
		// without adding any comments to the generated code
		if loc.Kind == ast.ExprSafeNav || loc.Kind == ast.ExprNullCoalesce ||
			loc.Kind == ast.ExprMatch || loc.Kind == ast.ExprTernary {
			// Count lines in original expression and generated code
			originalLineCount := bytes.Count(exprSrc, []byte{'\n'}) + 1
			generatedLineCount := bytes.Count(replacement, []byte{'\n'}) + 1

			// Only create mapping if the transform expands to multiple lines
			if generatedLineCount > 1 && originalSrcFile != nil {
				// Get Dingo line from ORIGINAL source, not the intermediate transformed source
				// The intermediate source (src) may have different line numbers due to enum expansion.
				// Reuse pre-calculated originalPos from start of loop iteration.
				var dingoLine int
				if originalPos >= 0 {
					// Found in original source - get accurate line number
					dingoLine = originalSrcFile.Line(originalSrcFile.Pos(originalPos))
				} else {
					// Fallback: use intermediate source position (may be inaccurate after enum expansion)
					dingoLine = srcFile.Line(srcFile.Pos(loc.Start))
				}

				// Calculate Go line number using the intermediate source line + cumulative delta.
				// srcFile gives us the line in the original intermediate source.
				// cumulativeLineDelta accounts for expansions from LATER transforms (already processed).
				srcLine := srcFile.Line(srcFile.Pos(loc.Start))
				goStartLine := srcLine + cumulativeLineDelta

				// If a //line directive was emitted, the actual code starts one line later.
				// The directive occupies the first line of the replacement, so we add 1 to
				// GoLineStart to point to where the actual transformed code begins.
				// We also subtract 1 from the line count since the directive isn't part of
				// the transformed code range.
				// This fixes off-by-one errors where code before the transform (e.g., function
				// declaration) was incorrectly included in the line mapping range.
				directiveEmitted := len(genResult.StatementOutput) > 0 && loc.StatementEnd > loc.StatementStart &&
					filename != "" && originalSrc != nil && len(originalSrc) > 0
				codeLineCount := generatedLineCount
				if directiveEmitted {
					goStartLine++
					codeLineCount-- // Exclude directive from code range
				}

				lineMappings = append(lineMappings, sourcemap.LineMapping{
					DingoLine:      dingoLine,
					DingoLineCount: originalLineCount,
					GoLineStart:    goStartLine,
					GoLineEnd:      goStartLine + codeLineCount - 1,
					Kind:           loc.Kind.String(),
				})
			}

			// Update cumulative delta: this transform added (generated - original) lines
			// Since we process end-to-start, this delta affects earlier (lower offset) positions
			lineDelta := generatedLineCount - originalLineCount
			cumulativeLineDelta += lineDelta
		}

		result = newResult
	}

	return result, lineMappings, nil
}

// findOriginalPosition finds the position of an expression in the original source
// by matching the expression content and kind. Returns -1 if not found.
// Since we process expressions in descending order, we also track which original
// locations have been used to avoid matching the same one twice.
func findOriginalPosition(originalLocations []ast.ExprLocation, loc ast.ExprLocation, exprContent []byte, originalSrc []byte) int {
	// Look for a matching expression by kind AND content
	// Process from end to beginning (matching the descending order of transforms).
	// Mark used locations by setting Start to -1 to avoid returning same position twice.
	for i := len(originalLocations) - 1; i >= 0; i-- {
		origLoc := &originalLocations[i] // Use pointer to modify in place
		if origLoc.Kind == loc.Kind && origLoc.Start >= 0 {
			// Extract content from original source at this location
			if origLoc.End <= len(originalSrc) {
				origContent := originalSrc[origLoc.Start:origLoc.End]
				// Compare content - must match to ensure correct correspondence
				if bytes.Equal(exprContent, origContent) {
					pos := origLoc.Start
					origLoc.Start = -1 // Mark as used so next call finds a different one
					return pos
				}
			}
		}
	}
	return -1 // Not found
}

// findOriginalErrorPropPosition finds the position in original source for error prop statements.
// If originalSrc == src (no earlier transforms), returns the position unchanged.
// Otherwise, attempts to find a matching ? operator in the original source.
func findOriginalErrorPropPosition(originalSrc []byte, transformedSrc []byte, pos int) int {
	// If sources are the same, no adjustment needed
	if bytes.Equal(originalSrc, transformedSrc) {
		return pos
	}

	// Simple heuristic: return the position as-is
	// The tracker will use this position relative to originalSrc
	// More sophisticated matching could be added if needed
	return pos
}

// originalLineInfo stores line/column info from original source for error prop statements
type originalLineInfo struct {
	line   int
	column int
}

// buildOriginalLineMap scans the original source for error propagation statements
// and returns a list of line info in source order.
// This is needed because earlier transforms (like enum expansion) change line counts.
// We use a list instead of a map because the same expression text can appear multiple times.
func buildOriginalLineMap(originalSrc []byte) []originalLineInfo {
	// Scan original source for error prop locations
	origLocations, err := ast.FindErrorPropStatements(originalSrc)
	if err != nil || len(origLocations) == 0 {
		return nil
	}

	// Build list of line info in source order (sorted by position)
	// Sort ascending by Start position to match processing order
	sort.Slice(origLocations, func(i, j int) bool {
		return origLocations[i].Start < origLocations[j].Start
	})

	lineInfos := make([]originalLineInfo, 0, len(origLocations))
	for _, loc := range origLocations {
		lineInfos = append(lineInfos, originalLineInfo{
			line:   loc.Line,
			column: loc.Column,
		})
	}

	return lineInfos
}

// transformErrorPropStatements transforms error propagation statements.
// originalSrc should be the original Dingo source (before any transforms) for accurate position tracking.
// filename is used for TypeResolver (cross-file type resolution) and optionally //line directives.
// emitDirectives controls whether //line directives are emitted (false for clean output).
// typeCache provides pre-loaded types for multi-file builds (nil falls back to per-file loading).
// Returns line mappings and column mappings for precise hover/go-to-definition.
func transformErrorPropStatements(src []byte, originalSrc []byte, filename string, emitDirectives bool, typeCache *typeloader.BuildCache) ([]byte, []sourcemap.LineMapping, []sourcemap.ColumnMapping, error) {
	var profileStart time.Time
	if os.Getenv("DINGO_PROFILE") == "1" {
		profileStart = time.Now()
	}

	locations, err := ast.FindErrorPropStatements(src)
	if err != nil {
		return src, nil, nil, err
	}

	if os.Getenv("DINGO_PROFILE") == "1" {
		fmt.Fprintf(os.Stderr, "    [PROFILE] FindErrorPropStatements           %v\n", time.Since(profileStart))
		profileStart = time.Now()
	}

	if len(locations) == 0 {
		return src, nil, nil, nil
	}

	// Create TypeResolver for accurate cross-file type resolution.
	// This is OPTIONAL - if it fails, we fall back to local search.
	var resolver *codegen.TypeResolver
	workingDir := extractWorkingDir(filename)
	if typeCache != nil {
		// Fast path: use pre-loaded cache (~0.05ms per file)
		resolver, _ = codegen.NewTypeResolverWithCache(src, workingDir, typeCache)
	} else {
		// Fallback: per-file loading (slower, used for single-file transpilation)
		resolver, _ = codegen.NewTypeResolver(src, workingDir)
	}
	// Ignore resolver creation errors - it's an optimization, not a requirement

	if os.Getenv("DINGO_PROFILE") == "1" {
		fmt.Fprintf(os.Stderr, "    [PROFILE] NewTypeResolver                   %v (skipped=%v)\n", time.Since(profileStart), resolver == nil)
		profileStart = time.Now()
	}

	// Pre-scan original source to get accurate line numbers
	// Earlier transforms (like enum expansion) change line counts, so we need original positions
	originalLineInfos := buildOriginalLineMap(originalSrc)

	// Sort locations by position (ascending) to match with originalLineInfos order
	// We'll reverse after to process end-to-beginning
	sort.Slice(locations, func(i, j int) bool {
		return locations[i].Start < locations[j].Start
	})

	// Create mapping from sorted index to original line info
	// This allows us to look up the correct line when processing each transform
	locationToLineInfo := make(map[int]originalLineInfo) // map[location index] → line info
	for i := 0; i < len(locations) && i < len(originalLineInfos); i++ {
		locationToLineInfo[i] = originalLineInfos[i]
	}

	// Now reverse to process end-to-beginning (to avoid position shifting)
	for i, j := 0, len(locations)-1; i < j; i, j = i+1, j-1 {
		locations[i], locations[j] = locations[j], locations[i]
	}

	result := src
	var lineMappings []sourcemap.LineMapping
	var columnMappings []sourcemap.ColumnMapping
	// Count only the locations that generate fresh (counter-named) variables.
	// Locations that use named return parameters directly (isPlainAssign with a named error
	// return, or bare statements whose errVar is a named return) do NOT consume a counter slot.
	// This ensures: first counter-consuming statement in source gets tmp/err (no suffix),
	// second gets tmp1/err1, etc.
	counterConsuming := 0
	for _, loc := range locations {
		switch loc.Kind {
		case ast.StmtErrorPropAssign, ast.StmtErrorPropLet:
			if loc.IsPlainAssign {
				// Uses named error return directly — no counter slot needed
				if codegen.InferEnclosingFunctionNamedErrorReturn(src, loc.ExprStart) == "" {
					counterConsuming++
				}
			} else {
				counterConsuming++
			}
		case ast.StmtErrorPropBare:
			// Uses named error return directly when the function has one — no counter slot needed
			if codegen.InferEnclosingFunctionNamedErrorReturn(src, loc.ExprStart) == "" {
				counterConsuming++
			}
		default:
			// Return statements and other kinds do not generate new variables
		}
	}
	counter := counterConsuming

	// First pass: calculate all deltas to know final positions
	// We need to track byte deltas from transforms to calculate correct Go positions
	type transformInfo struct {
		loc          ast.StmtLocation
		generated    []byte
		delta        int    // len(generated) - original length
		goLHSLen     int    // Length of Go LHS (e.g., "tmp, err := ")
		dingoLHSLen  int    // Length of Dingo LHS (e.g., "varName := ")
		counterValue int    // Counter value for this transform (determines tmp/tmpN naming)
		exprText     string // Expression text for looking up original line info
		origIndex    int    // Index in original (ascending) order for line lookup
	}
	transforms := make([]transformInfo, 0, len(locations))

	for loopIdx, loc := range locations {
		// Calculate original index (locations are now reversed, so last in loop = first in source)
		origIndex := len(locations) - 1 - loopIdx
		// Extract the expression between operator and ?
		exprBytes := src[loc.ExprStart : loc.ExprEnd-1] // -1 to exclude ?

		// Infer return types from enclosing function
		returnTypes := codegen.InferReturnTypes(result, loc.Start)

		// Check if enclosing function can propagate errors
		canPropagate, funcName := codegen.CanPropagateError(result, loc.Start)
		if !canPropagate {
			return nil, nil, nil, fmt.Errorf(
				"cannot use ? operator in function %q which has no error return type; "+
					"refactor error-prone code into a helper function that returns error, "+
					"or handle the error explicitly with if err != nil { ... }",
				funcName,
			)
		}

		// Extract lambda body if present (using token positions from finder)
		var lambdaBody []byte
		if loc.ErrorKind == ast.ErrorPropLambda && loc.LambdaBodyEnd > loc.LambdaBodyStart {
			lambdaBody = src[loc.LambdaBodyStart:loc.LambdaBodyEnd]
		}

		// Generate statement-level code
		// Capture counter before it decrements (for column mapping calculation)
		currentCounter := counter
		var generated []byte
		var goLHSLen, dingoLHSLen int
		switch loc.Kind {
		case ast.StmtErrorPropAssign, ast.StmtErrorPropLet:
			// x := foo()? or let x = foo()? or a, b := foo()?
			// Use TupleLHS for tuple assignments, otherwise VarName
			varNameOrTuple := loc.VarName
			if loc.TupleLHS != "" {
				varNameOrTuple = loc.TupleLHS
			}
			generated = generateErrorPropStatementAdvanced(result, exprBytes, loc.ExprStart, varNameOrTuple, returnTypes, &counter, loc.ErrorKind, loc.ErrorContext, loc.LambdaParam, lambdaBody, resolver, loc.IsPlainAssign)
			// Calculate LHS lengths for column mapping
			// Dingo: "varName := " or "a, b := " for tuples
			dingoLHSLen = len(varNameOrTuple) + 4 // " := " or " = " (always 4 with leading space consideration)
			// Go: either "tmp := " (Result types) or "tmpN, errN := " (tuple types)
			// Check if expression returns Result type to determine Go LHS format
			isResultType, _, _ := codegen.InferExprReturnsResultWithResolver(result, exprBytes, loc.ExprStart, resolver)
			if isResultType {
				// Result types use: tmp := expr (no err variable)
				if currentCounter == 1 {
					goLHSLen = 3 + 4 // "tmp" + " := "
				} else {
					goLHSLen = len(fmt.Sprintf("tmp%d", currentCounter-1)) + 4 // "tmpN" + " := "
				}
			} else {
				// Tuple types use: tmp, err := expr
				if currentCounter == 1 {
					goLHSLen = 3 + 2 + 3 + 4 // "tmp" + ", " + "err" + " := "
				} else {
					// tmpN and errN where N = currentCounter-1
					goLHSLen = len(fmt.Sprintf("tmp%d", currentCounter-1)) + 2 + len(fmt.Sprintf("err%d", currentCounter-1)) + 4
				}
			}
		case ast.StmtErrorPropReturn:
			// return foo()?
			generated = generateErrorPropReturnAdvanced(result, exprBytes, loc.ExprStart, returnTypes, &counter, loc.ErrorKind, loc.ErrorContext, loc.LambdaParam, lambdaBody, resolver)
			// For return statements, no variable assignment - expression starts immediately after "return "
			dingoLHSLen = 0
			goLHSLen = 0
		case ast.StmtErrorPropBare:
			// foo()?
			generated = generateErrorPropBareAdvanced(result, exprBytes, loc.ExprStart, returnTypes, &counter, loc.ErrorKind, loc.ErrorContext, loc.LambdaParam, lambdaBody, resolver)
			// For bare statements, no variable assignment
			dingoLHSLen = 0
			goLHSLen = 0
		}

		// Prepend //line directive before EACH line of generated code (when emitDirectives is true).
		// This ensures all lines of multi-line generated code (like error propagation)
		// map back to the same Dingo source line.
		// Look up correct line number from original source using original index
		// This is needed because earlier transforms (like enum expansion) change line counts
		var finalGenerated []byte
		var replaceStart int
		if emitDirectives {
			// Find line start and indentation for proper //line directive placement.
			// IMPORTANT: //line directives MUST start at column 1 (not indented).
			// Go's parser ignores //line directives that don't start at column 1.
			lineStart, indent := findLineStartAndIndent(result, loc.Start)
			replaceStart = lineStart

			// Look up line info by original index (order preserved from original source)
			if lineInfo, found := locationToLineInfo[origIndex]; found && lineInfo.line > 0 {
				// Insert //line directive before each line of generated code
				finalGenerated = ast.InsertLineDirectivesForEachLine(filename, lineInfo.line, lineInfo.column, indent, generated)
			} else if loc.Line > 0 && loc.Column > 0 {
				// Fall back to loc.Line/Column if not found (same source or no enum transforms)
				finalGenerated = ast.InsertLineDirectivesForEachLine(filename, loc.Line, loc.Column, indent, generated)
			} else {
				finalGenerated = append(indent, generated...)
			}
		} else {
			// No //line directives - simple replacement at loc.Start
			replaceStart = loc.Start
			finalGenerated = generated
		}

		// Replace in result
		newResult := make([]byte, 0, len(result)-int(loc.End-replaceStart)+len(finalGenerated))
		newResult = append(newResult, result[:replaceStart]...)
		newResult = append(newResult, finalGenerated...)
		newResult = append(newResult, result[loc.End:]...)
		result = newResult

		// Store transform info for second pass
		originalLen := loc.End - loc.Start
		delta := len(generated) - originalLen
		transforms = append(transforms, transformInfo{
			loc:          loc,
			generated:    generated,
			delta:        delta,
			goLHSLen:     goLHSLen,
			dingoLHSLen:  dingoLHSLen,
			counterValue: currentCounter,
			exprText:     string(exprBytes),
			origIndex:    origIndex,
		})
	}

	// Second pass: calculate Go positions accounting for all transforms
	// Process in source order (reverse of our processing order) to calculate cumulative shifts
	// For each transform, GoStart = DingoStart + sum of deltas from all earlier transforms

	// Sort transforms by DingoStart (ascending order for correct delta accumulation)
	sortedTransforms := make([]transformInfo, len(transforms))
	copy(sortedTransforms, transforms)
	sort.Slice(sortedTransforms, func(i, j int) bool {
		return sortedTransforms[i].loc.Start < sortedTransforms[j].loc.Start
	})

	// Calculate cumulative line delta and generate mappings for each transform
	cumulativeLineDelta := 0
	for _, t := range sortedTransforms {
		// Look up correct line/col from original source using original index
		// This is needed because earlier transforms (like enum expansion) change line counts
		var dingoLine, dingoCol int
		if lineInfo, found := locationToLineInfo[t.origIndex]; found && lineInfo.line > 0 {
			dingoLine = lineInfo.line
			dingoCol = lineInfo.column
		} else {
			// Fall back to calculating from byte offset (works when no enum transforms)
			origStart := findOriginalErrorPropPosition(originalSrc, src, t.loc.Start)
			dingoLine, dingoCol = offsetToLineCol(originalSrc, origStart)
		}

		// Count lines in generated code
		generatedLineCount := bytes.Count(t.generated, []byte{'\n'}) + 1

		// Calculate Go line number (Dingo line + cumulative line delta from previous transforms)
		goLineStart := dingoLine + cumulativeLineDelta

		// Generate line mapping: this Dingo line maps to a range of Go lines
		lineMappings = append(lineMappings, sourcemap.LineMapping{
			DingoLine:   dingoLine,
			GoLineStart: goLineStart,
			GoLineEnd:   goLineStart + generatedLineCount - 1,
			Kind:        "error_prop",
		})

		// Generate column mapping for function call position translation
		// Only for assignment statements where LHS changes (not for bare/return statements)
		if t.goLHSLen > 0 || t.dingoLHSLen > 0 {
			// Calculate expression column (after the assignment operator)
			// Dingo: "varName := expr?" - expression starts at column dingoLHSLen+1
			// Go: "tmp, err := expr" - expression starts at column goLHSLen+1
			// Column offset = goLHSLen - dingoLHSLen
			colOffset := t.goLHSLen - t.dingoLHSLen

			// Expression length (function call without the ? operator)
			exprLen := t.loc.ExprEnd - t.loc.ExprStart - 1 // -1 for ?

			// Calculate tab adjustment for go/printer's space→tab conversion.
			// Dingo uses 4 spaces for indentation, go/printer converts to tabs.
			// LSP counts tabs as 1 character, so we need to adjust GoCol.
			//
			// Example: Dingo "    x := getInt()?" (4 spaces = column 5)
			//          Go    "\ttmp, err := getInt()" (1 tab = column 2)
			// The leading 4 spaces become 1 tab = 3 characters shorter.
			leadingSpaces := dingoCol - 1               // Characters before first non-space (0-indexed)
			leadingTabs := (leadingSpaces + 3) / 4      // Ceiling division: tabs after go/printer
			indentAdjust := leadingSpaces - leadingTabs // Characters "saved" by tab compression

			columnMappings = append(columnMappings, sourcemap.ColumnMapping{
				DingoLine: dingoLine,
				DingoCol:  dingoCol + t.dingoLHSLen, // Column where expression starts
				GoLine:    goLineStart,              // First line of generated Go code
				GoCol:     dingoCol + t.dingoLHSLen + colOffset - indentAdjust,
				Length:    exprLen,
				Kind:      "error_prop",
			})
		}

		// Accumulate line delta for subsequent transforms
		// (generatedLineCount - 1 because we replaced 1 line with generatedLineCount lines)
		cumulativeLineDelta += generatedLineCount - 1
	}

	return result, lineMappings, columnMappings, nil
}

// generateErrorPropStatementAdvanced generates code for statement-level error propagation
// with support for all three error kinds: basic, context, and lambda.
//
// For tuple returns (T, error):
//
//	tmp, err := expr; if err != nil { return ..., err }; data := tmp
//
// For Result[T, E] returns:
//
//	tmp := expr; if tmp.IsErr() { return dgo.Err[T, E](tmp.MustErr()) }; data := tmp.MustOk()
//
// Context and Lambda variants wrap the error appropriately.
func generateErrorPropStatementAdvanced(src []byte, expr []byte, exprPos int, varName string, returnTypes []string, counter *int, errorKind ast.ErrorPropKind, errorContext string, lambdaParam string, lambdaBody []byte, resolver *codegen.TypeResolver, isPlainAssign bool) []byte {
	// Check if expression returns a Result type (use resolver for cross-file types)
	isResult, exprOkType, exprErrType := codegen.InferExprReturnsResultWithResolver(src, expr, exprPos, resolver)

	if isResult {
		return generateResultErrorPropStatement(expr, varName, counter, errorKind, errorContext, lambdaParam, lambdaBody, src, exprPos, exprOkType, exprErrType)
	}

	// Original tuple-based error propagation
	return generateTupleErrorPropStatement(expr, varName, returnTypes, counter, errorKind, errorContext, lambdaParam, lambdaBody, src, exprPos, isPlainAssign)
}

// generateResultErrorPropStatement generates code for Result[T, E] error propagation.
//
// Pattern when enclosing function returns Result[T, E]:
//
//	tmp := expr
//	if tmp.IsErr() { return dgo.Err[EnclosingOkType, EnclosingErrType](tmp.MustErr()) }
//	varName := tmp.MustOk()
//
// Pattern when enclosing function returns just error:
//
//	tmp := expr
//	if tmp.IsErr() { return tmp.MustErr() }
//	varName := tmp.MustOk()
func generateResultErrorPropStatement(expr []byte, varName string, counter *int, errorKind ast.ErrorPropKind, errorContext string, lambdaParam string, lambdaBody []byte, src []byte, exprPos int, exprOkType string, exprErrType string) []byte {
	var buf bytes.Buffer

	// Generate unique variable name
	var tmpVar string
	if *counter == 1 {
		tmpVar = "tmp"
	} else {
		tmpVar = fmt.Sprintf("tmp%d", *counter-1)
	}
	*counter--

	// Check if enclosing function returns Result[T, E] or just error
	enclosingReturnsResult, enclosingOkType, enclosingErrType := codegen.InferEnclosingFunctionResultTypes(src, exprPos)

	// tmp := expr
	buf.WriteString(tmpVar)
	buf.WriteString(" := ")
	buf.Write(expr)
	buf.WriteByte('\n')

	// if tmp.IsErr() { return ... }
	buf.WriteString("if ")
	buf.WriteString(tmpVar)
	buf.WriteString(".IsErr() {\n\treturn ")

	if enclosingReturnsResult {
		// Enclosing function returns Result[T, E]
		// Generate: return dgo.Err[T, E](ERROR_VALUE)
		buf.WriteString("dgo.Err[")
		buf.WriteString(enclosingOkType)
		buf.WriteString(", ")
		buf.WriteString(enclosingErrType)
		buf.WriteString("](")

		// Generate the error value based on kind
		switch errorKind {
		case ast.ErrorPropContext:
			// fmt.Errorf("message: %w", tmp.MustErr())
			buf.WriteString(`fmt.Errorf("`)
			buf.WriteString(errorContext)
			buf.WriteString(`: %w", `)
			buf.WriteString(tmpVar)
			buf.WriteString(".MustErr())")
		case ast.ErrorPropLambda:
			// Substitute lambda param with tmp.MustErr() in body
			substituted := substituteIdentifier(lambdaBody, lambdaParam, tmpVar+".MustErr()")
			buf.Write(substituted)
		default:
			// Basic: just use tmp.MustErr()
			buf.WriteString(tmpVar)
			buf.WriteString(".MustErr()")
		}

		buf.WriteString(")")
	} else {
		// Enclosing function returns just error (not Result)
		// Generate: return ERROR_VALUE
		switch errorKind {
		case ast.ErrorPropContext:
			// fmt.Errorf("message: %w", tmp.MustErr())
			buf.WriteString(`fmt.Errorf("`)
			buf.WriteString(errorContext)
			buf.WriteString(`: %w", `)
			buf.WriteString(tmpVar)
			buf.WriteString(".MustErr())")
		case ast.ErrorPropLambda:
			// Substitute lambda param with tmp.MustErr() in body
			substituted := substituteIdentifier(lambdaBody, lambdaParam, tmpVar+".MustErr()")
			buf.Write(substituted)
		default:
			// Basic: just use tmp.MustErr()
			buf.WriteString(tmpVar)
			buf.WriteString(".MustErr()")
		}
	}

	buf.WriteString("\n}\n")

	// varName := tmp.MustOk() (or varName = tmp.MustOk() for underscore)
	buf.WriteString(varName)
	if varName == "_" {
		buf.WriteString(" = ")
	} else {
		buf.WriteString(" := ")
	}
	buf.WriteString(tmpVar)
	buf.WriteString(".MustOk()")

	return buf.Bytes()
}

// generateTupleErrorPropStatement generates code for tuple (T, error) error propagation.
//
// Pattern for tuple-returning enclosing function:
//
//	tmp, err := expr; if err != nil { return ..., err }; data := tmp
//
// Pattern for Result-returning enclosing function:
//
//	tmp, err := expr; if err != nil { return dgo.Err[T, E](err) }; data := tmp
func generateTupleErrorPropStatement(expr []byte, varName string, returnTypes []string, counter *int, errorKind ast.ErrorPropKind, errorContext string, lambdaParam string, lambdaBody []byte, src []byte, exprPos int, isPlainAssign bool) []byte {
	var buf bytes.Buffer

	// Check if varName contains comma - if so, it's a tuple LHS like "a, b"
	isTupleLHS := strings.Contains(varName, ",")

	// When isPlainAssign is true, the user wrote `varName = expr?` (plain = not :=).
	// This happens when varName is a named return parameter.
	// In that case, use the actual named error return variable and plain = assignment,
	// instead of counter-generated names and :=.
	var namedErrVar string
	if isPlainAssign {
		namedErrVar = codegen.InferEnclosingFunctionNamedErrorReturn(src, exprPos)
	}

	// Generate unique variable names
	var tmpVar, errVar string
	if namedErrVar != "" {
		// Plain assign with named return: use named error return directly, don't consume counter slot.
		// The counter is only consumed for generated (counter-named) variables to keep numbering stable.
		errVar = namedErrVar
	} else {
		if *counter == 1 {
			tmpVar = "tmp"
			errVar = "err"
		} else {
			tmpVar = fmt.Sprintf("tmp%d", *counter-1)
			errVar = fmt.Sprintf("err%d", *counter-1)
		}
		*counter--
	}

	// Check if enclosing function returns Result[T, E]
	// If so, we need to return dgo.Err[T, E](err) instead of tuple
	enclosingReturnsResult, enclosingOkType, enclosingErrType := codegen.InferEnclosingFunctionResultTypes(src, exprPos)

	// For tuple LHS, generate: a, b, err := expr (or a, b, err = expr for plain assign)
	// For single LHS with plain assign (named return), generate: varName, err = expr
	// For single LHS with define, generate: tmp, err := expr
	if isTupleLHS {
		// Tuple LHS: fullKey, keyHash, err := util.GeneratePersonalKey()
		declOp := " := "
		if isPlainAssign {
			declOp = " = "
		}
		buf.WriteString(varName)
		buf.WriteString(", ")
		buf.WriteString(errVar)
		buf.WriteString(declOp)
		buf.Write(expr)
		buf.WriteByte('\n')
	} else if isPlainAssign && namedErrVar != "" {
		// Single LHS with named return: result, err = expr
		buf.WriteString(varName)
		buf.WriteString(", ")
		buf.WriteString(errVar)
		buf.WriteString(" = ")
		buf.Write(expr)
		buf.WriteByte('\n')
	} else {
		// Single LHS: tmp, err := expr
		buf.WriteString(tmpVar)
		buf.WriteString(", ")
		buf.WriteString(errVar)
		buf.WriteString(" := ")
		buf.Write(expr)
		buf.WriteByte('\n')
	}

	// if err != nil { return ... }
	buf.WriteString("if ")
	buf.WriteString(errVar)
	buf.WriteString(" != nil {\n\treturn ")

	if enclosingReturnsResult {
		// Enclosing function returns Result[T, E], generate: return dgo.Err[T, E](ERROR_VALUE)
		buf.WriteString("dgo.Err[")
		buf.WriteString(enclosingOkType)
		buf.WriteString(", ")
		buf.WriteString(enclosingErrType)
		buf.WriteString("](")

		// Generate the error value based on kind
		switch errorKind {
		case ast.ErrorPropContext:
			// fmt.Errorf("message: %w", err)
			buf.WriteString(`fmt.Errorf("`)
			buf.WriteString(errorContext)
			buf.WriteString(`: %w", `)
			buf.WriteString(errVar)
			buf.WriteString(")")
		case ast.ErrorPropLambda:
			// Substitute lambda param with error var in body
			substituted := substituteIdentifier(lambdaBody, lambdaParam, errVar)
			buf.Write(substituted)
		default:
			// Basic: just use err
			buf.WriteString(errVar)
		}

		buf.WriteString(")")
	} else {
		// Enclosing function returns tuple, generate: return <zero-values>, ERROR_VALUE
		for i, zv := range returnTypes {
			if i > 0 {
				buf.WriteString(", ")
			}
			buf.WriteString(zv)
		}
		if len(returnTypes) > 0 {
			buf.WriteString(", ")
		}

		// Generate the error value based on kind
		switch errorKind {
		case ast.ErrorPropContext:
			// fmt.Errorf("message: %w", err)
			buf.WriteString(`fmt.Errorf("`)
			buf.WriteString(errorContext)
			buf.WriteString(`: %w", `)
			buf.WriteString(errVar)
			buf.WriteString(")")
		case ast.ErrorPropLambda:
			// Substitute lambda param with error var in body, emit directly (no IIFE)
			// e.g., |err| NewAppError(403, "msg", err) with err2 → NewAppError(403, "msg", err2)
			substituted := substituteIdentifier(lambdaBody, lambdaParam, errVar)
			buf.Write(substituted)
		default:
			// Basic: just return err
			buf.WriteString(errVar)
		}
	}

	// For tuple LHS, variables are already assigned (fullKey, keyHash, err := expr)
	// For single LHS with plain assign (named return), variables are also already assigned (varName, err = expr)
	// For single LHS with define, we need: varName := tmp
	if !isTupleLHS && !(isPlainAssign && namedErrVar != "") {
		buf.WriteString("\n}\n")
		buf.WriteString(varName)
		if varName == "_" {
			buf.WriteString(" = ")
		} else {
			buf.WriteString(" := ")
		}
		buf.WriteString(tmpVar)
	} else {
		// No trailing assignment — end the if block without an extra newline
		// to avoid introducing a blank line before the next source statement.
		buf.WriteString("\n}")
	}

	return buf.Bytes()
}

// inferSafeNavType attempts to infer the type of a safe navigation expression.
// It converts the ?. chain to regular . access and type-checks against the source context.
//
// Example:
//
//	exprSrc: "config?.Database?.Host"
//	Returns: "*string" (from go/types analysis)
//
// Returns empty string if type cannot be inferred.
func inferSafeNavType(fullSource []byte, exprSrc []byte) string {
	// Convert safe-nav chain to regular Go expression
	// config?.Database?.Host → config.Database.Host
	chainExpr := typechecker.ChainToExprString(string(exprSrc))
	if chainExpr == "" {
		return ""
	}

	// Convert entire source to valid Go by replacing ?. with .
	// This makes the source parseable by go/parser
	goSource := bytes.ReplaceAll(fullSource, []byte("?."), []byte("."))

	// Replace ?? expressions: "a ?? b" → "a" (we only need the left side for type inference)
	// This regex-like replacement handles the common patterns
	goSource = removeNullCoalesce(goSource)

	// Type check the converted source
	sc := typechecker.NewSourceChecker()
	if err := sc.ParseAndCheck("probe.go", goSource); err != nil {
		return ""
	}

	return sc.GetExprType(chainExpr)
}

// inferTernaryType attempts to infer the type of a ternary expression from its branches.
// It analyzes both the true and false branches to determine a common type.
//
// Example:
//
//	ternary: user.ID > 0 ? "Welcome" : "Hello"
//	Returns: "string"
//
// Returns empty string if type cannot be inferred.
func inferTernaryType(ternary *ast.TernaryExpr, fullSource []byte) string {
	// Use go/types to infer the type from the true branch (most specific)
	// This handles literals, complex expressions, and typed constants correctly
	if ternary.True != nil {
		if trueStr := ternary.True.String(); trueStr != "" && trueStr != "?:" {
			typeStr := inferExprTypeFromSource(fullSource, []byte(trueStr))
			if typeStr != "" {
				return typeStr
			}
		}
	}

	// Fallback: try false branch using go/types
	if ternary.False != nil {
		if falseStr := ternary.False.String(); falseStr != "" && falseStr != "?:" {
			typeStr := inferExprTypeFromSource(fullSource, []byte(falseStr))
			if typeStr != "" {
				return typeStr
			}
		}
	}

	// If go/types can't infer, return empty (codegen will use 'any')
	return ""
}

// inferExprTypeFromSource attempts to infer an expression's type using go/types.
func inferExprTypeFromSource(fullSource []byte, exprSrc []byte) string {
	// Convert Dingo syntax to Go for type checking
	goSource := bytes.ReplaceAll(fullSource, []byte("?."), []byte("."))
	goSource = removeNullCoalesce(goSource)

	// Type check the converted source
	sc := typechecker.NewSourceChecker()
	if err := sc.ParseAndCheck("probe.go", goSource); err != nil {
		return ""
	}

	return sc.GetExprType(string(exprSrc))
}

// removeNullCoalesce removes ?? operators and their right operands for type inference.
// Uses the Dingo tokenizer to properly find ?? tokens.
// "a ?? b" → "a" (we only need the left side for type checking)
func removeNullCoalesce(src []byte) []byte {
	// Use Dingo tokenizer to find ?? tokens
	tok := tokenizer.New(src)
	tokens, err := tok.Tokenize()
	if err != nil {
		// If tokenization fails, return source unchanged
		return src
	}

	// Collect positions of ?? operators and their right operands
	type removal struct {
		start int // Position of ??
		end   int // Position after right operand
	}
	var removals []removal

	for i, t := range tokens {
		if t.Kind == tokenizer.QUESTION_QUESTION {
			start := t.BytePos()
			// Find end: newline, semicolon, or closing delimiter
			endOffset := t.ByteEnd() + 1 // Start after ??
			for j := i + 1; j < len(tokens); j++ {
				nextTok := tokens[j]
				if nextTok.Kind == tokenizer.NEWLINE ||
					nextTok.Kind == tokenizer.SEMICOLON ||
					nextTok.Kind == tokenizer.EOF {
					endOffset = nextTok.BytePos()
					break
				}
				// For closing delimiters, stop before them
				if nextTok.Kind == tokenizer.RPAREN ||
					nextTok.Kind == tokenizer.RBRACE ||
					nextTok.Kind == tokenizer.RBRACKET {
					endOffset = nextTok.BytePos()
					break
				}
				// Keep advancing past the token
				endOffset = nextTok.ByteEnd() + 1
			}
			removals = append(removals, removal{start: start, end: endOffset})
		}
	}

	// Remove in reverse order to preserve offsets
	result := src
	for i := len(removals) - 1; i >= 0; i-- {
		r := removals[i]
		if r.end > len(result) {
			r.end = len(result)
		}
		newResult := make([]byte, 0, len(result)-(r.end-r.start))
		newResult = append(newResult, result[:r.start]...)
		newResult = append(newResult, result[r.end:]...)
		result = newResult
	}

	return result
}

// substituteIdentifier replaces occurrences of oldIdent with newIdent in src
// using token-aware replacement (only replaces IDENT tokens, not substrings).
// e.g., substituteIdentifier("NewAppError(403, err)", "err", "err2")
//
//	→ "NewAppError(403, err2)" (not "NewApperr2or(403, err2)")
func substituteIdentifier(src []byte, oldIdent, newIdent string) []byte {
	tok := tokenizer.New(src)
	tokens, err := tok.Tokenize()
	if err != nil {
		// Fallback: return original if tokenization fails
		return src
	}

	// Build result by copying source with replacements
	result := make([]byte, 0, len(src)+len(newIdent)*2)
	lastCopied := 0

	for _, t := range tokens {
		if t.Kind == tokenizer.IDENT && t.Lit == oldIdent {
			// Copy everything up to this token
			result = append(result, src[lastCopied:t.BytePos()]...)
			// Write new identifier
			result = append(result, newIdent...)
			// Skip past old identifier
			lastCopied = t.ByteEnd()
		}
	}

	// Copy remaining bytes
	if lastCopied < len(src) {
		result = append(result, src[lastCopied:]...)
	}

	return result
}

// generateErrorPropReturnAdvanced generates code for return statement error propagation
// with support for all three error kinds: basic, context, and lambda.
//
// For tuple returns (T, error):
//
//	tmp, err := expr; if err != nil { return ..., err }; return tmp
//
// For Result[T, E] returns:
//
//	tmp := expr; if tmp.IsErr() { return dgo.Err[T, E](tmp.MustErr()) }; return tmp.MustOk()
func generateErrorPropReturnAdvanced(src []byte, expr []byte, exprPos int, returnTypes []string, counter *int, errorKind ast.ErrorPropKind, errorContext string, lambdaParam string, lambdaBody []byte, resolver *codegen.TypeResolver) []byte {
	// Check if expression returns a Result type (use resolver for cross-file types)
	isResult, exprOkType, exprErrType := codegen.InferExprReturnsResultWithResolver(src, expr, exprPos, resolver)

	if isResult {
		return generateResultErrorPropReturn(expr, counter, errorKind, errorContext, lambdaParam, lambdaBody, src, exprPos, exprOkType, exprErrType)
	}

	// Original tuple-based error propagation
	return generateTupleErrorPropReturn(expr, returnTypes, counter, errorKind, errorContext, lambdaParam, lambdaBody)
}

// generateResultErrorPropReturn generates code for Result[T, E] return error propagation.
//
// Pattern when enclosing function returns Result[T, E]:
//
//	tmp := expr
//	if tmp.IsErr() { return dgo.Err[EnclosingOkType, EnclosingErrType](tmp.MustErr()) }
//	return tmp.MustOk()
//
// Pattern when enclosing function returns just error:
//
//	tmp := expr
//	if tmp.IsErr() { return tmp.MustErr() }
//	return tmp.MustOk()
func generateResultErrorPropReturn(expr []byte, counter *int, errorKind ast.ErrorPropKind, errorContext string, lambdaParam string, lambdaBody []byte, src []byte, exprPos int, exprOkType string, exprErrType string) []byte {
	var buf bytes.Buffer

	// Generate unique variable name
	var tmpVar string
	if *counter == 1 {
		tmpVar = "tmp"
	} else {
		tmpVar = fmt.Sprintf("tmp%d", *counter-1)
	}
	*counter--

	// Check if enclosing function returns Result[T, E] or just error
	enclosingReturnsResult, enclosingOkType, enclosingErrType := codegen.InferEnclosingFunctionResultTypes(src, exprPos)

	// tmp := expr
	buf.WriteString(tmpVar)
	buf.WriteString(" := ")
	buf.Write(expr)
	buf.WriteByte('\n')

	// if tmp.IsErr() { return ... }
	buf.WriteString("if ")
	buf.WriteString(tmpVar)
	buf.WriteString(".IsErr() {\n\treturn ")

	if enclosingReturnsResult {
		// Enclosing function returns Result[T, E]
		// Generate: return dgo.Err[T, E](ERROR_VALUE)
		buf.WriteString("dgo.Err[")
		buf.WriteString(enclosingOkType)
		buf.WriteString(", ")
		buf.WriteString(enclosingErrType)
		buf.WriteString("](")

		// Generate the error value based on kind
		switch errorKind {
		case ast.ErrorPropContext:
			// fmt.Errorf("message: %w", tmp.MustErr())
			buf.WriteString(`fmt.Errorf("`)
			buf.WriteString(errorContext)
			buf.WriteString(`: %w", `)
			buf.WriteString(tmpVar)
			buf.WriteString(".MustErr())")
		case ast.ErrorPropLambda:
			// Substitute lambda param with tmp.MustErr() in body
			substituted := substituteIdentifier(lambdaBody, lambdaParam, tmpVar+".MustErr()")
			buf.Write(substituted)
		default:
			// Basic: just use tmp.MustErr()
			buf.WriteString(tmpVar)
			buf.WriteString(".MustErr()")
		}

		buf.WriteString(")")
	} else {
		// Enclosing function returns just error (not Result)
		// Generate: return ERROR_VALUE
		switch errorKind {
		case ast.ErrorPropContext:
			// fmt.Errorf("message: %w", tmp.MustErr())
			buf.WriteString(`fmt.Errorf("`)
			buf.WriteString(errorContext)
			buf.WriteString(`: %w", `)
			buf.WriteString(tmpVar)
			buf.WriteString(".MustErr())")
		case ast.ErrorPropLambda:
			// Substitute lambda param with tmp.MustErr() in body
			substituted := substituteIdentifier(lambdaBody, lambdaParam, tmpVar+".MustErr()")
			buf.Write(substituted)
		default:
			// Basic: just use tmp.MustErr()
			buf.WriteString(tmpVar)
			buf.WriteString(".MustErr()")
		}
	}

	buf.WriteString("\n}\n")

	// return tmp.MustOk()
	buf.WriteString("return ")
	buf.WriteString(tmpVar)
	buf.WriteString(".MustOk()")

	return buf.Bytes()
}

// generateTupleErrorPropReturn generates code for tuple (T, error) return error propagation.
//
// Pattern:
//
//	tmp, err := expr; if err != nil { return ..., err }; return tmp
func generateTupleErrorPropReturn(expr []byte, returnTypes []string, counter *int, errorKind ast.ErrorPropKind, errorContext string, lambdaParam string, lambdaBody []byte) []byte {
	var buf bytes.Buffer

	// Generate unique variable names
	var tmpVar, errVar string
	if *counter == 1 {
		tmpVar = "tmp"
		errVar = "err"
	} else {
		tmpVar = fmt.Sprintf("tmp%d", *counter-1)
		errVar = fmt.Sprintf("err%d", *counter-1)
	}
	*counter--

	// tmp, err := expr
	buf.WriteString(tmpVar)
	buf.WriteString(", ")
	buf.WriteString(errVar)
	buf.WriteString(" := ")
	buf.Write(expr)
	buf.WriteByte('\n')

	// if err != nil { return ..., ERROR_VALUE }
	buf.WriteString("if ")
	buf.WriteString(errVar)
	buf.WriteString(" != nil {\n\treturn ")
	for i, zv := range returnTypes {
		if i > 0 {
			buf.WriteString(", ")
		}
		buf.WriteString(zv)
	}
	if len(returnTypes) > 0 {
		buf.WriteString(", ")
	}

	// Generate the error value based on kind
	switch errorKind {
	case ast.ErrorPropContext:
		// fmt.Errorf("message: %w", err)
		buf.WriteString(`fmt.Errorf("`)
		buf.WriteString(errorContext)
		buf.WriteString(`: %w", `)
		buf.WriteString(errVar)
		buf.WriteString(")")
	case ast.ErrorPropLambda:
		// Substitute lambda param with error var in body, emit directly (no IIFE)
		// e.g., |err| NewAppError(403, "msg", err) with err2 → NewAppError(403, "msg", err2)
		substituted := substituteIdentifier(lambdaBody, lambdaParam, errVar)
		buf.Write(substituted)
	default:
		// Basic: just return err
		buf.WriteString(errVar)
	}

	buf.WriteString("\n}\n")

	// return tmpVar
	buf.WriteString("return ")
	buf.WriteString(tmpVar)

	return buf.Bytes()
}

// generateErrorPropBareAdvanced generates code for bare statement error propagation.
//
// For expressions returning Result[T, E]:
//
//	tmp := expr; if tmp.IsErr() { return dgo.Err[T, E](tmp.MustErr()) }
//
// For expressions returning (T, error) tuples:
// Uses InferExprReturnCountWithResolver to detect:
// - Single-return functions (like row.Scan()) -> generates: err := expr
// - Multi-return functions (like db.Query()) -> generates: _, err := expr
//
// If resolver is provided, it enables cross-file/cross-package type resolution
// for accurate return count detection. This is especially important for external
// package methods like repository.Create().
//
// For external packages where detection fails, defaults to single-return (err :=)
// for bare statements since the user is explicitly not capturing any return value.
func generateErrorPropBareAdvanced(src []byte, expr []byte, exprPos int, returnTypes []string, counter *int, errorKind ast.ErrorPropKind, errorContext string, lambdaParam string, lambdaBody []byte, resolver *codegen.TypeResolver) []byte {
	// Check if expression returns a Result type (use resolver for cross-file types)
	isExprResult, exprOkType, exprErrType := codegen.InferExprReturnsResultWithResolver(src, expr, exprPos, resolver)

	if isExprResult {
		return generateResultErrorPropBare(expr, counter, errorKind, errorContext, lambdaParam, lambdaBody, src, exprPos, exprOkType, exprErrType)
	}

	// Original tuple-based error propagation for bare statements
	return generateTupleErrorPropBare(src, expr, exprPos, returnTypes, counter, errorKind, errorContext, lambdaParam, lambdaBody, resolver)
}

// generateResultErrorPropBare generates code for Result[T, E] bare error propagation.
//
// Pattern when enclosing function returns Result[T, E]:
//
//	tmp := expr
//	if tmp.IsErr() { return dgo.Err[EnclosingOkType, EnclosingErrType](tmp.MustErr()) }
//
// Pattern when enclosing function returns just error:
//
//	tmp := expr
//	if tmp.IsErr() { return tmp.MustErr() }
func generateResultErrorPropBare(expr []byte, counter *int, errorKind ast.ErrorPropKind, errorContext string, lambdaParam string, lambdaBody []byte, src []byte, exprPos int, exprOkType string, exprErrType string) []byte {
	var buf bytes.Buffer

	// Generate unique variable name
	var tmpVar string
	if *counter == 1 {
		tmpVar = "tmp"
	} else {
		tmpVar = fmt.Sprintf("tmp%d", *counter-1)
	}
	*counter--

	// Check if enclosing function returns Result[T, E] or just error
	enclosingReturnsResult, enclosingOkType, enclosingErrType := codegen.InferEnclosingFunctionResultTypes(src, exprPos)

	// tmp := expr
	buf.WriteString(tmpVar)
	buf.WriteString(" := ")
	buf.Write(expr)
	buf.WriteByte('\n')

	// if tmp.IsErr() { return ... }
	buf.WriteString("if ")
	buf.WriteString(tmpVar)
	buf.WriteString(".IsErr() {\n\treturn ")

	if enclosingReturnsResult {
		// Enclosing function returns Result[T, E]
		// Generate: return dgo.Err[T, E](ERROR_VALUE)
		buf.WriteString("dgo.Err[")
		buf.WriteString(enclosingOkType)
		buf.WriteString(", ")
		buf.WriteString(enclosingErrType)
		buf.WriteString("](")

		// Generate the error value based on kind
		switch errorKind {
		case ast.ErrorPropContext:
			// fmt.Errorf("message: %w", tmp.MustErr())
			buf.WriteString(`fmt.Errorf("`)
			buf.WriteString(errorContext)
			buf.WriteString(`: %w", `)
			buf.WriteString(tmpVar)
			buf.WriteString(".MustErr())")
		case ast.ErrorPropLambda:
			// Substitute lambda param with tmp.MustErr() in body
			substituted := substituteIdentifier(lambdaBody, lambdaParam, tmpVar+".MustErr()")
			buf.Write(substituted)
		default:
			// Basic: just use tmp.MustErr()
			buf.WriteString(tmpVar)
			buf.WriteString(".MustErr()")
		}

		buf.WriteString(")")
	} else {
		// Enclosing function returns just error (not Result)
		// Generate: return ERROR_VALUE
		switch errorKind {
		case ast.ErrorPropContext:
			// fmt.Errorf("message: %w", tmp.MustErr())
			buf.WriteString(`fmt.Errorf("`)
			buf.WriteString(errorContext)
			buf.WriteString(`: %w", `)
			buf.WriteString(tmpVar)
			buf.WriteString(".MustErr())")
		case ast.ErrorPropLambda:
			// Substitute lambda param with tmp.MustErr() in body
			substituted := substituteIdentifier(lambdaBody, lambdaParam, tmpVar+".MustErr()")
			buf.Write(substituted)
		default:
			// Basic: just use tmp.MustErr()
			buf.WriteString(tmpVar)
			buf.WriteString(".MustErr()")
		}
	}

	buf.WriteString("\n}")

	return buf.Bytes()
}

// generateTupleErrorPropBare generates code for tuple (T, error) bare error propagation.
//
// Pattern:
//
//	err := expr; if err != nil { return ..., err }  (single-return)
//	_, err := expr; if err != nil { return ..., err }  (multi-return)
func generateTupleErrorPropBare(src []byte, expr []byte, exprPos int, returnTypes []string, counter *int, errorKind ast.ErrorPropKind, errorContext string, lambdaParam string, lambdaBody []byte, resolver *codegen.TypeResolver) []byte {
	var buf bytes.Buffer

	// Check if there is a named error return parameter in the enclosing function.
	// If so, use that variable name directly (do NOT consume a counter slot).
	// This avoids "no new variables on left side of :=" compile errors.
	namedErrReturn := codegen.InferEnclosingFunctionNamedErrorReturn(src, exprPos)

	// Generate unique variable names
	var errVar string
	var errDeclOp string
	if namedErrReturn != "" {
		// Use the named error return directly — no new variable needed
		errVar = namedErrReturn
		errDeclOp = " = "
		// Do NOT decrement counter: this location uses named returns, not a fresh counter slot
	} else {
		if *counter == 1 {
			errVar = "err"
		} else {
			errVar = fmt.Sprintf("err%d", *counter-1)
		}
		*counter--
		errDeclOp = " := "
	}

	// Detect return count: 1 = single (error only), 2+ = multi (T, error), -1 = unknown
	// Use the resolver for cross-file type resolution if available
	returnCount := codegen.InferExprReturnCountWithResolver(src, expr, exprPos, resolver)

	// Detect if enclosing function returns Result type
	isResultReturn, resultOkType, resultErrType := codegen.InferEnclosingFunctionResultTypes(src, exprPos)

	// Generate assignment based on return count
	// For BARE statements (no user-specified LHS variable), we default to single-return when unknown
	// because the user is explicitly not capturing any non-error return value.
	// If a function returns (T, error) but user wrote a bare `foo()?`, they want to ignore T.
	if returnCount == 1 || returnCount == -1 {
		// Single return or unknown: err := expr (or err = expr for named returns)
		// For bare statements, single-return is safer default (avoids "assignment mismatch" errors)
		buf.WriteString(errVar)
		buf.WriteString(errDeclOp)
	} else {
		// Multi-return (returnCount > 1): _, err := expr (or _, err = expr for named returns)
		buf.WriteString("_, ")
		buf.WriteString(errVar)
		buf.WriteString(errDeclOp)
	}
	buf.Write(expr)
	buf.WriteByte('\n')

	// if err != nil { return ERROR_VALUE }
	buf.WriteString("if ")
	buf.WriteString(errVar)
	buf.WriteString(" != nil {\n\treturn ")

	// For Result-returning functions, we don't need zero values for other returns
	// Just return the error wrapped in dgo.Err[T]
	if !isResultReturn {
		for i, zv := range returnTypes {
			if i > 0 {
				buf.WriteString(", ")
			}
			buf.WriteString(zv)
		}
		if len(returnTypes) > 0 {
			buf.WriteString(", ")
		}
	}

	// Generate the error value based on kind and return type
	switch errorKind {
	case ast.ErrorPropContext:
		// fmt.Errorf("message: %w", err)
		errExpr := fmt.Sprintf(`fmt.Errorf("%s: %%w", %s)`, errorContext, errVar)
		if isResultReturn {
			buf.WriteString("dgo.Err[")
			buf.WriteString(resultOkType)
			buf.WriteString(", ")
			buf.WriteString(resultErrType)
			buf.WriteString("](")
			buf.WriteString(errExpr)
			buf.WriteString(")")
		} else {
			buf.WriteString(errExpr)
		}
	case ast.ErrorPropLambda:
		// Substitute lambda param with error var in body, emit directly (no IIFE)
		substituted := substituteIdentifier(lambdaBody, lambdaParam, errVar)
		if isResultReturn {
			buf.WriteString("dgo.Err[")
			buf.WriteString(resultOkType)
			buf.WriteString(", ")
			buf.WriteString(resultErrType)
			buf.WriteString("](")
			buf.Write(substituted)
			buf.WriteString(")")
		} else {
			buf.Write(substituted)
		}
	default:
		// Basic: just return err
		if isResultReturn {
			buf.WriteString("dgo.Err[")
			buf.WriteString(resultOkType)
			buf.WriteString(", ")
			buf.WriteString(resultErrType)
			buf.WriteString("](")
			buf.WriteString(errVar)
			buf.WriteString(")")
		} else {
			buf.WriteString(errVar)
		}
	}

	buf.WriteString("\n}")

	return buf.Bytes()
}

// transformGuardStatements transforms guard statements.
// This MUST run after error propagation but before expression-level transforms.
// originalSrc should be the original Dingo source (before any transforms) for accurate position tracking.
// filename is used for //line directive generation (pass "" to disable).
//
// Transforms:
//
//	guard user = FindUser(id) else |err| { return Err(err) }  →  tmp := FindUser(id); if tmp.IsErr() { err := *tmp.err; return ResultErr(err) }; user := *tmp.ok
//	guard (a, b) = ParseInfo(data) else { return None() }     →  tmp := ParseInfo(data); if tmp.IsNone() { return OptionNone[Info]() }; a := (*tmp.ok).Item1; b := (*tmp.ok).Item2
func transformGuardStatements(src []byte, originalSrc []byte, filename string) ([]byte, error) {
	locations, err := ast.FindGuardStatements(src)
	if err != nil {
		return src, err
	}

	if len(locations) == 0 {
		return src, nil
	}

	// Sort by position (descending) to avoid position shifting
	// Process from end to beginning so byte offsets remain valid
	for i, j := 0, len(locations)-1; i < j; i, j = i+1, j-1 {
		locations[i], locations[j] = locations[j], locations[i]
	}

	result := src

	// Share counter across all guard statements
	// Locations are sorted descending, so we iterate last-to-first in source order.
	// Counter starts high and decrements, so first guard in source gets lowest counter (tmp, tmp1, ...).
	// Example: 3 guards → counters 3, 2, 1 → generates tmp, tmp1, tmp2 in source order.
	counter := len(locations)

	for _, loc := range locations {
		// Infer type from expression and binding presence
		// HasBinding (|err|) is a strong signal for Result type
		exprType, err := codegen.InferExprTypeWithBinding(loc.ExprText, loc.HasBinding)
		if err != nil {
			return nil, &TranspileError{
				File:    filename,
				Line:    loc.Line,
				Col:     loc.Column,
				Message: fmt.Sprintf("cannot infer type: %v", err),
			}
		}

		// Generate code with source context and shared counter
		gen := codegen.NewGuardGenerator(loc, exprType)
		gen.SourceBytes = src
		gen.Counter = counter
		counter-- // Decrement for next guard
		genResult := gen.Generate()

		// Prepend //line directive if filename is provided
		// Use loc.Line and loc.Column directly - they're already calculated during parsing
		var generated []byte
		var replaceStart int
		if filename != "" && loc.Line > 0 && loc.Column > 0 {
			// Find line start and indentation for proper //line directive placement.
			// IMPORTANT: //line directives MUST start at column 1 (not indented).
			// Go's parser ignores //line directives that don't start at column 1.
			lineStart, indent := findLineStartAndIndent(result, loc.Start)
			replaceStart = lineStart

			// //line directive at column 1, then indented generated code
			lineDirective := ast.FormatLineDirective(filename, loc.Line, loc.Column)
			generated = append([]byte(lineDirective), indent...)
			generated = append(generated, genResult.Output...)
		} else {
			// No //line directives - simple replacement at loc.Start
			replaceStart = loc.Start
			generated = genResult.Output
		}

		// Replace in result
		oldLen := loc.End - replaceStart
		newResult := make([]byte, 0, len(result)-oldLen+len(generated))
		newResult = append(newResult, result[:replaceStart]...)
		newResult = append(newResult, generated...)
		newResult = append(newResult, result[loc.End:]...)
		result = newResult
	}

	return result, nil
}

// filterExprNestedInTernary removes expressions that are nested inside ternary expressions.
// These will be handled by the ternary's codegen via GenerateExpr.
// Other expressions (e.g., standalone safe nav, null coalesce) should still be processed.
//
// Example: For input "len(config?.Region) > 0 ? x : y", FindDingoExpressions returns:
//   - Ternary at 0-30
//   - SafeNav at 4-20
//
// After filtering, only the ternary is returned.
// The SafeNav will be handled when parsing/generating the ternary's condition.
func filterExprNestedInTernary(locations []ast.ExprLocation) []ast.ExprLocation {
	if len(locations) <= 1 {
		return locations
	}

	// Mark which expressions are nested inside ternary expressions
	isNestedInTernary := make([]bool, len(locations))

	for i := range locations {
		for j := range locations {
			if i == j {
				continue
			}
			// Only filter if the outer expression is a ternary
			if locations[j].Kind != ast.ExprTernary {
				continue
			}
			// Check if locations[i] is fully contained within the ternary locations[j]
			if locations[j].Start <= locations[i].Start && locations[i].End <= locations[j].End {
				// locations[i] is nested inside a ternary
				isNestedInTernary[i] = true
				break
			}
		}
	}

	// Return only expressions not nested in ternary
	var result []ast.ExprLocation
	for i, loc := range locations {
		if !isNestedInTernary[i] {
			result = append(result, loc)
		}
	}

	return result
}

// findLineStartAndIndent finds the start of the line (position after previous newline)
// and extracts the indentation between line start and the given position.
// Returns (lineStart, indentation) where lineStart is the byte position right after
// the previous newline (or 0 if at start of file).
//
// This is needed for //line directives which MUST start at column 1 (not indented).
// Go's parser ignores //line directives that don't start at column 1.
func findLineStartAndIndent(src []byte, pos int) (lineStart int, indent []byte) {
	// Scan backward to find the newline
	lineStart = 0
	for i := pos - 1; i >= 0; i-- {
		if src[i] == '\n' {
			lineStart = i + 1
			break
		}
	}

	// Extract indentation (whitespace between line start and pos)
	// CRITICAL: Return a COPY of the indentation to avoid slice aliasing bugs.
	// Callers often do append(indent, ...) which would corrupt the source buffer
	// if we returned a slice into src. Go's append writes into the underlying
	// array when there's capacity, which src always has.
	if lineStart < pos {
		indentSlice := src[lineStart:pos]
		indent = make([]byte, len(indentSlice))
		copy(indent, indentSlice)
	}
	return lineStart, indent
}

// offsetToLineCol converts a byte offset in source to 1-indexed line:col.
// Returns (0, 0) if offset is invalid.
//
// This uses Go's token.FileSet which handles line counting internally.
// The FileSet is the proper token-based approach for position tracking.
func offsetToLineCol(src []byte, offset int) (line, col int) {
	if offset < 0 || offset >= len(src) {
		return 0, 0
	}

	// Create a FileSet and add the source file
	fset := token.NewFileSet()
	file := fset.AddFile("", fset.Base(), len(src))

	// SetLinesForContent scans the source and records newline positions
	// This is the token-based way to set up line info
	file.SetLinesForContent(src)

	// Convert byte offset to token.Pos, then to Position (line:col)
	pos := file.Pos(offset)
	position := fset.Position(pos)

	return position.Line, position.Column
}

// extractWorkingDir extracts the directory containing a file.
// Returns "." if filename is empty or has no directory component.
// This is used to determine the working directory for go/packages loading.
func extractWorkingDir(filename string) string {
	if filename == "" {
		return "."
	}
	dir := filepath.Dir(filename)
	if dir == "" {
		return "."
	}
	return dir
}
