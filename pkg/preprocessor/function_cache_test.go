package preprocessor

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestIsLocalSymbol(t *testing.T) {
	cache := NewFunctionExclusionCache("/tmp/test-pkg")

	// Populate with test data
	cache.mu.Lock()
	cache.localFunctions["ReadFile"] = true
	cache.localFunctions["ProcessData"] = true
	cache.mu.Unlock()

	tests := []struct {
		name     string
		symbol   string
		expected bool
	}{
		{"local function exists", "ReadFile", true},
		{"local function exists 2", "ProcessData", true},
		{"stdlib function", "Printf", false},
		{"unknown function", "DoesNotExist", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cache.IsLocalSymbol(tt.symbol)
			if result != tt.expected {
				t.Errorf("IsLocalSymbol(%q) = %v, want %v", tt.symbol, result, tt.expected)
			}
		})
	}

	// Verify cache hit metrics
	metrics := cache.Metrics()
	if metrics.CacheHits == 0 {
		t.Error("Expected cache hits to be recorded")
	}
	if metrics.CacheMisses == 0 {
		t.Error("Expected cache misses to be recorded")
	}
}

func TestScanPackage(t *testing.T) {
	// Create temp directory with test files
	tmpDir := t.TempDir()

	// Write test file 1
	file1 := filepath.Join(tmpDir, "file1.dingo")
	content1 := `package main

func ReadFile(path string) string {
	return "data"
}

func ProcessData(data string) {
	// process
}
`
	if err := os.WriteFile(file1, []byte(content1), 0644); err != nil {
		t.Fatal(err)
	}

	// Write test file 2
	file2 := filepath.Join(tmpDir, "file2.dingo")
	content2 := `package main

func ValidateInput(input string) bool {
	return true
}

// Method (should be ignored)
type User struct{}

func (u *User) GetName() string {
	return "name"
}
`
	if err := os.WriteFile(file2, []byte(content2), 0644); err != nil {
		t.Fatal(err)
	}

	// Create cache and scan
	cache := NewFunctionExclusionCache(tmpDir)
	err := cache.ScanPackage([]string{file1, file2})
	if err != nil {
		t.Fatalf("ScanPackage failed: %v", err)
	}

	// Verify functions were detected
	expectedFuncs := []string{"ReadFile", "ProcessData", "ValidateInput"}
	for _, fn := range expectedFuncs {
		if !cache.IsLocalSymbol(fn) {
			t.Errorf("Expected function %q to be in exclusion list", fn)
		}
	}

	// Verify method was NOT included
	if cache.IsLocalSymbol("GetName") {
		t.Error("Method GetName should not be in exclusion list")
	}

	// Verify file hashes were stored
	if len(cache.fileHashes) != 2 {
		t.Errorf("Expected 2 file hashes, got %d", len(cache.fileHashes))
	}

	// Verify metrics
	metrics := cache.Metrics()
	if metrics.ColdStarts != 1 {
		t.Errorf("Expected 1 cold start, got %d", metrics.ColdStarts)
	}
	if metrics.TotalSymbols != 3 {
		t.Errorf("Expected 3 symbols, got %d", metrics.TotalSymbols)
	}
	if metrics.ScanDuration == 0 {
		t.Error("Expected scan duration to be recorded")
	}
}

func TestNeedsRescan(t *testing.T) {
	tmpDir := t.TempDir()

	// Create initial file
	file1 := filepath.Join(tmpDir, "file1.dingo")
	content1 := `package main

func ReadFile(path string) string {
	return "data"
}
`
	if err := os.WriteFile(file1, []byte(content1), 0644); err != nil {
		t.Fatal(err)
	}

	// Scan initial state
	cache := NewFunctionExclusionCache(tmpDir)
	if err := cache.ScanPackage([]string{file1}); err != nil {
		t.Fatal(err)
	}

	// Test 1: No changes - should not need rescan
	t.Run("no changes", func(t *testing.T) {
		if cache.NeedsRescan([]string{file1}) {
			t.Error("Expected no rescan needed when file unchanged")
		}
	})

	// Test 2: Content changed but symbols same - should not need rescan
	t.Run("content changed, symbols same", func(t *testing.T) {
		newContent := `package main

func ReadFile(path string) string {
	// Updated implementation
	return "new data"
}
`
		if err := os.WriteFile(file1, []byte(newContent), 0644); err != nil {
			t.Fatal(err)
		}

		// This should detect no symbol changes (QuickScanFile optimization)
		if cache.NeedsRescan([]string{file1}) {
			t.Error("Expected no rescan when only function body changed")
		}
	})

	// Test 3: New function added - should need rescan
	t.Run("new function added", func(t *testing.T) {
		newContent := `package main

func ReadFile(path string) string {
	return "data"
}

func NewFunction() {
	// new function
}
`
		if err := os.WriteFile(file1, []byte(newContent), 0644); err != nil {
			t.Fatal(err)
		}

		if !cache.NeedsRescan([]string{file1}) {
			t.Error("Expected rescan when new function added")
		}
	})

	// Test 4: File count changed - should need rescan
	t.Run("file count changed", func(t *testing.T) {
		file2 := filepath.Join(tmpDir, "file2.dingo")
		if err := os.WriteFile(file2, []byte("package main\n"), 0644); err != nil {
			t.Fatal(err)
		}

		if !cache.NeedsRescan([]string{file1, file2}) {
			t.Error("Expected rescan when file count changed")
		}
	})
}

