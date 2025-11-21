// Package generator generates Go source code from AST
package generator

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/importer"
	"go/printer"
	"go/token"
	"go/types"
	"strings"

	dingoast "github.com/MadAppGang/dingo/pkg/ast"
	"github.com/MadAppGang/dingo/pkg/plugin"
	"github.com/MadAppGang/dingo/pkg/plugin/builtin"
)

// Generator generates Go source code from a Dingo AST
type Generator struct {
	fset     *token.FileSet
	registry *plugin.Registry
	pipeline *plugin.Pipeline
	logger   plugin.Logger
}

// New creates a new generator with default configuration
func New(fset *token.FileSet) *Generator {
	return &Generator{
		fset:     fset,
		registry: plugin.NewRegistry(),
		logger:   plugin.NewNoOpLogger(), // Silent by default
	}
}

// NewWithPlugins creates a new generator with a custom plugin registry
func NewWithPlugins(fset *token.FileSet, registry *plugin.Registry, logger plugin.Logger) (*Generator, error) {
	if logger == nil {
		logger = plugin.NewNoOpLogger()
	}

	ctx := &plugin.Context{
		FileSet:     fset,
		TypeInfo:    nil, // TODO: Add type information when available
		Config:      &plugin.Config{
			EmitGeneratedMarkers: true, // Default: enabled
		},
		Registry:    registry,
		Logger:      logger,
		CurrentFile: nil, // Will be set during transformation
	}

	pipeline, err := plugin.NewPipeline(registry, ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create plugin pipeline: %w", err)
	}

	// Register built-in plugins in correct order
	// Phase 4 Integration: Added PatternMatchPlugin and NoneContextPlugin
	// Order matters for dependencies:
	// 1. ResultTypePlugin (injects Result types)
	// 2. OptionTypePlugin (injects Option types)
	// 3. PatternMatchPlugin (uses Result/Option types, checks exhaustiveness)
	// 4. NoneContextPlugin (uses parent map and types.Info)
	// 5. UnusedVarsPlugin (cleanup, runs last)

	resultPlugin := builtin.NewResultTypePlugin()
	pipeline.RegisterPlugin(resultPlugin)

	optionPlugin := builtin.NewOptionTypePlugin()
	pipeline.RegisterPlugin(optionPlugin)

	// Tuple plugin for tuple literals and type generation (Phase 8)
	tuplePlugin := builtin.NewTuplePlugin()
	pipeline.RegisterPlugin(tuplePlugin)

	// Lambda type inference plugin (infers lambda parameter types from context)
	lambdaTypeInferencePlugin := builtin.NewLambdaTypeInferencePlugin()
	pipeline.RegisterPlugin(lambdaTypeInferencePlugin)

	// Phase 4 - Pattern matching plugin (Task D, F)
	patternMatchPlugin := builtin.NewPatternMatchPlugin()
	pipeline.RegisterPlugin(patternMatchPlugin)

	// Phase 4 - None context inference plugin (Task E)
	noneContextPlugin := builtin.NewNoneContextPlugin()
	pipeline.RegisterPlugin(noneContextPlugin)

	// Register unused variable handling plugin (runs last)
	unusedVarsPlugin := builtin.NewUnusedVarsPlugin()
	pipeline.RegisterPlugin(unusedVarsPlugin)

	// Inject type inference factory to avoid circular dependency
	pipeline.SetTypeInferenceFactory(func(fsetInterface interface{}, file *ast.File, loggerInterface plugin.Logger) (interface{}, error) {
		fset, ok := fsetInterface.(*token.FileSet)
		if !ok {
			return nil, fmt.Errorf("invalid FileSet type")
		}
		return builtin.NewTypeInferenceService(fset, file, loggerInterface)
	})

	return &Generator{
		fset:     fset,
		registry: registry,
		pipeline: pipeline,
		logger:   logger,
	}, nil
}

// SetLogger sets the logger for the generator
func (g *Generator) SetLogger(logger plugin.Logger) {
	g.logger = logger
}

