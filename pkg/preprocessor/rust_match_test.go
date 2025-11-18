package preprocessor

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRustMatchProcessor_SimpleResult(t *testing.T) {
	processor := NewRustMatchProcessor()

	input := `match result {
    Ok(x) => x * 2,
    Err(e) => 0
}`

	output, mappings, err := processor.Process([]byte(input))
	if err != nil {
		t.Fatalf("Process() error: %v", err)
	}

	result := string(output)

	// Check for key components
	expected := []string{
		"__match_0 := result",
		"// DINGO_MATCH_START: result",
		"switch __match_0.tag {",
		"case ResultTagOk:",
		"// DINGO_PATTERN: Ok(x)",
		"x := *__match_0.ok_0",
		"x * 2",
		"case ResultTagErr:",
		"// DINGO_PATTERN: Err(e)",
		"e := __match_0.err_0",
		"// DINGO_MATCH_END",
	}

	for _, exp := range expected {
		if !strings.Contains(result, exp) {
			t.Errorf("Expected output to contain %q, but it didn't.\nGot:\n%s", exp, result)
		}
	}

	// Check mappings were created
	if len(mappings) == 0 {
		t.Error("Expected mappings to be created, got none")
	}
}

func TestRustMatchProcessor_SimpleOption(t *testing.T) {
	processor := NewRustMatchProcessor()

	input := `match value {
    Some(v) => v,
    None => "default"
}`

	output, mappings, err := processor.Process([]byte(input))
	if err != nil {
		t.Fatalf("Process() error: %v", err)
	}

	result := string(output)

	// Check for key components
	expected := []string{
		"__match_0 := value",
		"// DINGO_MATCH_START: value",
		"switch __match_0.tag {",
		"case OptionTagSome:",
		"// DINGO_PATTERN: Some(v)",
		"v := *__match_0.some_0",
		"case OptionTagNone:",
		"// DINGO_PATTERN: None",
		"\t\"default\"",
		"// DINGO_MATCH_END",
	}

	for _, exp := range expected {
		if !strings.Contains(result, exp) {
			t.Errorf("Expected output to contain %q, but it didn't.\nGot:\n%s", exp, result)
		}
	}

	// Check mappings were created
	if len(mappings) == 0 {
		t.Error("Expected mappings to be created, got none")
	}
}

func TestRustMatchProcessor_Wildcard(t *testing.T) {
	processor := NewRustMatchProcessor()

	input := `match status {
    Active => "active",
    _ => "other"
}`

	output, _, err := processor.Process([]byte(input))
	if err != nil {
		t.Fatalf("Process() error: %v", err)
	}

	result := string(output)

	// Check for wildcard handling
	expected := []string{
		"case ActiveTag:",
		"// DINGO_PATTERN: Active",
		"default:",
		"// DINGO_PATTERN: _",
		"\t\"other\"",
	}

	for _, exp := range expected {
		if !strings.Contains(result, exp) {
			t.Errorf("Expected output to contain %q, but it didn't.\nGot:\n%s", exp, result)
		}
	}
}

func TestRustMatchProcessor_MultipleMatches(t *testing.T) {
	processor := NewRustMatchProcessor()

	input := `match result1 {
    Ok(x) => x,
    Err(e) => 0
}
match result2 {
    Ok(y) => y,
    Err(e) => 1
}`

	output, _, err := processor.Process([]byte(input))
	if err != nil {
		t.Fatalf("Process() error: %v", err)
	}

	result := string(output)

	// Check that both matches are processed with different counter values
	if !strings.Contains(result, "__match_0") {
		t.Error("Expected first match to use __match_0")
	}
	if !strings.Contains(result, "__match_1") {
		t.Error("Expected second match to use __match_1")
	}
}

func TestRustMatchProcessor_NoMatch(t *testing.T) {
	processor := NewRustMatchProcessor()

	input := `let x = 42
return x`

	output, mappings, err := processor.Process([]byte(input))
	if err != nil {
		t.Fatalf("Process() error: %v", err)
	}

	result := string(output)

	// Should pass through unchanged
	if result != input {
		t.Errorf("Expected unchanged output for non-match code.\nExpected:\n%s\nGot:\n%s", input, result)
	}

	// Should have no mappings
	if len(mappings) != 0 {
		t.Errorf("Expected no mappings for non-match code, got %d", len(mappings))
	}
}

func TestRustMatchProcessor_MultilineMatch(t *testing.T) {
	processor := NewRustMatchProcessor()

	input := `match result {
    Ok(user) => {
        println(user.name)
        return user.id
    },
    Err(e) => {
        log(e)
        return 0
    }
}`

	output, _, err := processor.Process([]byte(input))
	if err != nil {
		t.Fatalf("Process() error: %v", err)
	}

	result := string(output)

	// Check basic structure is present
	if !strings.Contains(result, "__match_0 := result") {
		t.Error("Expected match transformation")
	}
	if !strings.Contains(result, "case ResultTagOk:") {
		t.Error("Expected Ok case")
	}
	if !strings.Contains(result, "case ResultTagErr:") {
		t.Error("Expected Err case")
	}
}

