package transpiler

import (
	"bytes"
	"fmt"
	goast "go/ast"
	"go/token"
	gotoken "go/token"
	"sort"

	"github.com/MadAppGang/dingo/pkg/ast"
	"github.com/MadAppGang/dingo/pkg/codegen"
	"github.com/MadAppGang/dingo/pkg/parser"
	"github.com/MadAppGang/dingo/pkg/sourcemap"
	"github.com/MadAppGang/dingo/pkg/tokenizer"
	"github.com/MadAppGang/dingo/pkg/typechecker"
)

// tupleNodeWithPos holds a parsed tuple AST node with position information
type tupleNodeWithPos struct {
	kind        ast.TupleKind
	literal     *ast.TupleLiteral
	destructure *ast.TupleDestructure
}

// transformTupleTypeAliases transforms tuple type aliases before Go parsing.
// This is a PRE-PASS that must run before the Go parser because the syntax
// `type Point = (int, int)` is not valid Go and would cause parse errors.
//
// Pattern: type Name = (Type1, Type2, ...) → type Name = __tupleType{N}__(Type1, Type2, ...)
//
// NOTE: This uses tokenizer-based scanning which is normally discouraged per CLAUDE.md,
// but is necessary here because type declarations cannot be parsed by our statement parser.
func transformTupleTypeAliases(src []byte) ([]byte, error) {
	tok := tokenizer.New(src)
	result := src

	// Find all type alias locations (scan backwards to avoid offset drift)
	type typeAliasLoc struct {
		tupleStart int      // Position of '('
		tupleEnd   int      // Position after ')'
		types      []string // Element type strings
	}
	var locs []typeAliasLoc

	for {
		current := tok.NextToken()
		if current.Kind == tokenizer.EOF {
			break
		}

		// Look for: type IDENT = (
		if current.Kind == tokenizer.TYPE {
			// Save position to restore if not a tuple type
			savedPos := tok.SavePos()

			ident := tok.NextToken()
			if ident.Kind != tokenizer.IDENT {
				tok.RestorePos(savedPos)
				continue
			}

			assign := tok.NextToken()
			if assign.Kind != tokenizer.ASSIGN {
				tok.RestorePos(savedPos)
				continue
			}

			lparen := tok.NextToken()
			if lparen.Kind != tokenizer.LPAREN {
				tok.RestorePos(savedPos)
				continue
			}

			// Found "type X = (" - now parse the tuple type elements
			// NOTE: token.Pos is 1-based, so subtract 1 for 0-based array indexing
			tupleStart := int(lparen.Pos) - 1
			var types []string
			depth := 1
			typeStart := int(lparen.End) - 1
			hasComma := false
			var tupleEnd int

			for depth > 0 {
				t := tok.NextToken()
				if t.Kind == tokenizer.EOF {
					break
				}

				switch t.Kind {
				case tokenizer.LPAREN:
					depth++
				case tokenizer.RPAREN:
					depth--
					if depth == 0 {
						tupleEnd = int(t.End) - 1
						if hasComma {
							// Collect final type (1-based to 0-based conversion)
							typeStr := string(src[typeStart : int(t.Pos)-1])
							types = append(types, trimTypeWhitespace(typeStr))
						}
					}
				case tokenizer.COMMA:
					if depth == 1 {
						// Collect type before comma (1-based to 0-based conversion)
						typeStr := string(src[typeStart : int(t.Pos)-1])
						types = append(types, trimTypeWhitespace(typeStr))
						typeStart = int(t.End) - 1
						hasComma = true
					}
				}
			}

			// Only add if it's actually a tuple (has comma)
			if hasComma && len(types) >= 2 {
				locs = append(locs, typeAliasLoc{
					tupleStart: tupleStart,
					tupleEnd:   tupleEnd,
					types:      types,
				})
			}
		}
	}

	// Transform from end to beginning (reverse order)
	for i := len(locs) - 1; i >= 0; i-- {
		loc := locs[i]
		// Generate Go generic type: tuples.Tuple{N}[Type1, Type2, ...]
		// This directly produces valid Go code without needing Pass 2 resolution
		marker := fmt.Sprintf("tuples.Tuple%d[%s]", len(loc.types), joinTypes(loc.types))

		// Replace the tuple syntax with the marker
		newResult := make([]byte, 0, len(result)-(loc.tupleEnd-loc.tupleStart)+len(marker))
		newResult = append(newResult, result[:loc.tupleStart]...)
		newResult = append(newResult, []byte(marker)...)
		newResult = append(newResult, result[loc.tupleEnd:]...)
		result = newResult
	}

	return result, nil
}

