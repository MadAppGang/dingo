package preprocessor

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
)

// Package-level compiled regexes for enum processing
var (
	// Matches: enum Name { ... }
	enumPattern = regexp.MustCompile(`(?s)enum\s+(\w+)\s*\{([^}]*)\}`)

	// Matches unit variant: Variant,
	unitVariantPattern = regexp.MustCompile(`^\s*(\w+)\s*,?\s*$`)

	// Matches struct variant: Variant { field1: type1, field2: type2 }
	structVariantPattern = regexp.MustCompile(`^\s*(\w+)\s*\{\s*([^}]*)\s*\}\s*,?\s*$`)
)

// EnumProcessor transforms enum declarations into Go sum types
type EnumProcessor struct {
	mappings []Mapping
}

// NewEnumProcessor creates a new enum preprocessor
func NewEnumProcessor() *EnumProcessor {
	return &EnumProcessor{
		mappings: []Mapping{},
	}
}

// Name returns the processor name
func (e *EnumProcessor) Name() string {
	return "enum"
}

// Process transforms enum declarations to Go sum types
func (e *EnumProcessor) Process(source []byte) ([]byte, []Mapping, error) {
	e.mappings = []Mapping{}

	// Find all enum declarations using manual parsing (handles nested braces)
	enums := e.findEnumDeclarations(source)
	if len(enums) == 0 {
		// No enums found, return as-is
		return source, nil, nil
	}

	// Process enums in reverse order to maintain correct offsets
	result := source
	for i := len(enums) - 1; i >= 0; i-- {
		enum := enums[i]

		// Parse variants
		variants, err := e.parseVariants(enum.body)
		if err != nil {
			// Lenient error handling - log but continue
			// In a real implementation, we'd use a proper logger
			continue
		}

		// Generate Go sum type
		generated := e.generateSumType(enum.name, variants)

		// Replace enum declaration with generated code
		result = append(result[:enum.start], append([]byte(generated), result[enum.end:]...)...)
	}

	return result, e.mappings, nil
}

// enumDecl represents a parsed enum declaration
type enumDecl struct {
	start int
	end   int
	name  string
	body  string
}

// findEnumDeclarations finds all enum declarations with proper brace matching
func (e *EnumProcessor) findEnumDeclarations(source []byte) []enumDecl {
	decls := []enumDecl{}
	src := string(source)
	pos := 0

	for {
		// Find next "enum" keyword
		idx := strings.Index(src[pos:], "enum")
		if idx == -1 {
			break
		}
		idx += pos

		// Skip if "enum" is part of a larger word
		if idx > 0 && isIdentifierChar(src[idx-1]) {
			pos = idx + 4
			continue
		}
		if idx+4 < len(src) && isIdentifierChar(src[idx+4]) {
			pos = idx + 4
			continue
		}

		// Parse enum name
		nameStart := idx + 4
		for nameStart < len(src) && (src[nameStart] == ' ' || src[nameStart] == '\t' || src[nameStart] == '\n') {
			nameStart++
		}
		if nameStart >= len(src) {
			break
		}

		nameEnd := nameStart
		for nameEnd < len(src) && isIdentifierChar(src[nameEnd]) {
			nameEnd++
		}
		if nameEnd == nameStart {
			pos = idx + 4
			continue
		}

		enumName := src[nameStart:nameEnd]

		// Find opening brace
		braceStart := nameEnd
		for braceStart < len(src) && src[braceStart] != '{' {
			braceStart++
		}
		if braceStart >= len(src) {
			break
		}

		// Find matching closing brace
		braceEnd := e.findMatchingBrace(src, braceStart)
		if braceEnd == -1 {
			pos = idx + 4
			continue
		}

		// Extract body (between braces)
		body := src[braceStart+1 : braceEnd]

		decls = append(decls, enumDecl{
			start: idx,
			end:   braceEnd + 1,
			name:  enumName,
			body:  body,
		})

		pos = braceEnd + 1
	}

	return decls
}

// findMatchingBrace finds the closing brace that matches the opening brace at pos
func (e *EnumProcessor) findMatchingBrace(src string, openPos int) int {
	if openPos >= len(src) || src[openPos] != '{' {
		return -1
	}

	depth := 1
	pos := openPos + 1

	for pos < len(src) && depth > 0 {
		ch := src[pos]
		if ch == '{' {
			depth++
		} else if ch == '}' {
			depth--
		}
		pos++
	}

	if depth == 0 {
		return pos - 1
	}
	return -1
}

// isIdentifierChar checks if a character is valid in an identifier
func isIdentifierChar(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_'
}

// Variant represents a single enum variant
type Variant struct {
	Name   string
	Fields []Field
}

// Field represents a field in a struct variant
type Field struct {
	Name string
	Type string
}