func TestSaveLoadDisk(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test file
	file1 := filepath.Join(tmpDir, "file1.dingo")
	content := `package main

func ReadFile(path string) string {
	return "data"
}

func ProcessData(data string) {
	// process
}
`
	if err := os.WriteFile(file1, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Create and populate cache
	cache1 := NewFunctionExclusionCache(tmpDir)
	if err := cache1.ScanPackage([]string{file1}); err != nil {
		t.Fatal(err)
	}

	// Save to disk
	if err := cache1.SaveToDisk(); err != nil {
		t.Fatalf("SaveToDisk failed: %v", err)
	}

	// Verify cache file exists
	cacheFilePath := filepath.Join(tmpDir, ".dingo-cache.json")
	if _, err := os.Stat(cacheFilePath); os.IsNotExist(err) {
		t.Fatal("Cache file was not created")
	}

	// Load into new cache instance
	cache2 := NewFunctionExclusionCache(tmpDir)
	if err := cache2.LoadFromDisk(); err != nil {
		t.Fatalf("LoadFromDisk failed: %v", err)
	}

	// Verify functions were restored
	if !cache2.IsLocalSymbol("ReadFile") {
		t.Error("ReadFile not found after loading from disk")
	}
	if !cache2.IsLocalSymbol("ProcessData") {
		t.Error("ProcessData not found after loading from disk")
	}

	// Verify file hashes were restored
	if len(cache2.fileHashes) != 1 {
		t.Errorf("Expected 1 file hash, got %d", len(cache2.fileHashes))
	}

	// Verify package path
	if cache2.packagePath != tmpDir {
		t.Errorf("Package path mismatch: got %q, want %q", cache2.packagePath, tmpDir)
	}
}

func TestQuickScanFileOptimization(t *testing.T) {
	tmpDir := t.TempDir()

	file1 := filepath.Join(tmpDir, "file1.dingo")
	originalContent := `package main

func ReadFile(path string) string {
	return "original"
}
`
	if err := os.WriteFile(file1, []byte(originalContent), 0644); err != nil {
		t.Fatal(err)
	}

	cache := NewFunctionExclusionCache(tmpDir)
	if err := cache.ScanPackage([]string{file1}); err != nil {
		t.Fatal(err)
	}

	// Test 1: Only implementation changed (fast path)
	t.Run("implementation changed", func(t *testing.T) {
		newContent := `package main

func ReadFile(path string) string {
	// New implementation
	return "modified"
}
`
		if err := os.WriteFile(file1, []byte(newContent), 0644); err != nil {
			t.Fatal(err)
		}

		// Should detect no symbol changes
		changed := cache.quickScanFileSymbolsChanged(file1, []byte(newContent))
		if changed {
			t.Error("Expected no symbol changes when only implementation changed")
		}
	})

	// Test 2: Function signature changed (slow path)
	t.Run("signature changed", func(t *testing.T) {
		newContent := `package main

func ReadFile(path string, mode int) string {
	return "data"
}
`
		// Note: Signature changes don't affect function names, so this should still be false
		// unless the function name itself changes
		changed := cache.quickScanFileSymbolsChanged(file1, []byte(newContent))
		if changed {
			t.Error("Expected no symbol changes when signature changed (name unchanged)")
		}
	})

	// Test 3: Function added (slow path)
	t.Run("function added", func(t *testing.T) {
		newContent := `package main

func ReadFile(path string) string {
	return "data"
}

func NewFunction() {
	// new
}
`
		changed := cache.quickScanFileSymbolsChanged(file1, []byte(newContent))
		if !changed {
			t.Error("Expected symbol changes when function added")
		}
	})

	// Test 4: Function removed (slow path)
	t.Run("function removed", func(t *testing.T) {
		newContent := `package main

// ReadFile was removed
`
		changed := cache.quickScanFileSymbolsChanged(file1, []byte(newContent))
		if !changed {
			t.Error("Expected symbol changes when function removed")
		}
	})
}

func TestContainsUnqualifiedPattern(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected bool
	}{
		{
			"unqualified stdlib call",
			"data := ReadFile(path)",
			true,
		},
		{
			"qualified call",
			"data := os.ReadFile(path)",
			false,
		},
		{
			"multiple unqualified",
			"Printf(\"hello\")\nAtoi(str)",
			true,
		},
		{
			"no function calls",
			"package main\nvar x = 42",
			false,
		},
		{
			"lowercase function",
			"data := readFile(path)",
			false,
		},
		{
			"type declaration",
			"type User struct { Name string }",
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := containsUnqualifiedPattern([]byte(tt.content))
			if result != tt.expected {
				t.Errorf("containsUnqualifiedPattern(%q) = %v, want %v",
					tt.content, result, tt.expected)
			}
		})
	}
}

