package lsp

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/MadAppGang/dingo/pkg/preprocessor"
)

// MaxSupportedSourceMapVersion is the highest source map version this LSP can handle
const MaxSupportedSourceMapVersion = 1

// SourceMapGetter is an interface for retrieving source maps
type SourceMapGetter interface {
	Get(goFilePath string) (*preprocessor.SourceMap, error)
	Invalidate(goFilePath string)
	InvalidateAll()
	Size() int
}

// SourceMapCache provides in-memory caching of source maps with version validation
type SourceMapCache struct {
	mu      sync.RWMutex
	maps    map[string]*preprocessor.SourceMap // mapPath -> SourceMap
	logger  Logger
	maxSize int
}

// NewSourceMapCache creates a new source map cache
func NewSourceMapCache(logger Logger) (*SourceMapCache, error) {
	return &SourceMapCache{
		maps:    make(map[string]*preprocessor.SourceMap),
		logger:  logger,
		maxSize: 100, // LRU limit (future: implement eviction)
	}, nil
}

// Get retrieves a source map from cache or loads it from disk
func (c *SourceMapCache) Get(goFilePath string) (*preprocessor.SourceMap, error) {
	mapPath := goFilePath + ".map"

	// CRITICAL FIX C5: Safe double-check locking pattern
	// Try read lock first (optimistic)
	c.mu.RLock()
	if sm, ok := c.maps[mapPath]; ok {
		c.mu.RUnlock()
		c.logger.Debugf("Source map cache hit: %s", mapPath)
		return sm, nil
	}
	c.mu.RUnlock()

	// Cache miss, load from disk (write lock)
	c.mu.Lock()
	defer c.mu.Unlock()

	// CRITICAL: Re-check under write lock (safe - blocks all readers)
	// This prevents race where another goroutine loaded it between RUnlock and Lock
	if sm, ok := c.maps[mapPath]; ok {
		return sm, nil
	}

	// Load source map
	data, err := os.ReadFile(mapPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("source map not found: %s (transpile .dingo file first: dingo build)", mapPath)
		}
		return nil, fmt.Errorf("failed to read source map %s: %w", mapPath, err)
	}

	// Parse JSON
	sm, err := c.parseSourceMap(data)
	if err != nil {
		return nil, fmt.Errorf("invalid source map %s: %w", mapPath, err)
	}

	// Validate version
	if err := c.validateVersion(sm, mapPath); err != nil {
		return nil, err
	}

	// Store with consistent key
	c.maps[mapPath] = sm
	c.logger.Infof("Source map loaded: %s (version %d, %d mappings)", mapPath, sm.Version, len(sm.Mappings))

	return sm, nil
}

func (c *SourceMapCache) parseSourceMap(data []byte) (*preprocessor.SourceMap, error) {
	var sm preprocessor.SourceMap
	if err := json.Unmarshal(data, &sm); err != nil {
		return nil, fmt.Errorf("JSON parse error: %w", err)
	}
	return &sm, nil
}

func (c *SourceMapCache) validateVersion(sm *preprocessor.SourceMap, mapPath string) error {
	// Default to version 1 if not specified (legacy files from Phase 3)
	if sm.Version == 0 {
		sm.Version = 1
		c.logger.Debugf("Source map %s missing version field, assuming version 1", mapPath)
	}

	if sm.Version > MaxSupportedSourceMapVersion {
		return fmt.Errorf(
			"unsupported source map version %d (max: %d). "+
				"Update dingo-lsp to latest version or downgrade dingo transpiler. "+
				"File: %s",
			sm.Version,
			MaxSupportedSourceMapVersion,
			mapPath,
		)
	}

	return nil
}

// Invalidate removes a source map from cache (called after file changes)
func (c *SourceMapCache) Invalidate(goFilePath string) {
	mapPath := goFilePath + ".map"

	c.mu.Lock()
	defer c.mu.Unlock()

	// CRITICAL FIX C3: Use mapPath as key (consistent with Get())
	if _, ok := c.maps[mapPath]; ok {
		delete(c.maps, mapPath)
		c.logger.Debugf("Source map invalidated: %s", mapPath)
	}
}

// InvalidateAll clears the entire cache
func (c *SourceMapCache) InvalidateAll() {
	c.mu.Lock()
	defer c.mu.Unlock()

	count := len(c.maps)
	c.maps = make(map[string]*preprocessor.SourceMap)
	c.logger.Infof("All source maps invalidated (%d entries cleared)", count)
}

// Size returns the number of cached source maps
func (c *SourceMapCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.maps)
}
