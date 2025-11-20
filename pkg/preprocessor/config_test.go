package preprocessor

import (
	"strings"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg == nil {
		t.Fatal("DefaultConfig() returned nil")
	}

	if cfg.MultiValueReturnMode != "full" {
		t.Errorf("Expected default mode 'full', got %q", cfg.MultiValueReturnMode)
	}
}

func TestValidateMultiValueReturnMode(t *testing.T) {
	tests := []struct {
		name      string
		mode      string
		wantError bool
	}{
		{
			name:      "full mode is valid",
			mode:      "full",
			wantError: false,
		},
		{
			name:      "single mode is valid",
			mode:      "single",
			wantError: false,
		},
		{
			name:      "invalid mode returns error",
			mode:      "invalid",
			wantError: true,
		},
		{
			name:      "empty mode returns error",
			mode:      "",
			wantError: true,
		},
		{
			name:      "partial mode returns error",
			mode:      "ful",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				MultiValueReturnMode: tt.mode,
			}

			err := cfg.ValidateMultiValueReturnMode()

			if tt.wantError && err == nil {
				t.Errorf("Expected error for mode %q, got nil", tt.mode)
			}

			if !tt.wantError && err != nil {
				t.Errorf("Expected no error for mode %q, got: %v", tt.mode, err)
			}
		})
	}
}

func TestConfigFullMode_MultiValueReturns(t *testing.T) {
	// Test that "full" mode allows multi-value returns (3+ values)
	source := `package main

func processData(data string) (string, int, error) {
	let result = ReadFile(path)?
	return result, 42, nil
}
`

	config := &Config{
		MultiValueReturnMode: "full",
	}

	proc := NewErrorPropProcessorWithConfig(config)
	_, _, err := proc.Process([]byte(source))

	if err != nil {
		t.Errorf("Full mode should allow multi-value returns, got error: %v", err)
	}
}

func TestConfigSingleMode_MultiValueReturns(t *testing.T) {
	// Test that "single" mode rejects multi-value returns (3+ values)
	source := `package main

func processData(data string) (string, int, error) {
	return ReadFile(path)?
}
`

	config := &Config{
		MultiValueReturnMode: "single",
	}

	proc := NewErrorPropProcessorWithConfig(config)
	_, _, err := proc.Process([]byte(source))

	if err == nil {
		t.Error("Single mode should reject multi-value error propagation, got nil error")
	}

	// Verify error message contains expected information
	if !strings.Contains(err.Error(), "multi-value error propagation not allowed") {
		t.Errorf("Expected error to mention multi-value restriction, got: %v", err)
	}

	if !strings.Contains(err.Error(), "--multi-value-return=full") {
		t.Errorf("Expected error to mention --multi-value-return=full flag, got: %v", err)
	}
}

func TestConfigSingleMode_TwoValueReturns(t *testing.T) {
	// Test that "single" mode allows 2-value returns (T, error)
	source := `package main

func readConfig(path string) (string, error) {
	let content = ReadFile(path)?
	return content, nil
}
`

	config := &Config{
		MultiValueReturnMode: "single",
	}

	proc := NewErrorPropProcessorWithConfig(config)
	_, _, err := proc.Process([]byte(source))

	if err != nil {
		t.Errorf("Single mode should allow (T, error) returns, got error: %v", err)
	}
}

func TestConfigSingleMode_ReturnStatement(t *testing.T) {
	// Test return statement specifically in single mode with multi-value function
	source := `package main

func getData() (int, string, error) {
	return fetchData()?
}
`

	config := &Config{
		MultiValueReturnMode: "single",
	}

	proc := NewErrorPropProcessorWithConfig(config)
	_, _, err := proc.Process([]byte(source))

	if err == nil {
		t.Error("Single mode should reject return statement with multi-value propagation")
	}

	// Verify it reports correct number of values
	if !strings.Contains(err.Error(), "2 values") {
		t.Errorf("Expected error to mention '2 values', got: %v", err)
	}
}

func TestConfigNilDefault(t *testing.T) {
	// Test that nil config defaults to "full" mode
	proc := NewErrorPropProcessorWithConfig(nil)

	if proc.config == nil {
		t.Fatal("NewErrorPropProcessorWithConfig(nil) should create default config")
	}

	if proc.config.MultiValueReturnMode != "full" {
		t.Errorf("Nil config should default to 'full' mode, got %q", proc.config.MultiValueReturnMode)
	}
}

func TestConfigFullMode_ComplexCase(t *testing.T) {
	// Test complex multi-value return case in full mode
	source := `package main

func complexOperation() (string, int, bool, error) {
	return processComplex()?
}
`

	config := &Config{
		MultiValueReturnMode: "full",
	}

	proc := NewErrorPropProcessorWithConfig(config)
	result, _, err := proc.Process([]byte(source))

	if err != nil {
		t.Errorf("Full mode should allow 3+ value returns, got error: %v", err)
	}

	// Verify the transformation generated multiple temporary variables
	resultStr := string(result)
	if !strings.Contains(resultStr, "tmp1") {
		t.Error("Expected tmp1 in output")
	}
	if !strings.Contains(resultStr, "tmp2") {
		t.Error("Expected tmp2 in output")
	}
	if !strings.Contains(resultStr, "tmp3") {
		t.Error("Expected tmp3 in output")
	}
}

func TestConfigSingleMode_AssignmentAllowed(t *testing.T) {
	// Test that single mode doesn't restrict assignments (only return statements)
	source := `package main

func readData() (string, error) {
	let data = ReadFile(path)?
	return data, nil
}
`

	config := &Config{
		MultiValueReturnMode: "single",
	}

	proc := NewErrorPropProcessorWithConfig(config)
	_, _, err := proc.Process([]byte(source))

	if err != nil {
		t.Errorf("Single mode should allow assignments with ?, got error: %v", err)
	}
}
