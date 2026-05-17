package tokenizer

import (
	"go/token"
	"testing"
)

func TestTokenKindString(t *testing.T) {
	tests := []struct {
		kind TokenKind
		want string
	}{
		{ILLEGAL, "ILLEGAL"},
		{EOF, "EOF"},
		{COMMENT, "COMMENT"},
		{IDENT, "IDENT"},
		{MATCH, "match"},
		{ARROW, "=>"},
		{LPAREN, "("},
		{NEWLINE, "NEWLINE"},
	}

	for _, tt := range tests {
		if got := tt.kind.String(); got != tt.want {
			t.Errorf("TokenKind.String() = %q, want %q", got, tt.want)
		}
	}
}

func TestTokenIsKeyword(t *testing.T) {
	tests := []struct {
		kind TokenKind
		want bool
	}{
		{MATCH, true},
		{IF, true},
		{WHERE, true},
		{ENUM, true},
		{GUARD, true},
		{IDENT, false},
		{ARROW, false},
		{COMMENT, false},
	}

	for _, tt := range tests {
		tok := Token{Kind: tt.kind}
		if got := tok.IsKeyword(); got != tt.want {
			t.Errorf("Token{Kind: %s}.IsKeyword() = %v, want %v", tt.kind, got, tt.want)
		}
	}
}

func TestScannerBasics(t *testing.T) {
	src := []byte("hello world")
	s := NewScanner(src)

	if s.AtEOF() {
		t.Error("NewScanner should not start at EOF")
	}

	if r := s.Peek(); r != 'h' {
		t.Errorf("Peek() = %c, want 'h'", r)
	}

	if r := s.Next(); r != 'h' {
		t.Errorf("Next() = %c, want 'h'", r)
	}

	if r := s.Peek(); r != 'e' {
		t.Errorf("Peek() after Next() = %c, want 'e'", r)
	}
}

func TestScannerPeekN(t *testing.T) {
	src := []byte("hello")
	s := NewScanner(src)

	if got := s.PeekN(5); got != "hello" {
		t.Errorf("PeekN(5) = %q, want %q", got, "hello")
	}

	if got := s.PeekN(2); got != "he" {
		t.Errorf("PeekN(2) = %q, want %q", got, "he")
	}

	// PeekN beyond source length
	if got := s.PeekN(10); got != "hello" {
		t.Errorf("PeekN(10) = %q, want %q", got, "hello")
	}
}

func TestScannerPosition(t *testing.T) {
	src := []byte("line1\nline2\nline3")
	s := NewScanner(src)

	line, col := s.Position()
	if line != 1 || col != 1 {
		t.Errorf("Initial position = (%d, %d), want (1, 1)", line, col)
	}

	// Advance to newline
	for i := 0; i < 5; i++ {
		s.Next()
	}

	line, col = s.Position()
	if line != 1 || col != 6 {
		t.Errorf("Position before newline = (%d, %d), want (1, 6)", line, col)
	}

	// Cross newline
	s.Next() // consume '\n'

	line, col = s.Position()
	if line != 2 || col != 1 {
		t.Errorf("Position after newline = (%d, %d), want (2, 1)", line, col)
	}
}

func TestTokenizeIdentifiers(t *testing.T) {
	src := []byte("match if where enum guard foo bar_123")
	tok := New(src)

	tokens, err := tok.Tokenize()
	if err != nil {
		t.Fatalf("Tokenize() error = %v", err)
	}

	expected := []struct {
		kind TokenKind
		lit  string
	}{
		{MATCH, "match"},
		{IF, "if"},
		{WHERE, "where"},
		{ENUM, "enum"},
		{GUARD, "guard"},
		{IDENT, "foo"},
		{IDENT, "bar_123"},
		{EOF, ""},
	}

	if len(tokens) != len(expected) {
		t.Fatalf("got %d tokens, want %d", len(tokens), len(expected))
	}

	for i, want := range expected {
		if tokens[i].Kind != want.kind {
			t.Errorf("token[%d].Kind = %s, want %s", i, tokens[i].Kind, want.kind)
		}
		if tokens[i].Lit != want.lit {
			t.Errorf("token[%d].Lit = %q, want %q", i, tokens[i].Lit, want.lit)
		}
	}
}

