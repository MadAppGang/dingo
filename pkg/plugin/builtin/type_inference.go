// Package builtin provides type inference service for Result and Option types
package builtin

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"github.com/MadAppGang/dingo/pkg/plugin"
)

// TypeInferenceService provides type inference for Dingo builtin types
//
// This service recognizes and analyzes:
// - Result<T, E> types (Result_T_E after sanitization)
// - Option<T> types (Option_T after sanitization)
// - None singleton for Option types
//
// Type inference strategy:
// 1. Parse type names to detect pattern (Result_*, Option_*)
// 2. Extract type parameters from sanitized names
// 3. Provide context-based inference for constructors (Ok, Err, None)
// 4. Use go/types for accurate type information when available
// 5. Cache results for performance
type TypeInferenceService struct {
	fset   *token.FileSet
	file   *ast.File
	logger plugin.Logger

	// go/types integration for accurate type inference
	typesInfo *types.Info

	// Phase 4: Parent tracking for context-based inference
	parentMap map[ast.Node]ast.Node

	// Cache for type analysis results
	resultTypeCache map[string]*ResultTypeInfo
	optionTypeCache map[string]*OptionTypeInfo

	// Type registry for synthetic types
	registry *TypeRegistry
}

// ResultTypeInfo contains parsed Result type information
type ResultTypeInfo struct {
	TypeName     string     // e.g., "Result_int_error"
	OkType       types.Type // T type parameter
	ErrType      types.Type // E type parameter
	OkTypeString string     // Original type string (e.g., "map[string]int")
	ErrTypeString string    // Original error type string (e.g., "error")
}

// OptionTypeInfo contains parsed Option type information
type OptionTypeInfo struct {
	TypeName       string     // e.g., "Option_int"
	ValueType      types.Type // T type parameter
	ValueTypeString string    // Original type string (e.g., "map[string]int")
}

// TypeRegistry manages synthetic types created by Dingo
type TypeRegistry struct {
	// Maps type names to their Type objects
	resultTypes map[string]*ResultTypeInfo
	optionTypes map[string]*OptionTypeInfo
}

// NewTypeRegistry creates a new type registry
func NewTypeRegistry() *TypeRegistry {
	return &TypeRegistry{
		resultTypes: make(map[string]*ResultTypeInfo),
		optionTypes: make(map[string]*OptionTypeInfo),
	}
}

// NewTypeInferenceService creates a type inference service
func NewTypeInferenceService(fset *token.FileSet, file *ast.File, logger plugin.Logger) (*TypeInferenceService, error) {
	if logger == nil {
		logger = plugin.NewNoOpLogger()
	}

	return &TypeInferenceService{
		fset:            fset,
		file:            file,
		logger:          logger,
		typesInfo:       nil, // Set later via SetTypesInfo()
		resultTypeCache: make(map[string]*ResultTypeInfo),
		optionTypeCache: make(map[string]*OptionTypeInfo),
		registry:        NewTypeRegistry(),
	}, nil
}

// SetTypesInfo sets the go/types information for accurate type inference
// This should be called after running the type checker
func (s *TypeInferenceService) SetTypesInfo(info *types.Info) {
	s.typesInfo = info
	s.logger.Debugf("Type inference service updated with go/types information")
}

// SetParentMap sets parent map for context-based type inference
func (s *TypeInferenceService) SetParentMap(parentMap map[ast.Node]ast.Node) {
	s.parentMap = parentMap
}

// IsResultType checks if a type name represents a Result type
//
// Recognizes patterns:
// - Result_T_E (e.g., Result_int_error)
// - Result_ptr_User_error
// - Result_slice_byte_CustomError
func (s *TypeInferenceService) IsResultType(typeName string) bool {
	return strings.HasPrefix(typeName, "Result_")
}

// IsOptionType checks if a type name represents an Option type
//
// Recognizes patterns:
// - Option_T (e.g., Option_int)
// - Option_ptr_User
// - Option_slice_byte
func (s *TypeInferenceService) IsOptionType(typeName string) bool {
	return strings.HasPrefix(typeName, "Option_")
}

// GetResultTypeParams extracts type parameters from Result type name
//
// Examples:
//
//	Result_int_error → (int, error, true)
//	Result_ptr_User_CustomError → (*User, CustomError, true)
//	Result_slice_byte_error → ([]byte, error, true)
//	NotAResult → (nil, nil, false)
//
// CRITICAL FIX #1: Only uses cached values - does NOT reverse-parse from type name
// Reverse parsing breaks for complex types like Result<map[string]int, error>
// because sanitization is lossy (e.g., "[" → "_", "]" → "_")
func (s *TypeInferenceService) GetResultTypeParams(typeName string) (T, E types.Type, ok bool) {
	if !s.IsResultType(typeName) {
		return nil, nil, false
	}

	// Check cache - this is the ONLY source of truth
	if cached, found := s.resultTypeCache[typeName]; found {
		return cached.OkType, cached.ErrType, true
	}

	// CRITICAL FIX #1: Don't reverse-parse - fail if not cached
	// Reverse parsing breaks for complex types like map[string]int
	s.logger.Warnf("Result type %s not in cache - cannot infer types (reverse parsing disabled)", typeName)
	return nil, nil, false
}

