package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	lspuri "go.lsp.dev/uri"
)

// ServerConfig holds configuration for the LSP server
type ServerConfig struct {
	Logger        Logger
	GoplsPath     string
	AutoTranspile bool
}

// Server implements the LSP proxy server
type Server struct {
	config        ServerConfig
	gopls         *GoplsClient
	mapCache      *SourceMapCache
	translator    *Translator
	transpiler    *AutoTranspiler
	watcher       *FileWatcher
	workspacePath string
	initialized   bool

	// CRITICAL FIX (Qwen): Protect connection and context with mutex
	connMu  sync.RWMutex
	ideConn jsonrpc2.Conn   // Store IDE connection for diagnostics
	ctx     context.Context // Store server context
}

// NewServer creates a new LSP server instance
func NewServer(cfg ServerConfig) (*Server, error) {
	// Initialize gopls client
	gopls, err := NewGoplsClient(cfg.GoplsPath, cfg.Logger)
	if err != nil {
		return nil, fmt.Errorf("failed to start gopls: %w", err)
	}

	// Initialize source map cache
	mapCache, err := NewSourceMapCache(cfg.Logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create source map cache: %w", err)
	}

	// Initialize translator
	translator := NewTranslator(mapCache)

	// Create server first (without transpiler)
	server := &Server{
		config:     cfg,
		gopls:      gopls,
		mapCache:   mapCache,
		translator: translator,
	}

	// Initialize auto-transpiler with server reference
	transpiler := NewAutoTranspiler(cfg.Logger, mapCache, gopls, server)
	server.transpiler = transpiler

	// Set diagnostics handler for gopls -> IDE diagnostics forwarding
	gopls.SetDiagnosticsHandler(server.handlePublishDiagnostics)

	return server, nil
}

// SetConn stores the connection and context in the server (thread-safe)
func (s *Server) SetConn(conn jsonrpc2.Conn, ctx context.Context) {
	s.connMu.Lock()
	defer s.connMu.Unlock()
	s.ideConn = conn
	s.ctx = ctx
}

// GetConn returns the IDE connection (thread-safe)
func (s *Server) GetConn() (jsonrpc2.Conn, context.Context) {
	s.connMu.RLock()
	defer s.connMu.RUnlock()
	return s.ideConn, s.ctx
}

// Handler returns a jsonrpc2 handler for this server
func (s *Server) Handler() jsonrpc2.Handler {
	return jsonrpc2.ReplyHandler(s.handleRequest)
}

// handleRequest routes LSP requests to appropriate handlers
func (s *Server) handleRequest(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	s.config.Logger.Debugf("Received request: %s", req.Method())

	switch req.Method() {
	case "initialize":
		return s.handleInitialize(ctx, reply, req)
	case "initialized":
		return s.handleInitialized(ctx, reply, req)
	case "shutdown":
		return s.handleShutdown(ctx, reply, req)
	case "exit":
		return s.handleExit(ctx, reply, req)
	case "textDocument/didOpen":
		return s.handleDidOpen(ctx, reply, req)
	case "textDocument/didChange":
		return s.handleDidChange(ctx, reply, req)
	case "textDocument/didSave":
		return s.handleDidSave(ctx, reply, req)
	case "textDocument/didClose":
		return s.handleDidClose(ctx, reply, req)
	case "textDocument/completion":
		return s.handleCompletion(ctx, reply, req)
	case "textDocument/definition":
		return s.handleDefinition(ctx, reply, req)
	case "textDocument/hover":
		return s.handleHover(ctx, reply, req)
	default:
		// Unknown method - try forwarding to gopls
		s.config.Logger.Debugf("Forwarding unknown method to gopls: %s", req.Method())
		return s.forwardToGopls(ctx, reply, req)
	}
}

