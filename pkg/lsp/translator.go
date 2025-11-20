package lsp

import (
	"bufio"
	"fmt"
	"log"
	"os"
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
		// For Go->Dingo without map, return error with original URI
		return uri, pos, fmt.Errorf("source map not found: %s", goPath)
	}

	// Translate position
	var newLine, newCol int
	var newURI protocol.DocumentURI

	if dir == DingoToGo {
		newLine, newCol = sm.MapToGenerated(line, col)
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

	// PHASE 1 FIX: Add bounds checking to prevent "column beyond end of line" errors
	// This clamps the column to the line length, providing graceful degradation
	newPos = clampPositionToLine(newPos, newURI.Filename())

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

// clampPositionToLine ensures the column doesn't exceed the line length
// This prevents "column is beyond end of line" errors from gopls
// PHASE 1 FIX: Graceful degradation for inaccurate source map mappings
func clampPositionToLine(pos protocol.Position, filePath string) protocol.Position {
	log.Printf("[LSP Translator] clampPositionToLine called: file=%s, line=%d, col=%d",
		filePath, pos.Line, pos.Character)

	// Read the file and get the line length
	lineLength, err := getLineLength(filePath, int(pos.Line))
	if err != nil {
		// CRITICAL: Log the error so we can diagnose why clamping isn't working
		log.Printf("[LSP Translator] ERROR: Failed to get line length: %v (file: %s, line: %d) - RETURNING UNCLAMPED POSITION",
			err, filePath, pos.Line)
		// We MUST clamp to 0 if we can't read the file, otherwise gopls will fail
		// Return column 0 as safest fallback
		log.Printf("[LSP Translator] EMERGENCY FALLBACK: Clamping column to 0 to prevent gopls crash")
		pos.Character = 0
		return pos
	}

	// LSP positions are 0-based, so valid range is [0, lineLength]
	// lineLength is the number of characters, so max valid column is lineLength
	maxCol := uint32(lineLength)

	log.Printf("[LSP Translator] Line length: %d, max valid column: %d", lineLength, maxCol)

	if pos.Character > maxCol {
		log.Printf("[LSP Translator] WARNING: Column %d exceeds line length %d (file: %s, line: %d), clamping to %d",
			pos.Character, lineLength, filePath, pos.Line, maxCol)
		pos.Character = maxCol
	} else {
		log.Printf("[LSP Translator] Column %d is within bounds (max: %d), no clamping needed",
			pos.Character, maxCol)
	}

	return pos
}

// getLineLength returns the length of a specific line in a file (0-based line number)
func getLineLength(filePath string, lineNum int) (int, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return 0, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	currentLine := 0

	for scanner.Scan() {
		if currentLine == lineNum {
			// Return the length of this line
			return len(scanner.Text()), nil
		}
		currentLine++
	}

	if err := scanner.Err(); err != nil {
		return 0, fmt.Errorf("error reading file: %w", err)
	}

	// Line number is beyond file length
	return 0, fmt.Errorf("line %d not found in file (file has %d lines)", lineNum, currentLine)
}
