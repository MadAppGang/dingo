package lsp

import (
	"context"
	"encoding/json"
	"fmt"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
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
	ideConn       jsonrpc2.Conn  // CRITICAL FIX C1: Store IDE connection for diagnostics
	ctx           context.Context // IMPORTANT FIX I3: Store server context
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

	// Initialize auto-transpiler
	transpiler := NewAutoTranspiler(cfg.Logger, mapCache, gopls)

	return &Server{
		config:     cfg,
		gopls:      gopls,
		mapCache:   mapCache,
		translator: translator,
		transpiler: transpiler,
	}, nil
}

// Serve starts the LSP server with the given connection
func (s *Server) Serve(ctx context.Context, conn jsonrpc2.Conn) error {
	// CRITICAL FIX C1 & IMPORTANT FIX I3: Store IDE connection and context
	s.ideConn = conn
	s.ctx = ctx

	_ = jsonrpc2.ReplyHandler(s.handleRequest)
	// Start serving (connection runs until closed)
	<-conn.Done()
	return conn.Err()
}

// handleRequest routes LSP requests to appropriate handlers
func (s *Server) handleRequest(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
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
	var params protocol.InitializeParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, fmt.Errorf("invalid initialize params: %w", err))
	}

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
	goplsResult, err := s.gopls.Initialize(ctx, params)
	if err != nil {
		return reply(ctx, nil, fmt.Errorf("gopls initialize failed: %w", err))
	}

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

	// If .dingo file, translate to .go file and forward
	if isDingoFile(params.TextDocument.URI) {
		goPath := dingoToGoPath(params.TextDocument.URI.Filename())
		params.TextDocument.URI = uri.File(goPath)
	}

	// Forward to gopls
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

	// If .dingo file, translate to .go file and forward
	if isDingoFile(params.TextDocument.URI) {
		goPath := dingoToGoPath(params.TextDocument.URI.Filename())
		params.TextDocument.URI = uri.File(goPath)
	}

	// Forward to gopls
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

		// Trigger transpilation (non-blocking, handled by transpiler)
		go s.transpiler.OnFileChange(ctx, dingoPath)
	}

	// Forward to gopls (with translated URI if .dingo file)
	if isDingoFile(params.TextDocument.URI) {
		goPath := dingoToGoPath(params.TextDocument.URI.Filename())
		params.TextDocument.URI = uri.File(goPath)
	}

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

	// If .dingo file, translate to .go file and forward
	if isDingoFile(params.TextDocument.URI) {
		goPath := dingoToGoPath(params.TextDocument.URI.Filename())
		params.TextDocument.URI = uri.File(goPath)
	}

	// Forward to gopls
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

// forwardToGopls forwards unknown requests directly to gopls
func (s *Server) forwardToGopls(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	// This is a simplified forwarding - full implementation would use gopls connection directly
	s.config.Logger.Debugf("Method %s not implemented, returning error", req.Method())
	return reply(ctx, nil, fmt.Errorf("method not implemented: %s", req.Method()))
}
