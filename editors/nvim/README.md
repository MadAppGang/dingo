# dingo.nvim

Neovim plugin for the [Dingo](https://github.com/MadAppGang/dingo) programming language - a meta-language for Go with enhanced type safety and modern syntax.

## Features

- **LSP Integration**: Full language server support via `dingo-lsp` (wraps `gopls` with source map translation)
- **Tree-sitter Syntax**: Superior syntax highlighting for Dingo-specific features (enum, match, `?` operator, lambdas)
- **Format on Save**: Automatic formatting with `dingo fmt`
- **Lint Integration**: Real-time diagnostics with `dingo lint`
- **Build Commands**: `:DingoBuild` and `:DingoRun` for compilation and execution
- **Health Checks**: `:checkhealth dingo` to verify setup
- **Mason Support**: Automatic tool installation via mason.nvim

## Installation

### Using lazy.nvim (Recommended)

#### Basic Installation

```lua
{
  "MadAppGang/dingo",
  subdir = "editors/nvim",  -- Required: plugin is in subdirectory
  ft = "dingo",
  config = function()
    require("dingo").setup()
  end,
}
```

> **Note**: The `subdir` option is required because the Neovim plugin lives in `editors/nvim/` within the main Dingo repository.

#### With Mason for Automatic Tool Installation

```lua
-- Install Mason and mason-lspconfig first
{
  "williamboman/mason.nvim",
  opts = {},
},

{
  "williamboman/mason-lspconfig.nvim",
  opts = {
    ensure_installed = { "gopls" },  -- gopls is required by dingo-lsp
  },
},

-- Then the Dingo plugin
{
  "MadAppGang/dingo",
  subdir = "editors/nvim",
  ft = "dingo",
  dependencies = {
    "williamboman/mason.nvim",
  },
  config = function()
    require("dingo").setup()
  end,
}
```

#### Full-Featured Setup with lazy.nvim

```lua
return {
  -- Mason for tool management
  {
    "williamboman/mason.nvim",
    lazy = false,
    config = true,
  },

  {
    "williamboman/mason-lspconfig.nvim",
    dependencies = { "williamboman/mason.nvim" },
    opts = {
      ensure_installed = { "gopls" },
      automatic_installation = true,
    },
  },

  -- Dingo language support
  {
    "MadAppGang/dingo",
    subdir = "editors/nvim",
    ft = "dingo",
    dependencies = {
      "williamboman/mason.nvim",
      "nvim-treesitter/nvim-treesitter",  -- Optional: for tree-sitter support
    },
    config = function()
      require("dingo").setup({
        lsp = {
          enabled = true,
          log_level = "info",
        },
        format = {
          enabled = true,
          on_save = true,
        },
        lint = {
          enabled = true,
          on_save = true,
        },
      })
    end,
    keys = {
      { "<leader>db", "<cmd>DingoBuild<cr>", desc = "Dingo Build" },
      { "<leader>dr", "<cmd>DingoRun<cr>", desc = "Dingo Run" },
      { "<leader>df", "<cmd>DingoFormat<cr>", desc = "Dingo Format" },
      { "<leader>dl", "<cmd>DingoLint<cr>", desc = "Dingo Lint" },
      { "<leader>dt", "<cmd>DingoTranspile<cr>", desc = "Dingo Transpile" },
    },
  },

  -- Optional: nvim-treesitter configuration
  {
    "nvim-treesitter/nvim-treesitter",
    opts = function(_, opts)
      opts.ensure_installed = opts.ensure_installed or {}
      vim.list_extend(opts.ensure_installed, { "go", "dingo" })
    end,
  },
}
```

#### Using Bundled Lazy Spec

The plugin includes a ready-to-use lazy.nvim spec with full configuration:

```lua
-- In your lazy.nvim setup (init.lua or lua/config/lazy.lua)
require("lazy").setup({
  {
    "MadAppGang/dingo",
    subdir = "editors/nvim",
    import = "dingo.lazy",  -- Import full spec with keybindings and Mason
  },
  -- your other plugins...
})
```

This automatically sets up:
- Dingo plugin with sensible defaults
- Keybindings (`<leader>db`, `<leader>dr`, `<leader>df`, `<leader>dl`, `<leader>dt`)
- Mason integration for gopls
- Tree-sitter configuration

#### LazyVim Distribution

If you're using [LazyVim](https://www.lazyvim.org/), add this to your plugins:

```lua
-- lua/plugins/dingo.lua
return {
  {
    "MadAppGang/dingo",
    subdir = "editors/nvim",
    ft = "dingo",
    opts = {
      lsp = { enabled = true },
      format = { on_save = true },
      lint = { on_save = true },
    },
    keys = {
      { "<leader>cB", "<cmd>DingoBuild<cr>", desc = "Build (Dingo)" },
      { "<leader>cR", "<cmd>DingoRun<cr>", desc = "Run (Dingo)" },
      { "<leader>cT", "<cmd>DingoTranspile<cr>", desc = "Transpile (Dingo)" },
    },
  },

  -- Ensure gopls is installed via Mason
  {
    "williamboman/mason.nvim",
    opts = function(_, opts)
      opts.ensure_installed = opts.ensure_installed or {}
      vim.list_extend(opts.ensure_installed, { "gopls" })
    end,
  },
}
```

### Using packer.nvim

```lua
use {
  "MadAppGang/dingo",
  rtp = "editors/nvim",  -- Required: plugin is in subdirectory
  ft = "dingo",
  requires = {
    "williamboman/mason.nvim",
    "williamboman/mason-lspconfig.nvim",
  },
  config = function()
    require("dingo").setup()
  end,
}
```

### Using vim-plug

vim-plug doesn't support subdirectory plugins well. Use the `rtp` option:

```vim
Plug 'williamboman/mason.nvim'
Plug 'williamboman/mason-lspconfig.nvim'
Plug 'MadAppGang/dingo', { 'for': 'dingo', 'rtp': 'editors/nvim' }

lua << EOF
require("mason").setup()
require("mason-lspconfig").setup({
  ensure_installed = { "gopls" },
})
require("dingo").setup()
EOF
```

> **Note**: If `rtp` doesn't work, consider using lazy.nvim which has better subdirectory support.

### Local Development Installation

For contributing or testing local changes:

```lua
-- lazy.nvim with local path
{
  dir = "~/projects/dingo/editors/nvim",  -- Your local path
  ft = "dingo",
  config = function()
    require("dingo").setup()
  end,
}
```

## Mason Integration

### Automatic gopls Installation

The `dingo-lsp` language server requires `gopls` to work. Mason can automatically install it:

```lua
require("mason").setup()
require("mason-lspconfig").setup({
  ensure_installed = { "gopls" },
  automatic_installation = true,
})
```

### Manual Dingo Tools Installation

Currently, `dingo` and `dingo-lsp` need to be installed manually via Go:

```bash
# Install dingo CLI
go install github.com/MadAppGang/dingo/cmd/dingo@latest

# Install dingo-lsp (Language Server)
go install github.com/MadAppGang/dingo/cmd/dingo-lsp@latest
```

> **Note**: Mason registry support for `dingo` and `dingo-lsp` is planned. Once available, you'll be able to install them with `:MasonInstall dingo dingo-lsp`.

### Custom Tool Paths with Mason

If Mason installs tools to a custom location, configure the paths:

```lua
require("dingo").setup({
  lsp = {
    cmd = { vim.fn.stdpath("data") .. "/mason/bin/dingo-lsp" },
  },
  format = {
    cmd = { vim.fn.stdpath("data") .. "/mason/bin/dingo", "fmt", "-w" },
  },
  lint = {
    cmd = { vim.fn.stdpath("data") .. "/mason/bin/dingo", "lint", "--json" },
  },
})
```

## Prerequisites

### Required Tools

| Tool | Purpose | Installation |
|------|---------|--------------|
| `dingo` | CLI for building/formatting/linting | `go install github.com/MadAppGang/dingo/cmd/dingo@latest` |
| `dingo-lsp` | Language Server | `go install github.com/MadAppGang/dingo/cmd/dingo-lsp@latest` |
| `gopls` | Go Language Server (used internally) | `go install golang.org/x/tools/gopls@latest` or `:MasonInstall gopls` |

### Environment Setup

Ensure `$GOPATH/bin` is in your `$PATH`:

```bash
# In ~/.bashrc, ~/.zshrc, or equivalent
export PATH="$PATH:$(go env GOPATH)/bin"
```

### Verify Installation

```bash
# Check all tools are available
which dingo dingo-lsp gopls

# Test each tool
dingo --version
gopls version
```

Or in Neovim:

```vim
:checkhealth dingo
```

## Configuration

### Default Configuration

```lua
require("dingo").setup({
  lsp = {
    enabled = true,
    cmd = { "dingo-lsp" },
    filetypes = { "dingo" },
    root_patterns = { "go.mod", ".git" },
    settings = {},
    log_level = "info",  -- trace, debug, info, warn, error
  },

  format = {
    enabled = true,
    on_save = true,
    timeout_ms = 2000,
    cmd = { "dingo", "fmt", "-w" },
  },

  lint = {
    enabled = true,
    on_save = true,
    on_insert_leave = false,
    cmd = { "dingo", "lint", "--json" },
    virtual_text = true,
    signs = true,
  },

  build = {
    cmd = { "dingo", "build", "--no-mascot" },
    run_cmd = { "dingo", "run" },
  },
})
```

### Common Configurations

#### Minimal (LSP only)

```lua
require("dingo").setup({
  format = { enabled = false },
  lint = { enabled = false },
})
```

#### Disable Format on Save

```lua
require("dingo").setup({
  format = { on_save = false },
})
```

#### Debug LSP Issues

```lua
require("dingo").setup({
  lsp = { log_level = "debug" },
})
```

#### Aggressive Linting

```lua
require("dingo").setup({
  lint = {
    on_save = true,
    on_insert_leave = true,  -- Lint when leaving insert mode
  },
})
```

## Commands

| Command | Description |
|---------|-------------|
| `:DingoBuild` | Build the current Dingo project |
| `:DingoBuild!` | Build only the current file |
| `:DingoRun [args]` | Run the current Dingo file with optional arguments |
| `:DingoTranspile` | Transpile to Go and open the generated `.go` file |
| `:DingoFormat` | Format the current buffer |
| `:DingoLint` | Lint the current buffer |
| `:DingoLintClear` | Clear lint diagnostics |

### Suggested Keybindings

```lua
-- In your Neovim config
vim.keymap.set("n", "<leader>db", "<cmd>DingoBuild<cr>", { desc = "Dingo Build" })
vim.keymap.set("n", "<leader>dr", "<cmd>DingoRun<cr>", { desc = "Dingo Run" })
vim.keymap.set("n", "<leader>df", "<cmd>DingoFormat<cr>", { desc = "Dingo Format" })
vim.keymap.set("n", "<leader>dl", "<cmd>DingoLint<cr>", { desc = "Dingo Lint" })
vim.keymap.set("n", "<leader>dt", "<cmd>DingoTranspile<cr>", { desc = "Dingo Transpile" })
```

## LSP Features

With `dingo-lsp` running, you get full IDE support:

- **Go to Definition**: Jump to symbol definitions (in `.dingo` or `.go` files)
- **Hover Documentation**: View type information and docs
- **Completion**: Intelligent code completion
- **Diagnostics**: Real-time error checking from `gopls`
- **Rename**: Refactor symbol names across files
- **References**: Find all usages of a symbol
- **Signature Help**: Function parameter hints

All LSP features work seamlessly because `dingo-lsp` translates positions between `.dingo` and `.go` files using source maps (`.dmap` files).

## Tree-sitter

The plugin includes a tree-sitter grammar for Dingo, providing:

- Syntax highlighting for Dingo-specific features
- Code folding
- Indentation
- Text objects (future)

### Installing the Parser

If you use `nvim-treesitter`:

```lua
require("nvim-treesitter.configs").setup({
  ensure_installed = { "dingo" },  -- After parser is published
})
```

For local development:

```bash
cd /path/to/dingo/editors/nvim/tree-sitter-dingo
npm install
npm run build
```

Then in Neovim:

```vim
:TSInstallFromGrammar dingo
```

## File Type Settings

The plugin automatically configures Dingo files (`.dingo`) with:

- Comment string: `// %s`
- Indentation: Tabs (like Go)
- Tab width: 4 spaces
- Smart indent enabled

## Troubleshooting

### Check Plugin Health

```vim
:checkhealth dingo
```

This will verify:
- Neovim version (0.8+ required)
- `dingo` executable is in PATH
- `dingo-lsp` executable is in PATH
- `gopls` is installed
- Tree-sitter parser is available

### LSP Not Starting

1. Verify `dingo-lsp` is installed:
   ```bash
   which dingo-lsp
   ```

2. Check LSP logs:
   ```lua
   :lua vim.cmd('e ' .. vim.lsp.get_log_path())
   ```

3. Enable debug logging:
   ```lua
   require("dingo").setup({
     lsp = { log_level = "debug" }
   })
   ```

4. Ensure `gopls` is installed (required by `dingo-lsp`):
   ```bash
   which gopls
   # Or install via Mason
   :MasonInstall gopls
   ```

### Format Not Working

1. Verify `dingo fmt` works from command line:
   ```bash
   dingo fmt -w yourfile.dingo
   ```

2. Check file has a name (`:echo expand('%:p')`)

3. Disable and format manually:
   ```vim
   :DingoFormat
   ```

### Lint Diagnostics Not Showing

1. Verify `dingo lint` works:
   ```bash
   dingo lint --json yourfile.dingo
   ```

2. Check lint is enabled in config

3. Run manually:
   ```vim
   :DingoLint
   ```

### Tree-sitter Highlighting Not Working

1. Check parser is installed:
   ```vim
   :TSInstallInfo dingo
   ```

2. Verify tree-sitter is loaded:
   ```vim
   :lua =vim.treesitter.language.add("dingo", true)
   ```

3. Inspect parsed tree:
   ```vim
   :InspectTree
   ```

## Development

### Project Structure

```
editors/nvim/
├── ftdetect/dingo.lua       # Filetype detection
├── ftplugin/dingo.lua       # Filetype settings
├── lua/dingo/
│   ├── init.lua             # Main entry point
│   ├── config.lua           # Configuration management
│   ├── lsp.lua              # LSP client setup
│   ├── format.lua           # Formatter integration
│   ├── lint.lua             # Linter integration
│   ├── commands.lua         # User commands
│   ├── health.lua           # Health checks
│   └── treesitter.lua       # Parser registration
├── queries/dingo/           # Tree-sitter queries
│   ├── highlights.scm       # Syntax highlighting
│   ├── folds.scm            # Code folding
│   └── indents.scm          # Indentation
├── doc/dingo.txt            # Vim help documentation
└── tree-sitter-dingo/       # Tree-sitter grammar
```

### Running Tests

```bash
# Test tree-sitter grammar
cd tree-sitter-dingo
npm test

# Test in Neovim
nvim --headless -u minimal_init.lua -c "PlenaryBustedDirectory tests/ {minimal_init = 'minimal_init.lua'}"
```

## Contributing

Contributions welcome! Please:

1. Follow existing code style
2. Test with `:checkhealth dingo`
3. Update documentation
4. Add tests for new features

## License

Apache License 2.0 - see [LICENSE](../../LICENSE) for details.

## Related Projects

- [Dingo](https://github.com/MadAppGang/dingo) - The Dingo language transpiler
- [dingo-lsp](https://github.com/MadAppGang/dingo/tree/main/cmd/dingo-lsp) - Language Server Protocol implementation
- [mason.nvim](https://github.com/williamboman/mason.nvim) - Package manager for Neovim
- [tree-sitter](https://tree-sitter.github.io/tree-sitter/) - Parser generator and incremental parsing library

## Resources

- [Dingo Documentation](https://github.com/MadAppGang/dingo#readme)
- [LSP Specification](https://microsoft.github.io/language-server-protocol/)
- [Neovim LSP Guide](https://neovim.io/doc/user/lsp.html)
- [Tree-sitter Documentation](https://tree-sitter.github.io/tree-sitter/creating-parsers)
- [lazy.nvim Documentation](https://github.com/folke/lazy.nvim)
- [Mason Documentation](https://github.com/williamboman/mason.nvim)
