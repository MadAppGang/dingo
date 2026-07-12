package main

import (
	"testing"
)

func TestIsWorkspacePattern(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "triple dots only",
			input:    "...",
			expected: true,
		},
		{
			name:     "dot slash triple dots",
			input:    "./...",
			expected: true,
		},
		{
			name:     "path with triple dots",
			input:    "./tests/golden/...",
			expected: true,
		},
		{
			name:     "path without dot slash",
			input:    "tests/golden/...",
			expected: true,
		},
		{
			name:     "pkg pattern",
			input:    "./pkg/...",
			expected: true,
		},
		{
			name:     "regular file path",
			input:    "./tests/golden",
			expected: false,
		},
		{
			name:     "file with extension",
			input:    "tests/golden/test.dingo",
			expected: false,
		},
		{
			name:     "glob pattern",
			input:    "tests/golden/*.dingo",
			expected: false,
		},
		{
			name:     "single file",
			input:    "main.dingo",
			expected: false,
		},
		{
			name:     "dots in middle",
			input:    "test.../file.dingo",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isWorkspacePattern(tt.input)
			if result != tt.expected {
				t.Errorf("isWorkspacePattern(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestShouldUseFancyOutput(t *testing.T) {
	tests := []struct {
		name     string
		isTTY    bool
		noMascot bool
		want     bool
	}{
		{name: "interactive default", isTTY: true, want: true},
		{name: "interactive no mascot", isTTY: true, noMascot: true, want: false},
		{name: "non interactive default", isTTY: false, want: false},
		{name: "non interactive no mascot", isTTY: false, noMascot: true, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldUseFancyOutput(tt.isTTY, tt.noMascot); got != tt.want {
				t.Fatalf("shouldUseFancyOutput(%v, %v) = %v, want %v", tt.isTTY, tt.noMascot, got, tt.want)
			}
		})
	}
}
