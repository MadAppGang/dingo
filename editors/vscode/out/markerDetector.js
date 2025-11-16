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
exports.MarkerDetector = void 0;
const vscode = __importStar(require("vscode"));
class MarkerDetector {
    constructor() {
        this.startPattern = /\/\/\s*DINGO:GENERATED:START(?:\s+(\w+))?(?:\s+(.+))?$/;
        this.endPattern = /\/\/\s*DINGO:GENERATED:END\s*$/;
    }
    /**
     * Find all DINGO:GENERATED marker ranges in a document
     */
    findMarkerRanges(document) {
        const markers = [];
        let inBlock = false;
        let blockStart = null;
        let blockType = 'unknown';
        let blockContext;
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
                        range: new vscode.Range(startLine.range.start, endLine.range.end),
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
    hasMarkers(document) {
        const text = document.getText();
        return text.includes('DINGO:GENERATED:START');
    }
    /**
     * Find marker comment lines (just the START and END lines themselves)
     */
    findMarkerLines(document) {
        const markerLines = [];
        for (let i = 0; i < document.lineCount; i++) {
            const line = document.lineAt(i);
            const text = line.text.trim();
            // Check if this line is a marker comment
            if (text.match(this.startPattern) || text.match(this.endPattern)) {
                markerLines.push(line.range);
            }
        }
        return markerLines;
    }
}
exports.MarkerDetector = MarkerDetector;
//# sourceMappingURL=markerDetector.js.map