package ast

import (
	"bytes"
	"fmt"
)

// ValueEnumCodeGen generates Go code from ValueEnumDecl AST nodes.
// Produces type declaration + const block with iota or explicit values.
type ValueEnumCodeGen struct {
	buf bytes.Buffer
}

// NewValueEnumCodeGen creates a new value enum code generator.
func NewValueEnumCodeGen() *ValueEnumCodeGen {
	return &ValueEnumCodeGen{}
}

// Generate produces Go code for a ValueEnumDecl.
// Returns the generated Go code as bytes.
//
// Example output for: enum Status: int { Pending, Active, Closed }
//
//	type Status int
//
//	const (
//		StatusPending Status = iota
//		StatusActive
//		StatusClosed
//	)
//
// Example output for: enum contextKey: string { UserID = "user_id" }
//
//	type contextKey string
//
//	const (
//		contextKeyUserID contextKey = "user_id"
//	)
//
// GenerateResult wraps the output with optional error
type GenerateResult struct {
	Output []byte
	Error  error
}

// Generate produces Go code for a ValueEnumDecl.
// Returns the generated Go code as bytes.
// Returns error if the enum declaration is invalid (e.g., empty variants).
func (g *ValueEnumCodeGen) Generate(decl *ValueEnumDecl, filename string, line, col int) []byte {
	result := g.GenerateWithError(decl, filename, line, col)
	return result.Output
}

// GenerateWithError produces Go code for a ValueEnumDecl with error handling.
// Returns error if the enum declaration is invalid (e.g., empty variants).
func (g *ValueEnumCodeGen) GenerateWithError(decl *ValueEnumDecl, filename string, line, col int) GenerateResult {
	g.buf.Reset()

	// Validate: empty enums are not allowed
	if len(decl.Variants) == 0 {
		return GenerateResult{
			Error: fmt.Errorf("empty enum %q: value enums must have at least one variant", decl.Name.Name),
		}
	}

	// Emit //line directive at start
	if filename != "" && line > 0 && col > 0 {
		directive := FormatLineDirective(filename, line, col)
		g.buf.WriteString(directive)
	}

	enumName := decl.Name.Name
	baseType := decl.BaseType.Text

	// Check prefix setting from attributes
	usePrefix := g.shouldUsePrefix(decl)

	// 1. Generate type declaration
	g.buf.WriteString("type ")
	g.buf.WriteString(enumName)
	g.buf.WriteString(" ")
	g.buf.WriteString(baseType)
	g.buf.WriteString("\n\n")

	// 2. Generate const block
	g.buf.WriteString("const (\n")

	hasExplicitValues := g.hasAnyExplicitValue(decl.Variants)
	useIota := !hasExplicitValues && isIotaCompatibleType(baseType)

	for i, variant := range decl.Variants {
		// Const name (with or without prefix)
		constName := g.getConstName(enumName, variant.Name.Name, usePrefix)

		g.buf.WriteString("\t")
		g.buf.WriteString(constName)

		// Value
		if variant.Value != nil {
			// Explicit value: UserID contextKey = "user_id"
			g.buf.WriteString(" ")
			g.buf.WriteString(enumName)
			g.buf.WriteString(" = ")
			g.buf.WriteString(variant.Value.String())
		} else if useIota {
			if i == 0 {
				// First iota: Pending Status = iota
				g.buf.WriteString(" ")
				g.buf.WriteString(enumName)
				g.buf.WriteString(" = iota")
			}
			// Subsequent iota values: just the name (Go auto-increments with same type)
		} else {
			// Non-iota, non-explicit: shouldn't happen but handle gracefully
			g.buf.WriteString(" ")
			g.buf.WriteString(enumName)
		}

		g.buf.WriteString("\n")
	}

	g.buf.WriteString(")\n")

	return GenerateResult{Output: g.buf.Bytes()}
}