// trimTypeWhitespace removes leading/trailing whitespace from type string
func trimTypeWhitespace(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}

// joinTypes joins type strings with ", "
func joinTypes(types []string) string {
	result := ""
	for i, t := range types {
		if i > 0 {
			result += ", "
		}
		result += t
	}
	return result
}

// tupleLiteralLoc holds location and elements of a tuple literal
type tupleLiteralLoc struct {
	start    int      // start position of '('
	end      int      // end position after ')'
	elements []string // element expressions as strings
}

// transformTupleLiterals transforms tuple literal expressions before Go parsing.
// This is a PRE-PASS that must run before the Go parser because the syntax
// `(expr1, expr2)` in expression contexts is ambiguous.
//
// Pattern: (a, b) → tuples.Tuple2[<types>]{First: a, Second: b}
// Since we don't have types yet, we use: (a, b) → __tuple2__(a, b)
// which will be resolved in Pass 2 with type information.
//
// Key distinction from type aliases:
// - Type aliases: `type X = (A, B)` - types after `=`
// - Literals: `return (a, b)` - expressions in various contexts
func transformTupleLiterals(src []byte) ([]byte, error) {
	tok := tokenizer.New(src)
	result := src
	var locs []tupleLiteralLoc

	// Track context to distinguish tuple literals from other parens
	// We need to find `(expr, expr)` but NOT:
	// - Function calls: `foo(a, b)` (preceded by IDENT)
	// - Type assertions: `x.(Type)` (preceded by `.`)
	// - Grouped expressions: `(a + b)` (no comma at depth 1)
	// - Slice index: `a[i:j]` (inside brackets)
	// - Let destructure patterns: `let ((a, b), c) = ...` (all parens after let until =)

	var prevToken tokenizer.Token
	var prevPrevToken tokenizer.Token
	inLetDestructure := false // Track if we're inside a let destructure pattern
	letDestructureDepth := 0  // Track paren depth within let destructure

	for {
		t := tok.NextToken()
		if t.Kind == tokenizer.EOF {
			break
		}

		// Track let destructure context
		if t.Kind == tokenizer.LET {
			inLetDestructure = true
			letDestructureDepth = 0
			prevPrevToken = prevToken
			prevToken = t
			continue
		}

		// Exit let destructure context when we see = (the assignment)
		if inLetDestructure && t.Kind == tokenizer.ASSIGN {
			inLetDestructure = false
			letDestructureDepth = 0
			prevPrevToken = prevToken
			prevToken = t
			continue
		}

		// Track paren depth within let destructure
		if inLetDestructure {
			if t.Kind == tokenizer.LPAREN {
				letDestructureDepth++
				prevPrevToken = prevToken
				prevToken = t
				continue // Skip all parens within let destructure
			}
			if t.Kind == tokenizer.RPAREN {
				letDestructureDepth--
				prevPrevToken = prevToken
				prevToken = t
				continue
			}
			// Continue to next token if we're in destructure pattern
			prevPrevToken = prevToken
			prevToken = t
			continue
		}

		// Look for LPAREN that could start a tuple literal
		if t.Kind == tokenizer.LPAREN {
			// Parenthesized declaration groups are Go syntax, not tuple
			// literals. Their declarations may contain commas inside composite
			// literals or multi-name specs, so scanning the whole group as a
			// tuple corrupts otherwise valid source.
			if prevToken.Kind == tokenizer.VAR || prevToken.Kind == tokenizer.CONST ||
				prevToken.Kind == tokenizer.IMPORT || prevToken.Kind == tokenizer.TYPE {
				prevPrevToken = prevToken
				prevToken = t
				continue
			}

			// Skip if this is a function call (preceded by IDENT or RPAREN)
			// Also skip if preceded by FUNC keyword (function literal parameters)
			// Also skip if preceded by RBRACKET (generic function parameters: func F[T any](...))
			if prevToken.Kind == tokenizer.IDENT || prevToken.Kind == tokenizer.RPAREN ||
				prevToken.Kind == tokenizer.FUNC || prevToken.Kind == tokenizer.RBRACKET {
				prevPrevToken = prevToken
				prevToken = t
				continue
			}

			// Skip if this is a type assertion (preceded by .)
			if prevToken.Kind == tokenizer.DOT {
				prevPrevToken = prevToken
				prevToken = t
				continue
			}

			// Skip if this is a type alias context (handled by transformTupleTypeAliases)
			// Pattern: type X = ( → already handled
			if prevToken.Kind == tokenizer.ASSIGN && prevPrevToken.Kind == tokenizer.IDENT {
				prevPrevToken = prevToken
				prevToken = t
				continue
			}

			// Potential tuple literal - scan to find if it has a comma at depth 1
			startPos := int(t.Pos) - 1 // 1-based to 0-based
			depth := 1
			braceDepth := 0
			bracketDepth := 0
			hasCommaAtDepth1 := false
			var elements []string
			elemStart := int(t.End) - 1
			var rparenEnd int // Track the end position of the closing paren

			for depth > 0 {
				inner := tok.NextToken()
				if inner.Kind == tokenizer.EOF {
					break
				}

				switch inner.Kind {
				case tokenizer.LPAREN:
					depth++
				case tokenizer.RPAREN:
					depth--
					if depth == 0 {
						rparenEnd = int(inner.End) - 1
						// Collect final element
						if hasCommaAtDepth1 {
							elemStr := string(src[elemStart : int(inner.Pos)-1])
							elements = append(elements, trimTypeWhitespace(elemStr))
						}
					}
				case tokenizer.LBRACE:
					braceDepth++
				case tokenizer.RBRACE:
					if braceDepth > 0 {
						braceDepth--
					}
				case tokenizer.LBRACKET:
					bracketDepth++
				case tokenizer.RBRACKET:
					if bracketDepth > 0 {
						bracketDepth--
					}
				case tokenizer.COMMA:
					if depth == 1 && braceDepth == 0 && bracketDepth == 0 {
						// Collect element before comma
						elemStr := string(src[elemStart : int(inner.Pos)-1])
						elements = append(elements, trimTypeWhitespace(elemStr))
						elemStart = int(inner.End) - 1
						hasCommaAtDepth1 = true
					}
				}
			}

			// After collecting elements, check if this is actually a lambda parameter list
			// or a tuple destructuring pattern by looking ahead
			if hasCommaAtDepth1 && len(elements) >= 2 {
				// Peek at the next token to see if it's => (lambda indicator) or := (destructuring)
				nextTok := tok.NextToken()
				if nextTok.Kind == tokenizer.ARROW {
					// This is a lambda parameter list (acc, u) => ..., not a tuple
					// Don't add to locs, continue processing
					prevPrevToken = prevToken
					prevToken = nextTok
					continue
				}
				if nextTok.Kind == tokenizer.DEFINE {
					// This is tuple DESTRUCTURING: (x, y) := expr
					// Don't treat as tuple literal - will be handled by transformTupleDestructuring
					// which runs separately and generates proper Go code: x, y := expr
					prevPrevToken = prevToken
					prevToken = nextTok
					continue
				}
				// Not a lambda or destructuring, it's a tuple literal - add to locs
				locs = append(locs, tupleLiteralLoc{
					start:    startPos,
					end:      rparenEnd,
					elements: elements,
				})
				// Restore position since we consumed nextTok
				// Note: We can't easily restore, so we update prevToken
				prevPrevToken = prevToken
				prevToken = nextTok
				continue
			}
		}

		prevPrevToken = prevToken
		prevToken = t
	}

	// Transform from end to beginning (reverse order to preserve positions)
	for i := len(locs) - 1; i >= 0; i-- {
		loc := locs[i]
		if len(loc.elements) < 2 {
			continue // Not a valid tuple
		}

		// Generate marker: __tuple{N}__(elem1, elem2, ...)
		// This will be resolved in Pass 2 with type information
		var buf bytes.Buffer
		buf.WriteString(fmt.Sprintf("__tuple%d__(", len(loc.elements)))
		for j, elem := range loc.elements {
			if j > 0 {
				buf.WriteString(", ")
			}
			buf.WriteString(elem)
		}
		buf.WriteString(")")
		marker := buf.Bytes()

		// Replace the tuple literal with the marker
		newResult := make([]byte, 0, len(result)-(loc.end-loc.start)+len(marker))
		newResult = append(newResult, result[:loc.start]...)
		newResult = append(newResult, marker...)
		newResult = append(newResult, result[loc.end:]...)
		result = newResult
	}

	return result, nil
}