func TestRustMatchProcessor_ComplexExpression(t *testing.T) {
	processor := NewRustMatchProcessor()

	input := `match fetchUser(id) {
    Ok(user) => user.name,
    Err(e) => "unknown"
}`

	output, _, err := processor.Process([]byte(input))
	if err != nil {
		t.Fatalf("Process() error: %v", err)
	}

	result := string(output)

	// Check that scrutinee is preserved
	if !strings.Contains(result, "__match_0 := fetchUser(id)") {
		t.Errorf("Expected scrutinee to be preserved.\nGot:\n%s", result)
	}
}

func TestRustMatchProcessor_GetNeededImports(t *testing.T) {
	processor := NewRustMatchProcessor()

	imports := processor.GetNeededImports()

	// Rust match doesn't require additional imports
	if len(imports) != 0 {
		t.Errorf("Expected no imports, got %v", imports)
	}
}

func TestRustMatchProcessor_Name(t *testing.T) {
	processor := NewRustMatchProcessor()

	name := processor.Name()
	expected := "rust_match"

	if name != expected {
		t.Errorf("Expected name %q, got %q", expected, name)
	}
}

func TestRustMatchProcessor_ParseArms(t *testing.T) {
	processor := NewRustMatchProcessor()

	tests := []struct {
		name        string
		input       string
		wantCount   int
		wantFirst   string
		wantBinding string
	}{
		{
			name:        "simple patterns",
			input:       "Ok(x) => x * 2, Err(e) => 0",
			wantCount:   2,
			wantFirst:   "Ok",
			wantBinding: "x",
		},
		{
			name:        "no binding",
			input:       "None => \"default\"",
			wantCount:   1,
			wantFirst:   "None",
			wantBinding: "",
		},
		{
			name:        "wildcard",
			input:       "_ => fallback",
			wantCount:   1,
			wantFirst:   "_",
			wantBinding: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			arms, err := processor.parseArms(tt.input)
			if err != nil {
				t.Fatalf("parseArms() error: %v", err)
			}

			if len(arms) != tt.wantCount {
				t.Errorf("Expected %d arms, got %d", tt.wantCount, len(arms))
			}

			if len(arms) > 0 {
				if arms[0].pattern != tt.wantFirst {
					t.Errorf("Expected first pattern %q, got %q", tt.wantFirst, arms[0].pattern)
				}
				if arms[0].binding != tt.wantBinding {
					t.Errorf("Expected first binding %q, got %q", tt.wantBinding, arms[0].binding)
				}
			}
		})
	}
}

func TestRustMatchProcessor_GetTagName(t *testing.T) {
	processor := NewRustMatchProcessor()

	tests := []struct {
		pattern string
		want    string
	}{
		{"Ok", "ResultTagOk"},
		{"Err", "ResultTagErr"},
		{"Some", "OptionTagSome"},
		{"None", "OptionTagNone"},
		{"Active", "ActiveTag"},
		{"Pending", "PendingTag"},
	}

	for _, tt := range tests {
		t.Run(tt.pattern, func(t *testing.T) {
			got := processor.getTagName(tt.pattern)
			if got != tt.want {
				t.Errorf("getTagName(%q) = %q, want %q", tt.pattern, got, tt.want)
			}
		})
	}
}

func TestRustMatchProcessor_GenerateBinding(t *testing.T) {
	processor := NewRustMatchProcessor()

	tests := []struct {
		pattern string
		binding string
		want    string
	}{
		{"Ok", "x", "x := *__match_0.ok_0"},
		{"Err", "e", "e := __match_0.err_0"},
		{"Some", "v", "v := *__match_0.some_0"},
		{"Active", "id", "id := __match_0.active_0"},
	}

	for _, tt := range tests {
		t.Run(tt.pattern, func(t *testing.T) {
			got := processor.generateBinding("__match_0", tt.pattern, tt.binding)
			if got != tt.want {
				t.Errorf("generateBinding(%q, %q) = %q, want %q", tt.pattern, tt.binding, got, tt.want)
			}
		})
	}
}
// ========================================================================
// Guard Pattern Tests (Phase 4.2 - Task A1)
// ========================================================================

