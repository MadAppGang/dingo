package preprocessor

import (
	"strings"
	"testing"
)

// TestSwiftMatchProcessor_BasicParsing tests basic Swift syntax parsing
func TestSwiftMatchProcessor_BasicParsing(t *testing.T) {
	processor := NewSwiftMatchProcessor()

	input := `switch result {
case .Ok(let x):
    handleOk(x)
case .Err(let e):
    handleErr(e)
}`

	output, mappings, err := processor.Process([]byte(input))
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	outputStr := string(output)

	// Verify DINGO_MATCH_START marker
	if !strings.Contains(outputStr, "// DINGO_MATCH_START: result") {
		t.Errorf("Missing DINGO_MATCH_START marker")
	}

	// Verify DINGO_MATCH_END marker
	if !strings.Contains(outputStr, "// DINGO_MATCH_END") {
		t.Errorf("Missing DINGO_MATCH_END marker")
	}

	// Verify case tags
	if !strings.Contains(outputStr, "case ResultTagOk:") {
		t.Errorf("Missing ResultTagOk case")
	}
	if !strings.Contains(outputStr, "case ResultTagErr:") {
		t.Errorf("Missing ResultTagErr case")
	}

	// Verify DINGO_PATTERN markers
	if !strings.Contains(outputStr, "// DINGO_PATTERN: Ok(x)") {
		t.Errorf("Missing DINGO_PATTERN for Ok(x)")
	}
	if !strings.Contains(outputStr, "// DINGO_PATTERN: Err(e)") {
		t.Errorf("Missing DINGO_PATTERN for Err(e)")
	}

	// Verify bindings
	if !strings.Contains(outputStr, "x := *__match_0.ok_0") {
		t.Errorf("Missing Ok binding extraction")
	}
	if !strings.Contains(outputStr, "e := __match_0.err_0") {
		t.Errorf("Missing Err binding extraction")
	}

	// Verify mappings generated
	if len(mappings) == 0 {
		t.Errorf("No mappings generated")
	}
}

// TestSwiftMatchProcessor_WhereGuards tests 'where' guard keyword
func TestSwiftMatchProcessor_WhereGuards(t *testing.T) {
	processor := NewSwiftMatchProcessor()

	input := `switch result {
case .Ok(let x) where x > 0:
    handlePositive(x)
case .Ok(let x):
    handleNonPositive(x)
case .Err(let e):
    handleError(e)
}`

	output, _, err := processor.Process([]byte(input))
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	outputStr := string(output)

	// Verify DINGO_GUARD marker with 'where' guard
	if !strings.Contains(outputStr, "// DINGO_PATTERN: Ok(x) | DINGO_GUARD: x > 0") {
		t.Errorf("Missing DINGO_GUARD marker for 'where' guard. Output:\n%s", outputStr)
	}

	// Verify second Ok case without guard
	lines := strings.Split(outputStr, "\n")
	okPatternCount := 0
	for _, line := range lines {
		if strings.Contains(line, "// DINGO_PATTERN: Ok(x)") {
			okPatternCount++
		}
	}
	if okPatternCount != 2 {
		t.Errorf("Expected 2 Ok patterns (one with guard, one without), got %d", okPatternCount)
	}
}

// TestSwiftMatchProcessor_IfGuards tests 'if' guard keyword (Rust-style)
func TestSwiftMatchProcessor_IfGuards(t *testing.T) {
	processor := NewSwiftMatchProcessor()

	input := `switch result {
case .Ok(let x) if x > 0:
    handlePositive(x)
case .Err(let e):
    handleError(e)
}`

	output, _, err := processor.Process([]byte(input))
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	outputStr := string(output)

	// Verify DINGO_GUARD marker with 'if' guard
	if !strings.Contains(outputStr, "// DINGO_PATTERN: Ok(x) | DINGO_GUARD: x > 0") {
		t.Errorf("Missing DINGO_GUARD marker for 'if' guard. Output:\n%s", outputStr)
	}
}

