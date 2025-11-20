# Error Propagation

Error propagation is a language feature that simplifies error handling by automatically propagating errors up the call stack, similar to Rust's `?` operator, Swift's `try`, or Kotlin's exception handling.

## Overview

In Go, error handling follows this pattern:

```go
result, err := somethingThatCanFail()
if err != nil {
    return zeroValue, err
}
// Use result...
```

This becomes repetitive in functions with multiple error-prone operations. Dingo provides three syntax options for concise error propagation:

### Question Syntax (Default)

```dingo
func processUser(id: int) (User, error) {
    let user = fetchUser(id)?      // Propagate error if non-nil
    let profile = fetchProfile(user.ID)?
    return profile, nil
}
```

### Bang Syntax

```dingo
func processUser(id: int) (User, error) {
    let user = fetchUser(id)!      // Propagate error if non-nil
    let profile = fetchProfile(user.ID)!
    return profile, nil
}
```

### Try Keyword

```dingo
func processUser(id: int) (User, error) {
    let user = try fetchUser(id)   // Propagate error if non-nil
    let profile = try fetchProfile(user.ID)
    return profile, nil
}
```

## How It Works

All three syntaxes generate identical Go code:

```go
func processUser(id int) (User, error) {
    tmp1, err1 := fetchUser(id)
    if err1 != nil {
        return User{}, err1
    }
    user := tmp1

    tmp2, err2 := fetchProfile(user.ID)
    if err2 != nil {
        return User{}, err2
    }
    profile := tmp2

    return profile, nil
}
```

## Configuration

Choose your preferred syntax in `dingo.toml`:

```toml
[features]
error_propagation_syntax = "question"  # "question", "bang", or "try"
```

Or via CLI flag:

```bash
dingo build --syntax=bang main.dingo
```

## Requirements

The error propagation operator can only be used with expressions that return `(T, error)`:

```dingo
// ✅ Valid: returns (string, error)
let user = fetchUser(id)?

// ❌ Invalid: returns only string (no error)
let name = getName()?  // Compile error

// ❌ Invalid: returns (string, string) (not error)
let result = getSomething()?  // Compile error
```

## Real-World Examples

### HTTP Client

```dingo
import "net/http"
import "io"

func fetchURL(url: string) (string, error) {
    let resp = http.Get(url)?
    defer resp.Body.Close()

    let body = io.ReadAll(resp.Body)?
    return string(body), nil
}
```

### File Operations

```dingo
import "os"
import "encoding/json"

func loadConfig(path: string) (*Config, error) {
    let data = os.ReadFile(path)?

    var config Config
    let err = json.Unmarshal(data, &config)?
    return &config, nil
}
```

### Database Queries

```dingo
import "database/sql"

func queryUser(db: *sql.DB, id: int) (*User, error) {
    let row = db.QueryRow("SELECT name, email FROM users WHERE id = ?", id)

    var user User
    let err = row.Scan(&user.Name, &user.Email)?
    return &user, nil
}
```

## Comparison with Go

### Before (Go)

```go
func processTransaction(userID int, amount float64) error {
    user, err := fetchUser(userID)
    if err != nil {
        return fmt.Errorf("fetch user: %w", err)
    }

    account, err := getAccount(user.AccountID)
    if err != nil {
        return fmt.Errorf("get account: %w", err)
    }

    err = validateBalance(account, amount)
    if err != nil {
        return fmt.Errorf("validate balance: %w", err)
    }

    err = deductAmount(account, amount)
    if err != nil {
        return fmt.Errorf("deduct amount: %w", err)
    }

    return nil
}
```

### After (Dingo)

```dingo
func processTransaction(userID: int, amount: float64) error {
    let user = fetchUser(userID)?
    let account = getAccount(user.AccountID)?
    let _ = validateBalance(account, amount)?
    let _ = deductAmount(account, amount)?
    return nil
}
```

**Lines of code**: 22 → 6 (73% reduction)
**Error handling noise**: Eliminated

## Best Practices

### 1. Use for Straightforward Propagation

```dingo
// ✅ Good: Simple error propagation
let data = fetchData()?
let processed = processData(data)?
```

### 2. Add Context When Needed

For errors that need additional context, use traditional Go error wrapping:

```dingo
let user = fetchUser(id)?  // Simple propagation
// vs
user, err := fetchUser(id)
if err != nil {
    return fmt.Errorf("failed to fetch user %d: %w", id, err)
}
```

### 3. Combine with Traditional Error Handling

You can mix both styles in the same function:

```dingo
func process(id: int) error {
    let data = fetchData(id)?  // Simple propagation

    result, err := complexOperation(data)
    if err != nil {
        log.Error("Complex operation failed", err)
        return fmt.Errorf("operation failed: %w", err)
    }

    let _ = saveResult(result)?  // Back to simple propagation
    return nil
}
```

## Syntax Choice Guidelines

| Syntax | Best For | Precedent |
|--------|----------|-----------|
| `expr?` | Concise code, Rust developers | Rust, Kotlin, Swift |
| `expr!` | Distinguishing from ternary | Swift force unwrap |
| `try expr` | Clear intent, newcomers | Swift, Zig, C# |

**Recommendation**: Start with `?` (question) as it's the most widely adopted and concise.

## Limitations

- Only works with `(T, error)` return types
- Cannot customize error messages (use traditional error handling for that)
- Requires function to return `error` as last return value

## Future Enhancements

- Integration with `Result<T, E>` types
- Custom error context: `fetchUser(id)? wrap "user fetch failed"`
- Automatic error wrapping strategies

## See Also

- [Configuration Guide](../configuration.md)
- [Go Interoperability](../go-interop.md)
- [Result Types](./result-types.md) (planned)
