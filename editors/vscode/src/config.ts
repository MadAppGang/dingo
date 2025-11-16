import * as vscode from 'vscode';

export enum HighlightStyle {
    Subtle = 'subtle',
    Bold = 'bold',
    Outline = 'outline',
    Disabled = 'disabled'
}

export class ConfigManager {
    private config: vscode.WorkspaceConfiguration;

    constructor() {
        this.config = vscode.workspace.getConfiguration('dingo');
    }

    /**
     * Reload configuration (call when settings change)
     */
    public reload() {
        this.config = vscode.workspace.getConfiguration('dingo');
    }

    /**
     * Check if highlighting is enabled
     */
    public isHighlightingEnabled(): boolean {
        return this.config.get<boolean>('highlightGeneratedCode', true);
    }

    /**
     * Get the highlight style
     */
    public getHighlightStyle(): HighlightStyle {
        const style = this.config.get<string>('generatedCodeStyle', 'subtle');

        switch (style) {
            case 'bold':
                return HighlightStyle.Bold;
            case 'outline':
                return HighlightStyle.Outline;
            case 'disabled':
                return HighlightStyle.Disabled;
            case 'subtle':
            default:
                return HighlightStyle.Subtle;
        }
    }

    /**
     * Get the background color
     */
    public getBackgroundColor(): string {
        return this.config.get<string>('generatedCodeColor', '#3b82f660');
    }

    /**
     * Get the border color
     */
    public getBorderColor(): string {
        return this.config.get<string>('generatedCodeBorderColor', '#3b82f680');
    }

    /**
     * Check if marker comments should be hidden
     */
    public shouldHideMarkers(): boolean {
        return this.config.get<boolean>('hideGeneratedMarkers', false);
    }
}
