package codegen

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"github.com/MadAppGang/dingo/pkg/ast"
	"github.com/MadAppGang/dingo/pkg/typechecker"
)

// MatchCodeGen generates Go code for match expressions.
// Transforms Dingo match expressions to Go switch statements with type assertions.
//
// Example transformation:
//
//	Dingo: match result { Ok(value) => value, Err(e) => 0 }
//	Go:    func() int { switch v := result.(type) { case Ok: value := v.Value; return value; case Err: e := v.Value; return 0 } }()
//
// With return context (human-like output - no IIFE):
//
//	Dingo: return match s { Status_Pending => "waiting", Status_Active => "active" }
//	Go:    switch s.(type) {
//	           case StatusPending: return "waiting"
//	           case StatusActive: return "active"
//	       }
//	       panic("unreachable: exhaustive match")
//
// Pattern types supported:
//   - ConstructorPattern: Ok(x), Err(e), Some(v), None
//   - VariablePattern: x (matches anything, binds to variable)
//   - WildcardPattern: _ (matches anything, no binding)
//   - LiteralPattern: 1, "hello", true (matches specific value)
//   - TuplePattern: (a, b) (matches tuple, binds elements)
type MatchCodeGen struct {
	*BaseGenerator
	Match            *ast.MatchExpr
	Context          *GenContext       // Optional context for human-like code generation
	scrutineeTempVar string            // Temp var name for scrutinee in type switch (e.g., "v")
	ValueEnumReg     *ast.EnumRegistry // Registry for value enum detection (from Phase 1/2)
}

// SharedTempVar generates unique temp var names using shared counter if available.
// Uses incrementing counter to ensure positive numbers in variable names.
// Falls back to local counter if no shared counter is provided.
func (g *MatchCodeGen) SharedTempVar(base string) string {
	if g.Context != nil && g.Context.TempCounter != nil {
		// Increment counter to get unique name
		// First call gets no suffix, subsequent calls get 1, 2, 3, ...
		counter := *g.Context.TempCounter
		*g.Context.TempCounter++
		if counter == 0 {
			return base
		}
		return fmt.Sprintf("%s%d", base, counter)
	}
	return g.TempVar(base)
}

// Constructor is in expr.go to avoid import cycles

// Generate generates Go code for the match expression.
// Returns CodeGenResult with generated code and source mappings.
func (g *MatchCodeGen) Generate() ast.CodeGenResult {
	if g.Match == nil {
		return ast.CodeGenResult{}
	}

	// CRITICAL: Check for Option/Result patterns FIRST before any other processing
	// Option and Result are structs, not interfaces, so they need method-based code
	if info := g.detectOptionResult(); info != nil {
		return g.generateOptionResultMatch(info)
	}

	// Check for value enum patterns SECOND
	// Value enums are typed constants, so they use simple switch (not type switch)
	if info := g.detectValueEnum(); info != nil {
		// Check exhaustiveness for expression matches
		if g.Match.IsExpr {
			if errResult := g.checkValueEnumExhaustiveness(info); errResult.Error != nil {
				return errResult
			}
		}
		return g.generateValueEnumMatch(info)
	}

	// Check exhaustiveness for expression matches (sum types)
	if g.Match.IsExpr {
		if errResult := g.checkExhaustiveness(); len(errResult.Output) > 0 {
			// Error result has non-empty Output containing error code
			return errResult
		}
	}

	// Check for assignment context - can generate temp var pattern
	if g.Match.IsExpr && g.Context != nil && g.Context.Context == ast.ContextAssignment {
		return g.generateHumanLikeAssignment()
	}

	// Check for argument context - can hoist switch before function call
	if g.Match.IsExpr && g.Context != nil && g.Context.Context == ast.ContextArgument {
		return g.generateHoistedMatch()
	}

	// Check for return context - can unwrap IIFE for human-like output
	if g.Match.IsExpr && g.Context != nil && g.Context.Context == ast.ContextReturn {
		// Return context: unwrap IIFE, generate switch with direct returns
		// This produces more idiomatic Go code
		g.generateMatchSwitch()

		// Move output to StatementOutput for statement-level replacement
		result := g.Result()
		result.StatementOutput = result.Output
		result.Output = nil
		return result
	} else if g.Match.IsExpr {
		// Other expression contexts (no context or unknown): wrap in IIFE
		g.generateMatchIIFE()
	} else {
		// Statement context: no IIFE needed
		g.generateMatchSwitch()
	}

	return g.Result()
}

// generateMatchIIFE generates IIFE wrapper for match expressions.
// Example: func() TYPE { switch ... }()
func (g *MatchCodeGen) generateMatchIIFE() {
	g.Write("func() interface{} {\n")
	g.generateMatchSwitch()
	g.Write("\n}()")
}

// generateMatchSwitch generates switch statement for match expression.
// Example: switch v := scrutinee.(type) { case Pattern: ... }
func (g *MatchCodeGen) generateMatchSwitch() {
	// Generate scrutinee
	scrutineeResult := GenerateExpr(g.Match.Scrutinee)
	scrutineeCode := string(scrutineeResult.Output)

	// CRITICAL FIX: Always bind match value to temp var to prevent double evaluation
	// This prevents side-effect functions from executing multiple times
	scrutineeTempVar := g.SharedTempVar("val")
	g.Write(fmt.Sprintf("%s := %s\n", scrutineeTempVar, scrutineeCode))

	// Check if we need type assertion (for constructor patterns)
	needsTypeSwitch := g.hasConstructorPatterns()

	// Check if we have any bindings (to decide on temp var usage)
	hasBindings := g.hasBindings()

	if needsTypeSwitch {
		// Generate type switch: switch v := scrutinee.(type) or switch scrutinee.(type)
		if hasBindings {
			// Need temp var for bindings
			tempVar := g.SharedTempVar("v")
			g.scrutineeTempVar = tempVar // Store for generateArmBody
			g.Write("switch " + tempVar + " := ")
			g.Write(scrutineeTempVar)
			g.Write(".(type) {\n")
		} else {
			// No bindings - just use scrutinee.(type) without variable
			g.Write("switch ")
			g.Write(scrutineeTempVar)
			g.Write(".(type) {\n")
		}
	} else {
		// Generate value switch: switch scrutinee {
		g.Write("switch ")
		g.Write(scrutineeTempVar)
		g.Write(" {\n")
	}

	// Group arms by pattern type to avoid duplicate case clauses
	// This handles guarded patterns correctly: multiple patterns for the same
	// constructor are combined into a single case with if/else chain
	g.generateGroupedCases(needsTypeSwitch)

	g.WriteByte('}')

	// CRITICAL: For expression matches in return context, add unreachable panic
	// This is NOT for runtime safety (exhaustiveness checker ensures all cases covered)
	// This is for Go's control flow analysis - it requires a return after the switch
	// The Go compiler will optimize this away as dead code
	if g.Match.IsExpr && g.Context != nil && g.Context.Context == ast.ContextReturn {
		g.Write("\npanic(\"unreachable: exhaustive match\")")
	}

	// NO PANIC for statement matches: silent fall-through per user requirement
}

// armGroup represents a group of arms that share the same case clause
type armGroup struct {
	typeName  string // The Go type name for constructor patterns
	arms      []*ast.MatchArm
	isDefault bool // true for wildcard/variable patterns (default case)
}

// generateGroupedCases groups arms by pattern type and generates cases.
// This is critical for handling guarded patterns:
// - Multiple arms for the same constructor become if/else chain in one case
// - Avoids duplicate case clauses which are illegal in Go
func (g *MatchCodeGen) generateGroupedCases(typeSwitch bool) {
	// Group arms by their case key (type name for constructors, "default" for wildcards)
	groups := make(map[string]*armGroup)
	var order []string // Preserve order

	for _, arm := range g.Match.Arms {
		key := g.getPatternCaseKey(arm.Pattern)

		if _, exists := groups[key]; !exists {
			groups[key] = &armGroup{
				typeName:  key,
				isDefault: key == "default",
			}
			order = append(order, key)
		}
		groups[key].arms = append(groups[key].arms, arm)
	}

	// Generate case for each group
	for _, key := range order {
		group := groups[key]
		g.generateGroupedCase(group, typeSwitch)
	}
}