// shouldUsePrefix checks @prefix attribute
// Default: true (use prefix like StatusPending)
// With @prefix(false): no prefix (just Pending)
func (g *ValueEnumCodeGen) shouldUsePrefix(decl *ValueEnumDecl) bool {
	for _, attr := range decl.Attributes {
		if attr.Name.Name == "prefix" {
			if len(attr.Args) == 0 {
				// @prefix with no args - default to false (no prefix)
				return false
			}
			if len(attr.Args) > 0 {
				// Check first arg for boolean value
				if rawExpr, ok := attr.Args[0].(*RawExpr); ok {
					if rawExpr.Text == "false" {
						return false
					}
					if rawExpr.Text == "true" {
						return true
					}
				}
			}
		}
	}
	return true // Default: use prefix
}

// hasAnyExplicitValue checks if any variant has an explicit value
func (g *ValueEnumCodeGen) hasAnyExplicitValue(variants []*ValueEnumVariant) bool {
	for _, v := range variants {
		if v.Value != nil {
			return true
		}
	}
	return false
}

// getConstName generates const name with optional prefix
func (g *ValueEnumCodeGen) getConstName(enumName, variantName string, usePrefix bool) string {
	if usePrefix {
		return enumName + variantName
	}
	return variantName
}

// isIotaCompatibleType checks if type supports iota
func isIotaCompatibleType(baseType string) bool {
	switch baseType {
	case "int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64",
		"byte", "rune":
		return true
	default:
		return false
	}
}

// =============================================================================
// EnumRegistry: Unified registry for both sum types and value enums
// =============================================================================

// ValueEnumInfo stores metadata about a value enum
type ValueEnumInfo struct {
	EnumName    string   // "Status"
	Variants    []string // ["Pending", "Active", "Closed"] for exhaustiveness
	UsePrefix   bool     // true -> StatusPending, false -> Pending
	IsValueEnum bool     // Always true for this type (distinguishes from sum types)
}

// EnumRegistry stores all enum information (both sum types and value enums)
type EnumRegistry struct {
	// SumTypeVariants maps variant name to enum name for sum types
	// Example: "Ok" -> "Result"
	SumTypeVariants map[string]string

	// ValueEnumVariants maps variant name to ValueEnumInfo for value enums
	// Example: "Pending" -> &ValueEnumInfo{EnumName: "Status", ...}
	ValueEnumVariants map[string]*ValueEnumInfo

	// EnumToVariants maps enum name to all its variants (for exhaustiveness checking)
	// Example: "Status" -> ["Pending", "Active", "Closed"]
	EnumToVariants map[string][]string

	// Collisions tracks naming conflicts for error reporting
	Collisions []string
}

// NewEnumRegistry creates a new unified enum registry
func NewEnumRegistry() *EnumRegistry {
	return &EnumRegistry{
		SumTypeVariants:   make(map[string]string),
		ValueEnumVariants: make(map[string]*ValueEnumInfo),
		EnumToVariants:    make(map[string][]string),
	}
}

// RegisterSumTypeVariant adds a sum type variant to the registry
func (r *EnumRegistry) RegisterSumTypeVariant(variantName, enumName string) {
	// Check for collision with value enum
	if info := r.ValueEnumVariants[variantName]; info != nil {
		r.Collisions = append(r.Collisions,
			fmt.Sprintf("variant %q exists in both sum type %q and value enum %q",
				variantName, enumName, info.EnumName))
	}

	r.SumTypeVariants[variantName] = enumName
	r.EnumToVariants[enumName] = append(r.EnumToVariants[enumName], variantName)
}

// RegisterValueEnum adds a value enum and all its variants to the registry
func (r *EnumRegistry) RegisterValueEnum(enumName string, variants []string, usePrefix bool) {
	info := &ValueEnumInfo{
		EnumName:    enumName,
		Variants:    variants,
		UsePrefix:   usePrefix,
		IsValueEnum: true,
	}

	// Register each variant
	for _, variantName := range variants {
		// Check for collision with sum type
		if existing := r.SumTypeVariants[variantName]; existing != "" {
			r.Collisions = append(r.Collisions,
				fmt.Sprintf("variant %q exists in both sum type %q and value enum %q",
					variantName, existing, enumName))
		}

		// Store variant info (bare name)
		r.ValueEnumVariants[variantName] = info

		// Also register prefixed name if using prefix
		if usePrefix {
			prefixedName := enumName + variantName
			r.ValueEnumVariants[prefixedName] = info
		}
	}

	// Build bidirectional mapping for exhaustiveness
	r.EnumToVariants[enumName] = variants
}

