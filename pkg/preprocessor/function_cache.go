package preprocessor

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cespare/xxhash/v2"
)

// FunctionExclusionCache tracks local functions to exclude from import inference.
// It implements a 3-tier caching strategy:
// - Tier 1: In-memory map (fastest, ~1ms lookup)
// - Tier 2: On-disk JSON cache (persistent, ~11ms load)
// - Tier 3: Full rescan via go/parser (~50ms for 10 files)
type FunctionExclusionCache struct {
	// Core data (Tier 1: In-memory)
	localFunctions map[string]bool // "ReadFile" → true
	symbolsByFile  map[string][]string
	packagePath    string

	// Cache invalidation tracking
	fileHashes map[string]uint64 // file → xxhash

	// Performance optimization (early bailout)
	hasUnqualifiedImports bool

	// Telemetry
	lastScanTime time.Time
	scanDuration time.Duration
	cacheHits    uint64
	cacheMisses  uint64
	coldStarts   uint64

	// Cache file path
	cacheFile string

	// Thread-safe access
	mu sync.RWMutex
}

// CacheData represents the on-disk cache format (.dingo-cache.json)
type CacheData struct {
	Version               string            `json:"version"`
	DingoVersion          string            `json:"dingoVersion"`
	PackagePath           string            `json:"packagePath"`
	LastScanTime          string            `json:"lastScanTime"`
	ScanDuration          string            `json:"scanDuration"`
	LocalFunctions        []string          `json:"localFunctions"`
	FileHashes            map[string]uint64 `json:"fileHashes"`
	Files                 []string          `json:"files"`
	HasUnqualifiedImports bool              `json:"hasUnqualifiedImports"`
}

// NewFunctionExclusionCache creates a new cache for the given package path
func NewFunctionExclusionCache(packagePath string) *FunctionExclusionCache {
	cacheFile := filepath.Join(packagePath, ".dingo-cache.json")
	return &FunctionExclusionCache{
		localFunctions:        make(map[string]bool),
		symbolsByFile:         make(map[string][]string),
		fileHashes:            make(map[string]uint64),
		packagePath:           packagePath,
		cacheFile:             cacheFile,
		hasUnqualifiedImports: false,
	}
}

// IsLocalSymbol checks if a symbol is a locally-defined function (Tier 1: fast path)
// Returns true if the symbol should NOT be transformed.
// Time complexity: O(1) map lookup, ~1ms
func (c *FunctionExclusionCache) IsLocalSymbol(name string) bool {
	c.mu.RLock()
	found := c.localFunctions[name]
	c.mu.RUnlock()

	// Use atomic operations for counters to avoid race with concurrent reads
	if found {
		atomic.AddUint64(&c.cacheHits, 1)
		return true
	}

	atomic.AddUint64(&c.cacheMisses, 1)
	return false
}

// ScanPackage scans all files in the package and builds the exclusion list (Tier 3: full rescan)
// Uses go/parser for 100% accuracy. Time: ~50ms for 10 files.
func (c *FunctionExclusionCache) ScanPackage(files []string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	startTime := time.Now()

	// Reset state
	c.localFunctions = make(map[string]bool)
	c.symbolsByFile = make(map[string][]string)
	c.fileHashes = make(map[string]uint64)
	c.hasUnqualifiedImports = false

	// Scan each file
	for _, file := range files {
		symbols, hash, err := c.scanFile(file)
		if err != nil {
			return fmt.Errorf("scanning %s: %w", file, err)
		}

		// Store file hash
		c.fileHashes[file] = hash

		// Store symbols for this file
		c.symbolsByFile[file] = symbols

		// Add to global exclusion list
		for _, sym := range symbols {
			c.localFunctions[sym] = true
		}

		// Check for unqualified patterns (early bailout optimization)
		if !c.hasUnqualifiedImports {
			content, _ := os.ReadFile(file)
			if containsUnqualifiedPattern(content) {
				c.hasUnqualifiedImports = true
			}
		}
	}

	c.lastScanTime = time.Now()
	c.scanDuration = time.Since(startTime)
	c.coldStarts++

	return nil
}

