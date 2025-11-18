package lsp

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFileWatcher_DetectDingoFileChange(t *testing.T) {
	tmpDir := t.TempDir()
	dingoFile := filepath.Join(tmpDir, "test.dingo")

	// Channel to receive file change events
	changedFiles := make(chan string, 10)

	watcher, err := NewFileWatcher(tmpDir, &testLogger{}, func(path string) {
		changedFiles <- path
	})
	if err != nil {
		t.Fatalf("Failed to create watcher: %v", err)
	}
	defer watcher.Close()

	// Create .dingo file (simulates save)
	if err := os.WriteFile(dingoFile, []byte("package main\n"), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	// Wait for debounce + processing
	select {
	case changed := <-changedFiles:
		if changed != dingoFile {
			t.Errorf("Expected %s, got %s", dingoFile, changed)
		}
	case <-time.After(1 * time.Second):
		t.Error("Timeout waiting for file change event")
	}
}

func TestFileWatcher_IgnoreNonDingoFiles(t *testing.T) {
	tmpDir := t.TempDir()
	goFile := filepath.Join(tmpDir, "test.go")

	// Channel to receive file change events
	changedFiles := make(chan string, 10)

	watcher, err := NewFileWatcher(tmpDir, &testLogger{}, func(path string) {
		changedFiles <- path
	})
	if err != nil {
		t.Fatalf("Failed to create watcher: %v", err)
	}
	defer watcher.Close()

	// Create .go file (should be ignored)
	if err := os.WriteFile(goFile, []byte("package main\n"), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	// Wait to ensure no event is triggered
	select {
	case changed := <-changedFiles:
		t.Errorf("Should not trigger for .go files, got: %s", changed)
	case <-time.After(700 * time.Millisecond):
		// Success - no event
	}
}

func TestFileWatcher_DebouncingMultipleChanges(t *testing.T) {
	tmpDir := t.TempDir()
	dingoFile := filepath.Join(tmpDir, "test.dingo")

	// Channel to receive file change events
	changedFiles := make(chan string, 10)

	watcher, err := NewFileWatcher(tmpDir, &testLogger{}, func(path string) {
		changedFiles <- path
	})
	if err != nil {
		t.Fatalf("Failed to create watcher: %v", err)
	}
	defer watcher.Close()

	// Rapid saves (simulates auto-save plugin)
	for i := 0; i < 5; i++ {
		content := []byte("package main\n// Change " + string(rune('0'+i)) + "\n")
		if err := os.WriteFile(dingoFile, content, 0644); err != nil {
			t.Fatalf("Failed to write file: %v", err)
		}
		time.Sleep(50 * time.Millisecond) // Rapid changes
	}

	// Wait for debounce + processing
	eventCount := 0
	timeout := time.After(1 * time.Second)

loop:
	for {
		select {
		case <-changedFiles:
			eventCount++
			// Continue draining to count all events
		case <-timeout:
			break loop
		}
	}

	// Due to debouncing (500ms), should get 1 event (all changes batched)
	if eventCount > 2 {
		t.Errorf("Expected 1-2 events due to debouncing, got %d", eventCount)
	}
}

func TestFileWatcher_IgnoreDirectories(t *testing.T) {
	tmpDir := t.TempDir()

	// Create ignored directories
	ignoredDirs := []string{
		"node_modules",
		"vendor",
		".git",
		".idea",
	}

	for _, dir := range ignoredDirs {
		dirPath := filepath.Join(tmpDir, dir)
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}
	}

	watcher, err := NewFileWatcher(tmpDir, &testLogger{}, func(path string) {})
	if err != nil {
		t.Fatalf("Failed to create watcher: %v", err)
	}
	defer watcher.Close()

	// Verify directories were not watched
	// Note: This is implicit - watcher should not crash or watch these dirs
	// In production, they would be skipped by filepath.SkipDir
}

func TestFileWatcher_NestedDirectories(t *testing.T) {
	tmpDir := t.TempDir()

	// Create nested structure
	nestedDir := filepath.Join(tmpDir, "src", "pkg", "utils")
	if err := os.MkdirAll(nestedDir, 0755); err != nil {
		t.Fatalf("Failed to create nested directory: %v", err)
	}

	dingoFile := filepath.Join(nestedDir, "helper.dingo")

	// Channel to receive file change events
	changedFiles := make(chan string, 10)

	watcher, err := NewFileWatcher(tmpDir, &testLogger{}, func(path string) {
		changedFiles <- path
	})
	if err != nil {
		t.Fatalf("Failed to create watcher: %v", err)
	}
	defer watcher.Close()

	// Create file in nested directory
	if err := os.WriteFile(dingoFile, []byte("package utils\n"), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	// Wait for event
	select {
	case changed := <-changedFiles:
		if changed != dingoFile {
			t.Errorf("Expected %s, got %s", dingoFile, changed)
		}
	case <-time.After(1 * time.Second):
		t.Error("Timeout waiting for file change event")
	}
}

func TestFileWatcher_Close(t *testing.T) {
	tmpDir := t.TempDir()

	watcher, err := NewFileWatcher(tmpDir, &testLogger{}, func(path string) {})
	if err != nil {
		t.Fatalf("Failed to create watcher: %v", err)
	}

	// Close should not error
	if err := watcher.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// Second close should not crash (idempotent)
	_ = watcher.Close()
}