// Generate converts a Dingo AST to Go source code
func (g *Generator) Generate(file *dingoast.File) ([]byte, error) {
	// Step 1: Set the current file in the pipeline context
	if g.pipeline != nil && g.pipeline.Ctx != nil {
		g.pipeline.Ctx.CurrentFile = file
	}

	// Step 2: Build parent map for context-aware inference (Phase 4 - Task B)
	// This must happen BEFORE type checking and plugin execution
	if g.pipeline != nil && g.pipeline.Ctx != nil {
		g.pipeline.Ctx.BuildParentMap(file.File)
		if g.logger != nil {
			g.logger.Debug("Parent map built successfully")
		}
	}

	// Step 3: Run type checker to populate type information (Fix A5)
	// This enables accurate type inference for plugins
	typesInfo, err := g.runTypeChecker(file.File)
	if err != nil {
		// Type checking failure is not fatal - we can still generate code
		// but type inference will be limited to structural analysis
		if g.logger != nil {
			g.logger.Warn("Type checker failed: %v (continuing with limited type inference)", err)
		}
	} else {
		// Make types.Info available to the pipeline context
		if g.pipeline != nil && g.pipeline.Ctx != nil {
			g.pipeline.Ctx.TypeInfo = typesInfo
			if g.logger != nil {
				g.logger.Debug("Type checker completed successfully")
			}
		}
	}

	// Step 4: Transform AST using plugin pipeline (if configured)
	transformed := file.File
	if g.pipeline != nil {
		var err error
		transformed, err = g.pipeline.Transform(file.File)
		if err != nil {
			return nil, fmt.Errorf("transformation failed: %w", err)
		}

		if g.logger != nil {
			stats := g.pipeline.GetStats()
			g.logger.Debug("Transformation complete: %d/%d plugins executed",
				stats.EnabledPlugins, stats.TotalPlugins)
		}

		// C3 FIX: Check for compile errors from plugins (exhaustiveness, type inference, etc.)
		if g.pipeline.Ctx != nil && g.pipeline.Ctx.HasErrors() {
			errors := g.pipeline.Ctx.GetErrors()
			// Format all errors into a single message
			var errMsg strings.Builder
			errMsg.WriteString("compilation errors detected:\n")
			for _, e := range errors {
				errMsg.WriteString("  - ")
				errMsg.WriteString(e.Error())
				errMsg.WriteString("\n")
			}
			return nil, fmt.Errorf("%s", errMsg.String())
		}
	}

	// Step 5: Print AST to Go source code in correct order
	// Order: package statement -> imports -> injected types -> user declarations
	var buf bytes.Buffer

	cfg := printer.Config{
		Mode:     printer.TabIndent | printer.UseSpaces,
		Tabwidth: 8,
	}

	// 1. Print package statement from main AST
	fmt.Fprintf(&buf, "package %s\n\n", transformed.Name.Name)

	// 2. Print imports from main AST (if any)
	if len(transformed.Imports) > 0 {
		buf.WriteString("import (\n")
		for _, imp := range transformed.Imports {
			if err := cfg.Fprint(&buf, g.fset, imp); err != nil {
				return nil, fmt.Errorf("failed to print import: %w", err)
			}
			buf.WriteString("\n")
		}
		buf.WriteString(")\n\n")
	}

	// 3. Print injected type declarations (if any)
	if g.pipeline != nil {
		injectedAST := g.pipeline.GetInjectedTypesAST()
		if injectedAST != nil && len(injectedAST.Decls) > 0 {
			for _, decl := range injectedAST.Decls {
				if err := cfg.Fprint(&buf, g.fset, decl); err != nil {
					return nil, fmt.Errorf("failed to print injected type declaration: %w", err)
				}
				buf.WriteString("\n")
			}
			buf.WriteString("\n")
		}
	}

	// 4. Print main AST declarations ONLY (skip package/imports - already printed)
	for _, decl := range transformed.Decls {
		// Skip import declarations (already printed in step 2)
		if genDecl, ok := decl.(*ast.GenDecl); ok && genDecl.Tok == token.IMPORT {
			continue
		}

		if err := cfg.Fprint(&buf, g.fset, decl); err != nil {
			return nil, fmt.Errorf("failed to print declaration: %w", err)
		}
		buf.WriteString("\n")
	}

	// Step 6: Format the generated code
	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		// If formatting fails, return unformatted code
		// This helps with debugging malformed output
		if g.logger != nil {
			g.logger.Warn("Failed to format generated code: %v", err)
		}
		return buf.Bytes(), nil
	}

	// Step 7: Inject DINGO:GENERATED markers (post-processing)
	markersEnabled := true // Default
	if g.pipeline != nil && g.pipeline.Ctx != nil && g.pipeline.Ctx.Config != nil {
		markersEnabled = g.pipeline.Ctx.Config.EmitGeneratedMarkers
	}

	injector := NewMarkerInjector(markersEnabled)
	withMarkers, err := injector.InjectMarkers(formatted)
	if err != nil {
		if g.logger != nil {
			g.logger.Warn("Failed to inject markers: %v", err)
		}
		return formatted, nil // Return without markers on error
	}

	// Step 8: Remove extra blank lines around dingo source mapping markers
	cleaned := removeBlankLinesAroundDingoMarkers(withMarkers)

	// Step 9: Remove extra blank lines between top-level declarations
	// This ensures consistent formatting matching golden files
	final := removeBlankLinesBetweenDeclarations(cleaned)

	return final, nil
}