// getPatternCaseKey returns the case key for a pattern.
// Constructor patterns return their full type name.
// Wildcard and variable patterns return "default".
func (g *MatchCodeGen) getPatternCaseKey(pattern ast.Pattern) string {
	switch p := pattern.(type) {
	case *ast.ConstructorPattern:
		return g.constructorToTypeName(p.Name)
	case *ast.LiteralPattern:
		return "literal:" + p.Value
	case *ast.WildcardPattern, *ast.VariablePattern:
		return "default"
	default:
		return "default"
	}
}

// generateGroupedCase generates a single case clause for a group of arms.
// If the group has multiple arms, generates if/else chain for guards.
func (g *MatchCodeGen) generateGroupedCase(group *armGroup, typeSwitch bool) {
	if len(group.arms) == 0 {
		return
	}

	firstArm := group.arms[0]

	// Generate case clause header
	if group.isDefault {
		g.Write("default:\n")
	} else if lit, ok := firstArm.Pattern.(*ast.LiteralPattern); ok {
		// Literal pattern
		g.Write("case " + lit.Value + ":\n")
	} else {
		// Constructor pattern
		g.Write("case " + group.typeName + ":\n")
	}

	// Extract bindings from first arm (all arms in group have same pattern structure)
	var bindings []Binding
	if cp, ok := firstArm.Pattern.(*ast.ConstructorPattern); ok {
		numParams := len(cp.Params)
		isTupleVariant := g.isTupleVariant(group.typeName)
		for i, param := range cp.Params {
			bindings = append(bindings, g.extractBindings(param, i, numParams, isTupleVariant)...)
		}
	} else if vp, ok := firstArm.Pattern.(*ast.VariablePattern); ok {
		bindings = []Binding{{Name: vp.Name, FieldPath: ""}}
	}

	// Generate bindings once (they're the same for all arms in this group)
	for _, binding := range bindings {
		bindingCode := binding.Name + " := "
		if binding.FieldPath != "" {
			tempVar := g.scrutineeTempVar
			if tempVar == "" {
				tempVar = "v"
			}
			bindingCode += tempVar + "." + binding.FieldPath
		} else {
			// Use scrutinee temp var to avoid double evaluation
			tempVar := g.scrutineeTempVar
			if tempVar == "" {
				// Fallback: generate expression (shouldn't happen in normal flow)
				scrutineeResult := GenerateExpr(g.Match.Scrutinee)
				bindingCode += string(scrutineeResult.Output)
			} else {
				bindingCode += tempVar
			}
		}
		bindingCode += "\n"
		g.Write(bindingCode)
	}

	// Generate if/else chain for multiple arms (guards)
	g.generateArmsChain(group.arms)
}

// generateArmsChain generates an if/else chain for a group of arms.
// This handles multiple patterns for the same constructor with guards.
func (g *MatchCodeGen) generateArmsChain(arms []*ast.MatchArm) {
	for i, arm := range arms {
		hasGuard := arm.Guard != nil
		isFirst := i == 0
		isLast := i == len(arms)-1

		if hasGuard {
			guardResult := GenerateExpr(arm.Guard)

			if isFirst {
				g.Write("if " + string(guardResult.Output) + " {\n")
			} else {
				g.Write("} else if " + string(guardResult.Output) + " {\n")
			}
		} else {
			// Unguarded arm
			if !isFirst {
				// This is the else fallback after guarded arms
				g.Write("} else {\n")
			}
			// If first and unguarded, no wrapper needed (direct code in case)
		}

		// Generate body
		bodyResult := GenerateExpr(arm.Body)
		// Check if body is already a return statement (ReturnExpr)
		_, isReturnExpr := arm.Body.(*ast.ReturnExpr)
		if g.Match.IsExpr && !isReturnExpr {
			// Only prepend "return" if body is not already a return statement
			g.Write("return " + string(bodyResult.Output) + "\n")
		} else {
			g.Buf.Write(bodyResult.Output)
			g.WriteByte('\n')
		}

		// Close if/else if last arm has guard or previous arms had guards
		if isLast && (hasGuard || i > 0 && arms[0].Guard != nil) {
			g.Write("}\n")
		}
	}
}

// generateGroupedCasesWithAssignment is like generateGroupedCases but for assignment context.
func (g *MatchCodeGen) generateGroupedCasesWithAssignment(varName string, typeSwitch bool) {
	// Group arms by their case key
	groups := make(map[string]*armGroup)
	var order []string

	for _, arm := range g.Match.Arms {
		key := g.getPatternCaseKey(arm.Pattern)
		if _, exists := groups[key]; !exists {
			groups[key] = &armGroup{
				typeName:  key,
				isDefault: key == "default",
			}
			order = append(order, key)
		}
		groups[key].arms = append(groups[key].arms, arm)
	}

	for _, key := range order {
		group := groups[key]
		g.generateGroupedCaseWithAssignment(group, varName, typeSwitch)
	}
}

// generateGroupedCaseWithAssignment generates a single case clause with assignment for a group.
func (g *MatchCodeGen) generateGroupedCaseWithAssignment(group *armGroup, varName string, typeSwitch bool) {
	if len(group.arms) == 0 {
		return
	}

	firstArm := group.arms[0]

	// Generate case clause header
	if group.isDefault {
		g.Write("default:\n")
	} else if lit, ok := firstArm.Pattern.(*ast.LiteralPattern); ok {
		g.Write("case " + lit.Value + ":\n")
	} else {
		g.Write("case " + group.typeName + ":\n")
	}

	// Extract bindings from first arm
	var bindings []Binding
	if cp, ok := firstArm.Pattern.(*ast.ConstructorPattern); ok {
		numParams := len(cp.Params)
		isTupleVariant := g.isTupleVariant(group.typeName)
		for i, param := range cp.Params {
			bindings = append(bindings, g.extractBindings(param, i, numParams, isTupleVariant)...)
		}
	} else if vp, ok := firstArm.Pattern.(*ast.VariablePattern); ok {
		bindings = []Binding{{Name: vp.Name, FieldPath: ""}}
	}

	// Generate bindings once
	for _, binding := range bindings {
		bindingCode := binding.Name + " := "
		if binding.FieldPath != "" {
			tempVar := g.scrutineeTempVar
			if tempVar == "" {
				tempVar = "v"
			}
			bindingCode += tempVar + "." + binding.FieldPath
		} else {
			scrutineeResult := GenerateExpr(g.Match.Scrutinee)
			bindingCode += string(scrutineeResult.Output)
		}
		bindingCode += "\n"
		g.Write(bindingCode)
	}

	// Generate if/else chain for multiple arms (guards) with assignment
	g.generateArmsChainWithAssignment(group.arms, varName)
}

// generateArmsChainWithAssignment generates an if/else chain with assignments.
func (g *MatchCodeGen) generateArmsChainWithAssignment(arms []*ast.MatchArm, varName string) {
	for i, arm := range arms {
		hasGuard := arm.Guard != nil
		isFirst := i == 0
		isLast := i == len(arms)-1

		if hasGuard {
			guardResult := GenerateExpr(arm.Guard)

			if isFirst {
				g.Write("if " + string(guardResult.Output) + " {\n")
			} else {
				g.Write("} else if " + string(guardResult.Output) + " {\n")
			}
		} else {
			if !isFirst {
				g.Write("} else {\n")
			}
		}

		// Generate assignment
		bodyResult := GenerateExpr(arm.Body)
		g.Write(fmt.Sprintf("\t%s = %s\n", varName, string(bodyResult.Output)))

		// Close if/else chain
		if isLast && (hasGuard || i > 0 && arms[0].Guard != nil) {
			g.Write("}\n")
		}
	}
}