// GetOptionTypeParam extracts the type parameter from Option type name
//
// Examples:
//
//	Option_int → (int, true)
//	Option_ptr_User → (*User, true)
//	Option_slice_byte → ([]byte, true)
//	NotAnOption → (nil, false)
//
// CRITICAL FIX #1: Only uses cached values - does NOT reverse-parse from type name
func (s *TypeInferenceService) GetOptionTypeParam(typeName string) (T types.Type, ok bool) {
	if !s.IsOptionType(typeName) {
		return nil, false
	}

	// Check cache - this is the ONLY source of truth
	if cached, found := s.optionTypeCache[typeName]; found {
		return cached.ValueType, true
	}

	// CRITICAL FIX #1: Don't reverse-parse - fail if not cached
	s.logger.Warnf("Option type %s not in cache - cannot infer types (reverse parsing disabled)", typeName)
	return nil, false
}

// parseTypeFromTokensBackward parses a type from tokens working backward
// Returns the type and the number of tokens consumed
//
// Handles: ptr_, slice_, basic types
func (s *TypeInferenceService) parseTypeFromTokensBackward(tokens []string) (types.Type, int) {
	if len(tokens) == 0 {
		return types.Typ[types.Invalid], 0
	}

	// Start from the last token
	lastToken := tokens[len(tokens)-1]

	// Simple type (no prefix)
	if len(tokens) == 1 {
		return s.makeBasicType(lastToken), 1
	}

	// Check for type modifiers in reverse
	if len(tokens) >= 2 {
		modifier := tokens[len(tokens)-2]

		switch modifier {
		case "ptr":
			// ptr_TypeName
			baseType := s.makeBasicType(lastToken)
			return types.NewPointer(baseType), 2

		case "slice":
			// slice_TypeName
			elemType := s.makeBasicType(lastToken)
			return types.NewSlice(elemType), 2
		}
	}

	// Default: treat as simple type
	return s.makeBasicType(lastToken), 1
}

// parseTypeFromTokensForward parses a type from tokens working forward
// Returns the type and the number of tokens consumed
func (s *TypeInferenceService) parseTypeFromTokensForward(tokens []string) (types.Type, int) {
	if len(tokens) == 0 {
		return types.Typ[types.Invalid], 0
	}

	firstToken := tokens[0]

	// Handle type modifiers
	switch firstToken {
	case "ptr":
		// ptr_TypeName
		if len(tokens) >= 2 {
			baseType, consumed := s.parseTypeFromTokensForward(tokens[1:])
			return types.NewPointer(baseType), consumed + 1
		}
		return types.Typ[types.Invalid], 1

	case "slice":
		// slice_TypeName
		if len(tokens) >= 2 {
			elemType, consumed := s.parseTypeFromTokensForward(tokens[1:])
			return types.NewSlice(elemType), consumed + 1
		}
		return types.Typ[types.Invalid], 1

	default:
		// Simple type
		return s.makeBasicType(firstToken), 1
	}
}

// makeBasicType creates a basic type from a token string
func (s *TypeInferenceService) makeBasicType(typeName string) types.Type {
	// Map token to basic Go types
	switch typeName {
	case "int":
		return types.Typ[types.Int]
	case "int8":
		return types.Typ[types.Int8]
	case "int16":
		return types.Typ[types.Int16]
	case "int32":
		return types.Typ[types.Int32]
	case "int64":
		return types.Typ[types.Int64]
	case "uint":
		return types.Typ[types.Uint]
	case "uint8":
		return types.Typ[types.Uint8]
	case "uint16":
		return types.Typ[types.Uint16]
	case "uint32":
		return types.Typ[types.Uint32]
	case "uint64":
		return types.Typ[types.Uint64]
	case "float32":
		return types.Typ[types.Float32]
	case "float64":
		return types.Typ[types.Float64]
	case "string":
		return types.Typ[types.String]
	case "bool":
		return types.Typ[types.Bool]
	case "byte":
		return types.Typ[types.Byte]
	case "rune":
		return types.Typ[types.Rune]
	case "error":
		// error is an interface, create a named type
		return types.Universe.Lookup("error").Type()
	case "interface{}":
		return types.NewInterfaceType(nil, nil)
	default:
		// Unknown type - create a named type placeholder
		return types.NewNamed(
			types.NewTypeName(token.NoPos, nil, typeName, nil),
			types.Typ[types.Invalid],
			nil,
		)
	}
}

