// Package feature provides a pluggable architecture for Dingo language features.
// Features can be enabled/disabled via configuration and executed in priority order.
package feature

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"sort"
	"sync"
)

// PluginType indicates whether a plugin operates at character or token level
type PluginType int

const (
	// CharacterLevel plugins operate on raw source bytes before tokenization
	CharacterLevel PluginType = iota
	// TokenLevel plugins operate on tokenized source
	TokenLevel
)

// Plugin defines the interface for Dingo language feature plugins.
// Each plugin is responsible for detecting and transforming a specific syntax feature.
type Plugin interface {
	// Name returns the unique identifier for this plugin (e.g., "enum", "match", "lambdas")
	Name() string

	// Version returns the plugin version (e.g., "1.0.0")
	Version() string

	// Type returns whether this plugin operates at character or token level
	Type() PluginType

	// Priority returns the execution priority (lower = runs earlier)
	// Recommended ranges:
	//   10-30: Structural syntax (enum, match)
	//   40-80: Expression syntax (error_prop, lambdas)
	//   100-120: Token transforms (type annotations, generics)
	Priority() int

	// Dependencies returns names of plugins that must run before this one
	Dependencies() []string

	// Conflicts returns names of plugins that cannot be used with this one
	Conflicts() []string

	// Detect checks if this plugin's syntax is present in the source.
	// Used to report errors when disabled syntax is used.
	Detect(src []byte) []SyntaxLocation

	// Transform applies this plugin's transformation to the source.
	// Returns the transformed source and any error.
	Transform(src []byte, ctx *Context) ([]byte, error)
}

// SyntaxLocation identifies where a syntax feature was detected
type SyntaxLocation struct {
	Line    int    // 1-based line number
	Column  int    // 1-based column number
	EndLine int    // End line (for multi-line constructs)
	EndCol  int    // End column
	Snippet string // Short snippet of the detected syntax
}

// Context provides shared state and configuration to plugins during transformation
type Context struct {
	// Registry provides shared state between plugins (e.g., enum definitions)
	Registry *SharedRegistry

	// FileSet for position tracking
	FileSet *token.FileSet

	// Filename being processed
	Filename string

	// Config holds feature-specific configuration
	// This will be populated from dingo.toml
	Config FeatureConfig
}

// FeatureConfig holds per-feature configuration options
type FeatureConfig struct {
	// Feature-specific options will be added here
	// For now, just enable/disable is supported via the registry
}

// SharedRegistry provides thread-safe shared state between plugins
type SharedRegistry struct {
	mu   sync.RWMutex
	data map[string]interface{}
}

// NewSharedRegistry creates a new shared registry
func NewSharedRegistry() *SharedRegistry {
	return &SharedRegistry{
		data: make(map[string]interface{}),
	}
}

// Set stores a value in the registry
func (r *SharedRegistry) Set(key string, value interface{}) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.data[key] = value
}

// Get retrieves a value from the registry
func (r *SharedRegistry) Get(key string) (interface{}, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.data[key]
	return v, ok
}

// GetString retrieves a string value from the registry
func (r *SharedRegistry) GetString(key string) string {
	v, ok := r.Get(key)
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

// GetStringSlice retrieves a string slice from the registry
func (r *SharedRegistry) GetStringSlice(key string) []string {
	v, ok := r.Get(key)
	if !ok {
		return nil
	}
	s, _ := v.([]string)
	return s
}

// GetMap retrieves a map from the registry
func (r *SharedRegistry) GetMap(key string) map[string]interface{} {
	v, ok := r.Get(key)
	if !ok {
		return nil
	}
	m, _ := v.(map[string]interface{})
	return m
}

// Registry keys used by built-in plugins
const (
	// EnumRegistryKey stores enum definitions for use by match plugin
	EnumRegistryKey = "enum_definitions"
)

// Validator is an optional plugin extension. Plugins that need to inspect
// the Go AST + type information AFTER transformation and type checking
// implement this interface in addition to Plugin.
//
// The engine collects every registered, enabled plugin that implements
// Validator and runs Validate in priority order during the post-typecheck
// validation phase. Errors are reported at Dingo source positions (Pos is
// already a token.Pos into the active FileSet; Line/Column are populated
// by Validate for convenience).
type Validator interface {
	// Validate runs after Go AST construction and type checking.
	// The plugin reads its own annotations from ctx.Registry (set during
	// Transform), inspects ctx.GoFile against ctx.TypeInfo, and returns
	// a slice of validation errors. nil / empty slice means "all good".
	Validate(ctx *ValidateContext) []ValidationError
}

// ValidateContext provides the post-AST validation environment to a
// Validator plugin. All fields are read-only for the plugin.
type ValidateContext struct {
	// FileSet is the active token.FileSet for the parsed Go AST.
	FileSet *token.FileSet

	// GoFile is the Go AST after all source transformations.
	GoFile *ast.File

	// TypeInfo is the type checker result for GoFile. May be nil if type
	// inference was disabled or failed; validators should degrade gracefully.
	TypeInfo *types.Info

	// Registry is the shared cross-phase registry. Validators retrieve
	// annotations previously stored by their Transform pass via a
	// plugin-specific key (conventionally "<plugin>_annotations").
	Registry *SharedRegistry

	// Filename is the original Dingo source filename (for diagnostics).
	Filename string
}

// ValidationError is a structured diagnostic from a Validator plugin.
// Pos points at a position in ctx.FileSet; Line/Column are 1-based and
// populated by the validator for convenience.
type ValidationError struct {
	// Plugin is the name of the plugin that produced the error. Plugins
	// may leave this empty; the engine populates it from p.Name() before
	// returning the error to the caller. The pipeline uses it to prefix
	// the diagnostic with a feature-agnostic label (e.g. "<plugin> check
	// error at ...").
	Plugin string

	// Pos points into the active FileSet.
	Pos token.Pos

	// Line is the 1-based line number in the Dingo source.
	Line int

	// Column is the 1-based column number in the Dingo source.
	Column int

	// Message describes the problem (no file path prefix; the engine adds it).
	Message string
}

// Error implements the error interface for ValidationError so callers can
// surface it via the standard error machinery.
func (e *ValidationError) Error() string {
	return fmt.Sprintf("%d:%d: %s", e.Line, e.Column, e.Message)
}

// --- Plugin Registry ---

var (
	pluginsMu sync.RWMutex
	plugins   = make(map[string]Plugin)
)

// Register adds a plugin to the global registry.
// Typically called from init() in plugin packages.
func Register(p Plugin) {
	pluginsMu.Lock()
	defer pluginsMu.Unlock()
	plugins[p.Name()] = p
}

// GetPlugin returns a registered plugin by name
func GetPlugin(name string) (Plugin, bool) {
	pluginsMu.RLock()
	defer pluginsMu.RUnlock()
	p, ok := plugins[name]
	return p, ok
}

// ListPlugins returns all registered plugins sorted by priority
func ListPlugins() []Plugin {
	pluginsMu.RLock()
	defer pluginsMu.RUnlock()

	result := make([]Plugin, 0, len(plugins))
	for _, p := range plugins {
		result = append(result, p)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Priority() < result[j].Priority()
	})

	return result
}

// ListPluginNames returns the names of all registered plugins
func ListPluginNames() []string {
	pluginsMu.RLock()
	defer pluginsMu.RUnlock()

	names := make([]string, 0, len(plugins))
	for name := range plugins {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