// handleInitialize processes the initialize request
func (s *Server) handleInitialize(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	s.config.Logger.Debugf("handleInitialize: Starting")

	var params protocol.InitializeParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		s.config.Logger.Errorf("handleInitialize: Failed to unmarshal params: %v", err)
		return reply(ctx, nil, fmt.Errorf("invalid initialize params: %w", err))
	}
	s.config.Logger.Debugf("handleInitialize: Params unmarshaled")

	// Extract workspace path
	if params.RootURI != "" {
		s.workspacePath = params.RootURI.Filename()
		s.config.Logger.Infof("Workspace path: %s", s.workspacePath)

		// Start file watcher if auto-transpile enabled
		if s.config.AutoTranspile {
			watcher, err := NewFileWatcher(s.workspacePath, s.config.Logger, s.handleDingoFileChange)
			if err != nil {
				s.config.Logger.Warnf("Failed to start file watcher: %v (auto-transpile disabled)", err)
			} else {
				s.watcher = watcher
			}
		}
	}

	// Forward initialize to gopls
	s.config.Logger.Debugf("handleInitialize: Forwarding to gopls")
	goplsResult, err := s.gopls.Initialize(ctx, params)
	if err != nil {
		s.config.Logger.Errorf("handleInitialize: gopls failed: %v", err)
		return reply(ctx, nil, fmt.Errorf("gopls initialize failed: %w", err))
	}
	s.config.Logger.Debugf("handleInitialize: gopls responded")

	// Return modified capabilities (Dingo-specific)
	result := protocol.InitializeResult{
		Capabilities: protocol.ServerCapabilities{
			TextDocumentSync: protocol.TextDocumentSyncOptions{
				OpenClose: true,
				Change:    protocol.TextDocumentSyncKindFull,
				Save: &protocol.SaveOptions{
					IncludeText: false,
				},
			},
			CompletionProvider: &protocol.CompletionOptions{
				TriggerCharacters: []string{".", ":", " "},
			},
			HoverProvider:      goplsResult.Capabilities.HoverProvider,
			DefinitionProvider: goplsResult.Capabilities.DefinitionProvider,
		},
		ServerInfo: &protocol.ServerInfo{
			Name:    "dingo-lsp",
			Version: "0.1.0",
		},
	}

	s.initialized = true
	s.config.Logger.Debugf("Sending initialize response to client")
	s.config.Logger.Infof("Server initialized (auto-transpile: %v)", s.config.AutoTranspile)

	return reply(ctx, result, nil)
}

// handleInitialized processes the initialized notification
func (s *Server) handleInitialized(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.InitializedParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, fmt.Errorf("invalid initialized params: %w", err))
	}

	// Forward to gopls
	if err := s.gopls.Initialized(ctx, &params); err != nil {
		s.config.Logger.Warnf("gopls initialized notification failed: %v", err)
	}

	return reply(ctx, nil, nil)
}

// handleShutdown processes the shutdown request
func (s *Server) handleShutdown(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	s.config.Logger.Infof("Shutdown requested")

	// Stop file watcher
	if s.watcher != nil {
		if err := s.watcher.Close(); err != nil {
			s.config.Logger.Warnf("File watcher close failed: %v", err)
		}
	}

	// Shutdown gopls
	if err := s.gopls.Shutdown(ctx); err != nil {
		s.config.Logger.Warnf("gopls shutdown failed: %v", err)
	}

	s.initialized = false
	return reply(ctx, nil, nil)
}

// handleExit processes the exit notification
func (s *Server) handleExit(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	s.config.Logger.Infof("Exit requested")
	return reply(ctx, nil, nil)
}