// InferType infers the type of an AST expression using go/types
//
// This is the primary type inference method that leverages go/types.Info
// when available. It falls back to structural analysis for simple cases.
//
// Returns:
//   - The inferred types.Type, or nil if inference fails
//   - A boolean indicating whether inference succeeded
//
// Example usage:
//
//	typ, ok := service.InferType(expr)
//	if ok {
//	    typeName := service.TypeToString(typ)
//	}
func (s *TypeInferenceService) InferType(expr ast.Expr) (types.Type, bool) {
	if expr == nil {
		s.logger.Debugf("InferType: nil expression")
		return nil, false
	}

	// Strategy 1: Use go/types if available (most accurate)
	if s.typesInfo != nil && s.typesInfo.Types != nil {
		if tv, ok := s.typesInfo.Types[expr]; ok && tv.Type != nil {
			s.logger.Debugf("InferType: go/types resolved %T to %s", expr, tv.Type)
			return tv.Type, true
		}
		s.logger.Debugf("InferType: go/types has no information for %T", expr)
	}

	// Strategy 2: Structural inference for basic literals (fallback)
	switch e := expr.(type) {
	case *ast.BasicLit:
		return s.inferBasicLitType(e), true

	case *ast.Ident:
		// Check for built-in constants
		if typ := s.inferBuiltinIdent(e); typ != nil {
			return typ, true
		}
		// For variables, we need go/types - can't infer without it
		s.logger.Debugf("InferType: identifier %q requires go/types for accurate inference", e.Name)
		return nil, false

	case *ast.UnaryExpr:
		if e.Op == token.AND {
			// &expr - pointer to expr's type
			if innerType, ok := s.InferType(e.X); ok {
				return types.NewPointer(innerType), true
			}
		}
		return nil, false

	case *ast.CompositeLit:
		// Composite literal with explicit type
		if e.Type != nil {
			// This requires parsing the type expression to types.Type
			// For now, return nil - proper implementation needs type reconstruction
			s.logger.Debugf("InferType: composite literal type requires AST->types.Type conversion")
			return nil, false
		}
		return nil, false

	case *ast.CallExpr:
		// Special handling for tuple markers and builtin type constructors
		if ident, ok := e.Fun.(*ast.Ident); ok {
			// Check for tuple marker pattern: __TUPLE_{N}__LITERAL__{hash}
			if s.isTupleMarker(ident.Name) {
				// Recursively infer the tuple type from its elements
				tupleTypeName := s.inferTupleType(e)
				if tupleTypeName != "" {
					s.logger.Debugf("InferType: Tuple marker %s → %s", ident.Name, tupleTypeName)
					return types.NewNamed(types.NewTypeName(0, nil, tupleTypeName, nil), nil, nil), true
				}
				s.logger.Debugf("InferType: Failed to infer tuple type for %s", ident.Name)
				return nil, false
			}

			switch ident.Name {
			case "Ok":
				// Ok(value) → Result<T, E> where T is inferred from value
				if len(e.Args) == 1 {
					if argType, ok := s.InferType(e.Args[0]); ok {
						// Result type is Result_T_error (standard error type)
						resultTypeName := s.resultTypeNameFromOk(argType)
						s.logger.Debugf("InferType: Ok(%s) → %s", argType, resultTypeName)
						// Return a named type for Result
						return types.NewNamed(types.NewTypeName(0, nil, resultTypeName, nil), nil, nil), true
					}
				}
			case "Err":
				// Err(error) → Result<T, E> where E is inferred from error
				// For tuples, this is less common but we'll handle it
				if len(e.Args) == 1 {
					if argType, ok := s.InferType(e.Args[0]); ok {
						// Result type is Result_interface{}_E (generic T)
						resultTypeName := s.resultTypeNameFromErr(argType)
						s.logger.Debugf("InferType: Err(%s) → %s", argType, resultTypeName)
						return types.NewNamed(types.NewTypeName(0, nil, resultTypeName, nil), nil, nil), true
					}
				}
			case "Some":
				// Some(value) → Option<T> where T is inferred from value
				if len(e.Args) == 1 {
					if argType, ok := s.InferType(e.Args[0]); ok {
						optionTypeName := s.optionTypeNameFromSome(argType)
						s.logger.Debugf("InferType: Some(%s) → %s", argType, optionTypeName)
						return types.NewNamed(types.NewTypeName(0, nil, optionTypeName, nil), nil, nil), true
					}
				}
			case "None":
				// None → Option<T> (T unknown without context)
				// For tuples, this will fall back to interface{} which is acceptable
				s.logger.Debugf("InferType: None requires context for type inference")
				return nil, false
			}
		}

		// General function call - need go/types to determine return type
		s.logger.Debugf("InferType: function call %T requires go/types for return type", e.Fun)
		return nil, false

	default:
	s.logger.Debugf("InferType: unsupported expression type %T", expr)
		return nil, false
	}
}

// inferBasicLitType infers the type of a basic literal
func (s *TypeInferenceService) inferBasicLitType(lit *ast.BasicLit) types.Type {
	switch lit.Kind {
	case token.INT:
		return types.Typ[types.UntypedInt]
	case token.FLOAT:
		return types.Typ[types.UntypedFloat]
	case token.STRING:
		return types.Typ[types.UntypedString]
	case token.CHAR:
		return types.Typ[types.UntypedRune]
	default:
		return types.Typ[types.Invalid]
	}
}

// inferBuiltinIdent infers the type of built-in identifiers
func (s *TypeInferenceService) inferBuiltinIdent(ident *ast.Ident) types.Type {
	switch ident.Name {
	case "nil":
		return types.Typ[types.UntypedNil]
	case "true", "false":
		return types.Typ[types.UntypedBool]
	default:
		return nil
	}
}

// resultTypeNameFromOk generates Result type name from Ok(value) call
// Ok(42) → Result_int_error
// Ok("hello") → Result_string_error
func (s *TypeInferenceService) resultTypeNameFromOk(valueType types.Type) string {
	valueTypeStr := s.TypeToString(valueType)
	// Result types use pattern: Result_T_error (standard error type)
	return fmt.Sprintf("Result_%s_error", sanitizeTypeForName(valueTypeStr))
}

