package codegen

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/token"
	"go/types"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"

	"golang.org/x/tools/go/gcexportdata"
)

// goListCache caches the export map from `go list -export -deps -json` calls.
// This avoids re-running the same `go list` command multiple times within a
// single process (e.g., Phase 0 validation + Step 2.1 error propagation both
// create TypeResolvers with the same imports).
var goListCache = &exportMapCache{
	entries: make(map[string]map[string]string),
}

type exportMapCache struct {
	mu      sync.RWMutex
	entries map[string]map[string]string // cacheKey -> (importPath -> exportFile)
}

func (c *exportMapCache) get(key string) (map[string]string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	exports, ok := c.entries[key]
	return exports, ok
}

func (c *exportMapCache) set(key string, exports map[string]string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = exports
}

// ClearGoListCache clears the go list export cache.
// Call this at the start of a new build to avoid stale data.
func ClearGoListCache() {
	goListCache.mu.Lock()
	defer goListCache.mu.Unlock()
	goListCache.entries = make(map[string]map[string]string)
}

// gcExportImporter reads type information from Go's build cache export data.
// This is ~1000x faster than the source importer for large dependency graphs.
//
// How it works:
//  1. Run `go list -export -deps -json <patterns>` to find export files
//  2. Build importPath -> exportFilePath map
//  3. On Import(path): read export data via gcexportdata.Read()
//  4. Cache loaded packages in memory
type gcExportImporter struct {
	exports map[string]string         // importPath -> export file path
	loaded  map[string]*types.Package // in-memory cache (also used by gcexportdata)
	fset    *token.FileSet
	mu      sync.Mutex // for concurrent safety
}

// goListPackage is the subset of `go list -json` output we need.
type goListPackage struct {
	ImportPath string `json:"ImportPath"`
	Export     string `json:"Export"`
	Name       string `json:"Name"`
}

// NewGCExportImporter creates a fast importer backed by gcexportdata.
//
// It runs `go list -export -deps -json <patterns>` in workingDir to discover
// all export files, then serves Import() calls by reading those files directly.
// This avoids the O(n * seconds) cost of the source importer.
//
// If `go list` fails (e.g., no module, no build cache), returns an error.
// The caller should fall back to the source importer.
func NewGCExportImporter(fset *token.FileSet, workingDir string, patterns ...string) (*gcExportImporter, error) {
	if len(patterns) == 0 {
		return nil, fmt.Errorf("gcExportImporter: no import patterns provided")
	}

	// Build a cache key from workingDir + sorted patterns
	sortedPatterns := make([]string, len(patterns))
	copy(sortedPatterns, patterns)
	sort.Strings(sortedPatterns)
	cacheKey := workingDir + ":" + strings.Join(sortedPatterns, ",")

	// Check the go list cache first to avoid re-running go list
	// within the same process (Phase 0 + Step 2.1 use same imports)
	if cached, ok := goListCache.get(cacheKey); ok {
		return &gcExportImporter{
			exports: cached,
			loaded:  make(map[string]*types.Package),
			fset:    fset,
		}, nil
	}

	// Build the go list command
	args := []string{"list", "-export", "-deps", "-json"}
	args = append(args, patterns...)

	cmd := exec.Command("go", args...)
	cmd.Dir = workingDir

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("gcExportImporter: go list failed: %w (stderr: %s)", err, stderr.String())
	}

	// Parse the streaming JSON output (one object per package, NOT a JSON array)
	exports := make(map[string]string)
	decoder := json.NewDecoder(bytes.NewReader(output))
	for decoder.More() {
		var pkg goListPackage
		if err := decoder.Decode(&pkg); err != nil {
			// Skip malformed entries but continue
			continue
		}
		if pkg.ImportPath != "" && pkg.Export != "" {
			exports[pkg.ImportPath] = pkg.Export
		}
	}

	if len(exports) == 0 {
		return nil, fmt.Errorf("gcExportImporter: go list returned no packages with export data")
	}

	// Cache the export map for subsequent calls with the same imports
	goListCache.set(cacheKey, exports)

	return &gcExportImporter{
		exports: exports,
		loaded:  make(map[string]*types.Package),
		fset:    fset,
	}, nil
}

// Import implements types.Importer. It reads export data from the build cache
// and returns a *types.Package with full type information.
func (imp *gcExportImporter) Import(path string) (*types.Package, error) {
	imp.mu.Lock()
	defer imp.mu.Unlock()

	// Check in-memory cache first
	if pkg, ok := imp.loaded[path]; ok && pkg.Complete() {
		return pkg, nil
	}

	// Find the export file
	exportFile, ok := imp.exports[path]
	if !ok {
		return nil, fmt.Errorf("gcExportImporter: no export data for %q", path)
	}

	// Open and read the export data
	f, err := os.Open(exportFile)
	if err != nil {
		return nil, fmt.Errorf("gcExportImporter: cannot open export file for %q: %w", path, err)
	}
	defer f.Close()

	// gcexportdata.NewReader skips the archive header to find the export data section
	r, err := gcexportdata.NewReader(f)
	if err != nil {
		return nil, fmt.Errorf("gcExportImporter: cannot create reader for %q: %w", path, err)
	}

	// Read the export data. The `imp.loaded` map is passed so that gcexportdata
	// can resolve cross-references between packages (it populates this map as a
	// side effect, which is exactly what we want for caching).
	pkg, err := gcexportdata.Read(r, imp.fset, imp.loaded, path)
	if err != nil {
		return nil, fmt.Errorf("gcExportImporter: cannot read export data for %q: %w", path, err)
	}

	return pkg, nil
}

// ExportCount returns the number of packages with known export files.
// Useful for diagnostics and testing.
func (imp *gcExportImporter) ExportCount() int {
	return len(imp.exports)
}