func TestTokenizeOperators(t *testing.T) {
	src := []byte("=> == != <= >= && || ( ) { } , _")
	tok := New(src)

	tokens, err := tok.Tokenize()
	if err != nil {
		t.Fatalf("Tokenize() error = %v", err)
	}

	expected := []TokenKind{
		ARROW, EQ, NE, LE, GE, AND, OR,
		LPAREN, RPAREN, LBRACE, RBRACE, COMMA, UNDERSCORE,
		EOF,
	}

	if len(tokens) != len(expected) {
		t.Fatalf("got %d tokens, want %d", len(tokens), len(expected))
	}

	for i, want := range expected {
		if tokens[i].Kind != want {
			t.Errorf("token[%d].Kind = %s, want %s", i, tokens[i].Kind, want)
		}
	}
}

func TestTokenizeNumbers(t *testing.T) {
	tests := []struct {
		input string
		kind  TokenKind
		lit   string
	}{
		{"123", INT, "123"},
		{"0", INT, "0"},
		{"42", INT, "42"},
		{"3.14", FLOAT, "3.14"},
		{"0.5", FLOAT, "0.5"},
		{".5", FLOAT, ".5"},
	}

	for _, tt := range tests {
		tok := New([]byte(tt.input))
		tokens, err := tok.Tokenize()
		if err != nil {
			t.Fatalf("Tokenize(%q) error = %v", tt.input, err)
		}

		if len(tokens) < 1 {
			t.Fatalf("Tokenize(%q) returned no tokens", tt.input)
		}

		if tokens[0].Kind != tt.kind {
			t.Errorf("Tokenize(%q).Kind = %s, want %s", tt.input, tokens[0].Kind, tt.kind)
		}

		if tokens[0].Lit != tt.lit {
			t.Errorf("Tokenize(%q).Lit = %q, want %q", tt.input, tokens[0].Lit, tt.lit)
		}
	}
}

func TestTokenizeStrings(t *testing.T) {
	tests := []struct {
		input string
		lit   string
	}{
		{`"hello"`, `"hello"`},
		{`"hello world"`, `"hello world"`},
		{`"with\nnewline"`, `"with\nnewline"`},
		{`"with\"quote"`, `"with\"quote"`},
		{"`raw string`", "`raw string`"},
		{"`multi\nline`", "`multi\nline`"},
	}

	for _, tt := range tests {
		tok := New([]byte(tt.input))
		tokens, err := tok.Tokenize()
		if err != nil {
			t.Fatalf("Tokenize(%q) error = %v", tt.input, err)
		}

		if len(tokens) < 1 {
			t.Fatalf("Tokenize(%q) returned no tokens", tt.input)
		}

		if tokens[0].Kind != STRING {
			t.Errorf("Tokenize(%q).Kind = %s, want STRING", tt.input, tokens[0].Kind)
		}

		if tokens[0].Lit != tt.lit {
			t.Errorf("Tokenize(%q).Lit = %q, want %q", tt.input, tokens[0].Lit, tt.lit)
		}
	}
}

func TestTokenizeCharLiteral(t *testing.T) {
	tests := []struct {
		input string
		lit   string
	}{
		{"'a'", "'a'"},
		{"'\\n'", "'\\n'"},
		{"'\\''", "'\\''"},
	}

	for _, tt := range tests {
		tok := New([]byte(tt.input))
		tokens, err := tok.Tokenize()
		if err != nil {
			t.Fatalf("Tokenize(%q) error = %v", tt.input, err)
		}

		if tokens[0].Kind != CHAR {
			t.Errorf("Tokenize(%q).Kind = %s, want CHAR", tt.input, tokens[0].Kind)
		}

		if tokens[0].Lit != tt.lit {
			t.Errorf("Tokenize(%q).Lit = %q, want %q", tt.input, tokens[0].Lit, tt.lit)
		}
	}
}