// resultTypeNameFromErr generates Result type name from Err(error) call
// Err(fmt.Errorf(...)) → Result_interface{}_error
func (s *TypeInferenceService) resultTypeNameFromErr(errType types.Type) string {
	errTypeStr := s.TypeToString(errType)
	// When only error is known, T defaults to interface{}
	return fmt.Sprintf("Result_interface{}__%s", sanitizeTypeForName(errTypeStr))
}

// optionTypeNameFromSome generates Option type name from Some(value) call
// Some(42) → Option_int
// Some("hello") → Option_string
func (s *TypeInferenceService) optionTypeNameFromSome(valueType types.Type) string {
	valueTypeStr := s.TypeToString(valueType)
	return fmt.Sprintf("Option_%s", sanitizeTypeForName(valueTypeStr))
}

// sanitizeTypeForName converts type string to valid identifier component
// int → int
// *int → ptr_int
// []string → slice_string
// map[string]int → map_string_int
func sanitizeTypeForName(typeName string) string {
	// Replace special characters with underscores
	result := strings.ReplaceAll(typeName, "*", "ptr_")
	result = strings.ReplaceAll(result, "[]", "slice_")
	result = strings.ReplaceAll(result, "[", "_")
	result = strings.ReplaceAll(result, "]", "_")
	result = strings.ReplaceAll(result, " ", "_")
	result = strings.ReplaceAll(result, "{", "_")
	result = strings.ReplaceAll(result, "}", "_")
	// Remove double underscores
	for strings.Contains(result, "__") {
		result = strings.ReplaceAll(result, "__", "_")
	}
	// Remove trailing underscore
	result = strings.TrimSuffix(result, "_")
	return result
}

// TypeToString converts a types.Type to its Go source representation
//
// This method converts types.Type objects back to Go source code strings.
// It handles all standard Go types and produces idiomatic output.
//
// Examples:
//
//	types.Typ[types.Int] → "int"
//	types.NewPointer(types.Typ[types.String]) → "*string"
//	types.NewSlice(types.Typ[types.Byte]) → "[]byte"
//
// This is essential for generating correct type names in code generation.
func (s *TypeInferenceService) TypeToString(typ types.Type) string {
	if typ == nil {
		return "interface{}"
	}

	switch t := typ.(type) {
	case *types.Basic:
		// Handle untyped constants by converting to typed equivalents
		switch t.Kind() {
		case types.UntypedBool:
			return "bool"
		case types.UntypedInt:
			return "int"
		case types.UntypedRune:
			return "rune"
		case types.UntypedFloat:
			return "float64"
		case types.UntypedComplex:
			return "complex128"
		case types.UntypedString:
			return "string"
		case types.UntypedNil:
			return "interface{}" // nil has no specific type
		default:
			return t.String()
		}

	case *types.Pointer:
		return "*" + s.TypeToString(t.Elem())

	case *types.Slice:
		return "[]" + s.TypeToString(t.Elem())

	case *types.Array:
		return fmt.Sprintf("[%d]%s", t.Len(), s.TypeToString(t.Elem()))

	case *types.Map:
		return fmt.Sprintf("map[%s]%s", s.TypeToString(t.Key()), s.TypeToString(t.Elem()))

	case *types.Chan:
		switch t.Dir() {
		case types.SendRecv:
			return "chan " + s.TypeToString(t.Elem())
		case types.SendOnly:
			return "chan<- " + s.TypeToString(t.Elem())
		case types.RecvOnly:
			return "<-chan " + s.TypeToString(t.Elem())
		}

	case *types.Named:
		// Named type (struct, interface, or type alias)
		obj := t.Obj()
		if obj != nil {
			// Check if the type is from a package
			if pkg := obj.Pkg(); pkg != nil && pkg.Name() != "" {
				// Qualified name: pkg.Type
				return pkg.Name() + "." + obj.Name()
			}
			// Local type or built-in
			return obj.Name()
		}
		return t.String()

	case *types.Struct:
		// Anonymous struct
		return "struct{}"

	case *types.Interface:
		// Interface type
		if t.NumMethods() == 0 && t.NumEmbeddeds() == 0 {
			return "interface{}"
		}
		// For non-empty interfaces, use the full type string
		return t.String()

	case *types.Signature:
		// Function type
		return s.signatureToString(t)

	case *types.Tuple:
		// Tuple (multiple return values)
		if t.Len() == 0 {
			return ""
		}
		parts := make([]string, t.Len())
		for i := 0; i < t.Len(); i++ {
			parts[i] = s.TypeToString(t.At(i).Type())
		}
		if t.Len() == 1 {
			return parts[0]
		}
		return "(" + strings.Join(parts, ", ") + ")"

	default:
		// Fallback to string representation
		return typ.String()
	}

	return "interface{}"
}

// signatureToString converts a function signature to a string
func (s *TypeInferenceService) signatureToString(sig *types.Signature) string {
	// Build parameter list
	params := s.tupleToParamString(sig.Params())

	// Build result list
	results := ""
	if sig.Results() != nil && sig.Results().Len() > 0 {
		if sig.Results().Len() == 1 {
			results = " " + s.TypeToString(sig.Results().At(0).Type())
		} else {
			results = " " + s.tupleToParamString(sig.Results())
		}
	}

	return "func(" + params + ")" + results
}

