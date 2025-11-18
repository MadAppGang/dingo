// Package preprocessor implements the Dingo source code preprocessor
// that transforms Dingo syntax to valid Go syntax with semantic placeholders
package preprocessor

import (
	"encoding/json"
	"fmt"
)

// SourceMap tracks position mappings between original Dingo source
// and preprocessed Go source for error reporting and LSP integration
type SourceMap struct {
	Version  int       `json:"version"`           // Source map format version
	DingoFile string   `json:"dingo_file,omitempty"` // Original .dingo file path
	GoFile    string   `json:"go_file,omitempty"`    // Generated .go file path
	Mappings []Mapping `json:"mappings"`
}

// Mapping represents a single position mapping
type Mapping struct {
	// Preprocessed (generated) position
	GeneratedLine   int `json:"generated_line"`
	GeneratedColumn int `json:"generated_column"`

	// Original (Dingo) position
	OriginalLine    int `json:"original_line"`
	OriginalColumn  int `json:"original_column"`

	// Length of the mapped segment
	Length int `json:"length"`

	// Optional name/description for debugging
	Name string `json:"name,omitempty"`
}

// NewSourceMap creates a new empty source map
func NewSourceMap() *SourceMap {
	return &SourceMap{
		Version:  1, // Current version
		Mappings: make([]Mapping, 0),
	}
}

// AddMapping adds a new position mapping
func (sm *SourceMap) AddMapping(m Mapping) {
	sm.Mappings = append(sm.Mappings, m)
}

// MapToOriginal maps a preprocessed position to the original Dingo position
// Returns the mapped position or the input position if no mapping found
func (sm *SourceMap) MapToOriginal(line, col int) (int, int) {
	// CRITICAL FIX C7: Use column information for disambiguation
	// When multiple mappings exist for same generated line, choose closest column
	var bestMatch *Mapping

	for i := range sm.Mappings {
		m := &sm.Mappings[i]
		if m.GeneratedLine == line {
			// Check if position is within this mapping's range
			if col >= m.GeneratedColumn && col < m.GeneratedColumn+m.Length {
				// Exact match within range
				offset := col - m.GeneratedColumn
				return m.OriginalLine, m.OriginalColumn + offset
			}

			// Track closest mapping for fallback
			if bestMatch == nil {
				bestMatch = m
			} else {
				// Closer column match wins
				currDist := abs(m.GeneratedColumn - col)
				bestDist := abs(bestMatch.GeneratedColumn - col)
				if currDist < bestDist {
					bestMatch = m
				}
			}
		}
	}

	if bestMatch != nil {
		// Calculate offset within mapping
		offset := col - bestMatch.GeneratedColumn
		return bestMatch.OriginalLine, bestMatch.OriginalColumn + offset
	}

	// No mapping found, return as-is
	return line, col
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// MapToGenerated maps an original Dingo position to the preprocessed position
// Returns the mapped position or the input position if no mapping found
func (sm *SourceMap) MapToGenerated(line, col int) (int, int) {
	// Find the mapping that contains this position
	for _, m := range sm.Mappings {
		if m.OriginalLine == line &&
		   col >= m.OriginalColumn &&
		   col < m.OriginalColumn+m.Length {
			// Calculate offset within the mapping
			offset := col - m.OriginalColumn
			return m.GeneratedLine, m.GeneratedColumn + offset
		}
	}

	// No mapping found, return as-is
	return line, col
}

// ToJSON serializes the source map to JSON
func (sm *SourceMap) ToJSON() ([]byte, error) {
	return json.MarshalIndent(sm, "", "  ")
}

// FromJSON deserializes a source map from JSON
func FromJSON(data []byte) (*SourceMap, error) {
	var sm SourceMap
	if err := json.Unmarshal(data, &sm); err != nil {
		return nil, fmt.Errorf("failed to parse source map: %w", err)
	}
	return &sm, nil
}

// Merge combines multiple source maps into one
// Useful when multiple preprocessors run in sequence
func (sm *SourceMap) Merge(other *SourceMap) {
	sm.Mappings = append(sm.Mappings, other.Mappings...)
}
