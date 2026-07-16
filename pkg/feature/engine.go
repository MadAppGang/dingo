package feature

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"sort"
)

// EnabledFeatures defines which features are enabled
type EnabledFeatures map[string]bool

// Engine orchestrates plugin execution with dependency resolution and configuration
type Engine struct {
	// enabled tracks which features are enabled
	enabled EnabledFeatures

	// registry provides shared state between plugins
	registry *SharedRegistry

	// charPlugins are character-level plugins sorted by priority
	charPlugins []Plugin

	// tokenPlugins are token-level plugins sorted by priority
	tokenPlugins []Plugin

	// fileSet for position tracking
	fileSet *token.FileSet
}

// NewEngine creates a new feature engine with the given enabled features.
// If enabled is nil, all registered plugins are enabled by default.
func NewEngine(enabled EnabledFeatures) (*Engine, error) {
	e := &Engine{
		enabled:  enabled,
		registry: NewSharedRegistry(),
		fileSet:  token.NewFileSet(),
	}

	if err := e.loadPlugins(); err != nil {
		return nil, err
	}

	return e, nil
}

// NewEngineAllEnabled creates an engine with all plugins enabled
func NewEngineAllEnabled() (*Engine, error) {
	return NewEngine(nil)
}

// loadPlugins loads and validates all enabled plugins
func (e *Engine) loadPlugins() error {
	allPlugins := ListPlugins()

	// Separate by type and filter by enabled status
	for _, p := range allPlugins {
		if !e.isEnabled(p.Name()) {
			continue
		}

		// Validate dependencies
		if err := e.validateDependencies(p); err != nil {
			return err
		}

		// Check for conflicts
		if err := e.checkConflicts(p); err != nil {
			return err
		}

		// Add to appropriate list
		switch p.Type() {
		case CharacterLevel:
			e.charPlugins = append(e.charPlugins, p)
		case TokenLevel:
			e.tokenPlugins = append(e.tokenPlugins, p)
		}
	}

	// Sort by priority
	sort.Slice(e.charPlugins, func(i, j int) bool {
		return e.charPlugins[i].Priority() < e.charPlugins[j].Priority()
	})
	sort.Slice(e.tokenPlugins, func(i, j int) bool {
		return e.tokenPlugins[i].Priority() < e.tokenPlugins[j].Priority()
	})

	return nil
}

// IsEnabled reports whether a feature is enabled in this engine. The caller
// can use this to gate optional pipeline phases on a particular plugin's
// config without having to walk EnabledPluginNames.
func (e *Engine) IsEnabled(name string) bool {
	return e.isEnabled(name)
}

// isEnabled checks if a feature is enabled
func (e *Engine) isEnabled(name string) bool {
	if e.enabled == nil {
		return true // All enabled by default
	}
	enabled, exists := e.enabled[name]
	if !exists {
		return true // Default to enabled if not specified
	}
	return enabled
}

// validateDependencies checks that all plugin dependencies are satisfied
func (e *Engine) validateDependencies(p Plugin) error {
	deps := p.Dependencies()
	if len(deps) == 0 {
		return nil
	}

	var missing, disabled []string

	for _, dep := range deps {
		plugin, exists := GetPlugin(dep)
		if !exists {
			missing = append(missing, dep)
			continue
		}
		if !e.isEnabled(plugin.Name()) {
			disabled = append(disabled, dep)
		}
	}

	if len(missing) > 0 || len(disabled) > 0 {
		return &DependencyError{
			Plugin:   p.Name(),
			Missing:  missing,
			Disabled: disabled,
		}
	}

	return nil
}

// checkConflicts checks that no conflicting plugins are enabled
func (e *Engine) checkConflicts(p Plugin) error {
	conflicts := p.Conflicts()
	if len(conflicts) == 0 {
		return nil
	}

	var enabled []string
	for _, c := range conflicts {
		if e.isEnabled(c) {
			enabled = append(enabled, c)
		}
	}

	if len(enabled) > 0 {
		return &ConflictError{
			Plugin:    p.Name(),
			Conflicts: enabled,
		}
	}

	return nil
}

// Transform applies all enabled plugins to the source
func (e *Engine) Transform(src []byte, filename string) ([]byte, error) {
	// First, check for disabled syntax
	if err := e.checkDisabledSyntax(src); err != nil {
		return nil, err
	}

	ctx := &Context{
		Registry: e.registry,
		FileSet:  e.fileSet,
		Filename: filename,
	}

	var err error

	// Apply character-level plugins
	for _, p := range e.charPlugins {
		src, err = p.Transform(src, ctx)
		if err != nil {
			return nil, &TransformError{
				Plugin:  p.Name(),
				Phase:   "transform",
				Message: err.Error(),
				Cause:   err,
			}
		}
	}

	// Apply token-level plugins
	for _, p := range e.tokenPlugins {
		src, err = p.Transform(src, ctx)
		if err != nil {
			return nil, &TransformError{
				Plugin:  p.Name(),
				Phase:   "transform",
				Message: err.Error(),
				Cause:   err,
			}
		}
	}

	return src, nil
}

