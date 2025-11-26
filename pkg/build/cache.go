package build

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

// BuildCache manages incremental build cache.
//
// Concurrency Safety:
// - Read operations (NeedsRebuild, GetCacheStats) are safe for concurrent use
// - Write operations (MarkBuilt, Invalidate, save) must be externally synchronized
// - When using with WorkspaceBuilder parallel builds, cache updates are protected by mutex
type BuildCache struct {
	Root      string                 // Workspace root
	CacheDir  string                 // .dingo-cache directory
	Entries   map[string]*CacheEntry // File path -> cache entry
	cacheFile string                 // Cache metadata file path
}

// CacheEntry represents cached build information for a file
type CacheEntry struct {
	SourcePath   string    // Original .dingo file path
	OutputPath   string    // Generated .go file path
	SourceHash   string    // SHA-256 hash of source content
	OutputHash   string    // SHA-256 hash of output content
	LastBuilt    time.Time // When file was last built
	Dependencies []string  // List of files this file depends on
}

// NewBuildCache creates or loads a build cache
func NewBuildCache(workspaceRoot string) (*BuildCache, error) {
	cacheDir := filepath.Join(workspaceRoot, ".dingo-cache")
	cacheFile := filepath.Join(cacheDir, "build-cache.json")

	// Create cache directory if it doesn't exist
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	cache := &BuildCache{
		Root:      workspaceRoot,
		CacheDir:  cacheDir,
		Entries:   make(map[string]*CacheEntry),
		cacheFile: cacheFile,
	}

	// Load existing cache if it exists
	if err := cache.load(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to load cache: %w", err)
	}

	return cache, nil
}

// NeedsRebuild checks if a file needs to be rebuilt
func (c *BuildCache) NeedsRebuild(sourcePath string) (bool, error) {
	// Get absolute path
	absPath, err := filepath.Abs(sourcePath)
	if err != nil {
		return true, err
	}

	// Check if we have a cache entry
	entry, exists := c.Entries[absPath]
	if !exists {
		return true, nil // No cache entry = needs build
	}

	// Check if source file exists
	sourceInfo, err := os.Stat(absPath)
	if err != nil {
		return true, err
	}

	// Check if output file exists
	outputPath := getOutputPath(absPath)
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		return true, nil // Output doesn't exist = needs build
	}

	// Check if source was modified after last build
	if sourceInfo.ModTime().After(entry.LastBuilt) {
		return true, nil // Source modified = needs build
	}

	// Check if source content changed (hash comparison)
	currentHash, err := hashFile(absPath)
	if err != nil {
		return true, err
	}
	if currentHash != entry.SourceHash {
		return true, nil // Content changed = needs build
	}

	// Check if any dependencies changed
	for _, depPath := range entry.Dependencies {
		depInfo, err := os.Stat(depPath)
		if err != nil {
			return true, nil // Dependency missing = needs build
		}
		if depInfo.ModTime().After(entry.LastBuilt) {
			return true, nil // Dependency modified = needs build
		}
	}

	return false, nil // Cache valid, no rebuild needed
}

// MarkBuilt updates the cache after a successful build
func (c *BuildCache) MarkBuilt(sourcePath string) error {
	absPath, err := filepath.Abs(sourcePath)
	if err != nil {
		return err
	}

	outputPath := getOutputPath(absPath)

	// Hash source and output files
	sourceHash, err := hashFile(absPath)
	if err != nil {
		return fmt.Errorf("failed to hash source: %w", err)
	}

	outputHash, err := hashFile(outputPath)
	if err != nil {
		return fmt.Errorf("failed to hash output: %w", err)
	}

	// Extract dependencies from import statements
	dependencies, err := extractImports(absPath)
	if err != nil {
		// Log warning but don't fail the build
		dependencies = []string{}
	}

	// Create/update cache entry
	entry := &CacheEntry{
		SourcePath:   absPath,
		OutputPath:   outputPath,
		SourceHash:   sourceHash,
		OutputHash:   outputHash,
		LastBuilt:    time.Now(),
		Dependencies: dependencies,
	}

	c.Entries[absPath] = entry

	// Save cache to disk
	return c.save()
}

// Invalidate removes a cache entry
func (c *BuildCache) Invalidate(sourcePath string) error {
	absPath, err := filepath.Abs(sourcePath)
	if err != nil {
		return err
	}

	delete(c.Entries, absPath)
	return c.save()
}

// InvalidateAll clears the entire cache
func (c *BuildCache) InvalidateAll() error {
	c.Entries = make(map[string]*CacheEntry)
	return c.save()
}

// load reads cache from disk
func (c *BuildCache) load() error {
	data, err := os.ReadFile(c.cacheFile)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, &c.Entries)
}

// save writes cache to disk
func (c *BuildCache) save() error {
	data, err := json.MarshalIndent(c.Entries, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal cache: %w", err)
	}

	return os.WriteFile(c.cacheFile, data, 0644)
}

// hashFile computes SHA-256 hash of a file
func hashFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

// getOutputPath converts .dingo path to .go path
func getOutputPath(dingoPath string) string {
	if len(dingoPath) < 6 || dingoPath[len(dingoPath)-6:] != ".dingo" {
		return dingoPath + ".go" // Fallback
	}
	return dingoPath[:len(dingoPath)-6] + ".go"
}

// GetCacheStats returns statistics about the cache
func (c *BuildCache) GetCacheStats() map[string]interface{} {
	totalEntries := len(c.Entries)
	totalSize := int64(0)

	for _, entry := range c.Entries {
		if info, err := os.Stat(entry.OutputPath); err == nil {
			totalSize += info.Size()
		}
	}

	return map[string]interface{}{
		"entries":    totalEntries,
		"total_size": totalSize,
		"cache_dir":  c.CacheDir,
	}
}

// Clean removes stale cache entries (outputs that no longer exist)
func (c *BuildCache) Clean() error {
	toRemove := make([]string, 0)

	for path, entry := range c.Entries {
		// Check if source exists
		if _, err := os.Stat(entry.SourcePath); os.IsNotExist(err) {
			toRemove = append(toRemove, path)
			continue
		}

		// Check if output exists
		if _, err := os.Stat(entry.OutputPath); os.IsNotExist(err) {
			toRemove = append(toRemove, path)
			continue
		}
	}

	for _, path := range toRemove {
		delete(c.Entries, path)
	}

	if len(toRemove) > 0 {
		return c.save()
	}

	return nil
}

// extractImports extracts import paths from a .dingo file
func extractImports(sourcePath string) ([]string, error) {
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		return nil, err
	}

	// Match import statements: import "package/path"
	importRegex := regexp.MustCompile(`import\s+"([^"]+)"`)
	matches := importRegex.FindAllSubmatch(data, -1)

	deps := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) > 1 {
			deps = append(deps, string(match[1]))
		}
	}

	return deps, nil
}