// runTypeChecker runs the Go type checker on the AST
//
// This function performs type checking on the provided AST file to populate
// a types.Info structure with accurate type information. This enables plugins
// to use go/types for precise type inference.
//
// The type checker runs in a limited mode that:
// - Uses the default importer for standard library packages
// - Creates a temporary package scope for the file
// - Gracefully handles errors (incomplete code is common during transpilation)
//
// Returns:
//   - *types.Info containing type information for expressions and identifiers
//   - error if type checking completely fails (warnings are logged, not returned)
func (g *Generator) runTypeChecker(file *ast.File) (*types.Info, error) {
	if file == nil {
		return nil, fmt.Errorf("cannot run type checker on nil file")
	}

	// Create types.Info to store type information
	info := &types.Info{
		Types:      make(map[ast.Expr]types.TypeAndValue),
		Defs:       make(map[*ast.Ident]types.Object),
		Uses:       make(map[*ast.Ident]types.Object),
		Implicits:  make(map[ast.Node]types.Object),
		Selections: make(map[*ast.SelectorExpr]*types.Selection),
		Scopes:     make(map[ast.Node]*types.Scope),
	}

	// Create a Config for the type checker
	conf := &types.Config{
		// Use default importer for standard library packages
		Importer: importer.Default(),

		// Ignore errors - incomplete code is common during transpilation
		// We want partial type information even if there are errors
		Error: func(err error) {
			if g.logger != nil {
				g.logger.Debug("Type checker: %v", err)
			}
		},

		// Don't require complete function bodies
		// This allows type checking of incomplete code
		DisableUnusedImportCheck: true,
	}

	// Determine package name
	pkgName := "main"
	if file.Name != nil {
		pkgName = file.Name.Name
	}

	// Create a package for type checking
	pkg, err := conf.Check(pkgName, g.fset, []*ast.File{file}, info)
	if err != nil {
		// Type checking may fail for incomplete code
		// But we still want the partial type information we collected
		if g.logger != nil {
			g.logger.Debug("Type checking completed with errors: %v", err)
		}
		// Return the info even if there were errors - partial information is useful
		return info, nil
	}

	if g.logger != nil && pkg != nil {
		g.logger.Debug("Type checker: package %q checked successfully", pkg.Name())
	}

	return info, nil
}

// removeBlankLinesAroundDingoMarkers removes extra blank lines before/after // dingo: markers
// The Go formatter (format.Source) tends to add blank lines around comments for readability,
// but we want tight spacing around our source mapping markers.
func removeBlankLinesAroundDingoMarkers(output []byte) []byte {
	lines := strings.Split(string(output), "\n")
	result := []string{}

	for i, line := range lines {
		// Skip blank lines immediately before // dingo:s: or // dingo:e:
		if line == "" && i+1 < len(lines) {
			nextLine := strings.TrimSpace(lines[i+1])
			if strings.HasPrefix(nextLine, "// dingo:s:") || strings.HasPrefix(nextLine, "// dingo:e:") {
				continue
			}
		}

		// Skip blank lines immediately after // dingo:s: or // dingo:e:
		if i > 0 && line == "" {
			prevLine := strings.TrimSpace(lines[i-1])
			if strings.HasPrefix(prevLine, "// dingo:s:") || strings.HasPrefix(prevLine, "// dingo:e:") {
				continue
			}
		}

		result = append(result, line)
	}

	return []byte(strings.Join(result, "\n"))
}

// removeBlankLinesBetweenDeclarations removes blank lines between consecutive func declarations
// This matches the expected formatting in golden test files
func removeBlankLinesBetweenDeclarations(output []byte) []byte {
	lines := strings.Split(string(output), "\n")
	result := []string{}

	for i, line := range lines {
		// Skip blank lines that appear between two consecutive func declarations
		if line == "" && i > 0 && i+1 < len(lines) {
			prevLine := strings.TrimSpace(lines[i-1])
			nextLine := strings.TrimSpace(lines[i+1])

			// Only remove if BOTH prev is "}" after a func AND next starts with "func"
			// Check if previous line is closing brace of a function
			isPrevFuncEnd := prevLine == "}"
			isNextFunc := strings.HasPrefix(nextLine, "func ")

			// Need to verify the "}" is from a function, not a struct/const/type
			// Look backwards to find if there's a "func" keyword before this "}"
			if isPrevFuncEnd && isNextFunc {
				// Scan backwards from i-1 to find if this is a function closing brace
				foundFunc := false
				braceDepth := 1
				for j := i - 1; j >= 0 && braceDepth > 0; j-- {
					jLine := strings.TrimSpace(lines[j])
					braceDepth += strings.Count(jLine, "}") - strings.Count(jLine, "{")
					if strings.Contains(jLine, "func ") || strings.Contains(jLine, "func(") {
						foundFunc = true
						break
					}
				}

				// Only skip the blank line if previous "}" was from a function
				if foundFunc {
					continue
				}
			}
		}

		result = append(result, line)
	}

	return []byte(strings.Join(result, "\n"))
}
