package lsp

import (
	"fmt"
	"log"
	"strings"

	"go.lsp.dev/protocol"
	lspuri "go.lsp.dev/uri"

	"github.com/MadAppGang/dingo/pkg/preprocessor"
)

// Direction specifies translation direction
type Direction int

const (
	DingoToGo Direction = iota // .dingo → .go
	GoToDingo                   // .go → .dingo
)

// Translator handles bidirectional position translation using source maps
type Translator struct {
	cache SourceMapGetter
	// Keep preprocessor import for SourceMap type
	_ *preprocessor.SourceMap
}

// NewTranslator creates a new position translator
func NewTranslator(cache SourceMapGetter) *Translator {
	return &Translator{cache: cache}
}

// TranslatePosition translates a single position between Dingo and Go files
func (t *Translator) TranslatePosition(
	uri protocol.DocumentURI,
	pos protocol.Position,
	dir Direction,
) (protocol.DocumentURI, protocol.Position, error) {
	dirName := "DingoToGo"
	if dir == GoToDingo {
		dirName = "GoToDingo"
	}
	log.Printf("[LSP Translator] TranslatePosition START: direction=%s, uri=%s, line=%d, col=%d",
		dirName, uri.Filename(), pos.Line, pos.Character)

	// Convert LSP position (0-based) to source map position (1-based)
	line := int(pos.Line) + 1
	col := int(pos.Character) + 1

	// Determine file paths
	var goPath string
	if dir == DingoToGo {
		goPath = dingoToGoPath(uri.Filename())
	} else {
		goPath = uri.Filename()
	}

	// Load source map
	sm, err := t.cache.Get(goPath)
	if err != nil {
		// CRITICAL FIX C6: Still translate URI even with 1:1 positions
		// Bug was: returning .dingo URI to gopls when source map missing
		if dir == DingoToGo {
			// Must return .go URI, not .dingo URI
			goURI := lspuri.File(goPath)
			return goURI, pos, fmt.Errorf("source map not found: %s (file not transpiled)", goPath)
		}
		// CRITICAL FIX: For Go->Dingo without map, return location unchanged
		// This handles standard library and external package definitions
		// No source map = not a transpiled file, so return the .go location as-is
		log.Printf("[LSP Translator] No source map for %s, returning location unchanged (likely stdlib/external package)", goPath)
		return uri, pos, nil
	}

	// Translate position
	var newLine, newCol int
	var newURI protocol.DocumentURI

	if dir == DingoToGo {
		log.Printf("[LSP Translator] BEFORE MapToGenerated: line=%d, col=%d", line, col)
		newLine, newCol = sm.MapToGenerated(line, col)
		log.Printf("[LSP Translator] AFTER MapToGenerated: newLine=%d, newCol=%d", newLine, newCol)
		newURI = lspuri.File(goPath)
	} else {
		newLine, newCol = sm.MapToOriginal(line, col)
		dingoPath := goToDingoPath(goPath)
		newURI = lspuri.File(dingoPath)
	}

	// Convert back to LSP position (0-based)
	newPos := protocol.Position{
		Line:      uint32(newLine - 1),
		Character: uint32(newCol - 1),
	}

	log.Printf("[LSP Translator] TranslatePosition END: returning uri=%s, line=%d, col=%d",
		newURI.Filename(), newPos.Line, newPos.Character)

	return newURI, newPos, nil
}

// TranslateRange translates a range between Dingo and Go files
func (t *Translator) TranslateRange(
	uri protocol.DocumentURI,
	rng protocol.Range,
	dir Direction,
) (protocol.DocumentURI, protocol.Range, error) {
	// Translate start position
	newURI, newStart, err := t.TranslatePosition(uri, rng.Start, dir)
	if err != nil {
		return uri, rng, err
	}

	// Translate end position
	_, newEnd, err := t.TranslatePosition(uri, rng.End, dir)
	if err != nil {
		return uri, rng, err
	}

	newRange := protocol.Range{
		Start: newStart,
		End:   newEnd,
	}

	return newURI, newRange, nil
}

// TranslateLocation translates a location (URI + range)
func (t *Translator) TranslateLocation(
	loc protocol.Location,
	dir Direction,
) (protocol.Location, error) {
	newURI, newRange, err := t.TranslateRange(loc.URI, loc.Range, dir)
	if err != nil {
		return loc, err
	}

	return protocol.Location{
		URI:   newURI,
		Range: newRange,
	}, nil
}

// Helper functions for file path conversion

func isDingoFile(uri protocol.DocumentURI) bool {
	return strings.HasSuffix(string(uri), ".dingo")
}

func isDingoFilePath(path string) bool {
	return strings.HasSuffix(path, ".dingo")
}

func dingoToGoPath(dingoPath string) string {
	if !strings.HasSuffix(dingoPath, ".dingo") {
		return dingoPath
	}
	return strings.TrimSuffix(dingoPath, ".dingo") + ".go"
}

func goToDingoPath(goPath string) string {
	if !strings.HasSuffix(goPath, ".go") {
		return goPath
	}
	return strings.TrimSuffix(goPath, ".go") + ".dingo"
}
