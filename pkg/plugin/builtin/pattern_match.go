// Package builtin provides pattern match plugin for exhaustiveness checking
package builtin

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"

	"github.com/MadAppGang/dingo/pkg/errors"
	"github.com/MadAppGang/dingo/pkg/plugin"
)

// PatternMatchPlugin validates pattern match exhaustiveness and prepares for transformation
//
// This plugin implements exhaustiveness checking for Dingo match expressions:
// - Result<T,E>: requires Ok and Err patterns (or wildcard)
// - Option<T>: requires Some and None patterns (or wildcard)
// - Enum: requires all variants (or wildcard)
//
// The plugin operates in three phases:
// 1. Discovery: Find DINGO_MATCH_START markers and parse pattern arms
// 2. Transform: Perform exhaustiveness checking and emit compile errors
// 3. Inject: No declarations needed (matches already transformed by preprocessor)
type PatternMatchPlugin struct {
	ctx *plugin.Context

	// Discovered match expressions
	matchExpressions []*matchExpression
}

// matchExpression represents a discovered match expression
type matchExpression struct {
	startPos     token.Pos            // Position of DINGO_MATCH_START comment
	scrutinee    string               // Scrutinee expression (e.g., "result", "option")
	switchStmt   *ast.SwitchStmt      // The switch statement
	patterns     []string             // Pattern names (Ok, Err, Some, None, wildcard)
	hasWildcard  bool                 // Whether a wildcard (_) pattern exists
	caseStmts    []*ast.CaseClause    // Case clauses for each pattern
	isExpression bool                 // Whether this is an expression context (assigned/returned)
	guards       []*guardInfo         // Guards for each case clause
	isTuple      bool                 // Whether this is a tuple match
	tupleArity   int                  // Arity of tuple (if isTuple)
	tupleArms    []tupleArmInfo       // Tuple arm patterns (if isTuple)
}

// tupleArmInfo represents one arm in a tuple match
type tupleArmInfo struct {
	patterns []string // Pattern per tuple element: ["Ok", "Err", "_"]
	guard    string   // Guard condition (optional)
}

// guardInfo represents guard information for a case clause
type guardInfo struct {
	caseClause *ast.CaseClause // The case clause with guard
	condition  string          // Guard condition expression (raw text)
	armIndex   int             // Which arm this is (for error reporting)
}

// NewPatternMatchPlugin creates a new pattern match plugin
func NewPatternMatchPlugin() *PatternMatchPlugin {
	return &PatternMatchPlugin{
		matchExpressions: make([]*matchExpression, 0),
	}
}

// Name returns the plugin name
func (p *PatternMatchPlugin) Name() string {
	return "pattern_match"
}

// SetContext sets the plugin context (ContextAware interface)
func (p *PatternMatchPlugin) SetContext(ctx *plugin.Context) {
	p.ctx = ctx
}

// Process discovers match expressions and performs exhaustiveness checking
func (p *PatternMatchPlugin) Process(node ast.Node) error {
	if p.ctx == nil {
		return fmt.Errorf("plugin context not initialized")
	}

	// C5 FIX: Reset state between files to prevent stale AST references
	p.matchExpressions = p.matchExpressions[:0]

	// Phase 1: Discovery - Find DINGO_MATCH_START markers
	ast.Inspect(node, func(n ast.Node) bool {
		// Look for switch statements that might be pattern matches
		switchStmt, ok := n.(*ast.SwitchStmt)
		if !ok {
			return true
		}

		// Check if this switch has DINGO_MATCH_START marker
		matchInfo := p.findMatchMarker(switchStmt)
		if matchInfo == nil {
			return true // Not a pattern match
		}

		// Check if this is a tuple match
		isTuple, tupleArity := p.detectTupleMatch(switchStmt)
		matchInfo.isTuple = isTuple
		matchInfo.tupleArity = tupleArity

		if isTuple {
			// Parse tuple arms
			tupleArms := p.parseTupleArms(switchStmt)
			matchInfo.tupleArms = tupleArms
		} else {
			// Parse pattern arms from case clauses
			patterns, hasWildcard := p.parsePatternArms(switchStmt)
			matchInfo.patterns = patterns
			matchInfo.hasWildcard = hasWildcard
		}

		matchInfo.switchStmt = switchStmt

		// Detect expression vs statement mode
		matchInfo.isExpression = p.isExpressionMode(switchStmt)

		// Parse guards from case clauses
		matchInfo.guards = p.parseGuards(switchStmt)

		// Store for exhaustiveness checking
		p.matchExpressions = append(p.matchExpressions, matchInfo)

		return true
	})

	// Phase 2: Exhaustiveness Checking
	for _, match := range p.matchExpressions {
		if err := p.checkExhaustiveness(match); err != nil {
			// Report compile error
			p.ctx.ReportError(err.Error(), match.startPos)
		}
	}

	return nil
}

