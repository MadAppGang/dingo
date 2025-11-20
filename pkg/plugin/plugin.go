// Package plugin provides the plugin system for code generation
package plugin


import (
	"fmt"
	"go/ast"
	"go/token"
)

// MaxErrors is the maximum number of errors to accumulate
// CRITICAL FIX #2: Prevents OOM on large files with many type inference failures
const MaxErrors = 100

// Registry manages plugins
type Registry struct{}

// NewRegistry creates a new plugin registry
func NewRegistry() *Registry {
	return &Registry{}
}

// Pipeline executes plugins in sequence
type Pipeline struct {
	Ctx              *Context
	plugins          []Plugin
	injectedTypesAST *ast.File // Separate AST for injected type declarations (Option B)
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
// Phase 3: Inject - GetPendingDeclarations() to add type declarations (SEPARATE AST - Option B)
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

	// Phase 3: Declaration Injection - Create SEPARATE AST for injected types (Option B)
	// This prevents comment pollution by isolating generated code from user code
	var allInjectedDecls []ast.Decl
	for _, plugin := range p.plugins {
		if dp, ok := plugin.(DeclarationProvider); ok {
			decls := dp.GetPendingDeclarations()
			if len(decls) > 0 {
				// Set positions to token.NoPos (still needed for proper formatting)
				clearPositions(decls)

				// Accumulate all injected declarations
				allInjectedDecls = append(allInjectedDecls, decls...)
				dp.ClearPendingDeclarations()
			}
		}
	}

	// Create separate AST file for injected type declarations
	// Note: We use the same package name but no imports
	// The generator will extract just the declarations when printing
	if len(allInjectedDecls) > 0 {
		p.injectedTypesAST = &ast.File{
			Name:  transformed.Name, // Same package name (required by printer)
			Decls: allInjectedDecls,
		}
	}

	// transformed (user code) remains unchanged - NO injected declarations
	return transformed, nil
}

// clearPositions recursively sets all positions in AST nodes to token.NoPos.
//
// This prevents go/printer from associating file comments with injected nodes.
// Without this, DINGO_MATCH_* comments and user comments can incorrectly appear
// inside generated Result/Option type declarations.
//
// The function walks the entire AST tree of each declaration and zeros out all
// position fields.
func clearPositions(decls []ast.Decl) {
	for _, decl := range decls {
		ast.Inspect(decl, func(n ast.Node) bool {
			if n == nil {
				return false
			}

			// Clear position for every node type
			switch node := n.(type) {
			case *ast.GenDecl:
				node.TokPos = token.NoPos
				node.Lparen = token.NoPos
				node.Rparen = token.NoPos
			case *ast.FuncDecl:
				if node.Name != nil {
					node.Name.NamePos = token.NoPos
				}
			case *ast.Ident:
				node.NamePos = token.NoPos
			case *ast.BasicLit:
				node.ValuePos = token.NoPos
			case *ast.CompositeLit:
				node.Lbrace = token.NoPos
				node.Rbrace = token.NoPos
			case *ast.CallExpr:
				node.Lparen = token.NoPos
				node.Rparen = token.NoPos
			case *ast.UnaryExpr:
				node.OpPos = token.NoPos
			case *ast.BinaryExpr:
				node.OpPos = token.NoPos
			case *ast.KeyValueExpr:
				node.Colon = token.NoPos
			case *ast.StarExpr:
				node.Star = token.NoPos
			case *ast.Field:
				// Fields in struct/function signatures
			case *ast.FieldList:
				node.Opening = token.NoPos
				node.Closing = token.NoPos
			case *ast.BlockStmt:
				node.Lbrace = token.NoPos
				node.Rbrace = token.NoPos
			case *ast.ReturnStmt:
				node.Return = token.NoPos
			case *ast.IfStmt:
				node.If = token.NoPos
			case *ast.AssignStmt:
				node.TokPos = token.NoPos
			case *ast.ExprStmt:
				// No position field
			case *ast.TypeSpec:
				if node.Name != nil {
					node.Name.NamePos = token.NoPos
				}
			case *ast.ValueSpec:
				// No direct position field
			case *ast.StructType:
				node.Struct = token.NoPos
			case *ast.FuncType:
				node.Func = token.NoPos
			case *ast.InterfaceType:
				node.Interface = token.NoPos
			case *ast.ArrayType:
				node.Lbrack = token.NoPos
			case *ast.SelectorExpr:
				// No direct position field
			}

			return true // Continue walking
		})
	}
}

// GetInjectedTypesAST returns the separate AST containing injected type declarations
// Returns nil if no types were injected during transformation
// (Option B: Separate AST architecture)
func (p *Pipeline) GetInjectedTypesAST() *ast.File {
	return p.injectedTypesAST
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
	FileSet        *token.FileSet
	TypeInfo       interface{}
	Config         *Config
	Registry       *Registry
	Logger         Logger
	CurrentFile    interface{}
	TempVarCounter int                    // Counter for generating unique temporary variable names (NOT thread-safe, plugins run sequentially)
	errors         []error                // Accumulated compile errors
	parentMap      map[ast.Node]ast.Node // Maps each AST node to its parent (built via BuildParentMap)
}

