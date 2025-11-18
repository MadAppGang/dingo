// Package builtin provides tuple exhaustiveness checking for pattern matching
package builtin

import (
	"fmt"
	"strings"
)

// TupleExhaustivenessChecker checks tuple pattern exhaustiveness
// Uses decision tree algorithm with wildcard catch-all semantics
type TupleExhaustivenessChecker struct {
	arity    int       // Number of tuple elements
	variants []string  // All possible variants (Ok, Err, Some, None)
	patterns [][]string // Patterns from each arm: [["Ok", "Ok"], ["Ok", "Err"], ["Err", "_"]]
}

// NewTupleExhaustivenessChecker creates a new tuple exhaustiveness checker
func NewTupleExhaustivenessChecker(arity int, variants []string, patterns [][]string) *TupleExhaustivenessChecker {
	return &TupleExhaustivenessChecker{
		arity:    arity,
		variants: variants,
		patterns: patterns,
	}
}

// Check performs exhaustiveness checking using decision tree algorithm
// Returns: (isExhaustive, missingPatterns, error)
func (c *TupleExhaustivenessChecker) Check() (bool, []string, error) {
	// Validate arity consistency
	for _, pattern := range c.patterns {
		if len(pattern) != c.arity {
			return false, nil, fmt.Errorf(
				"inconsistent tuple arity: expected %d elements, got %d",
				c.arity, len(pattern),
			)
		}
	}

	// Check if any pattern is all-wildcard: (_, _, _)
	// This makes the match exhaustive immediately
	for _, pattern := range c.patterns {
		if c.isAllWildcard(pattern) {
			return true, nil, nil // Exhaustive
		}
	}

	// Use recursive coverage checking
	missing := c.findMissingPatterns(0, []string{})
	if len(missing) == 0 {
		return true, nil, nil // Exhaustive
	}

	return false, missing, nil
}

// isAllWildcard checks if a pattern is all wildcards
func (c *TupleExhaustivenessChecker) isAllWildcard(pattern []string) bool {
	for _, elem := range pattern {
		if elem != "_" {
			return false
		}
	}
	return true
}

// findMissingPatterns recursively finds missing patterns using decision tree
// position: current tuple element index (0..arity-1)
// prefix: accumulated pattern prefix so far (e.g., ["Ok", "Ok"] for position 2)
// Returns: list of missing complete patterns
func (c *TupleExhaustivenessChecker) findMissingPatterns(position int, prefix []string) []string {
	// Base case: checked all positions
	if position >= c.arity {
		// This is a complete pattern - check if it's covered
		if c.isCovered(prefix) {
			return []string{} // Covered, no missing patterns
		}
		// Not covered - return this pattern as missing
		return []string{c.formatPattern(prefix)}
	}

	// Check if any pattern has wildcard at this position with matching prefix
	// Wildcard at any position covers ALL variants at that position
	if c.hasWildcardAtPosition(position, prefix) {
		// Wildcard covers all variants - continue to next position with any variant
		// We just need to check one path (e.g., first variant)
		if len(c.variants) > 0 {
			newPrefix := append(prefix, c.variants[0])
			return c.findMissingPatterns(position+1, newPrefix)
		}
		return []string{}
	}

	// No wildcard - must check all variants at this position
	var missing []string
	for _, variant := range c.variants {
		newPrefix := append(prefix, variant)
		missing = append(missing, c.findMissingPatterns(position+1, newPrefix)...)
	}

	return missing
}

// hasWildcardAtPosition checks if any pattern has wildcard at given position
// and matches the prefix up to that position
func (c *TupleExhaustivenessChecker) hasWildcardAtPosition(position int, prefix []string) bool {
	for _, pattern := range c.patterns {
		// Check if prefix matches
		if !c.prefixMatches(pattern, prefix) {
			continue
		}

		// Check if this pattern has wildcard at position
		if pattern[position] == "_" {
			return true
		}
	}
	return false
}

// prefixMatches checks if pattern matches prefix
// For each position in prefix, pattern must either match exactly or be wildcard
func (c *TupleExhaustivenessChecker) prefixMatches(pattern []string, prefix []string) bool {
	if len(prefix) > len(pattern) {
		return false
	}

	for i, elem := range prefix {
		if pattern[i] != elem && pattern[i] != "_" {
			return false
		}
	}

	return true
}

// isCovered checks if a complete pattern is covered by any arm
func (c *TupleExhaustivenessChecker) isCovered(pattern []string) bool {
	for _, armPattern := range c.patterns {
		if c.patternCovers(armPattern, pattern) {
			return true
		}
	}
	return false
}

// patternCovers checks if armPattern covers the given pattern
// armPattern can have wildcards that match any variant
func (c *TupleExhaustivenessChecker) patternCovers(armPattern []string, pattern []string) bool {
	if len(armPattern) != len(pattern) {
		return false
	}

	for i := 0; i < len(pattern); i++ {
		// Wildcard matches anything
		if armPattern[i] == "_" {
			continue
		}

		// Must match exactly
		if armPattern[i] != pattern[i] {
			return false
		}
	}

	return true
}

// formatPattern formats a pattern for error messages
// Example: ["Ok", "Err"] → "(Ok, Err)"
func (c *TupleExhaustivenessChecker) formatPattern(pattern []string) string {
	return "(" + strings.Join(pattern, ", ") + ")"
}

// ParseTuplePatterns parses tuple pattern info from DINGO_TUPLE_PATTERN marker
// Example: "(Ok, Ok) | (Ok, Err) | (Err, _)"
// Returns: [][]string{{"Ok", "Ok"}, {"Ok", "Err"}, {"Err", "_"}}
func ParseTuplePatterns(markerValue string) ([][]string, error) {
	// Split on " | " to get individual patterns
	patternStrs := strings.Split(markerValue, " | ")

	patterns := make([][]string, 0, len(patternStrs))
	for _, patternStr := range patternStrs {
		patternStr = strings.TrimSpace(patternStr)

		// Remove outer parens
		if !strings.HasPrefix(patternStr, "(") || !strings.HasSuffix(patternStr, ")") {
			return nil, fmt.Errorf("invalid tuple pattern format: %s", patternStr)
		}
		inner := patternStr[1 : len(patternStr)-1]

		// Split on commas
		elements := strings.Split(inner, ",")
		pattern := make([]string, len(elements))
		for i, elem := range elements {
			pattern[i] = strings.TrimSpace(elem)
		}

		patterns = append(patterns, pattern)
	}

	return patterns, nil
}

// ParseArityFromMarker extracts arity from DINGO_TUPLE_PATTERN marker
// Example: "(Ok, Ok) | (Ok, Err) | (Err, _) | ARITY: 2" → 2
func ParseArityFromMarker(marker string) (int, error) {
	// Look for "ARITY: N" in marker
	arityIdx := strings.Index(marker, "ARITY:")
	if arityIdx == -1 {
		// Fall back to inferring from first pattern
		patterns, err := ParseTuplePatterns(marker)
		if err != nil {
			return 0, err
		}
		if len(patterns) == 0 {
			return 0, fmt.Errorf("no patterns found in marker")
		}
		return len(patterns[0]), nil
	}

	// Extract number after "ARITY:"
	arityStr := strings.TrimSpace(marker[arityIdx+6:])
	var arity int
	_, err := fmt.Sscanf(arityStr, "%d", &arity)
	if err != nil {
		return 0, fmt.Errorf("invalid arity format: %s", arityStr)
	}

	return arity, nil
}
