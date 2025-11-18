package lsp

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"sync"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// GoplsClient manages a gopls subprocess and forwards LSP requests
type GoplsClient struct {
	cmd          *exec.Cmd
	conn         jsonrpc2.Conn
	logger       Logger
	goplsPath    string
	restarts     int
	maxRestarts  int
	mu           sync.Mutex
	shuttingDown bool           // CRITICAL FIX C2: Track shutdown state
	closeMu      sync.Mutex     // CRITICAL FIX C2: Protect shutdown flag
}

// NewGoplsClient creates and starts a gopls subprocess
func NewGoplsClient(goplsPath string, logger Logger) (*GoplsClient, error) {
	// Verify gopls exists
	if _, err := exec.LookPath(goplsPath); err != nil {
		return nil, fmt.Errorf("gopls not found at %s: %w (install: go install golang.org/x/tools/gopls@latest)", goplsPath, err)
	}

	client := &GoplsClient{
		logger:      logger,
		goplsPath:   goplsPath,
		maxRestarts: 3,
	}

	if err := client.start(); err != nil {
		return nil, err
	}

	return client, nil
}

func (c *GoplsClient) start() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Start gopls subprocess with -mode=stdio
	c.cmd = exec.Command(c.goplsPath, "-mode=stdio")

	stdin, err := c.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := c.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := c.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start gopls
	if err := c.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start gopls: %w", err)
	}

	// Log stderr in background
	go c.logStderr(stderr)

	// Create JSON-RPC connection using a ReadWriteCloser wrapper
	rwc := &readWriteCloser{stdin: stdin, stdout: stdout}
	stream := jsonrpc2.NewStream(rwc)
	c.conn = jsonrpc2.NewConn(stream)

	c.logger.Infof("gopls started (PID: %d)", c.cmd.Process.Pid)

	// CRITICAL FIX C2: Monitor process exit for crash recovery
	go func() {
		err := c.cmd.Wait()

		c.closeMu.Lock()
		shutdown := c.shuttingDown
		c.closeMu.Unlock()

		if err != nil && !shutdown {
			c.logger.Warnf("gopls process exited unexpectedly: %v", err)
			if crashErr := c.handleCrash(); crashErr != nil {
				c.logger.Errorf("Failed to restart gopls: %v", crashErr)
			}
		}
	}()

	return nil
}

func (c *GoplsClient) logStderr(stderr io.Reader) {
	// IMPORTANT FIX I4: Use bufio.Scanner for better handling
	// Handles large panic stack traces without truncation
	scanner := bufio.NewScanner(stderr)
	scanner.Buffer(make([]byte, 4096), 1024*1024) // 4KB initial, 1MB max

	for scanner.Scan() {
		line := scanner.Text()
		c.logger.Debugf("gopls stderr: %s", line)
	}

	if err := scanner.Err(); err != nil && err != io.EOF {
		c.logger.Debugf("stderr scan error: %v", err)
	}
}

// Initialize sends initialize request to gopls
func (c *GoplsClient) Initialize(ctx context.Context, params protocol.InitializeParams) (*protocol.InitializeResult, error) {
	var result protocol.InitializeResult
	_, err := c.conn.Call(ctx, "initialize", params, &result)
	if err != nil {
		return nil, fmt.Errorf("gopls initialize failed: %w", err)
	}
	return &result, nil
}

// Initialized sends initialized notification to gopls
func (c *GoplsClient) Initialized(ctx context.Context, params *protocol.InitializedParams) error {
	return c.conn.Notify(ctx, "initialized", params)
}

// Completion forwards completion request to gopls
func (c *GoplsClient) Completion(ctx context.Context, params protocol.CompletionParams) (*protocol.CompletionList, error) {
	var result protocol.CompletionList
	_, err := c.conn.Call(ctx, "textDocument/completion", params, &result)
	if err != nil {
		return nil, fmt.Errorf("gopls completion failed: %w", err)
	}
	return &result, nil
}

// Definition forwards definition request to gopls
func (c *GoplsClient) Definition(ctx context.Context, params protocol.DefinitionParams) ([]protocol.Location, error) {
	var result []protocol.Location
	_, err := c.conn.Call(ctx, "textDocument/definition", params, &result)
	if err != nil {
		return nil, fmt.Errorf("gopls definition failed: %w", err)
	}
	return result, nil
}

