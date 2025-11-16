import * as vscode from 'vscode';
import { MarkerDetector } from './markerDetector';
import { DecoratorManager } from './decoratorManager';
import { ConfigManager } from './config';

let decoratorManager: DecoratorManager | null = null;
let markerDetector: MarkerDetector | null = null;
let configManager: ConfigManager | null = null;

// Debounce map for file change events
const updateTimeouts = new Map<vscode.TextDocument, NodeJS.Timeout>();

export function activate(context: vscode.ExtensionContext) {
    console.log('Dingo extension activating...');

    // Initialize managers
    configManager = new ConfigManager();
    markerDetector = new MarkerDetector();
    decoratorManager = new DecoratorManager(configManager);

    // Update highlights when document opens
    context.subscriptions.push(
        vscode.workspace.onDidOpenTextDocument((document) => {
            if (shouldProcess(document)) {
                updateHighlights(document);
            }
        })
    );

    // Debounced updates on document changes
    context.subscriptions.push(
        vscode.workspace.onDidChangeTextDocument((event) => {
            if (shouldProcess(event.document)) {
                debounceUpdate(event.document);
            }
        })
    );

    // Update when active editor changes
    context.subscriptions.push(
        vscode.window.onDidChangeActiveTextEditor((editor) => {
            if (editor && shouldProcess(editor.document)) {
                updateHighlights(editor.document);
            }
        })
    );

    // Update when configuration changes
    context.subscriptions.push(
        vscode.workspace.onDidChangeConfiguration((event) => {
            if (event.affectsConfiguration('dingo')) {
                configManager?.reload();
                decoratorManager?.updateDecorationType();
                refreshAllVisibleEditors();
            }
        })
    );

    // Close decorations when document closes
    context.subscriptions.push(
        vscode.workspace.onDidCloseTextDocument((document) => {
            if (shouldProcess(document)) {
                clearDecorations(document);
            }
        })
    );

    // Command: Toggle generated code highlighting
    context.subscriptions.push(
        vscode.commands.registerCommand('dingo.toggleGeneratedCodeHighlighting', async () => {
            const config = vscode.workspace.getConfiguration('dingo');
            const current = config.get<boolean>('highlightGeneratedCode', true);
            await config.update('highlightGeneratedCode', !current, vscode.ConfigurationTarget.Global);

            const newState = !current ? 'enabled' : 'disabled';
            vscode.window.showInformationMessage(`Dingo generated code highlighting ${newState}`);
        })
    );

    // Highlight all currently open editors
    vscode.window.visibleTextEditors.forEach((editor) => {
        if (shouldProcess(editor.document)) {
            updateHighlights(editor.document);
        }
    });

    console.log('Dingo extension activated');
}

export function deactivate() {
    decoratorManager?.dispose();
    decoratorManager = null;
    markerDetector = null;
    configManager = null;

    // Clear all pending timeouts
    updateTimeouts.forEach(timeout => clearTimeout(timeout));
    updateTimeouts.clear();
}

function shouldProcess(document: vscode.TextDocument): boolean {
    // Process .go and .go.golden files
    return document.languageId === 'go' ||
           document.fileName.endsWith('.go.golden');
}

function updateHighlights(document: vscode.TextDocument) {
    if (!configManager || !markerDetector || !decoratorManager) {
        return;
    }

    // Check if highlighting is enabled
    if (!configManager.isHighlightingEnabled()) {
        clearDecorations(document);
        return;
    }

    // Find marker ranges
    const ranges = markerDetector.findMarkerRanges(document);

    // Apply decorations to all visible editors showing this document
    vscode.window.visibleTextEditors
        .filter(editor => editor.document === document)
        .forEach(editor => {
            decoratorManager?.applyDecorations(editor, ranges);
        });
}

function debounceUpdate(document: vscode.TextDocument) {
    // Clear existing timeout for this document
    const existingTimeout = updateTimeouts.get(document);
    if (existingTimeout) {
        clearTimeout(existingTimeout);
    }

    // Set new timeout
    const timeout = setTimeout(() => {
        updateHighlights(document);
        updateTimeouts.delete(document);
    }, 300); // 300ms debounce

    updateTimeouts.set(document, timeout);
}

function clearDecorations(document: vscode.TextDocument) {
    vscode.window.visibleTextEditors
        .filter(editor => editor.document === document)
        .forEach(editor => {
            decoratorManager?.clearDecorations(editor);
        });
}

function refreshAllVisibleEditors() {
    vscode.window.visibleTextEditors.forEach((editor) => {
        if (shouldProcess(editor.document)) {
            updateHighlights(editor.document);
        }
    });
}