// hasConstructorPatterns checks if any arm has constructor patterns.
// This determines whether we need a type switch or value switch.
func (g *MatchCodeGen) hasConstructorPatterns() bool {
	for _, arm := range g.Match.Arms {
		if _, ok := arm.Pattern.(*ast.ConstructorPattern); ok {
			return true
		}
	}
	return false
}

// hasBindings checks if any arm has patterns that need variable bindings.
// This determines whether we need a temp var in the type switch.
func (g *MatchCodeGen) hasBindings() bool {
	for _, arm := range g.Match.Arms {
		if g.patternHasBindings(arm.Pattern) {
			return true
		}
	}
	return false
}

// patternHasBindings recursively checks if a pattern has any variable bindings.
func (g *MatchCodeGen) patternHasBindings(pattern ast.Pattern) bool {
	switch p := pattern.(type) {
	case *ast.ConstructorPattern:
		// Constructor has bindings if any parameter has bindings
		for _, param := range p.Params {
			if g.patternHasBindings(param) {
				return true
			}
		}
		return false
	case *ast.VariablePattern:
		// Variable pattern is a binding
		return true
	case *ast.TuplePattern:
		// Tuple pattern has bindings if any element does
		for _, elem := range p.Elements {
			if g.patternHasBindings(elem) {
				return true
			}
		}
		return false
	default:
		// Wildcard, literal - no bindings
		return false
	}
}

// generateMatchArm generates a case clause for a match arm.
// Handles pattern matching, guards, and body generation.
func (g *MatchCodeGen) generateMatchArm(arm *ast.MatchArm, typeSwitch bool) {
	// Generate case clause based on pattern type
	switch pattern := arm.Pattern.(type) {
	case *ast.ConstructorPattern:
		g.generateConstructorCase(pattern, arm, typeSwitch)
	case *ast.LiteralPattern:
		g.generateLiteralCase(pattern, arm)
	case *ast.WildcardPattern:
		g.generateWildcardCase(arm)
	case *ast.VariablePattern:
		g.generateVariableCase(pattern, arm)
	case *ast.TuplePattern:
		g.generateTupleCase(pattern, arm)
	default:
		// Unknown pattern type - generate default case
		g.Write("default:\n")
		g.generateArmBody(arm, nil)
	}
}

// generateConstructorCase generates case for constructor patterns (Ok(x), Err(e), Some(v), None).
func (g *MatchCodeGen) generateConstructorCase(pattern *ast.ConstructorPattern, arm *ast.MatchArm, typeSwitch bool) {
	// Convert constructor name to full type name
	// - Result/Option: Ok → ResultOk, Some → OptionSome
	// - Enum: Status_Pending → StatusPending (strip underscore prefix)
	typeName := g.constructorToTypeName(pattern.Name)

	g.Write("case " + typeName + ":\n")

	// Extract bindings from constructor parameters
	var bindings []Binding
	numParams := len(pattern.Params)
	isTupleVariant := g.isTupleVariant(typeName)
	for i, param := range pattern.Params {
		bindings = append(bindings, g.extractBindings(param, i, numParams, isTupleVariant)...)
	}

	g.generateArmBody(arm, bindings)
}

// Binding represents a variable binding extracted from a pattern.
type Binding struct {
	Name      string
	FieldPath string // e.g., "Value", "First", "Second"
}

// extractBindings recursively extracts variable bindings from patterns.
// numParams indicates the total number of parameters in the constructor pattern.
// isTupleVariant indicates whether the current variant uses tuple (Value) or struct (named) fields.
func (g *MatchCodeGen) extractBindings(pattern ast.Pattern, index int, numParams int, isTupleVariant bool) []Binding {
	switch p := pattern.(type) {
	case *ast.VariablePattern:
		// Variable binding: x
		// For tuple variants: use "Value" (single field) or "ValueN" (multiple fields)
		// For struct variants: use the binding name as field name (e.g., userID)
		fieldPath := p.Name // Default: assume struct variant

		// Tuple variants use positional fields: Value, Value0/Value1/...
		if isTupleVariant {
			if numParams == 1 {
				fieldPath = "Value"
			} else {
				fieldPath = fmt.Sprintf("Value%d", index)
			}
		}

		return []Binding{{
			Name:      p.Name,
			FieldPath: fieldPath,
		}}
	case *ast.TuplePattern:
		// Tuple pattern: (a, b)
		var bindings []Binding
		tupleSize := len(p.Elements)
		for i, elem := range p.Elements {
			elemBindings := g.extractBindings(elem, i, tupleSize, isTupleVariant)
			for j := range elemBindings {
				// Adjust field path for tuple element
				elemBindings[j].FieldPath = fmt.Sprintf("Field%d", i)
			}
			bindings = append(bindings, elemBindings...)
		}
		return bindings
	case *ast.ConstructorPattern:
		// Nested constructor: Ok(Some(x))
		var bindings []Binding
		nestedNumParams := len(p.Params)
		// TODO: Determine if nested constructor is tuple variant
		nestedIsTuple := false // Conservative: assume struct for nested
		for i, param := range p.Params {
			bindings = append(bindings, g.extractBindings(param, i, nestedNumParams, nestedIsTuple)...)
		}
		return bindings
	default:
		// Wildcard or literal - no bindings
		return nil
	}
}

// generateLiteralCase generates case for literal patterns (1, "hello", true).
func (g *MatchCodeGen) generateLiteralCase(pattern *ast.LiteralPattern, arm *ast.MatchArm) {
	g.Write("case " + pattern.Value + ":\n")
	g.generateArmBody(arm, nil)
}

// generateWildcardCase generates default case for wildcard pattern (_).
func (g *MatchCodeGen) generateWildcardCase(arm *ast.MatchArm) {
	g.Write("default:\n")
	g.generateArmBody(arm, nil)
}

// generateVariableCase generates default case with binding for variable pattern (x).
func (g *MatchCodeGen) generateVariableCase(pattern *ast.VariablePattern, arm *ast.MatchArm) {
	g.Write("default:\n")

	// Bind variable to scrutinee value
	bindings := []Binding{{
		Name:      pattern.Name,
		FieldPath: "", // Bind to scrutinee itself
	}}
	g.generateArmBody(arm, bindings)
}

// generateTupleCase generates case for tuple patterns ((a, b)).
func (g *MatchCodeGen) generateTupleCase(pattern *ast.TuplePattern, arm *ast.MatchArm) {
	// Tuple patterns need special handling - not implemented yet
	// For now, generate default case
	g.Write("default:\n")
	g.generateArmBody(arm, nil)
}

// generateArmBody generates the body of a match arm.
// Handles variable bindings, guards, and body expression.
func (g *MatchCodeGen) generateArmBody(arm *ast.MatchArm, bindings []Binding) {
	// Generate variable bindings
	for _, binding := range bindings {
		bindingCode := binding.Name + " := "

		// If binding has a field path, use type assertion variable
		if binding.FieldPath != "" {
			// Use scrutineeTempVar (e.g., "v") set in generateMatchSwitch
			tempVar := g.scrutineeTempVar
			if tempVar == "" {
				tempVar = "v" // Fallback if not set
			}
			bindingCode += tempVar + "." + binding.FieldPath
		} else {
			// Bind to scrutinee temp var (for variable patterns in default case)
			// Use scrutinee temp var to avoid double evaluation
			tempVar := g.scrutineeTempVar
			if tempVar == "" {
				// Fallback: generate expression (shouldn't happen in normal flow)
				scrutineeResult := GenerateExpr(g.Match.Scrutinee)
				bindingCode += string(scrutineeResult.Output)
			} else {
				bindingCode += tempVar
			}
		}

		bindingCode += "\n"
		g.Write(bindingCode)
	}

	// Generate guard check if present
	if arm.Guard != nil {
		guardResult := GenerateExpr(arm.Guard)
		g.Write("if " + string(guardResult.Output) + " {\n")
	}

	// Generate body
	bodyResult := GenerateExpr(arm.Body)
	// Check if body is already a return statement (ReturnExpr)
	_, isReturnExpr := arm.Body.(*ast.ReturnExpr)
	if g.Match.IsExpr && !isReturnExpr {
		// Only prepend "return" if body is not already a return statement
		g.Write("return " + string(bodyResult.Output) + "\n")
	} else {
		g.Buf.Write(bodyResult.Output)
		g.WriteByte('\n')
	}

	// Close guard if present
	if arm.Guard != nil {
		g.Write("}\n")
		// NO PANIC: Guarded patterns are handled by exhaustiveness checking
		// Expression matches with guards must have unguarded fallback (verified by checker)
		// Statement matches with guards simply fall through if guard fails
	}
}

