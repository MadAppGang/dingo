package ast

import (
	"strings"
	"testing"
)

func TestIsValueEnum(t *testing.T) {
	tests := []struct {
		name   string
		src    string
		expect bool
	}{
		{
			name:   "int value enum",
			src:    "enum Status: int { Pending, Active }",
			expect: true,
		},
		{
			name:   "string value enum",
			src:    `enum Key: string { UserID = "user_id" }`,
			expect: true,
		},
		{
			name:   "sum type enum",
			src:    "enum Result { Ok(T), Err(E) }",
			expect: false,
		},
		{
			name:   "sum type with struct variant",
			src:    "enum Color { RGB { r: int, g: int, b: int } }",
			expect: false,
		},
		{
			name:   "byte value enum",
			src:    "enum Flags: byte { Read, Write }",
			expect: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsValueEnum([]byte(tt.src))
			if got != tt.expect {
				t.Errorf("IsValueEnum(%q) = %v, want %v", tt.src, got, tt.expect)
			}
		})
	}
}

func TestParseValueEnumDecl(t *testing.T) {
	tests := []struct {
		name          string
		src           string
		expectName    string
		expectType    string
		expectCount   int
		expectVariant string
		expectValue   string
		expectError   bool
	}{
		{
			name:          "int enum with iota",
			src:           "enum Status: int { Pending, Active, Closed }",
			expectName:    "Status",
			expectType:    "int",
			expectCount:   3,
			expectVariant: "Pending",
			expectValue:   "",
		},
		{
			name:          "string enum with values",
			src:           `enum Key: string { UserID = "user_id", CompanyID = "company_id" }`,
			expectName:    "Key",
			expectType:    "string",
			expectCount:   2,
			expectVariant: "UserID",
			expectValue:   `"user_id"`,
		},
		{
			name:          "byte enum",
			src:           "enum Flags: byte { Read = 1, Write = 2, Execute = 4 }",
			expectName:    "Flags",
			expectType:    "byte",
			expectCount:   3,
			expectVariant: "Read",
			expectValue:   "1",
		},
		{
			name:        "invalid base type",
			src:         "enum Invalid: float64 { A }",
			expectError: true,
		},
		{
			name:        "string without value",
			src:         `enum Key: string { MissingValue }`,
			expectError: true,
		},
		{
			name:        "duplicate variants",
			src:         "enum Status: int { Active, Active }",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewValueEnumParser([]byte(tt.src), 0)
			decl, _, err := parser.ParseValueEnumDecl()

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if decl.Name.Name != tt.expectName {
				t.Errorf("name = %q, want %q", decl.Name.Name, tt.expectName)
			}

			if decl.BaseType.Text != tt.expectType {
				t.Errorf("base type = %q, want %q", decl.BaseType.Text, tt.expectType)
			}

			if len(decl.Variants) != tt.expectCount {
				t.Errorf("variant count = %d, want %d", len(decl.Variants), tt.expectCount)
			}

			if len(decl.Variants) > 0 && decl.Variants[0].Name.Name != tt.expectVariant {
				t.Errorf("first variant = %q, want %q", decl.Variants[0].Name.Name, tt.expectVariant)
			}

			if tt.expectValue != "" && len(decl.Variants) > 0 {
				if decl.Variants[0].Value == nil {
					t.Errorf("first variant value is nil, want %q", tt.expectValue)
				} else if decl.Variants[0].Value.String() != tt.expectValue {
					t.Errorf("first variant value = %q, want %q", decl.Variants[0].Value.String(), tt.expectValue)
				}
			}
		})
	}
}

