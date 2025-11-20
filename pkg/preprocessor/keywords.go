package preprocessor

import (
	"regexp"
)

// Package-level compiled regex (Issue 2: Regex Performance)
var (
	// Match: let identifier(s) [: type] = expression
	// Handles:
	//   - Single: let x = 5
	//   - Multiple: let x, y, z = func()
	//   - With type: let name: string = "hello"
	//   - With complex type: let opt: Option<int> = Some(42)
	// Captures identifiers and optional type annotation
	letPattern = regexp.MustCompile(`\blet\s+([\w\s,]+?)(?:\s*:\s*[^=]+?)?\s*=`)
)

// KeywordProcessor converts Dingo keywords to Go keywords
type KeywordProcessor struct{}

// NewKeywordProcessor creates a new keyword processor
func NewKeywordProcessor() *KeywordProcessor {
	return &KeywordProcessor{}
}

// Name returns the processor name
func (k *KeywordProcessor) Name() string {
	return "keywords"
}

// Process transforms Dingo keywords to Go keywords
// Converts: let x = value → x := value
func (k *KeywordProcessor) Process(source []byte) ([]byte, []Mapping, error) {
	// Replace `let x = ` with `x := `
	result := letPattern.ReplaceAll(source, []byte("$1 :="))

	return result, nil, nil
}
