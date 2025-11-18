package lsp

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"go.lsp.dev/protocol"
)

// AutoTranspiler handles automatic transpilation of .dingo files
type AutoTranspiler struct {
	logger   Logger
	mapCache *SourceMapCache
	gopls    *GoplsClient
}

// NewAutoTranspiler creates an auto-transpiler instance
func NewAutoTranspiler(logger Logger, mapCache *SourceMapCache, gopls *GoplsClient) *AutoTranspiler {
	return &AutoTranspiler{
		logger:   logger,
		mapCache: mapCache,
		gopls:    gopls,
	}
}

// TranspileFile transpiles a single .dingo file
func (at *AutoTranspiler) TranspileFile(ctx context.Context, dingoPath string) error {
	at.logger.Infof("Transpiling: %s", dingoPath)

	// IMPORTANT FIX I7: Add timeout for transpilation
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Execute dingo build
	cmd := exec.CommandContext(ctx, "dingo", "build", dingoPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Parse output for error details
		errMsg := string(output)
		if errMsg == "" {
			errMsg = err.Error()
		}
		return fmt.Errorf("transpilation failed: %s", strings.TrimSpace(errMsg))
	}

	at.logger.Infof("Transpilation successful: %s", dingoPath)
	return nil
}

// OnFileChange handles a .dingo file change (called by watcher)
func (at *AutoTranspiler) OnFileChange(ctx context.Context, dingoPath string) {
	// Transpile the file
	if err := at.TranspileFile(ctx, dingoPath); err != nil {
		at.logger.Errorf("Auto-transpile failed for %s: %v", dingoPath, err)
		// Note: Diagnostic publishing would happen here when IDE connection is ready
		// For now, we just log the error
		return
	}

	// Invalidate source map cache
	goPath := dingoToGoPath(dingoPath)
	at.mapCache.Invalidate(goPath)
	at.logger.Debugf("Source map cache invalidated: %s", goPath)

	// Notify gopls of .go file change
	if err := at.notifyGoplsFileChange(ctx, goPath); err != nil {
		at.logger.Warnf("Failed to notify gopls of file change: %v", err)
	}
}

// notifyGoplsFileChange notifies gopls that a .go file changed
func (at *AutoTranspiler) notifyGoplsFileChange(ctx context.Context, goPath string) error {
	return at.gopls.NotifyFileChange(ctx, goPath)
}

// ParseTranspileError parses transpiler output into LSP diagnostic
// Returns nil if output is not an error
func ParseTranspileError(dingoPath string, output string) *protocol.Diagnostic {
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
