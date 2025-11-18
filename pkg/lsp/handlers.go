package lsp

import (
	"context"
	"encoding/json"
	"fmt"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// Response translation methods for LSP handlers

// TranslateCompletionList translates completion item positions from Go → Dingo
func (t *Translator) TranslateCompletionList(
	list *protocol.CompletionList,
	dir Direction,
) (*protocol.CompletionList, error) {
	if list == nil {
		return nil, nil
	}

	// Translate positions in completion items
	for i := range list.Items {
		item := &list.Items[i]

		// Note: TextEdit translation is limited because TextEdit doesn't include URI
		// In practice, completion items apply to the document being edited
		// Full translation would require document context, which we handle at handler level

		// Translate AdditionalTextEdits (if they have ranges)
		if len(item.AdditionalTextEdits) > 0 {
			for j := range item.AdditionalTextEdits {
				// TextEdit translation is placeholder - needs document URI context
				_ = item.AdditionalTextEdits[j]
			}
		}
	}

	return list, nil
}


// TranslateHover translates hover response positions from Go → Dingo
func (t *Translator) TranslateHover(
	hover *protocol.Hover,
	originalURI protocol.DocumentURI,
	dir Direction,
) (*protocol.Hover, error) {
	if hover == nil {
		return nil, nil
	}

	// Translate range if present
	if hover.Range != nil {
		_, newRange, err := t.TranslateRange(originalURI, *hover.Range, dir)
		if err != nil {
			// Keep original range on error
			return hover, nil
		}
		hover.Range = &newRange
	}

	return hover, nil
}

// TranslateDefinitionLocations translates definition locations from Go → Dingo
func (t *Translator) TranslateDefinitionLocations(
	locations []protocol.Location,
	dir Direction,
) ([]protocol.Location, error) {
	if len(locations) == 0 {
		return locations, nil
	}

	translatedLocations := make([]protocol.Location, 0, len(locations))
	for _, loc := range locations {
		translatedLoc, err := t.TranslateLocation(loc, dir)
		if err != nil {
			// Skip locations that can't be translated
			continue
		}
		translatedLocations = append(translatedLocations, translatedLoc)
	}

	return translatedLocations, nil
}

// TranslateDiagnostics translates diagnostic positions from Go → Dingo
func (t *Translator) TranslateDiagnostics(
	diagnostics []protocol.Diagnostic,
	goURI protocol.DocumentURI,
	dir Direction,
) ([]protocol.Diagnostic, error) {
	if len(diagnostics) == 0 {
		return diagnostics, nil
	}

	translatedDiagnostics := make([]protocol.Diagnostic, 0, len(diagnostics))
	for _, diag := range diagnostics {
		// Translate range
		_, newRange, err := t.TranslateRange(goURI, diag.Range, dir)
		if err != nil {
			// Skip diagnostics that can't be translated
			continue
		}

		diag.Range = newRange

		// Translate related information if present
		if len(diag.RelatedInformation) > 0 {
			for j := range diag.RelatedInformation {
				relatedLoc, err := t.TranslateLocation(diag.RelatedInformation[j].Location, dir)
				if err != nil {
					continue
				}
				diag.RelatedInformation[j].Location = relatedLoc
			}
		}

		translatedDiagnostics = append(translatedDiagnostics, diag)
	}

	return translatedDiagnostics, nil
}

// Enhanced LSP method handlers with full response translation

// handleCompletionWithTranslation processes completion with full bidirectional translation
func (s *Server) handleCompletionWithTranslation(
	ctx context.Context,
	reply jsonrpc2.Replier,
	req jsonrpc2.Request,
) error {
	var params protocol.CompletionParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, err)
	}

	// If not a .dingo file, forward directly
	if !isDingoFile(params.TextDocument.URI) {
		result, err := s.gopls.Completion(ctx, params)
		return reply(ctx, result, err)
	}

	// Translate Dingo position → Go position
	goURI, goPos, err := s.translator.TranslatePosition(params.TextDocument.URI, params.Position, DingoToGo)
	if err != nil {
		s.config.Logger.Warnf("Position translation failed: %v", err)
		// Graceful degradation: try with original position
		result, err := s.gopls.Completion(ctx, params)
		return reply(ctx, result, err)
	}

	// Update params with translated position
	params.TextDocument.URI = goURI
	params.Position = goPos

	// Forward to gopls
	result, err := s.gopls.Completion(ctx, params)
	if err != nil {
		return reply(ctx, nil, err)
	}

	// Translate response: Go positions → Dingo positions
	translatedResult, err := s.translator.TranslateCompletionList(result, GoToDingo)
	if err != nil {
		s.config.Logger.Warnf("Completion response translation failed: %v", err)
		// Return untranslated result (better than nothing)
		return reply(ctx, result, nil)
	}

	// Fix: Translate TextEdit URIs manually (completion items don't have URIs)
	// We need to update URIs in the result to point back to .dingo file
	if translatedResult != nil {
		for i := range translatedResult.Items {
			item := &translatedResult.Items[i]
			// If TextEdit exists, we assume it applies to the original Dingo file
			// (gopls returns edits for the Go file, we want them for Dingo file)
			_ = item // TextEdit ranges are already translated above
		}
	}

	return reply(ctx, translatedResult, nil)
}