func TestTokenizeLineComment(t *testing.T) {
	src := []byte("foo // this is a comment\nbar")
	tok := New(src)

	tokens, err := tok.Tokenize()
	if err != nil {
		t.Fatalf("Tokenize() error = %v", err)
	}

	expected := []struct {
		kind TokenKind
		lit  string
	}{
		{IDENT, "foo"},
		{COMMENT, "// this is a comment"},
		{NEWLINE, ""},
		{IDENT, "bar"},
		{EOF, ""},
	}

	if len(tokens) != len(expected) {
		t.Fatalf("got %d tokens, want %d", len(tokens), len(expected))
	}

	for i, want := range expected {
		if tokens[i].Kind != want.kind {
			t.Errorf("token[%d].Kind = %s, want %s", i, tokens[i].Kind, want.kind)
		}
		if tokens[i].Lit != want.lit {
			t.Errorf("token[%d].Lit = %q, want %q", i, tokens[i].Lit, want.lit)
		}
	}
}

func TestTokenizeBlockComment(t *testing.T) {
	src := []byte("foo /* multi\nline\ncomment */ bar")
	tok := New(src)

	tokens, err := tok.Tokenize()
	if err != nil {
		t.Fatalf("Tokenize() error = %v", err)
	}

	if len(tokens) < 3 {
		t.Fatalf("got %d tokens, want at least 3", len(tokens))
	}

	if tokens[1].Kind != COMMENT {
		t.Errorf("token[1].Kind = %s, want COMMENT", tokens[1].Kind)
	}

	expectedComment := "/* multi\nline\ncomment */"
	if tokens[1].Lit != expectedComment {
		t.Errorf("token[1].Lit = %q, want %q", tokens[1].Lit, expectedComment)
	}
}

func TestTokenizeMatchExpression(t *testing.T) {
	src := []byte(`match result {
		Ok(x) => x,
		Err(e) => 0
	}`)

	tok := New(src)
	tokens, err := tok.Tokenize()
	if err != nil {
		t.Fatalf("Tokenize() error = %v", err)
	}

	// Verify key tokens are present
	expectedKinds := []TokenKind{
		MATCH, IDENT, LBRACE, NEWLINE,
		IDENT, LPAREN, IDENT, RPAREN, ARROW, IDENT, COMMA, NEWLINE,
		IDENT, LPAREN, IDENT, RPAREN, ARROW, INT, NEWLINE,
		RBRACE, EOF,
	}

	if len(tokens) != len(expectedKinds) {
		t.Fatalf("got %d tokens, want %d", len(tokens), len(expectedKinds))
	}

	for i, want := range expectedKinds {
		if tokens[i].Kind != want {
			t.Errorf("token[%d].Kind = %s, want %s (lit=%q)", i, tokens[i].Kind, want, tokens[i].Lit)
		}
	}
}

func TestTokenizeWithComments(t *testing.T) {
	src := []byte(`match event {
		Click(x, y) => handleClick(x, y), // Click events
		_ => {}
	}`)

	tok := New(src)
	tokens, err := tok.Tokenize()
	if err != nil {
		t.Fatalf("Tokenize() error = %v", err)
	}

	// Find the comment token
	var commentFound bool
	for _, token := range tokens {
		if token.Kind == COMMENT {
			commentFound = true
			if token.Lit != "// Click events" {
				t.Errorf("Comment lit = %q, want %q", token.Lit, "// Click events")
			}
			break
		}
	}

	if !commentFound {
		t.Error("Comment token not found in match expression with inline comment")
	}
}

func TestParserHelperMethods(t *testing.T) {
	src := []byte("foo bar baz")
	tok := New(src)

	_, err := tok.Tokenize()
	if err != nil {
		t.Fatalf("Tokenize() error = %v", err)
	}

	// Test Current
	if curr := tok.Current(); curr.Lit != "foo" {
		t.Errorf("Current().Lit = %q, want %q", curr.Lit, "foo")
	}

	// Test Advance
	tok.Advance()
	if curr := tok.Current(); curr.Lit != "bar" {
		t.Errorf("After Advance, Current().Lit = %q, want %q", curr.Lit, "bar")
	}

	// Test PeekToken
	if peek := tok.PeekToken(); peek.Lit != "baz" {
		t.Errorf("PeekToken().Lit = %q, want %q", peek.Lit, "baz")
	}

	// Verify peek doesn't advance
	if curr := tok.Current(); curr.Lit != "bar" {
		t.Errorf("After PeekToken, Current().Lit = %q, want %q", curr.Lit, "bar")
	}

	// Test Reset
	tok.Reset()
	if curr := tok.Current(); curr.Lit != "foo" {
		t.Errorf("After Reset, Current().Lit = %q, want %q", curr.Lit, "foo")
	}
}