// tupleDestructureLoc holds location and bindings for a tuple destructure pattern
type tupleDestructureLoc struct {
	start    int      // start position of '('
	end      int      // end position after ':='
	rhsStart int      // start position of RHS expression
	bindings []string // encoded bindings: "name:path" format (supports nesting)
}

// tupleFieldName returns the struct field name for a given tuple element index.
// This follows the naming convention in runtime/tuples package.
var tupleFieldNames = []string{"First", "Second", "Third", "Fourth", "Fifth", "Sixth", "Seventh", "Eighth", "Ninth", "Tenth"}

// parseNestedDestructurePattern parses a potentially nested destructure pattern.
// Returns encoded bindings ("name:path" format) and whether parsing succeeded.
// Handles patterns like: (x, y), ((a, b), c), ((_, b), (c, _))
// Wildcards (_) are skipped - no binding generated for them.
// pathPrefix is the path accumulated so far (empty at top level).
func parseNestedDestructurePattern(tok *tokenizer.Tokenizer, pathPrefix string) ([]string, bool) {
	var bindings []string
	elemIndex := 0

	for {
		t := tok.NextToken()
		if t.Kind == tokenizer.EOF {
			return nil, false
		}

		// Build path for current element
		currentPath := pathPrefix
		if currentPath != "" {
			currentPath += "."
		}
		currentPath += fmt.Sprintf("%d", elemIndex)

		switch t.Kind {
		case tokenizer.IDENT:
			// Simple variable binding
			bindings = append(bindings, fmt.Sprintf("%s:%s", t.Lit, currentPath))
			elemIndex++

		case tokenizer.UNDERSCORE:
			// Wildcard - skip binding but count element
			elemIndex++

		case tokenizer.LPAREN:
			// Nested pattern - recurse
			nestedBindings, ok := parseNestedDestructurePattern(tok, currentPath)
			if !ok {
				return nil, false
			}
			bindings = append(bindings, nestedBindings...)
			elemIndex++

		case tokenizer.RPAREN:
			// End of this pattern level
			return bindings, true

		case tokenizer.COMMA:
			// Continue to next element
			continue

		default:
			// Unexpected token - not a destructure pattern
			return nil, false
		}
	}
}