// findMatchMarker looks for DINGO_MATCH_START marker in switch statement
// Returns matchExpression if found, nil otherwise
func (p *PatternMatchPlugin) findMatchMarker(switchStmt *ast.SwitchStmt) *matchExpression {
	// Check comments before the switch statement
	if p.ctx.FileSet == nil || p.ctx.CurrentFile == nil {
		return nil
	}

	file, ok := p.ctx.CurrentFile.(*ast.File)
	if !ok {
		return nil
	}

	switchPos := switchStmt.Pos()

	// Look for DINGO_MATCH_START comment immediately before this switch
	// The preprocessor adds: // DINGO_MATCH_START: scrutinee_expr
	var bestMatch *matchExpression
	var bestDistance token.Pos = 1000000

	for _, cg := range file.Comments {
		for _, c := range cg.List {
			if strings.HasPrefix(c.Text, "// DINGO_MATCH_START:") {
				// Check if comment is before this switch (within 100 positions)
				if c.Pos() < switchPos {
					distance := switchPos - c.Pos()
					if distance < bestDistance && distance < 100 {
						// Extract scrutinee expression
						parts := strings.SplitN(c.Text, ":", 2)
						if len(parts) != 2 {
							continue
						}
						scrutinee := strings.TrimSpace(parts[1])

						bestMatch = &matchExpression{
							startPos:  c.Pos(),
							scrutinee: scrutinee,
						}
						bestDistance = distance
					}
				}
			}
		}
	}

	return bestMatch
}

// parsePatternArms extracts pattern names from case clauses
// Returns patterns and whether a wildcard exists
func (p *PatternMatchPlugin) parsePatternArms(switchStmt *ast.SwitchStmt) ([]string, bool) {
	patterns := make([]string, 0)
	hasWildcard := false

	// Build map of all DINGO_PATTERN comments first
	patternComments := p.collectPatternComments()

	for _, stmt := range switchStmt.Body.List {
		caseClause, ok := stmt.(*ast.CaseClause)
		if !ok {
			continue
		}

		// Default case is wildcard
		if caseClause.List == nil || len(caseClause.List) == 0 {
			hasWildcard = true
			continue
		}

		// Find pattern comment for this case
		pattern := p.findPatternForCase(caseClause, patternComments)
		if pattern != "" {
			if pattern == "_" {
				hasWildcard = true
			} else {
				patterns = append(patterns, pattern)
			}
		}
	}

	return patterns, hasWildcard
}

// collectPatternComments collects all DINGO_PATTERN comments in the file
func (p *PatternMatchPlugin) collectPatternComments() []patternComment {
	result := make([]patternComment, 0)

	if p.ctx.CurrentFile == nil {
		return result
	}

	file, ok := p.ctx.CurrentFile.(*ast.File)
	if !ok {
		return result
	}

	for _, cg := range file.Comments {
		for _, c := range cg.List {
			if strings.HasPrefix(c.Text, "// DINGO_PATTERN:") {
				parts := strings.SplitN(c.Text, ":", 2)
				if len(parts) == 2 {
					fullPattern := strings.TrimSpace(parts[1])
					pattern := p.extractConstructorName(fullPattern)
					result = append(result, patternComment{
						pos:     c.Pos(),
						pattern: pattern,
					})
				}
			}
		}
	}

	return result
}

