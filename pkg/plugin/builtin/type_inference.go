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
// 4. Cache results for performance
type TypeInferenceService struct {
	fset   *token.FileSet
	file   *ast.File
	logger plugin.Logger

	// Cache for type analysis results
	resultTypeCache map[string]*ResultTypeInfo
	optionTypeCache map[string]*OptionTypeInfo

	// Type registry for synthetic types
	registry *TypeRegistry
}

// ResultTypeInfo contains parsed Result type information
type ResultTypeInfo struct {
	TypeName string      // e.g., "Result_int_error"
	OkType   types.Type  // T type parameter
	ErrType  types.Type  // E type parameter
}

// OptionTypeInfo contains parsed Option type information
type OptionTypeInfo struct {
	TypeName  string     // e.g., "Option_int"
	ValueType types.Type // T type parameter
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
		resultTypeCache: make(map[string]*ResultTypeInfo),
		optionTypeCache: make(map[string]*OptionTypeInfo),
		registry:        NewTypeRegistry(),
	}, nil
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
//   Result_int_error → (int, error, true)
//   Result_ptr_User_CustomError → (*User, CustomError, true)
//   Result_slice_byte_error → ([]byte, error, true)
//   NotAResult → (nil, nil, false)
//
// Algorithm:
// 1. Strip "Result_" prefix
// 2. Split by "_" to get tokens
// 3. Parse tokens to reconstruct T and E types
// 4. Handle pointer (ptr_), slice (slice_), map prefixes
func (s *TypeInferenceService) GetResultTypeParams(typeName string) (T, E types.Type, ok bool) {
	if !s.IsResultType(typeName) {
		return nil, nil, false
	}

	// Check cache first
	if cached, found := s.resultTypeCache[typeName]; found {
		return cached.OkType, cached.ErrType, true
	}

	// Parse type name: Result_{T}_{E}
	parts := strings.TrimPrefix(typeName, "Result_")
	tokens := strings.Split(parts, "_")

	if len(tokens) < 2 {
		s.logger.Warn("Invalid Result type name: %s (expected at least 2 tokens)", typeName)
		return nil, nil, false
	}

	// Parse T and E types
	// Strategy: Find the split point between T and E
	// E is typically a simple error type, so we work backwards

	// Simple heuristic: Last token is likely the error type
	// More complex: Handle composite types

	eType, eTokens := s.parseTypeFromTokensBackward(tokens)
	tTokens := tokens[:len(tokens)-eTokens]
	tType, _ := s.parseTypeFromTokensForward(tTokens)

	// Cache the result
	info := &ResultTypeInfo{
		TypeName: typeName,
		OkType:   tType,
		ErrType:  eType,
	}
	s.resultTypeCache[typeName] = info
	s.registry.resultTypes[typeName] = info

	s.logger.Debug("Parsed Result type: %s → T=%v, E=%v", typeName, tType, eType)

	return tType, eType, true
}

// GetOptionTypeParam extracts the type parameter from Option type name
//
// Examples:
//   Option_int → (int, true)
//   Option_ptr_User → (*User, true)
//   Option_slice_byte → ([]byte, true)
//   NotAnOption → (nil, false)
func (s *TypeInferenceService) GetOptionTypeParam(typeName string) (T types.Type, ok bool) {
	if !s.IsOptionType(typeName) {
		return nil, false
	}

	// Check cache first
	if cached, found := s.optionTypeCache[typeName]; found {
		return cached.ValueType, true
	}

	// Parse type name: Option_{T}
	parts := strings.TrimPrefix(typeName, "Option_")
	tokens := strings.Split(parts, "_")

	if len(tokens) < 1 {
		s.logger.Warn("Invalid Option type name: %s", typeName)
		return nil, false
	}

	// Parse T type
	tType, _ := s.parseTypeFromTokensForward(tokens)

	// Cache the result
	info := &OptionTypeInfo{
		TypeName:  typeName,
		ValueType: tType,
	}
	s.optionTypeCache[typeName] = info
	s.registry.optionTypes[typeName] = info

	s.logger.Debug("Parsed Option type: %s → T=%v", typeName, tType)

	return tType, true
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

// InferTypeFromContext attempts to infer type from surrounding context
//
// Checks:
// 1. Assignment statements (let x: Result<int, error> = ...)
// 2. Function return types
// 3. Variable declarations with explicit types
// 4. Function call arguments with typed parameters
func (s *TypeInferenceService) InferTypeFromContext(node ast.Node) (types.Type, bool) {
	// This is a placeholder for context-based type inference
	// Full implementation would use go/types.Info and walk the AST

	s.logger.Debug("InferTypeFromContext called for node type: %T", node)

	// TODO: Implement full context inference
	// For now, return nil to indicate inference failed
	return nil, false
}

// RegisterResultType registers a Result type in the type registry
func (s *TypeInferenceService) RegisterResultType(typeName string, okType, errType types.Type) {
	info := &ResultTypeInfo{
		TypeName: typeName,
		OkType:   okType,
		ErrType:  errType,
	}
	s.resultTypeCache[typeName] = info
	s.registry.resultTypes[typeName] = info

	s.logger.Debug("Registered Result type: %s (T=%v, E=%v)", typeName, okType, errType)
}

// RegisterOptionType registers an Option type in the type registry
func (s *TypeInferenceService) RegisterOptionType(typeName string, valueType types.Type) {
	info := &OptionTypeInfo{
		TypeName:  typeName,
		ValueType: valueType,
	}
	s.optionTypeCache[typeName] = info
	s.registry.optionTypes[typeName] = info

	s.logger.Debug("Registered Option type: %s (T=%v)", typeName, valueType)
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

	s.logger.Debug("ValidateNoneInference called for expr at pos %v", s.fset.Position(noneExpr.Pos()))

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
