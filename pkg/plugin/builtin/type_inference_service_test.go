// Package builtin provides built-in Dingo transformation plugins
package builtin

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"testing"
)

// TestNewTypeInferenceService tests creating a new type inference service
func TestNewTypeInferenceService(t *testing.T) {
	fset := token.NewFileSet()
	src := `package main
func main() {
	x := 42
}
`
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("Failed to parse file: %v", err)
	}

	service, err := NewTypeInferenceService(fset, file, nil)
	if err != nil {
		t.Fatalf("Failed to create type inference service: %v", err)
	}

	if service == nil {
		t.Fatal("Expected non-nil service")
	}

	if service.fset != fset {
		t.Error("FileSet not properly stored")
	}

	if !service.cacheEnabled {
		t.Error("Cache should be enabled by default")
	}
}

// TestInferType tests basic type inference with caching
func TestInferType(t *testing.T) {
	fset := token.NewFileSet()
	src := `package main
func main() {
	x := 42
	y := "hello"
	z := 3.14
}
`
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("Failed to parse file: %v", err)
	}

	service, err := NewTypeInferenceService(fset, file, nil)
	if err != nil {
		t.Fatalf("Failed to create type inference service: %v", err)
	}

	// Find the x := 42 expression
	var xExpr ast.Expr
	ast.Inspect(file, func(n ast.Node) bool {
		if bl, ok := n.(*ast.BasicLit); ok && bl.Value == "42" {
			xExpr = bl
			return false
		}
		return true
	})

	if xExpr == nil {
		t.Fatal("Could not find x expression")
	}

	// First call - should miss cache
	typ1, err := service.InferType(xExpr)
	if err != nil {
		t.Fatalf("Failed to infer type: %v", err)
	}

	if typ1 == nil {
		t.Fatal("Expected non-nil type")
	}

	if service.typeChecks != 1 {
		t.Errorf("Expected 1 type check, got %d", service.typeChecks)
	}

	if service.cacheHits != 0 {
		t.Errorf("Expected 0 cache hits, got %d", service.cacheHits)
	}

	// Second call - should hit cache
	typ2, err := service.InferType(xExpr)
	if err != nil {
		t.Fatalf("Failed to infer type on second call: %v", err)
	}

	if typ2 != typ1 {
		t.Error("Expected same type from cache")
	}

	if service.cacheHits != 1 {
		t.Errorf("Expected 1 cache hit, got %d", service.cacheHits)
	}
}

// TestIsPointerType tests pointer type detection
func TestIsPointerType(t *testing.T) {
	fset := token.NewFileSet()
	src := `package main
func main() {
	var x int
	var y *int
}
`
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("Failed to parse file: %v", err)
	}

	service, err := NewTypeInferenceService(fset, file, nil)
	if err != nil {
		t.Fatalf("Failed to create type inference service: %v", err)
	}

	// Test with actual pointer type
	pointerType := types.NewPointer(types.Typ[types.Int])
	if !service.IsPointerType(pointerType) {
		t.Error("Expected pointer type to be detected")
	}

	// Test with non-pointer type
	if service.IsPointerType(types.Typ[types.Int]) {
		t.Error("Expected non-pointer type to not be detected as pointer")
	}
}

// TestIsErrorType tests error type detection
func TestIsErrorType(t *testing.T) {
	fset := token.NewFileSet()
	src := `package main
import "errors"
func main() {
	var err error
	_ = errors.New("test")
}
`
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("Failed to parse file: %v", err)
	}

	service, err := NewTypeInferenceService(fset, file, nil)
	if err != nil {
		t.Fatalf("Failed to create type inference service: %v", err)
	}

	// Get error type from universe
	errorType := types.Universe.Lookup("error").Type()

	if !service.IsErrorType(errorType) {
		t.Error("Expected error type to be detected")
	}

	// Test with non-error type
	if service.IsErrorType(types.Typ[types.Int]) {
		t.Error("Expected int type to not be detected as error")
	}
}

// TestIsGoErrorTuple tests (T, error) tuple detection
func TestIsGoErrorTuple(t *testing.T) {
	fset := token.NewFileSet()
	src := `package main
import "os"
func main() {
	os.ReadFile("test.txt")
}
`
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("Failed to parse file: %v", err)
	}

	service, err := NewTypeInferenceService(fset, file, nil)
	if err != nil {
		t.Fatalf("Failed to create type inference service: %v", err)
	}

	// Create a function signature that returns (int, error)
	intType := types.Typ[types.Int]
	errorType := types.Universe.Lookup("error").Type()

	params := types.NewTuple()
	results := types.NewTuple(
		types.NewVar(token.NoPos, nil, "", intType),
		types.NewVar(token.NoPos, nil, "", errorType),
	)

	sig := types.NewSignature(nil, params, results, false)

	valueType, ok := service.IsGoErrorTuple(sig)
	if !ok {
		t.Error("Expected (int, error) to be detected as Go error tuple")
	}

	if valueType != intType {
		t.Errorf("Expected value type to be int, got %v", valueType)
	}

	// Test with non-error tuple
	results2 := types.NewTuple(
		types.NewVar(token.NoPos, nil, "", intType),
		types.NewVar(token.NoPos, nil, "", types.Typ[types.String]),
	)

	sig2 := types.NewSignature(nil, params, results2, false)

	_, ok = service.IsGoErrorTuple(sig2)
	if ok {
		t.Error("Expected (int, string) to not be detected as Go error tuple")
	}
}

