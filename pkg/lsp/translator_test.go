package lsp

import (
	"strings"
	"testing"

	"github.com/MadAppGang/dingo/pkg/preprocessor"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// mockTranslator wraps translator with a mock source map for testing
type mockTranslator struct {
	*Translator
	mockSM *preprocessor.SourceMap
}

func newMockTranslator(sm *preprocessor.SourceMap) *Translator {
	// Create a test cache that always returns our test source map
	return &Translator{
		cache: &testCache{sm: sm},
	}
}

// testCache is a simple cache that always returns the test source map
type testCache struct {
	sm *preprocessor.SourceMap
}

func (c *testCache) Get(goFilePath string) (*preprocessor.SourceMap, error) {
	return c.sm, nil
}

func (c *testCache) Invalidate(goFilePath string) {}
func (c *testCache) InvalidateAll()              {}
func (c *testCache) Size() int                   { return 1 }

func TestTranslatePosition_DingoToGo(t *testing.T) {
	sm := &preprocessor.SourceMap{
		Version: 1,
		Mappings: []preprocessor.Mapping{
			{
				OriginalLine:    5,
				OriginalColumn:  10,
				GeneratedLine:   12,
				GeneratedColumn: 15,
				Length:          3,
				Name:            "error_prop",
			},
		},
	}

	translator := newMockTranslator(sm)

	// Test Dingo → Go translation
	testURI, pos, err := translator.TranslatePosition(
		uri.File("test.dingo"),
		protocol.Position{Line: 4, Character: 9}, // 0-based
		DingoToGo,
	)

	if err != nil {
		t.Fatalf("Translation failed: %v", err)
	}

	expectedSuffix := "test.go"
	if !strings.HasSuffix(testURI.Filename(), expectedSuffix) {
		t.Errorf("Expected URI ending with %s, got %s", expectedSuffix, testURI.Filename())
	}

	// Expect: original (5,10) 1-based → generated (12,15) 1-based → (11,14) 0-based LSP
	expectedLine := uint32(11)
	expectedChar := uint32(14)

	if pos.Line != expectedLine {
		t.Errorf("Expected line %d, got %d", expectedLine, pos.Line)
	}

	if pos.Character != expectedChar {
		t.Errorf("Expected character %d, got %d", expectedChar, pos.Character)
	}
}

func TestTranslatePosition_GoToDingo(t *testing.T) {
	sm := &preprocessor.SourceMap{
		Version: 1,
		Mappings: []preprocessor.Mapping{
			{
				OriginalLine:    5,
				OriginalColumn:  10,
				GeneratedLine:   12,
				GeneratedColumn: 15,
				Length:          3,
				Name:            "error_prop",
			},
		},
	}

	translator := newMockTranslator(sm)

	// Test Go → Dingo translation
	testURI, pos, err := translator.TranslatePosition(
		uri.File("test.go"),
		protocol.Position{Line: 11, Character: 14}, // 0-based (generated 12,15 in 1-based)
		GoToDingo,
	)

	if err != nil {
		t.Fatalf("Translation failed: %v", err)
	}

	expectedSuffix := "test.dingo"
	if !strings.HasSuffix(testURI.Filename(), expectedSuffix) {
		t.Errorf("Expected URI ending with %s, got %s", expectedSuffix, testURI.Filename())
	}

	// Expect: generated (12,15) 1-based → original (5,10) 1-based → (4,9) 0-based LSP
	expectedLine := uint32(4)
	expectedChar := uint32(9)

	if pos.Line != expectedLine {
		t.Errorf("Expected line %d, got %d", expectedLine, pos.Line)
	}

	if pos.Character != expectedChar {
		t.Errorf("Expected character %d, got %d", expectedChar, pos.Character)
	}
}

func TestTranslateRange(t *testing.T) {
	sm := &preprocessor.SourceMap{
		Version: 1,
		Mappings: []preprocessor.Mapping{
			{
				OriginalLine:    5,
				OriginalColumn:  10,
				GeneratedLine:   12,
				GeneratedColumn: 15,
				Length:          10, // Longer mapping to include the end position
				Name:            "test",
			},
		},
	}

	translator := newMockTranslator(sm)

	// Test range from (5,10) to (5,15) in original (1-based)
	// Which is (4,9) to (4,14) in 0-based LSP coordinates
	rng := protocol.Range{
		Start: protocol.Position{Line: 4, Character: 9},  // 5,10 in 1-based
		End:   protocol.Position{Line: 4, Character: 14}, // 5,15 in 1-based
	}

	testURI, newRange, err := translator.TranslateRange(
		uri.File("test.dingo"),
		rng,
		DingoToGo,
	)

	if err != nil {
		t.Fatalf("Range translation failed: %v", err)
	}

	if !strings.HasSuffix(testURI.Filename(), "test.go") {
		t.Errorf("Expected URI ending with test.go, got %s", testURI.Filename())
	}

	// Both start and end should be translated
	// Original (5,10) → Generated (12,15), but in 0-based LSP coordinates: (11,14)
	expectedStartLine := uint32(11)
	if newRange.Start.Line != expectedStartLine {
		t.Errorf("Expected start line %d, got %d", expectedStartLine, newRange.Start.Line)
	}

	// End position - same line since original range is on same line
	if newRange.End.Line != expectedStartLine {
		t.Errorf("Expected end line %d, got %d", expectedStartLine, newRange.End.Line)
	}
}

func TestHelperFunctions(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		funcName string
		expected string
	}{
		{"dingoToGo", "test.dingo", "dingoToGo", "test.go"},
		{"dingoToGo non-dingo", "test.go", "dingoToGo", "test.go"},
		{"goToDingo", "test.go", "goToDingo", "test.dingo"},
		{"goToDingo non-go", "test.dingo", "goToDingo", "test.dingo"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result string
			if tt.funcName == "dingoToGo" {
				result = dingoToGoPath(tt.input)
			} else {
				result = goToDingoPath(tt.input)
			}

			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestIsDingoFile(t *testing.T) {
	tests := []struct {
		uri      protocol.DocumentURI
		expected bool
	}{
		{uri.File("test.dingo"), true},
		{uri.File("test.go"), false},
		{uri.File("/path/to/file.dingo"), true},
		{uri.File("file.txt"), false},
	}

	for _, tt := range tests {
		result := isDingoFile(tt.uri)
		if result != tt.expected {
			t.Errorf("isDingoFile(%s) = %v, expected %v", tt.uri, result, tt.expected)
		}
	}
}