func TestValueEnumCodeGen(t *testing.T) {
	tests := []struct {
		name       string
		src        string
		contains   []string
		notContain []string
	}{
		{
			name: "int enum with iota",
			src:  "enum Status: int { Pending, Active, Closed }",
			contains: []string{
				"type Status int",
				"StatusPending Status = iota",
				"\tStatusActive\n", // No type on subsequent iota lines
				"\tStatusClosed\n",
				"const (",
			},
		},
		{
			name: "string enum with values",
			src:  `enum Key: string { UserID = "user_id", CompanyID = "company_id" }`,
			contains: []string{
				"type Key string",
				`KeyUserID Key = "user_id"`,
				`KeyCompanyID Key = "company_id"`,
			},
			notContain: []string{
				"iota",
			},
		},
		{
			name: "byte enum with explicit values",
			src:  "enum Flags: byte { Read = 1, Write = 2 }",
			contains: []string{
				"type Flags byte",
				"FlagsRead Flags = 1",
				"FlagsWrite Flags = 2",
			},
			notContain: []string{
				"iota",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewValueEnumParser([]byte(tt.src), 0)
			decl, _, err := parser.ParseValueEnumDecl()
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}

			codegen := NewValueEnumCodeGen()
			result := string(codegen.Generate(decl, "", 0, 0))

			for _, want := range tt.contains {
				if !strings.Contains(result, want) {
					t.Errorf("result missing %q\nGot:\n%s", want, result)
				}
			}

			for _, notWant := range tt.notContain {
				if strings.Contains(result, notWant) {
					t.Errorf("result should not contain %q\nGot:\n%s", notWant, result)
				}
			}
		})
	}
}

func TestValueEnumWithPrefix(t *testing.T) {
	// Test @prefix(false) attribute
	decl := &ValueEnumDecl{
		Name:     &Ident{Name: "Status"},
		BaseType: &TypeExpr{Text: "int"},
		Variants: []*ValueEnumVariant{
			{Name: &Ident{Name: "Pending"}},
			{Name: &Ident{Name: "Active"}},
		},
		Attributes: []*Attribute{
			{
				Name: &Ident{Name: "prefix"},
				Args: []Expr{&RawExpr{Text: "false"}},
			},
		},
	}

	codegen := NewValueEnumCodeGen()
	result := string(codegen.Generate(decl, "", 0, 0))

	// Should NOT have prefix
	if strings.Contains(result, "StatusPending") {
		t.Errorf("@prefix(false) should not use prefix, got:\n%s", result)
	}

	// Should have bare names (first one with type and iota)
	if !strings.Contains(result, "\tPending Status = iota\n") {
		t.Errorf("expected bare 'Pending Status = iota', got:\n%s", result)
	}

	// Second one should just be the bare name
	if !strings.Contains(result, "\tActive\n") {
		t.Errorf("expected bare 'Active', got:\n%s", result)
	}
}

func TestEnumRegistry(t *testing.T) {
	registry := NewEnumRegistry()

	// Register a sum type
	registry.RegisterSumTypeVariant("Ok", "Result")
	registry.RegisterSumTypeVariant("Err", "Result")

	// Register a value enum
	registry.RegisterValueEnum("Status", []string{"Pending", "Active"}, true)

	// Test sum type lookup
	if enum, ok := registry.IsSumTypeVariant("Ok"); !ok || enum != "Result" {
		t.Errorf("IsSumTypeVariant(Ok) = (%q, %v), want (Result, true)", enum, ok)
	}

	// Test value enum lookup
	info := registry.LookupVariant("Pending")
	if info == nil || info.EnumName != "Status" {
		t.Errorf("LookupVariant(Pending) = %v, want Status", info)
	}

	// Test prefixed lookup
	info = registry.LookupVariant("StatusPending")
	if info == nil || info.EnumName != "Status" {
		t.Errorf("LookupVariant(StatusPending) = %v, want Status", info)
	}

	// Test GetVariants
	variants := registry.GetVariants("Status")
	if len(variants) != 2 || variants[0] != "Pending" {
		t.Errorf("GetVariants(Status) = %v, want [Pending Active]", variants)
	}

	// Test legacy map
	legacy := registry.ToLegacyMap()
	if legacy["Ok"] != "Result" {
		t.Errorf("legacy[Ok] = %q, want Result", legacy["Ok"])
	}
	if legacy["Pending"] != "Status" {
		t.Errorf("legacy[Pending] = %q, want Status", legacy["Pending"])
	}
}

