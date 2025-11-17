package tests

import (
	"bytes"
	"fmt"
	"go/ast"
	goparser "go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MadAppGang/dingo/pkg/generator"
	"github.com/MadAppGang/dingo/pkg/parser"
	"github.com/MadAppGang/dingo/pkg/plugin"
	"github.com/MadAppGang/dingo/pkg/plugin/builtin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEndToEndTranspilation tests the full pipeline: parse → transform → generate
func TestEndToEndTranspilation(t *testing.T) {
	tests := []struct {
		name    string
		dingo   string
		wantErr bool
	}{
		{
			name: "simple_error_propagation",
			dingo: `package main
func readFile(path: string) ([]byte, error) {
	let data = ReadFile(path)?
	return data, nil
}`,
			wantErr: false,
		},
		{
			name: "error_wrapping",
			dingo: `package main
func readConfig() ([]byte, error) {
	let data = ReadFile("config.json")? "failed to read config"
	return data, nil
}`,
			wantErr: false,
		},
		{
			name: "expression_context",
			dingo: `package main
func parse(s: string) (int, error) {
	return Atoi(s)?
}`,
			wantErr: false,
		},
		{
			name: "multiple_propagations",
			dingo: `package main
func load() (map[string]interface{}, error) {
	let data = ReadFile("data.json")?
	var result map[string]interface{}
	let err = Unmarshal(data, &result)?
	return result, nil
}`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse
			fset := token.NewFileSet()

			// Write to temp file for parsing
			tmpDingo := filepath.Join(t.TempDir(), "test.dingo")
			err := os.WriteFile(tmpDingo, []byte(tt.dingo), 0644)
			require.NoError(t, err)

			dingoAST, err := parser.ParseFile(fset, tmpDingo, []byte(tt.dingo), 0)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)

			// Transform
			registry := plugin.NewRegistry()

			// Register sum_types first (dependency of error_propagation)
			sumTypesPlugin := builtin.NewSumTypesPlugin()
			err = registry.Register(sumTypesPlugin)
			require.NoError(t, err)

			errPropPlugin := builtin.NewErrorPropagationPlugin()
			err = registry.Register(errPropPlugin)
			require.NoError(t, err)

			logger := &testLogger{t: t}
			gen, err := generator.NewWithPlugins(fset, registry, logger)
			require.NoError(t, err)

			// Generate
			output, err := gen.Generate(dingoAST)
			require.NoError(t, err)
			require.NotEmpty(t, output)

			// Verify it's valid Go syntax
			_, err = goparser.ParseFile(fset, "output.go", output, 0)
			require.NoError(t, err, "Generated code is not valid Go:\n%s", string(output))

			t.Logf("Generated code:\n%s", string(output))
		})
	}
}

