package codegen

import (
	"strings"
	"testing"
)

// TestCanPropagateError verifies that CanPropagateError correctly identifies
// whether the enclosing function at a given position can propagate errors via ?.
// Returns (true, "") for functions with error/Result returns.
// Returns (false, funcName) for void functions or functions without error returns.
func TestCanPropagateError(t *testing.T) {
	tests := []struct {
		name         string
		src          string
		exprMarker   string // unique string to locate exprPos
		wantCanProp  bool
		wantFuncName string
	}{
		{
			name: "void function main - cannot propagate",
			src: `package main

func main() {
	x := doSomething() // MARKER
}`,
			exprMarker:   "MARKER",
			wantCanProp:  false,
			wantFuncName: "main",
		},
		{
			name: "void function init - cannot propagate",
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
			name: "returns (string, int, error) - can propagate",
			src: `package main

func fetchData() (string, int, error) {
	x := load() // MARKER
	return "", 0, nil
}`,
			exprMarker:  "MARKER",
			wantCanProp: true,
		},
		{
			name: "returns int only (no error) - cannot propagate",
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
			name: "method with void return - cannot propagate",
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
			if tt.wantCanProp && funcName != "" {
				t.Errorf("CanPropagateError() funcName should be empty for propagatable functions, got %q", funcName)
			}
		})
	}
}

// TestInferReturnTypesVoidFunction verifies that InferReturnTypes returns an empty
// slice (not ["nil"]) for void functions. This was the root cause of issue #44:
// the old behavior returned ["nil"] which caused invalid Go code generation.
func TestInferReturnTypesVoidFunction(t *testing.T) {
	tests := []struct {
		name       string
		src        string
		exprMarker string
		wantLen    int
	}{
		{
			name: "void main function",
			src: `package main

func main() {
	x := doSomething() // MARKER
}`,
			exprMarker: "MARKER",
			wantLen:    0,
		},
		{
			name: "void init function",
			src: `package main

func init() {
	setup() // MARKER
}`,
			exprMarker: "MARKER",
			wantLen:    0,
		},
		{
			name: "void custom function",
			src: `package main

func process() {
	handle() // MARKER
}`,
			exprMarker: "MARKER",
			wantLen:    0,
		},
		{
			name: "void method",
			src: `package main

func (s *Server) Run() {
	s.start() // MARKER
}`,
			exprMarker: "MARKER",
			wantLen:    0,
		},
		{
			name:       "error-returning function still works",
			src:        "package main\nfunc wrapper() error { doSomething() // MARKER\nreturn nil\n}",
			exprMarker: "MARKER",
			wantLen:    0, // error is the last return, so zero values list is empty (0 non-error returns)
		},
		{
			name:       "tuple-returning function still works",
			src:        "package main\nfunc fetch() (int, error) { x := get() // MARKER\nreturn 0, nil\n}",
			exprMarker: "MARKER",
			wantLen:    1, // one non-error return: int
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pos := strings.Index(tt.src, tt.exprMarker)
			if pos < 0 {
				t.Fatalf("marker %q not found in source", tt.exprMarker)
			}

			result := InferReturnTypes([]byte(tt.src), pos)
			if len(result) != tt.wantLen {
				t.Errorf("InferReturnTypes() returned %v (len=%d), want len=%d", result, len(result), tt.wantLen)
			}
		})
	}
}