// TestSwiftMatchProcessor_BothGuardKeywords tests both 'if' and 'where' in same switch
func TestSwiftMatchProcessor_BothGuardKeywords(t *testing.T) {
	processor := NewSwiftMatchProcessor()

	input := `switch result {
case .Ok(let x) if x > 0:
    handlePositive(x)
case .Ok(let x) where x < 0:
    handleNegative(x)
case .Ok(let x):
    handleZero(x)
case .Err(let e):
    handleError(e)
}`

	output, _, err := processor.Process([]byte(input))
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	outputStr := string(output)

	// Verify both guard styles work
	if !strings.Contains(outputStr, "// DINGO_PATTERN: Ok(x) | DINGO_GUARD: x > 0") {
		t.Errorf("Missing DINGO_GUARD marker for 'if' guard")
	}
	if !strings.Contains(outputStr, "// DINGO_PATTERN: Ok(x) | DINGO_GUARD: x < 0") {
		t.Errorf("Missing DINGO_GUARD marker for 'where' guard")
	}
}

// TestSwiftMatchProcessor_ComplexGuards tests complex guard expressions
func TestSwiftMatchProcessor_ComplexGuards(t *testing.T) {
	processor := NewSwiftMatchProcessor()

	input := `switch result {
case .Ok(let x) where x > 0 && x < 100:
    handleRange(x)
case .Err(let e) where e != nil:
    handleError(e)
}`

	output, _, err := processor.Process([]byte(input))
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	outputStr := string(output)

	// Verify complex guard expressions preserved
	if !strings.Contains(outputStr, "// DINGO_PATTERN: Ok(x) | DINGO_GUARD: x > 0 && x < 100") {
		t.Errorf("Complex guard expression not preserved")
	}
}

// TestSwiftMatchProcessor_BareStatements tests case bodies without braces
func TestSwiftMatchProcessor_BareStatements(t *testing.T) {
	processor := NewSwiftMatchProcessor()

	input := `switch result {
case .Ok(let x):
    return x * 2
case .Err(let e):
    return 0
}`

	output, _, err := processor.Process([]byte(input))
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	outputStr := string(output)

	// Verify bare statements preserved
	if !strings.Contains(outputStr, "return x * 2") {
		t.Errorf("Bare statement not preserved")
	}
	if !strings.Contains(outputStr, "return 0") {
		t.Errorf("Bare statement not preserved")
	}
}

// TestSwiftMatchProcessor_BracedBodies tests case bodies with braces
func TestSwiftMatchProcessor_BracedBodies(t *testing.T) {
	processor := NewSwiftMatchProcessor()

	input := `switch result {
case .Ok(let x): {
    log("Success")
    return x
}
case .Err(let e): {
    log("Error")
    return 0
}
}`

	output, _, err := processor.Process([]byte(input))
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	outputStr := string(output)

	// Verify braced bodies preserved
	if !strings.Contains(outputStr, `log("Success")`) {
		t.Errorf("Braced body statement not preserved")
	}
}

// TestSwiftMatchProcessor_OptionType tests Option<T> patterns
func TestSwiftMatchProcessor_OptionType(t *testing.T) {
	processor := NewSwiftMatchProcessor()

	input := `switch option {
case .Some(let v):
    handleValue(v)
case .None:
    handleNone()
}`

	output, _, err := processor.Process([]byte(input))
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	outputStr := string(output)

	// Verify Option type tags
	if !strings.Contains(outputStr, "case OptionTagSome:") {
		t.Errorf("Missing OptionTagSome case")
	}
	if !strings.Contains(outputStr, "case OptionTagNone:") {
		t.Errorf("Missing OptionTagNone case")
	}

	// Verify Some binding
	if !strings.Contains(outputStr, "v := *__match_0.some_0") {
		t.Errorf("Missing Some binding extraction")
	}
}