// TestGeneratedCodeCompiles verifies generated code compiles with go build
func TestGeneratedCodeCompiles(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping compilation test in short mode")
	}

	tests := []struct {
		name  string
		dingo string
	}{
		{
			name: "complete_program",
			dingo: `package main

func readFile(path: string) (string, error) {
	let data = ReadFile(path)?
	return string(data), nil
}

func main() {
	Println("Hello")
}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp directory
			tmpDir := t.TempDir()

			// Parse and generate
			fset := token.NewFileSet()

			dingoFile := filepath.Join(tmpDir, "main.dingo")
			err := os.WriteFile(dingoFile, []byte(tt.dingo), 0644)
			require.NoError(t, err)

			dingoAST, err := parser.ParseFile(fset, dingoFile, []byte(tt.dingo), 0)
			require.NoError(t, err)

			registry := plugin.NewRegistry()

			// Register sum_types first (dependency of error_propagation)
			sumTypesPlugin := builtin.NewSumTypesPlugin()
			err = registry.Register(sumTypesPlugin)
			require.NoError(t, err)

			errPropPlugin := builtin.NewErrorPropagationPlugin()
			err = registry.Register(errPropPlugin)
			require.NoError(t, err)

			gen, err := generator.NewWithPlugins(fset, registry, &testLogger{t: t})
			require.NoError(t, err)

			output, err := gen.Generate(dingoAST)
			require.NoError(t, err)

			// Write generated Go file
			goFile := filepath.Join(tmpDir, "main.go")
			err = os.WriteFile(goFile, output, 0644)
			require.NoError(t, err)

			// Initialize go module
			cmd := exec.Command("go", "mod", "init", "test")
			cmd.Dir = tmpDir
			err = cmd.Run()
			require.NoError(t, err)

			// Try to build
			cmd = exec.Command("go", "build", "-o", "test")
			cmd.Dir = tmpDir
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			err = cmd.Run()
			if err != nil {
				t.Logf("Build error:\n%s", stderr.String())
				t.Logf("Generated code:\n%s", string(output))
				t.Fatalf("Failed to build: %v", err)
			}

			t.Logf("✓ Code compiles successfully")
		})
	}
}

// TestImportInjection verifies fmt import is added when needed
func TestImportInjection(t *testing.T) {
	tests := []struct {
		name      string
		dingo     string
		wantFmt   bool
		wantCount int // expected import count
	}{
		{
			name: "no_error_wrapping",
			dingo: `package main
func read() ([]byte, error) {
	let data = ReadFile("test")?
	return data, nil
}`,
			wantFmt:   false,
			wantCount: 0, // no imports
		},
		{
			name: "with_error_wrapping",
			dingo: `package main
func read() ([]byte, error) {
	let data = ReadFile("test")? "read failed"
	return data, nil
}`,
			wantFmt:   true,
			wantCount: 1, // just "fmt"
		},
		{
			name: "multiple_wrappings",
			dingo: `package main
func read() ([]byte, error) {
	let data1 = ReadFile("test1")? "read1 failed"
	let data2 = ReadFile("test2")? "read2 failed"
	return data1, nil
}`,
			wantFmt:   true,
			wantCount: 1, // just "fmt"
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fset := token.NewFileSet()
			

			tmpFile := filepath.Join(t.TempDir(), "test.dingo")
			err := os.WriteFile(tmpFile, []byte(tt.dingo), 0644)
			require.NoError(t, err)

			dingoAST, err := parser.ParseFile(fset, tmpFile, []byte(tt.dingo), 0)
			require.NoError(t, err)

			registry := plugin.NewRegistry()

			// Register sum_types first (dependency of error_propagation)
			sumTypesPlugin := builtin.NewSumTypesPlugin()
			err = registry.Register(sumTypesPlugin)
			require.NoError(t, err)

			errPropPlugin := builtin.NewErrorPropagationPlugin()
			err = registry.Register(errPropPlugin)
			require.NoError(t, err)

			gen, err := generator.NewWithPlugins(fset, registry, &testLogger{t: t})
			require.NoError(t, err)

			output, err := gen.Generate(dingoAST)
			require.NoError(t, err)

			// Parse generated code and check imports
			genFile, err := goparser.ParseFile(fset, "output.go", output, 0)
			require.NoError(t, err)

			hasFmt := false
			importCount := 0
			for _, imp := range genFile.Imports {
				importCount++
				if imp.Path.Value == `"fmt"` {
					hasFmt = true
				}
			}

			assert.Equal(t, tt.wantFmt, hasFmt, "fmt import presence mismatch")
			assert.Equal(t, tt.wantCount, importCount, "import count mismatch")

			t.Logf("Imports: %d, has fmt: %v", importCount, hasFmt)
		})
	}
}

// TestTypeInferenceIntegration tests type inference in real transpilation
func TestTypeInferenceIntegration(t *testing.T) {
	tests := []struct {
		name         string
		dingo        string
		wantZeroVal  string // what zero value should appear in output
		wantContains string // what the generated code should contain
	}{
		{
			name: "int_return",
			dingo: `package main
func parse(s: string) (int, error) {
	let x = Atoi(s)?
	return x, nil
}`,
			wantContains: "return 0,",
		},
		{
			name: "string_return",
			dingo: `package main
func read() (string, error) {
	let data = ReadFile("test")?
	return string(data), nil
}`,
			wantContains: `return "",`,
		},
		{
			name: "pointer_return",
			dingo: `package main
type User struct { Name string }
func get() (*User, error) {
	let data = ReadFile("user")?
	return &User{Name: string(data)}, nil
}`,
			wantContains: "return nil,",
		},
		{
			name: "slice_return",
			dingo: `package main
func get() ([]byte, error) {
	let data = ReadFile("test")?
	return data, nil
}`,
			wantContains: "return nil,",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fset := token.NewFileSet()
			

			tmpFile := filepath.Join(t.TempDir(), "test.dingo")
			err := os.WriteFile(tmpFile, []byte(tt.dingo), 0644)
			require.NoError(t, err)

			dingoAST, err := parser.ParseFile(fset, tmpFile, []byte(tt.dingo), 0)
			require.NoError(t, err)

			registry := plugin.NewRegistry()

			// Register sum_types first (dependency of error_propagation)
			sumTypesPlugin := builtin.NewSumTypesPlugin()
			err = registry.Register(sumTypesPlugin)
			require.NoError(t, err)

			errPropPlugin := builtin.NewErrorPropagationPlugin()
			err = registry.Register(errPropPlugin)
			require.NoError(t, err)

			gen, err := generator.NewWithPlugins(fset, registry, &testLogger{t: t})
			require.NoError(t, err)

			output, err := gen.Generate(dingoAST)
			require.NoError(t, err)

			outputStr := string(output)

			if tt.wantContains != "" {
				assert.Contains(t, outputStr, tt.wantContains,
					"Generated code should contain correct zero value")
			}

			t.Logf("Generated:\n%s", outputStr)
		})
	}
}

// TestErrorCases tests graceful handling of edge cases
func TestErrorCases(t *testing.T) {
	tests := []struct {
		name      string
		dingo     string
		shouldErr bool
		skipGen   bool // if true, expect parse to fail
	}{
		{
			name: "function_without_error_return",
			dingo: `package main
func test() int {
	let x = getValue()?
	return x
}`,
			shouldErr: false, // Should handle gracefully with nil fallback
			skipGen:   false,
		},
		{
			name: "empty_file",
			dingo: `package main`,
			shouldErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fset := token.NewFileSet()
			

			tmpFile := filepath.Join(t.TempDir(), "test.dingo")
			err := os.WriteFile(tmpFile, []byte(tt.dingo), 0644)
			require.NoError(t, err)

			dingoAST, err := parser.ParseFile(fset, tmpFile, []byte(tt.dingo), 0)
			if tt.skipGen {
				if tt.shouldErr {
					assert.Error(t, err)
				}
				return
			}
			require.NoError(t, err)

			registry := plugin.NewRegistry()

			// Register sum_types first (dependency of error_propagation)
			sumTypesPlugin := builtin.NewSumTypesPlugin()
			err = registry.Register(sumTypesPlugin)
			require.NoError(t, err)

			errPropPlugin := builtin.NewErrorPropagationPlugin()
			err = registry.Register(errPropPlugin)
			require.NoError(t, err)

			gen, err := generator.NewWithPlugins(fset, registry, &testLogger{t: t})
			require.NoError(t, err)

			output, err := gen.Generate(dingoAST)
			if tt.shouldErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, output)
			}
		})
	}
}

// TestStatementInjectionOrder verifies statements are injected in correct order
func TestStatementInjectionOrder(t *testing.T) {
	dingo := `package main
func multi() ([]byte, error) {
	let a = ReadFile("a")?
	let b = ReadFile("b")?
	let c = ReadFile("c")?
	return append(a, b...), nil
}`

	fset := token.NewFileSet()

	tmpFile := filepath.Join(t.TempDir(), "test.dingo")
	err := os.WriteFile(tmpFile, []byte(dingo), 0644)
	require.NoError(t, err)

	dingoAST, err := parser.ParseFile(fset, tmpFile, []byte(dingo), 0)
	require.NoError(t, err)

	registry := plugin.NewRegistry()

	// Register sum_types first (dependency of error_propagation)
	sumTypesPlugin := builtin.NewSumTypesPlugin()
	err = registry.Register(sumTypesPlugin)
	require.NoError(t, err)

	errPropPlugin := builtin.NewErrorPropagationPlugin()
	err = registry.Register(errPropPlugin)
	require.NoError(t, err)

	gen, err := generator.NewWithPlugins(fset, registry, &testLogger{t: t})
	require.NoError(t, err)

	output, err := gen.Generate(dingoAST)
	require.NoError(t, err)

	outputStr := string(output)

	// Verify error variables are sequential
	assert.Contains(t, outputStr, "__err0")
	assert.Contains(t, outputStr, "__err1")
	assert.Contains(t, outputStr, "__err2")

	// Verify order: __err0 appears before __err1
	idx0 := strings.Index(outputStr, "__err0")
	idx1 := strings.Index(outputStr, "__err1")
	idx2 := strings.Index(outputStr, "__err2")

	assert.Greater(t, idx1, idx0, "__err1 should appear after __err0")
	assert.Greater(t, idx2, idx1, "__err2 should appear after __err1")

	t.Logf("Generated:\n%s", outputStr)
}

// parseGoFile parses a Go file (helper for testing)
func ParseGoFile(fset *token.FileSet, filename string) (*ast.File, error) {
	content, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	return goparser.ParseFile(fset, filename, content, 0)
}
