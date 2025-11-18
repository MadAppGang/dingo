package builtin

import (
	"testing"
)

func TestTupleExhaustivenessChecker_SimpleResultTuple(t *testing.T) {
	// Result<T,E> has 2 variants: Ok, Err
	// 2-element tuple: 2^2 = 4 possible patterns
	checker := NewTupleExhaustivenessChecker(
		2, // arity
		[]string{"Ok", "Err"},
		[][]string{
			{"Ok", "Ok"},
			{"Ok", "Err"},
			{"Err", "Ok"},
			{"Err", "Err"},
		},
	)

	exhaustive, missing, err := checker.Check()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !exhaustive {
		t.Errorf("expected exhaustive, got missing patterns: %v", missing)
	}
}

func TestTupleExhaustivenessChecker_NonExhaustive(t *testing.T) {
	// Missing (Err, Err) pattern
	checker := NewTupleExhaustivenessChecker(
		2,
		[]string{"Ok", "Err"},
		[][]string{
			{"Ok", "Ok"},
			{"Ok", "Err"},
			{"Err", "Ok"},
			// Missing: (Err, Err)
		},
	)

	exhaustive, missing, err := checker.Check()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if exhaustive {
		t.Errorf("expected non-exhaustive, got exhaustive")
	}

	if len(missing) != 1 {
		t.Errorf("expected 1 missing pattern, got %d: %v", len(missing), missing)
	}

	if missing[0] != "(Err, Err)" {
		t.Errorf("expected missing pattern (Err, Err), got %s", missing[0])
	}
}

func TestTupleExhaustivenessChecker_WildcardCatchAll(t *testing.T) {
	// Wildcard in any position makes match exhaustive
	checker := NewTupleExhaustivenessChecker(
		2,
		[]string{"Ok", "Err"},
		[][]string{
			{"Ok", "Ok"},
			{"Ok", "Err"},
			{"Err", "_"}, // Wildcard covers (Err, Ok) and (Err, Err)
		},
	)

	exhaustive, missing, err := checker.Check()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !exhaustive {
		t.Errorf("expected exhaustive with wildcard, got missing: %v", missing)
	}
}

func TestTupleExhaustivenessChecker_AllWildcard(t *testing.T) {
	// (_, _) covers all patterns
	checker := NewTupleExhaustivenessChecker(
		2,
		[]string{"Ok", "Err"},
		[][]string{
			{"_", "_"}, // All-wildcard covers everything
		},
	)

	exhaustive, missing, err := checker.Check()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !exhaustive {
		t.Errorf("expected exhaustive with all-wildcard, got missing: %v", missing)
	}
}

func TestTupleExhaustivenessChecker_ThreeElements(t *testing.T) {
	// 3-element Result tuple: 2^3 = 8 patterns
	checker := NewTupleExhaustivenessChecker(
		3,
		[]string{"Ok", "Err"},
		[][]string{
			{"Ok", "Ok", "Ok"},
			{"Ok", "Ok", "Err"},
			{"Ok", "Err", "Ok"},
			{"Ok", "Err", "Err"},
			{"Err", "Ok", "Ok"},
			{"Err", "Ok", "Err"},
			{"Err", "Err", "Ok"},
			{"Err", "Err", "Err"},
		},
	)

	exhaustive, missing, err := checker.Check()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !exhaustive {
		t.Errorf("expected exhaustive, got missing: %v", missing)
	}
}

func TestTupleExhaustivenessChecker_ThreeElementsWithWildcard(t *testing.T) {
	// Using wildcards strategically
	checker := NewTupleExhaustivenessChecker(
		3,
		[]string{"Ok", "Err"},
		[][]string{
			{"Ok", "Ok", "Ok"},
			{"Ok", "Ok", "Err"},
			{"Ok", "Err", "_"}, // Covers 2 patterns
			{"Err", "_", "_"},  // Covers 4 patterns
		},
	)

	exhaustive, missing, err := checker.Check()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !exhaustive {
		t.Errorf("expected exhaustive with wildcards, got missing: %v", missing)
	}
}

func TestTupleExhaustivenessChecker_SixElements(t *testing.T) {
	// 6-element tuple (max allowed): 2^6 = 64 patterns
	// Use all-wildcard to make it exhaustive without listing all 64
	checker := NewTupleExhaustivenessChecker(
		6,
		[]string{"Ok", "Err"},
		[][]string{
			{"Ok", "Ok", "Ok", "Ok", "Ok", "Ok"},
			{"_", "_", "_", "_", "_", "_"}, // Catch-all
		},
	)

	exhaustive, missing, err := checker.Check()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !exhaustive {
		t.Errorf("expected exhaustive, got missing: %v", missing)
	}
}

func TestTupleExhaustivenessChecker_ArityMismatch(t *testing.T) {
	// Pattern with wrong arity should error
	checker := NewTupleExhaustivenessChecker(
		2,
		[]string{"Ok", "Err"},
		[][]string{
			{"Ok", "Ok"},
			{"Ok", "Err", "Extra"}, // Wrong arity!
		},
	)

	_, _, err := checker.Check()
	if err == nil {
		t.Errorf("expected arity mismatch error, got nil")
	}
}

