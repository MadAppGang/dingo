# Tuples Phase 8 - Known Limitations

## Nested Tuples Not Supported (Scope Reduction)

### Summary

Phase 8 tuples implementation **does not support nested tuples**. Attempting to use nested tuples will result in the code being passed through unchanged, which will then fail at the Go parser stage.

### What Doesn't Work

```dingo
// ❌ NOT SUPPORTED - Nested tuple literals
let matrix = ((1, 2), (3, 4))
let nested = ((user, error), (data, ok))

// ❌ NOT SUPPORTED - Nested tuples in function returns
func getRange() ((int, int), (int, int)) {
    return ((0, 10), (100, 200))
}
```

### What Works

```dingo
// ✅ SUPPORTED - Single-level tuples
let point = (1, 2, 3)
let user = (name, email, age)

// ✅ SUPPORTED - Tuples with function calls (not nested tuples)
let result = (foo(a), bar(b), baz(c))

// ✅ SUPPORTED - Single-level tuple destructuring
let (x, y, z) = getCoordinates()
```

### Technical Reason

The Phase 8 preprocessor uses line-by-line processing and detects tuple elements that are themselves tuples (start and end with balanced parentheses). When nested tuples are detected, they are silently ignored to prevent generating invalid Go code.

This design decision was made to ship a working subset of tuple functionality rather than shipping broken code that would fail unpredictably.

### Workaround

Flatten nested tuples into single-level tuples with more elements:

```dingo
// Instead of: let matrix = ((1, 2), (3, 4))
// Use: let matrix = (1, 2, 3, 4)  // Flatten to 4 elements

// Then destructure:
let (a, b, c, d) = matrix
// Treat (a,b) as first pair, (c,d) as second pair
```

### Future Plans

Nested tuple support will be added in a future release (likely v1.1) when the preprocessor is refactored to use balanced parenthesis scanning instead of line-by-line processing.

## Multi-Line Tuples Not Supported (Architectural Limitation)

### Summary

Tuples must be written on a single line. Multi-line tuple formatting is not supported.

### What Doesn't Work

```dingo
// ❌ NOT SUPPORTED - Multi-line tuple literals
let coordinates = (
    latitude: 37.7749,
    longitude: -122.4194,
    altitude: 52.0
)
```

### What Works

```dingo
// ✅ SUPPORTED - Single-line tuples
let coordinates = (37.7749, -122.4194, 52.0)
```

### Technical Reason

The Phase 8 preprocessor processes source code line-by-line. When a tuple spans multiple lines, each line is processed independently, and the preprocessor cannot detect that they form a single tuple literal.

### Workaround

Write all tuple literals on a single line:

```dingo
// Format on one line with clear spacing
let coordinates = (37.7749, -122.4194, 52.0)
```

### Future Plans

Multi-line tuple support requires refactoring the preprocessor to accumulate lines until parentheses balance. This will be addressed in a future release alongside nested tuple support.

---

**Status**: Phase 8 ships with single-level, single-line tuples only
**Timeline**: Nested and multi-line support planned for v1.1
**Impact**: Known limitation, documented and tested
