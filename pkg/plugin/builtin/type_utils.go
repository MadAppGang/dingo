// Package builtin provides built-in Dingo transformation plugins
package builtin

import (
	"fmt"
	"go/types"
	"strings"
)

// typeToString converts a types.Type to a string representation for naming.
// This is used by Result and Option plugins to generate type names.
//
// Examples:
//   - *types.Basic{int} -> "int"
//   - *types.Named{MyStruct} -> "MyStruct"
//   - *types.Pointer{*int} -> "ptr_int"
//   - *types.Slice{[]string} -> "slice_string"
func typeToString(typ types.Type) string {
	if typ == nil {
		return "unknown"
	}

	// Handle basic types
	switch t := typ.(type) {
	case *types.Basic:
		return t.Name()
	case *types.Named:
		obj := t.Obj()
		if obj != nil {
			return obj.Name()
		}
	case *types.Pointer:
		elem := typeToString(t.Elem())
		return "ptr_" + elem
	case *types.Slice:
		elem := typeToString(t.Elem())
		return "slice_" + elem
	case *types.Array:
		elem := typeToString(t.Elem())
		return fmt.Sprintf("array_%s", elem)
	case *types.Map:
		key := typeToString(t.Key())
		val := typeToString(t.Elem())
		return fmt.Sprintf("map_%s_%s", key, val)
	}

	// Fallback to String() method
	str := typ.String()
	// Remove package paths for cleaner names
	if idx := strings.LastIndex(str, "."); idx >= 0 {
		str = str[idx+1:]
	}
	return str
}

// sanitizeTypeName ensures type names are valid Go identifiers.
// It replaces special characters with underscores and pointer markers.
//
// Examples:
//   - "map[string]int" -> "map_string_int"
//   - "*User" -> "ptr_User"
//   - "pkg.Type" -> "pkg_Type"
func sanitizeTypeName(name string) string {
	// Replace invalid characters with underscores
	name = strings.ReplaceAll(name, ".", "_")
	name = strings.ReplaceAll(name, "[", "_")
	name = strings.ReplaceAll(name, "]", "_")
	name = strings.ReplaceAll(name, "*", "ptr_")
	name = strings.ReplaceAll(name, " ", "_")
	name = strings.ReplaceAll(name, "(", "_")
	name = strings.ReplaceAll(name, ")", "_")
	name = strings.ReplaceAll(name, ",", "_")
	return name
}
