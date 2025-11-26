// Package builtin provides tuple type generation plugin
package builtin

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"regexp"
	"strings"

	"github.com/MadAppGang/dingo/pkg/plugin"
	"golang.org/x/tools/go/ast/astutil"
)

// TuplePlugin generates tuple type declarations and transforms tuple literals
//
// This plugin handles AST-level processing for tuples preprocessed by TupleProcessor.
// It discovers __TUPLE_{N}__LITERAL__{hash}() markers, performs type inference,
// generates canonical struct types, and transforms markers to struct construction.
//
// Generated structure for Tuple2IntString:
//
//	type Tuple2IntString struct {
//	    _0 int
//	    _1 string
//	}
//
// Transformation:
//   - Input:  __TUPLE_2__LITERAL__{hash}(10, "hello")
//   - Output: Tuple2IntString{_0: 10, _1: "hello"}
type TuplePlugin struct {
	ctx *plugin.Context

	// Track which tuple types we've already emitted to avoid duplicates
	emittedTypes map[string]bool

	// Declarations to inject at package level
	pendingDecls []ast.Decl

	// Type inference service for accurate type resolution
	typeInference *TypeInferenceService

	// Marker pattern: __TUPLE_{N}__LITERAL__{hash}
	markerPattern *regexp.Regexp
}

// NewTuplePlugin creates a new tuple plugin
func NewTuplePlugin() *TuplePlugin {
	return &TuplePlugin{
		emittedTypes:  make(map[string]bool),
		pendingDecls:  make([]ast.Decl, 0),
		markerPattern: regexp.MustCompile(`^__TUPLE_(\d+)__LITERAL__([a-zA-Z0-9]+)$`),
	}
}

// Name returns the plugin name
func (p *TuplePlugin) Name() string {
	return "tuples"
}

// SetContext sets the plugin context (ContextAware interface)
func (p *TuplePlugin) SetContext(ctx *plugin.Context) {
	p.ctx = ctx

	// Initialize type inference service with go/types integration
	if ctx != nil && ctx.FileSet != nil {
		service, err := NewTypeInferenceService(ctx.FileSet, nil, ctx.Logger)
		if err != nil {
			ctx.Logger.Warnf("Failed to create type inference service: %v", err)
		} else {
			p.typeInference = service

			// Inject go/types.Info if available in context
			if ctx.TypeInfo != nil {
				if typesInfo, ok := ctx.TypeInfo.(*types.Info); ok {
					service.SetTypesInfo(typesInfo)
					ctx.Logger.Debugf("Tuple plugin: go/types integration enabled")
				}
			}
		}
	}
}

// Process processes AST nodes to find and transform tuple markers
// This is the Discovery phase - find all __TUPLE_{N}__LITERAL__{hash}() calls
func (p *TuplePlugin) Process(node ast.Node) error {
	if p.ctx == nil {
		return fmt.Errorf("plugin context not initialized")
	}

	// Walk the AST to find tuple literal markers
	ast.Inspect(node, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			p.handleTupleMarker(call)
		}
		return true
	})

	return nil
}

// handleTupleMarker detects and processes __TUPLE_{N}__LITERAL__{hash}() markers
func (p *TuplePlugin) handleTupleMarker(call *ast.CallExpr) {
	// Check if this is a tuple marker call
	ident, ok := call.Fun.(*ast.Ident)
	if !ok {
		return
	}

	matches := p.markerPattern.FindStringSubmatch(ident.Name)
	if matches == nil {
		return
	}

	// Extract arity from marker name
	// matches[1] = arity (e.g., "2" from __TUPLE_2__LITERAL__...)
	// matches[2] = hash (e.g., "abc123def" from __TUPLE_2__LITERAL__abc123def)
	arity := len(call.Args) // Use actual arg count (more reliable than parsing)

	if arity < 2 || arity > 12 {
		p.ctx.Logger.Warnf("Tuple marker has invalid arity %d (expected 2-12)", arity)
		return
	}

	// Perform type inference on each element
	elementTypes := make([]types.Type, arity)
	elementTypeNames := make([]string, arity)

	for i, arg := range call.Args {
		inferredType, ok := p.inferElementType(arg)
		if !ok {
			p.ctx.Logger.Warnf("Failed to infer type for tuple element %d", i)
			// Fallback to interface{} for failed inference
			elementTypeNames[i] = "interface{}"
			continue
		}
		elementTypes[i] = inferredType
		elementTypeNames[i] = p.typeToString(inferredType)
	}

	// Generate canonical type name
	typeName := p.generateTypeName(arity, elementTypeNames)

	// Emit type declaration if not already emitted
	if !p.emittedTypes[typeName] {
		p.emitTupleDeclaration(typeName, elementTypeNames)
		p.emittedTypes[typeName] = true
		p.ctx.Logger.Debugf("Generated tuple type: %s", typeName)
	}
}