// scanFile parses a single file and extracts function declarations
// Returns: symbols, file hash, error
func (c *FunctionExclusionCache) scanFile(filePath string) ([]string, uint64, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, 0, err
	}

	// Calculate hash for invalidation
	hash := xxhash.Sum64(content)

	// CRITICAL: Preprocess .dingo syntax → Go syntax before parsing
	// Must use minimal preprocessing (no cache) to avoid circular dependency
	prep := New(content)
	preprocessed, _, err := prep.ProcessBytes()
	if err != nil {
		return nil, 0, fmt.Errorf("preprocessing failed: %w", err)
	}

	// Parse with go/parser
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, preprocessed, parser.SkipObjectResolution)
	if err != nil {
		return nil, 0, fmt.Errorf("parsing %s: %w", filePath, err)
	}

	// Extract top-level function declarations
	var symbols []string
	for _, decl := range node.Decls {
		if funcDecl, ok := decl.(*ast.FuncDecl); ok {
			// Skip methods (they have receivers)
			if funcDecl.Recv == nil {
				symbols = append(symbols, funcDecl.Name.Name)
			}
		}
	}

	return symbols, hash, nil
}

// NeedsRescan checks if any files have changed since last scan
// Returns true if cache invalidation is needed
func (c *FunctionExclusionCache) NeedsRescan(files []string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Check if file set changed
	if len(files) != len(c.fileHashes) {
		return true
	}

	// Check each file's hash
	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			return true // File disappeared or unreadable
		}

		currentHash := xxhash.Sum64(content)
		cachedHash, exists := c.fileHashes[file]

		if !exists || currentHash != cachedHash {
			// File changed, but check if symbols changed (QuickScanFile optimization)
			if c.quickScanFileSymbolsChanged(file, content) {
				return true
			}
		}
	}

	return false
}

// quickScanFileSymbolsChanged checks if a file's symbols changed (not just content)
// This is the "QuickScanFile" optimization from the Internal proposal.
// Returns true if symbols changed, false if only comments/body changed.
func (c *FunctionExclusionCache) quickScanFileSymbolsChanged(file string, content []byte) bool {
	// Parse just this file
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, file, content, parser.SkipObjectResolution)
	if err != nil {
		return true // Parse error, assume changed
	}

	// Extract new symbols
	var newSymbols []string
	for _, decl := range node.Decls {
		if funcDecl, ok := decl.(*ast.FuncDecl); ok {
			if funcDecl.Recv == nil {
				newSymbols = append(newSymbols, funcDecl.Name.Name)
			}
		}
	}

	// Compare with cached symbols
	oldSymbols := c.symbolsByFile[file]
	return !symbolsEqual(newSymbols, oldSymbols)
}

// symbolsEqual checks if two symbol lists are equal (order-independent)
func symbolsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	aSet := make(map[string]bool)
	for _, s := range a {
		aSet[s] = true
	}

	for _, s := range b {
		if !aSet[s] {
			return false
		}
	}

	return true
}

// SaveToDisk persists the cache to disk (Tier 2)
// Cache file: .dingo-cache.json in package directory
func (c *FunctionExclusionCache) SaveToDisk() error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Build function list
	funcs := make([]string, 0, len(c.localFunctions))
	for fn := range c.localFunctions {
		funcs = append(funcs, fn)
	}

	// Build file list
	files := make([]string, 0, len(c.fileHashes))
	for f := range c.fileHashes {
		files = append(files, f)
	}

	data := CacheData{
		Version:               "1.0",
		DingoVersion:          "0.5.0", // TODO: Get from build info
		PackagePath:           c.packagePath,
		LastScanTime:          c.lastScanTime.Format(time.RFC3339),
		ScanDuration:          c.scanDuration.String(),
		LocalFunctions:        funcs,
		FileHashes:            c.fileHashes,
		Files:                 files,
		HasUnqualifiedImports: c.hasUnqualifiedImports,
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling cache: %w", err)
	}

	if err := os.WriteFile(c.cacheFile, jsonData, 0644); err != nil {
		return fmt.Errorf("writing cache file: %w", err)
	}

	return nil
}