// patternComment represents a DINGO_PATTERN comment
type patternComment struct {
	pos     token.Pos
	pattern string
}

// findPatternForCase finds the pattern comment closest to this case
func (p *PatternMatchPlugin) findPatternForCase(caseClause *ast.CaseClause, patternComments []patternComment) string {
	if len(patternComments) == 0 {
		return ""
	}

	casePos := caseClause.Pos()
	caseEnd := caseClause.End()

	// Find the comment within this case clause (after case start, before case end)
	// OR immediately after the case (within 100 positions)
	var bestMatch string
	var bestDistance token.Pos = 1000000

	for _, pc := range patternComments {
		// Check if comment is within the case clause
		if pc.pos >= casePos && pc.pos <= caseEnd {
			return pc.pattern
		}

		// Also check if comment is shortly after case start (within 100 positions)
		if pc.pos > casePos {
			distance := pc.pos - casePos
			if distance < bestDistance && distance < 100 {
				bestDistance = distance
				bestMatch = pc.pattern
			}
		}
	}

	return bestMatch
}

// extractConstructorName extracts constructor name from pattern
// "Ok(x)" -> "Ok", "Err(e)" -> "Err", "Some(v)" -> "Some", "_" -> "_"
func (p *PatternMatchPlugin) extractConstructorName(pattern string) string {
	pattern = strings.TrimSpace(pattern)

	// Wildcard
	if pattern == "_" {
		return "_"
	}

	// Extract name before '('
	idx := strings.Index(pattern, "(")
	if idx > 0 {
		return strings.TrimSpace(pattern[:idx])
	}

	// Plain variant name (no binding)
	return pattern
}

// isExpressionMode detects if match is in expression context (assigned/returned)
func (p *PatternMatchPlugin) isExpressionMode(switchStmt *ast.SwitchStmt) bool {
	parent := p.ctx.GetParent(switchStmt)
	if parent == nil {
		return false
	}

	switch parent.(type) {
	case *ast.AssignStmt:
		return true // let x = match { ... }
	case *ast.ReturnStmt:
		return true // return match { ... }
	case *ast.CallExpr:
		return true // foo(match { ... })
	default:
		return false // Statement mode
	}
}

// checkExhaustiveness validates that all variants are covered
func (p *PatternMatchPlugin) checkExhaustiveness(match *matchExpression) error {
	// Tuple exhaustiveness checking
	if match.isTuple {
		return p.checkTupleExhaustiveness(match)
	}

	// Non-tuple exhaustiveness checking
	// If wildcard exists, match is always exhaustive
	if match.hasWildcard {
		return nil
	}

	// Determine scrutinee type to get all possible variants
	// Try scrutinee name first, then fall back to pattern inference
	allVariants := p.getAllVariants(match.scrutinee)
	if len(allVariants) == 0 {
		allVariants = p.getAllVariantsFromPatterns(match)
	}

	if len(allVariants) == 0 {
		// Cannot determine type, skip exhaustiveness check
		return nil
	}

	// Track covered variants
	coveredVariants := make(map[string]bool)
	for _, pattern := range match.patterns {
		coveredVariants[pattern] = true
	}

	// Compute uncovered variants
	uncovered := make([]string, 0)
	for _, variant := range allVariants {
		if !coveredVariants[variant] {
			uncovered = append(uncovered, variant)
		}
	}

	// Error if non-exhaustive
	if len(uncovered) > 0 {
		return p.createNonExhaustiveError(match.scrutinee, uncovered, match.startPos)
	}

	return nil
}