// handleDidOpen processes didOpen notifications
func (s *Server) handleDidOpen(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.DidOpenTextDocumentParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, err)
	}

	// CRITICAL FIX D1: When .dingo file opens, ensure .go file exists and open with gopls
	// This is necessary for gopls to analyze the file and send diagnostics
	if isDingoFile(params.TextDocument.URI) {
		dingoPath := params.TextDocument.URI.Filename()
		s.config.Logger.Infof("[didOpen] Opened .dingo file: %s", dingoPath)

		// Check if .go file exists, if not auto-transpile
		goPath := dingoToGoPath(dingoPath)
		if _, err := os.Stat(goPath); os.IsNotExist(err) {
			s.config.Logger.Infof("[didOpen] .go file missing, auto-transpiling: %s", dingoPath)

			// Auto-transpile to generate .go file
			if s.transpiler != nil {
				s.transpiler.OnFileChange(ctx, dingoPath)
				s.config.Logger.Infof("[didOpen] Auto-transpile completed (check for errors above)")
			} else {
				s.config.Logger.Warnf("[didOpen] Transpiler not available!")
			}
		} else {
			s.config.Logger.Infof("[didOpen] .go file already exists: %s", goPath)
		}

		// Open corresponding .go file with gopls (if transpilation succeeded)
		if err := s.openGoFileWithGopls(ctx, dingoPath); err != nil {
			s.config.Logger.Warnf("[didOpen] Failed to open .go file with gopls: %v", err)
		} else {
			s.config.Logger.Infof("[didOpen] Successfully opened .go file with gopls")
		}

		return reply(ctx, nil, nil)
	}

	// Forward non-dingo files to gopls
	if err := s.gopls.DidOpen(ctx, params); err != nil {
		s.config.Logger.Warnf("gopls didOpen failed: %v", err)
	}

	return reply(ctx, nil, nil)
}

// handleDidChange processes didChange notifications
func (s *Server) handleDidChange(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.DidChangeTextDocumentParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, err)
	}

	// IMPORTANT: Don't forward .dingo file changes to gopls
	// gopls reads .go files from disk (updated by auto-transpiler on save)
	// We translate positions during queries instead
	if isDingoFile(params.TextDocument.URI) {
		s.config.Logger.Debugf("Changed .dingo file (not forwarding to gopls): %s", params.TextDocument.URI)
		return reply(ctx, nil, nil)
	}

	// Forward non-dingo files to gopls
	if err := s.gopls.DidChange(ctx, params); err != nil {
		s.config.Logger.Warnf("gopls didChange failed: %v", err)
	}

	return reply(ctx, nil, nil)
}

// handleDidSave processes didSave notifications
func (s *Server) handleDidSave(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.DidSaveTextDocumentParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, err)
	}

	// Auto-transpile if enabled and this is a .dingo file
	if s.config.AutoTranspile && isDingoFile(params.TextDocument.URI) {
		dingoPath := params.TextDocument.URI.Filename()
		s.config.Logger.Debugf("Auto-transpile on save: %s", dingoPath)

		// Trigger transpilation (AutoTranspiler will notify gopls after completion)
		go s.transpiler.OnFileChange(ctx, dingoPath)

		// Don't forward to gopls - transpiler handles it after successful transpilation
		return reply(ctx, nil, nil)
	}

	// Forward non-dingo files to gopls
	if err := s.gopls.DidSave(ctx, params); err != nil {
		s.config.Logger.Warnf("gopls didSave failed: %v", err)
	}

	return reply(ctx, nil, nil)
}

// handleDidClose processes didClose notifications
func (s *Server) handleDidClose(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.DidCloseTextDocumentParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, err)
	}

	// CRITICAL FIX D1: When .dingo file closes, close corresponding .go file with gopls
	if isDingoFile(params.TextDocument.URI) {
		s.config.Logger.Debugf("Closed .dingo file: %s", params.TextDocument.URI)

		// Close corresponding .go file with gopls
		if err := s.closeGoFileWithGopls(ctx, params.TextDocument.URI.Filename()); err != nil {
			s.config.Logger.Warnf("Failed to close .go file with gopls: %v", err)
		}

		return reply(ctx, nil, nil)
	}

	// Forward non-dingo files to gopls
	if err := s.gopls.DidClose(ctx, params); err != nil {
		s.config.Logger.Warnf("gopls didClose failed: %v", err)
	}

	return reply(ctx, nil, nil)
}

// handleCompletion processes completion requests with position translation
func (s *Server) handleCompletion(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	// Use enhanced handler with full response translation
	return s.handleCompletionWithTranslation(ctx, reply, req)
}

// handleDefinition processes definition requests with position translation
func (s *Server) handleDefinition(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	// Use enhanced handler with full response translation
	return s.handleDefinitionWithTranslation(ctx, reply, req)
}

// handleHover processes hover requests with position translation
func (s *Server) handleHover(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	// Use enhanced handler with full response translation
	return s.handleHoverWithTranslation(ctx, reply, req)
}

