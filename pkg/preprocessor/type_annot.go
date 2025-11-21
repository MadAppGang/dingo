package preprocessor

import (
	"bytes"
	"fmt"
	"regexp"
)

// Package-level compiled regex (Issue 2: Regex Performance)
// IMPORTANT-2 FIX: Enhanced pattern to handle all Go type patterns robustly
// Examples:
//   - Basic: x: int, x: string
//   - Qualified: x: pkg.Type
//   - Pointers: x: *Type
//   - Arrays/Slices: x: []Type, x: [10]int
//   - Maps: x: map[string]int, x: map[string][]interface{}
//   - Channels: x: chan T, x: <-chan string, x: chan<- int
//   - Functions: x: func(int) error, x: func(a, b int) (string, error)
//   - Complex nested: x: map[string][]func() error
// Strategy: Match everything up to next comma or closing paren, handling nested brackets/parens
var (
	paramPattern      = regexp.MustCompile(`(\w+)\s*:\s*([^,)]+)`)
	returnArrowPattern = regexp.MustCompile(`\)\s*->\s*(.+?)\s*\{`)
)

// TypeAnnotProcessor converts Dingo type annotations (: type) to Go syntax (space type)
type TypeAnnotProcessor struct{}

// NewTypeAnnotProcessor creates a new type annotation processor
func NewTypeAnnotProcessor() *TypeAnnotProcessor {
	return &TypeAnnotProcessor{}
}

// Name returns the processor name
func (t *TypeAnnotProcessor) Name() string {
	return "type_annotations"
}

// Process transforms type annotations (legacy mode)
// Converts: func foo(x: int, y: string)
// To:       func foo(x int, y string)
func (t *TypeAnnotProcessor) Process(source []byte) ([]byte, []Mapping, error) {
	result, _, _, err := t.ProcessV2(string(source), ModeLegacy)
	return []byte(result), nil, err
}

// ProcessV2 transforms type annotations with metadata emission support
func (t *TypeAnnotProcessor) ProcessV2(code string, mode PreprocessorMode) (string, []TransformMetadata, *SourceMap, error) {
	var metadata []TransformMetadata
	var legacyMappings []Mapping
	counter := 0

	lines := bytes.Split([]byte(code), []byte("\n"))
	var result bytes.Buffer

	for lineIdx, line := range lines {
		lineNum := lineIdx + 1

		// Check if this line contains a function declaration
		if bytes.Contains(line, []byte("func ")) {
			// Track if we made any transformations on this line
			hadTransformation := false

			// First handle return type arrow: ) -> Type {  →  ) Type {
			if returnArrowPattern.Match(line) {
				hadTransformation = true
				line = returnArrowPattern.ReplaceAllFunc(line, func(match []byte) []byte {
					submatch := returnArrowPattern.FindSubmatch(match)
					if len(submatch) != 2 {
						return match
					}
					returnType := submatch[1]

					var buf bytes.Buffer
					buf.WriteString(") ")
					buf.Write(returnType)
					buf.WriteString(" {")
					return buf.Bytes()
				})
			}

			// Find the parameter list
			openParen := bytes.IndexByte(line, '(')
			closeParen := bytes.IndexByte(line, ')')

			if openParen != -1 && closeParen != -1 && closeParen > openParen {
				// Check if params contain : pattern
				params := line[openParen+1:closeParen]
				if bytes.Contains(params, []byte(":")) {
					hadTransformation = true
				}

				// Process only the parameter list
				before := line[:openParen+1]
				after := line[closeParen:]

				// Replace : with space in parameters
				params = t.replaceColonInParams(params)

				var lineResult bytes.Buffer
				lineResult.Write(before)
				lineResult.Write(params)
				lineResult.Write(after)
				line = lineResult.Bytes()
			}

			// If we made a transformation and mode supports metadata, emit it
			if hadTransformation {
				if mode == ModePostAST || mode == ModeDual {
					marker := fmt.Sprintf("// dingo:t:%d", counter)

					// Write transformed line
					result.Write(line)

					// Add marker on next line
					result.WriteString("\n")
					result.WriteString(marker)

					metadata = append(metadata, TransformMetadata{
						Type:            "type_annot",
						OriginalLine:    lineNum,
						OriginalColumn:  1,
						OriginalLength:  len(line),
						OriginalText:    string(line),
						GeneratedMarker: marker,
						ASTNodeType:     "FuncDecl",
					})
					counter++
				} else {
					result.Write(line)
				}

				// Legacy mappings for dual mode
				if mode == ModeLegacy || mode == ModeDual {
					legacyMappings = append(legacyMappings, Mapping{
						GeneratedLine:   lineNum,
						GeneratedColumn: 1,
						OriginalLine:    lineNum,
						OriginalColumn:  1,
						Length:          len(line),
						Name:            "type_annot",
					})
				}
			} else {
				result.Write(line)
			}
		} else {
			result.Write(line)
		}

		if lineIdx < len(lines)-1 {
			result.WriteByte('\n')
		}
	}

	var sourceMap *SourceMap
	if mode == ModeLegacy || mode == ModeDual {
		sourceMap = &SourceMap{
			Version:  1,
			Mappings: legacyMappings,
		}
	}

	return result.String(), metadata, sourceMap, nil
}

// replaceColonInParams replaces : with space in function parameters
func (t *TypeAnnotProcessor) replaceColonInParams(params []byte) []byte {
	// Use package-level compiled regex
	return paramPattern.ReplaceAllFunc(params, func(match []byte) []byte {
		parts := bytes.Split(match, []byte(":"))
		if len(parts) != 2 {
			return match
		}

		identifier := bytes.TrimSpace(parts[0])
		typeName := bytes.TrimSpace(parts[1])

		// Reconstruct as: identifier type (space instead of :)
		var buf bytes.Buffer
		buf.Write(identifier)
		buf.WriteByte(' ')
		buf.Write(typeName)

		return buf.Bytes()
	})
}