// Hover forwards hover request to gopls
func (c *GoplsClient) Hover(ctx context.Context, params protocol.HoverParams) (*protocol.Hover, error) {
	var result protocol.Hover
	_, err := c.conn.Call(ctx, "textDocument/hover", params, &result)
	if err != nil {
		return nil, fmt.Errorf("gopls hover failed: %w", err)
	}
	return &result, nil
}

// DidOpen notifies gopls of opened file
func (c *GoplsClient) DidOpen(ctx context.Context, params protocol.DidOpenTextDocumentParams) error {
	return c.conn.Notify(ctx, "textDocument/didOpen", params)
}

// DidChange notifies gopls of file changes
func (c *GoplsClient) DidChange(ctx context.Context, params protocol.DidChangeTextDocumentParams) error {
	return c.conn.Notify(ctx, "textDocument/didChange", params)
}

// DidSave notifies gopls of file save
func (c *GoplsClient) DidSave(ctx context.Context, params protocol.DidSaveTextDocumentParams) error {
	return c.conn.Notify(ctx, "textDocument/didSave", params)
}

// DidClose notifies gopls of closed file
func (c *GoplsClient) DidClose(ctx context.Context, params protocol.DidCloseTextDocumentParams) error {
	return c.conn.Notify(ctx, "textDocument/didClose", params)
}

// NotifyFileChange notifies gopls that a .go file changed (for auto-transpile)
func (c *GoplsClient) NotifyFileChange(ctx context.Context, goPath string) error {
	fileURI := uri.File(goPath)
	fileEvent := protocol.FileEvent{
		URI:  fileURI,
		Type: protocol.FileChangeTypeChanged,
	}
	params := protocol.DidChangeWatchedFilesParams{
		Changes: []*protocol.FileEvent{&fileEvent},
	}
	return c.conn.Notify(ctx, "workspace/didChangeWatchedFiles", params)
}

// Shutdown gracefully shuts down gopls
func (c *GoplsClient) Shutdown(ctx context.Context) error {
	// CRITICAL FIX C2: Set shutdown flag to prevent crash recovery
	c.closeMu.Lock()
	c.shuttingDown = true
	c.closeMu.Unlock()

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return nil
	}

	// Send shutdown request
	if _, err := c.conn.Call(ctx, "shutdown", nil, nil); err != nil {
		c.logger.Warnf("gopls shutdown request failed: %v", err)
	}

	// Send exit notification
	if err := c.conn.Notify(ctx, "exit", nil); err != nil {
		c.logger.Warnf("gopls exit notification failed: %v", err)
	}

	// Close connection
	if err := c.conn.Close(); err != nil {
		c.logger.Debugf("gopls connection close error: %v", err)
	}

	// Wait for process to exit
	if c.cmd != nil && c.cmd.Process != nil {
		if err := c.cmd.Wait(); err != nil {
			c.logger.Debugf("gopls process wait error: %v", err)
		}
		c.logger.Infof("gopls stopped (PID: %d)", c.cmd.Process.Pid)
	}

	return nil
}

// handleCrash attempts to restart gopls after a crash
func (c *GoplsClient) handleCrash() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.restarts >= c.maxRestarts {
		return fmt.Errorf("gopls crashed %d times, giving up", c.restarts)
	}

	c.logger.Warnf("gopls crashed, restarting (attempt %d/%d)", c.restarts+1, c.maxRestarts)
	c.restarts++

	return c.start()
}

// readWriteCloser combines separate Read and Write closers into a single ReadWriteCloser
type readWriteCloser struct {
	stdin  io.WriteCloser
	stdout io.ReadCloser
}

func (rwc *readWriteCloser) Read(p []byte) (n int, err error) {
	return rwc.stdout.Read(p)
}

func (rwc *readWriteCloser) Write(p []byte) (n int, err error) {
	return rwc.stdin.Write(p)
}

func (rwc *readWriteCloser) Close() error {
	err1 := rwc.stdin.Close()
	err2 := rwc.stdout.Close()
	if err1 != nil {
		return err1
	}
	return err2
}

