package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Features.ErrorPropagationSyntax != SyntaxQuestion {
		t.Errorf("Expected default syntax to be 'question', got %q", cfg.Features.ErrorPropagationSyntax)
	}

	if !cfg.SourceMap.Enabled {
		t.Error("Expected source maps to be enabled by default")
	}

	if cfg.SourceMap.Format != FormatInline {
		t.Errorf("Expected default format to be 'inline', got %q", cfg.SourceMap.Format)
	}

	// Test Match defaults
	if cfg.Match.Syntax != "rust" {
		t.Errorf("Expected default match syntax to be 'rust', got %q", cfg.Match.Syntax)
	}

	// Test Lambda style defaults
	if cfg.Features.LambdaStyle != "typescript" {
		t.Errorf("Expected default lambda_style to be 'typescript', got %q", cfg.Features.LambdaStyle)
	}

	// Test Result type defaults
	if !cfg.Features.ResultType.Enabled {
		t.Error("Expected Result type to be enabled by default")
	}
	if cfg.Features.ResultType.GoInterop != "opt-in" {
		t.Errorf("Expected Result go_interop default to be 'opt-in', got %q", cfg.Features.ResultType.GoInterop)
	}

	// Test Option type defaults
	if !cfg.Features.OptionType.Enabled {
		t.Error("Expected Option type to be enabled by default")
	}
	if cfg.Features.OptionType.GoInterop != "opt-in" {
		t.Errorf("Expected Option go_interop default to be 'opt-in', got %q", cfg.Features.OptionType.GoInterop)
	}
}