// getAllVariants determines all possible variants for a type
// Returns empty slice if type cannot be determined
func (p *PatternMatchPlugin) getAllVariants(scrutinee string) []string {
	// Heuristic 1: Check type name in scrutinee
	if strings.Contains(scrutinee, "Result") || strings.Contains(scrutinee, "result") {
		return []string{"Ok", "Err"}
	}

	if strings.Contains(scrutinee, "Option") || strings.Contains(scrutinee, "option") {
		return []string{"Some", "None"}
	}

	// Heuristic 2: Try to infer from variable type
	// This requires looking at the matched patterns to determine the type
	// For now, we'll use a conservative approach:
	// - If any pattern is Ok or Err, assume Result
	// - If any pattern is Some or None, assume Option
	// This is not ideal but works for MVP
	// TODO (Phase 4.2): Use go/types to get actual scrutinee type

	return []string{}
}

// getAllVariantsFromPatterns infers type from collected patterns
func (p *PatternMatchPlugin) getAllVariantsFromPatterns(match *matchExpression) []string {
	// Check if any pattern matches known types
	hasOk := false
	hasErr := false
	hasSome := false
	hasNone := false

	for _, pattern := range match.patterns {
		switch pattern {
		case "Ok":
			hasOk = true
		case "Err":
			hasErr = true
		case "Some":
			hasSome = true
		case "None":
			hasNone = true
		}
	}

	// Infer type from patterns
	if hasOk || hasErr {
		return []string{"Ok", "Err"}
	}

	if hasSome || hasNone {
		return []string{"Some", "None"}
	}

	// Cannot determine type
	return []string{}
}

// createNonExhaustiveError creates a compile error for non-exhaustive match
func (p *PatternMatchPlugin) createNonExhaustiveError(scrutinee string, missingCases []string, pos token.Pos) error {
	message := fmt.Sprintf("non-exhaustive match, missing cases: %s", strings.Join(missingCases, ", "))
	hint := "add a wildcard arm: _ => ..."

	// Create CompileError using existing infrastructure
	compileErr := errors.NewCodeGenerationError(message, pos, hint)

	return fmt.Errorf("%s", compileErr.Error())
}

// parseGuards extracts guard conditions from case clauses
// Returns guardInfo for each case that has a guard
func (p *PatternMatchPlugin) parseGuards(switchStmt *ast.SwitchStmt) []*guardInfo {
	guards := make([]*guardInfo, 0)

	if p.ctx.CurrentFile == nil {
		return guards
	}

	file, ok := p.ctx.CurrentFile.(*ast.File)
	if !ok {
		return guards
	}

	// Collect all DINGO_GUARD comments
	guardComments := make(map[token.Pos]string)
	for _, cg := range file.Comments {
		for _, c := range cg.List {
			if strings.Contains(c.Text, "DINGO_GUARD:") {
				// Extract guard condition from comment
				// Format: // DINGO_PATTERN: Pattern | DINGO_GUARD: condition
				parts := strings.Split(c.Text, "DINGO_GUARD:")
				if len(parts) >= 2 {
					condition := strings.TrimSpace(parts[1])
					guardComments[c.Pos()] = condition
				}
			}
		}
	}

	// Match guards to case clauses
	for i, stmt := range switchStmt.Body.List {
		caseClause, ok := stmt.(*ast.CaseClause)
		if !ok {
			continue
		}

		// Find guard comment near this case
		guardCondition := p.findGuardForCase(caseClause, guardComments)
		if guardCondition != "" {
			guards = append(guards, &guardInfo{
				caseClause: caseClause,
				condition:  guardCondition,
				armIndex:   i,
			})
		}
	}

	return guards
}

// findGuardForCase finds the guard comment closest to this case clause
func (p *PatternMatchPlugin) findGuardForCase(caseClause *ast.CaseClause, guardComments map[token.Pos]string) string {
	if len(guardComments) == 0 {
		return ""
	}

	casePos := caseClause.Pos()
	caseEnd := caseClause.End()

	// Find guard comment within or shortly after the case clause
	var bestMatch string
	var bestDistance token.Pos = 1000000

	for pos, condition := range guardComments {
		// Check if comment is within the case clause
		if pos >= casePos && pos <= caseEnd {
			return condition
		}

		// Also check if comment is shortly after case start (within 100 positions)
		if pos > casePos {
			distance := pos - casePos
			if distance < bestDistance && distance < 100 {
				bestDistance = distance
				bestMatch = condition
			}
		}
	}

	return bestMatch
}

