# Dingo Language Support for VS Code

Syntax highlighting and language support for [Dingo](https://github.com/yourusername/dingo) - a modern meta-language for Go with Result types, error propagation, pattern matching, and more.

## Features

### Syntax Highlighting
- **Dingo language features**:
  - `Result<T, E>` and `Option<T>` types
  - `?` error propagation operator
  - `match` pattern matching expressions
  - Lambda functions with `|params| expr` syntax
  - Enums and sum types
  - All standard Go syntax

- **Bracket Matching** for `{}`, `[]`, `()`, and `<>`
- **Auto-closing Pairs** for brackets, quotes, and strings
- **Comment Support** with `//` and `/* */`
- **Code Folding** for regions and blocks

### Generated Code Highlighting (NEW in v0.2.0)
- **Visual highlighting** of transpiler-generated code in `.go` files
- **Marker detection** for `DINGO:GENERATED:START/END` blocks
- **Configurable styles**:
  - Subtle: Light background (default)
  - Bold: Background + border
  - Outline: Border only
  - Disabled: No highlighting
- **Theme-aware colors** that work in light and dark modes
- **Real-time updates** with debounced performance
- **Support for `.go.golden` test files**

### Enhanced Dingo Syntax (NEW in v0.2.0)
- **Error messages**: Special highlighting for `expr? "custom message"` syntax
- **Generated variables**: Muted colors for `__err0`, `__tmp0` variables
- **Result/Option types**: Improved highlighting for `Result<T,E>` and `Option<T>`
- **Constructors**: Distinct colors for `Ok()`, `Err()`, `Some()`, `None()`
- **Error propagation**: More visually distinct `?` operator

### Golden File Support (NEW in v0.2.0)
- **Side-by-side comparison**: Compare `.dingo` files with `.go.golden` test files
- **Keyboard shortcut**: `Ctrl+Shift+D` (or `Cmd+Shift+D` on Mac)
- **Syntax highlighting**: `.go.golden` files get full Dingo syntax support

### Commands
- `Dingo: Toggle Generated Code Highlighting` - Quickly enable/disable highlighting
- `Dingo: Compare with Source File` - Open side-by-side diff view (keyboard: `Ctrl+Shift+D`)

## Installation

### From Marketplace (Coming Soon)

Search for "Dingo" in the VS Code Extensions marketplace.

### Manual Installation

1. Clone the Dingo repository:
   ```bash
   git clone https://github.com/yourusername/dingo.git
   ```

2. Copy the extension to your VS Code extensions folder:
   ```bash
   cp -r dingo/editors/vscode ~/.vscode/extensions/dingo-language
   ```

3. Reload VS Code

### Development

To work on the extension:

1. Install dependencies:
   ```bash
   cd editors/vscode
   npm install
   ```

2. Compile TypeScript:
   ```bash
   npm run compile
   # Or for watch mode:
   npm run watch
   ```

3. Test in VS Code:
   - Open the `editors/vscode` folder in VS Code
   - Press `F5` to launch the Extension Development Host
   - Open a `.dingo` or `.go` file to see highlighting

## Syntax Examples

### Result Type
```dingo
func fetchUser(id: string) -> Result<User, error> {
    if id == "" {
        return Err(errors.New("invalid ID"))
    }
    return Ok(user)
}
```

### Error Propagation
```dingo
func processUser(id: string) -> Result<User, error> {
    let user = fetchUser(id)?  // Automatically unwrap or return error
    return Ok(user)
}
```

### Pattern Matching
```dingo
match fetchUser(id) {
    Ok(user) => println("Found: ${user.name}")
    Err(error) => println("Error: ${error}")
}
```

### Lambdas
```dingo
let numbers = []int{1, 2, 3, 4, 5}
let evens = numbers.filter(|n| n % 2 == 0)
let doubled = evens.map(|n| n * 2)
```

## Configuration

The extension provides several settings to customize generated code highlighting:

### `dingo.highlightGeneratedCode`
- **Type**: boolean
- **Default**: `true`
- **Description**: Enable or disable highlighting of generated code sections

### `dingo.generatedCodeStyle`
- **Type**: `"subtle"` | `"bold"` | `"outline"` | `"disabled"`
- **Default**: `"subtle"`
- **Description**: Visual style for generated code highlighting
  - `subtle`: Light background only (recommended)
  - `bold`: Background color with border
  - `outline`: Border outline only
  - `disabled`: No highlighting

### `dingo.generatedCodeColor`
- **Type**: string (hex color)
- **Default**: `"#3b82f620"`
- **Description**: Background color for generated code (hex with alpha channel)

### `dingo.generatedCodeBorderColor`
- **Type**: string (hex color)
- **Default**: `"#3b82f660"`
- **Description**: Border color for bold/outline styles

## Building from Source

```bash
cd editors/vscode
npm install
npm run compile
npm run build-grammar
npm run package
```

This creates a `.vsix` file that can be installed via:
```bash
code --install-extension dingo-0.2.0.vsix
```

## Maintaining the Extension

When adding new Dingo language features:

1. **Update the grammar**: Edit `syntaxes/dingo.tmLanguage.json`
2. **Add examples**: Create test files in `examples/`
3. **Test**: Use the Scope Inspector (`Developer: Inspect Editor Tokens and Scopes`)
4. **Document**: Update this README and version in `package.json`

### Grammar Structure

The grammar is organized into sections:
- `keywords`: Control flow, declarations, modifiers
- `result-type`: Result<T, E>, Ok(), Err()
- `option-type`: Option<T>, Some(), None
- `enum-variants`: Pattern matching variants
- `types`: Built-in and user-defined types
- `functions`: Function declarations and calls
- `lambdas`: Lambda/arrow function syntax
- `operators`: All operators including `?` and `??`
- `strings`: String literals with interpolation
- `numbers`: Integer, float, hex, binary, octal
- `constants`: true, false, nil, iota
- `attributes`: #[attribute] syntax

### Adding a New Feature

Example: Adding ternary operator support

1. Read `features/ternary-operator.md` to understand the syntax
2. Add pattern to `syntaxes/dingo.tmLanguage.json`:
   ```json
   "ternary": {
     "patterns": [
       {
         "name": "keyword.operator.ternary.dingo",
         "match": "\\?|:"
       }
     ]
   }
   ```
3. Include in main patterns: `{ "include": "#ternary" }`
4. Create `examples/ternary.dingo` with test cases
5. Test in VS Code with Scope Inspector
6. Commit with descriptive message

## Contributing

See the main [Dingo repository](https://github.com/yourusername/dingo) for contribution guidelines.

## License

Same as Dingo project (see root LICENSE file).

## Resources

- [Dingo Documentation](https://github.com/yourusername/dingo)
- [VS Code Language Extension Guide](https://code.visualstudio.com/api/language-extensions/overview)
- [TextMate Grammar Guide](https://macromates.com/manual/en/language_grammars)