// LookupVariant checks if a variant name belongs to a value enum
// Returns nil if not found or if it's a sum type variant
func (r *EnumRegistry) LookupVariant(name string) *ValueEnumInfo {
	return r.ValueEnumVariants[name]
}

// IsSumTypeVariant checks if a variant belongs to a sum type
func (r *EnumRegistry) IsSumTypeVariant(name string) (enumName string, ok bool) {
	enumName, ok = r.SumTypeVariants[name]
	return
}

// GetVariants returns all variants for an enum (works for both types)
func (r *EnumRegistry) GetVariants(enumName string) []string {
	return r.EnumToVariants[enumName]
}

// HasCollisions returns true if there are naming conflicts
func (r *EnumRegistry) HasCollisions() bool {
	return len(r.Collisions) > 0
}

// CollisionErrors returns all collision messages as formatted errors
func (r *EnumRegistry) CollisionErrors() []string {
	return r.Collisions
}

// FormatCollisions returns a formatted string of all collision errors
func (r *EnumRegistry) FormatCollisions() string {
	if !r.HasCollisions() {
		return ""
	}
	result := "enum registry collisions:\n"
	for _, collision := range r.Collisions {
		result += "  - " + collision + "\n"
	}
	return result
}

// ToLegacyMap converts to the legacy map[string]string format
// Used for backward compatibility with existing code
func (r *EnumRegistry) ToLegacyMap() map[string]string {
	result := make(map[string]string)

	// Add sum type variants
	for variant, enum := range r.SumTypeVariants {
		result[variant] = enum
	}

	// Add value enum variants (bare names only, not prefixed)
	for variant, info := range r.ValueEnumVariants {
		// Skip prefixed entries
		if len(variant) > len(info.EnumName) && variant[:len(info.EnumName)] == info.EnumName {
			continue
		}
		result[variant] = info.EnumName
	}

	return result
}

// =============================================================================
// Transform integration
// =============================================================================

// findAttributeStart looks backwards from enumStart to find any @prefix(...) attribute.
// Returns the position where the attribute starts, or enumStart if no attribute found.
func findAttributeStart(src []byte, enumStart int) int {
	// Look backwards from enumStart, skipping whitespace
	pos := enumStart - 1

	// Skip backwards through whitespace
	for pos >= 0 && (src[pos] == ' ' || src[pos] == '\t' || src[pos] == '\n' || src[pos] == '\r') {
		pos--
	}

	// Check if we're at ')' (end of attribute arguments)
	if pos >= 0 && src[pos] == ')' {
		// Find matching '('
		depth := 1
		pos--
		for pos >= 0 && depth > 0 {
			if src[pos] == ')' {
				depth++
			} else if src[pos] == '(' {
				depth--
			}
			pos--
		}

		// Skip whitespace between attribute name and '('
		for pos >= 0 && (src[pos] == ' ' || src[pos] == '\t') {
			pos--
		}

		// Now we should be at end of attribute name, find its start
		for pos >= 0 && ((src[pos] >= 'a' && src[pos] <= 'z') || (src[pos] >= 'A' && src[pos] <= 'Z') || (src[pos] >= '0' && src[pos] <= '9') || src[pos] == '_') {
			pos--
		}

		// Check for '@' symbol
		if pos >= 0 && src[pos] == '@' {
			return pos
		}
	}

	// Check if we're at end of an attribute name without parentheses (just @name)
	if pos >= 0 && ((src[pos] >= 'a' && src[pos] <= 'z') || (src[pos] >= 'A' && src[pos] <= 'Z') || src[pos] == '_') {
		endName := pos
		// Find start of attribute name
		for pos >= 0 && ((src[pos] >= 'a' && src[pos] <= 'z') || (src[pos] >= 'A' && src[pos] <= 'Z') || (src[pos] >= '0' && src[pos] <= '9') || src[pos] == '_') {
			pos--
		}

		// Check for '@' symbol
		if pos >= 0 && src[pos] == '@' {
			// Verify this is actually an attribute name we recognize
			attrName := string(src[pos+1 : endName+1])
			if attrName == "prefix" {
				return pos
			}
		}
	}

	return enumStart
}

