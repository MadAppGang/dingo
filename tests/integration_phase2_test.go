package tests

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestIntegrationPhase2EndToEnd tests the complete pipeline:
// .dingo → transpile → .go → compile → run
func TestIntegrationPhase2EndToEnd(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "dingo-integration-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Test case 1: Error propagation with Result type
	t.Run("error_propagation_result_type", func(t *testing.T) {
		dingoFile := filepath.Join(tmpDir, "test_result.dingo")
		dingoCode := `package main

import "fmt"

type Result<T, E> struct {
	tag ResultTag
	ok_0 *T
	err_0 *E
}

func processNumber(s string) (int, error) {
	num := parseInt(s)?
	return num * 2, nil
}

func parseInt(s string) (int, error) {
	return 42, nil
}

func main() {
	result, err := processNumber("21")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Println("Result:", result)
}
`
		if err := os.WriteFile(dingoFile, []byte(dingoCode), 0644); err != nil {
			t.Fatal(err)
		}

		// Build with dingo CLI
		goFile := strings.TrimSuffix(dingoFile, ".dingo") + ".go"
		cmd := exec.Command("go", "run", "../../cmd/dingo", "build", dingoFile)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Logf("Build output: %s", output)
			t.Fatalf("Failed to transpile: %v", err)
		}

		// Verify .go file was created
		if _, err := os.Stat(goFile); os.IsNotExist(err) {
			t.Fatal("Generated .go file not found")
		}

		// Compile the generated Go file
		cmd = exec.Command("go", "build", "-o", filepath.Join(tmpDir, "test_result"), goFile)
		output, err = cmd.CombinedOutput()
		if err != nil {
			t.Logf("Generated Go file:\n%s", mustReadFile(t, goFile))
			t.Logf("Compile output: %s", output)
			t.Fatalf("Failed to compile generated Go: %v", err)
		}

		t.Log("✓ Integration test passed: .dingo → .go → compile successful")
	})

	// Test case 2: Enum type generation
	t.Run("enum_type_generation", func(t *testing.T) {
		dingoFile := filepath.Join(tmpDir, "test_enum.dingo")
		dingoCode := `package main

enum Status {
	Pending,
	Active,
	Complete
}

func main() {
	s := Status_Active()
	if s.IsActive() {
		println("Status is active")
	}
}
`
		if err := os.WriteFile(dingoFile, []byte(dingoCode), 0644); err != nil {
			t.Fatal(err)
		}

		// Build with dingo CLI
		goFile := strings.TrimSuffix(dingoFile, ".dingo") + ".go"
		cmd := exec.Command("go", "run", "../../cmd/dingo", "build", dingoFile)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Logf("Build output: %s", output)
			t.Fatalf("Failed to transpile: %v", err)
		}

		// Verify .go file was created
		if _, err := os.Stat(goFile); os.IsNotExist(err) {
			t.Fatal("Generated .go file not found")
		}

		// Compile the generated Go file
		cmd = exec.Command("go", "build", "-o", filepath.Join(tmpDir, "test_enum"), goFile)
		output, err = cmd.CombinedOutput()
		if err != nil {
			t.Logf("Generated Go file:\n%s", mustReadFile(t, goFile))
			t.Logf("Compile output: %s", output)
			t.Fatalf("Failed to compile generated Go: %v", err)
		}

		// Verify enum code was generated
		goCode := mustReadFile(t, goFile)
		if !strings.Contains(goCode, "StatusTag") {
			t.Error("Generated code missing StatusTag enum")
		}
		if !strings.Contains(goCode, "Status_Pending") {
			t.Error("Generated code missing Status_Pending constructor")
		}
		if !strings.Contains(goCode, "IsActive()") {
			t.Error("Generated code missing IsActive() method")
		}

		t.Log("✓ Integration test passed: Enum generation working correctly")
	})
}

func mustReadFile(t *testing.T, path string) string {
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read file %s: %v", path, err)
	}
	return string(content)
}
