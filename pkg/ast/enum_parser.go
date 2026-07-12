package ast

import (
	"fmt"
	"go/scanner"
	"go/token"
)

// EnumParser parses Dingo enum declarations into AST nodes.
// This provides proper AST-based parsing instead of regex/string manipulation.
type EnumParser struct {
	src    []byte
	pos    int
	fset   *token.FileSet
	file   *token.File
	offset int // Base offset in source file
}

// NewEnumParser creates a new enum parser for the given source.
func NewEnumParser(src []byte, offset int) *EnumParser {
	fset := token.NewFileSet()
	file := fset.AddFile("", -1, len(src))
	return &EnumParser{
		src:    src,
		pos:    0,
		fset:   fset,
		file:   file,
		offset: offset,
	}
}

// ParseEnumDecl parses an enum declaration starting at current position.
// Returns the EnumDecl and the end position in the source.
func (p *EnumParser) ParseEnumDecl() (*EnumDecl, int, error) {
	startPos := p.pos

	// Expect "enum" keyword
	if !p.matchKeyword("enum") {
		return nil, startPos, fmt.Errorf("expected 'enum' keyword")
	}
	enumPos := token.Pos(p.offset + startPos + 1)

	p.skipWhitespace()

	// Parse enum name
	name, err := p.parseIdent()
	if err != nil {
		return nil, p.pos, fmt.Errorf("expected enum name: %w", err)
	}

	p.skipWhitespace()

	// Parse optional type parameters [T, E] (Go-style generic syntax)
	var typeParams *TypeParamList
	if p.peek() == '[' {
		typeParams, err = p.parseTypeParams()
		if err != nil {
			return nil, p.pos, fmt.Errorf("invalid type parameters: %w", err)
		}
		p.skipWhitespace()
	}

	// Expect '{'
	if p.peek() != '{' {
		return nil, p.pos, fmt.Errorf("expected '{' after enum name, got %q", string(p.peek()))
	}
	lbracePos := token.Pos(p.offset + p.pos + 1)
	p.advance()

	// Parse variants
	variants, err := p.parseVariants()
	if err != nil {
		return nil, p.pos, fmt.Errorf("invalid variants: %w", err)
	}

	p.skipWhitespaceAndCommas()

	// Expect '}'
	if p.peek() != '}' {
		return nil, p.pos, fmt.Errorf("expected '}' to close enum, got %q at pos %d", string(p.peek()), p.pos)
	}
	rbracePos := token.Pos(p.offset + p.pos + 1)
	p.advance()

	return &EnumDecl{
		Enum:       enumPos,
		Name:       name,
		TypeParams: typeParams,
		LBrace:     lbracePos,
		Variants:   variants,
		RBrace:     rbracePos,
	}, p.pos, nil
}

// parseIdent parses an identifier
func (p *EnumParser) parseIdent() (*Ident, error) {
	start := p.pos
	if !p.isAlpha(p.peek()) {
		return nil, fmt.Errorf("expected identifier, got %q", string(p.peek()))
	}

	for p.isAlphaNum(p.peek()) {
		p.advance()
	}

	return &Ident{
		NamePos: token.Pos(p.offset + start + 1),
		Name:    string(p.src[start:p.pos]),
	}, nil
}

// parseTypeParams parses generic type parameters: [T, E] (Go-style generic syntax)
func (p *EnumParser) parseTypeParams() (*TypeParamList, error) {
	if p.peek() != '[' {
		return nil, fmt.Errorf("expected '['")
	}
	opening := token.Pos(p.offset + p.pos + 1)
	p.advance()

	var params []*Ident
	for {
		p.skipWhitespace()

		if p.peek() == ']' {
			break
		}

		ident, err := p.parseIdent()
		if err != nil {
			return nil, err
		}
		params = append(params, ident)

		p.skipWhitespace()

		if p.peek() == ',' {
			p.advance()
			continue
		}
		if p.peek() == ']' {
			break
		}
		return nil, fmt.Errorf("expected ',' or ']' in type params")
	}

	if p.peek() != ']' {
		return nil, fmt.Errorf("expected ']'")
	}
	closing := token.Pos(p.offset + p.pos + 1)
	p.advance()

	return &TypeParamList{
		Opening: opening,
		Params:  params,
		Closing: closing,
	}, nil
}

// parseVariants parses all enum variants
func (p *EnumParser) parseVariants() ([]*EnumVariant, error) {
	var variants []*EnumVariant

	for {
		p.skipWhitespaceAndCommas()

		// Check for end of variants
		if p.peek() == '}' || p.pos >= len(p.src) {
			break
		}

		variant, err := p.parseVariant()
		if err != nil {
			return nil, err
		}
		variants = append(variants, variant)
	}

	return variants, nil
}

// parseVariant parses a single enum variant
func (p *EnumParser) parseVariant() (*EnumVariant, error) {
	name, err := p.parseIdent()
	if err != nil {
		return nil, fmt.Errorf("expected variant name: %w", err)
	}

	p.skipWhitespace()

	variant := &EnumVariant{
		Name: name,
		Kind: UnitVariant,
	}

	// Check for tuple variant: Variant(T)
	if p.peek() == '(' {
		variant.Kind = TupleVariant
		variant.LDelim = token.Pos(p.offset + p.pos + 1)
		p.advance()

		fields, err := p.parseTupleFields()
		if err != nil {
			return nil, err
		}
		variant.Fields = fields

		if p.peek() != ')' {
			return nil, fmt.Errorf("expected ')' to close tuple variant")
		}
		variant.RDelim = token.Pos(p.offset + p.pos + 1)
		p.advance()
	}

	// Check for struct variant: Variant { field: type }
	if p.peek() == '{' {
		variant.Kind = StructVariant
		variant.LDelim = token.Pos(p.offset + p.pos + 1)
		p.advance()

		fields, err := p.parseStructFields()
		if err != nil {
			return nil, err
		}
		variant.Fields = fields

		p.skipWhitespaceAndCommas()
		if p.peek() != '}' {
			return nil, fmt.Errorf("expected '}' to close struct variant")
		}
		variant.RDelim = token.Pos(p.offset + p.pos + 1)
		p.advance()
	}

	return variant, nil
}

