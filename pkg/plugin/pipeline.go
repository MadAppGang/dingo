// Package plugin provides the transformation pipeline
package plugin

import (
	"fmt"
	"go/ast"
	"reflect"
)

// TypeInferenceFactory is a function that creates a type inference service
// This pattern avoids circular dependencies between plugin and builtin packages
type TypeInferenceFactory func(fset interface{}, file *ast.File, logger Logger) (interface{}, error)

// Pipeline executes plugins in dependency order
type Pipeline struct {
	registry              *Registry
	Ctx                   *Context // Exported for generator access
	typeInferenceFactory  TypeInferenceFactory
}

// NewPipeline creates a new transformation pipeline
func NewPipeline(registry *Registry, ctx *Context) (*Pipeline, error) {
	// Sort plugins by dependencies
	if err := registry.SortByDependencies(); err != nil {
		return nil, fmt.Errorf("failed to resolve plugin dependencies: %w", err)
	}

	return &Pipeline{
		registry:             registry,
		Ctx:                  ctx,
		typeInferenceFactory: nil, // Will be set by SetTypeInferenceFactory if needed
	}, nil
}

// SetTypeInferenceFactory sets the factory function for creating type inference services
func (p *Pipeline) SetTypeInferenceFactory(factory TypeInferenceFactory) {
	p.typeInferenceFactory = factory
}

// Transform runs all enabled plugins on the AST
func (p *Pipeline) Transform(file *ast.File) (*ast.File, error) {
	// CRITICAL FIX #6: Add nil checks
	if file == nil {
		return nil, fmt.Errorf("file cannot be nil")
	}
	if p.Ctx == nil {
		return nil, fmt.Errorf("pipeline context cannot be nil")
	}

	// Create shared type inference service
	// Note: We import the builtin package to avoid circular dependency, but store as interface{}
	// Plugins will type-assert to *builtin.TypeInferenceService when needed
	typeService, err := p.createTypeInferenceService(file)
	if err != nil {
		if p.Ctx.Logger != nil {
			p.Ctx.Logger.Warn("Type inference initialization failed: %v (continuing without types)", err)
		}
		// Continue without type inference - plugins should degrade gracefully
	} else {
		p.Ctx.TypeInference = typeService
		defer p.closeTypeInferenceService(typeService)
	}

	// Get enabled plugins in execution order
	plugins := p.registry.Enabled()

	if len(plugins) == 0 {
		// No plugins enabled, return original file
		return file, nil
	}

	// Log pipeline execution
	if p.Ctx.Logger != nil {
		p.Ctx.Logger.Debug("Running transformation pipeline with %d plugins", len(plugins))
		for _, plugin := range plugins {
			p.Ctx.Logger.Debug("  - %s: %s", plugin.Name(), plugin.Description())
		}
	}

	// Walk the AST and apply transformations
	ast.Inspect(file, func(node ast.Node) bool {
		if node == nil || err != nil {
			return false
		}

		// Apply each plugin to this node
		for _, plugin := range plugins {
			// CRITICAL FIX #6: Add nil check for plugin
			if plugin == nil {
				if p.Ctx.Logger != nil {
					p.Ctx.Logger.Warn("Encountered nil plugin in pipeline")
				}
				continue
			}

			var transformed ast.Node
			transformed, err = plugin.Transform(p.Ctx, node)
			if err != nil {
				err = fmt.Errorf("plugin %q failed: %w", plugin.Name(), err)
				return false
			}

			// Update node if transformed (with nil check)
			if transformed != nil && transformed != node {
				node = transformed
				// Note: In a real implementation, we'd need to update the parent
				// reference. For now, we're just demonstrating the pattern.
			}
		}

		return true
	})

	if err != nil {
		return nil, err
	}

	// Refresh type information after all transformations
	// This allows later analysis to see types of generated code
	if typeService != nil {
		if err := p.refreshTypeInferenceService(typeService, file); err != nil {
			if p.Ctx.Logger != nil {
				p.Ctx.Logger.Warn("Type refresh after transformations failed: %v", err)
			}
			// Non-fatal - type info may be stale but continue
		}
	}

	return file, nil
}

// TransformNode runs all enabled plugins on a specific node
func (p *Pipeline) TransformNode(node ast.Node) (ast.Node, error) {
	if node == nil {
		return nil, fmt.Errorf("node cannot be nil")
	}

	plugins := p.registry.Enabled()

	for _, plugin := range plugins {
		transformed, err := plugin.Transform(p.Ctx, node)
		if err != nil {
			return nil, fmt.Errorf("plugin %q failed: %w", plugin.Name(), err)
		}
		node = transformed
	}

	return node, nil
}

// Stats returns statistics about the pipeline execution
type Stats struct {
	TotalPlugins   int
	EnabledPlugins int
	PluginNames    []string
	ExecutionOrder []string
}

// GetStats returns pipeline statistics
func (p *Pipeline) GetStats() Stats {
	enabled := p.registry.Enabled()
	names := p.registry.List()
	order := make([]string, 0)

	for _, plugin := range enabled {
		order = append(order, plugin.Name())
	}

	return Stats{
		TotalPlugins:   len(p.registry.All()),
		EnabledPlugins: len(enabled),
		PluginNames:    names,
		ExecutionOrder: order,
	}
}

// createTypeInferenceService creates a new type inference service for the file
// Uses a factory function to avoid circular dependency with builtin package
func (p *Pipeline) createTypeInferenceService(file *ast.File) (interface{}, error) {
	if p.Ctx.FileSet == nil {
		return nil, fmt.Errorf("FileSet is nil")
	}

	// Use injected factory function to create the service
	// This avoids circular import: plugin -> builtin -> plugin
	if p.typeInferenceFactory != nil {
		return p.typeInferenceFactory(p.Ctx.FileSet, file, p.Ctx.Logger)
	}

	// No factory injected, type inference will not be available
	return nil, nil
}

// refreshTypeInferenceService refreshes type information after AST modifications
func (p *Pipeline) refreshTypeInferenceService(serviceInterface interface{}, file *ast.File) error {
	if serviceInterface == nil {
		return nil
	}

	// Use reflection to call Refresh method
	val := reflect.ValueOf(serviceInterface)
	if !val.IsValid() {
		return nil
	}

	refreshMethod := val.MethodByName("Refresh")
	if !refreshMethod.IsValid() {
		return nil
	}

	results := refreshMethod.Call([]reflect.Value{reflect.ValueOf(file)})
	if len(results) > 0 && !results[0].IsNil() {
		if err, ok := results[0].Interface().(error); ok {
			return err
		}
	}

	return nil
}

// closeTypeInferenceService releases resources used by the type inference service
func (p *Pipeline) closeTypeInferenceService(serviceInterface interface{}) {
	if serviceInterface == nil {
		return
	}

	// Use reflection to call Close method
	val := reflect.ValueOf(serviceInterface)
	if !val.IsValid() {
		return
	}

	closeMethod := val.MethodByName("Close")
	if closeMethod.IsValid() {
		closeMethod.Call(nil)
	}
}