// TestSwiftMatchProcessor_NoBindingPattern tests patterns without bindings
func TestSwiftMatchProcessor_NoBindingPattern(t *testing.T) {
	processor := NewSwiftMatchProcessor()

	input := `switch option {
case .Some(let v):
    handleValue(v)
case .None:
    handleNone()
}`

	output, _, err := processor.Process([]byte(input))
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	outputStr := string(output)

	// Verify None case has no binding
	lines := strings.Split(outputStr, "\n")
	foundNoneCase := false
	for i, line := range lines {
		if strings.Contains(line, "case OptionTagNone:") {
			foundNoneCase = true
			// Next line should be DINGO_PATTERN marker
			if i+1 < len(lines) {
				nextLine := lines[i+1]
				if strings.Contains(nextLine, "// DINGO_PATTERN: None") {
					// Check there's no binding extraction
					if i+2 < len(lines) {
						lineAfterMarker := lines[i+2]
						if strings.Contains(lineAfterMarker, ":=") {
							t.Errorf("None case should not have binding extraction")
						}
					}
				}
			}
		}
	}

	if !foundNoneCase {
		t.Errorf("None case not found")
	}
}

// TestSwiftMatchProcessor_RustEquivalence tests Swift and Rust generate identical markers
func TestSwiftMatchProcessor_RustEquivalence(t *testing.T) {
	swiftProc := NewSwiftMatchProcessor()
	rustProc := NewRustMatchProcessor()

	// Swift input
	swiftInput := `switch result {
case .Ok(let x):
    return x * 2
case .Err(let e):
    return 0
}`

	// Rust input (equivalent)
	rustInput := `match result {
    Ok(x) => return x * 2,
    Err(e) => return 0
}`

	swiftOutput, _, err := swiftProc.Process([]byte(swiftInput))
	if err != nil {
		t.Fatalf("Swift process failed: %v", err)
	}

	rustOutput, _, err := rustProc.Process([]byte(rustInput))
	if err != nil {
		t.Fatalf("Rust process failed: %v", err)
	}

	// Extract markers from both outputs
	swiftMarkers := extractDingoMarkers(string(swiftOutput))
	rustMarkers := extractDingoMarkers(string(rustOutput))

	// Verify markers are identical
	if len(swiftMarkers) != len(rustMarkers) {
		t.Errorf("Marker count mismatch: Swift=%d, Rust=%d", len(swiftMarkers), len(rustMarkers))
	}

	for i := 0; i < len(swiftMarkers) && i < len(rustMarkers); i++ {
		if swiftMarkers[i] != rustMarkers[i] {
			t.Errorf("Marker %d mismatch:\nSwift: %s\nRust:  %s", i, swiftMarkers[i], rustMarkers[i])
		}
	}
}

// extractDingoMarkers extracts all DINGO_* markers from output
func extractDingoMarkers(output string) []string {
	markers := []string{}
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "DINGO_") {
			markers = append(markers, strings.TrimSpace(line))
		}
	}
	return markers
}

// TestSwiftMatchProcessor_PassThrough tests non-switch lines pass through
func TestSwiftMatchProcessor_PassThrough(t *testing.T) {
	processor := NewSwiftMatchProcessor()

	input := `let x = 10
switch result {
case .Ok(let y):
    return y
}
let z = 20`

	output, _, err := processor.Process([]byte(input))
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	outputStr := string(output)

	// Verify non-switch lines preserved
	if !strings.Contains(outputStr, "let x = 10") {
		t.Errorf("Line before switch not preserved")
	}
	if !strings.Contains(outputStr, "let z = 20") {
		t.Errorf("Line after switch not preserved")
	}
}

// TestSwiftMatchProcessor_Name tests processor name
func TestSwiftMatchProcessor_Name(t *testing.T) {
	processor := NewSwiftMatchProcessor()
	if processor.Name() != "swift_match" {
		t.Errorf("Expected name 'swift_match', got '%s'", processor.Name())
	}
}

// TestSwiftMatchProcessor_GetNeededImports tests import provider interface
func TestSwiftMatchProcessor_GetNeededImports(t *testing.T) {
	processor := NewSwiftMatchProcessor()
	imports := processor.GetNeededImports()
	if len(imports) != 0 {
		t.Errorf("Expected no imports, got %v", imports)
	}
}
