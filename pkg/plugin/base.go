// Package plugin provides base plugin implementation
package plugin

import (
	"go/ast"
)

// BasePlugin provides common functionality for all plugins
// Embed this in your plugin implementation to get default behavior
type BasePlugin struct {
	name         string
	description  string
	dependencies []string
	enabled      bool
}

// NewBasePlugin creates a new base plugin
func NewBasePlugin(name, description string, dependencies []string) *BasePlugin {
	return &BasePlugin{
		name:         name,
		description:  description,
		dependencies: dependencies,
		enabled:      true, // Enabled by default
	}
}

// Name returns the plugin name
func (p *BasePlugin) Name() string {
	return p.name
}

// Description returns the plugin description
func (p *BasePlugin) Description() string {
	return p.description
}

// Dependencies returns the plugin dependencies
func (p *BasePlugin) Dependencies() []string {
	if p.dependencies == nil {
		return []string{}
	}
	return p.dependencies
}

// Enabled returns whether the plugin is enabled
func (p *BasePlugin) Enabled() bool {
	return p.enabled
}

// SetEnabled sets the plugin enabled state
func (p *BasePlugin) SetEnabled(enabled bool) {
	p.enabled = enabled
}

// Transform is the default implementation (no-op)
// Override this in your plugin implementation
func (p *BasePlugin) Transform(ctx *Context, node ast.Node) (ast.Node, error) {
	return node, nil
}
