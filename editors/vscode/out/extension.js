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
exports.activate = activate;
exports.deactivate = deactivate;
const vscode = __importStar(require("vscode"));
const markerDetector_1 = require("./markerDetector");
const decoratorManager_1 = require("./decoratorManager");
const config_1 = require("./config");
const goldenFileSupport_1 = require("./goldenFileSupport");
const lspClient_1 = require("./lspClient");
let decoratorManager = null;
let markerDetector = null;
let configManager = null;
// Debounce map for file change events
const updateTimeouts = new Map();
async function activate(context) {
    console.log('Dingo extension activating...');
    // Initialize managers
    configManager = new config_1.ConfigManager();
    markerDetector = new markerDetector_1.MarkerDetector();
    decoratorManager = new decoratorManager_1.DecoratorManager(configManager);
    // Activate LSP client
    await (0, lspClient_1.activateLSPClient)(context);
    // Update highlights when document opens
    context.subscriptions.push(vscode.workspace.onDidOpenTextDocument((document) => {
        if (shouldProcess(document)) {
            updateHighlights(document);
        }
    }));
    // Debounced updates on document changes
    context.subscriptions.push(vscode.workspace.onDidChangeTextDocument((event) => {
        if (shouldProcess(event.document)) {
            debounceUpdate(event.document);
        }
    }));
    // Update when active editor changes
    context.subscriptions.push(vscode.window.onDidChangeActiveTextEditor((editor) => {
        if (editor && shouldProcess(editor.document)) {
            updateHighlights(editor.document);
        }
    }));
    // Update when configuration changes
    context.subscriptions.push(vscode.workspace.onDidChangeConfiguration((event) => {
        if (event.affectsConfiguration('dingo')) {
            configManager?.reload();
            decoratorManager?.updateDecorationType();
            refreshAllVisibleEditors();
        }
    }));
    // Close decorations when document closes
    context.subscriptions.push(vscode.workspace.onDidCloseTextDocument((document) => {
        if (shouldProcess(document)) {
            clearDecorations(document);
        }
    }));
    // Command: Toggle generated code highlighting
    context.subscriptions.push(vscode.commands.registerCommand('dingo.toggleGeneratedCodeHighlighting', async () => {
        const config = vscode.workspace.getConfiguration('dingo');
        const current = config.get('highlightGeneratedCode', true);
        await config.update('highlightGeneratedCode', !current, vscode.ConfigurationTarget.Global);
        const newState = !current ? 'enabled' : 'disabled';
        vscode.window.showInformationMessage(`Dingo generated code highlighting ${newState}`);
    }));
    // Command: Compare with source/golden file
    const goldenFileSupport = new goldenFileSupport_1.GoldenFileSupport();
    context.subscriptions.push(vscode.commands.registerCommand('dingo.compareWithSource', () => {
        goldenFileSupport.compareWithSource();
    }));
    // Highlight all currently open editors
    vscode.window.visibleTextEditors.forEach((editor) => {
        if (shouldProcess(editor.document)) {
            updateHighlights(editor.document);
        }
    });
    console.log('Dingo extension activated');
}
async function deactivate() {
    // Deactivate LSP client
    await (0, lspClient_1.deactivateLSPClient)();
    decoratorManager?.dispose();
    decoratorManager = null;
    markerDetector = null;
    configManager = null;
    // Clear all pending timeouts
    updateTimeouts.forEach(timeout => clearTimeout(timeout));
    updateTimeouts.clear();
}
function shouldProcess(document) {
    // Process .go and .go.golden files
    return document.languageId === 'go' ||
        document.fileName.endsWith('.go.golden');
}
function updateHighlights(document) {
    if (!configManager || !markerDetector || !decoratorManager) {
        return;
    }
    // Check if highlighting is enabled
    if (!configManager.isHighlightingEnabled()) {
        clearDecorations(document);
        return;
    }
    // Find marker ranges, marker lines, and generated variables
    const ranges = markerDetector.findMarkerRanges(document);
    const markerLines = markerDetector.findMarkerLines(document);
    const variableRanges = markerDetector.findGeneratedVariables(document);
    // Apply decorations to all visible editors showing this document
    vscode.window.visibleTextEditors
        .filter(editor => editor.document === document)
        .forEach(editor => {
        decoratorManager?.applyDecorations(editor, ranges, markerLines, variableRanges);
    });
}
function debounceUpdate(document) {
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
function clearDecorations(document) {
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
//# sourceMappingURL=extension.js.map