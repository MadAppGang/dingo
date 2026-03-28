package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MadAppGang/dingo/pkg/transpiler"
)

// goldenTestCases lists the .dingo base names (without extension) that are
// managed by TestGolden.  Only files in this list are run so that stale golden
// files for other features do not pollute the output.  Add new entries here when
// introducing a new golden test.
var goldenTestCases = []string{
	"error_prop_named_return",
}

// TestGolden runs golden file tests for the cases listed in goldenTestCases.
// Each entry must have a matching .dingo source file and a .go.golden expected
// output in tests/golden/.  The test fails when the transpiler output differs
// from the golden file, making it the TDD RED step for any regression it covers.
func TestGolden(t *testing.T) {
	goldenDir := filepath.Join("golden")

	for _, base := range goldenTestCases {
		dingoFile := filepath.Join(goldenDir, base+".dingo")
		goldenFile := filepath.Join(goldenDir, base+".go.golden")

		if _, err := os.Stat(dingoFile); os.IsNotExist(err) {
			t.Errorf("missing dingo source file: %s", dingoFile)
			continue
		}
		if _, err := os.Stat(goldenFile); os.IsNotExist(err) {
			t.Errorf("missing golden file: %s", goldenFile)
			continue
		}

		base := base // capture
		t.Run(base, func(t *testing.T) {
			runGoldenTest(t, dingoFile, goldenFile)
		})
	}
}

func runGoldenTest(t *testing.T, dingoFile, goldenFile string) {
	t.Helper()

	src, err := os.ReadFile(dingoFile)
	if err != nil {
		t.Fatalf("cannot read dingo source %s: %v", dingoFile, err)
	}

	expected, err := os.ReadFile(goldenFile)
	if err != nil {
		t.Fatalf("cannot read golden file %s: %v", goldenFile, err)
	}

	got, transpileErr := transpiler.PureASTTranspile(src, filepath.Base(dingoFile))
	if transpileErr != nil {
		t.Fatalf("transpile failed: %v", transpileErr)
	}

	// Normalise line endings before comparing.
	gotStr := normaliseLineEndings(string(got))
	wantStr := normaliseLineEndings(string(expected))

	if gotStr != wantStr {
		t.Errorf("transpiler output does not match golden file %s\n\n--- want ---\n%s\n--- got ---\n%s\n--- diff (first mismatch) ---\n%s",
			goldenFile, wantStr, gotStr, firstDiff(wantStr, gotStr))
	}
}

// normaliseLineEndings converts \r\n to \n and trims a trailing newline so
// comparisons are not thrown off by editor/OS differences.
func normaliseLineEndings(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.TrimRight(s, "\n")
}

// firstDiff returns a short human-readable description of the first line that
// differs between want and got so the test failure is easy to diagnose.
func firstDiff(want, got string) string {
	wantLines := strings.Split(want, "\n")
	gotLines := strings.Split(got, "\n")

	maxLen := len(wantLines)
	if len(gotLines) < maxLen {
		maxLen = len(gotLines)
	}

	for i := 0; i < maxLen; i++ {
		if wantLines[i] != gotLines[i] {
			return "line " + itoa(i+1) + ":\n  want: " + wantLines[i] + "\n   got: " + gotLines[i]
		}
	}

	if len(wantLines) != len(gotLines) {
		return "line count differs: want " + itoa(len(wantLines)) + ", got " + itoa(len(gotLines))
	}

	return "(no difference found — check whitespace)"
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	result := ""
	for n > 0 {
		result = string(rune('0'+n%10)) + result
		n /= 10
	}
	return result
}