// transformTupleDestructuring transforms tuple destructuring patterns to markers.
// This handles both flat and nested syntax: (x, y) := expr, ((a, b), c) := expr
//
// Input:  (x, y) := p
// Output: _ = __tupleDest2__("x:0", "y:1", p)
//
// Input:  ((a, b), c) := bbox
// Output: _ = __tupleDest3__("a:0.0", "b:0.1", "c:1", bbox)
//
// The marker is then resolved in Pass 2 (TupleTypeResolver) which uses go/types
// to determine whether the RHS is:
// - A tuple struct (e.g., Point2D = tuples.Tuple2[...]) → tmp := p; x := tmp.First; y := tmp.Second
// - A Go multiple return call (e.g., getPoint()) → x, y := getPoint()
//
// This transform MUST run before transformTupleLiterals and before the Go parser.
func transformTupleDestructuring(src []byte) ([]byte, error) {
	tok := tokenizer.New(src)
	result := src
	var locs []tupleDestructureLoc

	for {
		t := tok.NextToken()
		if t.Kind == tokenizer.EOF {
			break
		}

		// Look for LPAREN that could start a destructure pattern
		if t.Kind == tokenizer.LPAREN {
			startPos := int(t.Pos) - 1 // 1-based to 0-based

			// Save position so we can restore if this isn't a destructure pattern
			savedPos := tok.SavePos()

			// Try to parse a potentially nested destructure pattern
			// Uses recursive parser to handle: (x, y), ((a, b), c), (((a, b), c), d), etc.
			bindings, ok := parseNestedDestructurePattern(tok, "")
			if !ok {
				tok.RestorePos(savedPos)
				continue
			}

			// Check for := after the pattern
			nextTok := tok.NextToken()
			if nextTok.Kind != tokenizer.DEFINE {
				tok.RestorePos(savedPos)
				continue
			}

			// This is a tuple destructure!
			// The RHS starts right after :=
			rhsStart := int(nextTok.End) - 1

			// Now find the end of the RHS expression
			rhsEnd := findExpressionEnd(src, rhsStart)

			locs = append(locs, tupleDestructureLoc{
				start:    startPos,
				end:      rhsEnd,
				rhsStart: rhsStart,
				bindings: bindings,
			})
			// Continue from here, don't restore
		}
	}

	// Transform from end to beginning (reverse order to preserve positions)
	for i := len(locs) - 1; i >= 0; i-- {
		loc := locs[i]

		// Extract the RHS expression from source
		rhsSrc := string(result[loc.rhsStart:loc.end])
		// Trim leading/trailing whitespace from RHS
		rhsSrc = trimExprWhitespace(rhsSrc)

		// Generate replacement code
		// Special case: all wildcards (zero bindings)
		// For patterns like (_, _) := pair, generate "_ = pair" to evaluate RHS for side effects
		var replacement []byte
		if len(loc.bindings) == 0 {
			replacement = []byte("_ = " + rhsSrc)
		} else {
			// Generate marker: _ = __tupleDest{N}__("var1:0", "var2:1.0", expr)
			// The marker format is: _ = __tupleDest{N}__("name:path", ..., expr)
			// where path is dot-separated indices for nested access.
			//
			// Pass 2 (TupleTypeResolver) will use go/types to determine whether the
			// RHS expression is a tuple struct or a Go multiple return, and generate
			// the appropriate code:
			// - Tuple struct: tpl := expr; var1 := tpl.First; var2 := tpl.Second.First
			// - Go multi-return: var1, var2 := expr (only for flat patterns)
			var buf bytes.Buffer
			buf.WriteString(fmt.Sprintf("_ = __tupleDest%d__(", len(loc.bindings)))
			for j, binding := range loc.bindings {
				if j > 0 {
					buf.WriteString(", ")
				}
				// Binding is already in "name:path" format
				buf.WriteString(fmt.Sprintf("\"%s\"", binding))
			}
			buf.WriteString(", ")
			buf.WriteString(rhsSrc)
			buf.WriteString(")")
			replacement = buf.Bytes()
		}

		// Replace "((a, b), c) := expr" with "_ = __tupleDest3__("a:0.0", "b:0.1", "c:1", expr)"
		newResult := make([]byte, 0, len(result)-(loc.end-loc.start)+len(replacement))
		newResult = append(newResult, result[:loc.start]...)
		newResult = append(newResult, replacement...)
		newResult = append(newResult, result[loc.end:]...)
		result = newResult
	}

	return result, nil
}