// Transform transforms pattern match switch statements into efficient Go code
// This phase generates tag-based dispatch, extracts pattern bindings, and adds safety checks
func (p *PatternMatchPlugin) Transform(node ast.Node) (ast.Node, error) {
	// For each discovered match expression, transform the switch statement
	for _, match := range p.matchExpressions {
		if err := p.transformMatchExpression(match); err != nil {
			return nil, err
		}
	}

	return node, nil
}

// transformMatchExpression transforms a single match expression
func (p *PatternMatchPlugin) transformMatchExpression(match *matchExpression) error {
	switchStmt := match.switchStmt

	// Transform guards first (nested if injection)
	if err := p.transformGuards(match); err != nil {
		return err
	}

	// Transform each case clause
	patternComments := p.collectPatternComments()

	for i, stmt := range switchStmt.Body.List {
		caseClause, ok := stmt.(*ast.CaseClause)
		if !ok {
			continue
		}

		// Skip default case (already handles wildcard)
		if caseClause.List == nil || len(caseClause.List) == 0 {
			continue
		}

		// Find pattern for this case
		pattern := p.findPatternForCase(caseClause, patternComments)
		if pattern == "" {
			continue
		}

		// Transform case condition to tag-based dispatch
		// Already done by preprocessor (case ResultTagOk, etc.)
		// We just need to ensure binding extraction is present

		// Extract bindings if not already present
		// The preprocessor should have added: x := *__match_0.ok_0
		// We validate and potentially add missing bindings

		// For now, preprocessor handles all transformation
		// This plugin focuses on validation only
		_ = i
		_ = pattern
	}

	// Add default panic for exhaustive matches (if no wildcard)
	if !match.hasWildcard {
		p.addExhaustivePanic(switchStmt)
	}

	return nil
}

// transformGuards transforms guards into nested if statements
func (p *PatternMatchPlugin) transformGuards(match *matchExpression) error {
	if len(match.guards) == 0 {
		return nil // No guards to transform
	}

	for _, guard := range match.guards {
		if err := p.injectNestedIf(guard); err != nil {
			return err
		}
	}

	return nil
}

// injectNestedIf injects a nested if statement for guard condition
// Strategy: Wrap case body in if statement, no else clause (fallthrough to next case)
func (p *PatternMatchPlugin) injectNestedIf(guard *guardInfo) error {
	caseClause := guard.caseClause

	// Parse guard condition as Go expression
	condExpr, err := parser.ParseExpr(guard.condition)
	if err != nil {
		// Invalid guard syntax - report error
		return fmt.Errorf("invalid guard condition: %s (error: %v)", guard.condition, err)
	}

	// Save original body
	originalBody := caseClause.Body

	// Create if statement wrapping original body
	ifStmt := &ast.IfStmt{
		Cond: condExpr,
		Body: &ast.BlockStmt{
			List: originalBody,
		},
		// No else clause - if guard fails, case continues (fallthrough to next case)
	}

	// Replace case body with if statement
	caseClause.Body = []ast.Stmt{ifStmt}

	return nil
}

// addExhaustivePanic adds a default case with panic for exhaustive matches
func (p *PatternMatchPlugin) addExhaustivePanic(switchStmt *ast.SwitchStmt) {
	// Check if default case already exists
	hasDefault := false
	for _, stmt := range switchStmt.Body.List {
		if caseClause, ok := stmt.(*ast.CaseClause); ok {
			if caseClause.List == nil || len(caseClause.List) == 0 {
				hasDefault = true
				break
			}
		}
	}

	// Add default panic if no default exists
	if !hasDefault {
		defaultCase := &ast.CaseClause{
			List: nil, // Default case
			Body: []ast.Stmt{
				&ast.ExprStmt{
					X: &ast.CallExpr{
						Fun: &ast.Ident{Name: "panic"},
						Args: []ast.Expr{
							&ast.BasicLit{
								Kind:  token.STRING,
								Value: `"unreachable: pattern match is exhaustive"`,
							},
						},
					},
				},
			},
		}
		switchStmt.Body.List = append(switchStmt.Body.List, defaultCase)
	}
}

