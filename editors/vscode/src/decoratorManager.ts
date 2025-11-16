import * as vscode from 'vscode';
import { MarkerRange } from './markerDetector';
import { ConfigManager, HighlightStyle } from './config';

export class DecoratorManager {
    private decorationType: vscode.TextEditorDecorationType;
    private markerHideDecorationType: vscode.TextEditorDecorationType;

    constructor(private configManager: ConfigManager) {
        this.decorationType = this.createDecorationType();
        this.markerHideDecorationType = this.createMarkerHideDecorationType();
    }

    /**
     * Update decoration type based on current configuration
     */
    public updateDecorationType() {
        this.decorationType.dispose();
        this.markerHideDecorationType.dispose();
        this.decorationType = this.createDecorationType();
        this.markerHideDecorationType = this.createMarkerHideDecorationType();
    }

    /**
     * Apply decorations to an editor
     */
    public applyDecorations(editor: vscode.TextEditor, ranges: MarkerRange[], markerLines?: vscode.Range[]) {
        if (!this.configManager.isHighlightingEnabled()) {
            this.clearDecorations(editor);
            return;
        }

        // Extract just the ranges for decoration
        const decorationRanges = ranges.map(m => m.range);
        editor.setDecorations(this.decorationType, decorationRanges);

        // Apply marker hiding if enabled
        if (markerLines && this.configManager.shouldHideMarkers()) {
            editor.setDecorations(this.markerHideDecorationType, markerLines);
        } else {
            editor.setDecorations(this.markerHideDecorationType, []);
        }
    }

    /**
     * Clear all decorations from an editor
     */
    public clearDecorations(editor: vscode.TextEditor) {
        editor.setDecorations(this.decorationType, []);
        editor.setDecorations(this.markerHideDecorationType, []);
    }

    /**
     * Clean up resources
     */
    public dispose() {
        this.decorationType.dispose();
        this.markerHideDecorationType.dispose();
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

    /**
     * Create decoration type for hiding marker comments
     */
    private createMarkerHideDecorationType(): vscode.TextEditorDecorationType {
        return vscode.window.createTextEditorDecorationType({
            opacity: '0.3',
            fontStyle: 'italic',
            color: '#888888'
        });
    }
}
