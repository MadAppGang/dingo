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
exports.GoldenFileSupport = void 0;
const vscode = __importStar(require("vscode"));
const path = __importStar(require("path"));
const fs = __importStar(require("fs"));
/**
 * Provides support for .go.golden test files, including side-by-side comparison
 * with corresponding .dingo source files.
 */
class GoldenFileSupport {
    /**
     * Opens a side-by-side diff view comparing a .dingo file with its .go.golden file,
     * or vice versa.
     */
    async compareWithSource() {
        const activeEditor = vscode.window.activeTextEditor;
        if (!activeEditor) {
            vscode.window.showWarningMessage('No active editor');
            return;
        }
        const currentPath = activeEditor.document.fileName;
        let sourcePath;
        let goldenPath;
        let title;
        // Determine file pair based on current file
        if (currentPath.endsWith('.dingo')) {
            sourcePath = currentPath;
            goldenPath = currentPath.replace('.dingo', '.go.golden');
            title = `${path.basename(sourcePath)} ↔ ${path.basename(goldenPath)}`;
        }
        else if (currentPath.endsWith('.go.golden')) {
            goldenPath = currentPath;
            sourcePath = currentPath.replace('.go.golden', '.dingo');
            title = `${path.basename(sourcePath)} ↔ ${path.basename(goldenPath)}`;
        }
        else {
            vscode.window.showErrorMessage('Not a Dingo or golden file. Use this command on .dingo or .go.golden files.');
            return;
        }
        // Validate that both files exist
        if (!fs.existsSync(sourcePath)) {
            vscode.window.showErrorMessage(`Source file not found: ${path.basename(sourcePath)}`);
            return;
        }
        if (!fs.existsSync(goldenPath)) {
            vscode.window.showErrorMessage(`Golden file not found: ${path.basename(goldenPath)}`);
            return;
        }
        // Open side-by-side diff
        const sourceUri = vscode.Uri.file(sourcePath);
        const goldenUri = vscode.Uri.file(goldenPath);
        try {
            await vscode.commands.executeCommand('vscode.diff', sourceUri, goldenUri, title);
        }
        catch (error) {
            vscode.window.showErrorMessage(`Failed to open comparison: ${error}`);
        }
    }
}
exports.GoldenFileSupport = GoldenFileSupport;
//# sourceMappingURL=goldenFileSupport.js.map