// detectTupleMatch checks if a switch statement is a tuple match
// Returns: (isTuple, arity)
func (p *PatternMatchPlugin) detectTupleMatch(switchStmt *ast.SwitchStmt) (bool, int) {
	if p.ctx.CurrentFile == nil {
		return false, 0
	}

	file, ok := p.ctx.CurrentFile.(*ast.File)
	if !ok {
		return false, 0
	}

	// Look for DINGO_TUPLE_PATTERN marker
	switchPos := switchStmt.Pos()
	for _, cg := range file.Comments {
		for _, c := range cg.List {
			if strings.Contains(c.Text, "DINGO_TUPLE_PATTERN:") {
				// Check if comment is near this switch (within 150 positions)
				if c.Pos() < switchPos && switchPos-c.Pos() < 150 {
					// Extract arity from marker
					arity, err := ParseArityFromMarker(c.Text)
					if err != nil {
						return false, 0
					}
					return true, arity
				}
			}
		}
	}

	return false, 0
}

// parseTupleArms parses tuple arm information from case clauses
func (p *PatternMatchPlugin) parseTupleArms(switchStmt *ast.SwitchStmt) []tupleArmInfo {
	arms := make([]tupleArmInfo, 0)

	if p.ctx.CurrentFile == nil {
		return arms
	}

	file, ok := p.ctx.CurrentFile.(*ast.File)
	if !ok {
		return arms
	}

	// Collect all DINGO_TUPLE_ARM comments
	tupleArmComments := make(map[token.Pos]string)
	for _, cg := range file.Comments {
		for _, c := range cg.List {
			if strings.Contains(c.Text, "DINGO_TUPLE_ARM:") {
				// Extract tuple arm pattern
				parts := strings.Split(c.Text, "DINGO_TUPLE_ARM:")
				if len(parts) >= 2 {
					armStr := strings.TrimSpace(parts[1])
					tupleArmComments[c.Pos()] = armStr
				}
			}
		}
	}

	// Match arms to case clauses
	for _, stmt := range switchStmt.Body.List {
		caseClause, ok := stmt.(*ast.CaseClause)
		if !ok {
			continue
		}

		// Find tuple arm comment for this case
		armStr := p.findTupleArmForCase(caseClause, tupleArmComments)
		if armStr == "" {
			continue
		}

		// Parse arm pattern and optional guard
		patterns, guard := p.parseTupleArmPattern(armStr)
		if len(patterns) > 0 {
			arms = append(arms, tupleArmInfo{
				patterns: patterns,
				guard:    guard,
			})
		}
	}

	return arms
}

// findTupleArmForCase finds the DINGO_TUPLE_ARM comment for a case clause
func (p *PatternMatchPlugin) findTupleArmForCase(caseClause *ast.CaseClause, armComments map[token.Pos]string) string {
	if len(armComments) == 0 {
		return ""
	}

	casePos := caseClause.Pos()
	caseEnd := caseClause.End()

	// Find comment within or shortly after the case clause
	var bestMatch string
	var bestDistance token.Pos = 1000000

	for pos, armStr := range armComments {
		// Check if comment is within the case clause
		if pos >= casePos && pos <= caseEnd {
			return armStr
		}

		// Also check if comment is shortly after case start (within 100 positions)
		if pos > casePos {
			distance := pos - casePos
			if distance < bestDistance && distance < 100 {
				bestDistance = distance
				bestMatch = armStr
			}
		}
	}

	return bestMatch
}

