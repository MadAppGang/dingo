# Dingo

A meta-language for Go that reduces boilerplate while maintaining full compatibility with the Go ecosystem and tooling.

## What is Dingo?

Dingo is a TypeScript-for-Go approach: a language that transpiles to idiomatic Go code while providing enhanced error handling, null safety, and modern language features. Like TypeScript, Dingo compiles to standard Go with zero runtime overhead.

## Core Philosophy

- **Zero Runtime Overhead**: Transpiles to clean, idiomatic Go code
- **Full Go Compatibility**: Works seamlessly with existing Go packages and tools
- **IDE-First Design**: Maintains full gopls integration
- **Simplicity Over Complexity**: Only adds features that meaningfully reduce boilerplate

## Architecture

**Two-Component System:**

1. **Transpiler** (`dingo build`) - Converts `.dingo` files to `.go` files with source maps
2. **Language Server** (`dingo-lsp`) - Wraps gopls to provide full IDE support

## Planned Features

- **Enhanced Error Handling**: `Result<T, E>` type and `?` operator
- **Null Safety**: `Option<T>` type
- **Pattern Matching**: Exhaustive `match` expressions
- **Sum Types**: Algebraic data types via `enum`

## Example

```dingo
fn fetchUser(id: int) Result<User, Error> {
    data := api.get("/users/" + id)?
    user := parseUser(data)?
    Ok(user)
}
```

Transpiles to clean, idiomatic Go:

```go
func fetchUser(id int) (User, error) {
    data, err := api.get("/users/" + id)
    if err != nil {
        return User{}, err
    }
    user, err := parseUser(data)
    if err != nil {
        return User{}, err
    }
    return user, nil
}
```

## Status

**Current Stage: Research Complete → Moving to Implementation**

We've completed comprehensive research on meta-language architecture, Go AST manipulation, and language server design. Next phase: building the core transpiler.

## Roadmap

1. **Phase 1**: Core Transpiler (3 months)
2. **Phase 2**: Source Mapping (1 month)
3. **Phase 3**: Basic Language Server (2 months)
4. **Phase 4**: IDE Features (3 months)
5. **Phase 5**: Enhanced Features (2 months)
6. **Phase 6**: Polish & Release (2 months)

**Estimated Timeline**: 12-15 months to v1.0

## Inspirations

- **TypeScript**: Meta-language architecture and tooling
- **Borgo**: Rust-inspired syntax transpiling to Go
- **templ**: gopls proxy pattern
- **Goa**: Production code generation

## Why Dingo?

Go is excellent, but has pain points:
- Verbose error handling
- Nil pointer risks
- Lack of sum types and pattern matching

Dingo addresses these while maintaining Go's simplicity and ecosystem compatibility.

## License

TBD
