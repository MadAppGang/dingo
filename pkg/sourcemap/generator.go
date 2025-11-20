// Package sourcemap provides source map generation for Dingo → Go transpilation
package sourcemap

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"go/token"

	"github.com/go-sourcemap/sourcemap"
)

// Generator collects position mappings during transpilation and generates source maps
type Generator struct {
	sourceFile string
	genFile    string
	mappings   []Mapping
}

// Mapping represents a single position mapping from Dingo source to Go output
type Mapping struct {
	// Source position (original .dingo file)
	SourceLine   int
	SourceColumn int

	// Generated position (output .go file)
	GenLine   int
	GenColumn int

	// Optional: Name at this position (for identifier mapping)
	Name string
}

// NewGenerator creates a new source map generator
func NewGenerator(sourceFile, genFile string) *Generator {
	return &Generator{
		sourceFile: sourceFile,
		genFile:    genFile,
		mappings:   make([]Mapping, 0),
	}
}

// AddMapping records a position mapping from source to generated code
func (g *Generator) AddMapping(src, gen token.Position) {
	g.mappings = append(g.mappings, Mapping{
		SourceLine:   src.Line,
		SourceColumn: src.Column,
		GenLine:      gen.Line,
		GenColumn:    gen.Column,
	})
}

// AddMappingWithName records a position mapping with an identifier name
func (g *Generator) AddMappingWithName(src, gen token.Position, name string) {
	g.mappings = append(g.mappings, Mapping{
		SourceLine:   src.Line,
		SourceColumn: src.Column,
		GenLine:      gen.Line,
		GenColumn:    gen.Column,
		Name:         name,
	})
}

// SourceMapV3 represents a Source Map v3 JSON structure
type SourceMapV3 struct {
	Version    int      `json:"version"`
	File       string   `json:"file"`
	SourceRoot string   `json:"sourceRoot"`
	Sources    []string `json:"sources"`
	Names      []string `json:"names"`
	Mappings   string   `json:"mappings"`
}

// Generate creates a source map in JSON format with VLQ-encoded mappings
func (g *Generator) Generate() ([]byte, error) {
	// Sort mappings by generated position for correct VLQ encoding
	sortedMappings := make([]Mapping, len(g.mappings))
	copy(sortedMappings, g.mappings)

	// Filter out invalid mappings
	validMappings := make([]Mapping, 0, len(sortedMappings))
	for _, m := range sortedMappings {
		if m.GenLine >= 1 && m.GenColumn >= 1 && m.SourceLine >= 1 && m.SourceColumn >= 1 {
			validMappings = append(validMappings, m)
		}
	}

	// Simple bubble sort by generated line and column
	for i := 0; i < len(validMappings); i++ {
		for j := i + 1; j < len(validMappings); j++ {
			if validMappings[i].GenLine > validMappings[j].GenLine ||
				(validMappings[i].GenLine == validMappings[j].GenLine &&
					validMappings[i].GenColumn > validMappings[j].GenColumn) {
				validMappings[i], validMappings[j] = validMappings[j], validMappings[i]
			}
		}
	}

	// Collect unique names
	names := g.collectUniqueNames(validMappings)

	// Generate VLQ-encoded mappings
	mappings := generateVLQMappings(validMappings)

	// Build source map structure
	sm := SourceMapV3{
		Version:    3,
		File:       g.genFile,
		SourceRoot: "",
		Sources:    []string{g.sourceFile},
		Names:      names,
		Mappings:   mappings,
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(sm, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal source map: %w", err)
	}

	return data, nil
}

// collectUniqueNames extracts unique identifier names from mappings
func (g *Generator) collectUniqueNames(mappings []Mapping) []string {
	nameSet := make(map[string]bool)
	names := make([]string, 0)

	for _, m := range mappings {
		if m.Name != "" && !nameSet[m.Name] {
			nameSet[m.Name] = true
			names = append(names, m.Name)
		}
	}

	return names
}

// GenerateInline creates a base64-encoded inline source map comment
func (g *Generator) GenerateInline() (string, error) {
	data, err := g.Generate()
	if err != nil {
		return "", err
	}

	// Base64 encode
	encoded := base64.StdEncoding.EncodeToString(data)

	// Return as inline comment
	return fmt.Sprintf("//# sourceMappingURL=data:application/json;base64,%s", encoded), nil
}


// Consumer provides source map lookup functionality
type Consumer struct {
	sm *sourcemap.Consumer
}

// NewConsumer creates a source map consumer from raw source map data
func NewConsumer(data []byte) (*Consumer, error) {
	sm, err := sourcemap.Parse("", data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse source map: %w", err)
	}

	return &Consumer{sm: sm}, nil
}

// Source looks up the original source position for a generated position
func (c *Consumer) Source(line, column int) (*token.Position, error) {
	// Note: go-sourcemap uses 0-based indexing, but we use 1-based
	file, _, line, col, ok := c.sm.Source(line-1, column-1)
	if !ok {
		return nil, fmt.Errorf("no mapping found for position %d:%d", line, column)
	}

	return &token.Position{
		Filename: file,
		Line:     line + 1,
		Column:   col + 1,
	}, nil
}