// constructorToTypeName converts a bare constructor name to the full type name.
// This handles both built-in Result/Option types and user-defined enums.
//
// NOTE: This is SEMANTIC TRANSFORMATION, not source parsing.
// The constructorName comes from ConstructorPattern.Name which was already
// properly parsed by pkg/parser. This function transforms naming conventions
// from Dingo pattern syntax to Go type names.
//
// Examples:
//   - Ok → ResultOk
//   - Err → ResultErr
//   - Some → OptionSome
//   - None → OptionNone
//   - Status_Pending → StatusPending (strip underscore)
//   - Status_Active → StatusActive (strip underscore)
//   - UserCreated (with registry) → EventUserCreated (prefix from enum registry)
func (g *MatchCodeGen) constructorToTypeName(constructorName string) string {
	// Check for built-in Result/Option constructors
	switch constructorName {
	case "Ok":
		return "ResultOk"
	case "Err":
		return "ResultErr"
	case "Some":
		return "OptionSome"
	case "None":
		return "OptionNone"
	}

	// For user-defined enums: Pattern has TypeName_Variant format
	// Example: "Status_Pending" → "StatusPending" (remove underscore)
	// This matches the generated enum variant type names
	//
	// Use strings.Cut instead of character scanning (CLAUDE.md compliant)
	if before, after, found := strings.Cut(constructorName, "_"); found && before != "" {
		return before + after
	}

	// Check enum registry for unqualified variant names
	// Example: "UserCreated" with registry["UserCreated"] = "Event" → "EventUserCreated"
	if g.Context != nil && g.Context.EnumRegistry != nil {
		if enumName, ok := g.Context.EnumRegistry[constructorName]; ok {
			return enumName + constructorName
		}
	}

	// Fallback: return as-is
	// This handles cases where the pattern already has the correct format
	return constructorName
}

// isTupleVariant determines if a type name represents a tuple variant.
// Tuple variants use positional fields (Value, Value0, Value1, ...).
// Struct variants use named fields matching the struct definition.
//
// Currently handles:
//   - Result variants: ResultOk, ResultErr (tuple)
//   - Option variants: OptionSome, OptionNone (tuple)
//   - Custom enums: assumed to be struct variants
//
// TODO: This should lookup variant metadata from a type registry
// instead of hard-coding known types.
func (g *MatchCodeGen) isTupleVariant(typeName string) bool {
	// Result and Option built-in types use tuple variants
	switch typeName {
	case "ResultOk", "ResultErr", "OptionSome", "OptionNone":
		return true
	}

	// Custom enums currently assumed to be struct variants
	// This will be fixed when we have a proper type registry (see action items #5)
	return false
}

// generateHumanLikeAssignment generates code for assignment context (x := match ...).
// Produces: var x TYPE; switch { case ...: x = value }
// Falls back to IIFE if type cannot be inferred.
func (g *MatchCodeGen) generateHumanLikeAssignment() ast.CodeGenResult {
	varName := g.Context.VarName
	varType := g.Context.VarType

	if varType == "" {
		// IIFE fallback when type cannot be inferred
		g.generateMatchIIFE()
		return g.Result()
	}

	// Generate var declaration
	g.Write(fmt.Sprintf("var %s %s\n", varName, varType))

	// Generate switch with assignments
	g.generateMatchSwitchWithAssignment(varName)

	// Get result and move output to StatementOutput
	result := g.Result()
	result.StatementOutput = result.Output // Move to statement output
	result.Output = nil                    // Clear expression output

	return result
}

// generateHoistedMatch generates code for argument context (fn(match ...)).
// Produces: var tmp TYPE; switch { case ...: tmp = value }; fn(tmp)
// Returns temp var name as expression replacement.
// Falls back to IIFE if type cannot be inferred.
func (g *MatchCodeGen) generateHoistedMatch() ast.CodeGenResult {
	tmpVar := g.TempVar("result")
	varType := g.Context.VarType

	if varType == "" {
		// IIFE fallback when type cannot be inferred
		g.generateMatchIIFE()
		return g.Result()
	}

	// Generate hoisted code: var tmp TYPE; switch { case ...: tmp = value }
	hoistedBuf := &bytes.Buffer{}
	hoistedBuf.WriteString(fmt.Sprintf("var %s %s\n", tmpVar, varType))

	// Save current buffer and switch to hoistedBuf
	origBuf := g.Buf
	g.Buf = *hoistedBuf

	// Generate switch with assignments to tmpVar
	g.generateMatchSwitchWithAssignment(tmpVar)

	// Get the hoisted code from the buffer
	hoistedCode := g.Buf.Bytes()

	// Restore original buffer
	g.Buf = origBuf

	// Build result
	result := g.Result()
	result.HoistedCode = hoistedCode // Hoisted code goes BEFORE the statement
	result.Output = []byte(tmpVar)   // Replace match expression with tmpVar

	return result
}

// generateMatchSwitchWithAssignment generates switch statement that assigns to varName.
// Example: switch x.(type) { case A: varName = "a"; case B: varName = "b" }
// Includes default panic with scrutinee value.
func (g *MatchCodeGen) generateMatchSwitchWithAssignment(varName string) {
	// Generate scrutinee
	scrutineeResult := GenerateExpr(g.Match.Scrutinee)
	scrutineeCode := string(scrutineeResult.Output)

	// CRITICAL FIX: Always bind match value to temp var to prevent double evaluation
	// This prevents side-effect functions (e.g., popQueue()) from executing twice:
	// once in switch condition and once in panic clause
	scrutineeTempVar := g.SharedTempVar("val")
	g.Write(fmt.Sprintf("%s := %s\n", scrutineeTempVar, scrutineeCode))

	// Check if we need type assertion (for constructor patterns)
	needsTypeSwitch := g.hasConstructorPatterns()

	// Check if we have any bindings (to decide on temp var usage)
	hasBindings := g.hasBindings()

	if needsTypeSwitch {
		// Generate type switch: switch v := scrutinee.(type) or switch scrutinee.(type)
		if hasBindings {
			// Need temp var for bindings
			tempVar := g.SharedTempVar("v")
			g.scrutineeTempVar = tempVar // Store for generateArmBodyWithAssignment
			g.Write("switch " + tempVar + " := ")
			g.Write(scrutineeTempVar)
			g.Write(".(type) {\n")
		} else {
			// No bindings - just use scrutinee.(type) without variable
			g.Write("switch ")
			g.Write(scrutineeTempVar)
			g.Write(".(type) {\n")
		}
	} else {
		// Generate value switch: switch scrutinee {
		g.Write("switch ")
		g.Write(scrutineeTempVar)
		g.Write(" {\n")
	}

	// Group arms by pattern type to avoid duplicate case clauses
	// This handles guarded patterns correctly
	g.generateGroupedCasesWithAssignment(varName, needsTypeSwitch)

	// NO DEFAULT PANIC:
	// - Expression matches: verified exhaustive by checkExhaustiveness()
	// - Statement matches: silent fall-through per user requirement

	g.WriteByte('}')
}

