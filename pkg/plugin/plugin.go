// Package plugin provides the plugin system for Dingo language features
package plugin

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"sort"
)

// Plugin represents a Dingo language feature that transforms AST nodes
type Plugin interface {
	// Name returns the plugin name (e.g., "result-type", "error-propagation")
	Name() string

	// Description returns a human-readable description
	Description() string

	// Dependencies returns list of plugin names this plugin depends on
	// Example: "error-propagation" depends on "result-type"
	Dependencies() []string

	// Transform transforms a Dingo AST node to Go AST
	// Returns the transformed node, or the original if no transformation needed
	Transform(ctx *Context, node ast.Node) (ast.Node, error)

	// Enabled returns whether this plugin is currently enabled
	Enabled() bool

	// SetEnabled enables or disables the plugin
	SetEnabled(bool)
}

// Context provides plugins with necessary information
type Context struct {
	FileSet       *token.FileSet // Source file information
	TypeInfo      *types.Info    // Type information (when available)
	Config        *Config        // Plugin configuration
	Registry      *Registry      // Access to other plugins
	Logger        Logger         // Logging interface
	CurrentFile   ast.Node       // Current file being transformed (can be *dingoast.File)
	DingoConfig   interface{}    // Full Dingo configuration (*config.Config), stored as interface{} to avoid circular import
	TypeInference interface{}    // Shared type inference service (TypeInferenceService), stored as interface{} to avoid circular import
}

// GetDingoConfig safely extracts the Dingo configuration from the context.
// Returns nil if configuration is not available or not the expected type.
// This helper eliminates duplicated type assertion code across all plugins.
func (c *Context) GetDingoConfig() interface{} {
	return c.DingoConfig
}

// Config holds configuration for all plugins
type Config struct {
	EnabledPlugins        []string           // List of enabled plugin names
	PluginOptions         map[string]Options // Plugin-specific options
	EmitGeneratedMarkers  bool               // Whether to emit DINGO:GENERATED markers (default: true)
}

// Options is a map of configuration options for a plugin
type Options map[string]interface{}

// Logger provides logging interface for plugins
type Logger interface {
	Debug(format string, args ...interface{})
	Info(format string, args ...interface{})
	Warn(format string, args ...interface{})
	Error(format string, args ...interface{})
}

// Registry manages all available plugins
type Registry struct {
	plugins map[string]Plugin
	order   []string // Execution order after dependency resolution
}

// NewRegistry creates a new plugin registry
func NewRegistry() *Registry {
	return &Registry{
		plugins: make(map[string]Plugin),
		order:   make([]string, 0),
	}
}

// Register adds a plugin to the registry
func (r *Registry) Register(plugin Plugin) error {
	name := plugin.Name()

	if name == "" {
		return fmt.Errorf("plugin name cannot be empty")
	}

	if _, exists := r.plugins[name]; exists {
		return fmt.Errorf("plugin %q already registered", name)
	}

	r.plugins[name] = plugin
	return nil
}

// Get retrieves a plugin by name
func (r *Registry) Get(name string) (Plugin, bool) {
	plugin, ok := r.plugins[name]
	return plugin, ok
}

// All returns all registered plugins
func (r *Registry) All() []Plugin {
	plugins := make([]Plugin, 0, len(r.plugins))
	for _, plugin := range r.plugins {
		plugins = append(plugins, plugin)
	}
	return plugins
}

// Enabled returns all enabled plugins in execution order
func (r *Registry) Enabled() []Plugin {
	plugins := make([]Plugin, 0)
	for _, name := range r.order {
		if plugin, ok := r.plugins[name]; ok && plugin.Enabled() {
			plugins = append(plugins, plugin)
		}
	}
	return plugins
}

// SortByDependencies sorts plugins based on their dependencies
// Uses topological sort to ensure dependencies are executed before dependents
func (r *Registry) SortByDependencies() error {
	// Build dependency graph (reverse edges)
	// If B depends on A, create edge A -> B
	dependents := make(map[string][]string) // A -> [B, C] means B and C depend on A
	inDegree := make(map[string]int)

	// Initialize all plugins
	for name := range r.plugins {
		if _, exists := inDegree[name]; !exists {
			inDegree[name] = 0
		}
		if _, exists := dependents[name]; !exists {
			dependents[name] = []string{}
		}
	}

	// Build graph
	for name, plugin := range r.plugins {
		deps := plugin.Dependencies()
		inDegree[name] = len(deps)

		for _, dep := range deps {
			// dep is a dependency of name
			// So create edge dep -> name
			dependents[dep] = append(dependents[dep], name)
		}
	}

	// Topological sort using Kahn's algorithm
	queue := make([]string, 0)

	// Find all nodes with no incoming edges (no dependencies)
	for name, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, name)
		}
	}

	// Sort queue for deterministic output
	sort.Strings(queue)

	result := make([]string, 0)

	for len(queue) > 0 {
		// Pop from queue
		node := queue[0]
		queue = queue[1:]
		result = append(result, node)

		// For each dependent of this node
		for _, dependent := range dependents[node] {
			inDegree[dependent]--
			if inDegree[dependent] == 0 {
				queue = append(queue, dependent)
				sort.Strings(queue) // Keep deterministic
			}
		}
	}

	// Check for cycles
	if len(result) != len(r.plugins) {
		return fmt.Errorf("circular dependency detected in plugins")
	}

	r.order = result
	return nil
}

// EnablePlugin enables a plugin by name
func (r *Registry) EnablePlugin(name string) error {
	plugin, ok := r.Get(name)
	if !ok {
		return fmt.Errorf("plugin %q not found", name)
	}

	plugin.SetEnabled(true)

	// Enable dependencies recursively
	for _, dep := range plugin.Dependencies() {
		if err := r.EnablePlugin(dep); err != nil {
			return fmt.Errorf("failed to enable dependency %q: %w", dep, err)
		}
	}

	return nil
}

// DisablePlugin disables a plugin by name
func (r *Registry) DisablePlugin(name string) error {
	plugin, ok := r.Get(name)
	if !ok {
		return fmt.Errorf("plugin %q not found", name)
	}

	plugin.SetEnabled(false)

	// Check if any enabled plugins depend on this one
	for _, p := range r.All() {
		if !p.Enabled() {
			continue
		}
		for _, dep := range p.Dependencies() {
			if dep == name {
				return fmt.Errorf("cannot disable %q: plugin %q depends on it", name, p.Name())
			}
		}
	}

	return nil
}

// List returns a list of all plugin names
func (r *Registry) List() []string {
	names := make([]string, 0, len(r.plugins))
	for name := range r.plugins {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