// TransformValueEnumSource transforms Dingo source containing value enums to Go source.
// This is called by the main TransformEnumSource function when it detects a value enum.
// Supports @prefix(false) attributes before enum declarations.
func TransformValueEnumSource(src []byte, filename string) ([]byte, *EnumRegistry) {
	registry := NewEnumRegistry()

	enumPositions := FindEnumDeclarations(src)
	if len(enumPositions) == 0 {
		return src, registry
	}

	result := make([]byte, 0, len(src)+500)
	lastPos := 0

	for _, enumStart := range enumPositions {
		// Check if there's an attribute before this enum
		declStart := findAttributeStart(src, enumStart)

		// Skip if we already processed this declaration (due to attribute detection)
		if declStart < lastPos {
			continue
		}

		// Copy source before this declaration (including any preceding attribute)
		result = append(result, src[lastPos:declStart]...)

		// Check if this is a value enum
		if IsValueEnum(src[enumStart:]) {
			// Parse as value enum with potential attributes
			parser := NewValueEnumParser(src[declStart:], declStart)
			decl, endOffset, err := parser.ParseValueEnumWithAttributes()
			if err != nil {
				// Parsing failed, keep original source (will error later)
				result = append(result, src[declStart:enumStart+4]...)
				lastPos = enumStart + 4
				continue
			}

			// Validate @prefix attribute if present
			usePrefix, validateErr := ValidatePrefixAttribute(decl.Attributes)
			if validateErr != nil {
				// Validation failed - emit error comment and continue
				result = append(result, fmt.Sprintf("/* ERROR: %s */\n", validateErr.Error())...)
				usePrefix = true // Default to safe behavior
			}

			codegen := NewValueEnumCodeGen()

			// Calculate line:col from declaration start
			line, col := offsetToLineCol(src, declStart)

			// Generate Go code with validation
			genResult := codegen.GenerateWithError(decl, filename, line, col)
			if genResult.Error != nil {
				// Validation failed (e.g., empty enum) - emit error comment and continue
				result = append(result, fmt.Sprintf("/* ERROR: %s */\n", genResult.Error.Error())...)
				lastPos = declStart + endOffset
				continue
			}

			// Register variants (only after successful generation)
			variantNames := make([]string, len(decl.Variants))
			for i, v := range decl.Variants {
				variantNames[i] = v.Name.Name
			}
			registry.RegisterValueEnum(decl.Name.Name, variantNames, usePrefix)

			result = append(result, genResult.Output...)

			// Emit reset //line directive after the enum block
			if filename != "" {
				endLine, endCol := offsetToLineCol(src, declStart+endOffset)
				if endLine > 0 && endCol > 0 {
					resetDirective := FormatLineDirective(filename, endLine+1, 1)
					result = append(result, resetDirective...)
				}
			}

			lastPos = declStart + endOffset
		} else {
			// Sum type enum - use existing parser (no attribute support for sum types yet)
			parser := NewEnumParser(src[enumStart:], enumStart)
			decl, endOffset, err := parser.ParseEnumDecl()
			if err != nil {
				// Parsing failed, keep original source
				result = append(result, src[enumStart:enumStart+4]...)
				lastPos = enumStart + 4
				continue
			}

			// Register sum type variants
			for _, v := range decl.Variants {
				registry.RegisterSumTypeVariant(v.Name.Name, decl.Name.Name)
			}

			// Calculate line:col from enumStart
			line, col := offsetToLineCol(src, enumStart)

			// Generate Go code
			codegen := NewEnumCodeGen()
			goCode := codegen.Generate(decl, filename, line, col)
			result = append(result, goCode...)

			// Emit reset //line directive after the enum block
			if filename != "" {
				endLine, endCol := offsetToLineCol(src, enumStart+endOffset)
				if endLine > 0 && endCol > 0 {
					resetDirective := FormatLineDirective(filename, endLine+1, 1)
					result = append(result, resetDirective...)
				}
			}

			lastPos = enumStart + endOffset
		}
	}

	// Copy remaining source
	result = append(result, src[lastPos:]...)

	return result, registry
}
