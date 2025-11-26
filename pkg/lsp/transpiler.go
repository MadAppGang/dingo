package lsp

import (
	"context"
	"fmt"
	"strings"

	"go.lsp.dev/protocol"
	lspuri "go.lsp.dev/uri"
	"github.com/MadAppGang/dingo/pkg/transpiler"
)

// AutoTranspiler handles automatic transpilation of .dingo files
type AutoTranspiler struct {
	logger     Logger
	mapCache   *SourceMapCache
	gopls      *GoplsClient
	transpiler *transpiler.Transpiler
	server     *Server // For publishing Dingo-specific diagnostics
}

// NewAutoTranspiler creates an auto-transpiler instance
func NewAutoTranspiler(logger Logger, mapCache *SourceMapCache, gopls *GoplsClient, server *Server) *AutoTranspiler {
	// Create integrated transpiler
	t, err := transpiler.New()
	if err != nil {
		// Fall back to nil transpiler - will fail at transpile time
		logger.Warnf("Failed to create transpiler: %v", err)
	}

	return &AutoTranspiler{
		logger:     logger,
		mapCache:   mapCache,
		gopls:      gopls,
		transpiler: t,
		server:     server,
	}
}

// TranspileFile transpiles a single .dingo file
func (at *AutoTranspiler) TranspileFile(ctx context.Context, dingoPath string) error {
	at.logger.Infof("Auto-rebuild: %s", dingoPath)

	if at.transpiler == nil {
		return fmt.Errorf("transpiler not initialized")
	}

	// Use integrated transpiler library (no shell out!)
	err := at.transpiler.TranspileFile(dingoPath)
	if err != nil {
		return fmt.Errorf("transpilation failed: %w", err)
	}

	at.logger.Infof("Auto-rebuild complete: %s", dingoPath)
	return nil
}

// OnFileChange handles a .dingo file change (called by watcher)
func (at *AutoTranspiler) OnFileChange(ctx context.Context, dingoPath string) {
	uri := protocol.DocumentURI(lspuri.File(dingoPath))

	// Transpile the file
	if err := at.TranspileFile(ctx, dingoPath); err != nil {
		at.logger.Errorf("Auto-transpile failed for %s: %v", dingoPath, err)

		// Publish Dingo-specific diagnostic for transpilation error
		diagnostic := ParseTranspileError(dingoPath, err.Error())
		if diagnostic != nil && at.server != nil {
			at.server.publishDingoDiagnostics(uri, []protocol.Diagnostic{*diagnostic})
		}
		return // Don't proceed to gopls sync
	}

	// Clear diagnostics on successful transpilation
	if at.server != nil {
		at.server.publishDingoDiagnostics(uri, []protocol.Diagnostic{})
	}

	// Invalidate source map cache
	goPath := dingoToGoPath(dingoPath)
	at.mapCache.Invalidate(goPath)
	at.logger.Debugf("Source map cache invalidated: %s", goPath)

	// CRITICAL FIX: Synchronize gopls with new .go file content
	// This ensures gopls has the latest transpiled content in memory
	if err := at.syncGoplsWithGoFile(ctx, goPath); err != nil {
		at.logger.Warnf("Failed to sync gopls with .go file: %v", err)
	}
}

// syncGoplsWithGoFile sends the new .go file content to gopls via didChange
// This ensures gopls has the latest transpiled content in memory
func (at *AutoTranspiler) syncGoplsWithGoFile(ctx context.Context, goPath string) error {
	at.logger.Debugf("Synchronizing gopls with updated .go file: %s", goPath)
	return at.gopls.SyncFileContent(ctx, goPath)
}

// ParseTranspileError parses transpiler output into LSP diagnostic
// Returns nil if output is not an error
func ParseTranspileError(dingoPath, output string) *protocol.Diagnostic {
	// Simple heuristic: check for common error patterns
	// Format: "file.dingo:10:5: error message"
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, dingoPath) && strings.Contains(line, ":") {
			parts := strings.SplitN(line, ":", 4)
			if len(parts) >= 4 {
				// Try to extract line:col:message
				var lineNum, colNum int
				_, err1 := fmt.Sscanf(parts[1], "%d", &lineNum)
				_, err2 := fmt.Sscanf(parts[2], "%d", &colNum)
				if err1 == nil && err2 == nil {
					message := strings.TrimSpace(parts[3])
					return &protocol.Diagnostic{
						Range: protocol.Range{
							Start: protocol.Position{
								Line:      uint32(lineNum - 1), // 0-based
								Character: uint32(colNum - 1),  // 0-based
							},
							End: protocol.Position{
								Line:      uint32(lineNum - 1),
								Character: uint32(colNum - 1),
							},
						},
						Severity: protocol.DiagnosticSeverityError,
						Source:   "dingo",
						Message:  message,
					}
				}
			}
		}
	}

	// Fallback: generic error at top of file
	if strings.Contains(output, "error") || strings.Contains(output, "failed") {
		return &protocol.Diagnostic{
			Range: protocol.Range{
				Start: protocol.Position{Line: 0, Character: 0},
				End:   protocol.Position{Line: 0, Character: 0},
			},
			Severity: protocol.DiagnosticSeverityError,
			Source:   "dingo",
			Message:  strings.TrimSpace(output),
		}
	}

	return nil
}
