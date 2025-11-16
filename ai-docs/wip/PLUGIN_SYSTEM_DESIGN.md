# Dingo Plugin System Architecture

**Date:** 2025-11-16
**Phase:** 2 (Week 2)
**Status:** Design Complete, Implementation Starting

---

## Overview

The Plugin System provides a modular architecture for implementing Dingo language features. Each feature (Result types, pattern matching, `?` operator, etc.) is implemented as a plugin that can be enabled/disabled and composed into a transformation pipeline.

## Goals

1. **Modularity** - Features are independent, self-contained plugins
2. **Composability** - Plugins can be chained in a pipeline
3. **Enable/Disable** - Users can opt-in/out of features
4. **Extensibility** - Community can add plugins
5. **Clean Architecture** - Separation of concerns between parsing, transformation, and generation

## Architecture

### Plugin Interface

```go
// Plugin represents a Dingo language feature
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
```

### Plugin Context

```go
// Context provides plugins with necessary information
type Context struct {
    FileSet   *token.FileSet       // Source file information
    TypeInfo  *types.Info          // Type information (when available)
    Config    *PluginConfig        // Plugin configuration
    Registry  *PluginRegistry      // Access to other plugins
    Logger    Logger               // Logging interface
}

// PluginConfig holds configuration for all plugins
type PluginConfig struct {
    EnabledPlugins  []string           // List of enabled plugin names
    PluginOptions   map[string]Options // Plugin-specific options
}

// Options is a map of string key-value pairs for plugin configuration
type Options map[string]interface{}
```

### Plugin Registry

```go
// PluginRegistry manages all available plugins
type PluginRegistry struct {
    plugins     map[string]Plugin
    order       []string  // Execution order
}

// Register adds a plugin to the registry
func (r *PluginRegistry) Register(plugin Plugin) error

// Get retrieves a plugin by name
func (r *PluginRegistry) Get(name string) (Plugin, bool)

// All returns all registered plugins
func (r *PluginRegistry) All() []Plugin

// Enabled returns all enabled plugins in execution order
func (r *PluginRegistry) Enabled() []Plugin

// SortByDependencies sorts plugins based on their dependencies
func (r *PluginRegistry) SortByDependencies() error
```

### Transformation Pipeline

```go
// Pipeline executes plugins in order
type Pipeline struct {
    registry *PluginRegistry
    context  *Context
}

// NewPipeline creates a new transformation pipeline
func NewPipeline(registry *PluginRegistry, ctx *Context) *Pipeline

// Transform runs all enabled plugins on the AST
func (p *Pipeline) Transform(file *ast.File) (*ast.File, error)

// TransformNode runs plugins on a specific node
func (p *Pipeline) TransformNode(node ast.Node) (ast.Node, error)
```

## Plugin Types

### 1. Syntax Plugins (Parser Extensions)
Transform Dingo syntax nodes to Go AST.

**Examples:**
- `result-type` - Transforms `Result<T, E>` to Go structs
- `option-type` - Transforms `Option<T>` to Go pointers with validation
- `lambda` - Transforms `|x| x * 2` to Go func literals

### 2. Operator Plugins
Handle special operators.

**Examples:**
- `error-propagation` - Transforms `expr?` to error checking code
- `null-coalescing` - Transforms `a ?? b` to nil checks
- `null-safety` - Transforms `a?.b` to safe navigation

### 3. Statement Plugins
Transform statement-level constructs.

**Examples:**
- `pattern-matching` - Transforms `match` expressions to switch
- `immutability` - Enforces const correctness

### 4. Type System Plugins
Extend or validate the type system.

**Examples:**
- `sum-types` - Implements algebraic data types
- `type-inference` - Enhanced type inference

## Built-in Plugins

### Phase 2 (Week 2)

1. **`result-type`** - Result<T, E> implementation
   - Priority: P0
   - Dependencies: None
   - Status: Pending

2. **`error-propagation`** - `?` operator
   - Priority: P0
   - Dependencies: `result-type`
   - Status: Pending

3. **`option-type`** - Option<T> implementation
   - Priority: P0
   - Dependencies: None
   - Status: Pending

## Configuration

### Via CLI Flags

```bash
# Enable specific plugins
dingo build --plugin=result-type --plugin=error-propagation hello.dingo

# Disable specific plugins
dingo build --no-plugin=immutability hello.dingo

# Enable all plugins
dingo build --all-plugins hello.dingo
```

### Via Configuration File (`.dingorc.json`)

```json
{
  "plugins": {
    "enabled": ["result-type", "error-propagation", "option-type"],
    "disabled": [],
    "config": {
      "result-type": {
        "generate_helpers": true
      },
      "error-propagation": {
        "auto_wrap_errors": true
      }
    }
  }
}
```

### Via Code Comments

```dingo
// dingo:enable result-type,error-propagation
package main

func fetchUser(id: string) -> Result<User, Error> {
    // ...
}
```

## Plugin Lifecycle

