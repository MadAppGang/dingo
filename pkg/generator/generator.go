// Package generator generates Go source code from AST
package generator

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/printer"
	"go/token"

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

	// Step 2: Transform AST using plugin pipeline (if configured)
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
	}

	// Step 3: Print AST to Go source code
	var buf bytes.Buffer

	cfg := printer.Config{
		Mode:     printer.TabIndent | printer.UseSpaces,
		Tabwidth: 8,
	}

	if err := cfg.Fprint(&buf, g.fset, transformed); err != nil {
		return nil, fmt.Errorf("failed to print AST: %w", err)
	}

	// Step 4: Format the generated code
	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		// If formatting fails, return unformatted code
		// This helps with debugging malformed output
		if g.logger != nil {
			g.logger.Warn("Failed to format generated code: %v", err)
		}
		return buf.Bytes(), nil
	}

	// Step 5: Inject DINGO:GENERATED markers (post-processing)
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

	return withMarkers, nil
}