// findExpressionEnd finds the end position of an expression starting at startPos.
// It handles balanced parentheses, brackets, and braces, and stops at newline or EOF.
func findExpressionEnd(src []byte, startPos int) int {
	depth := 0
	pos := startPos

	// Skip leading whitespace
	for pos < len(src) && (src[pos] == ' ' || src[pos] == '\t') {
		pos++
	}

	for pos < len(src) {
		ch := src[pos]

		switch ch {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			if depth > 0 {
				depth--
			} else {
				// Unmatched closing delimiter - stop before it
				return pos
			}
		case '\n':
			// End of line - if we're not inside balanced delimiters, we're done
			if depth == 0 {
				return pos
			}
		case '/':
			// Check for comment start
			if pos+1 < len(src) && (src[pos+1] == '/' || src[pos+1] == '*') {
				if depth == 0 {
					return pos
				}
			}
		}
		pos++
	}

	return pos
}

// trimExprWhitespace removes leading and trailing whitespace from an expression string
func trimExprWhitespace(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}

// transformTuplePass1 transforms all tuple syntax using parser-produced AST nodes.
// This is Pass 1 of the two-pass tuple pipeline:
//   - Literals: (a, b) → __tuple2__(a, b)
//   - Destructuring: let (x, y) = point → let __tupleDest2__("x", "y", point)
//
// Note: Type aliases are handled by transformTupleTypeAliases() which runs BEFORE this.
//
// The markers are valid Go code that will be type-checked by go/types.
// Pass 2 will then resolve the markers to actual struct types.
//
// ARCHITECTURE: This function now uses the parser to produce AST nodes instead of
// scanning raw bytes. This follows the mandated pipeline: tokenizer → parser → AST → codegen.
func transformTuplePass1(src []byte) ([]byte, error) {
	// Parse the source to extract tuple AST nodes
	fset := token.NewFileSet()
	tok := tokenizer.New(src)
	stmtParser := parser.NewStmtParser(tok, fset)

	// Parse all statements to collect tuples
	// Note: This is a quick parse just to collect tuple nodes for transformation
	// The actual Go parsing happens later in the pipeline
	for {
		// Check for EOF before parsing
		if stmtParser.IsAtEnd() {
			break
		}
		stmt, err := stmtParser.ParseStatement()
		if err != nil {
			// Currently, ParseStatement wraps errors in recovery, so we treat any error
			// as a signal to stop parsing. In a future enhancement, we could distinguish
			// between fatal errors and "no statement" via error sentinels.
			// For now, this is conservative and allows the pipeline to continue.
			break
		}
		if stmt == nil {
			// Not an error - parser returns nil for most Go statements (package, import, func, etc.)
			// Continue to next statement instead of breaking
			continue
		}
	}

	// Collect all parsed tuple nodes
	var tupleNodes []tupleNodeWithPos

	// Add tuple destructures from parser
	for _, td := range stmtParser.TupleDestructures {
		tupleNodes = append(tupleNodes, tupleNodeWithPos{
			kind:        ast.TupleKindDestructure,
			destructure: td,
		})
	}

	// Add tuple literals from parser
	for _, tl := range stmtParser.TupleLiterals {
		tupleNodes = append(tupleNodes, tupleNodeWithPos{
			kind:    ast.TupleKindLiteral,
			literal: tl,
		})
	}

	// If no tuples found, return source unchanged
	if len(tupleNodes) == 0 {
		return src, nil
	}

	// Sort tuple nodes by position descending (transform from end to avoid offset shifts)
	sort.Slice(tupleNodes, func(i, j int) bool {
		return tupleNodes[i].Pos() > tupleNodes[j].Pos()
	})

	result := src

	// Transform each tuple from end to beginning
	for _, node := range tupleNodes {
		var genResult ast.CodeGenResult
		var replaceStart, replaceEnd int

		// IMPORTANT: Create fresh generator for each node to avoid buffer accumulation.
		// Each codegen has its own buffer, so reusing would concatenate all outputs.
		gen := codegen.NewTupleCodeGen()

		switch node.kind {
		case ast.TupleKindLiteral:
			// Generate code for tuple literal using AST node
			genResult = gen.GenerateLiteral(node.literal)
			// NOTE: token.Pos is 1-based, subtract 1 for 0-based array indexing
			replaceStart = int(node.literal.Lparen) - 1
			replaceEnd = int(node.literal.Rparen) // +1-1=0 (includes closing paren)

		case ast.TupleKindDestructure:
			// Generate code for tuple destructuring using AST node
			genResult = gen.GenerateDestructure(node.destructure)
			// NOTE: token.Pos is 1-based, subtract 1 for 0-based array indexing
			replaceStart = int(node.destructure.LetPos) - 1
			// Guard against nil Value (malformed input like "let (x, y) =")
			if node.destructure.Value == nil {
				continue // Skip malformed destructure
			}
			replaceEnd = int(node.destructure.Value.End()) - 1

			// Make it a valid Go statement: _ = marker
			// The destructure codegen produces the marker, we just need to prefix it
			genResult.Output = append([]byte("_ = "), genResult.Output...)
		}

		if len(genResult.Output) == 0 {
			continue
		}

		// TECHNICAL DEBT: This byte splicing approach works but violates the pure AST
		// pipeline philosophy documented in CLAUDE.md. In a future refactor, we should
		// build the complete AST first, then generate all code in one pass.
		// For now, this incremental approach is pragmatic and maintains backward compatibility.
		// See: https://github.com/MadAppGang/dingo/issues/XXX (TODO: create issue)
		marker := genResult.Output
		newResult := make([]byte, 0, len(result)-(replaceEnd-replaceStart)+len(marker))
		newResult = append(newResult, result[:replaceStart]...)
		newResult = append(newResult, marker...)
		newResult = append(newResult, result[replaceEnd:]...)
		result = newResult
	}

	return result, nil
}