// handleDingoFileChange handles file changes detected by the watcher
func (s *Server) handleDingoFileChange(dingoPath string) {
	// IMPORTANT FIX I3: Use server context instead of background
	s.transpiler.OnFileChange(s.ctx, dingoPath)
}

// openGoFileWithGopls opens the corresponding .go file with gopls
// CRITICAL FIX D1: This enables gopls to analyze the file and send diagnostics
func (s *Server) openGoFileWithGopls(ctx context.Context, dingoPath string) error {
	// Convert .dingo path to .go path
	goPath := dingoToGoPath(dingoPath)

	s.config.Logger.Debugf("[Diagnostic Fix] Opening .go file with gopls: %s", goPath)

	// Read .go file contents
	contents, err := os.ReadFile(goPath)
	if err != nil {
		return fmt.Errorf("failed to read .go file: %w", err)
	}

	// Create didOpen params for gopls
	params := protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI:        protocol.DocumentURI(lspuri.File(goPath)),
			LanguageID: "go",
			Version:    1,
			Text:       string(contents),
		},
	}

	// Open with gopls
	if err := s.gopls.DidOpen(ctx, params); err != nil {
		return fmt.Errorf("gopls didOpen failed: %w", err)
	}

	s.config.Logger.Debugf("[Diagnostic Fix] Successfully opened .go file with gopls: %s", goPath)
	return nil
}

// closeGoFileWithGopls closes the corresponding .go file with gopls
// CRITICAL FIX D1: Clean up when .dingo file is closed
func (s *Server) closeGoFileWithGopls(ctx context.Context, dingoPath string) error {
	// Convert .dingo path to .go path
	goPath := dingoToGoPath(dingoPath)

	s.config.Logger.Debugf("[Diagnostic Fix] Closing .go file with gopls: %s", goPath)

	// Create didClose params for gopls
	params := protocol.DidCloseTextDocumentParams{
		TextDocument: protocol.TextDocumentIdentifier{
			URI: protocol.DocumentURI(lspuri.File(goPath)),
		},
	}

	// Close with gopls
	if err := s.gopls.DidClose(ctx, params); err != nil {
		return fmt.Errorf("gopls didClose failed: %w", err)
	}

	s.config.Logger.Debugf("[Diagnostic Fix] Successfully closed .go file with gopls: %s", goPath)
	return nil
}

// publishDingoDiagnostics publishes Dingo-specific diagnostics (e.g., transpilation errors)
// This is separate from gopls diagnostics (which are translated and forwarded)
func (s *Server) publishDingoDiagnostics(uri protocol.DocumentURI, diagnostics []protocol.Diagnostic) {
	// Get IDE connection (thread-safe)
	ideConn, serverCtx := s.GetConn()
	if ideConn == nil {
		s.config.Logger.Warnf("[Dingo Diagnostics] No IDE connection available, cannot publish diagnostics")
		return
	}

	// Prepare params
	params := protocol.PublishDiagnosticsParams{
		URI:         uri,
		Diagnostics: diagnostics,
	}

	// Use server context if available, otherwise background
	publishCtx := serverCtx
	if publishCtx == nil {
		publishCtx = context.Background()
	}

	// Publish to IDE
	err := ideConn.Notify(publishCtx, "textDocument/publishDiagnostics", params)
	if err != nil {
		s.config.Logger.Errorf("[Dingo Diagnostics] Failed to publish diagnostics: %v", err)
		return
	}

	if len(diagnostics) > 0 {
		s.config.Logger.Debugf("[Dingo Diagnostics] Published %d Dingo-specific diagnostic(s) for %s", len(diagnostics), uri)
	} else {
		s.config.Logger.Debugf("[Dingo Diagnostics] Cleared diagnostics for %s", uri)
	}
}

// forwardToGopls forwards unknown requests directly to gopls
func (s *Server) forwardToGopls(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	// This is a simplified forwarding - full implementation would use gopls connection directly
	s.config.Logger.Debugf("Method %s not implemented, returning error", req.Method())
	return reply(ctx, nil, fmt.Errorf("method not implemented: %s", req.Method()))
}