// generateMatchArmWithAssignment generates a case clause with assignment to varName.
// Handles pattern matching, guards, and assignment generation.
func (g *MatchCodeGen) generateMatchArmWithAssignment(arm *ast.MatchArm, varName string, typeSwitch bool) {
	// Generate case clause based on pattern type
	switch pattern := arm.Pattern.(type) {
	case *ast.ConstructorPattern:
		g.generateConstructorCaseWithAssignment(pattern, arm, varName, typeSwitch)
	case *ast.LiteralPattern:
		g.generateLiteralCaseWithAssignment(pattern, arm, varName)
	case *ast.WildcardPattern:
		g.generateWildcardCaseWithAssignment(arm, varName)
	case *ast.VariablePattern:
		g.generateVariableCaseWithAssignment(pattern, arm, varName)
	case *ast.TuplePattern:
		g.generateTupleCaseWithAssignment(pattern, arm, varName)
	default:
		// Unknown pattern type - generate default case
		g.Write("default:\n")
		g.generateArmBodyWithAssignment(arm, varName, nil)
	}
}

// generateConstructorCaseWithAssignment generates case for constructor patterns with assignment.
func (g *MatchCodeGen) generateConstructorCaseWithAssignment(pattern *ast.ConstructorPattern, arm *ast.MatchArm, varName string, typeSwitch bool) {
	// Convert constructor name to full type name
	typeName := g.constructorToTypeName(pattern.Name)

	g.Write("case " + typeName + ":\n")

	// Extract bindings from constructor parameters
	var bindings []Binding
	numParams := len(pattern.Params)
	isTupleVariant := g.isTupleVariant(typeName)
	for i, param := range pattern.Params {
		bindings = append(bindings, g.extractBindings(param, i, numParams, isTupleVariant)...)
	}

	g.generateArmBodyWithAssignment(arm, varName, bindings)
}

// generateLiteralCaseWithAssignment generates case for literal patterns with assignment.
func (g *MatchCodeGen) generateLiteralCaseWithAssignment(pattern *ast.LiteralPattern, arm *ast.MatchArm, varName string) {
	g.Write("case " + pattern.Value + ":\n")
	g.generateArmBodyWithAssignment(arm, varName, nil)
}

// generateWildcardCaseWithAssignment generates default case for wildcard pattern with assignment.
func (g *MatchCodeGen) generateWildcardCaseWithAssignment(arm *ast.MatchArm, varName string) {
	g.Write("default:\n")
	g.generateArmBodyWithAssignment(arm, varName, nil)
}

// generateVariableCaseWithAssignment generates default case with binding for variable pattern with assignment.
func (g *MatchCodeGen) generateVariableCaseWithAssignment(pattern *ast.VariablePattern, arm *ast.MatchArm, varName string) {
	g.Write("default:\n")

	// Bind variable to scrutinee value
	bindings := []Binding{{
		Name:      pattern.Name,
		FieldPath: "", // Bind to scrutinee itself
	}}
	g.generateArmBodyWithAssignment(arm, varName, bindings)
}

// generateTupleCaseWithAssignment generates case for tuple patterns with assignment.
func (g *MatchCodeGen) generateTupleCaseWithAssignment(pattern *ast.TuplePattern, arm *ast.MatchArm, varName string) {
	// Tuple patterns need special handling - not implemented yet
	// For now, generate default case
	g.Write("default:\n")
	g.generateArmBodyWithAssignment(arm, varName, nil)
}

// generateArmBodyWithAssignment generates the body of a match arm with assignment to varName.
// Handles variable bindings, guards, and assignment generation.
func (g *MatchCodeGen) generateArmBodyWithAssignment(arm *ast.MatchArm, varName string, bindings []Binding) {
	// Generate variable bindings
	for _, binding := range bindings {
		bindingCode := binding.Name + " := "

		// If binding has a field path, use type assertion variable
		if binding.FieldPath != "" {
			// Use scrutineeTempVar (e.g., "v") set in generateMatchSwitchWithAssignment
			tempVar := g.scrutineeTempVar
			if tempVar == "" {
				tempVar = "v" // Fallback if not set
			}
			bindingCode += tempVar + "." + binding.FieldPath
		} else {
			// Bind to scrutinee temp var (for variable patterns in default case)
			// Use scrutinee temp var to avoid double evaluation
			tempVar := g.scrutineeTempVar
			if tempVar == "" {
				// Fallback: generate expression (shouldn't happen in normal flow)
				scrutineeResult := GenerateExpr(g.Match.Scrutinee)
				bindingCode += string(scrutineeResult.Output)
			} else {
				bindingCode += tempVar
			}
		}

		bindingCode += "\n"
		g.Write(bindingCode)
	}

	// Generate guard check if present
	if arm.Guard != nil {
		guardResult := GenerateExpr(arm.Guard)
		g.Write("if " + string(guardResult.Output) + " {\n")
	}

	// Generate body with assignment
	bodyResult := GenerateExpr(arm.Body)
	g.Write(fmt.Sprintf("\t%s = %s\n", varName, string(bodyResult.Output)))

	// Close guard if present
	if arm.Guard != nil {
		g.Write("}\n")
		// NO PANIC: Guarded patterns are handled by exhaustiveness checking
		// The guard failure simply means this case doesn't match, so try next case
	}
}

// checkExhaustiveness validates exhaustiveness for expression matches.
// Returns error result if non-exhaustive; otherwise returns empty result.
func (g *MatchCodeGen) checkExhaustiveness() ast.CodeGenResult {
	// Only check if we have a registry (might be nil in tests or simple cases)
	if g.Context == nil || g.Context.EnumRegistry == nil {
		return ast.CodeGenResult{}
	}

	// Create exhaustiveness checker with registry
	registry := typechecker.NewEnumRegistry()

	// Populate registry from context
	// GenContext.EnumRegistry is map[string]string (variant -> enum)
	// We need to convert to typechecker.EnumRegistry format
	enumVariants := make(map[string][]typechecker.VariantInfo)
	for variantName, enumName := range g.Context.EnumRegistry {
		vi := typechecker.VariantInfo{
			Name:     variantName,
			FullName: enumName + variantName,
			// Note: Fields/FieldTypes would need metadata from enum declaration
			// For exhaustiveness checking, we only need variant names
		}
		enumVariants[enumName] = append(enumVariants[enumName], vi)
	}

	// Register all enums in the registry
	for enumName, variants := range enumVariants {
		if err := registry.RegisterEnum(enumName, variants); err != nil {
			// Variant name collision detected
			return ast.CodeGenResult{
				Error: &ast.CodeGenError{
					Position: int(g.Match.Match),
					Message:  err.Error(),
					Hint:     "rename variants to avoid conflicts",
				},
			}
		}
	}

	// Pass type checker from context (if available)
	checker := typechecker.NewExhaustivenessChecker(registry, nil)

	// Check exhaustiveness
	result := checker.Check(g.Match, g.Match.IsExpr)

	// If non-exhaustive expression match, return error
	if !result.IsExhaustive && g.Match.IsExpr {
		return g.generateExhaustivenessError(result)
	}

	return ast.CodeGenResult{}
}

// generateExhaustivenessError creates a compile-time error for non-exhaustive match
func (g *MatchCodeGen) generateExhaustivenessError(result *typechecker.ExhaustivenessResult) ast.CodeGenResult {
	var msg string
	if len(result.MissingVariants) > 0 {
		msg = fmt.Sprintf(
			"non-exhaustive match on %s: missing variants: %s",
			result.EnumName,
			formatVariantList(result.MissingVariants),
		)
	} else {
		msg = "non-exhaustive match expression"
	}

	hint := "add missing variants or use a wildcard pattern (_)"

	// Return structured error that halts transpilation during dingo build
	// This produces a clear error at .dingo file location, not cryptic Go compiler error
	return ast.CodeGenResult{
		Error: &ast.CodeGenError{
			Position: int(result.Position),
			Message:  msg,
			Hint:     hint,
		},
	}
}

