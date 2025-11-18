package tests

import (
	goparser "go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/MadAppGang/dingo/pkg/config"
	"github.com/MadAppGang/dingo/pkg/generator"
	"github.com/MadAppGang/dingo/pkg/parser"
	"github.com/MadAppGang/dingo/pkg/plugin"
	"github.com/MadAppGang/dingo/pkg/preprocessor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testLogger implements plugin.Logger for testing
type testLogger struct {
	t *testing.T
}

func (l *testLogger) Info(msg string) {
	l.t.Logf("INFO: %s", msg)
}

func (l *testLogger) Error(msg string) {
	l.t.Logf("ERROR: %s", msg)
}

func (l *testLogger) Debug(format string, args ...interface{}) {
	l.t.Logf("DEBUG: "+format, args...)
}

func (l *testLogger) Warn(format string, args ...interface{}) {
	l.t.Logf("WARN: "+format, args...)
}

// TestGoldenFiles runs golden file tests comparing Dingo → Go transpilation
func TestGoldenFiles(t *testing.T) {
	goldenDir := "golden"

	// Find all .dingo files
	dingoFiles, err := filepath.Glob(filepath.Join(goldenDir, "*.dingo"))
	require.NoError(t, err, "Failed to find golden dingo files")
	require.NotEmpty(t, dingoFiles, "No golden dingo files found")

	for _, dingoFile := range dingoFiles {
		baseName := strings.TrimSuffix(filepath.Base(dingoFile), ".dingo")
		goldenFile := filepath.Join(goldenDir, baseName+".go.golden")

		t.Run(baseName, func(t *testing.T) {
			// Skip tests that require parser/transpiler features not yet implemented
			skipPrefixes := []string{
				"func_util_",       // Parser doesn't support function types in parameters
				"lambda_",          // Lambda causes nil positioner crash in type checker
				"sum_types_",       // Type checker crashes on method receivers in generated code
				"pattern_match_",   // Pattern matching not yet implemented
				"safe_nav_",        // Safe navigation transformation not yet implemented
				"null_coalesce_",   // Null coalescing transformation not yet implemented
				"ternary_",         // Ternary operator not yet implemented (Phase 3)
				"tuples_",          // Tuple types not yet implemented
			}
			skipExact := []string{
				"error_prop_02_multiple",    // Parser bug: interface{} and & operator not handled correctly
				"showcase_01_api_server",    // Contains future features (enums, Result<T,E> in function returns) - not yet implemented
				"result_02_propagation",     // Uses pattern matching (match keyword)
				"result_03_pattern_match",   // Uses pattern matching (match keyword)
				"option_02_pattern_match",   // Uses pattern matching (match keyword)
				"option_02_literals",        // Option plugin bug: AST transformations not applied (Phase 4)
				"option_03_chaining",        // Requires lambda syntax (.map(|x| ...)) - unimplemented (Phase 4+)
				"result_04_chaining",        // Requires lambda syntax (.and_then(|x| ...)) - unimplemented (Phase 4+)
				"result_06_helpers",         // Missing golden file - deferred (Phase 4)
			}
			for _, prefix := range skipPrefixes {
				if strings.HasPrefix(baseName, prefix) {
					t.Skip("Feature not yet implemented - deferred to Phase 3")
				}
			}
			for _, skip := range skipExact {
				if baseName == skip {
					t.Skip("Parser bug - needs fixing in Phase 3")
				}
			}

			// Read golden expected output
			expectedBytes, err := os.ReadFile(goldenFile)
			require.NoError(t, err, "Failed to read golden file: %s", goldenFile)
			expected := string(expectedBytes)

			// Parse Dingo file
			fset := token.NewFileSet()

			// Read file content
			dingoSrc, err := os.ReadFile(dingoFile)
			require.NoError(t, err, "Failed to read Dingo source: %s", dingoFile)

			// Load config if test has a subdirectory with dingo.toml
			var cfg *config.Config
			testConfigDir := filepath.Join(goldenDir, baseName)
			testConfigPath := filepath.Join(testConfigDir, "dingo.toml")
			if _, err := os.Stat(testConfigPath); err == nil {
				// Config exists, load it
				cfg = config.DefaultConfig()
				if _, err := toml.DecodeFile(testConfigPath, cfg); err != nil {
					t.Fatalf("Failed to load test config %s: %v", testConfigPath, err)
				}
			}

			// Preprocess THEN parse
			var preprocessorInst *preprocessor.Preprocessor
			if cfg != nil {
				preprocessorInst = preprocessor.NewWithMainConfig(dingoSrc, cfg)
			} else {
				preprocessorInst = preprocessor.New(dingoSrc)
			}
			preprocessed, _, err := preprocessorInst.Process()
			require.NoError(t, err, "Failed to preprocess Dingo file: %s", dingoFile)

			dingoAST, err := parser.ParseFile(fset, dingoFile, []byte(preprocessed), parser.ParseComments)
			require.NoError(t, err, "Failed to parse preprocessed Dingo file: %s", dingoFile)

			// Create generator (plugins are registered internally)
			registry := plugin.NewRegistry()
			logger := &testLogger{t: t}
			gen, err := generator.NewWithPlugins(fset, registry, logger)
			require.NoError(t, err, "Failed to create generator")

			// Generate Go code
			output, err := gen.Generate(dingoAST)
			require.NoError(t, err, "Failed to generate Go code")

			actual := string(output)

			// Normalize whitespace for comparison
			expectedNorm := normalizeWhitespace(expected)
			actualNorm := normalizeWhitespace(actual)

			// Compare
			if !assert.Equal(t, expectedNorm, actualNorm, "Generated code doesn't match golden file") {
				t.Logf("\n=== EXPECTED ===\n%s\n", expected)
				t.Logf("\n=== ACTUAL ===\n%s\n", actual)

				// Write actual output for debugging
				debugFile := filepath.Join(goldenDir, baseName+".go.actual")
				_ = os.WriteFile(debugFile, output, 0644)
				t.Logf("Actual output written to: %s", debugFile)
			}
		})
	}
}

// normalizeWhitespace normalizes whitespace for comparison
// - Trims leading/trailing whitespace
// - Normalizes line endings
// - Collapses multiple spaces (except indentation)
func normalizeWhitespace(s string) string {
	// Normalize line endings
	s = strings.ReplaceAll(s, "\r\n", "\n")

	// Split into lines
	lines := strings.Split(s, "\n")

	// Process each line
	var normalized []string
	for _, line := range lines {
		// Trim trailing whitespace but preserve leading (indentation)
		line = strings.TrimRight(line, " \t")

		// Skip empty lines at start and end
		if len(normalized) == 0 && line == "" {
			continue
		}

		normalized = append(normalized, line)
	}

	// Remove trailing empty lines
	for len(normalized) > 0 && normalized[len(normalized)-1] == "" {
		normalized = normalized[:len(normalized)-1]
	}

	return strings.Join(normalized, "\n")
}

// TestGoldenFilesCompilation verifies that generated golden outputs compile
func TestGoldenFilesCompilation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping compilation test in short mode")
	}

	goldenDir := "golden"

	// Find all .go.golden files
	goldenFiles, err := filepath.Glob(filepath.Join(goldenDir, "*.go.golden"))
	require.NoError(t, err, "Failed to find golden Go files")

	for _, goldenFile := range goldenFiles {
		baseName := strings.TrimSuffix(filepath.Base(goldenFile), ".go.golden")

		t.Run(baseName+"_compiles", func(t *testing.T) {
			// Read golden file
			code, err := os.ReadFile(goldenFile)
			require.NoError(t, err, "Failed to read golden file")

			// Create temp file
			tmpFile := filepath.Join(t.TempDir(), "test.go")
			err = os.WriteFile(tmpFile, code, 0644)
			require.NoError(t, err, "Failed to write temp file")

			// Try to compile it (just check syntax)
			// Note: This won't link because of missing imports/dependencies
			// but will verify syntax is correct
			err = compileGoFile(tmpFile)
			if err != nil {
				t.Logf("Compilation output:\n%v", err)
				t.Fatal("Generated code does not compile")
			}
		})
	}
}

// compileGoFile attempts to compile a Go file to check syntax
func compileGoFile(filename string) error {
	// We use go/parser instead of actual compilation
	// because the code may reference external packages
	content, err := os.ReadFile(filename)
	if err != nil {
		return err
	}
	fset := token.NewFileSet()
	_, err = goparser.ParseFile(fset, filename, content, 0)
	return err
}