// Pos returns the starting position of a tuple node
func (n tupleNodeWithPos) Pos() gotoken.Pos {
	switch n.kind {
	case ast.TupleKindLiteral:
		return n.literal.Lparen
	case ast.TupleKindDestructure:
		return n.destructure.LetPos
	default:
		return gotoken.NoPos
	}
}

// transformTuplePass2 resolves tuple markers to final struct types using go/types.
// This is Pass 2 of the two-pass tuple pipeline.
//
// Markers from Pass 1:
//   - __tuple2__(a, b) → Tuple2IntString{_0: a, _1: b}
//   - __tupleDest2__("x", "y", point) → tmp := point; x := tmp._0; y := tmp._1
//   - __tupleType2__(int, string) → Tuple2IntString
//
// Returns the transformed source with markers replaced by actual Go code,
// along with line mappings for tuple destructures.
func transformTuplePass2(fset *gotoken.FileSet, file *goast.File, checker *typechecker.Checker, src []byte) ([]byte, []sourcemap.LineMapping, error) {
	// Quick check: do we have any tuple markers?
	// If not, skip the transformation entirely to avoid overhead
	if !bytes.Contains(src, []byte("__tuple")) {
		return src, nil, nil
	}

	// Create a type resolver from the marker-infused source
	resolver, err := codegen.NewTupleTypeResolver(src)
	if err != nil {
		return nil, nil, fmt.Errorf("create tuple resolver: %w", err)
	}

	// Resolve markers to final Go code
	result, err := resolver.Resolve(src)
	if err != nil {
		return nil, nil, fmt.Errorf("resolve tuple markers: %w", err)
	}

	return result.Output, resolver.LineMappings(), nil
}