func TestTupleExhaustivenessChecker_PartialWildcard(t *testing.T) {
	// Wildcard in second position only
	checker := NewTupleExhaustivenessChecker(
		2,
		[]string{"Ok", "Err"},
		[][]string{
			{"Ok", "_"}, // Covers (Ok, Ok) and (Ok, Err)
			{"Err", "Ok"},
			{"Err", "Err"},
		},
	)

	exhaustive, missing, err := checker.Check()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !exhaustive {
		t.Errorf("expected exhaustive, got missing: %v", missing)
	}
}

func TestTupleExhaustivenessChecker_OptionType(t *testing.T) {
	// Option<T> has 2 variants: Some, None
	// Similar to Result
	checker := NewTupleExhaustivenessChecker(
		2,
		[]string{"Some", "None"},
		[][]string{
			{"Some", "Some"},
			{"Some", "None"},
			{"None", "_"}, // Wildcard covers rest
		},
	)

	exhaustive, missing, err := checker.Check()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !exhaustive {
		t.Errorf("expected exhaustive, got missing: %v", missing)
	}
}

func TestParseTuplePatterns_Simple(t *testing.T) {
	input := "(Ok, Ok) | (Ok, Err) | (Err, _)"
	patterns, err := ParseTuplePatterns(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := [][]string{
		{"Ok", "Ok"},
		{"Ok", "Err"},
		{"Err", "_"},
	}

	if len(patterns) != len(expected) {
		t.Errorf("expected %d patterns, got %d", len(expected), len(patterns))
	}

	for i, pattern := range patterns {
		if len(pattern) != len(expected[i]) {
			t.Errorf("pattern %d: expected length %d, got %d", i, len(expected[i]), len(pattern))
		}
		for j, elem := range pattern {
			if elem != expected[i][j] {
				t.Errorf("pattern %d, element %d: expected %s, got %s", i, j, expected[i][j], elem)
			}
		}
	}
}

func TestParseTuplePatterns_ThreeElements(t *testing.T) {
	input := "(Ok, Ok, Ok) | (Ok, Err, _) | (Err, _, _)"
	patterns, err := ParseTuplePatterns(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(patterns) != 3 {
		t.Errorf("expected 3 patterns, got %d", len(patterns))
	}

	// Check first pattern
	if len(patterns[0]) != 3 {
		t.Errorf("expected 3 elements in first pattern, got %d", len(patterns[0]))
	}
}

func TestParseArityFromMarker_WithArity(t *testing.T) {
	marker := "(Ok, Ok) | (Ok, Err) | (Err, _) | ARITY: 2"
	arity, err := ParseArityFromMarker(marker)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if arity != 2 {
		t.Errorf("expected arity 2, got %d", arity)
	}
}

func TestParseArityFromMarker_InferFromPattern(t *testing.T) {
	marker := "(Ok, Ok, Ok) | (Ok, Err, _)"
	arity, err := ParseArityFromMarker(marker)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if arity != 3 {
		t.Errorf("expected arity 3, got %d", arity)
	}
}

func TestTupleExhaustivenessChecker_FourElements(t *testing.T) {
	// 4-element tuple: 2^4 = 16 patterns
	// Use strategic wildcards
	checker := NewTupleExhaustivenessChecker(
		4,
		[]string{"Ok", "Err"},
		[][]string{
			{"Ok", "Ok", "Ok", "Ok"},
			{"Ok", "Ok", "Ok", "Err"},
			{"Ok", "Ok", "Err", "_"},
			{"Ok", "Err", "_", "_"},
			{"Err", "_", "_", "_"},
		},
	)

	exhaustive, missing, err := checker.Check()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !exhaustive {
		t.Errorf("expected exhaustive with strategic wildcards, got missing: %v", missing)
	}
}

func TestTupleExhaustivenessChecker_NonExhaustive_MultipleMissing(t *testing.T) {
	// Missing multiple patterns
	checker := NewTupleExhaustivenessChecker(
		2,
		[]string{"Ok", "Err"},
		[][]string{
			{"Ok", "Ok"},
			// Missing: (Ok, Err), (Err, Ok), (Err, Err)
		},
	)

	exhaustive, missing, err := checker.Check()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if exhaustive {
		t.Errorf("expected non-exhaustive, got exhaustive")
	}

	if len(missing) != 3 {
		t.Errorf("expected 3 missing patterns, got %d: %v", len(missing), missing)
	}

	// Check that all expected patterns are in missing list
	expectedMissing := map[string]bool{
		"(Ok, Err)":  true,
		"(Err, Ok)":  true,
		"(Err, Err)": true,
	}

	for _, pattern := range missing {
		if !expectedMissing[pattern] {
			t.Errorf("unexpected missing pattern: %s", pattern)
		}
	}
}