// tupleToParamString converts a parameter tuple to a string
func (s *TypeInferenceService) tupleToParamString(tuple *types.Tuple) string {
	if tuple == nil || tuple.Len() == 0 {
		return ""
	}

	parts := make([]string, tuple.Len())
	for i := 0; i < tuple.Len(); i++ {
		v := tuple.At(i)
		typeStr := s.TypeToString(v.Type())

		// Include parameter name if available
		if v.Name() != "" {
			parts[i] = v.Name() + " " + typeStr
		} else {
			parts[i] = typeStr
		}
	}
	return strings.Join(parts, ", ")
}

// InferTypeFromContext attempts to infer type from surrounding context
//
// This is the main Phase 4 implementation that leverages:
// 1. Parent tracking (Context.BuildParentMap())
// 2. go/types information for type resolution
// 3. AST pattern matching for different context types
//
// Supported Contexts:
// 1. Return statements: infers from function return type
// 2. Assignment statements: infers from variable type
// 3. Variable declarations: infers from explicit type annotation
// 4. Function call arguments: infers from parameter type
// 5. Binary operations: infers from other operand
//
func (s *TypeInferenceService) InferTypeFromContext(node ast.Node) (types.Type, bool) {
	s.logger.Debugf("InferTypeFromContext called for node type: %T", node)

	if node == nil {
		return nil, false
	}

	// Phase 4: Use parent tracking to find context
	if s.parentMap == nil {
		s.logger.Debugf("InferTypeFromContext: no parent map available")
		return nil, false
	}

	s.logger.Debugf("InferTypeFromContext: parent map has %d entries", len(s.parentMap))

	// Walk up parent chain to find type-defining context
	current := node
	for current != nil {
		parent := s.parentMap[current]
		if parent == nil {
			break
		}

		s.logger.Debugf("InferTypeFromContext: checking parent type %T", parent)

		// Context 1: Return statement - infer from function return type
		if retStmt, ok := parent.(*ast.ReturnStmt); ok {
			// Find the containing function
			fnType := s.findFunctionReturnType(retStmt)
			if fnType != nil {
				s.logger.Debugf("InferTypeFromContext: inferred from return type: %s", fnType)
				return fnType, true
			}
		}

		// Context 2: Assignment statement - infer from variable type
		if assign, ok := parent.(*ast.AssignStmt); ok {
			// Find which LHS this None is being assigned to
			varType := s.findAssignmentType(assign, node)
			if varType != nil {
				s.logger.Debugf("InferTypeFromContext: inferred from assignment type: %s", varType)
				return varType, true
			}
		}

		// Context 3: Variable declaration - infer from explicit type
		if decl, ok := parent.(*ast.GenDecl); ok && decl.Tok == token.VAR {
			// Find the value spec containing this node
			varType := s.findVarDeclType(decl, node)
			if varType != nil {
				s.logger.Debugf("InferTypeFromContext: inferred from var decl type: %s", varType)
				return varType, true
			}
		}

		// Context 4: Function call argument - infer from parameter type
		if call, ok := parent.(*ast.CallExpr); ok {
			paramType := s.findCallArgType(call, node)
			if paramType != nil {
				s.logger.Debugf("InferTypeFromContext: inferred from call param type: %s", paramType)
				return paramType, true
			}
		}

		// Move up the parent chain
		current = parent
	}

	s.logger.Debugf("InferTypeFromContext: no type context found")
	return nil, false
}

// Helper methods for InferTypeFromContext

// findFunctionReturnType finds the return type of the function containing a return statement
func (s *TypeInferenceService) findFunctionReturnType(retStmt *ast.ReturnStmt) types.Type {
	if s.typesInfo == nil {
		return nil
	}

	// Walk up parent chain to find *ast.FuncDecl or *ast.FuncLit
	current := ast.Node(retStmt)
	for current != nil {
		parent := s.parentMap[current]
		if parent == nil {
			break
		}

		// Case 1: Named function
		if funcDecl, ok := parent.(*ast.FuncDecl); ok {
			s.logger.Debugf("findFunctionReturnType: found function declaration %s", funcDecl.Name.Name)
			retType := s.extractReturnTypeFromFuncType(funcDecl.Type, retStmt)
			s.logger.Debugf("findFunctionReturnType: extracted return type: %v", retType)
			return retType
		}

		// Case 2: Anonymous function
		if funcLit, ok := parent.(*ast.FuncLit); ok {
			return s.extractReturnTypeFromFuncType(funcLit.Type, retStmt)
		}

		current = parent
	}
	return nil
}

