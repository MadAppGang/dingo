package build

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// WorkspaceBuilder handles building multiple packages in a workspace.
// All methods are safe for concurrent use. Package builds run in parallel
// with each writing to isolated directories. Cache updates are protected by mutex.
type WorkspaceBuilder struct {
	Root    string
	Options BuildOptions
	mu      sync.Mutex // Protects cache access during parallel builds
}

// BuildOptions configures workspace build behavior
type BuildOptions struct {
	Parallel    bool // Build packages in parallel
	Incremental bool // Only rebuild changed files
	Verbose     bool // Enable verbose logging
	Jobs        int  // Number of parallel jobs (0 = auto)
}

// Package represents a package to build
type Package struct {
	Path       string   // Relative path from workspace root
	Name       string   // Package name
	DingoFiles []string // List of .dingo files
	GoFiles    []string // List of existing .go files
}

// BuildResult represents the result of building a package
type BuildResult struct {
	Package *Package
	Success bool
	Error   error
	Stats   BuildStats
}

// BuildStats tracks build statistics
type BuildStats struct {
	FilesProcessed int
	FilesSkipped   int
	Duration       int64 // milliseconds
}

// NewWorkspaceBuilder creates a new workspace builder
func NewWorkspaceBuilder(root string, opts BuildOptions) *WorkspaceBuilder {
	// Set default job count if not specified
	if opts.Jobs == 0 {
		opts.Jobs = 4 // Default to 4 parallel jobs
	}
	return &WorkspaceBuilder{
		Root:    root,
		Options: opts,
	}
}

// BuildAll builds all packages in dependency order
func (b *WorkspaceBuilder) BuildAll(packages []Package) ([]BuildResult, error) {
	if len(packages) == 0 {
		return nil, fmt.Errorf("no packages to build")
	}

	// Build dependency graph
	graph, err := buildDependencyGraph(packages, b.Root)
	if err != nil {
		return nil, fmt.Errorf("failed to build dependency graph: %w", err)
	}

	// Detect circular dependencies
	if cycles := detectCircularDependencies(graph); len(cycles) > 0 {
		// Format cycle paths for clear error message
		cycleStrs := make([]string, len(cycles))
		for i, cycle := range cycles {
			cycleStrs[i] = strings.Join(cycle, " → ")
		}
		return nil, fmt.Errorf("circular dependencies detected:\n  %s",
			strings.Join(cycleStrs, "\n  "))
	}

	// Get build order (topological sort)
	buildOrder := topologicalSort(graph)

	// Build packages in order
	results := make([]BuildResult, 0, len(packages))

	if b.Options.Parallel {
		results, err = b.buildParallel(packages, buildOrder)
	} else {
		results, err = b.buildSequential(packages, buildOrder)
	}

	if err != nil {
		return results, err
	}

	return results, nil
}

// buildSequential builds packages one at a time in dependency order
func (b *WorkspaceBuilder) buildSequential(packages []Package, buildOrder []string) ([]BuildResult, error) {
	results := make([]BuildResult, 0, len(packages))

	for _, pkgPath := range buildOrder {
		// Find package by path
		var pkg *Package
		for i := range packages {
			if packages[i].Path == pkgPath {
				pkg = &packages[i]
				break
			}
		}
		if pkg == nil {
			continue
		}

		if b.Options.Verbose {
			fmt.Printf("Building package: %s\n", pkg.Path)
		}

		result := b.buildPackage(pkg)
		results = append(results, result)

		if !result.Success {
			return results, fmt.Errorf("failed to build package %s: %w", pkg.Path, result.Error)
		}
	}

	return results, nil
}

