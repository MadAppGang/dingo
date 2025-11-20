package sourcemap

import (
	"encoding/json"
	"go/token"
	"strings"
	"testing"
)

func TestNewGenerator(t *testing.T) {
	gen := NewGenerator("main.dingo", "main.go")

	if gen.sourceFile != "main.dingo" {
		t.Errorf("Expected sourceFile 'main.dingo', got %q", gen.sourceFile)
	}

	if gen.genFile != "main.go" {
		t.Errorf("Expected genFile 'main.go', got %q", gen.genFile)
	}

	if gen.mappings == nil {
		t.Error("Expected mappings to be initialized")
	}

	if len(gen.mappings) != 0 {
		t.Errorf("Expected empty mappings, got %d", len(gen.mappings))
	}
}

func TestAddMapping(t *testing.T) {
	gen := NewGenerator("test.dingo", "test.go")

	src := token.Position{Line: 10, Column: 5}
	gen1 := token.Position{Line: 15, Column: 8}

	gen.AddMapping(src, gen1)

	if len(gen.mappings) != 1 {
		t.Fatalf("Expected 1 mapping, got %d", len(gen.mappings))
	}

	m := gen.mappings[0]
	if m.SourceLine != 10 || m.SourceColumn != 5 {
		t.Errorf("Expected source 10:5, got %d:%d", m.SourceLine, m.SourceColumn)
	}

	if m.GenLine != 15 || m.GenColumn != 8 {
		t.Errorf("Expected gen 15:8, got %d:%d", m.GenLine, m.GenColumn)
	}

	if m.Name != "" {
		t.Errorf("Expected no name, got %q", m.Name)
	}
}

func TestAddMappingWithName(t *testing.T) {
	gen := NewGenerator("test.dingo", "test.go")

	src := token.Position{Line: 5, Column: 10}
	gen1 := token.Position{Line: 7, Column: 12}

	gen.AddMappingWithName(src, gen1, "fetchUser")

	if len(gen.mappings) != 1 {
		t.Fatalf("Expected 1 mapping, got %d", len(gen.mappings))
	}

	m := gen.mappings[0]
	if m.Name != "fetchUser" {
		t.Errorf("Expected name 'fetchUser', got %q", m.Name)
	}
}

func TestMultipleMappings(t *testing.T) {
	gen := NewGenerator("test.dingo", "test.go")

	// Add multiple mappings
	mappings := []struct {
		src  token.Position
		gen  token.Position
		name string
	}{
		{token.Position{Line: 1, Column: 1}, token.Position{Line: 1, Column: 1}, ""},
		{token.Position{Line: 5, Column: 10}, token.Position{Line: 8, Column: 5}, "fetchUser"},
		{token.Position{Line: 10, Column: 2}, token.Position{Line: 15, Column: 3}, ""},
		{token.Position{Line: 12, Column: 8}, token.Position{Line: 18, Column: 12}, "user"},
	}

	for _, m := range mappings {
		if m.name != "" {
			gen.AddMappingWithName(m.src, m.gen, m.name)
		} else {
			gen.AddMapping(m.src, m.gen)
		}
	}

	if len(gen.mappings) != 4 {
		t.Errorf("Expected 4 mappings, got %d", len(gen.mappings))
	}
}

func TestCollectNames(t *testing.T) {
	gen := NewGenerator("test.dingo", "test.go")

	// Add mappings with duplicate names
	gen.AddMappingWithName(token.Position{Line: 1, Column: 1}, token.Position{Line: 1, Column: 1}, "fetchUser")
	gen.AddMappingWithName(token.Position{Line: 2, Column: 1}, token.Position{Line: 2, Column: 1}, "user")
	gen.AddMappingWithName(token.Position{Line: 3, Column: 1}, token.Position{Line: 3, Column: 1}, "fetchUser") // Duplicate
	gen.AddMappingWithName(token.Position{Line: 4, Column: 1}, token.Position{Line: 4, Column: 1}, "id")
	gen.AddMapping(token.Position{Line: 5, Column: 1}, token.Position{Line: 5, Column: 1}) // No name

	names := gen.collectUniqueNames(gen.mappings)

	// Should have unique names only
	if len(names) != 3 {
		t.Errorf("Expected 3 unique names, got %d: %v", len(names), names)
	}

	// Check all expected names are present
	expectedNames := map[string]bool{"fetchUser": false, "user": false, "id": false}
	for _, name := range names {
		if _, exists := expectedNames[name]; !exists {
			t.Errorf("Unexpected name %q in names list", name)
		}
		expectedNames[name] = true
	}

	for name, found := range expectedNames {
		if !found {
			t.Errorf("Expected name %q not found in names list", name)
		}
	}
}