// extractReturnTypeFromFuncType extracts the return type from a function type
func (s *TypeInferenceService) extractReturnTypeFromFuncType(
	funcType *ast.FuncType,
	retStmt *ast.ReturnStmt,
) types.Type {
	if funcType.Results == nil || len(funcType.Results.List) == 0 {
		return nil
	}

	// Determine which return value corresponds to the return statement
	// For now, assume single return value (extend for multi-return later)
	if len(funcType.Results.List) > 0 {
		resultField := funcType.Results.List[0]
		s.logger.Debugf("extractReturnTypeFromFuncType: result field type: %T", resultField.Type)

		// PRIORITY 4 FIX: Try go/types first, then fall back to AST-based inference
		if s.typesInfo != nil {
			if tv, ok := s.typesInfo.Types[resultField.Type]; ok && tv.Type != nil {
				s.logger.Debugf("extractReturnTypeFromFuncType: go/types found type: %v", tv.Type)
				// Only use go/types result if it's NOT invalid type
				// (Option types won't be in go/types until they're generated)
				if tv.Type.String() != "invalid type" {
					return tv.Type
				}
				s.logger.Debugf("extractReturnTypeFromFuncType: go/types returned invalid type, falling back to AST")
			} else {
				s.logger.Debugf("extractReturnTypeFromFuncType: go/types did not have type info")
			}
		}

		// Fallback: Extract type name from AST (for types not yet in go/types like Option_int)
		// This handles cases where Option types haven't been generated yet
		if ident, ok := resultField.Type.(*ast.Ident); ok {
			// Simple identifier type (e.g., Option_int)
			s.logger.Debugf("extractReturnTypeFromFuncType: creating type from identifier: %s", ident.Name)
			namedType := s.makeBasicType(ident.Name)
			s.logger.Debugf("extractReturnTypeFromFuncType: created type: %v (%T)", namedType, namedType)
			return namedType
		}
	}

	return nil
}

// findAssignmentType finds the type of variable in an assignment statement
func (s *TypeInferenceService) findAssignmentType(assign *ast.AssignStmt, targetNode ast.Node) types.Type {
	if s.typesInfo == nil {
		return nil
	}

	// Find which RHS position the targetNode is in
	rhsIndex := -1
	for i, rhs := range assign.Rhs {
		if s.containsNode(rhs, targetNode) {
			rhsIndex = i
			break
		}
	}

	if rhsIndex == -1 {
		return nil
	}

	// Case 1: Regular assignment (same number of LHS and RHS)
	if rhsIndex < len(assign.Lhs) {
		lhsExpr := assign.Lhs[rhsIndex]

		// Use go/types to get type of LHS expression
		if tv, ok := s.typesInfo.Types[lhsExpr]; ok && tv.Type != nil {
			return tv.Type
		}
	}

	// Case 2: Multiple return values (one RHS, multiple LHS)
	if len(assign.Rhs) == 1 && len(assign.Lhs) > 1 && rhsIndex == 0 {
		// The RHS returns a tuple, find which LHS position we're assigning to
		// This case is handled above - we can't determine which tuple element
		// from just the RHS node. This would need more context.
		return nil
	}

	return nil
}

// findVarDeclType finds the explicit type in a variable declaration
func (s *TypeInferenceService) findVarDeclType(decl *ast.GenDecl, targetNode ast.Node) types.Type {
	if s.typesInfo == nil {
		return nil
	}

	// Find the ValueSpec containing targetNode
	for _, spec := range decl.Specs {
		if valueSpec, ok := spec.(*ast.ValueSpec); ok {
			// Check if targetNode is in this ValueSpec's values
			for i, value := range valueSpec.Values {
				if s.containsNode(value, targetNode) {
					// Case 1: Explicit type annotation
					if valueSpec.Type != nil {
						if tv, ok := s.typesInfo.Types[valueSpec.Type]; ok && tv.Type != nil {
							return tv.Type
						}
					}

					// Case 2: Infer from initializer (same as assignment)
					if i < len(valueSpec.Values) {
						if tv, ok := s.typesInfo.Types[valueSpec.Values[i]]; ok && tv.Type != nil {
							return tv.Type
						}
					}
				}
			}
		}
	}

	return nil
}

// findCallArgType finds the parameter type for a call argument
func (s *TypeInferenceService) findCallArgType(call *ast.CallExpr, targetNode ast.Node) types.Type {
	if s.typesInfo == nil {
		return nil
	}

	// Find which argument position targetNode is in
	argIndex := -1
	for i, arg := range call.Args {
		if s.containsNode(arg, targetNode) {
			argIndex = i
			break
		}
	}

	if argIndex == -1 {
		return nil
	}

	// Get the function type from go/types
	if tv, ok := s.typesInfo.Types[call.Fun]; ok && tv.Type != nil {
		if sig, ok := tv.Type.(*types.Signature); ok {
			params := sig.Params()

			// Handle variadic functions
			if sig.Variadic() && argIndex >= params.Len()-1 {
				// Last parameter is variadic
				if params.Len() > 0 {
					lastParam := params.At(params.Len() - 1)
					// Extract element type from slice
					if slice, ok := lastParam.Type().(*types.Slice); ok {
						return slice.Elem()
					}
				}
			} else if argIndex < params.Len() {
				return params.At(argIndex).Type()
			}
		}
	}

	return nil
}

// containsNode checks if parent AST node contains child in its subtree
func (s *TypeInferenceService) containsNode(root, target ast.Node) bool {
	if root == nil || target == nil {
		return false
	}
	found := false
	ast.Inspect(root, func(n ast.Node) bool {
		if n == target {
			found = true
			return false
		}
		return true
	})
	return found
}

