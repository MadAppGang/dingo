package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/MadAppGang/dingo/pkg/preprocessor"
)

func main() {
	// Load source map
	data, err := os.ReadFile("tests/golden/error_prop_01_simple.go.map")
	if err != nil {
		panic(err)
	}

	var sm preprocessor.SourceMap
	if err := json.Unmarshal(data, &sm); err != nil {
		panic(err)
	}

	fmt.Printf("Source map has %d mappings\n", len(sm.Mappings))
	for i, m := range sm.Mappings {
		fmt.Printf("Mapping %d: orig_line=%d, gen_line=%d, name=%s\n",
			i, m.OriginalLine, m.GeneratedLine, m.Name)
	}

	// Test MapToGenerated with line=3, col=12 (1-based, as translator would call it)
	fmt.Println("\n--- Testing MapToGenerated(3, 12) ---")
	newLine, newCol := sm.MapToGenerated(3, 12)
	fmt.Printf("Result: line=%d, col=%d\n", newLine, newCol)

	fmt.Println("\n--- Testing MapToGenerated(4, 20) ---")
	newLine, newCol = sm.MapToGenerated(4, 20)
	fmt.Printf("Result: line=%d, col=%d\n", newLine, newCol)
}