func TestSyntaxStyleValidation(t *testing.T) {
	tests := []struct {
		style SyntaxStyle
		valid bool
	}{
		{SyntaxQuestion, true},
		{SyntaxBang, true},
		{SyntaxTry, true},
		{SyntaxStyle("invalid"), false},
		{SyntaxStyle(""), false},
		{SyntaxStyle("QUESTION"), false}, // Case sensitive
	}

	for _, tt := range tests {
		t.Run(string(tt.style), func(t *testing.T) {
			got := tt.style.IsValid()
			if got != tt.valid {
				t.Errorf("IsValid() = %v, want %v for %q", got, tt.valid, tt.style)
			}
		})
	}
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name      string
		config    *Config
		wantError bool
		errorMsg  string
	}{
		{
			name:      "valid default config",
			config:    DefaultConfig(),
			wantError: false,
		},
		{
			name: "valid bang syntax",
			config: &Config{
				Features: FeatureConfig{
					ErrorPropagationSyntax: SyntaxBang,
				},
				SourceMap: SourceMapConfig{
					Enabled: true,
					Format:  FormatSeparate,
				},
			},
			wantError: false,
		},
		{
			name: "invalid syntax",
			config: &Config{
				Features: FeatureConfig{
					ErrorPropagationSyntax: SyntaxStyle("invalid"),
				},
				SourceMap: SourceMapConfig{
					Enabled: true,
					Format:  FormatInline,
				},
			},
			wantError: true,
			errorMsg:  "invalid error_propagation_syntax",
		},
		{
			name: "invalid source map format",
			config: &Config{
				Features: FeatureConfig{
					ErrorPropagationSyntax: SyntaxQuestion,
				},
				SourceMap: SourceMapConfig{
					Enabled: true,
					Format:  SourceMapFormat("bad_format"),
				},
			},
			wantError: true,
			errorMsg:  "invalid sourcemap format",
		},
		{
			name: "valid result go_interop opt-in",
			config: &Config{
				Features: FeatureConfig{
					ErrorPropagationSyntax: SyntaxQuestion,
					ResultType: ResultTypeConfig{
						Enabled:   true,
						GoInterop: "opt-in",
					},
				},
				SourceMap: SourceMapConfig{
					Enabled: true,
					Format:  FormatInline,
				},
			},
			wantError: false,
		},
		{
			name: "valid result go_interop auto",
			config: &Config{
				Features: FeatureConfig{
					ErrorPropagationSyntax: SyntaxQuestion,
					ResultType: ResultTypeConfig{
						Enabled:   true,
						GoInterop: "auto",
					},
				},
				SourceMap: SourceMapConfig{
					Enabled: true,
					Format:  FormatInline,
				},
			},
			wantError: false,
		},
		{
			name: "valid result go_interop disabled",
			config: &Config{
				Features: FeatureConfig{
					ErrorPropagationSyntax: SyntaxQuestion,
					ResultType: ResultTypeConfig{
						Enabled:   true,
						GoInterop: "disabled",
					},
				},
				SourceMap: SourceMapConfig{
					Enabled: true,
					Format:  FormatInline,
				},
			},
			wantError: false,
		},
		{
			name: "invalid result go_interop",
			config: &Config{
				Features: FeatureConfig{
					ErrorPropagationSyntax: SyntaxQuestion,
					ResultType: ResultTypeConfig{
						Enabled:   true,
						GoInterop: "invalid-mode",
					},
				},
				SourceMap: SourceMapConfig{
					Enabled: true,
					Format:  FormatInline,
				},
			},
			wantError: true,
			errorMsg:  "invalid result_type.go_interop",
		},
		{
			name: "valid option go_interop opt-in",
			config: &Config{
				Features: FeatureConfig{
					ErrorPropagationSyntax: SyntaxQuestion,
					OptionType: OptionTypeConfig{
						Enabled:   true,
						GoInterop: "opt-in",
					},
				},
				SourceMap: SourceMapConfig{
					Enabled: true,
					Format:  FormatInline,
				},
			},
			wantError: false,
		},
		{
			name: "valid option go_interop auto",
			config: &Config{
				Features: FeatureConfig{
					ErrorPropagationSyntax: SyntaxQuestion,
					OptionType: OptionTypeConfig{
						Enabled:   true,
						GoInterop: "auto",
					},
				},
				SourceMap: SourceMapConfig{
					Enabled: true,
					Format:  FormatInline,
				},
			},
			wantError: false,
		},
		{
			name: "valid option go_interop disabled",
			config: &Config{
				Features: FeatureConfig{
					ErrorPropagationSyntax: SyntaxQuestion,
					OptionType: OptionTypeConfig{
						Enabled:   true,
						GoInterop: "disabled",
					},
				},
				SourceMap: SourceMapConfig{
					Enabled: true,
					Format:  FormatInline,
				},
			},
			wantError: false,
		},
		{
			name: "invalid option go_interop",
			config: &Config{
				Features: FeatureConfig{
					ErrorPropagationSyntax: SyntaxQuestion,
					OptionType: OptionTypeConfig{
						Enabled:   true,
						GoInterop: "wrong",
					},
				},
				SourceMap: SourceMapConfig{
					Enabled: true,
					Format:  FormatInline,
				},
			},
			wantError: true,
			errorMsg:  "invalid option_type.go_interop",
		},
		{
			name: "both result and option configured",
			config: &Config{
				Features: FeatureConfig{
					ErrorPropagationSyntax: SyntaxQuestion,
					ResultType: ResultTypeConfig{
						Enabled:   true,
						GoInterop: "auto",
					},
					OptionType: OptionTypeConfig{
						Enabled:   true,
						GoInterop: "opt-in",
					},
				},
				SourceMap: SourceMapConfig{
					Enabled: true,
					Format:  FormatInline,
				},
			},
			wantError: false,
		},
		{
			name: "valid match syntax rust",
			config: &Config{
				Features: FeatureConfig{
					ErrorPropagationSyntax: SyntaxQuestion,
				},
				Match: MatchConfig{
					Syntax: "rust",
				},
				SourceMap: SourceMapConfig{
					Enabled: true,
					Format:  FormatInline,
				},
			},
			wantError: false,
		},
		{
			name: "invalid match syntax swift (removed in Phase 4.2)",
			config: &Config{
				Features: FeatureConfig{
					ErrorPropagationSyntax: SyntaxQuestion,
				},
				Match: MatchConfig{
					Syntax: "swift",
				},
				SourceMap: SourceMapConfig{
					Enabled: true,
					Format:  FormatInline,
				},
			},
			wantError: true,
		},
		{
			name: "invalid match syntax",
			config: &Config{
				Features: FeatureConfig{
					ErrorPropagationSyntax: SyntaxQuestion,
				},
				Match: MatchConfig{
					Syntax: "invalid",
				},
				SourceMap: SourceMapConfig{
					Enabled: true,
					Format:  FormatInline,
				},
			},
			wantError: true,
			errorMsg:  "invalid match.syntax",
		},
		{
			name: "empty match syntax uses default",
			config: &Config{
				Features: FeatureConfig{
					ErrorPropagationSyntax: SyntaxQuestion,
				},
				Match: MatchConfig{
					Syntax: "",
				},
				SourceMap: SourceMapConfig{
					Enabled: true,
					Format:  FormatInline,
				},
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantError {
				if err == nil {
					t.Errorf("Expected error containing %q, got nil", tt.errorMsg)
				} else if tt.errorMsg != "" && !contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing %q, got %q", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got %v", err)
				}
			}
		})
	}
}