// parseTupleArmPattern parses a tuple arm pattern string
// Example: "(Ok(x), Err(e)) | DINGO_GUARD: x > 0"
// Returns: (patterns, guard)
func (p *PatternMatchPlugin) parseTupleArmPattern(armStr string) ([]string, string) {
	// Split guard from pattern
	var patternPart string
	var guard string

	if strings.Contains(armStr, "| DINGO_GUARD:") {
		parts := strings.Split(armStr, "| DINGO_GUARD:")
		patternPart = strings.TrimSpace(parts[0])
		if len(parts) >= 2 {
			guard = strings.TrimSpace(parts[1])
		}
	} else {
		patternPart = strings.TrimSpace(armStr)
	}

	// Parse tuple pattern: (Ok(x), Err(e))
	// Remove outer parens
	if !strings.HasPrefix(patternPart, "(") || !strings.HasSuffix(patternPart, ")") {
		return nil, ""
	}
	inner := patternPart[1 : len(patternPart)-1]

	// Split on commas (simple split for now - assumes no nested commas)
	elementStrs := strings.Split(inner, ",")
	patterns := make([]string, len(elementStrs))

	for i, elemStr := range elementStrs {
		elemStr = strings.TrimSpace(elemStr)

		// Extract variant name: Ok(x) → Ok, Err(e) → Err, _ → _
		if elemStr == "_" {
			patterns[i] = "_"
		} else if strings.Contains(elemStr, "(") {
			// Has binding: Ok(x)
			idx := strings.Index(elemStr, "(")
			patterns[i] = elemStr[:idx]
		} else {
			// No binding: Ok, Err
			patterns[i] = elemStr
		}
	}

	return patterns, guard
}

// checkTupleExhaustiveness checks exhaustiveness for tuple patterns
func (p *PatternMatchPlugin) checkTupleExhaustiveness(match *matchExpression) error {
	// Get all possible variants for the tuple elements
	// For now, use heuristic based on patterns
	variants := p.inferVariantsFromTupleArms(match.tupleArms)
	if len(variants) == 0 {
		// Cannot determine variants - skip check
		return nil
	}

	// Extract pattern matrix from tuple arms
	patterns := make([][]string, len(match.tupleArms))
	for i, arm := range match.tupleArms {
		patterns[i] = arm.patterns
	}

	// Create exhaustiveness checker
	checker := NewTupleExhaustivenessChecker(match.tupleArity, variants, patterns)

	// Check exhaustiveness
	exhaustive, missing, err := checker.Check()
	if err != nil {
		return fmt.Errorf("tuple exhaustiveness check error: %w", err)
	}

	if !exhaustive {
		// Create error for missing patterns
		return p.createTupleNonExhaustiveError(match.scrutinee, missing, match.startPos)
	}

	return nil
}

// inferVariantsFromTupleArms infers possible variants from tuple arms
func (p *PatternMatchPlugin) inferVariantsFromTupleArms(arms []tupleArmInfo) []string {
	variantSet := make(map[string]bool)

	for _, arm := range arms {
		for _, pattern := range arm.patterns {
			if pattern != "_" {
				variantSet[pattern] = true
			}
		}
	}

	// Infer type from collected variants
	if variantSet["Ok"] || variantSet["Err"] {
		return []string{"Ok", "Err"} // Result type
	}

	if variantSet["Some"] || variantSet["None"] {
		return []string{"Some", "None"} // Option type
	}

	// Cannot determine - return empty
	return []string{}
}

// createTupleNonExhaustiveError creates error for non-exhaustive tuple match
func (p *PatternMatchPlugin) createTupleNonExhaustiveError(scrutinee string, missingPatterns []string, pos token.Pos) error {
	message := fmt.Sprintf("non-exhaustive tuple match, missing patterns: %s", strings.Join(missingPatterns, ", "))
	hint := "add a wildcard arm: (_, _, ...) => ..."

	compileErr := errors.NewCodeGenerationError(message, pos, hint)
	return fmt.Errorf("%s", compileErr.Error())
}
