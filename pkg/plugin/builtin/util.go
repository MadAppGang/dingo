package builtin

import (
	"fmt"
	"strings"
)

// SanitizeTypeName converts type name parts to underscore format with leading underscore
// This is used for Result<T,E> → Result_T_E and Option<T> → Option_T naming
// Examples:
//   ("int", "error") → "_int_error"
//   ("string") → "_string"
//   ("any", "error") → "_interface_error"
//   ("*User", "error") → "_ptr_User_error"
//   ("[]int", "error") → "_slice_int_error"
func SanitizeTypeName(parts ...string) string {
	var result strings.Builder
	for _, part := range parts {
		result.WriteString("_")
		result.WriteString(sanitizeTypeComponent(part))
	}
	return result.String()
}

// Package-level maps for performance (avoid recreating on every call)
var (
	// commonAcronyms maps lowercase acronyms to their canonical Go form.
	// Only include genuine acronyms (HTTP, URL, etc.), not regular words.
	// Regular words are handled by the default capitalization logic.
	commonAcronyms = map[string]string{
		"http":  "HTTP",
		"https": "HTTPS",
		"url":   "URL",
		"uri":   "URI",
		"json":  "JSON",
		"xml":   "XML",
		"api":   "API",
		"id":    "ID",
		"uuid":  "UUID",
		"sql":   "SQL",
		"html":  "HTML",
		"css":   "CSS",
		"tcp":   "TCP",
		"udp":   "UDP",
		"ip":    "IP",
	}

	// builtinTypes contains Go built-in types that should only capitalize the first letter
	builtinTypes = map[string]bool{
		"int": true, "int8": true, "int16": true, "int32": true, "int64": true,
		"uint": true, "uint8": true, "uint16": true, "uint32": true, "uint64": true,
		"float32": true, "float64": true,
		"string": true, "bool": true, "byte": true, "rune": true,
		"error": true, "any": true,
	}
)

// sanitizeTypeComponent sanitizes individual type components for underscore format
// Handles special prefixes like *, [], map[], chan, etc.
func sanitizeTypeComponent(s string) string {
	if s == "" {
		return s
	}

	// Handle pointer types: *T → ptr_T
	if strings.HasPrefix(s, "*") {
		return "ptr_" + sanitizeTypeComponent(strings.TrimPrefix(s, "*"))
	}

	// Handle slice types: []T → slice_T
	if strings.HasPrefix(s, "[]") {
		return "slice_" + sanitizeTypeComponent(strings.TrimPrefix(s, "[]"))
	}

	// Handle map types: map[K]V → map_K_V (simplified)
	if strings.HasPrefix(s, "map[") {
		// Complex parsing - for now just use "map"
		return "map"
	}

	// Handle chan types
	if strings.HasPrefix(s, "chan ") {
		return "chan_" + sanitizeTypeComponent(strings.TrimPrefix(s, "chan "))
	}

	// Handle interface{} → interface
	if s == "interface{}" {
		return "interface"
	}

	// Handle any → interface (Go 1.18+)
	if s == "any" {
		return "interface"
	}

	// Otherwise return as-is (preserves case for user types like User, CustomError)
	// Built-in types like int, string, error remain lowercase
	return s
}

// GenerateTempVarName generates temporary variable names with optional numbering
// First call returns base name (e.g., "ok"), subsequent calls add numbers ("ok1", "ok2")
// Examples:
//   ("ok", 0) → "ok"
//   ("ok", 1) → "ok1"
//   ("err", 0) → "err"
//   ("err", 1) → "err1"
func GenerateTempVarName(base string, index int) string {
	if index < 0 {
		index = 0 // Defensive: treat negative as zero
	}
	if index == 0 {
		return base // First variable: no number suffix
	}
	return fmt.Sprintf("%s%d", base, index) // ok1, ok2, ok3, ...
}
