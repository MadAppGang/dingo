package ast

import (
	"go/scanner"
	gotoken "go/token"
)

// tokenInfo holds token information during transformation
type tokenInfo struct {
	pos gotoken.Pos
	tok gotoken.Token
	lit string
}

// TransformSource transforms Dingo source to valid Go source.
// It uses a token-based transformer to handle simple Dingo syntax.
//
// This is a legacy implementation that handles basic token-level transformations.
// Most features are now handled by the AST-based pipeline in pkg/ast/ast_transformer.go
//
// Currently handles:
// - Enums: enum Name { Variant } → Go interface pattern
// - Type annotations: param: Type → param Type
//
// NOTE: Generic syntax uses Go's native [T] syntax directly. No transformation needed.
//
// Complex features handled by AST pipeline (ast_transformer.go):
// - Error propagation: x? → expanded error handling
// - Pattern matching: match expr { ... } → type switch
// - Lambdas: |x| → func(x any) any
// - Null coalescing: a ?? b → (future)
// - Safe navigation: x?.field → (future)
//
// Note: Previously returned []SourceMapping for byte-offset tracking, but this has
// been removed. Position tracking now uses .dmap files for position mapping.
// //line directives are no longer emitted to keep generated Go code clean.
func TransformSource(src []byte, filename string) ([]byte, error) {
	// First pass: Transform enums (uses separate parser + codegen)
	// Pass filename to enable //line directive emission for proper position tracking
	src, enumRegistry := TransformEnumSource(src, filename)

	// Second pass: Transform enum constructor calls to NewVariant() pattern
	src = TransformEnumConstructors(src, enumRegistry)

	// Create a file set for tokenization
	fset := gotoken.NewFileSet()
	file := fset.AddFile("", -1, len(src))

	// Use Go's scanner to tokenize
	var s scanner.Scanner
	s.Init(file, src, nil, scanner.ScanComments)

	// Collect all tokens with their positions
	var tokens []tokenInfo

	for {
		pos, tok, lit := s.Scan()
		tokens = append(tokens, tokenInfo{pos, tok, lit})
		if tok == gotoken.EOF {
			break
		}
	}

	// Process tokens and build output
	result := make([]byte, 0, len(src))
	lastCopied := 0

	// State tracking
	parenDepth := 0
	paramListDepth := -1    // Depth at which we entered a param list (-1 = not in param list)
	lambdaParamsDepth := -1 // Depth at which we entered lambda params (-1 = not in lambda)

	for i := 0; i < len(tokens)-1; i++ {
		t := tokens[i]
		offset := file.Offset(t.pos)

		// Track parentheses for parameter context
		// IMPORTANT: We only transform colons in function DECLARATIONS, not function CALLS.
		// Function calls like wrap(func() { &rw{X: 1} }) should NOT have colons transformed.
		if t.tok == gotoken.LPAREN {
			parenDepth++
			if i > 0 && paramListDepth == -1 {
				prev := tokens[i-1]
				// FUNC( = anonymous function or function type: func(x: int)
				if prev.tok == gotoken.FUNC {
					paramListDepth = parenDepth
				} else if prev.tok == gotoken.IDENT && i >= 2 {
					// Check if this is a named function declaration: func name(x: int)
					// Look for 'func' immediately before the identifier
					prevPrev := tokens[i-2]
					if prevPrev.tok == gotoken.FUNC {
						paramListDepth = parenDepth
					} else if prevPrev.tok == gotoken.RBRACK {
						// Handle generic functions: func name[T any](x: T)
						// Find 'func' before the type params
						for j := i - 3; j >= 0; j-- {
							pt := tokens[j]
							if pt.tok == gotoken.FUNC {
								paramListDepth = parenDepth
								break
							}
							if pt.tok == gotoken.SEMICOLON || pt.tok == gotoken.LBRACE ||
								pt.tok == gotoken.RBRACE {
								break
							}
						}
					} else if prevPrev.tok == gotoken.RPAREN {
						// Method declaration: func (recv) name(x: int).
						// Walk back to find the matching `(` of the receiver
						// list and verify FUNC sits in front of it.
						depth := 1
						for j := i - 3; j >= 0; j-- {
							switch tokens[j].tok {
							case gotoken.RPAREN:
								depth++
							case gotoken.LPAREN:
								depth--
								if depth == 0 {
									if j > 0 && tokens[j-1].tok == gotoken.FUNC {
										paramListDepth = parenDepth
									}
									j = -1 // break outer
								}
							case gotoken.SEMICOLON, gotoken.LBRACE, gotoken.RBRACE:
								j = -1 // break outer
							}
							if j < 0 {
								break
							}
						}
					}
				}
			}
			// Check for TypeScript-style lambda: ( ... ) =>
			// Lookahead to find matching ) followed by => or ): Type =>
			if isLambdaParenStart(tokens, i) && lambdaParamsDepth == -1 {
				lambdaParamsDepth = parenDepth
			}
		}
		if t.tok == gotoken.RPAREN {
			// Exit param list context when we return to the depth we entered at
			if parenDepth == paramListDepth {
				paramListDepth = -1
			}
			if parenDepth == lambdaParamsDepth {
				lambdaParamsDepth = -1
			}
			parenDepth--
		}

		// NOTE: Generic syntax transformation (<T> -> [T]) has been REMOVED.
		// Dingo now uses Go's native generic syntax [T] directly.
		// Users should write Result[int, error], not Result[int, error].

		// Handle type annotations: param: Type -> param Type
		// Also handles lambda parameter annotations and return type annotations
		if t.tok == gotoken.COLON {
			// Case 1: Inside parameter list (func params, method receiver)
			// Case 2: Inside lambda parameter list (x: Type) => ...
			// Only transform if we're at exactly the param list depth (not in nested braces/calls)
			inParamList := paramListDepth != -1 && parenDepth == paramListDepth
			inLambdaParams := lambdaParamsDepth != -1 && parenDepth == lambdaParamsDepth
			if (inParamList || inLambdaParams) && i > 0 && tokens[i-1].tok == gotoken.IDENT {
				result = append(result, src[lastCopied:offset]...)
				result = append(result, ' ')
				lastCopied = offset + 1
				continue
			}
			// Case 3: Lambda return type - ): Type => pattern
			// We just saw ), now we see :, and we expect IDENT then =>
			if i > 0 && tokens[i-1].tok == gotoken.RPAREN {
				if isLambdaReturnType(tokens, i) {
					// Remove the colon, lambda codegen will handle it properly
					result = append(result, src[lastCopied:offset]...)
					// Don't output colon - the lambda parser expects (x Type) string => not (x Type): string =>
					// Actually, we need to skip the colon AND the type, let lambda parser handle it
					// For now, just skip the colon - the issue is go/parser doesn't understand this syntax
					// Solution: We need to transform the ENTIRE lambda BEFORE this stage
					lastCopied = offset + 1 // Skip the colon
					continue
				}
			}
		}

	}

	// Copy remaining bytes
	if lastCopied < len(src) {
		result = append(result, src[lastCopied:]...)
	}

	return result, nil
}