// inferElementType performs type inference on a tuple element expression
func (p *TuplePlugin) inferElementType(expr ast.Expr) (types.Type, bool) {
	if p.typeInference == nil {
		return nil, false
	}

	inferredType, ok := p.typeInference.InferType(expr)
	if !ok || inferredType == nil {
		return nil, false
	}

	return inferredType, true
}

// typeToString converts a types.Type to a canonical Go type string
func (p *TuplePlugin) typeToString(t types.Type) string {
	if p.typeInference != nil {
		return p.typeInference.TypeToString(t)
	}

	// Fallback: use types.TypeString
	return types.TypeString(t, nil)
}

// generateTypeName creates canonical tuple type name following Go conventions
// Pattern: Tuple{N}{Type1}{Type2}...{TypeN} (CamelCase without underscores)
// Example: Tuple2IntString, Tuple3UserErrorBool
//
// Note: In extremely rare cases, type name collisions are theoretically possible
// (e.g., (UserError, Bool) and (User, ErrorBool) both → Tuple2UserErrorBool).
// In practice, this is vanishingly rare and detected at compile time if it occurs.
func (p *TuplePlugin) generateTypeName(arity int, elementTypes []string) string {
	parts := []string{fmt.Sprintf("Tuple%d", arity)}

	for _, typeName := range elementTypes {
		sanitized := sanitizeTupleTypeName(typeName)
		// Ensure first letter is capitalized for clear CamelCase boundaries
		capitalized := capitalize(sanitized)
		parts = append(parts, capitalized)
	}

	// Join without delimiters for idiomatic Go CamelCase
	// e.g., (int, string) → Tuple2IntString
	//       (User, Error) → Tuple2UserError
	fullName := strings.Join(parts, "")

	return fullName
}

// capitalize capitalizes the first letter of a string
func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// sanitizeTupleTypeName converts a Go type to CamelCase component for tuple name
// Examples:
//   - int → Int
//   - string → String
//   - *int → PtrInt
//   - []string → SliceString
//   - map[string]int → MapStringInt
//   - User → User (user types unchanged)
func sanitizeTupleTypeName(typeName string) string {
	// Remove package prefixes (e.g., "pkg.User" → "User")
	if idx := strings.LastIndex(typeName, "."); idx != -1 {
		typeName = typeName[idx+1:]
	}

	// Handle pointer types
	if strings.HasPrefix(typeName, "*") {
		return "Ptr" + sanitizeTupleTypeName(typeName[1:])
	}

	// Handle slice types
	if strings.HasPrefix(typeName, "[]") {
		return "Slice" + sanitizeTupleTypeName(typeName[2:])
	}

	// Handle map types: map[K]V → MapKV
	if strings.HasPrefix(typeName, "map[") {
		// Extract key and value types
		inner := typeName[4:] // Remove "map["
		if idx := strings.Index(inner, "]"); idx != -1 {
			keyType := inner[:idx]
			valueType := inner[idx+1:]
			return "Map" + sanitizeTupleTypeName(keyType) + sanitizeTupleTypeName(valueType)
		}
	}

	// Handle channel types
	if strings.HasPrefix(typeName, "chan ") {
		return "Chan" + sanitizeTupleTypeName(typeName[5:])
	}

	// Handle basic types - capitalize first letter
	switch typeName {
	case "int", "int8", "int16", "int32", "int64":
		return capitalize(typeName)
	case "uint", "uint8", "uint16", "uint32", "uint64":
		return capitalize(typeName)
	case "float32", "float64":
		return capitalize(typeName)
	case "bool":
		return "Bool"
	case "string":
		return "String"
	case "byte":
		return "Byte"
	case "rune":
		return "Rune"
	case "error":
		return "Error"
	case "interface{}":
		return "Any"
	default:
		// User-defined types: keep as-is (already CamelCase)
		return typeName
	}
}

