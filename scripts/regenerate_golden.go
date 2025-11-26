package main

import (
	"fmt"
	"go/token"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/MadAppGang/dingo/pkg/config"
	"github.com/MadAppGang/dingo/pkg/generator"
	"github.com/MadAppGang/dingo/pkg/parser"
	"github.com/MadAppGang/dingo/pkg/plugin"
	"github.com/MadAppGang/dingo/pkg/preprocessor"
)

// simpleLogger implements plugin.Logger
type simpleLogger struct{}

func (l *simpleLogger) Info(msg string)                  { fmt.Println("INFO:", msg) }
func (l *simpleLogger) Error(msg string)                 { fmt.Println("ERROR:", msg) }
func (l *simpleLogger) Debugf(format string, args ...any) { fmt.Printf("DEBUG: "+format+"\n", args...) }
func (l *simpleLogger) Warnf(format string, args ...any)  { fmt.Printf("WARN: "+format+"\n", args...) }

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run regenerate_golden.go <file.dingo>")
		os.Exit(1)
	}

	dingoFile := os.Args[1]

	// Read Dingo source
	dingoSrc, err := os.ReadFile(dingoFile)
	if err != nil {
		fmt.Printf("Failed to read %s: %v\n", dingoFile, err)
		os.Exit(1)
	}

	fset := token.NewFileSet()

	// Load config if exists
	var cfg *config.Config
	baseName := filepath.Base(dingoFile)
	baseName = baseName[:len(baseName)-len(".dingo")]
	testConfigDir := filepath.Join(filepath.Dir(dingoFile), baseName)
	testConfigPath := filepath.Join(testConfigDir, "dingo.toml")
	if _, err := os.Stat(testConfigPath); err == nil {
		cfg = config.DefaultConfig()
		if _, err := toml.DecodeFile(testConfigPath, cfg); err != nil {
			fmt.Printf("Failed to load config: %v\n", err)
			os.Exit(1)
		}
	}

	// Create cache for unqualified import inference
	pkgDir := filepath.Dir(dingoFile)
	cache := preprocessor.NewFunctionExclusionCache(pkgDir)
	err = cache.ScanPackage([]string{dingoFile})

	// Create preprocessor
	var preprocessorInst *preprocessor.Preprocessor
	if err != nil {
		// Cache scan failed, fall back to no cache
		if cfg != nil {
			preprocessorInst = preprocessor.NewWithMainConfig(dingoSrc, cfg)
		} else {
			preprocessorInst = preprocessor.New(dingoSrc)
		}
	} else {
		// Cache scan successful, use it for unqualified imports
		preprocessorInst = preprocessor.NewWithCache(dingoSrc, cache)
	}

	// Preprocess
	preprocessed, _, err := preprocessorInst.Process()
	if err != nil {
		fmt.Printf("Preprocessing failed: %v\n", err)
		os.Exit(1)
	}

	// Parse
	file, err := parser.ParseFile(fset, dingoFile, []byte(preprocessed), parser.ParseComments)
	if err != nil {
		fmt.Printf("Parse failed: %v\n", err)
		os.Exit(1)
	}

	// Create generator
	registry := plugin.NewRegistry()
	logger := &simpleLogger{}
	gen, err := generator.NewWithPlugins(fset, registry, logger)
	if err != nil {
		fmt.Printf("Failed to create generator: %v\n", err)
		os.Exit(1)
	}

	// Generate Go code
	output, err := gen.Generate(file)
	if err != nil {
		fmt.Printf("Generation failed: %v\n", err)
		os.Exit(1)
	}

	// Write .go.golden file
	goldenFile := filepath.Join(filepath.Dir(dingoFile), baseName+".go.golden")

	if err := os.WriteFile(goldenFile, output, 0644); err != nil {
		fmt.Printf("Failed to write %s: %v\n", goldenFile, err)
		os.Exit(1)
	}

	fmt.Printf("✓ Regenerated %s\n", goldenFile)
}