func TestTransformValueEnumSource(t *testing.T) {
	tests := []struct {
		name     string
		src      string
		contains []string
	}{
		{
			name: "int value enum",
			src:  "package main\n\nenum Status: int { Pending, Active }\n",
			contains: []string{
				"type Status int",
				"StatusPending Status = iota",
				"\tStatusActive\n", // Second line should just be the name
			},
		},
		{
			name: "string value enum",
			src:  "package main\n\nenum Key: string { UserID = \"user_id\" }\n",
			contains: []string{
				"type Key string",
				"KeyUserID Key =",
			},
		},
		{
			name: "sum type enum preserved",
			src:  "package main\n\nenum Option { Some(T), None }\n",
			contains: []string{
				"type Option interface",
				"isOption()",
			},
		},
		{
			name: "mixed enums",
			src:  "package main\n\nenum Status: int { Active }\nenum Result { Ok(T) }\n",
			contains: []string{
				"type Status int",
				"StatusActive Status = iota", // Single variant still gets full declaration
				"type Result interface",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, registry := TransformValueEnumSource([]byte(tt.src), "")

			for _, want := range tt.contains {
				if !strings.Contains(string(result), want) {
					t.Errorf("result missing %q\nGot:\n%s", want, string(result))
				}
			}

			if registry == nil {
				t.Error("registry should not be nil")
			}
		})
	}
}

func TestValidValueEnumBaseTypes(t *testing.T) {
	validTypes := []string{
		"string", "int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64", "byte", "rune",
	}

	for _, typ := range validTypes {
		if !isValidValueEnumBaseType(typ) {
			t.Errorf("isValidValueEnumBaseType(%q) = false, want true", typ)
		}
	}

	invalidTypes := []string{"float32", "float64", "complex64", "bool", "struct", "MyType"}
	for _, typ := range invalidTypes {
		if isValidValueEnumBaseType(typ) {
			t.Errorf("isValidValueEnumBaseType(%q) = true, want false", typ)
		}
	}
}

// =============================================================================
// Phase 2: Attribute Parsing Tests
// =============================================================================

func TestParseAttributes(t *testing.T) {
	tests := []struct {
		name       string
		src        string
		expectLen  int
		expectName string
		expectArgs int
		expectErr  bool
	}{
		{
			name:       "single attribute with false",
			src:        "@prefix(false)",
			expectLen:  1,
			expectName: "prefix",
			expectArgs: 1,
		},
		{
			name:       "single attribute with true",
			src:        "@prefix(true)",
			expectLen:  1,
			expectName: "prefix",
			expectArgs: 1,
		},
		{
			name:       "no attributes",
			src:        "enum Status: int { }",
			expectLen:  0,
			expectName: "",
			expectArgs: 0,
		},
		{
			name:       "attribute with whitespace",
			src:        "@prefix( false )",
			expectLen:  1,
			expectName: "prefix",
			expectArgs: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewValueEnumParser([]byte(tt.src), 0)
			attrs, err := parser.ParseAttributes()

			if tt.expectErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(attrs) != tt.expectLen {
				t.Errorf("attribute count = %d, want %d", len(attrs), tt.expectLen)
			}

			if tt.expectLen > 0 {
				if attrs[0].Name.Name != tt.expectName {
					t.Errorf("attribute name = %q, want %q", attrs[0].Name.Name, tt.expectName)
				}
				if len(attrs[0].Args) != tt.expectArgs {
					t.Errorf("attribute args count = %d, want %d", len(attrs[0].Args), tt.expectArgs)
				}
			}
		})
	}
}

func TestParseValueEnumWithAttributes(t *testing.T) {
	tests := []struct {
		name          string
		src           string
		expectName    string
		expectType    string
		expectAttrLen int
		expectPrefix  bool
		expectError   bool
	}{
		{
			name:          "prefix false attribute",
			src:           "@prefix(false)\nenum Status: int { Pending, Active }",
			expectName:    "Status",
			expectType:    "int",
			expectAttrLen: 1,
			expectPrefix:  false,
		},
		{
			name:          "prefix true attribute",
			src:           "@prefix(true)\nenum Status: int { Pending, Active }",
			expectName:    "Status",
			expectType:    "int",
			expectAttrLen: 1,
			expectPrefix:  true,
		},
		{
			name:          "no attribute",
			src:           "enum Status: int { Pending, Active }",
			expectName:    "Status",
			expectType:    "int",
			expectAttrLen: 0,
			expectPrefix:  true, // default
		},
		{
			name:          "attribute same line",
			src:           "@prefix(false) enum Status: int { Pending }",
			expectName:    "Status",
			expectType:    "int",
			expectAttrLen: 1,
			expectPrefix:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewValueEnumParser([]byte(tt.src), 0)
			decl, _, err := parser.ParseValueEnumWithAttributes()

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if decl.Name.Name != tt.expectName {
				t.Errorf("name = %q, want %q", decl.Name.Name, tt.expectName)
			}

			if decl.BaseType.Text != tt.expectType {
				t.Errorf("base type = %q, want %q", decl.BaseType.Text, tt.expectType)
			}

			if len(decl.Attributes) != tt.expectAttrLen {
				t.Errorf("attribute count = %d, want %d", len(decl.Attributes), tt.expectAttrLen)
			}

			// Validate prefix setting
			usePrefix, err := ValidatePrefixAttribute(decl.Attributes)
			if err != nil {
				t.Fatalf("ValidatePrefixAttribute error: %v", err)
			}
			if usePrefix != tt.expectPrefix {
				t.Errorf("usePrefix = %v, want %v", usePrefix, tt.expectPrefix)
			}
		})
	}
}