// abbreviateTupleName strategically abbreviates long tuple type names
// Only abbreviates if name exceeds 60 characters
func abbreviateTupleName(fullName string, arity int, elementTypes []string) string {
	// Simple strategy: use abbreviated type names
	abbreviations := map[string]string{
		"DatabaseConnection":     "DbConn",
		"HttpRequestHandler":     "HttpReqHandler",
		"AuthenticationService":  "AuthService",
		"Configuration":          "Config",
		"Repository":             "Repo",
		"Controller":             "Ctrl",
		"Manager":                "Mgr",
		"Service":                "Svc",
		"Handler":                "Hdlr",
		"Processor":              "Proc",
		"Interface":              "Iface",
	}

	var parts []string
	parts = append(parts, fmt.Sprintf("Tuple%d", arity))

	for _, typeName := range elementTypes {
		sanitized := sanitizeTupleTypeName(typeName)

		// Check for known abbreviations
		abbreviated := sanitized
		for long, short := range abbreviations {
			if strings.Contains(sanitized, long) {
				abbreviated = strings.ReplaceAll(sanitized, long, short)
				break
			}
		}

		parts = append(parts, abbreviated)
	}

	return strings.Join(parts, "")
}

// emitTupleDeclaration generates and queues a tuple struct declaration
// Example output for Tuple2IntString:
//
//	type Tuple2IntString struct {
//	    _0 int
//	    _1 string
//	}
func (p *TuplePlugin) emitTupleDeclaration(typeName string, elementTypes []string) {
	// Create struct type
	fields := &ast.FieldList{
		List: make([]*ast.Field, 0, len(elementTypes)),
	}

	for i, elemType := range elementTypes {
		fieldName := fmt.Sprintf("_%d", i)
		field := &ast.Field{
			Names: []*ast.Ident{ast.NewIdent(fieldName)},
			Type:  p.parseTypeExpr(elemType),
		}
		fields.List = append(fields.List, field)
	}

	structType := &ast.StructType{
		Fields: fields,
	}

	// Create type declaration
	typeSpec := &ast.TypeSpec{
		Name: ast.NewIdent(typeName),
		Type: structType,
	}

	genDecl := &ast.GenDecl{
		Tok:   token.TYPE,
		Specs: []ast.Spec{typeSpec},
	}

	// Queue for injection
	p.pendingDecls = append(p.pendingDecls, genDecl)
}

// parseTypeExpr converts a type string to an AST expression
// Handles basic types, pointers, slices, maps, etc.
func (p *TuplePlugin) parseTypeExpr(typeStr string) ast.Expr {
	// Handle pointer types
	if strings.HasPrefix(typeStr, "*") {
		return &ast.StarExpr{
			X: p.parseTypeExpr(typeStr[1:]),
		}
	}

	// Handle slice types
	if strings.HasPrefix(typeStr, "[]") {
		return &ast.ArrayType{
			Elt: p.parseTypeExpr(typeStr[2:]),
		}
	}

	// Handle map types: map[K]V
	if strings.HasPrefix(typeStr, "map[") {
		inner := typeStr[4:] // Remove "map["
		if idx := strings.Index(inner, "]"); idx != -1 {
			keyType := inner[:idx]
			valueType := inner[idx+1:]
			return &ast.MapType{
				Key:   p.parseTypeExpr(keyType),
				Value: p.parseTypeExpr(valueType),
			}
		}
	}

	// Handle channel types
	if strings.HasPrefix(typeStr, "chan ") {
		return &ast.ChanType{
			Value: p.parseTypeExpr(typeStr[5:]),
			Dir:   ast.SEND | ast.RECV,
		}
	}

	// Handle interface{} / any
	if typeStr == "interface{}" || typeStr == "any" {
		return &ast.InterfaceType{
			Methods: &ast.FieldList{},
		}
	}

	// Default: simple identifier (basic types or user types)
	return ast.NewIdent(typeStr)
}