func TestExpectMethod(t *testing.T) {
	src := []byte("match foo")
	tok := New(src)

	_, err := tok.Tokenize()
	if err != nil {
		t.Fatalf("Tokenize() error = %v", err)
	}

	// Test successful expect
	matchTok, err := tok.Expect(MATCH)
	if err != nil {
		t.Fatalf("Expect(MATCH) error = %v", err)
	}
	if matchTok.Lit != "match" {
		t.Errorf("Expect(MATCH).Lit = %q, want %q", matchTok.Lit, "match")
	}

	// Test failed expect
	_, err = tok.Expect(ARROW)
	if err == nil {
		t.Error("Expect(ARROW) should fail but got no error")
	}
}

func TestMatchMethod(t *testing.T) {
	src := []byte("foo, bar")
	tok := New(src)

	_, err := tok.Tokenize()
	if err != nil {
		t.Fatalf("Tokenize() error = %v", err)
	}

	// Test successful match
	if !tok.Match(IDENT) {
		t.Error("Match(IDENT) should succeed")
	}

	// Should advance past matched token
	if curr := tok.Current(); curr.Kind != COMMA {
		t.Errorf("After Match, Current().Kind = %s, want COMMA", curr.Kind)
	}

	// Test match with multiple kinds
	if !tok.Match(ARROW, COMMA, LPAREN) {
		t.Error("Match(ARROW, COMMA, LPAREN) should succeed (COMMA matches)")
	}

	// Test failed match
	if tok.Match(ARROW) {
		t.Error("Match(ARROW) should fail but succeeded")
	}
}

func TestTokenizeWithFileSet(t *testing.T) {
	src := []byte("match foo { _ => 0 }")
	fset := token.NewFileSet()
	tok := NewWithFileSet(src, fset, "test.dingo")

	tokens, err := tok.Tokenize()
	if err != nil {
		t.Fatalf("Tokenize() error = %v", err)
	}

	// Verify positions are valid
	for i, token := range tokens {
		if !token.Pos.IsValid() {
			t.Errorf("token[%d].Pos is invalid", i)
		}
		if !token.End.IsValid() {
			t.Errorf("token[%d].End is invalid", i)
		}
		if token.End < token.Pos {
			t.Errorf("token[%d].End < Pos (%d < %d)", i, token.End, token.Pos)
		}
	}
}

func TestErrorCases(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "unterminated string",
			input: `"hello`,
			want:  "unterminated string",
		},
		{
			name:  "unterminated raw string",
			input: "`hello",
			want:  "unterminated raw string",
		},
		{
			name:  "unterminated block comment",
			input: "/* hello",
			want:  "unterminated block comment",
		},
		{
			name:  "unterminated char",
			input: "'a",
			want:  "unterminated char",
		},
		{
			name:  "string with newline",
			input: "\"hello\nworld\"",
			want:  "unterminated string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tok := New([]byte(tt.input))
			_, err := tok.Tokenize()
			if err == nil {
				t.Errorf("Tokenize(%q) should return error", tt.input)
			}
		})
	}
}

func TestNestedStructures(t *testing.T) {
	// Test deep nesting doesn't break position tracking
	src := []byte("match x { Ok(Some(Err(y))) => y }")
	tok := New(src)

	tokens, err := tok.Tokenize()
	if err != nil {
		t.Fatalf("Tokenize() error = %v", err)
	}

	// Verify all positions are monotonically increasing (except newlines/EOF)
	for i := 1; i < len(tokens); i++ {
		prev := tokens[i-1]
		curr := tokens[i]

		if curr.Kind == NEWLINE || curr.Kind == EOF {
			continue
		}

		if curr.Pos < prev.End && prev.Kind != NEWLINE {
			t.Errorf("token[%d] position (%d) < prev end (%d): %s < %s",
				i, curr.Pos, prev.End, curr, prev)
		}
	}
}
