import * as vscode from 'vscode';
import * as path from 'path';
import {
    LanguageClient,
    LanguageClientOptions,
    ServerOptions,
    TransportKind
} from 'vscode-languageclient/node';

let client: LanguageClient | null = null;

export async function activateLSPClient(context: vscode.ExtensionContext): Promise<void> {
    const config = vscode.workspace.getConfiguration('dingo');

    // Check if LSP is enabled (could add opt-out setting later)
    const lspPath = config.get<string>('lsp.path', 'dingo-lsp');
    const logLevel = config.get<string>('lsp.logLevel', 'info');
    const transpileOnSave = config.get<boolean>('transpileOnSave', true);

    // Server options - start dingo-lsp binary
    const serverOptions: ServerOptions = {
        command: lspPath,
        args: [],
        transport: TransportKind.stdio,
        options: {
            env: {
                ...process.env,
                DINGO_LSP_LOG: logLevel,
                DINGO_AUTO_TRANSPILE: transpileOnSave.toString(),
            }
        }
    };

    // Client options - document selector and synchronization
    const clientOptions: LanguageClientOptions = {
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
            closed: () => ({ action: 1 })  // Don't restart automatically on close
        }
    };

    // Create and start the language client
    client = new LanguageClient(
        'dingo-lsp',
        'Dingo Language Server',
        serverOptions,
        clientOptions
    );

    try {
        await client.start();
        console.log('Dingo LSP client started successfully');

        // Show notification if gopls is not installed
        client.onNotification('window/showMessage', (params: any) => {
            if (params.message.includes('gopls not found')) {
                vscode.window.showErrorMessage(
                    params.message,
                    'Install gopls'
                ).then(selection => {
                    if (selection === 'Install gopls') {
                        vscode.env.openExternal(vscode.Uri.parse('https://github.com/golang/tools/tree/master/gopls#installation'));
                    }
                });
            }
        });

    } catch (error) {
        console.error('Failed to start Dingo LSP client:', error);

        // Show helpful error message
        if ((error as Error).message.includes('ENOENT') || (error as Error).message.includes('not found')) {
            vscode.window.showErrorMessage(
                'dingo-lsp binary not found. Please ensure dingo is installed and dingo-lsp is in your PATH.',
                'Install Dingo'
            ).then(selection => {
                if (selection === 'Install Dingo') {
                    vscode.env.openExternal(vscode.Uri.parse('https://dingolang.com/docs/installation'));
                }
            });
        } else {
            vscode.window.showErrorMessage(`Failed to start Dingo LSP: ${(error as Error).message}`);
        }
    }

    // Register command: Transpile current file
    context.subscriptions.push(
        vscode.commands.registerCommand('dingo.transpileCurrentFile', async () => {
            const editor = vscode.window.activeTextEditor;
            if (!editor || editor.document.languageId !== 'dingo') {
                vscode.window.showErrorMessage('Not a Dingo file');
                return;
            }

            const filePath = editor.document.uri.fsPath;
            const terminal = vscode.window.createTerminal('Dingo Transpile');
            terminal.sendText(`dingo build ${filePath}`);
            terminal.show();
        })
    );

    // Register command: Transpile workspace
    context.subscriptions.push(
        vscode.commands.registerCommand('dingo.transpileWorkspace', async () => {
            const terminal = vscode.window.createTerminal('Dingo Transpile');
            terminal.sendText('dingo build ./...');
            terminal.show();
        })
    );

    // Register command: Restart LSP
    context.subscriptions.push(
        vscode.commands.registerCommand('dingo.restartLSP', async () => {
            if (client) {
                await client.stop();
                await client.start();
                vscode.window.showInformationMessage('Dingo LSP restarted');
            } else {
                vscode.window.showWarningMessage('Dingo LSP is not running');
            }
        })
    );
}

export async function deactivateLSPClient(): Promise<void> {
    if (client) {
        await client.stop();
        client = null;
    }
}

export function getLSPClient(): LanguageClient | null {
    return client;
}