func TestGenerateSourceMap(t *testing.T) {
	gen := NewGenerator("main.dingo", "main.go")

	// Add some mappings
	gen.AddMapping(token.Position{Line: 1, Column: 0}, token.Position{Line: 1, Column: 0})
	gen.AddMappingWithName(token.Position{Line: 5, Column: 10}, token.Position{Line: 8, Column: 5}, "fetchUser")

	data, err := gen.Generate()
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	// Parse JSON to validate structure
	var sm map[string]interface{}
	if err := json.Unmarshal(data, &sm); err != nil {
		t.Fatalf("Failed to parse generated source map JSON: %v", err)
	}

	// Validate Source Map v3 structure
	if version, ok := sm["version"].(float64); !ok || version != 3 {
		t.Errorf("Expected version 3, got %v", sm["version"])
	}

	if file, ok := sm["file"].(string); !ok || file != "main.go" {
		t.Errorf("Expected file 'main.go', got %v", sm["file"])
	}

	if sourceRoot, ok := sm["sourceRoot"].(string); !ok || sourceRoot != "" {
		t.Errorf("Expected empty sourceRoot, got %v", sm["sourceRoot"])
	}

	sources, ok := sm["sources"].([]interface{})
	if !ok || len(sources) != 1 {
		t.Errorf("Expected 1 source, got %v", sm["sources"])
	} else if sources[0].(string) != "main.dingo" {
		t.Errorf("Expected source 'main.dingo', got %v", sources[0])
	}

	names, ok := sm["names"].([]interface{})
	if !ok {
		t.Errorf("Expected names array, got %v", sm["names"])
	} else if len(names) != 1 {
		t.Errorf("Expected 1 name, got %d", len(names))
	} else if names[0].(string) != "fetchUser" {
		t.Errorf("Expected name 'fetchUser', got %v", names[0])
	}

	// Mappings should now contain VLQ-encoded data
	if mappings, ok := sm["mappings"].(string); !ok {
		t.Errorf("Expected mappings string, got %T", sm["mappings"])
	} else if mappings == "" {
		t.Error("Expected non-empty VLQ-encoded mappings, got empty string")
	}
}

func TestGenerateInline(t *testing.T) {
	gen := NewGenerator("test.dingo", "test.go")
	gen.AddMapping(token.Position{Line: 1, Column: 0}, token.Position{Line: 1, Column: 0})

	inline, err := gen.GenerateInline()
	if err != nil {
		t.Fatalf("GenerateInline() error = %v", err)
	}

	// Check format
	if !strings.HasPrefix(inline, "//# sourceMappingURL=data:application/json;base64,") {
		t.Errorf("Expected inline source map comment, got %q", inline[:50])
	}

	// Verify it's base64 encoded
	parts := strings.Split(inline, ",")
	if len(parts) != 2 {
		t.Fatalf("Expected format '//# sourceMappingURL=data:application/json;base64,<data>', got %q", inline)
	}

	// The second part should be base64
	if len(parts[1]) == 0 {
		t.Error("Expected base64 data, got empty string")
	}
}

func TestGenerateEmpty(t *testing.T) {
	gen := NewGenerator("empty.dingo", "empty.go")

	data, err := gen.Generate()
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	var sm map[string]interface{}
	if err := json.Unmarshal(data, &sm); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	// Should still have valid structure
	if version, ok := sm["version"].(float64); !ok || version != 3 {
		t.Errorf("Expected version 3, got %v", sm["version"])
	}

	// Names should be empty array
	names, ok := sm["names"].([]interface{})
	if !ok || len(names) != 0 {
		t.Errorf("Expected empty names array, got %v", sm["names"])
	}
}

func TestConsumerCreation(t *testing.T) {
	gen := NewGenerator("test.dingo", "test.go")
	gen.AddMapping(token.Position{Line: 1, Column: 1}, token.Position{Line: 1, Column: 1})

	data, err := gen.Generate()
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	consumer, err := NewConsumer(data)
	if err != nil {
		t.Fatalf("NewConsumer() error = %v", err)
	}

	if consumer == nil {
		t.Error("Expected valid consumer, got nil")
	}
}

func TestConsumerInvalidJSON(t *testing.T) {
	invalidJSON := `{invalid json`

	_, err := NewConsumer([]byte(invalidJSON))
	if err == nil {
		t.Error("Expected error for invalid JSON, got nil")
	}
}