func TestValidatePrefixAttribute(t *testing.T) {
	tests := []struct {
		name         string
		attrs        []*Attribute
		expectPrefix bool
		expectError  bool
	}{
		{
			name:         "no attributes - default true",
			attrs:        nil,
			expectPrefix: true,
			expectError:  false,
		},
		{
			name: "prefix(false)",
			attrs: []*Attribute{
				{
					Name: &Ident{Name: "prefix"},
					Args: []Expr{&RawExpr{Text: "false"}},
				},
			},
			expectPrefix: false,
			expectError:  false,
		},
		{
			name: "prefix(true)",
			attrs: []*Attribute{
				{
					Name: &Ident{Name: "prefix"},
					Args: []Expr{&RawExpr{Text: "true"}},
				},
			},
			expectPrefix: true,
			expectError:  false,
		},
		{
			name: "prefix without argument - error",
			attrs: []*Attribute{
				{
					Name: &Ident{Name: "prefix"},
					Args: []Expr{},
				},
			},
			expectPrefix: false,
			expectError:  true,
		},
		{
			name: "prefix with string argument - error",
			attrs: []*Attribute{
				{
					Name: &Ident{Name: "prefix"},
					Args: []Expr{&RawExpr{Text: `"string"`}},
				},
			},
			expectPrefix: false,
			expectError:  true,
		},
		{
			name: "prefix with multiple args - error",
			attrs: []*Attribute{
				{
					Name: &Ident{Name: "prefix"},
					Args: []Expr{&RawExpr{Text: "true"}, &RawExpr{Text: "false"}},
				},
			},
			expectPrefix: false,
			expectError:  true,
		},
		{
			name: "other attribute ignored",
			attrs: []*Attribute{
				{
					Name: &Ident{Name: "deprecated"},
					Args: []Expr{},
				},
			},
			expectPrefix: true, // default when no @prefix
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			usePrefix, err := ValidatePrefixAttribute(tt.attrs)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if usePrefix != tt.expectPrefix {
				t.Errorf("usePrefix = %v, want %v", usePrefix, tt.expectPrefix)
			}
		})
	}
}

func TestTransformWithPrefixAttribute(t *testing.T) {
	tests := []struct {
		name       string
		src        string
		contains   []string
		notContain []string
	}{
		{
			name: "prefix(false) generates unprefixed const names",
			src:  "package main\n\n@prefix(false)\nenum Status: int { Pending, Active }\n",
			contains: []string{
				"type Status int",
				"\tPending Status = iota\n",
				"\tActive\n",
			},
			notContain: []string{
				"StatusPending",
				"StatusActive",
			},
		},
		{
			name: "prefix(true) generates prefixed const names",
			src:  "package main\n\n@prefix(true)\nenum Status: int { Pending, Active }\n",
			contains: []string{
				"type Status int",
				"StatusPending Status = iota",
				"\tStatusActive\n",
			},
			notContain: []string{
				"\tPending Status", // Should NOT have unprefixed
			},
		},
		{
			name: "no attribute generates prefixed (default)",
			src:  "package main\n\nenum Status: int { Pending, Active }\n",
			contains: []string{
				"type Status int",
				"StatusPending Status = iota",
				"\tStatusActive\n",
			},
		},
		{
			name: "prefix(false) same line",
			src:  "package main\n\n@prefix(false) enum Flags: byte { Read, Write }\n",
			contains: []string{
				"type Flags byte",
				"\tRead Flags = iota\n",
				"\tWrite\n",
			},
			notContain: []string{
				"FlagsRead",
				"FlagsWrite",
			},
		},
		{
			name: "string enum with prefix(false)",
			src:  "package main\n\n@prefix(false)\nenum Key: string { UserID = \"user_id\", CompanyID = \"company_id\" }\n",
			contains: []string{
				"type Key string",
				`UserID Key = "user_id"`,
				`CompanyID Key = "company_id"`,
			},
			notContain: []string{
				"KeyUserID",
				"KeyCompanyID",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, registry := TransformValueEnumSource([]byte(tt.src), "")

			resultStr := string(result)

			for _, want := range tt.contains {
				if !strings.Contains(resultStr, want) {
					t.Errorf("result missing %q\nGot:\n%s", want, resultStr)
				}
			}

			for _, notWant := range tt.notContain {
				if strings.Contains(resultStr, notWant) {
					t.Errorf("result should not contain %q\nGot:\n%s", notWant, resultStr)
				}
			}

			// Verify registry has correct UsePrefix value
			if registry != nil {
				// Check that enum was registered
				info := registry.LookupVariant("Pending")
				if info == nil {
					info = registry.LookupVariant("Read")
				}
				if info == nil {
					info = registry.LookupVariant("UserID")
				}
				// Only check if we expected to find a variant
				if info != nil && strings.Contains(tt.name, "prefix(false)") {
					if info.UsePrefix {
						t.Errorf("registry.UsePrefix = true, want false for @prefix(false)")
					}
				}
			}
		})
	}
}