func TestLoadConfigNoFiles(t *testing.T) {
	// Create temp directory without config files
	tmpDir, err := os.MkdirTemp("", "dingo-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Change to temp directory
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(oldWd) }()

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	// Override HOME to avoid loading user config
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	// Load config - should use defaults
	cfg, err := Load(nil)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Features.ErrorPropagationSyntax != SyntaxQuestion {
		t.Errorf("Expected default syntax 'question', got %q", cfg.Features.ErrorPropagationSyntax)
	}
}

func TestLoadConfigProjectFile(t *testing.T) {
	// Create temp directory with project config
	tmpDir, err := os.MkdirTemp("", "dingo-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Write project config
	projectConfig := `[features]
error_propagation_syntax = "bang"

[sourcemaps]
enabled = true
format = "separate"
`
	configPath := filepath.Join(tmpDir, "dingo.toml")
	if err := os.WriteFile(configPath, []byte(projectConfig), 0644); err != nil {
		t.Fatal(err)
	}

	// Change to temp directory
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(oldWd) }()

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	// Override HOME to avoid loading user config
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	// Load config
	cfg, err := Load(nil)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Features.ErrorPropagationSyntax != SyntaxBang {
		t.Errorf("Expected syntax 'bang' from project config, got %q", cfg.Features.ErrorPropagationSyntax)
	}

	if cfg.SourceMap.Format != FormatSeparate {
		t.Errorf("Expected format 'separate' from project config, got %q", cfg.SourceMap.Format)
	}
}

func TestLoadConfigCLIOverride(t *testing.T) {
	// Create temp directory with project config
	tmpDir, err := os.MkdirTemp("", "dingo-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Write project config with "bang"
	projectConfig := `[features]
error_propagation_syntax = "bang"
`
	configPath := filepath.Join(tmpDir, "dingo.toml")
	if err := os.WriteFile(configPath, []byte(projectConfig), 0644); err != nil {
		t.Fatal(err)
	}

	// Change to temp directory
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(oldWd) }()

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	// Override HOME
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	// Load config with CLI override to "try"
	overrides := &Config{
		Features: FeatureConfig{
			ErrorPropagationSyntax: SyntaxTry,
		},
	}

	cfg, err := Load(overrides)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// CLI override should win
	if cfg.Features.ErrorPropagationSyntax != SyntaxTry {
		t.Errorf("Expected syntax 'try' from CLI override, got %q", cfg.Features.ErrorPropagationSyntax)
	}
}

func TestLoadConfigInvalidTOML(t *testing.T) {
	// Create temp directory with invalid TOML
	tmpDir, err := os.MkdirTemp("", "dingo-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Write invalid TOML
	invalidConfig := `[features
error_propagation_syntax = "bang"  # Missing closing bracket
`
	configPath := filepath.Join(tmpDir, "dingo.toml")
	if err := os.WriteFile(configPath, []byte(invalidConfig), 0644); err != nil {
		t.Fatal(err)
	}

	// Change to temp directory
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(oldWd) }()

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	// Override HOME
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	// Load should fail
	_, err = Load(nil)
	if err == nil {
		t.Error("Expected error for invalid TOML, got nil")
	}
}

func TestLoadConfigInvalidValue(t *testing.T) {
	// Create temp directory with invalid syntax value
	tmpDir, err := os.MkdirTemp("", "dingo-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Write config with invalid syntax
	invalidConfig := `[features]
error_propagation_syntax = "invalid_syntax"
`
	configPath := filepath.Join(tmpDir, "dingo.toml")
	if err := os.WriteFile(configPath, []byte(invalidConfig), 0644); err != nil {
		t.Fatal(err)
	}

	// Change to temp directory
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(oldWd) }()

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	// Override HOME
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	// Load should fail validation
	_, err = Load(nil)
	if err == nil {
		t.Error("Expected validation error, got nil")
	}
	if !contains(err.Error(), "invalid configuration") {
		t.Errorf("Expected 'invalid configuration' error, got %v", err)
	}
}