// Transform replaces tuple marker calls with struct construction
// This is the Transform phase - rewrite __TUPLE_{N}__LITERAL__{hash}() to TupleNType{...}
func (p *TuplePlugin) Transform(node ast.Node) (ast.Node, error) {
	if p.ctx == nil {
		return node, fmt.Errorf("plugin context not initialized")
	}

	// Use astutil.Apply for safe AST transformation
	result := astutil.Apply(node, func(cursor *astutil.Cursor) bool {
		n := cursor.Node()

		if call, ok := n.(*ast.CallExpr); ok {
			if transformed := p.transformTupleMarker(call); transformed != nil {
				cursor.Replace(transformed)
			}
		}

		return true
	}, nil)

	if file, ok := result.(*ast.File); ok {
		return file, nil
	}

	return node, nil
}

// transformTupleMarker transforms a single tuple marker call to struct literal
// Input:  __TUPLE_2__LITERAL__{hash}(10, "hello")
// Output: Tuple2IntString{_0: 10, _1: "hello"}
func (p *TuplePlugin) transformTupleMarker(call *ast.CallExpr) ast.Expr {
	// Check if this is a tuple marker call
	ident, ok := call.Fun.(*ast.Ident)
	if !ok {
		return nil
	}

	matches := p.markerPattern.FindStringSubmatch(ident.Name)
	if matches == nil {
		return nil
	}

	arity := len(call.Args)
	if arity < 2 || arity > 12 {
		return nil
	}

	// Infer element types to reconstruct type name
	elementTypeNames := make([]string, arity)
	for i, arg := range call.Args {
		inferredType, ok := p.inferElementType(arg)
		if !ok {
			elementTypeNames[i] = "interface{}"
		} else {
			elementTypeNames[i] = p.typeToString(inferredType)
		}
	}

	// Generate type name (must match what was emitted in Process phase)
	typeName := p.generateTypeName(arity, elementTypeNames)

	// Create struct literal: TupleNType{_0: arg0, _1: arg1, ...}
	elts := make([]ast.Expr, 0, arity)
	for i, arg := range call.Args {
		fieldName := fmt.Sprintf("_%d", i)
		elts = append(elts, &ast.KeyValueExpr{
			Key:   ast.NewIdent(fieldName),
			Value: arg,
		})
	}

	structLit := &ast.CompositeLit{
		Type: ast.NewIdent(typeName),
		Elts: elts,
	}

	return structLit
}

// GetPendingDeclarations returns queued type declarations (DeclarationProvider interface)
func (p *TuplePlugin) GetPendingDeclarations() []ast.Decl {
	// Deduplicate type declarations before returning
	seen := make(map[string]bool)
	unique := []ast.Decl{}

	for _, decl := range p.pendingDecls {
		if genDecl, ok := decl.(*ast.GenDecl); ok {
			if genDecl.Tok == token.TYPE && len(genDecl.Specs) > 0 {
				if typeSpec, ok := genDecl.Specs[0].(*ast.TypeSpec); ok {
					typeName := typeSpec.Name.Name
					if !seen[typeName] {
						seen[typeName] = true
						unique = append(unique, decl)
					}
				}
			}
		}
	}

	return unique
}

// ClearPendingDeclarations clears queued declarations (DeclarationProvider interface)
func (p *TuplePlugin) ClearPendingDeclarations() {
	p.pendingDecls = make([]ast.Decl, 0)
}
