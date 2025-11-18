package main

import (
	"context"
	"io"
	"os"
	"os/exec"

	"github.com/MadAppGang/dingo/pkg/lsp"
	"go.lsp.dev/jsonrpc2"
)

func main() {
	// Configure logging from environment variable
	logLevel := os.Getenv("DINGO_LSP_LOG")
	if logLevel == "" {
		logLevel = "info"
	}
	logger := lsp.NewLogger(logLevel, os.Stderr)

	logger.Infof("Starting dingo-lsp server (log level: %s)", logLevel)

	// Find gopls in $PATH
	goplsPath := findGopls(logger)
	if goplsPath == "" {
		logger.Fatalf("gopls not found in $PATH. Install: go install golang.org/x/tools/gopls@latest")
	}

	// Create LSP proxy server
	server, err := lsp.NewServer(lsp.ServerConfig{
		Logger:        logger,
		GoplsPath:     goplsPath,
		AutoTranspile: true, // Default from user decision
	})
	if err != nil {
		logger.Fatalf("Failed to create server: %v", err)
	}

	// Create stdio transport using ReadWriteCloser wrapper
	rwc := &stdinoutCloser{stdin: os.Stdin, stdout: os.Stdout}
	stream := jsonrpc2.NewStream(rwc)
	conn := jsonrpc2.NewConn(stream)

	// Start serving
	ctx := context.Background()
	if err := server.Serve(ctx, conn); err != nil {
		logger.Errorf("Server error: %v", err)
		os.Exit(1)
	}

	logger.Infof("Server stopped")
}

// findGopls looks for gopls binary in $PATH
func findGopls(logger lsp.Logger) string {
	path, err := exec.LookPath("gopls")
	if err != nil {
		logger.Debugf("gopls not found in $PATH: %v", err)
		return ""
	}
	logger.Infof("Found gopls at: %s", path)
	return path
}

// stdinoutCloser wraps os.Stdin and os.Stdout as ReadWriteCloser
type stdinoutCloser struct {
	stdin  *os.File
	stdout *os.File
}

func (s *stdinoutCloser) Read(p []byte) (n int, err error) {
	return s.stdin.Read(p)
}

func (s *stdinoutCloser) Write(p []byte) (n int, err error) {
	return s.stdout.Write(p)
}

func (s *stdinoutCloser) Close() error {
	// Don't actually close stdin/stdout
	return nil
}

var _ io.ReadWriteCloser = (*stdinoutCloser)(nil)

