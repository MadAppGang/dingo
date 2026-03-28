# Dingo Language Support for JetBrains IDEs

Official Dingo language support plugin for GoLand, IntelliJ IDEA, and other JetBrains IDEs.

## Features

- **Syntax highlighting** - Full support for Dingo syntax including Result/Option types, pattern matching, lambdas, and error propagation (`?`)
- **Code completion** - Intelligent suggestions powered by the Dingo language server
- **Hover information** - View types and documentation on hover
- **Error diagnostics** - Real-time error checking and reporting
- **Go to Definition** - Navigate to symbol definitions
- **Find References** - Find all usages of a symbol

## Requirements

### 1. Install dingo-lsp

The Dingo language server must be installed and available in your PATH:

```bash
go install github.com/MadAppGang/dingo/cmd/dingo-lsp@latest
```

### 2. Install gopls

The Dingo LSP server uses gopls internally:

```bash
go install golang.org/x/tools/gopls@latest
```

### 3. Verify Installation

```bash
which dingo-lsp  # Should print path
which gopls      # Should print path
```

## Supported IDEs

| IDE | Version | LSP Support |
|-----|---------|-------------|
| GoLand | 2023.3+ | ✅ Full |
| IntelliJ IDEA Ultimate | 2023.3+ | ✅ Full |
| IntelliJ IDEA Community | 2023.3+ | Syntax only* |
| WebStorm | 2023.3+ | ✅ Full |
| PyCharm Professional | 2023.3+ | ✅ Full |
| PyCharm Community | 2023.3+ | Syntax only* |
| Rider | 2023.3+ | ✅ Full |
| CLion | 2023.3+ | ✅ Full |

*Community editions receive syntax highlighting but not LSP features (completion, hover, diagnostics) due to JetBrains licensing.

## Installation

### From JetBrains Marketplace

1. Open your IDE
2. Go to **Settings** → **Plugins** → **Marketplace**
3. Search for "Dingo Language Support"
4. Click **Install**
5. Restart your IDE

### From Disk

1. Download the latest `.zip` file from [Releases](https://github.com/MadAppGang/dingo/releases)
2. Go to **Settings** → **Plugins** → **⚙️** → **Install Plugin from Disk**
3. Select the downloaded `.zip` file
4. Restart your IDE

## Usage

1. Open any `.dingo` file
2. The Dingo language server will start automatically
3. Enjoy syntax highlighting, completion, and diagnostics!

## Troubleshooting

### LSP not starting

Check that `dingo-lsp` is in your PATH:

```bash
which dingo-lsp
```

If not found, ensure your `GOPATH/bin` is in your PATH:

```bash
export PATH=$PATH:$(go env GOPATH)/bin
```

### No completions or hover

- Ensure you're using a commercial JetBrains IDE (GoLand, IntelliJ Ultimate, etc.)
- Community editions only support syntax highlighting

### Syntax highlighting not working

1. Go to **Settings** → **Editor** → **TextMate Bundles**
2. Ensure "Dingo" is listed and enabled
3. Restart your IDE

## Development

### Requirements

- **Java 17** - Required by IntelliJ Platform Gradle Plugin 2.x
- **Gradle 8.5+** - Bundled via wrapper

On macOS with Homebrew:
```bash
brew install openjdk@17
export JAVA_HOME=/opt/homebrew/opt/openjdk@17/libexec/openjdk.jdk/Contents/Home
```

### Building from Source

```bash
cd editors/goland
./gradlew build        # Compile and test
./gradlew buildPlugin  # Create distributable zip
```

The plugin will be in `build/distributions/`.

### Running in Development

```bash
./gradlew runIde
```

This launches a sandboxed IDE with the plugin installed.

### Plugin Configuration

Uses [IntelliJ Platform Gradle Plugin 2.x](https://plugins.jetbrains.com/docs/intellij/tools-intellij-platform-gradle-plugin.html):
- Target: IntelliJ IDEA Ultimate 2024.1+
- Compatible: All JetBrains IDEs 2024.1+

## Links

- [Dingo Language](https://dingolang.com)
- [GitHub Repository](https://github.com/MadAppGang/dingo)
- [VS Code Extension](https://marketplace.visualstudio.com/items?itemName=MadAppGang.dingo)

## License

Apache License 2.0 - see [LICENSE](../../LICENSE) file.