func TestLoadConfigMatchSyntax(t *testing.T) {
	// Create temp directory with match config
	tmpDir, err := os.MkdirTemp("", "dingo-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Write project config with match syntax (Rust-only)
	projectConfig := `[match]
syntax = "rust"

[features]
error_propagation_syntax = "question"
`
	configPath := filepath.Join(tmpDir, "dingo.toml")
	if err := os.WriteFile(configPath, []byte(projectConfig), 0644); err != nil {
		t.Fatal(err)
	}

	// Change to temp directory
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(oldWd) }()

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	// Override HOME
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	// Load config
	cfg, err := Load(nil)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Match.Syntax != "rust" {
		t.Errorf("Expected match syntax 'rust' from project config, got %q", cfg.Match.Syntax)
	}
}

func TestLoadConfigInvalidMatchSyntax(t *testing.T) {
	// Create temp directory with invalid match syntax
	tmpDir, err := os.MkdirTemp("", "dingo-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Write config with invalid match syntax
	invalidConfig := `[match]
syntax = "scala"
`
	configPath := filepath.Join(tmpDir, "dingo.toml")
	if err := os.WriteFile(configPath, []byte(invalidConfig), 0644); err != nil {
		t.Fatal(err)
	}

	// Change to temp directory
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(oldWd) }()

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	// Override HOME
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	// Load should fail validation
	_, err = Load(nil)
	if err == nil {
		t.Error("Expected validation error for invalid match syntax, got nil")
	}
	if !contains(err.Error(), "invalid match.syntax") {
		t.Errorf("Expected 'invalid match.syntax' error, got %v", err)
	}
}

func TestLambdaStyleValidation(t *testing.T) {
	tests := []struct {
		name      string
		style     string
		wantError bool
		errorMsg  string
	}{
		{
			name:      "valid typescript style",
			style:     "typescript",
			wantError: false,
		},
		{
			name:      "valid rust style",
			style:     "rust",
			wantError: false,
		},
		{
			name:      "invalid arrow style (old syntax)",
			style:     "arrow",
			wantError: true,
			errorMsg:  "invalid lambda_style",
		},
		{
			name:      "invalid both style (old syntax)",
			style:     "both",
			wantError: true,
			errorMsg:  "invalid lambda_style",
		},
		{
			name:      "invalid kotlin style",
			style:     "kotlin",
			wantError: true,
			errorMsg:  "invalid lambda_style",
		},
		{
			name:      "empty string uses default",
			style:     "",
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Features: FeatureConfig{
					ErrorPropagationSyntax: SyntaxQuestion,
					LambdaStyle:            tt.style,
				},
				SourceMap: SourceMapConfig{
					Enabled: true,
					Format:  FormatInline,
				},
			}

			err := cfg.Validate()
			if tt.wantError {
				if err == nil {
					t.Errorf("Expected error containing %q, got nil", tt.errorMsg)
				} else if tt.errorMsg != "" && !contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing %q, got %q", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got %v", err)
				}
			}
		})
	}
}

func TestLoadLambdaStyleConfig(t *testing.T) {
	tests := []struct {
		name      string
		toml      string
		wantStyle string
		wantError bool
	}{
		{
			name: "typescript style",
			toml: `[features]
lambda_style = "typescript"
`,
			wantStyle: "typescript",
			wantError: false,
		},
		{
			name: "rust style",
			toml: `[features]
lambda_style = "rust"
`,
			wantStyle: "rust",
			wantError: false,
		},
		{
			name: "invalid style",
			toml: `[features]
lambda_style = "kotlin"
`,
			wantError: true,
		},
		{
			name:      "no config uses default",
			toml:      "",
			wantStyle: "typescript",
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp directory
			tmpDir, err := os.MkdirTemp("", "dingo-test-lambda-*")
			if err != nil {
				t.Fatal(err)
			}
			defer os.RemoveAll(tmpDir)

			// Write config if provided
			if tt.toml != "" {
				configPath := filepath.Join(tmpDir, "dingo.toml")
				if err := os.WriteFile(configPath, []byte(tt.toml), 0644); err != nil {
					t.Fatal(err)
				}
			}

			// Change to temp directory
			oldWd, err := os.Getwd()
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = os.Chdir(oldWd) }()

			if err := os.Chdir(tmpDir); err != nil {
				t.Fatal(err)
			}

			// Override HOME
			oldHome := os.Getenv("HOME")
			os.Setenv("HOME", tmpDir)
			defer os.Setenv("HOME", oldHome)

			// Load config
			cfg, err := Load(nil)
			if tt.wantError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}

			if cfg.Features.LambdaStyle != tt.wantStyle {
				t.Errorf("Expected lambda_style %q, got %q", tt.wantStyle, cfg.Features.LambdaStyle)
			}
		})
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
		containsHelper(s, substr)))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