// TestSyntheticTypeRegistry tests synthetic type registration and retrieval
func TestSyntheticTypeRegistry(t *testing.T) {
	fset := token.NewFileSet()
	src := `package main
func main() {}
`
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("Failed to parse file: %v", err)
	}

	service, err := NewTypeInferenceService(fset, file, nil)
	if err != nil {
		t.Fatalf("Failed to create type inference service: %v", err)
	}

	// Register a synthetic type
	typeName := "Result_int_error"
	info := &SyntheticTypeInfo{
		TypeName:   typeName,
		Underlying: nil, // Not testing the actual type here
		GenDecl:    nil,
	}

	service.RegisterSyntheticType(typeName, info)

	// Verify it's registered
	if !service.IsSyntheticType(typeName) {
		t.Error("Expected type to be registered")
	}

	// Retrieve it
	retrieved, ok := service.GetSyntheticType(typeName)
	if !ok {
		t.Error("Expected to retrieve registered type")
	}

	if retrieved.TypeName != typeName {
		t.Errorf("Expected type name %s, got %s", typeName, retrieved.TypeName)
	}

	// Try to get non-existent type
	_, ok = service.GetSyntheticType("NonExistent")
	if ok {
		t.Error("Expected non-existent type to not be found")
	}
}

// TestStats tests statistics collection
func TestStats(t *testing.T) {
	fset := token.NewFileSet()
	src := `package main
func main() {
	x := 42
}
`
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("Failed to parse file: %v", err)
	}

	service, err := NewTypeInferenceService(fset, file, nil)
	if err != nil {
		t.Fatalf("Failed to create type inference service: %v", err)
	}

	// Get initial stats
	stats := service.Stats()
	if stats.TypeChecks != 0 {
		t.Errorf("Expected 0 type checks initially, got %d", stats.TypeChecks)
	}

	// Find and infer a type
	var xExpr ast.Expr
	ast.Inspect(file, func(n ast.Node) bool {
		if bl, ok := n.(*ast.BasicLit); ok && bl.Value == "42" {
			xExpr = bl
			return false
		}
		return true
	})

	// Infer twice (once should hit cache)
	_, _ = service.InferType(xExpr)
	_, _ = service.InferType(xExpr)

	stats = service.Stats()
	if stats.TypeChecks != 1 {
		t.Errorf("Expected 1 type check, got %d", stats.TypeChecks)
	}

	if stats.CacheHits != 1 {
		t.Errorf("Expected 1 cache hit, got %d", stats.CacheHits)
	}

	if stats.CacheSize != 1 {
		t.Errorf("Expected cache size 1, got %d", stats.CacheSize)
	}
}

// TestRefresh tests refreshing type information
func TestRefresh(t *testing.T) {
	fset := token.NewFileSet()
	src := `package main
func main() {
	x := 42
}
`
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("Failed to parse file: %v", err)
	}

	service, err := NewTypeInferenceService(fset, file, nil)
	if err != nil {
		t.Fatalf("Failed to create type inference service: %v", err)
	}

	// Record initial generation
	initialGeneration := service.generation

	// Populate cache
	var xExpr ast.Expr
	ast.Inspect(file, func(n ast.Node) bool {
		if bl, ok := n.(*ast.BasicLit); ok && bl.Value == "42" {
			xExpr = bl
			return false
		}
		return true
	})

	_, _ = service.InferType(xExpr)

	// Verify cache has data
	if len(service.typeCache) == 0 {
		t.Error("Expected cache to have data")
	}

	// Record stats before refresh
	statsBefore := service.Stats()
	if statsBefore.TypeChecks == 0 {
		t.Error("Expected type checks > 0 before refresh")
	}

	// Refresh
	err = service.Refresh(file)
	if err != nil {
		t.Fatalf("Failed to refresh: %v", err)
	}

	// CRITICAL FIX #1: Verify cache was cleared
	if len(service.typeCache) != 0 {
		t.Error("Expected cache to be cleared after refresh")
	}

	// Verify generation counter was incremented
	if service.generation <= initialGeneration {
		t.Errorf("Expected generation to increment, was %d, now %d", initialGeneration, service.generation)
	}

	// Verify statistics were reset
	statsAfter := service.Stats()
	if statsAfter.TypeChecks != 0 {
		t.Errorf("Expected type checks to be reset to 0, got %d", statsAfter.TypeChecks)
	}
	if statsAfter.CacheHits != 0 {
		t.Errorf("Expected cache hits to be reset to 0, got %d", statsAfter.CacheHits)
	}
}