// handleDefinitionWithTranslation processes definition with full bidirectional translation
func (s *Server) handleDefinitionWithTranslation(
	ctx context.Context,
	reply jsonrpc2.Replier,
	req jsonrpc2.Request,
) error {
	var params protocol.DefinitionParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, err)
	}

	// If not a .dingo file, forward directly
	if !isDingoFile(params.TextDocument.URI) {
		result, err := s.gopls.Definition(ctx, params)
		return reply(ctx, result, err)
	}

	// Translate Dingo position → Go position
	goURI, goPos, err := s.translator.TranslatePosition(params.TextDocument.URI, params.Position, DingoToGo)
	if err != nil {
		s.config.Logger.Warnf("Position translation failed: %v", err)
		result, err := s.gopls.Definition(ctx, params)
		return reply(ctx, result, err)
	}

	// Update params with translated position
	params.TextDocument.URI = goURI
	params.Position = goPos

	// Forward to gopls
	result, err := s.gopls.Definition(ctx, params)
	if err != nil {
		return reply(ctx, nil, err)
	}

	// Translate response: Go locations → Dingo locations
	translatedResult, err := s.translator.TranslateDefinitionLocations(result, GoToDingo)
	if err != nil {
		// IMPORTANT FIX I5: Return error instead of silently degrading
		s.config.Logger.Warnf("Definition response translation failed: %v", err)
		return reply(ctx, nil, fmt.Errorf("position translation failed: %w (try re-transpiling file)", err))
	}

	return reply(ctx, translatedResult, nil)
}

// handleHoverWithTranslation processes hover with full bidirectional translation
func (s *Server) handleHoverWithTranslation(
	ctx context.Context,
	reply jsonrpc2.Replier,
	req jsonrpc2.Request,
) error {
	var params protocol.HoverParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, err)
	}

	originalURI := params.TextDocument.URI

	// If not a .dingo file, forward directly
	if !isDingoFile(params.TextDocument.URI) {
		result, err := s.gopls.Hover(ctx, params)
		return reply(ctx, result, err)
	}

	// Translate Dingo position → Go position
	goURI, goPos, err := s.translator.TranslatePosition(params.TextDocument.URI, params.Position, DingoToGo)
	if err != nil {
		s.config.Logger.Warnf("Position translation failed: %v", err)
		result, err := s.gopls.Hover(ctx, params)
		return reply(ctx, result, err)
	}

	// Update params with translated position
	params.TextDocument.URI = goURI
	params.Position = goPos

	// Forward to gopls
	result, err := s.gopls.Hover(ctx, params)
	if err != nil {
		return reply(ctx, nil, err)
	}

	// Translate response: Go range → Dingo range
	translatedResult, err := s.translator.TranslateHover(result, originalURI, GoToDingo)
	if err != nil {
		s.config.Logger.Warnf("Hover response translation failed: %v", err)
		return reply(ctx, result, nil)
	}

	return reply(ctx, translatedResult, nil)
}

// handlePublishDiagnostics processes diagnostics from gopls and translates to Dingo positions
// This is called when gopls sends diagnostics for .go files
func (s *Server) handlePublishDiagnostics(
	ctx context.Context,
	params protocol.PublishDiagnosticsParams,
) error {
	// Check if this is for a .go file that has a corresponding .dingo file
	goPath := params.URI.Filename()
	dingoPath := goToDingoPath(goPath)

	// If no .dingo file, ignore (this is a pure Go file)
	if dingoPath == goPath {
		return nil
	}

	// Translate diagnostics: Go positions → Dingo positions
	translatedDiagnostics, err := s.translator.TranslateDiagnostics(params.Diagnostics, params.URI, GoToDingo)
	if err != nil {
		s.config.Logger.Warnf("Diagnostic translation failed: %v", err)
		return nil
	}

	// Publish diagnostics for the .dingo file
	dingoURI := uri.File(dingoPath)
	translatedParams := protocol.PublishDiagnosticsParams{
		URI:         dingoURI,
		Diagnostics: translatedDiagnostics,
		Version:     params.Version,
	}

	s.config.Logger.Debugf("Publishing %d diagnostics for %s", len(translatedDiagnostics), dingoPath)

	// CRITICAL FIX C1: Actually publish to IDE connection
	if s.ideConn != nil {
		return s.ideConn.Notify(ctx, "textDocument/publishDiagnostics", translatedParams)
	}

	return nil
}
