// Package config provides configuration management for the Dingo compiler
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// SyntaxStyle represents the error propagation syntax style
type SyntaxStyle string

const (
	// SyntaxQuestion uses the ? operator (expr?)
	SyntaxQuestion SyntaxStyle = "question"

	// SyntaxBang uses the ! operator (expr!)
	SyntaxBang SyntaxStyle = "bang"

	// SyntaxTry uses the try keyword (try expr)
	SyntaxTry SyntaxStyle = "try"
)

// IsValid reports whether the syntax style is valid
func (s SyntaxStyle) IsValid() bool {
	switch s {
	case SyntaxQuestion, SyntaxBang, SyntaxTry:
		return true
	default:
		return false
	}
}

// SourceMapFormat represents the source map output format
type SourceMapFormat string

const (
	// FormatInline embeds source maps as comments in generated Go files
	FormatInline SourceMapFormat = "inline"

	// FormatSeparate writes source maps to .go.map files
	FormatSeparate SourceMapFormat = "separate"

	// FormatBoth writes both inline and separate source maps
	FormatBoth SourceMapFormat = "both"

	// FormatNone disables source map generation
	FormatNone SourceMapFormat = "none"
)

// Config represents the complete Dingo project configuration
type Config struct {
	Features  FeatureConfig   `toml:"features"`
	SourceMap SourceMapConfig `toml:"sourcemaps"`
}

// FeatureConfig controls which language features are enabled
type FeatureConfig struct {
	// ErrorPropagationSyntax selects the error propagation operator
	// Valid values: "question", "bang", "try"
	ErrorPropagationSyntax SyntaxStyle `toml:"error_propagation_syntax"`

	// ReuseErrVariable controls whether to reuse a single "err" variable
	// instead of generating __err0, __err1, etc. in the same scope
	// When true: always uses "err" (cleaner, more idiomatic)
	// When false: generates unique names (safer, avoids shadowing)
	ReuseErrVariable bool `toml:"reuse_err_variable"`

	// NilSafetyChecks controls nil pointer validation in pattern destructuring
	// Valid values: "off", "on", "debug"
	// - "off": No nil checks (trust constructors, maximum performance)
	// - "on": Always check with runtime panic (safe, default)
	// - "debug": Check only when DINGO_DEBUG env var is set
	NilSafetyChecks string `toml:"nil_safety_checks"`

	// LambdaSyntax controls which lambda function syntax styles are accepted
	// Valid values: "rust", "arrow", "both"
	// - "rust": Only Rust-style |x| expr syntax
	// - "arrow": Only JavaScript/TypeScript-style (x) => expr syntax
	// - "both": Accept both styles in the same file (default)
	LambdaSyntax string `toml:"lambda_syntax"`

	// SafeNavigationUnwrap controls how the ?. operator handles return types
	// Valid values: "always_option", "smart"
	// - "always_option": Always returns Option<T>
	// - "smart": Unwraps to T based on context (default)
	SafeNavigationUnwrap string `toml:"safe_navigation_unwrap"`

	// NullCoalescingPointers enables ?? operator for Go pointers (*T)
	// When true: Works with both Option<T> and *T
	// When false: Works only with Option<T> (stricter type safety)
	NullCoalescingPointers bool `toml:"null_coalescing_pointers"`

	// OperatorPrecedence controls ternary/null-coalescing precedence checking
	// Valid values: "standard", "explicit"
	// - "standard": Follow C/TypeScript precedence rules
	// - "explicit": Require parentheses for ambiguous mixing
	OperatorPrecedence string `toml:"operator_precedence"`

	// REMOVED: AutoWrapGoErrors and AutoWrapGoNils
	// These flags were never implemented and are misleading.
	// Follow YAGNI principle - add when actually needed in Phase 3+
}

// SourceMapConfig controls source map generation
type SourceMapConfig struct {
	// Enabled controls whether source maps are generated
	Enabled bool `toml:"enabled"`

	// Format controls the source map output format
	// Valid values: "inline", "separate", "both", "none"
	Format SourceMapFormat `toml:"format"`
}

// NilSafetyMode represents nil safety check modes
type NilSafetyMode int