// isIdentifier checks if a string is a valid Go identifier.
func isIdentifier(s string) bool {
	if len(s) == 0 {
		return false
	}
	for i, ch := range s {
		if i == 0 {
			if !isLetter(ch) && ch != '_' {
				return false
			}
		} else {
			if !isLetter(ch) && !isDigit(ch) && ch != '_' {
				return false
			}
		}
	}
	return true
}

func isLetter(ch rune) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')
}

func isDigit(ch rune) bool {
	return ch >= '0' && ch <= '9'
}

// isArrowToken checks if tokens[i] and tokens[i+1] form => (ASSIGN followed by GTR)
func isArrowToken(tokens []tokenInfo, i int) bool {
	if i+1 >= len(tokens) {
		return false
	}
	return tokens[i].tok == gotoken.ASSIGN && tokens[i+1].tok == gotoken.GTR
}

// isLambdaParenStart checks if this ( starts a TypeScript-style lambda
// by looking ahead for pattern like (params) => or (params): Type =>
// Note: Go scanner tokenizes => as two tokens: = (ASSIGN) and > (GTR)
func isLambdaParenStart(tokens []tokenInfo, start int) bool {
	// Look for matching ) then check for => or : Type =>
	depth := 0
	for i := start; i < len(tokens); i++ {
		switch tokens[i].tok {
		case gotoken.LPAREN:
			depth++
		case gotoken.RPAREN:
			depth--
			if depth == 0 {
				// Found matching ), check what follows
				if i+2 < len(tokens) {
					// Check for => (tokenized as = >)
					if isArrowToken(tokens, i+1) {
						return true
					}
					// Check for : Type => pattern
					if tokens[i+1].tok == gotoken.COLON {
						// : should be followed by IDENT (type) then = >
						if i+3 < len(tokens) && tokens[i+2].tok == gotoken.IDENT {
							if isArrowToken(tokens, i+3) {
								return true
							}
						}
					}
				}
				return false
			}
		case gotoken.SEMICOLON, gotoken.LBRACE, gotoken.RBRACE:
			// Hit statement boundary, not a lambda
			return false
		}
	}
	return false
}

// isLambdaReturnType checks if this : at position i is a lambda return type annotation
// Pattern: ): Type => (where => is tokenized as = >)
func isLambdaReturnType(tokens []tokenInfo, i int) bool {
	// Current token is :, previous is )
	// Check for IDENT (type name) followed by = >
	if i+3 < len(tokens) && tokens[i+1].tok == gotoken.IDENT {
		if isArrowToken(tokens, i+2) {
			return true
		}
	}
	return false
}
