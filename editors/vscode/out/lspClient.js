"use strict";
var __createBinding = (this && this.__createBinding) || (Object.create ? (function(o, m, k, k2) {
    if (k2 === undefined) k2 = k;
    var desc = Object.getOwnPropertyDescriptor(m, k);
    if (!desc || ("get" in desc ? !m.__esModule : desc.writable || desc.configurable)) {
      desc = { enumerable: true, get: function() { return m[k]; } };
    }
    Object.defineProperty(o, k2, desc);
}) : (function(o, m, k, k2) {
    if (k2 === undefined) k2 = k;
    o[k2] = m[k];
}));
var __setModuleDefault = (this && this.__setModuleDefault) || (Object.create ? (function(o, v) {
    Object.defineProperty(o, "default", { enumerable: true, value: v });
}) : function(o, v) {
    o["default"] = v;
});
var __importStar = (this && this.__importStar) || (function () {
    var ownKeys = function(o) {
        ownKeys = Object.getOwnPropertyNames || function (o) {
            var ar = [];
            for (var k in o) if (Object.prototype.hasOwnProperty.call(o, k)) ar[ar.length] = k;
            return ar;
        };
        return ownKeys(o);
    };
    return function (mod) {
        if (mod && mod.__esModule) return mod;
        var result = {};
        if (mod != null) for (var k = ownKeys(mod), i = 0; i < k.length; i++) if (k[i] !== "default") __createBinding(result, mod, k[i]);
        __setModuleDefault(result, mod);
        return result;
    };
})();
Object.defineProperty(exports, "__esModule", { value: true });
exports.activateLSPClient = activateLSPClient;
exports.deactivateLSPClient = deactivateLSPClient;
exports.getLSPClient = getLSPClient;
const vscode = __importStar(require("vscode"));
const node_1 = require("vscode-languageclient/node");
let client = null;
async function activateLSPClient(context) {
    const config = vscode.workspace.getConfiguration('dingo');
    // Check if LSP is enabled (could add opt-out setting later)
    const lspPath = config.get('lsp.path', 'dingo-lsp');
    const logLevel = config.get('lsp.logLevel', 'info');
    const transpileOnSave = config.get('transpileOnSave', true);
    // Server options - start dingo-lsp binary
    const serverOptions = {
        command: lspPath,
        args: [],
        transport: node_1.TransportKind.stdio,
        options: {
            env: {
                ...process.env,
                DINGO_LSP_LOG: logLevel,
                DINGO_AUTO_TRANSPILE: transpileOnSave.toString(),
            }
        }
    };
    // Client options - document selector and synchronization
    const clientOptions = {
        documentSelector: [
            { scheme: 'file', language: 'dingo' }
        ],
        synchronize: {
            // Notify server of .dingo and .go.map file changes
            fileEvents: vscode.workspace.createFileSystemWatcher('**/*.{dingo,go.map}')
        },
        outputChannelName: 'Dingo Language Server',
        // Show error notifications
        errorHandler: {
            error: () => ({ action: 2 }), // Continue
            closed: () => ({ action: 1 }) // Don't restart automatically on close
        }
    };
    // Create and start the language client
    client = new node_1.LanguageClient('dingo-lsp', 'Dingo Language Server', serverOptions, clientOptions);
    try {
        await client.start();
        console.log('Dingo LSP client started successfully');
        // Show notification if gopls is not installed
        client.onNotification('window/showMessage', (params) => {
            if (params.message.includes('gopls not found')) {
                vscode.window.showErrorMessage(params.message, 'Install gopls').then(selection => {
                    if (selection === 'Install gopls') {
                        vscode.env.openExternal(vscode.Uri.parse('https://github.com/golang/tools/tree/master/gopls#installation'));
                    }
                });
            }
        });
    }
    catch (error) {
        console.error('Failed to start Dingo LSP client:', error);
        // Show helpful error message
        if (error.message.includes('ENOENT') || error.message.includes('not found')) {
            vscode.window.showErrorMessage('dingo-lsp binary not found. Please ensure dingo is installed and dingo-lsp is in your PATH.', 'Install Dingo').then(selection => {
                if (selection === 'Install Dingo') {
                    vscode.env.openExternal(vscode.Uri.parse('https://dingolang.com/docs/installation'));
                }
            });
        }
        else {
            vscode.window.showErrorMessage(`Failed to start Dingo LSP: ${error.message}`);
        }
    }
    // Register command: Transpile current file
    context.subscriptions.push(vscode.commands.registerCommand('dingo.transpileCurrentFile', async () => {
        const editor = vscode.window.activeTextEditor;
        if (!editor || editor.document.languageId !== 'dingo') {
            vscode.window.showErrorMessage('Not a Dingo file');
            return;
        }
        const filePath = editor.document.uri.fsPath;
        const terminal = vscode.window.createTerminal('Dingo Transpile');
        terminal.sendText(`dingo build ${filePath}`);
        terminal.show();
    }));
    // Register command: Transpile workspace
    context.subscriptions.push(vscode.commands.registerCommand('dingo.transpileWorkspace', async () => {
        const terminal = vscode.window.createTerminal('Dingo Transpile');
        terminal.sendText('dingo build ./...');
        terminal.show();
    }));
    // Register command: Restart LSP
    context.subscriptions.push(vscode.commands.registerCommand('dingo.restartLSP', async () => {
        if (client) {
            await client.stop();
            await client.start();
            vscode.window.showInformationMessage('Dingo LSP restarted');
        }
        else {
            vscode.window.showWarningMessage('Dingo LSP is not running');
        }
    }));
}
async function deactivateLSPClient() {
    if (client) {
        await client.stop();
        client = null;
    }
}
function getLSPClient() {
    return client;
}
//# sourceMappingURL=lspClient.js.map