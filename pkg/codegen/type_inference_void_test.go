package codegen

import (
	"strings"
	"testing"
)

func TestCanPropagateError(t *testing.T) {
	tests := []struct {
		name         string
		src          string
		exprMarker   string
		wantCanProp  bool
		wantFuncName string
	}{
		{
			name: "void main - cannot propagate",
			src: `package main

func main() {
	x := doSomething() // MARKER
}`,
			exprMarker:   "MARKER",
			wantCanProp:  false,
			wantFuncName: "main",
		},
		{
			name: "void init - cannot propagate",
			src: `package main

func init() {
	setup() // MARKER
}`,
			exprMarker:   "MARKER",
			wantCanProp:  false,
			wantFuncName: "init",
		},
		{
			name: "void custom function - cannot propagate",
			src: `package main

func processItems() {
	handle() // MARKER
}`,
			exprMarker:   "MARKER",
			wantCanProp:  false,
			wantFuncName: "processItems",
		},
		{
			name: "returns error - can propagate",
			src: `package main

func wrapper() error {
	doSomething() // MARKER
	return nil
}`,
			exprMarker:  "MARKER",
			wantCanProp: true,
		},
		{
			name: "returns (int, error) - can propagate",
			src: `package main

func compute() (int, error) {
	val := calculate() // MARKER
	return val, nil
}`,
			exprMarker:  "MARKER",
			wantCanProp: true,
		},
		{
			name: "returns int only - cannot propagate",
			src: `package main

func getCount() int {
	n := count() // MARKER
	return n
}`,
			exprMarker:   "MARKER",
			wantCanProp:  false,
			wantFuncName: "getCount",
		},
		{
			name: "returns (int, string) no error - cannot propagate",
			src: `package main

func getData() (int, string) {
	x := fetch() // MARKER
	return 0, ""
}`,
			exprMarker:   "MARKER",
			wantCanProp:  false,
			wantFuncName: "getData",
		},
		{
			name: "returns Result[T, E] - can propagate",
			src: `package main

func loadConfig() Result[Config, error] {
	cfg := parse() // MARKER
	return cfg, nil
}`,
			exprMarker:  "MARKER",
			wantCanProp: true,
		},
		{
			name: "void method - cannot propagate",
			src: `package main

func (s *Server) Start() {
	s.init() // MARKER
}`,
			exprMarker:   "MARKER",
			wantCanProp:  false,
			wantFuncName: "Start",
		},
		{
			name: "method with error return - can propagate",
			src: `package main

func (s *Server) Start() error {
	s.init() // MARKER
	return nil
}`,
			exprMarker:  "MARKER",
			wantCanProp: true,
		},
		{
			name: "exprPos outside any function - conservative true",
			src: `package main
// MARKER
func main() {}`,
			exprMarker:  "MARKER",
			wantCanProp: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pos := strings.Index(tt.src, tt.exprMarker)
			if pos < 0 {
				t.Fatalf("marker %q not found in source", tt.exprMarker)
			}

			canProp, funcName := CanPropagateError([]byte(tt.src), pos)

			if canProp != tt.wantCanProp {
				t.Errorf("CanPropagateError() canPropagate = %v, want %v", canProp, tt.wantCanProp)
			}
			if !tt.wantCanProp && funcName != tt.wantFuncName {
				t.Errorf("CanPropagateError() funcName = %q, want %q", funcName, tt.wantFuncName)
			}
		})
	}
}