func TestFindAttributeStart(t *testing.T) {
	tests := []struct {
		name            string
		src             string
		enumStart       int
		expectDeclStart int
	}{
		{
			name:            "no attribute",
			src:             "enum Status: int { }",
			enumStart:       0,
			expectDeclStart: 0,
		},
		{
			name:            "attribute on same line",
			src:             "@prefix(false) enum Status: int { }",
			enumStart:       15,
			expectDeclStart: 0,
		},
		{
			name:            "attribute on previous line",
			src:             "@prefix(false)\nenum Status: int { }",
			enumStart:       15,
			expectDeclStart: 0,
		},
		{
			name:            "attribute with spaces",
			src:             "@prefix(false)  enum Status: int { }",
			enumStart:       16,
			expectDeclStart: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			declStart := findAttributeStart([]byte(tt.src), tt.enumStart)
			if declStart != tt.expectDeclStart {
				t.Errorf("findAttributeStart() = %d, want %d", declStart, tt.expectDeclStart)
			}
		})
	}
}

func TestRegistryUsePrefixTracking(t *testing.T) {
	// Test that registry correctly tracks UsePrefix for value enums
	registry := NewEnumRegistry()

	// Register with prefix
	registry.RegisterValueEnum("Status", []string{"Pending", "Active"}, true)

	// Register without prefix
	registry.RegisterValueEnum("Flags", []string{"Read", "Write"}, false)

	// Check Status (with prefix)
	statusInfo := registry.LookupVariant("Pending")
	if statusInfo == nil {
		t.Fatal("expected to find Pending variant")
	}
	if !statusInfo.UsePrefix {
		t.Errorf("Status.UsePrefix = false, want true")
	}
	if statusInfo.EnumName != "Status" {
		t.Errorf("Status.EnumName = %q, want Status", statusInfo.EnumName)
	}

	// Prefixed lookup should also work
	prefixedInfo := registry.LookupVariant("StatusPending")
	if prefixedInfo == nil {
		t.Fatal("expected to find StatusPending variant")
	}
	if prefixedInfo.EnumName != "Status" {
		t.Errorf("StatusPending.EnumName = %q, want Status", prefixedInfo.EnumName)
	}

	// Check Flags (without prefix)
	flagsInfo := registry.LookupVariant("Read")
	if flagsInfo == nil {
		t.Fatal("expected to find Read variant")
	}
	if flagsInfo.UsePrefix {
		t.Errorf("Flags.UsePrefix = true, want false")
	}
	if flagsInfo.EnumName != "Flags" {
		t.Errorf("Flags.EnumName = %q, want Flags", flagsInfo.EnumName)
	}

	// With UsePrefix=false, prefixed lookup should NOT exist
	flagsPrefixedInfo := registry.LookupVariant("FlagsRead")
	if flagsPrefixedInfo != nil {
		t.Errorf("FlagsRead should not exist in registry when UsePrefix=false")
	}
}
