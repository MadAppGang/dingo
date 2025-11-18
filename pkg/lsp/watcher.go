package lsp

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// FileWatcher monitors workspace for .dingo file changes
type FileWatcher struct {
	watcher       *fsnotify.Watcher
	logger        Logger
	onChange      func(dingoPath string)
	debounceTimer *time.Timer
	debounceDur   time.Duration
	pendingFiles  map[string]bool
	mu            sync.Mutex
	done          chan struct{}
	closed        bool
}

// NewFileWatcher creates a file watcher for the workspace
func NewFileWatcher(
	workspaceRoot string,
	logger Logger,
	onChange func(dingoPath string),
) (*FileWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	fw := &FileWatcher{
		watcher:      watcher,
		logger:       logger,
		onChange:     onChange,
		debounceDur:  500 * time.Millisecond, // User decision: 500ms debounce
		pendingFiles: make(map[string]bool),
		done:         make(chan struct{}),
	}

	// Watch workspace recursively
	if err := fw.watchRecursive(workspaceRoot); err != nil {
		watcher.Close()
		return nil, err
	}

	// Start event loop
	go fw.watchLoop()

	logger.Infof("File watcher started (workspace: %s, debounce: %s)", workspaceRoot, fw.debounceDur)
	return fw, nil
}

// watchRecursive adds all directories in the workspace to the watcher
func (fw *FileWatcher) watchRecursive(root string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip ignored directories
		if info.IsDir() && fw.shouldIgnore(path) {
			fw.logger.Debugf("Ignoring directory: %s", path)
			return filepath.SkipDir
		}

		// Watch directories only (fsnotify watches files within directories)
		if info.IsDir() {
			if err := fw.watcher.Add(path); err != nil {
				fw.logger.Warnf("Failed to watch %s: %v", path, err)
			} else {
				fw.logger.Debugf("Watching directory: %s", path)
			}
		}

		return nil
	})
}

// shouldIgnore checks if a directory should be ignored
func (fw *FileWatcher) shouldIgnore(path string) bool {
	base := filepath.Base(path)

	// User decision: Ignore common directories
	ignoreDirs := []string{
		"node_modules",
		"vendor",
		".git",
		".dingo_cache",
		"dist",
		"build",
		".idea",
		".vscode",
		"bin",
		"obj",
	}

	for _, ignore := range ignoreDirs {
		if base == ignore {
			return true
		}
	}

	// Ignore hidden directories (start with .)
	if strings.HasPrefix(base, ".") && base != "." {
		return true
	}

	return false
}

// watchLoop processes file system events
func (fw *FileWatcher) watchLoop() {
	for {
		select {
		case event, ok := <-fw.watcher.Events:
			if !ok {
				return
			}

			// IMPORTANT FIX I1: Handle directory creation
			if event.Op&fsnotify.Create == fsnotify.Create {
				info, err := os.Stat(event.Name)
				if err == nil && info.IsDir() {
					if !fw.shouldIgnore(event.Name) {
						if err := fw.watcher.Add(event.Name); err != nil {
							fw.logger.Warnf("Failed to watch new directory %s: %v", event.Name, err)
						} else {
							fw.logger.Debugf("Started watching new directory: %s", event.Name)
						}
					}
				}
			}

			// Filter: Only .dingo files (user decision: hybrid workspace strategy)
			if !isDingoFilePath(event.Name) {
				continue
			}

			// Handle write/create events
			if event.Op&fsnotify.Write == fsnotify.Write ||
				event.Op&fsnotify.Create == fsnotify.Create {
				fw.logger.Debugf("File event: %s (%s)", event.Name, event.Op.String())
				fw.handleFileChange(event.Name)
			}

		case err, ok := <-fw.watcher.Errors:
			if !ok {
				return
			}
			fw.logger.Errorf("File watcher error: %v", err)

		case <-fw.done:
			return
		}
	}
}

// handleFileChange adds a file to the pending set and resets debounce timer
func (fw *FileWatcher) handleFileChange(dingoPath string) {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	// Add to pending files
	fw.pendingFiles[dingoPath] = true

	// Reset debounce timer (user decision: 500ms to batch rapid saves)
	if fw.debounceTimer != nil {
		fw.debounceTimer.Stop()
	}

	fw.debounceTimer = time.AfterFunc(fw.debounceDur, func() {
		fw.processPendingFiles()
	})
}

// processPendingFiles processes all files that changed within the debounce window
func (fw *FileWatcher) processPendingFiles() {
	fw.mu.Lock()
	files := make([]string, 0, len(fw.pendingFiles))
	for path := range fw.pendingFiles {
		files = append(files, path)
	}
	fw.pendingFiles = make(map[string]bool)
	fw.mu.Unlock()

	// Process each file
	for _, path := range files {
		fw.logger.Debugf("Processing debounced file change: %s", path)
		fw.onChange(path)
	}
}

// Close stops the file watcher (idempotent)
func (fw *FileWatcher) Close() error {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	if fw.closed {
		return nil // Already closed
	}

	fw.closed = true
	close(fw.done)
	return fw.watcher.Close()
}
