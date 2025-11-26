package generator

import (
	"strings"
	"testing"
)

func TestMarkerInjector_InjectMarkers(t *testing.T) {
	tests := []struct {
		name     string
		enabled  bool
		source   string
		expected string
	}{
		{
			name:    "disabled - no markers",
			enabled: false,
			source: `package main

func process() error {
	x, __err0 := fetchUser()
	if __err0 != nil {
		return __err0
	}
	return nil
}
`,
			expected: `package main

func process() error {
	x, __err0 := fetchUser()
	if __err0 != nil {
		return __err0
	}
	return nil
}
`,
		},
		{
			name:    "enabled - adds markers",
			enabled: true,
			source: `package main

func process() error {
	x, __err0 := fetchUser()
	if __err0 != nil {
		return __err0
	}
	return nil
}
`,
			expected: `package main

func process() error {
	x, __err0 := fetchUser()
	// dingo:s:1
	if __err0 != nil {
		return __err0
	}
	// dingo:e:1
	return nil
}
`,
		},
		{
			name:    "enabled - multiple error checks",
			enabled: true,
			source: `package main

func process() error {
	x, __err0 := fetchUser()
	if __err0 != nil {
		return __err0
	}
	y, __err1 := fetchPost()
	if __err1 != nil {
		return __err1
	}
	return nil
}
`,
			expected: `package main

func process() error {
	x, __err0 := fetchUser()
	// dingo:s:1
	if __err0 != nil {
		return __err0
	}
	// dingo:e:1
	y, __err1 := fetchPost()
	// dingo:s:1
	if __err1 != nil {
		return __err1
	}
	// dingo:e:1
	return nil
}
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			injector := NewMarkerInjector(tt.enabled)
			result, err := injector.InjectMarkers([]byte(tt.source))
			if err != nil {
				t.Fatalf("InjectMarkers() error = %v", err)
			}

			resultStr := string(result)
			if !equalIgnoringWhitespace(resultStr, tt.expected) {
				t.Errorf("InjectMarkers() result mismatch\nGot:\n%s\n\nExpected:\n%s", resultStr, tt.expected)
			}
		})
	}
}

// equalIgnoringWhitespace compares two strings while normalizing whitespace
func equalIgnoringWhitespace(a, b string) bool {
	return strings.TrimSpace(a) == strings.TrimSpace(b)
}
