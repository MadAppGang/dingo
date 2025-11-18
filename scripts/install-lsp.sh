#!/bin/bash
# Install dingo-lsp to /usr/local/bin

set -e

echo "📦 Dingo LSP Installation"
echo "========================"
echo ""

# Check if in project root
if [ ! -f "cmd/dingo-lsp/main.go" ]; then
    echo "❌ Error: Must run from dingo project root"
    echo "   cd /path/to/dingo && ./scripts/install-lsp.sh"
    exit 1
fi

# Build if needed
if [ ! -f "./dingo-lsp" ]; then
    echo "⚙️  Building dingo-lsp..."
    go build -o dingo-lsp cmd/dingo-lsp/main.go
    echo "✅ Build complete"
else
    echo "✅ dingo-lsp binary found"
fi

# Create symlink
echo ""
echo "🔗 Creating symlink in /usr/local/bin..."
echo "   This requires sudo permissions"
echo ""

sudo ln -sf "$(pwd)/dingo-lsp" /usr/local/bin/dingo-lsp

echo ""
echo "✅ Installation complete!"
echo ""
echo "Symlink: /usr/local/bin/dingo-lsp -> $(pwd)/dingo-lsp"
echo ""
echo "Benefits:"
echo "  • dingo-lsp is now in your PATH"
echo "  • Always uses latest build from $(pwd)/dingo-lsp"
echo "  • Just rebuild and symlink auto-updates"
echo ""
echo "Verify installation:"
echo "  which dingo-lsp"
echo "  # Should show: /usr/local/bin/dingo-lsp"
echo ""
echo "Usage in VSCode settings:"
echo '  {"dingo.lsp.path": "dingo-lsp"}'
echo ""
