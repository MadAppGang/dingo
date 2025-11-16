// Package plugin provides the transformation pipeline
package plugin

import (
	"fmt"
	"go/ast"
)

// Pipeline executes plugins in dependency order
type Pipeline struct {
	registry *Registry
	context  *Context
}

// NewPipeline creates a new transformation pipeline
func NewPipeline(registry *Registry, ctx *Context) (*Pipeline, error) {
	// Sort plugins by dependencies
	if err := registry.SortByDependencies(); err != nil {
		return nil, fmt.Errorf("failed to resolve plugin dependencies: %w", err)
	}

	return &Pipeline{
		registry: registry,
		context:  ctx,
	}, nil
}

// Transform runs all enabled plugins on the AST
func (p *Pipeline) Transform(file *ast.File) (*ast.File, error) {
	if file == nil {
		return nil, fmt.Errorf("file cannot be nil")
	}

	// Get enabled plugins in execution order
	plugins := p.registry.Enabled()

	if len(plugins) == 0 {
		// No plugins enabled, return original file
		return file, nil
	}

	// Log pipeline execution
	if p.context.Logger != nil {
		p.context.Logger.Debug("Running transformation pipeline with %d plugins", len(plugins))
		for _, plugin := range plugins {
			p.context.Logger.Debug("  - %s: %s", plugin.Name(), plugin.Description())
		}
	}

	// Walk the AST and apply transformations
	var err error
	ast.Inspect(file, func(node ast.Node) bool {
		if node == nil || err != nil {
			return false
		}

		// Apply each plugin to this node
		for _, plugin := range plugins {
			var transformed ast.Node
			transformed, err = plugin.Transform(p.context, node)
			if err != nil {
				err = fmt.Errorf("plugin %q failed: %w", plugin.Name(), err)
				return false
			}

			// Update node if transformed
			if transformed != node {
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

	return file, nil
}

// TransformNode runs all enabled plugins on a specific node
func (p *Pipeline) TransformNode(node ast.Node) (ast.Node, error) {
	if node == nil {
		return nil, fmt.Errorf("node cannot be nil")
	}

	plugins := p.registry.Enabled()

	for _, plugin := range plugins {
		transformed, err := plugin.Transform(p.context, node)
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
