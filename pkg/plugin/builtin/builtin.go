// Package builtin provides default plugins
package builtin

import (
	"go/ast"
	"go/token"

	"github.com/MadAppGang/dingo/pkg/plugin"
)

// NewDefaultRegistry creates a registry with default plugins
func NewDefaultRegistry() (*plugin.Registry, error) {
	registry := plugin.NewRegistry()

	// Register Result type plugin
	// Note: Plugin registration will be enhanced in later tasks
	// For now, plugins can be created via NewResultTypePlugin()

	return registry, nil
}

// NewTypeInferenceServiceStub creates a type inference service (stub, deprecated)
// Use NewTypeInferenceService from type_inference.go instead
func NewTypeInferenceServiceStub(fset *token.FileSet, file *ast.File, logger plugin.Logger) (interface{}, error) {
	return nil, nil
}
