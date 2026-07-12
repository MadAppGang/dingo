package transpiler

import (
	"strings"
	"testing"
)

func TestLambdaInferenceFromGenericRangeVariable(t *testing.T) {
	source := []byte(`package main

import "github.com/MadAppGang/dingo/pkg/dgo"

type Product struct {
	Name     string
	Category string
}

func main() {
	products := []Product{{Name: "Laptop", Category: "Electronics"}}
	byCategory := dgo.GroupBy(products, |p| p.Category)
	for _, prods := range byCategory {
		names := dgo.Map(prods, |p| p.Name)
		_ = names
	}
	names := dgo.Map(
		dgo.Filter(products, |p| p.Category != ""),
		|p| p.Name,
	)
	_ = names
}
`)

	result, err := PureASTTranspile(source, "range_lambda.dingo")
	if err != nil {
		t.Fatalf("transpile failed: %v", err)
	}

	generated := string(result)
	if got := strings.Count(generated, "func(p Product) string"); got != 3 {
		t.Fatalf("expected three lambdas to infer func(p Product) string, got %d:\n%s", got, generated)
	}
	if !strings.Contains(generated, "func(p Product) bool") {
		t.Fatalf("expected nested filter lambda to infer func(p Product) bool:\n%s", generated)
	}
}