// TransformCharacterLevel applies only character-level plugins
func (e *Engine) TransformCharacterLevel(src []byte, filename string) ([]byte, error) {
	ctx := &Context{
		Registry: e.registry,
		FileSet:  e.fileSet,
		Filename: filename,
	}

	var err error
	for _, p := range e.charPlugins {
		src, err = p.Transform(src, ctx)
		if err != nil {
			return nil, &TransformError{
				Plugin:  p.Name(),
				Phase:   "transform",
				Message: err.Error(),
				Cause:   err,
			}
		}
	}

	return src, nil
}

// TransformTokenLevel applies only token-level plugins
func (e *Engine) TransformTokenLevel(src []byte, filename string) ([]byte, error) {
	ctx := &Context{
		Registry: e.registry,
		FileSet:  e.fileSet,
		Filename: filename,
	}

	var err error
	for _, p := range e.tokenPlugins {
		src, err = p.Transform(src, ctx)
		if err != nil {
			return nil, &TransformError{
				Plugin:  p.Name(),
				Phase:   "transform",
				Message: err.Error(),
				Cause:   err,
			}
		}
	}

	return src, nil
}

// checkDisabledSyntax detects if any disabled feature's syntax is present
func (e *Engine) checkDisabledSyntax(src []byte) error {
	allPlugins := ListPlugins()

	for _, p := range allPlugins {
		if e.isEnabled(p.Name()) {
			continue // Only check disabled plugins
		}

		locations := p.Detect(src)
		if len(locations) > 0 {
			return &DisabledFeatureError{
				Feature:   p.Name(),
				Locations: locations,
				Message:   fmt.Sprintf("feature '%s' is disabled in configuration", p.Name()),
			}
		}
	}

	return nil
}

// Registry returns the shared registry for direct access
func (e *Engine) Registry() *SharedRegistry {
	return e.registry
}

// Validate runs every enabled plugin that implements Validator against the
// post-typecheck Go AST + type info. Plugins access their previously stored
// annotations via the shared registry. Errors are concatenated in plugin
// priority order; the caller decides how to surface them.
//
// Plugins are walked in the same order as Transform (character-level first,
// then token-level), each sorted by Priority. Plugins that do not implement
// Validator are skipped.
func (e *Engine) Validate(fset *token.FileSet, file *ast.File, info *types.Info, filename string) []ValidationError {
	ctx := &ValidateContext{
		FileSet:  fset,
		GoFile:   file,
		TypeInfo: info,
		Registry: e.registry,
		Filename: filename,
	}

	var errs []ValidationError
	collect := func(p Plugin) {
		v, ok := p.(Validator)
		if !ok {
			return
		}
		pluginErrs := v.Validate(ctx)
		for i := range pluginErrs {
			if pluginErrs[i].Plugin == "" {
				pluginErrs[i].Plugin = p.Name()
			}
		}
		errs = append(errs, pluginErrs...)
	}
	for _, p := range e.charPlugins {
		collect(p)
	}
	for _, p := range e.tokenPlugins {
		collect(p)
	}
	return errs
}

// EnabledPluginNames returns the names of all enabled plugins
func (e *Engine) EnabledPluginNames() []string {
	names := make([]string, 0, len(e.charPlugins)+len(e.tokenPlugins))
	for _, p := range e.charPlugins {
		names = append(names, p.Name())
	}
	for _, p := range e.tokenPlugins {
		names = append(names, p.Name())
	}
	return names
}

// --- Default Engine ---

var defaultEngine *Engine

// InitDefaultEngine initializes the default engine with given features
func InitDefaultEngine(enabled EnabledFeatures) error {
	e, err := NewEngine(enabled)
	if err != nil {
		return err
	}
	defaultEngine = e
	return nil
}

// DefaultEngine returns the default engine, initializing if needed
func DefaultEngine() *Engine {
	if defaultEngine == nil {
		var err error
		defaultEngine, err = NewEngineAllEnabled()
		if err != nil {
			panic(fmt.Sprintf("failed to initialize default feature engine: %v", err))
		}
	}
	return defaultEngine
}

// Transform using the default engine
func Transform(src []byte, filename string) ([]byte, error) {
	return DefaultEngine().Transform(src, filename)
}