// formatVariantList formats variant names for error messages
func formatVariantList(variants []string) string {
	if len(variants) == 0 {
		return ""
	}
	if len(variants) == 1 {
		return variants[0]
	}
	if len(variants) == 2 {
		return variants[0] + ", " + variants[1]
	}
	// More than 2: show first 2 and count
	return fmt.Sprintf("%s, %s (and %d more)", variants[0], variants[1], len(variants)-2)
}

// OptionResultInfo contains type information for Option/Result matching
type OptionResultInfo struct {
	Kind string // "option" or "result"
}

// detectOptionResult detects if match patterns are for Option or Result types.
// Primary detection uses go/types to check scrutinee type.
// Falls back to constructor pattern detection (Some/None, Ok/Err) when type info unavailable.
// Returns nil if this is a regular enum match.
func (g *MatchCodeGen) detectOptionResult() *OptionResultInfo {
	if g.Match == nil || len(g.Match.Arms) == 0 {
		return nil
	}

	// PRIMARY: Try go/types detection first (most accurate)
	// NOTE: This would require scrutinee type info from an earlier analysis pass.
	// For now, we rely on the constructor pattern fallback below.
	// TODO: Add type annotation pass that stores scrutinee types in GenContext
	// so we can distinguish between dgo.Option[T] and user enums with Some/None constructors.
	if g.Context != nil && g.Context.TypeChecker != nil {
		// Future: Look up pre-computed type information from earlier pass
		// Example: if typeInfo, ok := g.Context.ScrutineeTypes[g.Match.Scrutinee]; ok { ... }
	}

	// FALLBACK: Check constructor patterns in arms (for when type info unavailable)
	hasSome := false
	hasNone := false
	hasOk := false
	hasErr := false

	for _, arm := range g.Match.Arms {
		if cp, ok := arm.Pattern.(*ast.ConstructorPattern); ok {
			switch cp.Name {
			case "Some":
				hasSome = true
			case "None":
				hasNone = true
			case "Ok":
				hasOk = true
			case "Err":
				hasErr = true
			}
		}
	}

	// Detect Option pattern: has Some or None
	if hasSome || hasNone {
		return &OptionResultInfo{Kind: "option"}
	}

	// Detect Result pattern: has Ok or Err
	if hasOk || hasErr {
		return &OptionResultInfo{Kind: "result"}
	}

	return nil
}

// generateOptionResultMatch generates if/else chain for Option/Result matching.
// This replaces type switch with method-based checks.
func (g *MatchCodeGen) generateOptionResultMatch(info *OptionResultInfo) ast.CodeGenResult {
	if info.Kind == "option" {
		return g.generateOptionMatch()
	} else if info.Kind == "result" {
		return g.generateResultMatch()
	}
	return ast.CodeGenResult{}
}

// generateOptionMatch generates if/else chain for Option matching.
// Output for match opt { Some(v) => expr1, None => expr2 }:
//
//	if opt.IsSome() {
//	    v := opt.MustSome()
//	    return expr1
//	} else {
//	    return expr2
//	}
func (g *MatchCodeGen) generateOptionMatch() ast.CodeGenResult {
	scrutineeResult := GenerateExpr(g.Match.Scrutinee)
	scrutineeCode := string(scrutineeResult.Output)

	// Store scrutinee in temp var to avoid double evaluation
	tmpVar := g.SharedTempVar("opt")
	g.Write(fmt.Sprintf("%s := %s\n", tmpVar, scrutineeCode))

	// Find Some and None arms
	var someArms []*ast.MatchArm
	var noneArm *ast.MatchArm

	for _, arm := range g.Match.Arms {
		switch p := arm.Pattern.(type) {
		case *ast.ConstructorPattern:
			if p.Name == "Some" {
				someArms = append(someArms, arm)
			} else if p.Name == "None" {
				noneArm = arm
			}
		case *ast.WildcardPattern:
			if noneArm == nil {
				noneArm = arm // _ can serve as None fallback
			}
		}
	}

	// Generate if IsSome() check
	g.Write(fmt.Sprintf("if %s.IsSome() {\n", tmpVar))

	if len(someArms) > 0 {
		// Extract binding from first Some arm
		firstSome := someArms[0]
		var someBinding string
		if cp, ok := firstSome.Pattern.(*ast.ConstructorPattern); ok && len(cp.Params) > 0 {
			if vp, ok := cp.Params[0].(*ast.VariablePattern); ok {
				someBinding = vp.Name
			}
		}

		// Binding: v := opt.MustSome()
		if someBinding != "" {
			g.Write(fmt.Sprintf("\t%s := %s.MustSome()\n", someBinding, tmpVar))
		}

		// Handle multiple Some arms (with guards)
		g.generateOptionSomeArms(someArms)
	} else {
		// Non-exhaustive match: missing Some arm
		if g.Match.IsExpr {
			// For expression context, panic with helpful message
			g.Write("\tpanic(\"non-exhaustive match on Option: missing Some arm\")\n")
		}
		// For statement context, empty if block is valid (no-op on Some)
	}

	g.Write("} else {\n")

	if noneArm != nil {
		bodyResult := GenerateExpr(noneArm.Body)
		_, isReturnExpr := noneArm.Body.(*ast.ReturnExpr)
		if g.Match.IsExpr && !isReturnExpr {
			g.Write(fmt.Sprintf("\treturn %s\n", string(bodyResult.Output)))
		} else {
			g.Write(fmt.Sprintf("\t%s\n", string(bodyResult.Output)))
		}
	} else {
		// Non-exhaustive match: missing None arm
		if g.Match.IsExpr {
			// For expression context, panic with helpful message
			g.Write("\tpanic(\"non-exhaustive match on Option: missing None arm\")\n")
		}
		// For statement context, empty else block is valid (no-op on None)
	}

	g.Write("}\n")

	// For expression matches, wrap in IIFE if not in return context
	result := g.Result()
	if g.Match.IsExpr && g.Context != nil && g.Context.Context == ast.ContextReturn {
		// Return context: use StatementOutput
		result.StatementOutput = result.Output
		result.Output = nil
	} else if g.Match.IsExpr {
		// Other contexts: wrap in IIFE
		result = g.wrapInIIFE(result)
	}

	return result
}

// generateOptionSomeArms generates code for one or more Some arms (handles guards).
func (g *MatchCodeGen) generateOptionSomeArms(arms []*ast.MatchArm) {
	for i, arm := range arms {
		hasGuard := arm.Guard != nil
		isFirst := i == 0
		isLast := i == len(arms)-1

		if hasGuard {
			guardResult := GenerateExpr(arm.Guard)
			if isFirst {
				g.Write(fmt.Sprintf("\tif %s {\n", string(guardResult.Output)))
			} else {
				g.Write(fmt.Sprintf("\t} else if %s {\n", string(guardResult.Output)))
			}
		} else if !isFirst {
			// Unguarded arm after guarded arms
			g.Write("\t} else {\n")
		}

		// Generate body
		bodyResult := GenerateExpr(arm.Body)
		indent := "\t"
		if hasGuard || (!isFirst && arms[0].Guard != nil) {
			indent = "\t\t"
		}
		_, isReturnExpr := arm.Body.(*ast.ReturnExpr)
		if g.Match.IsExpr && !isReturnExpr {
			g.Write(fmt.Sprintf("%sreturn %s\n", indent, string(bodyResult.Output)))
		} else {
			g.Write(fmt.Sprintf("%s%s\n", indent, string(bodyResult.Output)))
		}

		// Close guard if/else chain
		if isLast && (hasGuard || (!isFirst && arms[0].Guard != nil)) {
			g.Write("\t}\n")
		}
	}
}