func TestRustMatchProcessor_SplitPatternAndGuard_If(t *testing.T) {
	processor := NewRustMatchProcessor()

	tests := []struct {
		name         string
		input        string
		wantPattern  string
		wantGuard    string
	}{
		{
			name:        "simple if guard",
			input:       "Ok(x) if x > 0",
			wantPattern: "Ok(x)",
			wantGuard:   "x > 0",
		},
		{
			name:        "complex if guard",
			input:       "Ok(x) if x > 0 && x < 100",
			wantPattern: "Ok(x)",
			wantGuard:   "x > 0 && x < 100",
		},
		{
			name:        "if guard with function call",
			input:       "Ok(x) if isValid(x)",
			wantPattern: "Ok(x)",
			wantGuard:   "isValid(x)",
		},
		{
			name:        "no guard",
			input:       "Ok(x)",
			wantPattern: "Ok(x)",
			wantGuard:   "",
		},
		{
			name:        "None pattern no guard",
			input:       "None",
			wantPattern: "None",
			wantGuard:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pattern, guard := processor.splitPatternAndGuard(tt.input)
			assert.Equal(t, tt.wantPattern, pattern, "pattern mismatch")
			assert.Equal(t, tt.wantGuard, guard, "guard mismatch")
		})
	}
}

func TestRustMatchProcessor_SplitPatternAndGuard_Where(t *testing.T) {
	processor := NewRustMatchProcessor()

	tests := []struct {
		name         string
		input        string
		wantPattern  string
		wantGuard    string
	}{
		{
			name:        "simple where guard",
			input:       "Ok(x) where x > 0",
			wantPattern: "Ok(x)",
			wantGuard:   "x > 0",
		},
		{
			name:        "complex where guard",
			input:       "Ok(x) where x > 0 && x < 100",
			wantPattern: "Ok(x)",
			wantGuard:   "x > 0 && x < 100",
		},
		{
			name:        "where guard with function call",
			input:       "Ok(x) where isValid(x)",
			wantPattern: "Ok(x)",
			wantGuard:   "isValid(x)",
		},
		{
			name:        "where guard with nested conditions",
			input:       "Ok(user) where user.age > 18 && user.verified",
			wantPattern: "Ok(user)",
			wantGuard:   "user.age > 18 && user.verified",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pattern, guard := processor.splitPatternAndGuard(tt.input)
			assert.Equal(t, tt.wantPattern, pattern, "pattern mismatch")
			assert.Equal(t, tt.wantGuard, guard, "guard mismatch")
		})
	}
}

func TestRustMatchProcessor_GuardParsing_If(t *testing.T) {
	processor := NewRustMatchProcessor()

	input := `match result {
    Ok(x) if x > 0 => x * 2,
    Ok(x) => 0,
    Err(e) => -1
}`

	output, _, err := processor.Process([]byte(input))
	if err != nil {
		t.Fatalf("Process() error: %v", err)
	}

	result := string(output)

	// Check for guard marker in output
	expected := []string{
		"// DINGO_PATTERN: Ok(x) | DINGO_GUARD: x > 0",
		"// DINGO_PATTERN: Ok(x)",  // Second Ok arm without guard
		"// DINGO_PATTERN: Err(e)",
	}

	for _, exp := range expected {
		if !strings.Contains(result, exp) {
			t.Errorf("Expected output to contain %q, but it didn't.\nGot:\n%s", exp, result)
		}
	}
}

func TestRustMatchProcessor_GuardParsing_Where(t *testing.T) {
	processor := NewRustMatchProcessor()

	input := `match result {
    Ok(x) where x > 0 => x * 2,
    Ok(x) => 0,
    Err(e) => -1
}`

	output, _, err := processor.Process([]byte(input))
	if err != nil {
		t.Fatalf("Process() error: %v", err)
	}

	result := string(output)

	// Check for guard marker in output (should be same as 'if')
	expected := []string{
		"// DINGO_PATTERN: Ok(x) | DINGO_GUARD: x > 0",
		"// DINGO_PATTERN: Ok(x)",  // Second Ok arm without guard
		"// DINGO_PATTERN: Err(e)",
	}

	for _, exp := range expected {
		if !strings.Contains(result, exp) {
			t.Errorf("Expected output to contain %q, but it didn't.\nGot:\n%s", exp, result)
		}
	}
}

func TestRustMatchProcessor_MultipleGuards(t *testing.T) {
	processor := NewRustMatchProcessor()

	input := `match value {
    Ok(x) if x > 0 => "positive",
    Ok(x) if x < 0 => "negative",
    Ok(x) => "zero",
    Err(e) => "error"
}`

	output, _, err := processor.Process([]byte(input))
	if err != nil {
		t.Fatalf("Process() error: %v", err)
	}

	result := string(output)

	// Check that all guards are captured
	expected := []string{
		"// DINGO_PATTERN: Ok(x) | DINGO_GUARD: x > 0",
		"// DINGO_PATTERN: Ok(x) | DINGO_GUARD: x < 0",
		"// DINGO_PATTERN: Ok(x)",  // Third Ok arm without guard
	}

	for _, exp := range expected {
		if !strings.Contains(result, exp) {
			t.Errorf("Expected output to contain %q, but it didn't.\nGot:\n%s", exp, result)
		}
	}
}

