package lsp

import (
	"testing"

	"github.com/MadAppGang/dingo/pkg/preprocessor"
	"go.lsp.dev/protocol"
	lspuri "go.lsp.dev/uri"
)

// BenchmarkPositionTranslation measures position translation performance
// Target: <1ms per translation (1M ops/sec)
func BenchmarkPositionTranslation(b *testing.B) {
	sm := &preprocessor.SourceMap{
		Version:   1,
		DingoFile: "test.dingo",
		GoFile:    "test.go",
		Mappings: []preprocessor.Mapping{
			{OriginalLine: 1, OriginalColumn: 1, GeneratedLine: 1, GeneratedColumn: 1},
			{OriginalLine: 5, OriginalColumn: 10, GeneratedLine: 12, GeneratedColumn: 15, Length: 3, Name: "error_prop"},
			{OriginalLine: 10, OriginalColumn: 5, GeneratedLine: 20, GeneratedColumn: 8},
			{OriginalLine: 15, OriginalColumn: 20, GeneratedLine: 30, GeneratedColumn: 25},
		},
	}

	translator := newMockTranslator(sm)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		translator.TranslatePosition(
			lspuri.File("test.dingo"),
			protocol.Position{Line: 4, Character: 9},
			DingoToGo,
		)
	}
}

// BenchmarkPositionTranslationRoundTrip measures full round-trip translation
// Target: <2ms per round-trip
func BenchmarkPositionTranslationRoundTrip(b *testing.B) {
	sm := &preprocessor.SourceMap{
		Version:   1,
		DingoFile: "test.dingo",
		GoFile:    "test.go",
		Mappings: []preprocessor.Mapping{
			{OriginalLine: 5, OriginalColumn: 10, GeneratedLine: 12, GeneratedColumn: 15, Length: 3, Name: "error_prop"},
		},
	}

	translator := newMockTranslator(sm)

	dingoURI := lspuri.File("test.dingo")
	dingoPos := protocol.Position{Line: 4, Character: 9}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Dingo → Go
		goURI, goPos, _ := translator.TranslatePosition(dingoURI, dingoPos, DingoToGo)
		// Go → Dingo
		translator.TranslatePosition(goURI, goPos, GoToDingo)
	}
}

// BenchmarkSourceMapCacheGet measures source map loading (cached)
// Target: <1μs per cached get
func BenchmarkSourceMapCacheGet(b *testing.B) {
	cache, _ := NewSourceMapCache(&testLogger{})

	// Pre-load into cache
	sm := &preprocessor.SourceMap{
		Version:   1,
		DingoFile: "test.dingo",
		GoFile:    "test.go",
		Mappings:  []preprocessor.Mapping{{OriginalLine: 1, OriginalColumn: 1, GeneratedLine: 1, GeneratedColumn: 1}},
	}
	cache.maps["test.go.map"] = sm

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Get("test.go")
	}
}

// BenchmarkSourceMapCacheGetConcurrent measures concurrent cache access
// Tests thread-safety performance with RWMutex
func BenchmarkSourceMapCacheGetConcurrent(b *testing.B) {
	cache, _ := NewSourceMapCache(&testLogger{})

	sm := &preprocessor.SourceMap{
		Version:   1,
		DingoFile: "test.dingo",
		GoFile:    "test.go",
		Mappings:  []preprocessor.Mapping{{OriginalLine: 1, OriginalColumn: 1, GeneratedLine: 1, GeneratedColumn: 1}},
	}
	cache.maps["test.go.map"] = sm

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			cache.Get("test.go")
		}
	})
}

// BenchmarkTranslateRange measures range translation performance
// Target: <2ms per range (2 position translations)
func BenchmarkTranslateRange(b *testing.B) {
	sm := &preprocessor.SourceMap{
		Version:   1,
		DingoFile: "test.dingo",
		GoFile:    "test.go",
		Mappings: []preprocessor.Mapping{
			{OriginalLine: 10, OriginalColumn: 15, GeneratedLine: 18, GeneratedColumn: 22, Length: 10},
		},
	}

	translator := newMockTranslator(sm)

	uri := lspuri.File("test.dingo")
	rng := protocol.Range{
		Start: protocol.Position{Line: 9, Character: 14},
		End:   protocol.Position{Line: 9, Character: 24},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		translator.TranslateRange(uri, rng, DingoToGo)
	}
}

// BenchmarkIsDingoFile measures file extension checking
// Target: <100ns (simple string check)
func BenchmarkIsDingoFile(b *testing.B) {
	uri := lspuri.File("/path/to/myfile.dingo")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		isDingoFile(uri)
	}
}

// BenchmarkDingoToGoPath measures path conversion
// Target: <100ns (simple string operation)
func BenchmarkDingoToGoPath(b *testing.B) {
	path := "/path/to/myfile.dingo"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dingoToGoPath(path)
	}
}

// BenchmarkGoToDingoPath measures reverse path conversion
// Target: <100ns (simple string operation)
func BenchmarkGoToDingoPath(b *testing.B) {
	path := "/path/to/myfile.go"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		goToDingoPath(path)
	}
}