// generateResultMatch generates if/else chain for Result matching.
// Output for match res { Ok(v) => expr1, Err(e) => expr2 }:
//
//	if res.IsOk() {
//	    v := res.MustOk()
//	    return expr1
//	} else {
//	    e := res.MustErr()
//	    return expr2
//	}
func (g *MatchCodeGen) generateResultMatch() ast.CodeGenResult {
	scrutineeResult := GenerateExpr(g.Match.Scrutinee)
	scrutineeCode := string(scrutineeResult.Output)

	// Store scrutinee in temp var to avoid double evaluation
	tmpVar := g.SharedTempVar("res")
	g.Write(fmt.Sprintf("%s := %s\n", tmpVar, scrutineeCode))

	// Find Ok and Err arms
	var okArms []*ast.MatchArm
	var errArms []*ast.MatchArm

	for _, arm := range g.Match.Arms {
		switch p := arm.Pattern.(type) {
		case *ast.ConstructorPattern:
			if p.Name == "Ok" {
				okArms = append(okArms, arm)
			} else if p.Name == "Err" {
				errArms = append(errArms, arm)
			}
		case *ast.WildcardPattern:
			// Wildcard can match either - put in Err as fallback
			if len(errArms) == 0 {
				errArms = append(errArms, arm)
			}
		}
	}

	// Generate if IsOk() check
	g.Write(fmt.Sprintf("if %s.IsOk() {\n", tmpVar))

	if len(okArms) > 0 {
		// Extract binding from first Ok arm
		firstOk := okArms[0]
		var okBinding string
		if cp, ok := firstOk.Pattern.(*ast.ConstructorPattern); ok && len(cp.Params) > 0 {
			if vp, ok := cp.Params[0].(*ast.VariablePattern); ok {
				okBinding = vp.Name
			}
		}

		// Binding: v := res.MustOk()
		if okBinding != "" {
			g.Write(fmt.Sprintf("\t%s := %s.MustOk()\n", okBinding, tmpVar))
		}

		// Handle multiple Ok arms (with guards)
		g.generateResultOkArms(okArms)
	} else {
		// Non-exhaustive match: missing Ok arm
		if g.Match.IsExpr {
			// For expression context, panic with helpful message
			g.Write("\tpanic(\"non-exhaustive match on Result: missing Ok arm\")\n")
		}
		// For statement context, empty if block is valid (no-op on Ok)
	}

	g.Write("} else {\n")

	if len(errArms) > 0 {
		// Extract binding from first Err arm
		firstErr := errArms[0]
		var errBinding string
		if cp, ok := firstErr.Pattern.(*ast.ConstructorPattern); ok && len(cp.Params) > 0 {
			if vp, ok := cp.Params[0].(*ast.VariablePattern); ok {
				errBinding = vp.Name
				// Only bind if variable is used (not '_')
				if errBinding == "_" {
					errBinding = ""
				}
			}
		}

		// Binding: e := res.MustErr()
		// Only generate if binding is non-empty and used
		if errBinding != "" {
			g.Write(fmt.Sprintf("\t%s := %s.MustErr()\n", errBinding, tmpVar))
		}

		// Handle multiple Err arms (with guards)
		g.generateResultErrArms(errArms)
	} else {
		// Non-exhaustive match: missing Err arm
		if g.Match.IsExpr {
			// For expression context, panic with helpful message
			g.Write("\tpanic(\"non-exhaustive match on Result: missing Err arm\")\n")
		}
		// For statement context, empty else block is valid (no-op on Err)
	}

	g.Write("}\n")

	// For expression matches, wrap in IIFE if not in return context
	result := g.Result()
	if g.Match.IsExpr && g.Context != nil && g.Context.Context == ast.ContextReturn {
		// Return context: use StatementOutput
		result.StatementOutput = result.Output
		result.Output = nil
	} else if g.Match.IsExpr {
		// Other contexts: wrap in IIFE
		result = g.wrapInIIFE(result)
	}

	return result
}

// generateResultOkArms generates code for one or more Ok arms (handles guards).
func (g *MatchCodeGen) generateResultOkArms(arms []*ast.MatchArm) {
	for i, arm := range arms {
		hasGuard := arm.Guard != nil
		isFirst := i == 0
		isLast := i == len(arms)-1

		if hasGuard {
			guardResult := GenerateExpr(arm.Guard)
			if isFirst {
				g.Write(fmt.Sprintf("\tif %s {\n", string(guardResult.Output)))
			} else {
				g.Write(fmt.Sprintf("\t} else if %s {\n", string(guardResult.Output)))
			}
		} else if !isFirst {
			// Unguarded arm after guarded arms
			g.Write("\t} else {\n")
		}

		// Generate body
		bodyResult := GenerateExpr(arm.Body)
		indent := "\t"
		if hasGuard || (!isFirst && arms[0].Guard != nil) {
			indent = "\t\t"
		}
		_, isReturnExpr := arm.Body.(*ast.ReturnExpr)
		if g.Match.IsExpr && !isReturnExpr {
			g.Write(fmt.Sprintf("%sreturn %s\n", indent, string(bodyResult.Output)))
		} else {
			g.Write(fmt.Sprintf("%s%s\n", indent, string(bodyResult.Output)))
		}

		// Close guard if/else chain
		if isLast && (hasGuard || (!isFirst && arms[0].Guard != nil)) {
			g.Write("\t}\n")
		}
	}
}

// generateResultErrArms generates code for one or more Err arms (handles guards).
func (g *MatchCodeGen) generateResultErrArms(arms []*ast.MatchArm) {
	for i, arm := range arms {
		hasGuard := arm.Guard != nil
		isFirst := i == 0
		isLast := i == len(arms)-1

		if hasGuard {
			guardResult := GenerateExpr(arm.Guard)
			if isFirst {
				g.Write(fmt.Sprintf("\tif %s {\n", string(guardResult.Output)))
			} else {
				g.Write(fmt.Sprintf("\t} else if %s {\n", string(guardResult.Output)))
			}
		} else if !isFirst {
			// Unguarded arm after guarded arms
			g.Write("\t} else {\n")
		}

		// Generate body
		bodyResult := GenerateExpr(arm.Body)
		indent := "\t"
		if hasGuard || (!isFirst && arms[0].Guard != nil) {
			indent = "\t\t"
		}
		_, isReturnExpr := arm.Body.(*ast.ReturnExpr)
		if g.Match.IsExpr && !isReturnExpr {
			g.Write(fmt.Sprintf("%sreturn %s\n", indent, string(bodyResult.Output)))
		} else {
			g.Write(fmt.Sprintf("%s%s\n", indent, string(bodyResult.Output)))
		}

		// Close guard if/else chain
		if isLast && (hasGuard || (!isFirst && arms[0].Guard != nil)) {
			g.Write("\t}\n")
		}
	}
}

// wrapInIIFE wraps generated code in an IIFE for expression context.
func (g *MatchCodeGen) wrapInIIFE(inner ast.CodeGenResult) ast.CodeGenResult {
	var buf bytes.Buffer
	buf.WriteString("func() interface{} {\n")
	buf.Write(inner.Output)
	buf.WriteString("}()")

	return ast.CodeGenResult{
		Output: buf.Bytes(),
	}
}

// =============================================================================
// Value Enum Match Support (Phase 3)
// =============================================================================

// ValueEnumMatchInfo contains information about a value enum match.
// Used by detectValueEnum to return enum metadata for code generation.
type ValueEnumMatchInfo struct {
	EnumName  string   // "Status"
	Variants  []string // ["Pending", "Active", "Closed"]
	UsePrefix bool     // true -> StatusPending, false -> Pending
}