// TestClose tests resource cleanup
func TestClose(t *testing.T) {
	fset := token.NewFileSet()
	src := `package main
func main() {
	x := 42
}
`
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("Failed to parse file: %v", err)
	}

	service, err := NewTypeInferenceService(fset, file, nil)
	if err != nil {
		t.Fatalf("Failed to create type inference service: %v", err)
	}

	// Verify healthy before close
	if !service.IsHealthy() {
		t.Error("Expected service to be healthy before close")
	}

	// Close
	err = service.Close()
	if err != nil {
		t.Fatalf("Failed to close: %v", err)
	}

	// CRITICAL FIX #3: Verify resources were released
	if service.typeCache != nil {
		t.Error("Expected typeCache to be nil after close")
	}

	if service.syntheticTypes != nil {
		t.Error("Expected syntheticTypes to be nil after close")
	}

	// Verify not healthy after close
	if service.IsHealthy() {
		t.Error("Expected service to not be healthy after close")
	}
}

// TestErrorHandling tests error collection and inspection
func TestErrorHandling(t *testing.T) {
	fset := token.NewFileSet()
	// Code with intentional type errors
	src := `package main
func main() {
	x := undefinedVar // This will cause a type error
	y := unknownFunc() // This will cause a type error
}
`
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("Failed to parse file: %v", err)
	}

	service, err := NewTypeInferenceService(fset, file, nil)
	if err != nil {
		t.Fatalf("Failed to create type inference service: %v", err)
	}

	// CRITICAL FIX #2: Verify errors were collected
	if !service.HasErrors() {
		t.Error("Expected HasErrors() to return true for code with type errors")
	}

	errors := service.GetErrors()
	if len(errors) == 0 {
		t.Error("Expected GetErrors() to return collected errors")
	}

	// Service should still be healthy (degraded mode)
	if !service.IsHealthy() {
		t.Error("Expected service to be healthy even with type errors")
	}

	// Clear errors
	service.ClearErrors()
	if service.HasErrors() {
		t.Error("Expected HasErrors() to return false after ClearErrors()")
	}
}

// TestIsHealthy tests health status detection
func TestIsHealthy(t *testing.T) {
	fset := token.NewFileSet()
	src := `package main
func main() {}
`
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("Failed to parse file: %v", err)
	}

	service, err := NewTypeInferenceService(fset, file, nil)
	if err != nil {
		t.Fatalf("Failed to create type inference service: %v", err)
	}

	// Should be healthy initially
	if !service.IsHealthy() {
		t.Error("Expected new service to be healthy")
	}

	// Should remain healthy after refresh
	err = service.Refresh(file)
	if err != nil {
		t.Fatalf("Failed to refresh: %v", err)
	}
	if !service.IsHealthy() {
		t.Error("Expected service to be healthy after refresh")
	}

	// Should be unhealthy after close
	_ = service.Close()
	if service.IsHealthy() {
		t.Error("Expected service to be unhealthy after close")
	}
}

// TestServiceMethodsAfterClose verifies methods don't panic after Close()
func TestServiceMethodsAfterClose(t *testing.T) {
	fset := token.NewFileSet()
	src := `package main
func main() {
	x := 42
}
`
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("Failed to parse file: %v", err)
	}

	service, err := NewTypeInferenceService(fset, file, nil)
	if err != nil {
		t.Fatalf("Failed to create type inference service: %v", err)
	}

	// Find expression to test
	var xExpr ast.Expr
	ast.Inspect(file, func(n ast.Node) bool {
		if bl, ok := n.(*ast.BasicLit); ok && bl.Value == "42" {
			xExpr = bl
			return false
		}
		return true
	})

	// Close the service
	_ = service.Close()

	// CRITICAL FIX #3: Verify methods don't panic after Close()

	// InferType should return error, not panic
	_, err = service.InferType(xExpr)
	if err == nil {
		t.Error("Expected InferType to return error after Close()")
	}

	// IsResultType should return false, not panic
	if T, E, ok := service.IsResultType(types.Typ[types.Int]); ok {
		t.Errorf("Expected IsResultType to return false after Close(), got T=%v E=%v", T, E)
	}

	// IsOptionType should return false, not panic
	if T, ok := service.IsOptionType(types.Typ[types.Int]); ok {
		t.Errorf("Expected IsOptionType to return false after Close(), got T=%v", T)
	}

	// IsPointerType should not panic (doesn't use internal state)
	_ = service.IsPointerType(types.NewPointer(types.Typ[types.Int]))

	// Stats should not panic
	_ = service.Stats()

	// HasErrors should not panic
	_ = service.HasErrors()

	// GetErrors should not panic
	_ = service.GetErrors()
}
