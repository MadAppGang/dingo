// Package plugin provides the plugin system for code generation
package plugin

import (
	"fmt"
	"go/ast"
	"go/token"
)

// Registry manages plugins
type Registry struct{}

// NewRegistry creates a new plugin registry
func NewRegistry() *Registry {
	return &Registry{}
}

// Pipeline executes plugins in sequence
type Pipeline struct {
	Ctx     *Context
	plugins []Plugin
}

// NewPipeline creates a new plugin pipeline
func NewPipeline(registry *Registry, ctx *Context) (*Pipeline, error) {
	pipeline := &Pipeline{
		Ctx:     ctx,
		plugins: make([]Plugin, 0),
	}

	// Initialize built-in plugins
	// Import the builtin package to get NewResultTypePlugin
	// Note: We'll need to add this import at the top
	return pipeline, nil
}

// RegisterPlugin adds a plugin to the pipeline
func (p *Pipeline) RegisterPlugin(plugin Plugin) {
	p.plugins = append(p.plugins, plugin)

	// Set context if plugin is ContextAware
	if ca, ok := plugin.(ContextAware); ok {
		ca.SetContext(p.Ctx)
	}
}

// Transform transforms an AST using the 3-phase pipeline
// Phase 1: Discovery - Process() to discover types
// Phase 2: Transform - Transform() to replace constructor calls
// Phase 3: Inject - GetPendingDeclarations() to add type declarations
func (p *Pipeline) Transform(file *ast.File) (*ast.File, error) {
	if len(p.plugins) == 0 {
		return file, nil // No plugins, no transformation
	}

	// Phase 1: Discovery - Let plugins analyze the AST
	for _, plugin := range p.plugins {
		if err := plugin.Process(file); err != nil {
			return nil, fmt.Errorf("plugin %s Process failed: %w", plugin.Name(), err)
		}
	}

	// Phase 2: Transformation - Apply AST transformations
	transformed := file
	for _, plugin := range p.plugins {
		if trans, ok := plugin.(Transformer); ok {
			node, err := trans.Transform(transformed)
			if err != nil {
				return nil, fmt.Errorf("plugin %s Transform failed: %w", plugin.Name(), err)
			}
			if node != nil {
				if f, ok := node.(*ast.File); ok {
					transformed = f
				}
			}
		}
	}

	// Phase 3: Declaration Injection - Add pending declarations
	for _, plugin := range p.plugins {
		if dp, ok := plugin.(DeclarationProvider); ok {
			decls := dp.GetPendingDeclarations()
			if len(decls) > 0 {
				// Prepend declarations to the file
				// We put them at the beginning so they're available to all code
				transformed.Decls = append(decls, transformed.Decls...)
				dp.ClearPendingDeclarations()
			}
		}
	}

	return transformed, nil
}

// GetStats returns pipeline stats
func (p *Pipeline) GetStats() Stats {
	return Stats{
		EnabledPlugins: len(p.plugins),
		TotalPlugins:   len(p.plugins),
	}
}

// SetTypeInferenceFactory sets the type inference factory (no-op)
func (p *Pipeline) SetTypeInferenceFactory(f interface{}) {}

// Stats for pipeline execution
type Stats struct {
	EnabledPlugins int
	TotalPlugins   int
}

// Context holds pipeline context
type Context struct {
	FileSet     *token.FileSet
	TypeInfo    interface{}
	Config      *Config
	Registry    *Registry
	Logger      Logger
	CurrentFile interface{}
}

// Config for code generation
type Config struct {
	EmitGeneratedMarkers bool
}

// Logger interface for plugin logging
type Logger interface {
	Info(msg string)
	Error(msg string)
	Debug(format string, args ...interface{})
	Warn(format string, args ...interface{})
}

// NoOpLogger does nothing
type NoOpLogger struct{}

// NewNoOpLogger creates a no-op logger
func NewNoOpLogger() Logger {
	return &NoOpLogger{}
}

func (n *NoOpLogger) Info(msg string)                            {}
func (n *NoOpLogger) Error(msg string)                           {}
func (n *NoOpLogger) Debug(format string, args ...interface{})   {}
func (n *NoOpLogger) Warn(format string, args ...interface{})    {}

// Plugin interface
type Plugin interface {
	Name() string
	Process(node ast.Node) error
}

// ContextAware plugins can receive context information
type ContextAware interface {
	Plugin
	SetContext(ctx *Context)
}

// Transformer plugins can transform AST nodes
type Transformer interface {
	Plugin
	Transform(node ast.Node) (ast.Node, error)
}

// DeclarationProvider plugins can inject package-level declarations
type DeclarationProvider interface {
	Plugin
	GetPendingDeclarations() []ast.Decl
	ClearPendingDeclarations()
}