// inferTypeFromExpr infers types.Type from an AST expression
func (s *TypeInferenceService) inferTypeFromExpr(expr ast.Expr) (types.Type, bool) {
	// Use go/types if available
	if s.typesInfo != nil && s.typesInfo.Types != nil {
		if tv, ok := s.typesInfo.Types[expr]; ok && tv.Type != nil {
			return tv.Type, true
		}
	}

	// For Option and Result types, we may need to resolve from name
	switch e := expr.(type) {
	case *ast.Ident:
		// This is where we might resolve Option_int, Result_string, etc.
		// May need to look up in type registry or go/types
		if e.Obj != nil {
			if obj, ok := s.typesInfo.ObjectOf(e).(*types.TypeName); ok {
				return obj.Type(), true
			}
		}
	}

	return nil, false
}

// RegisterResultType registers a Result type in the type registry
//
// CRITICAL FIX #1: Now requires original type strings for validation
// CRITICAL FIX #4 (Code Review): Detects and reports type registry collisions
func (s *TypeInferenceService) RegisterResultType(typeName string, okType, errType types.Type, okTypeStr, errTypeStr string) {
	// CRITICAL FIX #4: Check for collision before registering
	if existing, found := s.resultTypeCache[typeName]; found {
		// Check if this is a different type combination with the same sanitized name
		if existing.OkTypeString != okTypeStr || existing.ErrTypeString != errTypeStr {
			s.logger.Error(fmt.Sprintf(
				"Type registry collision detected: %s represents both Result<%s, %s> and Result<%s, %s>",
				typeName,
				existing.OkTypeString, existing.ErrTypeString,
				okTypeStr, errTypeStr,
			))
			// Add warning to help users understand the issue
			s.logger.Warnf(
				"Collision hint: Use explicit type aliases to avoid ambiguity, or file a bug report",
			)
		}
		// If types match, this is just a re-registration (safe to continue)
	}

	info := &ResultTypeInfo{
		TypeName:      typeName,
		OkType:        okType,
		ErrType:       errType,
		OkTypeString:  okTypeStr,
		ErrTypeString: errTypeStr,
	}
	s.resultTypeCache[typeName] = info
	s.registry.resultTypes[typeName] = info

	s.logger.Debugf("Registered Result type: %s (T=%s, E=%s)", typeName, okTypeStr, errTypeStr)

	// CRITICAL FIX #1: Validate round-trip consistency
	// Ensure type name is actually derived from these type strings
	expectedTypeName := fmt.Sprintf("Result_%s_%s",
		s.sanitizeTypeName(okTypeStr),
		s.sanitizeTypeName(errTypeStr))
	if typeName != expectedTypeName {
		s.logger.Warnf("Type name mismatch: expected %s, got %s (sanitization may be lossy)", expectedTypeName, typeName)
	}
}

// sanitizeTypeName is a helper for validation
func (s *TypeInferenceService) sanitizeTypeName(typeName string) string {
	str := typeName
	if str == "interface{}" {
		return "any"
	}
	str = strings.ReplaceAll(str, "*", "ptr_")
	str = strings.ReplaceAll(str, "[]", "slice_")
	str = strings.ReplaceAll(str, "[", "_")
	str = strings.ReplaceAll(str, "]", "_")
	str = strings.ReplaceAll(str, ".", "_")
	str = strings.ReplaceAll(str, "{", "")
	str = strings.ReplaceAll(str, "}", "")
	str = strings.ReplaceAll(str, " ", "")
	str = strings.Trim(str, "_")
	return str
}

// RegisterOptionType registers an Option type in the type registry
//
// CRITICAL FIX #1: Now requires original type string for validation
// CRITICAL FIX #4 (Code Review): Detects and reports type registry collisions
func (s *TypeInferenceService) RegisterOptionType(typeName string, valueType types.Type, valueTypeStr string) {
	// CRITICAL FIX #4: Check for collision before registering
	if existing, found := s.optionTypeCache[typeName]; found {
		// Check if this is a different type with the same sanitized name
		if existing.ValueTypeString != valueTypeStr {
			s.logger.Error(fmt.Sprintf(
				"Type registry collision detected: %s represents both Option<%s> and Option<%s>",
				typeName,
				existing.ValueTypeString,
				valueTypeStr,
			))
			// Add warning to help users understand the issue
			s.logger.Warnf(
				"Collision hint: Use explicit type aliases to avoid ambiguity, or file a bug report",
			)
		}
		// If types match, this is just a re-registration (safe to continue)
	}

	info := &OptionTypeInfo{
		TypeName:        typeName,
		ValueType:       valueType,
		ValueTypeString: valueTypeStr,
	}
	s.optionTypeCache[typeName] = info
	s.registry.optionTypes[typeName] = info

	s.logger.Debugf("Registered Option type: %s (T=%s)", typeName, valueTypeStr)

	// CRITICAL FIX #1: Validate round-trip consistency
	expectedTypeName := fmt.Sprintf("Option_%s", s.sanitizeTypeName(valueTypeStr))
	if typeName != expectedTypeName {
		s.logger.Warnf("Type name mismatch: expected %s, got %s (sanitization may be lossy)", expectedTypeName, typeName)
	}
}

// GetRegistry returns the type registry for external access
func (s *TypeInferenceService) GetRegistry() *TypeRegistry {
	return s.registry
}