// LoadFromDisk loads the cache from disk (Tier 2)
// Returns error if cache file doesn't exist or is invalid
// Time: ~11ms (JSON parse + file I/O)
func (c *FunctionExclusionCache) LoadFromDisk() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	content, err := os.ReadFile(c.cacheFile)
	if err != nil {
		return fmt.Errorf("reading cache file: %w", err)
	}

	var data CacheData
	if err := json.Unmarshal(content, &data); err != nil {
		return fmt.Errorf("parsing cache file: %w", err)
	}

	// Version check
	if data.Version != "1.0" {
		return fmt.Errorf("unsupported cache version: %s", data.Version)
	}

	// Restore state
	c.localFunctions = make(map[string]bool)
	for _, fn := range data.LocalFunctions {
		c.localFunctions[fn] = true
	}

	c.fileHashes = data.FileHashes
	c.hasUnqualifiedImports = data.HasUnqualifiedImports
	c.packagePath = data.PackagePath

	// Parse scan time
	if t, err := time.Parse(time.RFC3339, data.LastScanTime); err == nil {
		c.lastScanTime = t
	}

	return nil
}

// Metrics returns cache performance metrics (telemetry)
type CacheMetrics struct {
	ColdStarts   uint64
	CacheHits    uint64
	CacheMisses  uint64
	TotalSymbols int
	LastScanTime time.Time
	ScanDuration time.Duration
}

// Metrics returns current cache performance metrics
func (c *FunctionExclusionCache) Metrics() CacheMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return CacheMetrics{
		ColdStarts:   c.coldStarts,
		CacheHits:    c.cacheHits,
		CacheMisses:  c.cacheMisses,
		TotalSymbols: len(c.localFunctions),
		LastScanTime: c.lastScanTime,
		ScanDuration: c.scanDuration,
	}
}

// HasUnqualifiedImports returns true if the package contains unqualified stdlib calls
// Used for early bailout optimization (skip processing if no unqualified imports)
func (c *FunctionExclusionCache) HasUnqualifiedImports() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.hasUnqualifiedImports
}

// containsUnqualifiedPattern checks if content has potential unqualified stdlib calls
// This is a quick heuristic for the early bailout optimization.
// Pattern: Capitalized function call (e.g., ReadFile(...), Printf(...))
// Excludes: qualified calls (os.ReadFile), lowercase functions (readFile)
func containsUnqualifiedPattern(content []byte) bool {
	// Simple check: Look for capitalized identifier followed by '('
	// This is a heuristic, not precise, but good enough for early bailout
	for i := 0; i < len(content)-1; i++ {
		if content[i] >= 'A' && content[i] <= 'Z' {
			// Check if preceded by identifier character or '.' (qualified call or mid-identifier)
			if i > 0 {
				prev := content[i-1]
				if (prev >= 'a' && prev <= 'z') || (prev >= 'A' && prev <= 'Z') ||
					(prev >= '0' && prev <= '9') || prev == '.' || prev == '_' {
					continue
				}
			}

			// Found uppercase letter at start of identifier, check if followed by alphanumeric then '('
			j := i + 1
			for j < len(content) && ((content[j] >= 'a' && content[j] <= 'z') ||
				(content[j] >= 'A' && content[j] <= 'Z') ||
				(content[j] >= '0' && content[j] <= '9')) {
				j++
			}
			if j < len(content) && content[j] == '(' {
				return true
			}
		}
	}
	return false
}