1. **Registration** - Plugins register themselves on init()
2. **Discovery** - Registry discovers available plugins
3. **Configuration** - Plugins are enabled/disabled based on config
4. **Dependency Resolution** - Sort plugins by dependencies
5. **Execution** - Pipeline runs plugins in order
6. **Transformation** - Each plugin transforms AST nodes
7. **Code Generation** - Final AST is printed to Go code

## Implementation Plan

### Week 2 - Phase 2

#### Day 1-2: Core Plugin System
- [x] Design plugin architecture (this document)
- [ ] Implement `Plugin` interface
- [ ] Implement `PluginRegistry`
- [ ] Implement `Pipeline`
- [ ] Implement `Context`
- [ ] Write unit tests

#### Day 3-4: Configuration & CLI
- [ ] Add CLI flags for plugin control
- [ ] Implement `.dingorc.json` config file parsing
- [ ] Add plugin listing command: `dingo plugins`
- [ ] Add plugin info command: `dingo plugin info <name>`

#### Day 5-7: First Plugin - Result Type
- [ ] Implement `result-type` plugin
- [ ] Transform `Result<T, E>` to Go structs
- [ ] Generate helper methods (Ok, Err, IsOk, IsErr, etc.)
- [ ] Add tests
- [ ] Update examples

## Benefits

### For Developers
- ✅ **Opt-in Features** - Only pay for what you use
- ✅ **Gradual Migration** - Enable features incrementally
- ✅ **Clean Codebase** - Each feature is isolated
- ✅ **Easy Debugging** - Disable plugins to isolate issues

### For Contributors
- ✅ **Easy to Extend** - Just implement Plugin interface
- ✅ **Clear Structure** - Well-defined plugin boundaries
- ✅ **Testable** - Each plugin can be tested independently
- ✅ **Community Plugins** - Third-party plugins possible

### For Maintainers
- ✅ **Modular Design** - Features don't interfere with each other
- ✅ **Feature Flags** - A/B test new features
- ✅ **Performance** - Disabled plugins have zero overhead
- ✅ **Backward Compatibility** - Old code works by disabling new features

## Example Plugin Implementation

```go
package plugins

import (
    "go/ast"
    "github.com/yourusername/dingo/pkg/plugin"
)

type ErrorPropagationPlugin struct {
    enabled bool
}

func NewErrorPropagationPlugin() *ErrorPropagationPlugin {
    return &ErrorPropagationPlugin{enabled: true}
}

func (p *ErrorPropagationPlugin) Name() string {
    return "error-propagation"
}

func (p *ErrorPropagationPlugin) Description() string {
    return "Transforms ? operator to error checking code"
}

func (p *ErrorPropagationPlugin) Dependencies() []string {
    return []string{"result-type"}
}

func (p *ErrorPropagationPlugin) Transform(ctx *plugin.Context, node ast.Node) (ast.Node, error) {
    // Transform expr? to error checking code
    switch n := node.(type) {
    case *dingoast.ErrorPropagationExpr:
        return p.transformErrorPropagation(ctx, n)
    default:
        return node, nil
    }
}

func (p *ErrorPropagationPlugin) Enabled() bool {
    return p.enabled
}

func (p *ErrorPropagationPlugin) SetEnabled(enabled bool) {
    p.enabled = enabled
}

func (p *ErrorPropagationPlugin) transformErrorPropagation(
    ctx *plugin.Context,
    expr *dingoast.ErrorPropagationExpr,
) (ast.Node, error) {
    // Generate:
    // __result := expr
    // if __result.err != nil {
    //     return Result{err: __result.err}
    // }
    // value := *__result.value

    // ... implementation ...
    return transformedNode, nil
}
```

## Testing Strategy

### Unit Tests
- Test each plugin independently
- Mock Context for isolated testing
- Test edge cases and error conditions

### Integration Tests
- Test plugin combinations
- Test dependency resolution
- Test pipeline execution order

### Golden File Tests
- `.dingo` input files
- Expected `.go` output files
- Compare generated code against expected

## Performance Considerations

1. **Lazy Evaluation** - Only load enabled plugins
2. **AST Reuse** - Transform in-place when possible
3. **Parallel Transformation** - Independent plugins can run in parallel
4. **Caching** - Cache plugin results for unchanged files

## Future Enhancements

### Phase 3+
- **Plugin Marketplace** - Community plugin repository
- **Hot Reload** - Reload plugins without recompiling
- **Plugin Composition** - Combine plugins into presets
- **Plugin Profiles** - "strict", "permissive", "experimental" presets
- **Source Maps** - Track transformations for debugging

## References

- **Babel Plugin System** - JavaScript transpiler architecture
- **Rust Compiler Plugins** - rustc plugin architecture
- **Go Build Tags** - Conditional compilation patterns
- **TypeScript Transformers** - AST transformation API

---

**Next Steps:**
1. Implement core plugin system (2 days)
2. Add CLI configuration (1 day)
3. Build first plugin: `result-type` (2-3 days)
4. Build second plugin: `error-propagation` (1-2 days)
5. Write comprehensive tests (1 day)

**Total: 7-9 days for complete plugin system + first features**
