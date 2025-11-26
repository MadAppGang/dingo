// Package generator generates Go source code from AST
package generator

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/importer"
	"go/parser"
	"go/printer"
	"go/token"
	"go/types"
	"strings"

	dingoast "github.com/MadAppGang/dingo/pkg/ast"
	"github.com/MadAppGang/dingo/pkg/plugin"
	"github.com/MadAppGang/dingo/pkg/plugin/builtin"
	"golang.org/x/tools/go/ast/astutil"
)

const (
	typeString  = "string"
	typeInt     = "int"
	typeFloat64 = "float64"
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

	// Safe navigation plugin (transforms ?. markers to IIFEs)
	safeNavPlugin := builtin.NewSafeNavTypePlugin()
	pipeline.RegisterPlugin(safeNavPlugin)

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

	// CRITICAL FIX: Placeholder resolution plugin
	// This MUST run after type inference plugins but before unused vars
	// Resolves all __INFER__, __UNWRAP__, __IS_SOME__ placeholders
	placeholderResolver := builtin.NewPlaceholderResolverPlugin()
	pipeline.RegisterPlugin(placeholderResolver)

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
			g.logger.Debugf("Parent map built successfully")
		}
	}

	// Step 3: Run type checker to populate type information (Fix A5)
	// This enables accurate type inference for plugins
	typesInfo, err := g.runTypeChecker(file.File)
	if err != nil {
		// Type checking failure is not fatal - we can still generate code
		// but type inference will be limited to structural analysis
		if g.logger != nil {
			g.logger.Warnf("Type checker failed: %v (continuing with limited type inference)", err)
		}
	} else {
		// Make types.Info available to the pipeline context
		if g.pipeline != nil && g.pipeline.Ctx != nil {
			g.pipeline.Ctx.TypeInfo = typesInfo
			if g.logger != nil {
				g.logger.Debugf("Type checker completed successfully")
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
			g.logger.Debugf("Transformation complete: %d/%d plugins executed",
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

	// Step 5: Merge injected type declarations into main AST
	// This ensures go/printer sees all declarations and comments together
	if g.pipeline != nil {
		injectedAST := g.pipeline.GetInjectedTypesAST()
		if injectedAST != nil && len(injectedAST.Decls) > 0 {
			// Insert injected types after imports but before other declarations
			// Find the position to insert (after last import)
			insertPos := 0
			for i, decl := range transformed.Decls {
				if genDecl, ok := decl.(*ast.GenDecl); ok && genDecl.Tok == token.IMPORT {
					insertPos = i + 1
				}
			}

			// Create new declaration slice with injected types
			newDecls := make([]ast.Decl, 0, len(transformed.Decls)+len(injectedAST.Decls))
			newDecls = append(newDecls, transformed.Decls[:insertPos]...)
			newDecls = append(newDecls, injectedAST.Decls...)
			newDecls = append(newDecls, transformed.Decls[insertPos:]...)
			transformed.Decls = newDecls
		}
	}

	// Step 6: Print the ENTIRE AST at once to preserve all comments
	// CRITICAL: Printing the full file preserves file.Comments which includes markers
	// Printing declarations individually loses free-floating comments
	var buf bytes.Buffer
	cfg := printer.Config{
		Mode:     printer.TabIndent | printer.UseSpaces,
		Tabwidth: 8,
	}

	if err := cfg.Fprint(&buf, g.fset, transformed); err != nil {
		return nil, fmt.Errorf("failed to print AST: %w", err)
	}

	// Step 6: CRITICAL FIX - DO NOT use format.Source()
	// format.Source() strips marker comments (// dingo:e:N) which breaks source maps
	// Instead, rely on go/printer's TabIndent mode (already configured above)
	// This preserves all comments while maintaining proper formatting
	//
	// Previous bug: format.Source() removed markers → PostASTGenerator couldn't match transformations
	// Fix: Use printer output directly (still properly formatted via printer.Config)
	printerOutput := buf.Bytes()

	// Step 6.5: Post-AST placeholder resolution
	// This step runs AFTER go/printer to resolve any remaining
	// __INFER__ placeholders that couldn't be resolved during AST transformation.
	// It re-parses the generated .go file and uses full go/types to determine
	// concrete types for generic Option types.
	resolved, err := g.resolvePostASTPlaceholders(printerOutput)
	if err != nil {
		if g.logger != nil {
			g.logger.Warnf("Post-AST placeholder resolution failed: %v (continuing with unresolved placeholders)", err)
		}
		// Continue with unresolved placeholders rather than failing
		resolved = printerOutput
	}

	// Step 7: Inject DINGO:GENERATED markers (post-processing)
	markersEnabled := true // Default
	if g.pipeline != nil && g.pipeline.Ctx != nil && g.pipeline.Ctx.Config != nil {
		markersEnabled = g.pipeline.Ctx.Config.EmitGeneratedMarkers
	}

	injector := NewMarkerInjector(markersEnabled)
	withMarkers, err := injector.InjectMarkers(resolved)
	if err != nil {
		if g.logger != nil {
			g.logger.Warnf("Failed to inject markers: %v", err)
		}
		return resolved, nil // Return without markers on error
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
				g.logger.Debugf("Type checker: %v", err)
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
			g.logger.Debugf("Type checking completed with errors: %v", err)
		}
		// Return the info even if there were errors - partial information is useful
		return info, nil
	}

	if g.logger != nil && pkg != nil {
		g.logger.Debugf("Type checker: package %q checked successfully", pkg.Name())
	}

	return info, nil
}

// resolvePostASTPlaceholders resolves remaining __INFER__ placeholders after go/printer
//
// This function is the Post-AST resolution step that runs AFTER go/printer has
// generated the .go file. It addresses the limitation of the AST-level
// PlaceholderResolverPlugin which cannot resolve types for generic Option types.
//
// The approach:
// 1. Parse the generated .go file (now it's valid Go code)
// 2. Run go/types type checker to get complete type information
// 3. Walk AST to find __INFER__ placeholders
// 4. Resolve types using the type checker's results
// 5. Replace placeholders with concrete types
// 6. Regenerate .go code
//
// This enables resolving cases like:
//   func find() Option { ... }  // Generic Option
//   result := find() ?? 0        // Generates func() __INFER__ { ... }
//
// With Post-AST resolution:
//   - Parse the .go file
//   - Type checker knows find() returns Option
//   - Infer that result should be int (from fallback 0)
//   - Replace __INFER__ with int
func (g *Generator) resolvePostASTPlaceholders(goCode []byte) ([]byte, error) {
	// Count placeholders before resolution
	placeholderCount := strings.Count(string(goCode), "__INFER__")
	if placeholderCount == 0 {
		// No placeholders to resolve
		return goCode, nil
	}

	if g.logger != nil {
		g.logger.Debugf("Post-AST resolution: Found %d __INFER__ placeholders", placeholderCount)
	}

	// Step 1: Parse the generated .go file
	postFset := token.NewFileSet()
	postFile, err := parser.ParseFile(postFset, "generated.go", goCode, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("failed to parse generated Go code: %w", err)
	}

	// Step 2: Run type checker to get complete type information
	postInfo := &types.Info{
		Types:      make(map[ast.Expr]types.TypeAndValue),
		Defs:       make(map[*ast.Ident]types.Object),
		Uses:       make(map[*ast.Ident]types.Object),
		Selections: make(map[*ast.SelectorExpr]*types.Selection),
		Scopes:     make(map[ast.Node]*types.Scope),
	}

	postConf := &types.Config{
		Importer: importer.Default(),
		Error: func(err error) {
			// Ignore errors - __INFER__ placeholders will cause type errors
			// We want partial type information even with errors
			if g.logger != nil {
				g.logger.Debugf("Post-AST type checker: %v", err)
			}
		},
		DisableUnusedImportCheck: true,
	}

	pkgName := "main"
	if postFile.Name != nil {
		pkgName = postFile.Name.Name
	}

	_, _ = postConf.Check(pkgName, postFset, []*ast.File{postFile}, postInfo)
	// Type checking will fail due to __INFER__ placeholders, but we still get partial info
	// We don't return the error - we use the partial information we collected

	// Step 3: Walk AST and resolve __INFER__ placeholders in a single pass
	// Use a stack to track the current FuncLit and its resolved type
	type funcContext struct {
		funcLit      *ast.FuncLit
		resolvedType string
	}
	funcStack := []funcContext{}
	replacements := 0

	modified := astutil.Apply(postFile,
		func(cursor *astutil.Cursor) bool {
			n := cursor.Node()

			// Look for func() __INFER__ patterns and resolve them
			if funcLit, ok := n.(*ast.FuncLit); ok {
				var resolvedType string

				if funcLit.Type != nil && funcLit.Type.Results != nil {
					if len(funcLit.Type.Results.List) == 1 {
						if ident, ok := funcLit.Type.Results.List[0].Type.(*ast.Ident); ok {
							if ident.Name == "__INFER__" {
								// Try to infer the return type from function body
								resolvedType = g.inferFuncLitReturnTypePostAST(funcLit, postInfo)
								if resolvedType != "" {
									// Replace __INFER__ with resolved type
									newField := &ast.Field{
										Type: ast.NewIdent(resolvedType),
									}
									funcLit.Type.Results.List[0] = newField
									replacements++

									if g.logger != nil {
										g.logger.Debugf("Post-AST resolution: Resolved func() __INFER__ → func() %s", resolvedType)
									}
								}
							}
						}
					}
				}

				// Push this FuncLit onto the stack with its resolved type
				funcStack = append(funcStack, funcContext{
					funcLit:      funcLit,
					resolvedType: resolvedType,
				})
			}

			// Look for __INFER___None() and __INFER___Some(val) patterns
			if callExpr, ok := n.(*ast.CallExpr); ok {
				if fun, ok := callExpr.Fun.(*ast.Ident); ok {
					if fun.Name == "__INFER___None" || fun.Name == "__INFER___Some" {
						// Find the containing function literal's resolved type from the stack
						var resolvedType string
						for i := len(funcStack) - 1; i >= 0; i-- {
							if funcStack[i].resolvedType != "" {
								resolvedType = funcStack[i].resolvedType
								break
							}
						}

						if resolvedType != "" {
							// Replace __INFER___None() with ResolvedType_None()
							if fun.Name == "__INFER___None" {
								fun.Name = resolvedType + "_None"
								replacements++
								if g.logger != nil {
									g.logger.Debugf("Post-AST resolution: Resolved __INFER___None() → %s_None()", resolvedType)
								}
							} else {
								fun.Name = resolvedType + "_Some"
								replacements++
								if g.logger != nil {
									g.logger.Debugf("Post-AST resolution: Resolved __INFER___Some() → %s_Some()", resolvedType)
								}
							}
						}
					}
				}
			}

			return true
		},
		func(cursor *astutil.Cursor) bool {
			// Pop FuncLit from stack when leaving
			if _, ok := cursor.Node().(*ast.FuncLit); ok {
				if len(funcStack) > 0 {
					funcStack = funcStack[:len(funcStack)-1]
				}
			}
			return true
		},
	)

	if replacements == 0 {
		// No replacements made - return original code
		if g.logger != nil {
			g.logger.Warnf("Post-AST resolution: Could not resolve any of %d __INFER__ placeholders", placeholderCount)
		}
		return goCode, nil
	}

	if g.logger != nil {
		g.logger.Debugf("Post-AST resolution: Resolved %d/%d placeholders", replacements, placeholderCount)
	}

	// Step 4: Regenerate .go code with resolved types
	var buf bytes.Buffer
	cfg := printer.Config{
		Mode:     printer.TabIndent | printer.UseSpaces,
		Tabwidth: 8,
	}

	if err := cfg.Fprint(&buf, postFset, modified); err != nil {
		return nil, fmt.Errorf("failed to print resolved AST: %w", err)
	}

	// Step 5: Format the resolved code
	resolved, err := format.Source(buf.Bytes())
	if err != nil {
		// Return unformatted if formatting fails
		if g.logger != nil {
			g.logger.Warnf("Post-AST resolution: Failed to format resolved code: %v", err)
		}
		return buf.Bytes(), nil
	}

	return resolved, nil
}

// inferFuncLitReturnTypePostAST infers the return type of a function literal
// using full go/types information (Post-AST approach)
//
// Special handling for safe navigation patterns:
//   func() __INFER__ {
//       if opt.IsNone() { return __INFER___None() }  // Skip this
//       val := opt.Unwrap()
//       return val.field  // Infer from this -> construct FieldOption
//   }
func (g *Generator) inferFuncLitReturnTypePostAST(funcLit *ast.FuncLit, info *types.Info) string {
	if funcLit.Body == nil || len(funcLit.Body.List) == 0 {
		return ""
	}

	// Collect types from non-placeholder returns
	var concreteType string
	var hasPlaceholderReturn bool

	ast.Inspect(funcLit.Body, func(n ast.Node) bool {
		if ret, ok := n.(*ast.ReturnStmt); ok && len(ret.Results) > 0 {
			result := ret.Results[0]

			// Skip __INFER___None() and __INFER___Some() calls
			if callExpr, ok := result.(*ast.CallExpr); ok {
				if fun, ok := callExpr.Fun.(*ast.Ident); ok {
					if strings.HasPrefix(fun.Name, "__INFER__") {
						hasPlaceholderReturn = true
						return true // Skip this return
					}
				}
			}

			// Try to get type from go/types first (most reliable)
			if tv, ok := info.Types[result]; ok && tv.Type != nil {
				typeName := tv.Type.String()
				// Clean up the type name (remove package prefix)
				if idx := strings.LastIndex(typeName, "."); idx >= 0 {
					typeName = typeName[idx+1:]
				}
				concreteType = typeName
				return false // Found concrete type, stop
			}

			// Fallback: AST-based inference for basic types
			switch expr := result.(type) {
			case *ast.BasicLit:
				switch expr.Kind {
				case token.STRING:
					concreteType = typeString
				case token.INT:
					concreteType = typeInt
				case token.FLOAT:
					concreteType = typeFloat64
				case token.CHAR:
					concreteType = "rune"
				}
			case *ast.Ident:
				// Try to get type from identifiers
				if obj, ok := info.Uses[expr]; ok && obj != nil && obj.Type() != nil {
					typeName := obj.Type().String()
					if idx := strings.LastIndex(typeName, "."); idx >= 0 {
						typeName = typeName[idx+1:]
					}
					concreteType = typeName
				}
			case *ast.SelectorExpr:
				// For field access like user.name, try to get the field type
				if tv, ok := info.Types[expr]; ok && tv.Type != nil {
					typeName := tv.Type.String()
					if idx := strings.LastIndex(typeName, "."); idx >= 0 {
						typeName = typeName[idx+1:]
					}
					concreteType = typeName
				} else {
					// AST-only heuristic: Guess type from common field name patterns
					fieldName := expr.Sel.Name
					switch fieldName {
					case "name", "title", "description", "email", "city", "zip", "street", "phone":
						concreteType = "string"
					case "age", "count", "id", "size", "length", "width", "height":
						concreteType = "int"
					}
				}
			}
		}
		return true
	})

	// If we found a concrete type and there's a placeholder return,
	// this is a safe navigation pattern -> construct Option type
	if concreteType != "" && hasPlaceholderReturn {
		// Construct specialized Option type name
		// string -> StringOption, int -> IntOption, etc.
		optionType := g.constructOptionTypeName(concreteType)
		if g.logger != nil {
			g.logger.Debugf("Post-AST: Inferred %s from concrete type %s (safe nav pattern)", optionType, concreteType)
		}
		return optionType
	}

	// If we only found a concrete type (no placeholder), return it as-is
	if concreteType != "" {
		return concreteType
	}

	return ""
}

// constructOptionTypeName constructs the specialized Option type name from a concrete type
// Examples: string -> StringOption, int -> IntOption, User -> UserOption
func (g *Generator) constructOptionTypeName(concreteType string) string {
	// Capitalize first letter for type name
	if concreteType == "" {
		return ""
	}

	// Handle common primitive types
	switch concreteType {
	case "string":
		return "StringOption"
	case "int":
		return "IntOption"
	case "int8":
		return "Int8Option"
	case "int16":
		return "Int16Option"
	case "int32":
		return "Int32Option"
	case "int64":
		return "Int64Option"
	case "uint":
		return "UintOption"
	case "uint8":
		return "Uint8Option"
	case "uint16":
		return "Uint16Option"
	case "uint32":
		return "Uint32Option"
	case "uint64":
		return "Uint64Option"
	case "float32":
		return "Float32Option"
	case "float64":
		return "Float64Option"
	case "bool":
		return "BoolOption"
	case "byte":
		return "ByteOption"
	case "rune":
		return "RuneOption"
	default:
		// For custom types, just append "Option"
		// e.g., User -> UserOption, Address -> AddressOption
		return concreteType + "Option"
	}
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