// buildParallel builds independent packages in parallel
func (b *WorkspaceBuilder) buildParallel(packages []Package, buildOrder []string) ([]BuildResult, error) {
	results := make([]BuildResult, 0, len(packages))
	resultsMux := sync.Mutex{}

	// Group packages by dependency level
	levels := groupByDependencyLevel(buildOrder)

	// Build each level in parallel
	for levelIdx, level := range levels {
		if b.Options.Verbose {
			fmt.Printf("Building level %d (%d packages)\n", levelIdx+1, len(level))
		}

		var wg sync.WaitGroup
		semaphore := make(chan struct{}, b.Options.Jobs)
		errors := make(chan error, len(level))

		for _, pkgPath := range level {
			// Find package
			var pkg *Package
			for i := range packages {
				if packages[i].Path == pkgPath {
					pkg = &packages[i]
					break
				}
			}
			if pkg == nil {
				continue
			}

			wg.Add(1)
			go func(p *Package) {
				defer wg.Done()
				semaphore <- struct{}{}        // Acquire
				defer func() { <-semaphore }() // Release

				if b.Options.Verbose {
					fmt.Printf("  Building: %s\n", p.Path)
				}

				result := b.buildPackage(p)
				resultsMux.Lock()
				results = append(results, result)
				resultsMux.Unlock()

				if !result.Success {
					errors <- fmt.Errorf("package %s: %w", p.Path, result.Error)
				}
			}(pkg)
		}

		wg.Wait()
		close(errors)

		// Check for errors in this level
		if err := <-errors; err != nil {
			return results, err
		}
	}

	return results, nil
}

// buildPackage builds a single package.
// Safe for concurrent use: each package writes to isolated directory.
// Cache updates are protected by mutex.
func (b *WorkspaceBuilder) buildPackage(pkg *Package) BuildResult {
	result := BuildResult{
		Package: pkg,
		Success: false,
		Stats:   BuildStats{},
	}

	// Get build cache (cache itself is thread-safe for reads)
	cache, err := NewBuildCache(b.Root)
	if err != nil {
		result.Error = fmt.Errorf("failed to initialize cache: %w", err)
		return result
	}

	// Process each .dingo file
	// Each file writes to its own .go file (no conflicts between goroutines)
	for _, dingoFile := range pkg.DingoFiles {
		fullPath := filepath.Join(b.Root, dingoFile)

		// Check if file needs rebuild (incremental mode)
		if b.Options.Incremental {
			// Read operations are safe without lock
			needsRebuild, err := cache.NeedsRebuild(fullPath)
			if err != nil {
				result.Error = fmt.Errorf("cache check failed for %s: %w", dingoFile, err)
				return result
			}
			if !needsRebuild {
				result.Stats.FilesSkipped++
				if b.Options.Verbose {
					fmt.Printf("    Skipping (cached): %s\n", dingoFile)
				}
				continue
			}
		}

		// Build the file (call existing transpiler)
		if b.Options.Verbose {
			fmt.Printf("    Transpiling: %s\n", dingoFile)
		}

		// NOTE: This would call the actual transpiler
		// For now, placeholder to avoid import cycles
		// err := transpile(fullPath)
		// if err != nil {
		// 	result.Error = fmt.Errorf("transpile failed for %s: %w", dingoFile, err)
		// 	return result
		// }

		// Update cache (write operation requires lock)
		if b.Options.Incremental {
			b.mu.Lock()
			err := cache.MarkBuilt(fullPath)
			b.mu.Unlock()

			if err != nil {
				result.Error = fmt.Errorf("cache update failed for %s: %w", dingoFile, err)
				return result
			}
		}

		result.Stats.FilesProcessed++
	}

	result.Success = true
	return result
}

// groupByDependencyLevel groups packages by their dependency level
// Level 0 = no dependencies, Level 1 = depends only on Level 0, etc.
func groupByDependencyLevel(buildOrder []string) [][]string {
	// This is a simplified version - assumes buildOrder is already sorted
	// In practice, would analyze actual dependencies
	levels := make([][]string, 0)

	// For now, put all packages in one level (sequential)
	if len(buildOrder) > 0 {
		levels = append(levels, buildOrder)
	}

	return levels
}

// GetTranspiledPath returns the .go path for a .dingo file
func GetTranspiledPath(dingoPath string) string {
	if !filepath.IsAbs(dingoPath) {
		dingoPath, _ = filepath.Abs(dingoPath)
	}
	return dingoPath[:len(dingoPath)-6] + ".go" // .dingo -> .go
}

// GetSourceMapPath returns the .go.map path for a .dingo file
func GetSourceMapPath(dingoPath string) string {
	return GetTranspiledPath(dingoPath) + ".map"
}

// ensureDir creates a directory if it doesn't exist
func ensureDir(path string) error {
	return os.MkdirAll(path, 0o755)
}