const (
	NilSafetyOff NilSafetyMode = iota
	NilSafetyOn
	NilSafetyDebug
)

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		Features: FeatureConfig{
			ErrorPropagationSyntax: SyntaxQuestion, // Default to ? operator
			ReuseErrVariable:       true,           // Default to reusing "err" for cleaner code
			NilSafetyChecks:        "on",           // Default to safe mode
			LambdaSyntax:           "rust",         // Default to Rust-style |x| expr
			SafeNavigationUnwrap:   "smart",        // Default to smart unwrapping
			NullCoalescingPointers: true,           // Default to supporting Go pointers
			OperatorPrecedence:     "standard",     // Default to standard precedence
		},
		SourceMap: SourceMapConfig{
			Enabled: true,
			Format:  FormatInline, // Default to inline for development
		},
	}
}

// Load loads configuration from multiple sources with precedence:
// 1. CLI flags (highest priority) - passed as overrides
// 2. Project dingo.toml (current directory)
// 3. User config (~/.dingo/config.toml)
// 4. Built-in defaults (lowest priority)
func Load(overrides *Config) (*Config, error) {
	// Start with defaults
	cfg := DefaultConfig()

	// Load user config if it exists
	userConfigPath := filepath.Join(os.Getenv("HOME"), ".dingo", "config.toml")
	if err := loadConfigFile(userConfigPath, cfg); err != nil {
		return nil, fmt.Errorf("failed to load user config: %w", err)
	}

	// Load project config if it exists
	projectConfigPath := "dingo.toml"
	if err := loadConfigFile(projectConfigPath, cfg); err != nil {
		return nil, fmt.Errorf("failed to load project config: %w", err)
	}

	// Apply overrides from CLI flags
	if overrides != nil {
		if overrides.Features.ErrorPropagationSyntax != "" {
			cfg.Features.ErrorPropagationSyntax = overrides.Features.ErrorPropagationSyntax
		}
		if overrides.SourceMap.Format != "" {
			cfg.SourceMap.Format = overrides.SourceMap.Format
		}
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return cfg, nil
}

// loadConfigFile loads a TOML configuration file into the provided config
// If the file doesn't exist, this is not an error (we use defaults)
func loadConfigFile(path string, cfg *Config) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil // File doesn't exist, use defaults
	}

	if _, err := toml.DecodeFile(path, cfg); err != nil {
		return fmt.Errorf("failed to parse %s: %w", path, err)
	}

	return nil
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	// Validate error propagation syntax
	if !c.Features.ErrorPropagationSyntax.IsValid() {
		return fmt.Errorf("invalid error_propagation_syntax: %q (must be 'question', 'bang', or 'try')",
			c.Features.ErrorPropagationSyntax)
	}

	// Validate nil safety mode
	if c.Features.NilSafetyChecks != "" {
		switch c.Features.NilSafetyChecks {
		case "off", "on", "debug":
			// Valid
		default:
			return fmt.Errorf("invalid nil_safety_checks: %q (must be 'off', 'on', or 'debug')",
				c.Features.NilSafetyChecks)
		}
	}

	// Validate lambda syntax
	if c.Features.LambdaSyntax != "" {
		switch c.Features.LambdaSyntax {
		case "rust", "arrow", "both":
			// Valid
		default:
			return fmt.Errorf("invalid lambda_syntax: %q (must be 'rust', 'arrow', or 'both')",
				c.Features.LambdaSyntax)
		}
	}

	// Validate safe navigation unwrap mode
	if c.Features.SafeNavigationUnwrap != "" {
		switch c.Features.SafeNavigationUnwrap {
		case "always_option", "smart":
			// Valid
		default:
			return fmt.Errorf("invalid safe_navigation_unwrap: %q (must be 'always_option' or 'smart')",
				c.Features.SafeNavigationUnwrap)
		}
	}

	// Validate operator precedence
	if c.Features.OperatorPrecedence != "" {
		switch c.Features.OperatorPrecedence {
		case "standard", "explicit":
			// Valid
		default:
			return fmt.Errorf("invalid operator_precedence: %q (must be 'standard' or 'explicit')",
				c.Features.OperatorPrecedence)
		}
	}

	// Validate source map format
	switch c.SourceMap.Format {
	case FormatInline, FormatSeparate, FormatBoth, FormatNone:
		// Valid
	default:
		return fmt.Errorf("invalid sourcemap format: %q (must be 'inline', 'separate', 'both', or 'none')",
			c.SourceMap.Format)
	}

	return nil
}

// GetNilSafetyMode parses the nil safety string into enum
func (c *Config) GetNilSafetyMode() NilSafetyMode {
	switch c.Features.NilSafetyChecks {
	case "off":
		return NilSafetyOff
	case "on":
		return NilSafetyOn
	case "debug":
		return NilSafetyDebug
	default:
		return NilSafetyOn // Default to safe mode
	}
}