func TestCacheInvalidation(t *testing.T) {
	tmpDir := t.TempDir()

	file1 := filepath.Join(tmpDir, "file1.dingo")
	content := `package main

func ReadFile(path string) string {
	return "data"
}
`
	if err := os.WriteFile(file1, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Initial scan
	cache := NewFunctionExclusionCache(tmpDir)
	if err := cache.ScanPackage([]string{file1}); err != nil {
		t.Fatal(err)
	}

	// Save to disk
	if err := cache.SaveToDisk(); err != nil {
		t.Fatal(err)
	}

	// Wait a bit to ensure mod time changes
	time.Sleep(10 * time.Millisecond)

	// Modify file
	newContent := `package main

func ReadFile(path string) string {
	return "new data"
}

func NewFunc() {}
`
	if err := os.WriteFile(file1, []byte(newContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create new cache instance and load from disk
	cache2 := NewFunctionExclusionCache(tmpDir)
	if err := cache2.LoadFromDisk(); err != nil {
		t.Fatal(err)
	}

	// Check if rescan is needed (should be true since NewFunc was added)
	if !cache2.NeedsRescan([]string{file1}) {
		t.Error("Expected rescan to be needed after file modification")
	}

	// Rescan
	if err := cache2.ScanPackage([]string{file1}); err != nil {
		t.Fatal(err)
	}

	// Verify new function is detected
	if !cache2.IsLocalSymbol("NewFunc") {
		t.Error("Expected NewFunc to be detected after rescan")
	}
}

func TestConcurrentAccess(t *testing.T) {
	cache := NewFunctionExclusionCache("/tmp/test-concurrent")

	// Populate cache
	cache.mu.Lock()
	cache.localFunctions["Func1"] = true
	cache.localFunctions["Func2"] = true
	cache.localFunctions["Func3"] = true
	cache.mu.Unlock()

	// Concurrent readers
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				cache.IsLocalSymbol("Func1")
				cache.IsLocalSymbol("Func2")
				cache.IsLocalSymbol("Func3")
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify no data races (test will fail with -race if there are issues)
	metrics := cache.Metrics()
	if metrics.CacheHits == 0 {
		t.Error("Expected cache hits from concurrent access")
	}
}

func TestEmptyPackage(t *testing.T) {
	tmpDir := t.TempDir()

	cache := NewFunctionExclusionCache(tmpDir)

	// Scan empty package
	err := cache.ScanPackage([]string{})
	if err != nil {
		t.Fatalf("ScanPackage failed on empty package: %v", err)
	}

	// Verify no symbols
	metrics := cache.Metrics()
	if metrics.TotalSymbols != 0 {
		t.Errorf("Expected 0 symbols in empty package, got %d", metrics.TotalSymbols)
	}

	// Save and load should work
	if err := cache.SaveToDisk(); err != nil {
		t.Fatalf("SaveToDisk failed: %v", err)
	}

	cache2 := NewFunctionExclusionCache(tmpDir)
	if err := cache2.LoadFromDisk(); err != nil {
		t.Fatalf("LoadFromDisk failed: %v", err)
	}
}

func TestPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	tmpDir := t.TempDir()

	// Create 10 test files
	files := make([]string, 10)
	for i := 0; i < 10; i++ {
		file := filepath.Join(tmpDir, fmt.Sprintf("file%d_test.dingo", i))
		content := `package main

func ReadFile(path string) string { return "data" }
func ProcessData(data string) {}
func ValidateInput(input string) bool { return true }
func TransformData(data string) string { return data }
func SaveResult(result string) error { return nil }
`
		if err := os.WriteFile(file, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
		files[i] = file
	}

	// Measure cold start time
	cache := NewFunctionExclusionCache(tmpDir)
	start := time.Now()
	if err := cache.ScanPackage(files); err != nil {
		t.Fatal(err)
	}
	coldStartDuration := time.Since(start)

	t.Logf("Cold start (10 files): %v", coldStartDuration)

	// Target: <100ms for 10 files (plan says ~50ms)
	if coldStartDuration > 100*time.Millisecond {
		t.Errorf("Cold start too slow: %v (target: <100ms)", coldStartDuration)
	}

	// Measure cache hit time
	start = time.Now()
	for i := 0; i < 1000; i++ {
		cache.IsLocalSymbol("ReadFile")
	}
	cacheHitDuration := time.Since(start) / 1000

	t.Logf("Cache hit (avg): %v", cacheHitDuration)

	// Target: <10ms (plan says ~1ms, but allow margin)
	if cacheHitDuration > 10*time.Millisecond {
		t.Errorf("Cache hit too slow: %v (target: <10ms)", cacheHitDuration)
	}
}
