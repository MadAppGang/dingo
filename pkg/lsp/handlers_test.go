package lsp

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/MadAppGang/dingo/pkg/preprocessor"
)

func TestTranslateCompletionList(t *testing.T) {
	// Create mock source map
	sm := &preprocessor.SourceMap{
		Version: 1,
		Mappings: []preprocessor.Mapping{
			{
				OriginalLine:    5,
				OriginalColumn:  10,
				GeneratedLine:   12,
				GeneratedColumn: 15,
				Length:          3,
				Name:            "test",
			},
		},
	}

	cache := &testCache{sm: sm}
	translator := NewTranslator(cache)

	// Create completion list with text edits
	list := &protocol.CompletionList{
		Items: []protocol.CompletionItem{
			{
				Label: "TestFunc",
				// TextEdit is *protocol.TextEdit in CompletionItem
				// Skipping TextEdit for this test as it requires more complex setup
			},
		},
	}

	// Translate Go → Dingo
	result, err := translator.TranslateCompletionList(list, GoToDingo)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.Items, 1)

	// Note: TextEdit translation is limited without URI context
	// This test validates the structure is preserved
	assert.Equal(t, "TestFunc", result.Items[0].Label)
}

func TestTranslateHover(t *testing.T) {
	// Create mock source map
	sm := &preprocessor.SourceMap{
		Version: 1,
		Mappings: []preprocessor.Mapping{
			{
				OriginalLine:    10,
				OriginalColumn:  5,
				GeneratedLine:   20,
				GeneratedColumn: 10,
				Length:          5,
				Name:            "hover_test",
			},
		},
	}

	cache := &testCache{sm: sm}
	translator := NewTranslator(cache)

	// Create hover response with range
	hoverRange := protocol.Range{
		Start: protocol.Position{Line: 19, Character: 9}, // Go position (0-based)
		End:   protocol.Position{Line: 19, Character: 14},
	}

	hover := &protocol.Hover{
		Contents: protocol.MarkupContent{
			Kind:  protocol.Markdown,
			Value: "func TestFunc()",
		},
		Range: &hoverRange,
	}

	// Translate Go → Dingo
	originalURI := uri.File("test.dingo")
	result, err := translator.TranslateHover(hover, originalURI, GoToDingo)
	require.NoError(t, err)
	assert.NotNil(t, result)

	// Check range was translated
	assert.NotNil(t, result.Range)
	assert.Equal(t, uint32(9), result.Range.Start.Line)   // 10 - 1 (0-based)
	assert.Equal(t, uint32(4), result.Range.Start.Character) // 5 - 1
}

func TestTranslateDefinitionLocations(t *testing.T) {
	// Create mock source map
	sm := &preprocessor.SourceMap{
		Version: 1,
		Mappings: []preprocessor.Mapping{
			{
				OriginalLine:    15,
				OriginalColumn:  8,
				GeneratedLine:   30,
				GeneratedColumn: 12,
				Length:          10,
				Name:            "definition_test",
			},
		},
	}

	cache := &testCache{sm: sm}
	translator := NewTranslator(cache)

	// Create definition locations (Go positions)
	locations := []protocol.Location{
		{
			URI: uri.File("test.go"),
			Range: protocol.Range{
				Start: protocol.Position{Line: 29, Character: 11}, // 0-based
				End:   protocol.Position{Line: 29, Character: 21},
			},
		},
	}

	// Translate Go → Dingo
	result, err := translator.TranslateDefinitionLocations(locations, GoToDingo)
	require.NoError(t, err)
	assert.Len(t, result, 1)

	// Check location was translated
	assert.True(t, strings.HasSuffix(result[0].URI.Filename(), "test.dingo"))
	assert.Equal(t, uint32(14), result[0].Range.Start.Line)   // 15 - 1
	assert.Equal(t, uint32(7), result[0].Range.Start.Character) // 8 - 1
}

func TestTranslateDiagnostics(t *testing.T) {
	// Create mock source map
	sm := &preprocessor.SourceMap{
		Version: 1,
		Mappings: []preprocessor.Mapping{
			{
				OriginalLine:    8,
				OriginalColumn:  3,
				GeneratedLine:   16,
				GeneratedColumn: 5,
				Length:          4,
				Name:            "diagnostic_test",
			},
		},
	}

	cache := &testCache{sm: sm}
	translator := NewTranslator(cache)

	// Create diagnostics (Go positions)
	diagnostics := []protocol.Diagnostic{
		{
			Range: protocol.Range{
				Start: protocol.Position{Line: 15, Character: 4}, // 0-based
				End:   protocol.Position{Line: 15, Character: 8},
			},
			Severity: protocol.DiagnosticSeverityError,
			Message:  "test error",
		},
	}

	// Translate Go → Dingo
	goURI := uri.File("test.go")
	result, err := translator.TranslateDiagnostics(diagnostics, goURI, GoToDingo)
	require.NoError(t, err)
	assert.Len(t, result, 1)

	// Check diagnostic was translated
	assert.Equal(t, uint32(7), result[0].Range.Start.Line)   // 8 - 1
	assert.Equal(t, uint32(2), result[0].Range.Start.Character) // 3 - 1
	assert.Equal(t, "test error", result[0].Message)
}

