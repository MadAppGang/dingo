# Changelog

All notable changes to the Dingo VS Code extension will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.2.0] - 2025-11-16

### Added

#### Generated Code Highlighting
- Visual highlighting of transpiler-generated code in `.go` files
- Automatic detection of `DINGO:GENERATED:START/END` marker blocks
- Four configurable highlighting styles: subtle, bold, outline, and disabled
- Configurable background and border colors via settings
- Real-time updates with debounced performance (300ms)
- Theme-aware color system that works in light and dark modes
- Command to toggle highlighting on/off: `Dingo: Toggle Generated Code Highlighting`

#### Enhanced Dingo Syntax Highlighting
- Special highlighting for error messages in `expr? "custom message"` syntax
- Muted/grayed colors for generated variables (`__err0`, `__tmp0`)
- Improved highlighting for `Result<T,E>` and `Option<T>` type parameters
- Distinct constructor highlighting for `Ok()`, `Err()`, `Some()`, `None()`
- More visually distinct `?` error propagation operator
- Better type parameter detection with proper scope names

#### Golden File Support
- Language association for `.go.golden` test files
- Full Dingo syntax highlighting for golden files
- Side-by-side comparison command: `Dingo: Compare with Source File`
- Keyboard shortcut: `Ctrl+Shift+D` (Windows/Linux) or `Cmd+Shift+D` (Mac)
- Automatic file pairing (`.dingo` ↔ `.go.golden`)

#### Configuration Options
- `dingo.highlightGeneratedCode` - Enable/disable generated code highlighting
- `dingo.generatedCodeStyle` - Choose highlighting style (subtle/bold/outline/disabled)
- `dingo.generatedCodeColor` - Customize background color (hex with alpha)
- `dingo.generatedCodeBorderColor` - Customize border color (hex with alpha)

### Changed
- Grammar patterns now include generated variable detection
- Result/Option type highlighting uses proper begin/end patterns for type parameters
- Error propagation operator now has conditional pattern for message detection
- Improved TextMate scope naming for better theme compatibility

### Technical
- Added TypeScript modules: `markerDetector.ts`, `decoratorManager.ts`, `config.ts`, `goldenFileSupport.ts`
- Implemented debounced document change handling
- Added proper cleanup and disposal of resources
- Enhanced package.json with new commands and keybindings
- Created build script for grammar compilation from YAML

## [0.1.0] - 2025-11-15

### Added
- Initial release
- Basic syntax highlighting for `.dingo` files
- Support for Dingo language features:
  - `Result<T, E>` and `Option<T>` types
  - Error propagation operator `?`
  - Pattern matching with `match` expressions
  - Lambda functions with `|params| expr` syntax
  - Enums and sum types
  - All standard Go syntax
- Bracket matching for `{}`, `[]`, `()`, and `<>`
- Auto-closing pairs for brackets and quotes
- Comment support with `//` and `/* */`
- Code folding for regions and blocks
- Language configuration with proper indentation rules
- TextMate grammar with comprehensive scope definitions

### Technical
- Built on TextMate grammar system
- YAML-based grammar source with JSON compilation
- VS Code language extension structure
- TypeScript-based extension activation

---

## Development Notes

### Version Numbering
- **0.x.y** - Pre-release versions during active development
- **1.0.0** - First stable release (when Dingo reaches v1.0)
- **Patch (0.0.x)** - Bug fixes and minor improvements
- **Minor (0.x.0)** - New features and enhancements
- **Major (x.0.0)** - Breaking changes or major milestones

### Release Process
1. Update version in `package.json`
2. Add entry to this CHANGELOG
3. Test all features in Extension Development Host
4. Build VSIX: `npm run package`
5. Test VSIX installation
6. Commit and tag: `git tag v0.2.0`
7. Push to repository

### Planned Features
- [ ] Hover tooltips showing original Dingo code for generated blocks
- [ ] Code lens showing percentage of generated vs original code
- [ ] Folding providers for generated code blocks
- [ ] Type-specific highlighting colors for different marker types
- [ ] Integration with Dingo LSP when available
- [ ] Snippet library for common Dingo patterns
- [ ] Automatic formatting on save
- [ ] Go to definition for Dingo symbols
- [ ] Find all references across .dingo files