// parseTupleFields parses tuple variant fields: (T, E)
func (p *EnumParser) parseTupleFields() ([]*EnumField, error) {
	var fields []*EnumField

	for {
		p.skipWhitespace()

		if p.peek() == ')' {
			break
		}

		typeExpr, err := p.parseTypeExpr()
		if err != nil {
			return nil, err
		}

		fields = append(fields, &EnumField{
			Type: typeExpr,
		})

		p.skipWhitespace()

		if p.peek() == ',' {
			p.advance()
			continue
		}
		if p.peek() == ')' {
			break
		}
		return nil, fmt.Errorf("expected ',' or ')' in tuple fields")
	}

	return fields, nil
}

// parseStructFields parses struct variant fields: { field: type, ... }
func (p *EnumParser) parseStructFields() ([]*EnumField, error) {
	var fields []*EnumField

	for {
		p.skipWhitespaceAndCommas()

		if p.peek() == '}' {
			break
		}

		// Parse field name
		name, err := p.parseIdent()
		if err != nil {
			return nil, fmt.Errorf("expected field name: %w", err)
		}

		p.skipWhitespace()

		// Expect ':'
		if p.peek() != ':' {
			return nil, fmt.Errorf("expected ':' after field name")
		}
		colonPos := token.Pos(p.offset + p.pos + 1)
		p.advance()

		p.skipWhitespace()

		// Parse type
		typeExpr, err := p.parseTypeExpr()
		if err != nil {
			return nil, err
		}

		fields = append(fields, &EnumField{
			Name:  name,
			Colon: colonPos,
			Type:  typeExpr,
		})
	}

	return fields, nil
}

// parseTypeExpr parses a type expression
func (p *EnumParser) parseTypeExpr() (*TypeExpr, error) {
	start := p.pos

	// Accept any number of '*' (pointer) and '[]' (slice) prefixes in any order.
	// Supports *T, []T, []*T, *[]T, [][]T, **T, etc.
	for {
		if p.peek() == '*' {
			p.advance()
			continue
		}
		if p.peek() == '[' && p.pos+1 < len(p.src) && p.src[p.pos+1] == ']' {
			p.advance() // '['
			p.advance() // ']'
			continue
		}
		break
	}

	// Parse base type name
	if !p.isAlpha(p.peek()) {
		return nil, fmt.Errorf("expected type name")
	}
	for p.isAlphaNum(p.peek()) || p.peek() == '.' {
		p.advance()
	}

	// Handle generic type arguments (Go-style [T, E] syntax)
	if p.peek() == '[' {
		depth := 1
		p.advance()
		for depth > 0 && p.pos < len(p.src) {
			if p.peek() == '[' {
				depth++
			} else if p.peek() == ']' {
				depth--
			}
			p.advance()
		}
	}

	return &TypeExpr{
		StartPos: token.Pos(p.offset + start + 1),
		EndPos:   token.Pos(p.offset + p.pos + 1),
		Text:     string(p.src[start:p.pos]),
	}, nil
}

// Helper methods

func (p *EnumParser) peek() byte {
	if p.pos >= len(p.src) {
		return 0
	}
	return p.src[p.pos]
}

func (p *EnumParser) advance() {
	if p.pos < len(p.src) {
		p.pos++
	}
}

func (p *EnumParser) skipWhitespace() {
	for p.pos < len(p.src) && (p.src[p.pos] == ' ' || p.src[p.pos] == '\t' || p.src[p.pos] == '\n' || p.src[p.pos] == '\r') {
		p.pos++
	}
}

func (p *EnumParser) skipWhitespaceAndCommas() {
	for p.pos < len(p.src) && (p.src[p.pos] == ' ' || p.src[p.pos] == '\t' || p.src[p.pos] == '\n' || p.src[p.pos] == '\r' || p.src[p.pos] == ',') {
		p.pos++
	}
}

func (p *EnumParser) matchKeyword(keyword string) bool {
	if p.pos+len(keyword) > len(p.src) {
		return false
	}
	if string(p.src[p.pos:p.pos+len(keyword)]) != keyword {
		return false
	}
	// Ensure it's not part of a larger identifier
	if p.pos+len(keyword) < len(p.src) && p.isAlphaNum(p.src[p.pos+len(keyword)]) {
		return false
	}
	p.pos += len(keyword)
	return true
}

func (p *EnumParser) isAlpha(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || b == '_'
}

func (p *EnumParser) isAlphaNum(b byte) bool {
	return p.isAlpha(b) || (b >= '0' && b <= '9')
}

// FindEnumDeclarations scans source and returns positions of all enum declarations.
// Uses Go's scanner to properly handle comments and string literals.
// This ensures "enum" inside comments or strings is not misidentified.
func FindEnumDeclarations(src []byte) []int {
	var positions []int

	fset := token.NewFileSet()
	file := fset.AddFile("", -1, len(src))

	var s scanner.Scanner
	s.Init(file, src, nil, scanner.ScanComments)

	for {
		pos, tok, lit := s.Scan()
		if tok == token.EOF {
			break
		}

		// Only consider IDENT tokens with literal "enum"
		// The scanner automatically skips comments and handles strings
		if tok == token.IDENT && lit == "enum" {
			offset := file.Offset(pos)
			positions = append(positions, offset)
		}
	}

	return positions
}