// parseVariants parses the enum body into variants
func (e *EnumProcessor) parseVariants(body string) ([]Variant, error) {
	variants := []Variant{}

	// Split by lines or commas
	lines := strings.Split(body, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}

		// Try to match struct variant first
		if matches := structVariantPattern.FindStringSubmatch(line); matches != nil {
			variantName := matches[1]
			fieldsStr := matches[2]

			fields, err := e.parseFields(fieldsStr)
			if err != nil {
				return nil, fmt.Errorf("failed to parse fields for variant %s: %w", variantName, err)
			}

			variants = append(variants, Variant{
				Name:   variantName,
				Fields: fields,
			})
			continue
		}

		// Try to match unit variant
		if matches := unitVariantPattern.FindStringSubmatch(line); matches != nil {
			variantName := matches[1]
			variants = append(variants, Variant{
				Name:   variantName,
				Fields: nil,
			})
			continue
		}
	}

	if len(variants) == 0 {
		return nil, fmt.Errorf("no variants found")
	}

	return variants, nil
}

// parseFields parses field declarations in a struct variant
// Input: "radius: float64" or "width: float64, height: float64"
func (e *EnumProcessor) parseFields(fieldsStr string) ([]Field, error) {
	fields := []Field{}

	// Split by comma
	parts := strings.Split(fieldsStr, ",")

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Parse: name: type
		colonIdx := strings.Index(part, ":")
		if colonIdx == -1 {
			return nil, fmt.Errorf("invalid field syntax: %s", part)
		}

		fieldName := strings.TrimSpace(part[:colonIdx])
		fieldType := strings.TrimSpace(part[colonIdx+1:])

		fields = append(fields, Field{
			Name: fieldName,
			Type: fieldType,
		})
	}

	return fields, nil
}

// generateSumType generates Go sum type code from enum definition
func (e *EnumProcessor) generateSumType(enumName string, variants []Variant) string {
	var buf bytes.Buffer

	// 1. Generate tag type
	buf.WriteString(fmt.Sprintf("type %sTag uint8\n\n", enumName))

	// 2. Generate tag constants
	buf.WriteString("const (\n")
	for i, variant := range variants {
		if i == 0 {
			buf.WriteString(fmt.Sprintf("\t%sTag_%s %sTag = iota\n", enumName, variant.Name, enumName))
		} else {
			buf.WriteString(fmt.Sprintf("\t%sTag_%s\n", enumName, variant.Name))
		}
	}
	buf.WriteString(")\n\n")

	// 3. Generate struct with tag and fields
	buf.WriteString(fmt.Sprintf("type %s struct {\n", enumName))
	buf.WriteString("\ttag " + enumName + "Tag\n")

	// Add fields for each variant
	for _, variant := range variants {
		if len(variant.Fields) > 0 {
			for _, field := range variant.Fields {
				// Generate field name: variantname_fieldname
				fieldName := strings.ToLower(variant.Name) + "_" + field.Name
				buf.WriteString(fmt.Sprintf("\t%s *%s\n", fieldName, field.Type))
			}
		}
	}
	buf.WriteString("}\n\n")

	// 4. Generate constructor functions
	for _, variant := range variants {
		if len(variant.Fields) == 0 {
			// Unit variant constructor
			buf.WriteString(fmt.Sprintf("func %s_%s() %s {\n", enumName, variant.Name, enumName))
			buf.WriteString(fmt.Sprintf("\treturn %s{tag: %sTag_%s}\n", enumName, enumName, variant.Name))
			buf.WriteString("}\n")
		} else {
			// Struct variant constructor
			params := []string{}
			assignments := []string{}

			for _, field := range variant.Fields {
				params = append(params, fmt.Sprintf("%s %s", field.Name, field.Type))
				fieldName := strings.ToLower(variant.Name) + "_" + field.Name
				assignments = append(assignments, fmt.Sprintf("%s: &%s", fieldName, field.Name))
			}

			buf.WriteString(fmt.Sprintf("func %s_%s(%s) %s {\n",
				enumName, variant.Name, strings.Join(params, ", "), enumName))
			buf.WriteString(fmt.Sprintf("\treturn %s{tag: %sTag_%s, %s}\n",
				enumName, enumName, variant.Name, strings.Join(assignments, ", ")))
			buf.WriteString("}\n")
		}
	}

	// 5. Generate Is* methods
	for _, variant := range variants {
		buf.WriteString(fmt.Sprintf("func (e %s) Is%s() bool {\n", enumName, variant.Name))
		buf.WriteString(fmt.Sprintf("\treturn e.tag == %sTag_%s\n", enumName, variant.Name))
		buf.WriteString("}\n")
	}

	return buf.String()
}