// detectValueEnum checks if all patterns in the match are value enum variants.
// Returns info if all patterns belong to a single value enum, nil otherwise.
//
// Detection strategy:
// 1. All patterns must be ConstructorPatterns with no params (value enums have no fields)
// 2. All pattern names must be found in ValueEnumReg
// 3. All patterns must belong to the same enum
func (g *MatchCodeGen) detectValueEnum() *ValueEnumMatchInfo {
	if g.Match == nil || len(g.Match.Arms) == 0 {
		return nil
	}

	// Need registry to detect value enums
	if g.ValueEnumReg == nil {
		return nil
	}

	var enumInfo *ast.ValueEnumInfo

	for _, arm := range g.Match.Arms {
		// Wildcards and variables are allowed (they become default case)
		if _, ok := arm.Pattern.(*ast.WildcardPattern); ok {
			continue
		}
		if _, ok := arm.Pattern.(*ast.VariablePattern); ok {
			continue
		}

		// For value enums, patterns must be simple ConstructorPatterns with NO params
		cp, ok := arm.Pattern.(*ast.ConstructorPattern)
		if !ok {
			return nil // Not a constructor pattern - not a value enum match
		}

		// Value enums don't have fields, so params must be empty
		if len(cp.Params) > 0 {
			return nil // Has params - this is a sum type match
		}

		// Look up in value enum registry
		info := g.ValueEnumReg.LookupVariant(cp.Name)
		if info == nil {
			return nil // Not a value enum variant
		}

		// All patterns must be from the same enum
		if enumInfo == nil {
			enumInfo = info
		} else if enumInfo.EnumName != info.EnumName {
			return nil // Mixed enums - not supported
		}

	}

	if enumInfo == nil {
		return nil // No enum patterns found
	}

	return &ValueEnumMatchInfo{
		EnumName:  enumInfo.EnumName,
		Variants:  enumInfo.Variants,
		UsePrefix: enumInfo.UsePrefix,
	}
}

// generateValueEnumMatch generates a simple switch statement for value enum matching.
// Value enums are typed constants, so we use value switch instead of type switch.
//
// Example output for: match status { Pending => "waiting", Active => "running" }
//
//	switch status {
//	case StatusPending:
//	    return "waiting"
//	case StatusActive:
//	    return "running"
//	}
func (g *MatchCodeGen) generateValueEnumMatch(info *ValueEnumMatchInfo) ast.CodeGenResult {
	// Generate scrutinee
	scrutineeResult := GenerateExpr(g.Match.Scrutinee)
	scrutineeCode := string(scrutineeResult.Output)

	// Store scrutinee in temp var to avoid double evaluation
	g.scrutineeTempVar = g.SharedTempVar("val")
	g.Write(fmt.Sprintf("%s := %s\n", g.scrutineeTempVar, scrutineeCode))

	// Generate value switch (NOT type switch)
	g.Write("switch ")
	g.Write(g.scrutineeTempVar)
	g.Write(" {\n")

	// Generate case for each arm
	for _, arm := range g.Match.Arms {
		g.generateValueEnumArm(arm, info)
	}

	g.WriteByte('}')

	// For expression matches in return context, add unreachable panic
	if g.Match.IsExpr && g.Context != nil && g.Context.Context == ast.ContextReturn {
		g.Write("\npanic(\"unreachable: exhaustive match\")")
	}

	// Build result
	result := g.Result()
	if g.Match.IsExpr && g.Context != nil && g.Context.Context == ast.ContextReturn {
		// Return context: use StatementOutput
		result.StatementOutput = result.Output
		result.Output = nil
	}

	return result
}

// generateValueEnumArm generates a single case clause for value enum match arm.
func (g *MatchCodeGen) generateValueEnumArm(arm *ast.MatchArm, info *ValueEnumMatchInfo) {
	switch p := arm.Pattern.(type) {
	case *ast.ConstructorPattern:
		// Value enum variant - generate: case ConstName:
		constName := g.valueEnumConstName(p.Name, info)
		g.Write("case ")
		g.Write(constName)
		g.Write(":\n")

	case *ast.WildcardPattern:
		// Wildcard - generate: default:
		g.Write("default:\n")

	case *ast.VariablePattern:
		// Variable pattern - generate: default: with binding
		g.Write("default:\n")
		// Generate binding to temp var
		g.Write("\t")
		g.Write(p.Name)
		g.Write(" := ")
		g.Write(g.scrutineeTempVar)
		if g.scrutineeTempVar == "" {
			// Fallback - shouldn't happen normally
			scrutineeResult := GenerateExpr(g.Match.Scrutinee)
			g.Buf.Write(scrutineeResult.Output)
		}
		g.Write("\n")
	}

	// Generate guard if present
	if arm.Guard != nil {
		guardResult := GenerateExpr(arm.Guard)
		g.Write("\tif ")
		g.Buf.Write(guardResult.Output)
		g.Write(" {\n")
		g.generateValueEnumBody(arm)
		g.Write("\t}\n")
	} else {
		g.generateValueEnumBody(arm)
	}
}

// generateValueEnumBody generates the body of a value enum match arm.
func (g *MatchCodeGen) generateValueEnumBody(arm *ast.MatchArm) {
	bodyResult := GenerateExpr(arm.Body)
	_, isReturnExpr := arm.Body.(*ast.ReturnExpr)

	if g.Match.IsExpr && !isReturnExpr {
		g.Write("\treturn ")
		g.Buf.Write(bodyResult.Output)
		g.WriteByte('\n')
	} else {
		g.WriteByte('\t')
		g.Buf.Write(bodyResult.Output)
		g.WriteByte('\n')
	}
}

// valueEnumConstName returns the Go constant name for a value enum variant.
// Respects the UsePrefix setting from the enum declaration.
//
// Examples:
//   - With prefix (default): Pending -> StatusPending
//   - Without prefix (@prefix(false)): Pending -> Pending
func (g *MatchCodeGen) valueEnumConstName(variantName string, info *ValueEnumMatchInfo) string {
	if info.UsePrefix {
		return info.EnumName + variantName
	}
	return variantName
}

// checkValueEnumExhaustiveness verifies all enum variants are covered.
// Returns an error result if non-exhaustive.
//
// For value enums:
//   - Get all variants from ValueEnumMatchInfo
//   - Compare with patterns in match arms
//   - If wildcard (_) present, exhaustive
//   - If missing variants without wildcard, error
func (g *MatchCodeGen) checkValueEnumExhaustiveness(info *ValueEnumMatchInfo) ast.CodeGenResult {
	// Get all expected variants
	allVariants := make(map[string]bool)
	for _, v := range info.Variants {
		allVariants[v] = false // false = not covered
	}

	hasWildcard := false

	// Mark covered variants
	for _, arm := range g.Match.Arms {
		switch p := arm.Pattern.(type) {
		case *ast.ConstructorPattern:
			// Mark as covered (both bare and prefixed names)
			// Pattern must match either:
			// 1. Bare variant name exactly (e.g., "Pending" matches "Pending")
			// 2. Prefixed variant name exactly (e.g., "StatusPending" matches "Pending" when enum is "Status")
			if _, exists := allVariants[p.Name]; exists {
				allVariants[p.Name] = true
			}
			// Check if pattern was written with exact prefix (enumName + variantName)
			// Use exact match to avoid false positives like StatusStatusActive matching StatusActive
			for variantName := range allVariants {
				if p.Name == info.EnumName+variantName {
					allVariants[variantName] = true
					break
				}
			}

		case *ast.WildcardPattern, *ast.VariablePattern:
			hasWildcard = true
		}
	}

	// If wildcard present, all cases are covered
	if hasWildcard {
		return ast.CodeGenResult{}
	}

	// Check for missing variants
	var missing []string
	for variant, covered := range allVariants {
		if !covered {
			missing = append(missing, variant)
		}
	}
	// Sort for deterministic error messages (map iteration is random)
	sort.Strings(missing)

	if len(missing) > 0 {
		msg := fmt.Sprintf(
			"non-exhaustive match on %s: missing variants: %s",
			info.EnumName,
			formatVariantList(missing),
		)
		hint := "add missing variants or use a wildcard pattern (_)"

		return ast.CodeGenResult{
			Error: &ast.CodeGenError{
				Position: int(g.Match.Match),
				Message:  msg,
				Hint:     hint,
			},
		}
	}

	return ast.CodeGenResult{}
}