func TestRustMatchProcessor_ComplexGuardExpressions(t *testing.T) {
	processor := NewRustMatchProcessor()

	input := `match result {
    Ok(user) if user.age > 18 && user.verified => processUser(user),
    Ok(user) where len(user.name) > 0 => createUser(user),
    Err(e) if e != nil => handleError(e)
}`

	output, _, err := processor.Process([]byte(input))
	if err != nil {
		t.Fatalf("Process() error: %v", err)
	}

	result := string(output)

	// Check complex guards are preserved correctly
	expected := []string{
		"// DINGO_PATTERN: Ok(user) | DINGO_GUARD: user.age > 18 && user.verified",
		"// DINGO_PATTERN: Ok(user) | DINGO_GUARD: len(user.name) > 0",
		"// DINGO_PATTERN: Err(e) | DINGO_GUARD: e != nil",
	}

	for _, exp := range expected {
		if !strings.Contains(result, exp) {
			t.Errorf("Expected output to contain %q, but it didn't.\nGot:\n%s", exp, result)
		}
	}
}

func TestRustMatchProcessor_GuardWithBlockExpression(t *testing.T) {
	processor := NewRustMatchProcessor()

	input := `match result {
    Ok(x) if x > 0 => {
        log("positive")
        return x * 2
    },
    Ok(x) => 0
}`

	output, _, err := processor.Process([]byte(input))
	if err != nil {
		t.Fatalf("Process() error: %v", err)
	}

	result := string(output)

	// Check guard with block expression
	if !strings.Contains(result, "// DINGO_PATTERN: Ok(x) | DINGO_GUARD: x > 0") {
		t.Errorf("Expected guard marker, got:\n%s", result)
	}
}

func TestRustMatchProcessor_ParseArmsWithGuards(t *testing.T) {
	processor := NewRustMatchProcessor()

	tests := []struct {
		name        string
		input       string
		wantCount   int
		checkArm    int
		wantPattern string
		wantBinding string
		wantGuard   string
	}{
		{
			name:        "if guard",
			input:       "Ok(x) if x > 0 => x * 2",
			wantCount:   1,
			checkArm:    0,
			wantPattern: "Ok",
			wantBinding: "x",
			wantGuard:   "x > 0",
		},
		{
			name:        "where guard",
			input:       "Ok(x) where x > 0 => x * 2",
			wantCount:   1,
			checkArm:    0,
			wantPattern: "Ok",
			wantBinding: "x",
			wantGuard:   "x > 0",
		},
		{
			name:        "no guard",
			input:       "Ok(x) => x * 2",
			wantCount:   1,
			checkArm:    0,
			wantPattern: "Ok",
			wantBinding: "x",
			wantGuard:   "",
		},
		{
			name:        "multiple arms with guards",
			input:       "Ok(x) if x > 0 => x * 2, Ok(x) => 0",
			wantCount:   2,
			checkArm:    0,
			wantPattern: "Ok",
			wantBinding: "x",
			wantGuard:   "x > 0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			arms, err := processor.parseArms(tt.input)
			if err != nil {
				t.Fatalf("parseArms() error: %v", err)
			}

			assert.Equal(t, tt.wantCount, len(arms), "arm count mismatch")

			if tt.checkArm < len(arms) {
				arm := arms[tt.checkArm]
				assert.Equal(t, tt.wantPattern, arm.pattern, "pattern mismatch")
				assert.Equal(t, tt.wantBinding, arm.binding, "binding mismatch")
				assert.Equal(t, tt.wantGuard, arm.guard, "guard mismatch")
			}
		})
	}
}

func TestRustMatchProcessor_BothIfAndWhere(t *testing.T) {
	processor := NewRustMatchProcessor()

	// Test that both 'if' and 'where' can be used in same match
	input := `match result {
    Ok(x) if x > 0 => x * 2,
    Ok(x) where x < 0 => x * -1,
    Ok(x) => 0
}`

	output, _, err := processor.Process([]byte(input))
	if err != nil {
		t.Fatalf("Process() error: %v", err)
	}

	result := string(output)

	// Both should generate guard markers
	expected := []string{
		"// DINGO_PATTERN: Ok(x) | DINGO_GUARD: x > 0",
		"// DINGO_PATTERN: Ok(x) | DINGO_GUARD: x < 0",
	}

	for _, exp := range expected {
		if !strings.Contains(result, exp) {
			t.Errorf("Expected output to contain %q, but it didn't.\nGot:\n%s", exp, result)
		}
	}
}
