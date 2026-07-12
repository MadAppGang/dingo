package transpiler

import "testing"

func TestGroupedVarCompositeLiterals(t *testing.T) {
	tests := map[string]string{
		"slice": `package main
var (
	values = []string{"hello", "world"}
)
`,
		"map": `package main
var (
	values = map[string]int{"one": 1, "two": 2}
)
`,
		"struct": `package main
type pair struct{ left, right int }
var (
	value = pair{1, 2}
)
`,
	}

	for name, source := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := PureASTTranspile([]byte(source), name+".dingo"); err != nil {
				t.Fatalf("transpile grouped var with %s literal: %v", name, err)
			}
		})
	}
}
