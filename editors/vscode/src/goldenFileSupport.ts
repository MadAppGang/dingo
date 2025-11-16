import * as vscode from 'vscode';
import * as path from 'path';
import * as fs from 'fs';

/**
 * Provides support for .go.golden test files, including side-by-side comparison
 * with corresponding .dingo source files.
 */
export class GoldenFileSupport {
    /**
     * Opens a side-by-side diff view comparing a .dingo file with its .go.golden file,
     * or vice versa.
     */
    public async compareWithSource(): Promise<void> {
        const activeEditor = vscode.window.activeTextEditor;
        if (!activeEditor) {
            vscode.window.showWarningMessage('No active editor');
            return;
        }

        const currentPath = activeEditor.document.fileName;
        let sourcePath: string;
        let goldenPath: string;
        let title: string;

        // Determine file pair based on current file
        if (currentPath.endsWith('.dingo')) {
            sourcePath = currentPath;
            goldenPath = currentPath.replace('.dingo', '.go.golden');
            title = `${path.basename(sourcePath)} ↔ ${path.basename(goldenPath)}`;
        } else if (currentPath.endsWith('.go.golden')) {
            goldenPath = currentPath;
            sourcePath = currentPath.replace('.go.golden', '.dingo');
            title = `${path.basename(sourcePath)} ↔ ${path.basename(goldenPath)}`;
        } else {
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
            await vscode.commands.executeCommand(
                'vscode.diff',
                sourceUri,
                goldenUri,
                title
            );
        } catch (error) {
            vscode.window.showErrorMessage(`Failed to open comparison: ${error}`);
        }
    }
}