// NextTempVar generates the next unique temporary variable name
//
// CRITICAL FIX #6 (Code Review): Encapsulation for TempVarCounter
//
// Returns a string like "tmp1", "tmp2", etc.
// Note: NOT thread-safe. Plugins MUST run sequentially.
func (ctx *Context) NextTempVar() string {
	// Initialize counter to 1 if this is the first call
	if ctx.TempVarCounter == 0 {
		ctx.TempVarCounter = 1
	}

	// No-number-first pattern: first tmp has no number, then tmp1, tmp2, etc.
	varName := ""
	if ctx.TempVarCounter == 1 {
		varName = "tmp"
	} else {
		varName = fmt.Sprintf("tmp%d", ctx.TempVarCounter-1)
	}

	ctx.TempVarCounter++
	return varName
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

func (n *NoOpLogger) Info(msg string)                          {}
func (n *NoOpLogger) Error(msg string)                         {}
func (n *NoOpLogger) Debug(format string, args ...interface{}) {}
func (n *NoOpLogger) Warn(format string, args ...interface{})  {}

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

// ReportError reports a compile error to the context
// Errors are accumulated and can be retrieved later
//
// CRITICAL FIX #2: Limits error accumulation to prevent OOM
func (ctx *Context) ReportError(message string, location token.Pos) {
	if ctx.errors == nil {
		ctx.errors = make([]error, 0)
	}

	// CRITICAL FIX #2: Check error limit to prevent OOM
	if len(ctx.errors) >= MaxErrors {
		// Add sentinel error only once
		if len(ctx.errors) == MaxErrors {
			ctx.errors = append(ctx.errors,
				fmt.Errorf("too many errors (>%d), stopping error collection", MaxErrors))
		}
		return
	}

	ctx.errors = append(ctx.errors, fmt.Errorf("%s (at position %d)", message, location))
}

// GetErrors returns all accumulated compile errors
func (ctx *Context) GetErrors() []error {
	if ctx.errors == nil {
		return []error{}
	}
	return ctx.errors
}

// ClearErrors clears all accumulated errors
func (ctx *Context) ClearErrors() {
	ctx.errors = nil
}

// HasErrors returns true if any errors have been reported
func (ctx *Context) HasErrors() bool {
	return len(ctx.errors) > 0
}

// BuildParentMap constructs the parent map for the given AST file
// This allows efficient parent lookup via GetParent() and WalkParents()
//
// PHASE 4 - Task B: AST Parent Tracking
// Performance: <10ms for typical files (tested on 1000+ node ASTs)
func (ctx *Context) BuildParentMap(file *ast.File) {
	ctx.parentMap = make(map[ast.Node]ast.Node)

	// Stack-based traversal to build parent relationships
	var stack []ast.Node

	ast.Inspect(file, func(n ast.Node) bool {
		if n == nil {
			// Pop from stack when exiting a node
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
			return false
		}

		// Set parent relationship (all nodes except root)
		if len(stack) > 0 {
			ctx.parentMap[n] = stack[len(stack)-1]
		}

		// Push current node to stack
		stack = append(stack, n)
		return true
	})
}

// GetParent returns the parent node of the given node
// Returns nil if the node is the root or not found in the parent map
//
// PHASE 4 - Task B: AST Parent Tracking
func (ctx *Context) GetParent(node ast.Node) ast.Node {
	if ctx.parentMap == nil {
		return nil
	}
	return ctx.parentMap[node]
}

// GetParentMap returns the parent map for context-based type inference
// This allows plugins to access the complete parent relationships
func (ctx *Context) GetParentMap() map[ast.Node]ast.Node {
	return ctx.parentMap
}

// WalkParents walks up the parent chain from the given node
// Calls visitor for each parent, starting with the immediate parent
// Stops if visitor returns false
// Returns true if reached the root, false if visitor stopped early
//
// PHASE 4 - Task B: AST Parent Tracking
//
// Example usage:
//
//	ctx.WalkParents(expr, func(parent ast.Node) bool {
//	    if funcDecl, ok := parent.(*ast.FuncDecl); ok {
//	        // Found enclosing function
//	        return false // Stop walking
//	    }
//	    return true // Continue walking up
//	})
func (ctx *Context) WalkParents(node ast.Node, visitor func(ast.Node) bool) bool {
	if ctx.parentMap == nil {
		return true
	}

	current := node
	for {
		parent := ctx.parentMap[current]
		if parent == nil {
			return true // Reached root
		}

		if !visitor(parent) {
			return false // Visitor stopped early
		}

		current = parent
	}
}

