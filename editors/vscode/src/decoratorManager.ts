import * as vscode from 'vscode';
import { MarkerRange } from './markerDetector';
import { ConfigManager, HighlightStyle } from './config';

export class DecoratorManager {
    private decorationType: vscode.TextEditorDecorationType;

    constructor(private configManager: ConfigManager) {
        this.decorationType = this.createDecorationType();
    }

    /**
     * Update decoration type based on current configuration
     */
    public updateDecorationType() {
        this.decorationType.dispose();
        this.decorationType = this.createDecorationType();
    }

    /**
     * Apply decorations to an editor
     */
    public applyDecorations(editor: vscode.TextEditor, ranges: MarkerRange[]) {
        if (!this.configManager.isHighlightingEnabled()) {
            this.clearDecorations(editor);
            return;
        }

        // Extract just the ranges for decoration
        const decorationRanges = ranges.map(m => m.range);
        editor.setDecorations(this.decorationType, decorationRanges);
    }

    /**
     * Clear all decorations from an editor
     */
    public clearDecorations(editor: vscode.TextEditor) {
        editor.setDecorations(this.decorationType, []);
    }

    /**
     * Clean up resources
     */
    public dispose() {
        this.decorationType.dispose();
    }

    /**
     * Create decoration type based on current configuration
     */
    private createDecorationType(): vscode.TextEditorDecorationType {
        const style = this.configManager.getHighlightStyle();

        // If disabled, return empty decoration type
        if (style === HighlightStyle.Disabled) {
            return vscode.window.createTextEditorDecorationType({});
        }

        const bgColor = this.configManager.getBackgroundColor();
        const borderColor = this.configManager.getBorderColor();

        const options: vscode.DecorationRenderOptions = {
            isWholeLine: true,
        };

        switch (style) {
            case HighlightStyle.Bold:
                options.backgroundColor = bgColor;
                options.border = `1px solid ${borderColor}`;
                options.borderRadius = '2px';
                break;

            case HighlightStyle.Outline:
                options.border = `1px solid ${borderColor}`;
                options.borderRadius = '2px';
                break;

            case HighlightStyle.Subtle:
            default:
                options.backgroundColor = bgColor;
                options.borderRadius = '2px';
                break;
        }

        return vscode.window.createTextEditorDecorationType(options);
    }
}