func TestTranslateDiagnostics_WithRelatedInformation(t *testing.T) {
	// Create mock source map
	sm := &preprocessor.SourceMap{
		Version: 1,
		Mappings: []preprocessor.Mapping{
			{
				OriginalLine:    5,
				OriginalColumn:  1,
				GeneratedLine:   10,
				GeneratedColumn: 1,
				Length:          20,
				Name:            "related_test",
			},
		},
	}

	cache := &testCache{sm: sm}
	translator := NewTranslator(cache)

	// Create diagnostic with related information
	diagnostics := []protocol.Diagnostic{
		{
			Range: protocol.Range{
				Start: protocol.Position{Line: 9, Character: 0},
				End:   protocol.Position{Line: 9, Character: 10},
			},
			Severity: protocol.DiagnosticSeverityWarning,
			Message:  "unused variable",
			RelatedInformation: []protocol.DiagnosticRelatedInformation{
				{
					Location: protocol.Location{
						URI: uri.File("test.go"),
						Range: protocol.Range{
							Start: protocol.Position{Line: 9, Character: 5},
							End:   protocol.Position{Line: 9, Character: 10},
						},
					},
					Message: "declared here",
				},
			},
		},
	}

	// Translate Go → Dingo
	goURI := uri.File("test.go")
	result, err := translator.TranslateDiagnostics(diagnostics, goURI, GoToDingo)
	require.NoError(t, err)
	assert.Len(t, result, 1)

	// Check related information was translated
	assert.Len(t, result[0].RelatedInformation, 1)
	assert.True(t, strings.HasSuffix(result[0].RelatedInformation[0].Location.URI.Filename(), "test.dingo"))
	assert.Equal(t, "declared here", result[0].RelatedInformation[0].Message)
}

func TestTranslateCompletionList_EmptyList(t *testing.T) {
	cache := &testCache{sm: &preprocessor.SourceMap{Version: 1}}
	translator := NewTranslator(cache)

	// Nil list
	result, err := translator.TranslateCompletionList(nil, GoToDingo)
	assert.NoError(t, err)
	assert.Nil(t, result)

	// Empty list
	emptyList := &protocol.CompletionList{Items: []protocol.CompletionItem{}}
	result, err = translator.TranslateCompletionList(emptyList, GoToDingo)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.Items, 0)
}

func TestTranslateDefinitionLocations_EmptyList(t *testing.T) {
	cache := &testCache{sm: &preprocessor.SourceMap{Version: 1}}
	translator := NewTranslator(cache)

	// Empty list
	result, err := translator.TranslateDefinitionLocations([]protocol.Location{}, GoToDingo)
	assert.NoError(t, err)
	assert.Len(t, result, 0)
}

func TestTranslateDiagnostics_EmptyList(t *testing.T) {
	cache := &testCache{sm: &preprocessor.SourceMap{Version: 1}}
	translator := NewTranslator(cache)

	// Empty list
	goURI := uri.File("test.go")
	result, err := translator.TranslateDiagnostics([]protocol.Diagnostic{}, goURI, GoToDingo)
	assert.NoError(t, err)
	assert.Len(t, result, 0)
}

func TestTranslateHover_NoRange(t *testing.T) {
	cache := &testCache{sm: &preprocessor.SourceMap{Version: 1}}
	translator := NewTranslator(cache)

	// Hover without range
	hover := &protocol.Hover{
		Contents: protocol.MarkupContent{
			Kind:  protocol.PlainText,
			Value: "simple text",
		},
		Range: nil,
	}

	originalURI := uri.File("test.dingo")
	result, err := translator.TranslateHover(hover, originalURI, GoToDingo)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Nil(t, result.Range)
	assert.Equal(t, "simple text", result.Contents.Value)
}

func TestTranslateCompletionList_WithAdditionalTextEdits(t *testing.T) {
	sm := &preprocessor.SourceMap{
		Version: 1,
		Mappings: []preprocessor.Mapping{
			{
				OriginalLine:    3,
				OriginalColumn:  1,
				GeneratedLine:   6,
				GeneratedColumn: 1,
				Length:          10,
				Name:            "additional_edit_test",
			},
		},
	}

	cache := &testCache{sm: sm}
	translator := NewTranslator(cache)

	// Completion item with additional text edits
	list := &protocol.CompletionList{
		Items: []protocol.CompletionItem{
			{
				Label: "ImportFunc",
				AdditionalTextEdits: []protocol.TextEdit{
					{
						Range: protocol.Range{
							Start: protocol.Position{Line: 5, Character: 0},
							End:   protocol.Position{Line: 5, Character: 0},
						},
						NewText: "import \"fmt\"\n",
					},
				},
			},
		},
	}

	// Translate
	result, err := translator.TranslateCompletionList(list, GoToDingo)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.Items, 1)
	assert.Len(t, result.Items[0].AdditionalTextEdits, 1)
}
