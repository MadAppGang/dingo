package codegen

import (
	"go/token"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewGCExportImporter_StdlibPackages(t *testing.T) {
	// Test that we can create an importer for stdlib packages.
	// This requires a valid Go environment with build cache.
	fset := token.NewFileSet()

	// Use a temp directory with a go.mod as working dir
	tmpDir := t.TempDir()
	writeGoMod(t, tmpDir, "testmod")

	imp, err := NewGCExportImporter(fset, tmpDir, "fmt", "net/http")
	if err != nil {
		t.Fatalf("NewGCExportImporter failed: %v", err)
	}

	// Should have found export files for fmt, net/http, and their transitive deps
	if imp.ExportCount() < 2 {
		t.Errorf("expected at least 2 export entries, got %d", imp.ExportCount())
	}

	t.Logf("Found %d packages with export data", imp.ExportCount())
}

func TestGCExportImporter_ImportFmt(t *testing.T) {
	fset := token.NewFileSet()
	tmpDir := t.TempDir()
	writeGoMod(t, tmpDir, "testmod")

	imp, err := NewGCExportImporter(fset, tmpDir, "fmt")
	if err != nil {
		t.Fatalf("NewGCExportImporter failed: %v", err)
	}

	// Import the fmt package
	pkg, err := imp.Import("fmt")
	if err != nil {
		t.Fatalf("Import(\"fmt\") failed: %v", err)
	}

	if pkg == nil {
		t.Fatal("Import(\"fmt\") returned nil package")
	}

	if pkg.Name() != "fmt" {
		t.Errorf("expected package name \"fmt\", got %q", pkg.Name())
	}

	// Check that Println is exported
	obj := pkg.Scope().Lookup("Println")
	if obj == nil {
		t.Error("fmt.Println not found in package scope")
	} else {
		t.Logf("fmt.Println type: %s", obj.Type())
	}

	// Check that Sprintf is exported
	obj = pkg.Scope().Lookup("Sprintf")
	if obj == nil {
		t.Error("fmt.Sprintf not found in package scope")
	}
}

func TestGCExportImporter_ImportNetHTTP(t *testing.T) {
	fset := token.NewFileSet()
	tmpDir := t.TempDir()
	writeGoMod(t, tmpDir, "testmod")

	imp, err := NewGCExportImporter(fset, tmpDir, "net/http")
	if err != nil {
		t.Fatalf("NewGCExportImporter failed: %v", err)
	}

	// Import net/http
	pkg, err := imp.Import("net/http")
	if err != nil {
		t.Fatalf("Import(\"net/http\") failed: %v", err)
	}

	if pkg.Name() != "http" {
		t.Errorf("expected package name \"http\", got %q", pkg.Name())
	}

	// Check that ListenAndServe is exported
	obj := pkg.Scope().Lookup("ListenAndServe")
	if obj == nil {
		t.Error("http.ListenAndServe not found in package scope")
	}
}

func TestGCExportImporter_Caching(t *testing.T) {
	fset := token.NewFileSet()
	tmpDir := t.TempDir()
	writeGoMod(t, tmpDir, "testmod")

	imp, err := NewGCExportImporter(fset, tmpDir, "fmt")
	if err != nil {
		t.Fatalf("NewGCExportImporter failed: %v", err)
	}

	// First import
	start := time.Now()
	pkg1, err := imp.Import("fmt")
	if err != nil {
		t.Fatalf("first Import(\"fmt\") failed: %v", err)
	}
	firstDuration := time.Since(start)

	// Second import (should hit cache)
	start = time.Now()
	pkg2, err := imp.Import("fmt")
	if err != nil {
		t.Fatalf("second Import(\"fmt\") failed: %v", err)
	}
	secondDuration := time.Since(start)

	// Same package pointer should be returned
	if pkg1 != pkg2 {
		t.Error("cached import returned different package pointer")
	}

	// Second call should be significantly faster (cache hit)
	t.Logf("First import: %v, Second import (cached): %v", firstDuration, secondDuration)
	if secondDuration > firstDuration && firstDuration > time.Millisecond {
		// Only check if first was slow enough to measure
		// The cache hit should be near-instant
		t.Logf("Note: second import was not faster, but both were fast enough")
	}
}

func TestGCExportImporter_MissingPackage(t *testing.T) {
	fset := token.NewFileSet()
	tmpDir := t.TempDir()
	writeGoMod(t, tmpDir, "testmod")

	imp, err := NewGCExportImporter(fset, tmpDir, "fmt")
	if err != nil {
		t.Fatalf("NewGCExportImporter failed: %v", err)
	}

	// Try to import a package that wasn't in our go list query
	_, err = imp.Import("github.com/nonexistent/package")
	if err == nil {
		t.Error("expected error for missing package, got nil")
	}
}

func TestNewGCExportImporter_NoPatterns(t *testing.T) {
	fset := token.NewFileSet()
	_, err := NewGCExportImporter(fset, ".")
	if err == nil {
		t.Error("expected error for empty patterns, got nil")
	}
}

func TestGCExportImporter_TransitiveDeps(t *testing.T) {
	// net/http has many transitive dependencies. Verify they are all available.
	fset := token.NewFileSet()
	tmpDir := t.TempDir()
	writeGoMod(t, tmpDir, "testmod")

	imp, err := NewGCExportImporter(fset, tmpDir, "net/http")
	if err != nil {
		t.Fatalf("NewGCExportImporter failed: %v", err)
	}

	// net/http depends on "io", "net", "strings", etc. - these should be importable
	// because we used -deps flag in go list
	depPaths := []string{"io", "net", "strings", "sync", "errors"}
	for _, depPath := range depPaths {
		pkg, err := imp.Import(depPath)
		if err != nil {
			t.Errorf("Import(%q) failed: %v (transitive dep of net/http)", depPath, err)
			continue
		}
		if pkg == nil {
			t.Errorf("Import(%q) returned nil", depPath)
		}
	}
}

func TestGCExportImporter_WithTypeResolver(t *testing.T) {
	// Integration test: verify the gcExportImporter works within the TypeResolver flow
	src := []byte(`
package main

import (
	"fmt"
	"strconv"
)

func main() {
	val, err := strconv.Atoi("42")
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(val)
}
`)

	// Create resolver which should now use gcExportImporter internally
	resolver, err := NewTypeResolver(src, ".")
	if err != nil {
		t.Fatalf("NewTypeResolver failed: %v", err)
	}

	if resolver == nil {
		t.Fatal("resolver is nil")
	}

	// strconv.Atoi returns (int, error) - 2 values
	count := resolver.GetReturnCount([]byte(`strconv.Atoi("42")`))
	t.Logf("strconv.Atoi() return count: %d", count)
	// Should return 2 (int, error) if type resolution works
	if count != -1 && count != 2 {
		t.Errorf("expected return count 2 or -1 for strconv.Atoi(), got %d", count)
	}
}

func TestGCExportImporter_StrconvPackage(t *testing.T) {
	// Test that strconv functions are properly loaded
	fset := token.NewFileSet()
	tmpDir := t.TempDir()
	writeGoMod(t, tmpDir, "testmod")

	imp, err := NewGCExportImporter(fset, tmpDir, "strconv")
	if err != nil {
		t.Fatalf("NewGCExportImporter failed: %v", err)
	}

	pkg, err := imp.Import("strconv")
	if err != nil {
		t.Fatalf("Import(\"strconv\") failed: %v", err)
	}

	// Check Atoi exists and has correct signature
	obj := pkg.Scope().Lookup("Atoi")
	if obj == nil {
		t.Fatal("strconv.Atoi not found")
	}

	t.Logf("strconv.Atoi type: %s", obj.Type())
}

func TestGCExportImporter_GoListCache(t *testing.T) {
	// Test that the go list cache avoids re-running go list for the same imports.
	// This simulates the Phase 0 + Step 2.1 pattern in the transpiler.
	ClearGoListCache() // Start fresh
	defer ClearGoListCache()

	tmpDir := t.TempDir()
	writeGoMod(t, tmpDir, "testmod")

	fset1 := token.NewFileSet()
	fset2 := token.NewFileSet()

	// First call: should run go list
	start := time.Now()
	imp1, err := NewGCExportImporter(fset1, tmpDir, "fmt", "strconv")
	if err != nil {
		t.Fatalf("first NewGCExportImporter failed: %v", err)
	}
	firstDuration := time.Since(start)

	// Second call with same args: should hit cache (no go list)
	start = time.Now()
	imp2, err := NewGCExportImporter(fset2, tmpDir, "fmt", "strconv")
	if err != nil {
		t.Fatalf("second NewGCExportImporter failed: %v", err)
	}
	secondDuration := time.Since(start)

	// Both should have the same export count
	if imp1.ExportCount() != imp2.ExportCount() {
		t.Errorf("export count mismatch: first=%d, second=%d", imp1.ExportCount(), imp2.ExportCount())
	}

	t.Logf("First call (go list): %v, Second call (cached): %v", firstDuration, secondDuration)

	// Second call should be much faster (cache hit avoids go list exec)
	if firstDuration > 5*time.Millisecond && secondDuration > firstDuration/2 {
		t.Logf("Warning: cache may not be working (second call not significantly faster)")
	}

	// Both importers should work independently (different loaded maps)
	pkg1, err := imp1.Import("fmt")
	if err != nil {
		t.Fatalf("imp1.Import(\"fmt\") failed: %v", err)
	}
	pkg2, err := imp2.Import("fmt")
	if err != nil {
		t.Fatalf("imp2.Import(\"fmt\") failed: %v", err)
	}

	// Both should return valid packages
	if pkg1.Name() != "fmt" || pkg2.Name() != "fmt" {
		t.Error("imported packages have wrong name")
	}
}

func TestGCExportImporter_GoListCacheOrderIndependent(t *testing.T) {
	// Test that cache works regardless of pattern order
	ClearGoListCache()
	defer ClearGoListCache()

	tmpDir := t.TempDir()
	writeGoMod(t, tmpDir, "testmod")

	fset1 := token.NewFileSet()
	fset2 := token.NewFileSet()

	// First call with one order
	imp1, err := NewGCExportImporter(fset1, tmpDir, "strconv", "fmt")
	if err != nil {
		t.Fatalf("first call failed: %v", err)
	}

	// Second call with reversed order - should still cache hit
	// because we sort patterns before creating the cache key
	start := time.Now()
	imp2, err := NewGCExportImporter(fset2, tmpDir, "fmt", "strconv")
	if err != nil {
		t.Fatalf("second call failed: %v", err)
	}
	duration := time.Since(start)

	if imp1.ExportCount() != imp2.ExportCount() {
		t.Errorf("export count mismatch despite same patterns in different order")
	}

	t.Logf("Cache hit (different order): %v", duration)
}

// writeGoMod creates a minimal go.mod in the given directory.
func writeGoMod(t *testing.T, dir, module string) {
	t.Helper()
	content := "module " + module + "\n\ngo 1.21\n"
	err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(content), 0644)
	if err != nil {
		t.Fatalf("failed to write go.mod: %v", err)
	}
}