// ValidateNoneInference checks if None can be type-inferred in context
//
// Returns:
// - ok=true if type can be inferred
// - suggestion: helpful error message if inference failed
func (s *TypeInferenceService) ValidateNoneInference(noneExpr ast.Expr) (ok bool, suggestion string) {
	// Check if None appears in a context where type can be inferred

	// TODO: Implement full context checking
	// For now, we'll use a simple heuristic:
	// - If None is in an assignment with explicit type, OK
	// - If None is a function argument, check parameter type
	// - If None is a return value, check function signature
	// - Otherwise, fail with suggestion

	s.logger.Debugf("ValidateNoneInference called for expr at pos %v", s.fset.Position(noneExpr.Pos()))

	// Placeholder: Always fail for now (Task 1.5 will implement this)
	return false, fmt.Sprintf(
		"Cannot infer type for None at %s\nHelp: Add explicit type annotation: let varName: Option<YourType> = None",
		s.fset.Position(noneExpr.Pos()),
	)
}

// GetResultTypes returns all registered Result types
func (r *TypeRegistry) GetResultTypes() map[string]*ResultTypeInfo {
	return r.resultTypes
}

// GetOptionTypes returns all registered Option types
func (r *TypeRegistry) GetOptionTypes() map[string]*OptionTypeInfo {
	return r.optionTypes
}

// isTupleMarker checks if an identifier is a tuple marker
// Pattern: __TUPLE_{N}__LITERAL__{hash}
func (s *TypeInferenceService) isTupleMarker(name string) bool {
	return strings.HasPrefix(name, "__TUPLE_") && strings.Contains(name, "__LITERAL__")
}

// inferTupleType recursively infers the type of a tuple marker call
// Returns the canonical tuple type name (e.g., "Tuple2IntString", "Tuple2Tuple2IntIntString")
func (s *TypeInferenceService) inferTupleType(call *ast.CallExpr) string {
	arity := len(call.Args)
	if arity < 2 || arity > 12 {
		return ""
	}

	// Infer each element type
	elementTypeNames := make([]string, arity)
	for i, arg := range call.Args {
		inferredType, ok := s.InferType(arg)
		if !ok {
			s.logger.Debugf("inferTupleType: Failed to infer type for element %d", i)
			return ""
		}
		elementTypeNames[i] = s.TypeToString(inferredType)
	}

	// Generate tuple type name using the same logic as TuplePlugin
	return s.generateTupleTypeName(arity, elementTypeNames)
}

// generateTupleTypeName creates canonical tuple type name
// Must match the logic in tuples.go:generateTypeName
func (s *TypeInferenceService) generateTupleTypeName(arity int, elementTypes []string) string {
	parts := []string{fmt.Sprintf("Tuple%d", arity)}

	for _, typeName := range elementTypes {
		sanitized := s.sanitizeTupleTypeName(typeName)
		capitalized := s.capitalize(sanitized)
		parts = append(parts, capitalized)
	}

	return strings.Join(parts, "")
}

// sanitizeTupleTypeName sanitizes a type name for use in tuple type names
// Must match the logic in tuples.go:sanitizeTupleTypeName
func (s *TypeInferenceService) sanitizeTupleTypeName(typeName string) string {
	// Remove package prefixes
	if idx := strings.LastIndex(typeName, "."); idx != -1 {
		typeName = typeName[idx+1:]
	}

	// Handle pointer types
	if strings.HasPrefix(typeName, "*") {
		return "Ptr" + s.sanitizeTupleTypeName(typeName[1:])
	}

	// Handle slice types
	if strings.HasPrefix(typeName, "[]") {
		return "Slice" + s.sanitizeTupleTypeName(typeName[2:])
	}

	// Handle map types
	if strings.HasPrefix(typeName, "map[") {
		inner := typeName[4:]
		if idx := strings.Index(inner, "]"); idx != -1 {
			keyType := inner[:idx]
			valueType := inner[idx+1:]
			return "Map" + s.capitalize(s.sanitizeTupleTypeName(keyType)) + s.capitalize(s.sanitizeTupleTypeName(valueType))
		}
	}

	// Handle channel types
	if strings.HasPrefix(typeName, "chan ") {
		return "Chan" + s.sanitizeTupleTypeName(typeName[5:])
	}
	if strings.HasPrefix(typeName, "<-chan ") {
		return "RecvChan" + s.sanitizeTupleTypeName(typeName[7:])
	}
	if strings.HasPrefix(typeName, "chan<- ") {
		return "SendChan" + s.sanitizeTupleTypeName(typeName[7:])
	}

	// Handle interface{}
	if typeName == "interface{}" {
		return "Interface"
	}

	// Capitalize built-in types
	switch typeName {
	case "int", "int8", "int16", "int32", "int64":
		return strings.Title(typeName)
	case "uint", "uint8", "uint16", "uint32", "uint64", "uintptr":
		return strings.Title(typeName)
	case "float32", "float64":
		return strings.Title(typeName)
	case "complex64", "complex128":
		return strings.Title(typeName)
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
	default:
		return typeName
	}
}

// capitalize capitalizes the first letter of a string
func (s *TypeInferenceService) capitalize(str string) string {
	if str == "" {
		return str
	}
	return strings.ToUpper(str[:1]) + str[1:]
}
