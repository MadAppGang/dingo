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

	// Note: Plugins are registered in NewPipeline, not here
	// This registry is just a placeholder for future plugin discovery

	return registry, nil
}

// RegisterDefaultPlugins registers default plugins to the pipeline
func RegisterDefaultPlugins(pipeline *plugin.Pipeline) {
	// Register tuple plugin for tuple literal and type generation
	pipeline.RegisterPlugin(NewTuplePlugin())
}

// NewTypeInferenceServiceStub creates a type inference service (stub, deprecated)
// Use NewTypeInferenceService from type_inference.go instead
func NewTypeInferenceServiceStub(fset *token.FileSet, file *ast.File, logger plugin.Logger) (interface{}, error) {
	return nil, nil
}
