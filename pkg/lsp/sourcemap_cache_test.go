package lsp

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/MadAppGang/dingo/pkg/preprocessor"
)

func TestSourceMapCache_HitAndMiss(t *testing.T) {
	logger := NewLogger("debug", &bytes.Buffer{})
	cache, err := NewSourceMapCache(logger)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	// Create temp source map file
	tmpDir := t.TempDir()
	goFile := filepath.Join(tmpDir, "test.go")
	mapFile := goFile + ".map"

	sm := &preprocessor.SourceMap{
		Version: 1,
		Mappings: []preprocessor.Mapping{
			{
				OriginalLine:    1,
				OriginalColumn:  1,
				GeneratedLine:   1,
				GeneratedColumn: 1,
				Length:          5,
			},
		},
	}

	writeSourceMap(t, mapFile, sm)

	// First call: cache miss (load from disk)
	sm1, err := cache.Get(goFile)
	if err != nil {
		t.Fatalf("First Get failed: %v", err)
	}
	if sm1 == nil {
		t.Fatal("Expected source map, got nil")
	}

	// Second call: cache hit (in-memory)
	sm2, err := cache.Get(goFile)
	if err != nil {
		t.Fatalf("Second Get failed: %v", err)
	}

	// Should be same pointer (cached)
	if sm1 != sm2 {
		t.Error("Expected same source map instance (cache hit)")
	}
}

func TestSourceMapCache_VersionValidation(t *testing.T) {
	logger := NewLogger("debug", &bytes.Buffer{})
	cache, _ := NewSourceMapCache(logger)

	tmpDir := t.TempDir()
	goFile := filepath.Join(tmpDir, "test.go")
	mapFile := goFile + ".map"

	tests := []struct {
		name        string
		version     int
		expectError bool
	}{
		{"version 1 (supported)", 1, false},
		{"version 0 (legacy, defaults to 1)", 0, false},
		{"version 99 (unsupported)", 99, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sm := &preprocessor.SourceMap{
				Version: tt.version,
				Mappings: []preprocessor.Mapping{
					{
						OriginalLine:    1,
						OriginalColumn:  1,
						GeneratedLine:   1,
						GeneratedColumn: 1,
					},
				},
			}

			writeSourceMap(t, mapFile, sm)

			// Invalidate cache from previous test
			cache.Invalidate(goFile)

			_, err := cache.Get(goFile)

			if tt.expectError && err == nil {
				t.Error("Expected error for unsupported version, got nil")
			}

			if !tt.expectError && err != nil {
				t.Errorf("Expected no error, got: %v", err)
			}
		})
	}
}

func TestSourceMapCache_Invalidation(t *testing.T) {
	logger := NewLogger("debug", &bytes.Buffer{})
	cache, _ := NewSourceMapCache(logger)

	tmpDir := t.TempDir()
	goFile := filepath.Join(tmpDir, "test.go")
	mapFile := goFile + ".map"

	sm := &preprocessor.SourceMap{
		Version: 1,
		Mappings: []preprocessor.Mapping{
			{
				OriginalLine:    1,
				OriginalColumn:  1,
				GeneratedLine:   1,
				GeneratedColumn: 1,
			},
		},
	}

	writeSourceMap(t, mapFile, sm)

	// Load into cache
	sm1, err := cache.Get(goFile)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if cache.Size() != 1 {
		t.Errorf("Expected cache size 1, got %d", cache.Size())
	}

	// Invalidate
	cache.Invalidate(goFile)

	if cache.Size() != 0 {
		t.Errorf("Expected cache size 0 after invalidation, got %d", cache.Size())
	}

	// Reload (should read from disk again, different pointer)
	sm2, err := cache.Get(goFile)
	if err != nil {
		t.Fatalf("Get after invalidation failed: %v", err)
	}

	if sm1 == sm2 {
		t.Error("Expected different source map instance after invalidation")
	}
}

func TestSourceMapCache_InvalidateAll(t *testing.T) {
	logger := NewLogger("debug", &bytes.Buffer{})
	cache, _ := NewSourceMapCache(logger)

	tmpDir := t.TempDir()

	// Create multiple source maps
	for i := 1; i <= 3; i++ {
		goFile := filepath.Join(tmpDir, "test"+string(rune('0'+i))+".go")
		mapFile := goFile + ".map"

		sm := &preprocessor.SourceMap{
			Version: 1,
			Mappings: []preprocessor.Mapping{
				{
					OriginalLine:    i,
					OriginalColumn:  1,
					GeneratedLine:   i,
					GeneratedColumn: 1,
				},
			},
		}

		writeSourceMap(t, mapFile, sm)

		// Load into cache
		_, err := cache.Get(goFile)
		if err != nil {
			t.Fatalf("Get failed for file %d: %v", i, err)
		}
	}

	if cache.Size() != 3 {
		t.Errorf("Expected cache size 3, got %d", cache.Size())
	}

	// Invalidate all
	cache.InvalidateAll()

	if cache.Size() != 0 {
		t.Errorf("Expected cache size 0 after InvalidateAll, got %d", cache.Size())
	}
}

func TestSourceMapCache_MissingFile(t *testing.T) {
	logger := NewLogger("debug", &bytes.Buffer{})
	cache, _ := NewSourceMapCache(logger)

	_, err := cache.Get("/nonexistent/file.go")

	if err == nil {
		t.Error("Expected error for missing file, got nil")
	}
}

func TestSourceMapCache_InvalidJSON(t *testing.T) {
	logger := NewLogger("debug", &bytes.Buffer{})
	cache, _ := NewSourceMapCache(logger)

	tmpDir := t.TempDir()
	goFile := filepath.Join(tmpDir, "test.go")
	mapFile := goFile + ".map"

	// Write invalid JSON
	err := os.WriteFile(mapFile, []byte("invalid json {{{"), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	_, err = cache.Get(goFile)

	if err == nil {
		t.Error("Expected error for invalid JSON, got nil")
	}
}

// Helper function to write source map file
func writeSourceMap(t *testing.T, path string, sm *preprocessor.SourceMap) {
	t.Helper()

	data, err := json.Marshal(sm)
	if err != nil {
		t.Fatalf("Failed to marshal source map: %v", err)
	}

	err = os.WriteFile(path, data, 0644)
	if err != nil {
		t.Fatalf("Failed to write source map: %v", err)
	}
}
