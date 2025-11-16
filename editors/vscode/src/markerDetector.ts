import * as vscode from 'vscode';

export interface MarkerRange {
    range: vscode.Range;
    type: string;
    context?: string;
}

export class MarkerDetector {
    private readonly startPattern = /\/\/\s*DINGO:GENERATED:START(?:\s+(\w+))?(?:\s+(.+))?$/;
    private readonly endPattern = /\/\/\s*DINGO:GENERATED:END\s*$/;

    /**
     * Find all DINGO:GENERATED marker ranges in a document
     */
    public findMarkerRanges(document: vscode.TextDocument): MarkerRange[] {
        const markers: MarkerRange[] = [];
        let inBlock = false;
        let blockStart: number | null = null;
        let blockType = 'unknown';
        let blockContext: string | undefined;

        for (let i = 0; i < document.lineCount; i++) {
            const line = document.lineAt(i);
            const text = line.text.trim();

            // Check for block start
            const startMatch = text.match(this.startPattern);
            if (startMatch && !inBlock) {
                inBlock = true;
                blockStart = i;
                blockType = startMatch[1] || 'unknown';
                blockContext = startMatch[2]?.trim();
                continue;
            }

            // Check for block end
            const endMatch = text.match(this.endPattern);
            if (endMatch && inBlock) {
                if (blockStart !== null) {
                    // Create a single range spanning all lines from start to end (inclusive)
                    const startLine = document.lineAt(blockStart);
                    const endLine = document.lineAt(i);

                    markers.push({
                        range: new vscode.Range(
                            startLine.range.start,
                            endLine.range.end
                        ),
                        type: blockType,
                        context: blockContext
                    });
                }
                inBlock = false;
                blockStart = null;
                blockType = 'unknown';
                blockContext = undefined;
            }
        }

        // Handle unclosed blocks (shouldn't happen, but be defensive)
        if (inBlock && blockStart !== null) {
            console.warn(`Unclosed DINGO:GENERATED block starting at line ${blockStart + 1}`);
        }

        return markers;
    }

    /**
     * Check if a document contains any DINGO:GENERATED markers
     */
    public hasMarkers(document: vscode.TextDocument): boolean {
        const text = document.getText();
        return text.includes('DINGO:GENERATED:START');
    }
}
