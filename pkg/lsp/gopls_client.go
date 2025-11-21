package lsp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// DiagnosticsHandler is called when gopls sends diagnostics
type DiagnosticsHandler func(ctx context.Context, params protocol.PublishDiagnosticsParams) error

// GoplsClient manages a gopls subprocess and forwards LSP requests
type GoplsClient struct {
	cmd                 *exec.Cmd
	conn                jsonrpc2.Conn
	logger              Logger
	goplsPath           string
	restarts            int
	maxRestarts         int
	mu                  sync.Mutex
	shuttingDown        bool           // CRITICAL FIX C2: Track shutdown state
	closeMu             sync.Mutex     // CRITICAL FIX C2: Protect shutdown flag
	diagnosticsHandler  DiagnosticsHandler // Callback for diagnostics
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

// SetDiagnosticsHandler sets the callback for handling diagnostics from gopls
func (c *GoplsClient) SetDiagnosticsHandler(handler DiagnosticsHandler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.diagnosticsHandler = handler
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

	// Create JSON-RPC connection using a buffered ReadWriteCloser wrapper (GPT-5 fix)
	rwc := newReadWriteCloser(stdin, stdout)
	stream := jsonrpc2.NewStream(rwc)
	c.conn = jsonrpc2.NewConn(stream)

	// Start handler to process gopls responses and notifications
	ctx := context.Background()
	handler := jsonrpc2.ReplyHandler(func(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
		// Log gopls -> dingo-lsp notifications/requests (if any)
		c.logger.Debugf("gopls notification/request: %s", req.Method())

		// Handle server->client requests from gopls
		switch req.Method() {
		case "client/registerCapability", "client/unregisterCapability":
			// Accept capability registration (we don't need to track them)
			return reply(ctx, nil, nil)
		case "window/showMessage", "window/logMessage":
			// Log messages from gopls
			var params map[string]interface{}
			if err := json.Unmarshal(req.Params(), &params); err == nil {
				c.logger.Debugf("gopls %s: %v", req.Method(), params)
			}
			return reply(ctx, nil, nil)
		case "textDocument/publishDiagnostics":
			// Forward diagnostics to handler (for translation to .dingo positions)
			c.logger.Debugf("[gopls → dingo-lsp] Received publishDiagnostics notification")

			var params protocol.PublishDiagnosticsParams
			if err := json.Unmarshal(req.Params(), &params); err != nil {
				c.logger.Warnf("[gopls → dingo-lsp] Failed to unmarshal diagnostics: %v", err)
				return reply(ctx, nil, nil)
			}

			c.logger.Debugf("[gopls → dingo-lsp] Diagnostics for URI=%s, count=%d",
				params.URI.Filename(), len(params.Diagnostics))

			// Log each diagnostic from gopls
			for i, diag := range params.Diagnostics {
				c.logger.Debugf("[gopls → dingo-lsp]   [%d] Severity=%d, Message=%q, Range=L%d:C%d-L%d:C%d",
					i, diag.Severity, diag.Message,
					diag.Range.Start.Line, diag.Range.Start.Character,
					diag.Range.End.Line, diag.Range.End.Character)
			}

			// Call diagnostics handler if set
			c.mu.Lock()
			handler := c.diagnosticsHandler
			c.mu.Unlock()

			if handler != nil {
				c.logger.Debugf("[gopls → dingo-lsp] Calling diagnostics handler")
				if err := handler(ctx, params); err != nil {
					c.logger.Warnf("[gopls → dingo-lsp] Diagnostics handler error: %v", err)
				} else {
					c.logger.Debugf("[gopls → dingo-lsp] Diagnostics handler completed successfully")
				}
			} else {
				c.logger.Debugf("[gopls → dingo-lsp] No diagnostics handler set, discarding %d diagnostics for %s",
					len(params.Diagnostics), params.URI)
			}

			return reply(ctx, nil, nil)
		default:
			// Unknown method - reply with empty result
			c.logger.Debugf("gopls unknown method: %s", req.Method())
			return reply(ctx, nil, nil)
		}
	})
	c.conn.Go(ctx, handler)

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

// Initialize sends initialize request to gopls with timeout (GPT-5 fix)
func (c *GoplsClient) Initialize(ctx context.Context, params protocol.InitializeParams) (*protocol.InitializeResult, error) {
	// Check if gopls process is still alive
	c.mu.Lock()
	if c.cmd == nil || c.cmd.Process == nil {
		c.mu.Unlock()
		return nil, fmt.Errorf("gopls process not running")
	}
	c.mu.Unlock()

	// Add timeout to prevent hanging forever
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	c.logger.Debugf("Calling gopls initialize")
	var result protocol.InitializeResult
	_, err := c.conn.Call(ctx, "initialize", params, &result)
	if err != nil {
		c.logger.Errorf("gopls initialize call failed: %v", err)
		return nil, fmt.Errorf("gopls initialize failed: %w", err)
	}
	c.logger.Debugf("gopls initialize succeeded")
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

// SyncFileContent synchronizes gopls with the current content of a .go file
// This is critical for keeping gopls in sync after transpilation
func (c *GoplsClient) SyncFileContent(ctx context.Context, goPath string) error {
	c.logger.Debugf("Reading .go file for sync: %s", goPath)

	// Read current .go file content from disk
	content, err := os.ReadFile(goPath)
	if err != nil {
		return fmt.Errorf("failed to read .go file: %w", err)
	}

	// Send didChange notification with full new content
	fileURI := protocol.DocumentURI(uri.File(goPath))
	params := protocol.DidChangeTextDocumentParams{
		TextDocument: protocol.VersionedTextDocumentIdentifier{
			TextDocumentIdentifier: protocol.TextDocumentIdentifier{
				URI: fileURI,
			},
			Version: int32(time.Now().Unix()), // Use timestamp as version
		},
		ContentChanges: []protocol.TextDocumentContentChangeEvent{
			{
				// Full document update (no range specified)
				Text: string(content),
			},
		},
	}

	c.logger.Debugf("Sending didChange to gopls with %d bytes for %s", len(content), goPath)
	if err := c.conn.Notify(ctx, "textDocument/didChange", params); err != nil {
		return fmt.Errorf("failed to send didChange to gopls: %w", err)
	}

	c.logger.Debugf("gopls synchronized with .go file: %s", goPath)
	return nil
}

// NotifyFileChange notifies gopls that a .go file changed (for auto-transpile)
// DEPRECATED: Use SyncFileContent instead for reliable synchronization
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

// readWriteCloser combines separate Read and Write closers with buffering (GPT-5 fix)
type readWriteCloser struct {
	stdin   io.WriteCloser
	stdout  io.ReadCloser
	reader  *bufio.Reader
	writer  *bufio.Writer
}

func newReadWriteCloser(stdin io.WriteCloser, stdout io.ReadCloser) *readWriteCloser {
	return &readWriteCloser{
		stdin:  stdin,
		stdout: stdout,
		reader: bufio.NewReaderSize(stdout, 32*1024), // 32KB buffer
		writer: bufio.NewWriterSize(stdin, 32*1024),  // 32KB buffer
	}
}

func (rwc *readWriteCloser) Read(p []byte) (n int, err error) {
	return rwc.reader.Read(p)
}

func (rwc *readWriteCloser) Write(p []byte) (n int, err error) {
	n, err = rwc.writer.Write(p)
	if err != nil {
		return n, err
	}
	// Flush after each write to ensure messages are sent immediately
	return n, rwc.writer.Flush()
}

func (rwc *readWriteCloser) Close() error {
	// Flush any remaining data
	_ = rwc.writer.Flush()
	err1 := rwc.stdin.Close()
	err2 := rwc.stdout.Close()
	if err1 != nil {
		return err1
	}
	return err2
}

