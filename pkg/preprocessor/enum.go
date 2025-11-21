package preprocessor

import (
	"bytes"
	"fmt"
	"regexp"
	"sort"
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

	// Matches tuple variant: Variant(type1, type2, ...)
	tupleVariantPattern = regexp.MustCompile(`^\s*(\w+)\s*\(([^)]*)\)\s*,?\s*$`)
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

// Process transforms enum declarations to Go sum types (legacy mode)
func (e *EnumProcessor) Process(source []byte) ([]byte, []Mapping, error) {
	result, _, _, err := e.ProcessV2(string(source), ModeLegacy)
	return []byte(result), nil, err
}

// ProcessV2 implements Post-AST source map support for enum transformations
func (e *EnumProcessor) ProcessV2(code string, mode PreprocessorMode) (string, []TransformMetadata, *SourceMap, error) {
	e.mappings = []Mapping{}
	var metadata []TransformMetadata
	counter := 0

	// Find all enum declarations using manual parsing (handles nested braces)
	enums := e.findEnumDeclarations([]byte(code))
	if len(enums) == 0 {
		// No enums found, return as-is
		return code, nil, nil, nil
	}

	// Process enums in reverse order to maintain correct offsets
	result := []byte(code)
	for i := len(enums) - 1; i >= 0; i-- {
		enum := enums[i]

		// Parse variants
		variants, err := e.parseVariants(enum.body)
		if err != nil {
			// Lenient error handling - continue
			continue
		}

		// Generate Go sum type with optional marker
		generated := e.generateSumTypeV2(enum.name, variants, &counter, mode)

		// Replace enum declaration with generated code
		result = append(result[:enum.start], append([]byte(generated), result[enum.end:]...)...)

		// Add metadata if in PostAST or Dual mode
		if mode == ModePostAST || mode == ModeDual {
			marker := fmt.Sprintf("// dingo:n:%d", counter-1)
			meta := TransformMetadata{
				Type:            "enum",
				OriginalLine:    1, // Line tracking is complex for multi-line enums
				OriginalColumn:  enum.start,
				OriginalLength:  enum.end - enum.start,
				OriginalText:    fmt.Sprintf("enum %s {...}", enum.name),
				GeneratedMarker: marker,
				ASTNodeType:     "TypeSpec",
			}
			metadata = append(metadata, meta)
		}
	}

	var sourceMap *SourceMap
	if mode == ModeLegacy || mode == ModeDual {
		sourceMap = &SourceMap{
			Version:  1,
			Mappings: e.mappings,
		}
	}

	return string(result), metadata, sourceMap, nil
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

		// Skip trailing whitespace after the enum closing brace
		enumEnd := braceEnd + 1
		for enumEnd < len(src) && (src[enumEnd] == '\n' || src[enumEnd] == ' ' || src[enumEnd] == '\t') {
			// Only skip ONE newline (preserve spacing between declarations)
			if src[enumEnd] == '\n' {
				enumEnd++
				break
			}
			enumEnd++
		}

		decls = append(decls, enumDecl{
			start: idx,
			end:   enumEnd,
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

// sortStrings sorts a string slice in-place using simple insertion sort
func sortStrings(arr []string) {
	for i := 1; i < len(arr); i++ {
		key := arr[i]
		j := i - 1
		for j >= 0 && arr[j] > key {
			arr[j+1] = arr[j]
			j--
		}
		arr[j+1] = key
	}
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

		// Try to match tuple variant
		if matches := tupleVariantPattern.FindStringSubmatch(line); matches != nil {
			variantName := matches[1]
			typesStr := matches[2]

			fields, err := e.parseTupleFields(typesStr)
			if err != nil {
				return nil, fmt.Errorf("failed to parse tuple for variant %s: %w", variantName, err)
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

// parseTupleFields parses tuple types in a variant
// Input: "float64, error" or "string"
// Output: Fields with auto-generated names (0, 1, 2, ...)
func (e *EnumProcessor) parseTupleFields(typesStr string) ([]Field, error) {
	fields := []Field{}

	// Split by comma
	parts := strings.Split(typesStr, ",")

	for i, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Auto-generate field name as index
		fieldName := fmt.Sprintf("%d", i)
		fieldType := part

		fields = append(fields, Field{
			Name: fieldName,
			Type: fieldType,
		})
	}

	if len(fields) == 0 {
		return nil, fmt.Errorf("no types found in tuple")
	}

	return fields, nil
}

// generateSumType generates Go sum type code from enum definition
func (e *EnumProcessor) generateSumType(enumName string, variants []Variant) string {
	var buf bytes.Buffer

	// 1. Generate tag type
	tagTypeName := fmt.Sprintf("%sTag", enumName)
	buf.WriteString(fmt.Sprintf("type %s uint8\n\n", tagTypeName))

	// 2. Generate tag constants
	buf.WriteString("const (\n")
	for i, variant := range variants {
		tagConstName := fmt.Sprintf("%s%s", tagTypeName, variant.Name)
		if i == 0 {
			buf.WriteString(fmt.Sprintf("\t%s %s = iota\n", tagConstName, tagTypeName))
		} else {
			buf.WriteString(fmt.Sprintf("\t%s\n", tagConstName))
		}
	}
	buf.WriteString(")\n\n")

	// 3. Generate struct with tag and fields
	buf.WriteString(fmt.Sprintf("type %s struct {\n", enumName))
	buf.WriteString("\ttag " + tagTypeName + "\n")

	// CRITICAL BUG FIX: Collect all fields from ALL variants into struct
	// OLD BUGGY APPROACH (nested loops): Generated duplicate fields with wrong names
	// NEW CORRECT APPROACH: Single pass, deduplicate field names

	// Strategy:
	// - Single-field tuple variants: Use lowercase variant name (ok, err, some)
	// - Multi-field tuple variants: Use variant name + suffix (first no suffix, then 1, 2, 3)
	//   Example: Triple(int, string, bool) → first, second1, third2
	// - Struct variants: Use variant_fieldname format

	fieldMap := make(map[string]string) // fieldName -> fieldType (for deduplication)

	for _, variant := range variants {
		if len(variant.Fields) > 0 {
			// Determine field naming strategy for this variant
			isSingleTupleVariant := len(variant.Fields) == 1 &&
				len(variant.Fields[0].Name) > 0 &&
				variant.Fields[0].Name[0] >= '0' &&
				variant.Fields[0].Name[0] <= '9'

			for fieldIdx, field := range variant.Fields {
				var fieldName string
				isTupleField := len(field.Name) > 0 && field.Name[0] >= '0' && field.Name[0] <= '9'

				if isSingleTupleVariant {
					// Single tuple field - use variant name (e.g., "ok", "err", "some")
					fieldName = strings.ToLower(variant.Name)
				} else if isTupleField {
					// Multiple tuple fields - use proper naming convention
					// First field: lowercase variant name (no suffix)
					// Second field: lowercase variant name + "1"
					// Third field: lowercase variant name + "2"
					// etc.
					baseName := strings.ToLower(variant.Name)
					if fieldIdx == 0 {
						fieldName = baseName // First field: no suffix
					} else {
						fieldName = fmt.Sprintf("%s%d", baseName, fieldIdx) // 2nd+ fields: suffix with index (1, 2, 3...)
					}
				} else {
					// Struct variant with named fields - use variant_fieldname
					fieldName = strings.ToLower(variant.Name) + "_" + field.Name
				}

				// Add to field map (deduplicates if same field used in multiple variants)
				fieldMap[fieldName] = field.Type
			}
		}
	}

	// Generate fields in alphabetical order for consistency
	var fieldNames []string
	for name := range fieldMap {
		fieldNames = append(fieldNames, name)
	}
	// Sort for deterministic output
	sort.Strings(fieldNames)

	for _, fieldName := range fieldNames {
		fieldType := fieldMap[fieldName]
		buf.WriteString(fmt.Sprintf("\t%s *%s\n", fieldName, fieldType))
	}

	buf.WriteString("}\n\n")

	// 4. Generate constructor functions
	for _, variant := range variants {
		constructorName := fmt.Sprintf("%s%s", enumName, variant.Name)
		tagConstName := fmt.Sprintf("%s%s", tagTypeName, variant.Name)

		if len(variant.Fields) == 0 {
			// Unit variant constructor
			buf.WriteString(fmt.Sprintf("func %s() %s {\n", constructorName, enumName))
			buf.WriteString(fmt.Sprintf("\treturn %s{tag: %s}\n", enumName, tagConstName))
			buf.WriteString("}\n")
		} else {
			// Struct variant constructor
			params := []string{}
			assignments := []string{}

			// CRITICAL: Use same field naming strategy as struct generation above
			isSingleTupleVariant := len(variant.Fields) == 1 &&
				len(variant.Fields[0].Name) > 0 &&
				variant.Fields[0].Name[0] >= '0' &&
				variant.Fields[0].Name[0] <= '9'

			for fieldIdx, field := range variant.Fields {
				// Determine parameter name
				paramName := field.Name
				isTupleField := len(field.Name) > 0 && field.Name[0] >= '0' && field.Name[0] <= '9'
				if isTupleField {
					// Tuple field - numeric name like "0", "1" → "arg0", "arg1"
					paramName = "arg" + field.Name
				}

				// Determine field name using SAME logic as struct generation
				var fieldName string
				if isSingleTupleVariant {
					// Single tuple field - use variant name (e.g., "ok", "err", "some")
					fieldName = strings.ToLower(variant.Name)
				} else if isTupleField {
					// Multiple tuple fields - use proper naming convention (first, second1, third2)
					baseName := strings.ToLower(variant.Name)
					if fieldIdx == 0 {
						fieldName = baseName // First field: no suffix
					} else {
						fieldName = fmt.Sprintf("%s%d", baseName, fieldIdx) // 2nd+ fields: suffix 1, 2, 3
					}
				} else {
					// Struct variant with named fields - use variant_fieldname
					fieldName = strings.ToLower(variant.Name) + "_" + field.Name
				}

				params = append(params, fmt.Sprintf("%s %s", paramName, field.Type))
				assignments = append(assignments, fmt.Sprintf("%s: &%s", fieldName, paramName))
			}

			buf.WriteString(fmt.Sprintf("func %s(%s) %s {\n",
				constructorName, strings.Join(params, ", "), enumName))
			buf.WriteString(fmt.Sprintf("\treturn %s{tag: %s, %s}\n",
				enumName, tagConstName, strings.Join(assignments, ", ")))
			buf.WriteString("}\n")
		}
	}

	// 5. Generate Is* methods
	for _, variant := range variants {
		tagConstName := fmt.Sprintf("%s%s", tagTypeName, variant.Name)
		buf.WriteString(fmt.Sprintf("func (e %s) Is%s() bool {\n", enumName, variant.Name))
		buf.WriteString(fmt.Sprintf("\treturn e.tag == %s\n", tagConstName))
		buf.WriteString("}\n")
	}

	// 6. Generate Map and AndThen methods for Option/Result-like enums
	e.generateHelperMethods(&buf, enumName, tagTypeName, variants)

	return buf.String()
}

// generateSumTypeV2 generates Go sum type code with marker support for ProcessV2
func (e *EnumProcessor) generateSumTypeV2(enumName string, variants []Variant, markerCounter *int, mode PreprocessorMode) string {
	// Generate the sum type using existing method
	generated := e.generateSumType(enumName, variants)

	// Insert marker if in PostAST or Dual mode
	if mode == ModePostAST || mode == ModeDual {
		marker := fmt.Sprintf("// dingo:n:%d\n", *markerCounter)
		generated = marker + generated
		*markerCounter++
	}

	return generated
}

// generateHelperMethods generates Map and AndThen methods for Option/Result-like enums
// These methods enable functional chaining patterns
func (e *EnumProcessor) generateHelperMethods(buf *bytes.Buffer, enumName, tagTypeName string, variants []Variant) {
	// Detect if this is an Option or Result type based on variant names
	isOption := e.hasVariants(variants, []string{"Some", "None"})
	isResult := e.hasVariants(variants, []string{"Ok", "Err"})

	if !isOption && !isResult {
		// Only generate helpers for Option/Result-like enums
		return
	}

	if isOption {
		e.generateOptionHelpers(buf, enumName, tagTypeName, variants)
	}

	if isResult {
		e.generateResultHelpers(buf, enumName, tagTypeName, variants)
	}
}

// hasVariants checks if the enum has specific variant names
func (e *EnumProcessor) hasVariants(variants []Variant, names []string) bool {
	found := make(map[string]bool)
	for _, v := range variants {
		for _, name := range names {
			if v.Name == name {
				found[name] = true
			}
		}
	}
	return len(found) == len(names)
}

// generateOptionHelpers generates Map and AndThen for Option types
func (e *EnumProcessor) generateOptionHelpers(buf *bytes.Buffer, enumName, tagTypeName string, variants []Variant) {
	// Find the Some variant to get the value type
	var someVariant *Variant
	for i := range variants {
		if variants[i].Name == "Some" {
			someVariant = &variants[i]
			break
		}
	}

	if someVariant == nil || len(someVariant.Fields) == 0 {
		return // Can't generate without knowing the type
	}

	valueType := someVariant.Fields[0].Type
	fieldName := "some" // lowercase variant name

	// Map(fn func(T) T) Option
	// Note: Since Go lacks generics, we can only map T -> T, not T -> U
	buf.WriteString("\n")
	buf.WriteString(fmt.Sprintf("func (o %s) Map(fn func(%s) %s) %s {\n", enumName, valueType, valueType, enumName))
	buf.WriteString("\tswitch o.tag {\n")
	buf.WriteString(fmt.Sprintf("\tcase %sSome:\n", tagTypeName))
	buf.WriteString(fmt.Sprintf("\t\tif o.%s != nil {\n", fieldName))
	buf.WriteString(fmt.Sprintf("\t\t\treturn %sSome(fn(*o.%s))\n", enumName, fieldName))
	buf.WriteString("\t\t}\n")
	buf.WriteString(fmt.Sprintf("\tcase %sNone:\n", tagTypeName))
	buf.WriteString("\t\treturn o\n")
	buf.WriteString("\t}\n")
	buf.WriteString(fmt.Sprintf("\tpanic(\"invalid %s state\")\n", enumName))
	buf.WriteString("}\n")

	// AndThen(fn func(T) Option) Option
	buf.WriteString("\n")
	buf.WriteString(fmt.Sprintf("func (o %s) AndThen(fn func(%s) %s) %s {\n", enumName, valueType, enumName, enumName))
	buf.WriteString("\tswitch o.tag {\n")
	buf.WriteString(fmt.Sprintf("\tcase %sSome:\n", tagTypeName))
	buf.WriteString(fmt.Sprintf("\t\tif o.%s != nil {\n", fieldName))
	buf.WriteString(fmt.Sprintf("\t\t\treturn fn(*o.%s)\n", fieldName))
	buf.WriteString("\t\t}\n")
	buf.WriteString(fmt.Sprintf("\tcase %sNone:\n", tagTypeName))
	buf.WriteString("\t\treturn o\n")
	buf.WriteString("\t}\n")
	buf.WriteString(fmt.Sprintf("\tpanic(\"invalid %s state\")\n", enumName))
	buf.WriteString("}\n")
}

// generateResultHelpers generates Map and AndThen for Result types
func (e *EnumProcessor) generateResultHelpers(buf *bytes.Buffer, enumName, tagTypeName string, variants []Variant) {
	// Find Ok and Err variants
	var okVariant, errVariant *Variant
	for i := range variants {
		if variants[i].Name == "Ok" {
			okVariant = &variants[i]
		} else if variants[i].Name == "Err" {
			errVariant = &variants[i]
		}
	}

	if okVariant == nil || errVariant == nil || len(okVariant.Fields) == 0 || len(errVariant.Fields) == 0 {
		return // Can't generate without both types
	}

	okType := okVariant.Fields[0].Type
	okFieldName := "ok" // lowercase variant name

	// Map(fn func(T) T) Result
	// Note: Since Go lacks generics, we can only map T -> T, not T -> U
	buf.WriteString("\n")
	buf.WriteString(fmt.Sprintf("func (r %s) Map(fn func(%s) %s) %s {\n", enumName, okType, okType, enumName))
	buf.WriteString("\tswitch r.tag {\n")
	buf.WriteString(fmt.Sprintf("\tcase %sOk:\n", tagTypeName))
	buf.WriteString(fmt.Sprintf("\t\tif r.%s != nil {\n", okFieldName))
	buf.WriteString(fmt.Sprintf("\t\t\treturn %sOk(fn(*r.%s))\n", enumName, okFieldName))
	buf.WriteString("\t\t}\n")
	buf.WriteString(fmt.Sprintf("\tcase %sErr:\n", tagTypeName))
	buf.WriteString("\t\treturn r\n")
	buf.WriteString("\t}\n")
	buf.WriteString(fmt.Sprintf("\tpanic(\"invalid %s state\")\n", enumName))
	buf.WriteString("}\n")

	// AndThen(fn func(T) Result) Result
	buf.WriteString("\n")
	buf.WriteString(fmt.Sprintf("func (r %s) AndThen(fn func(%s) %s) %s {\n", enumName, okType, enumName, enumName))
	buf.WriteString("\tswitch r.tag {\n")
	buf.WriteString(fmt.Sprintf("\tcase %sOk:\n", tagTypeName))
	buf.WriteString(fmt.Sprintf("\t\tif r.%s != nil {\n", okFieldName))
	buf.WriteString(fmt.Sprintf("\t\t\treturn fn(*r.%s)\n", okFieldName))
	buf.WriteString("\t\t}\n")
	buf.WriteString(fmt.Sprintf("\tcase %sErr:\n", tagTypeName))
	buf.WriteString("\t\treturn r\n")
	buf.WriteString("\t}\n")
	buf.WriteString(fmt.Sprintf("\tpanic(\"invalid %s state\")\n", enumName))
	buf.WriteString("}\n")
}
