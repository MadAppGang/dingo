// Package preprocessor implements the Dingo source code preprocessor
// that transforms Dingo syntax to valid Go syntax with semantic placeholders
package preprocessor

import (
	"encoding/json"
	"fmt"
	"log"
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
	return sm.MapToOriginalWithDebug(line, col, false)
}

// MapToOriginalWithDebug maps a preprocessed position to original Dingo position
// with optional debug logging to help troubleshoot mapping issues
func (sm *SourceMap) MapToOriginalWithDebug(line, col int, debug bool) (int, int) {
	var bestMatch *Mapping = nil
	var minDistanceOnLine int = -1

	if debug {
		fmt.Printf("DEBUG: MapToOriginal(line=%d, col=%d)\n", line, col)
		fmt.Printf("DEBUG: Total mappings: %d\n", len(sm.Mappings))
	}

	// First pass: Look for exact match within mapping range
	for i := range sm.Mappings {
		m := &sm.Mappings[i]
		if m.GeneratedLine == line {
			if debug {
				fmt.Printf("DEBUG: Found mapping on line %d: %s (gen_col=%d, length=%d, orig_col=%d)\n",
					m.GeneratedLine, m.Name, m.GeneratedColumn, m.Length, m.OriginalColumn)
			}

			// Case 1: Exact match within this mapping's range. This is the highest priority.
			if col >= m.GeneratedColumn && col < m.GeneratedColumn+m.Length {
				offset := col - m.GeneratedColumn
				if debug {
					fmt.Printf("DEBUG: EXACT MATCH found in %s, offset=%d\n", m.Name, offset)
					fmt.Printf("DEBUG: Result: orig_line=%d, orig_col=%d\n",
						m.OriginalLine, m.OriginalColumn+offset)
				}
				return m.OriginalLine, m.OriginalColumn + offset
			}

			// Case 2: Update bestMatchOnLine if this mapping is closer to 'col' on the same line.
			currentDistance := abs(m.GeneratedColumn - col)
			if bestMatch == nil || currentDistance < minDistanceOnLine {
				bestMatch = m
				minDistanceOnLine = currentDistance
				if debug {
					fmt.Printf("DEBUG: New bestMatch: %s (distance=%d)\n", m.Name, currentDistance)
				}
			}
		}
	}

	// Second pass: If no exact match, find the best mapping with improved heuristics
	if bestMatch != nil {
		if debug {
			fmt.Printf("DEBUG: No exact match. Best found: %s (distance=%d)\n", bestMatch.Name, minDistanceOnLine)
		}

		// PRIORITY 1: Prefer expr_mapping over error_prop when distances are reasonable
		// This ensures that errors in expressions (like ReadFile) map to the expression, not the ? operator
		if bestMatch.Name == "error_prop" && minDistanceOnLine > 3 {
			if debug {
				fmt.Printf("DEBUG: Searching for better expr_mapping candidate\n")
			}
			// Look for a better expr_mapping candidate
			for _, m := range sm.Mappings {
				if m.GeneratedLine == line && m.Name == "expr_mapping" {
					// Check if this expr_mapping is a better candidate
					exprDistance := abs(m.GeneratedColumn - col)

					if debug {
						fmt.Printf("DEBUG: Checking expr_mapping: gen_col=%d, length=%d, expr_dist=%d\n",
							m.GeneratedColumn, m.Length, exprDistance)
					}

					// Prefer expr_mapping if:
					// 1. It's closer to target column, OR
					// 2. The target column falls within the expression's reasonable range
					if exprDistance < minDistanceOnLine ||
					   (col >= m.GeneratedColumn && col <= m.GeneratedColumn+m.Length+10) {
						// Found a better expr_mapping match
						offset := col - m.GeneratedColumn
						// Ensure offset is reasonable (don't go too far outside of expression)
						if offset >= 0 && offset < m.Length+5 {
							if debug {
								fmt.Printf("DEBUG: BETTER MATCH: using expr_mapping, offset=%d\n", offset)
								fmt.Printf("DEBUG: Result: orig_line=%d, orig_col=%d\n",
									m.OriginalLine, m.OriginalColumn+offset)
							}
							return m.OriginalLine, m.OriginalColumn + offset
						}
					}
				}
			}
			if debug {
				fmt.Printf("DEBUG: No better expr_mapping found\n")
			}
		}

		// PRIORITY 2: Special handling for 'error_prop' mapping
		if bestMatch.Name == "error_prop" {
			if debug {
				fmt.Printf("DEBUG: Using error_prop mapping (points to ? operator)\n")
				fmt.Printf("DEBUG: Result: orig_line=%d, orig_col=%d\n",
					bestMatch.OriginalLine, bestMatch.OriginalColumn)
			}
			// For error_prop, we want to map to the ? operator position specifically
			// This is appropriate for compilation errors that relate to the error handling itself
			return bestMatch.OriginalLine, bestMatch.OriginalColumn
		}

		// PRIORITY 3: Standard mapping with offset calculation for other cases
		offset := col - bestMatch.GeneratedColumn

		if debug {
			fmt.Printf("DEBUG: Standard mapping: offset=%d, length=%d\n", offset, bestMatch.Length)
		}

		// Apply offset only if it's reasonable
		if offset >= 0 && offset < bestMatch.Length+10 {
			// Additional validation: don't create massive offsets that would map far outside of original content
			if offset < 50 { // Reasonable limit to prevent runaway offsets
				if debug {
					fmt.Printf("DEBUG: Applied offset, result: orig_line=%d, orig_col=%d\n",
						bestMatch.OriginalLine, bestMatch.OriginalColumn+offset)
				}
				return bestMatch.OriginalLine, bestMatch.OriginalColumn + offset
			}
		}

		// Fallback: return the start of the best mapping
		if debug {
			fmt.Printf("DEBUG: Fallback: using start of best mapping\n")
			fmt.Printf("DEBUG: Result: orig_line=%d, orig_col=%d\n",
				bestMatch.OriginalLine, bestMatch.OriginalColumn)
		}
		return bestMatch.OriginalLine, bestMatch.OriginalColumn
	}

	if debug {
		fmt.Printf("DEBUG: No relevant mapping found, returning identity mapping\n")
		fmt.Printf("DEBUG: Result: orig_line=%d, orig_col=%d\n", line, col)
	}
	// Final fallback: If no relevant mapping was found, return the identity mapping
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
	log.Printf("DEBUG MapToGenerated: input line=%d, col=%d, total mappings=%d\n", line, col, len(sm.Mappings))

	// Find the mapping that contains this position
	for _, m := range sm.Mappings {
		if m.OriginalLine == line &&
		   col >= m.OriginalColumn &&
		   col < m.OriginalColumn+m.Length {
			// Calculate offset within the mapping
			offset := col - m.OriginalColumn
			log.Printf("DEBUG MapToGenerated: EXACT MATCH, returning line=%d, col=%d\n", m.GeneratedLine, m.GeneratedColumn+offset)
			return m.GeneratedLine, m.GeneratedColumn + offset
		}
	}

	// CRITICAL FIX: No exact mapping found, calculate line offset from existing mappings
	// This handles cases where untransformed code needs to account for added imports, etc.

	// Strategy: Find the closest mapping (before OR after) to determine the line offset
	// Example: If .dingo line 4 maps to .go line 8, then lines 1-3 also have ~+4 offset
	var lineOffset int = 0
	var foundOffset bool = false

	// PRIORITY 1: Find any mapping on the same line to get the offset
	for _, m := range sm.Mappings {
		if m.OriginalLine == line {
			lineOffset = m.GeneratedLine - m.OriginalLine
			foundOffset = true
			log.Printf("DEBUG MapToGenerated: PRIORITY 1 (same line), lineOffset=%d\n", lineOffset)
			break
		}
	}

	// PRIORITY 2: Find the closest earlier mapping
	if !foundOffset {
		var closestBefore *Mapping = nil
		for i := range sm.Mappings {
			m := &sm.Mappings[i]
			if m.OriginalLine < line {
				if closestBefore == nil || m.OriginalLine > closestBefore.OriginalLine {
					closestBefore = m
				}
			}
		}
		if closestBefore != nil {
			lineOffset = closestBefore.GeneratedLine - closestBefore.OriginalLine
			foundOffset = true
			log.Printf("DEBUG MapToGenerated: PRIORITY 2 (closest before), orig=%d, gen=%d, lineOffset=%d\n",
				closestBefore.OriginalLine, closestBefore.GeneratedLine, lineOffset)
		}
	}

	// PRIORITY 3: If no mapping before, look for closest mapping AFTER current line
	// This handles lines before any transformations (e.g., package, imports, func decl before error prop)
	if !foundOffset {
		var closestAfter *Mapping = nil
		for i := range sm.Mappings {
			m := &sm.Mappings[i]
			if m.OriginalLine > line {
				if closestAfter == nil || m.OriginalLine < closestAfter.OriginalLine {
					closestAfter = m
				}
			}
		}
		if closestAfter != nil {
			// Use the offset from the next mapping, assuming structural changes apply to all lines
			lineOffset = closestAfter.GeneratedLine - closestAfter.OriginalLine
			foundOffset = true
			log.Printf("DEBUG MapToGenerated: PRIORITY 3 (closest after), orig=%d, gen=%d, lineOffset=%d\n",
				closestAfter.OriginalLine, closestAfter.GeneratedLine, lineOffset)
		}
	}

	// Apply offset to line, keep column as-is
	if foundOffset {
		log.Printf("DEBUG MapToGenerated: RETURNING line=%d (input %d + offset %d), col=%d\n", line+lineOffset, line, lineOffset, col)
		return line + lineOffset, col
	}

	// Final fallback: return as-is (identity mapping)
	log.Printf("DEBUG MapToGenerated: FALLBACK identity mapping, returning line=%d, col=%d\n", line, col)
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
