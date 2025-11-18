#!/bin/bash
# Quick LSP Test Script

echo "🧪 Dingo LSP Quick Test"
echo "======================="
echo ""

# Check prerequisites
echo "📋 Checking prerequisites..."

if ! command -v code &> /dev/null; then
    echo "❌ VSCode not found. Please install VSCode."
    exit 1
fi
echo "✅ VSCode found"

if ! command -v gopls &> /dev/null; then
    echo "❌ gopls not found. Installing..."
    go install golang.org/x/tools/gopls@latest

    # Ensure GOPATH/bin is in PATH
    GOPATH_BIN="$(go env GOPATH)/bin"
    if [[ ":$PATH:" != *":$GOPATH_BIN:"* ]]; then
        echo "⚙️  Adding $GOPATH_BIN to PATH..."
        export PATH="$PATH:$GOPATH_BIN"

        # Add to shell config if not already there
        SHELL_RC="${HOME}/.zshrc"
        if [ -f "$SHELL_RC" ] && ! grep -q "GOPATH/bin" "$SHELL_RC"; then
            echo "📝 Adding to $SHELL_RC for future sessions..."
            echo 'export PATH=$PATH:$(go env GOPATH)/bin' >> "$SHELL_RC"
        fi
    fi
fi
echo "✅ gopls found"

if [ ! -f "./dingo-lsp" ]; then
    echo "⚙️  Building dingo-lsp..."
    go build -o dingo-lsp cmd/dingo-lsp/main.go
fi
echo "✅ dingo-lsp built"

# Create symlink in /usr/local/bin if not already there
if [ ! -L "/usr/local/bin/dingo-lsp" ]; then
    echo "🔗 Creating symlink in /usr/local/bin..."
    sudo ln -sf "$(pwd)/dingo-lsp" /usr/local/bin/dingo-lsp
    echo "✅ Symlink created: /usr/local/bin/dingo-lsp"
fi

if [ ! -f "./editors/vscode/dingo-0.2.0.vsix" ]; then
    echo "❌ VSCode extension not found at editors/vscode/dingo-0.2.0.vsix"
    exit 1
fi
echo "✅ VSCode extension found"

echo ""
echo "🚀 Installing VSCode extension..."
code --install-extension editors/vscode/dingo-0.2.0.vsix --force

echo ""
echo "📝 Opening test files in VSCode..."
code examples/lsp-demo/pattern-matching-test.dingo

echo ""
echo "✅ Setup complete!"
echo ""
echo "📖 Next steps:"
echo "1. In VSCode, configure settings (Cmd+,):"
echo "   {\"dingo.lsp.path\": \"$(pwd)/dingo-lsp\"}"
echo ""
echo "2. Try these tests in pattern-matching-test.dingo:"
echo "   • Line 26: Hover over 'value' → should show 'value: int'"
echo "   • Line 46: Hover over 'user' → should show 'user: User'"
echo "   • Line 62: Comment out 'None' pattern → should show exhaustiveness error"
echo ""
echo "3. Full test guide: docs/MANUAL-LSP-TESTING.md"
echo ""
