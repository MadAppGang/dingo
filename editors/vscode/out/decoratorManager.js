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
exports.DecoratorManager = void 0;
const vscode = __importStar(require("vscode"));
const config_1 = require("./config");
class DecoratorManager {
    constructor(configManager) {
        this.configManager = configManager;
        this.decorationType = this.createDecorationType();
    }
    /**
     * Update decoration type based on current configuration
     */
    updateDecorationType() {
        this.decorationType.dispose();
        this.decorationType = this.createDecorationType();
    }
    /**
     * Apply decorations to an editor
     */
    applyDecorations(editor, ranges) {
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
    clearDecorations(editor) {
        editor.setDecorations(this.decorationType, []);
    }
    /**
     * Clean up resources
     */
    dispose() {
        this.decorationType.dispose();
    }
    /**
     * Create decoration type based on current configuration
     */
    createDecorationType() {
        const style = this.configManager.getHighlightStyle();
        // If disabled, return empty decoration type
        if (style === config_1.HighlightStyle.Disabled) {
            return vscode.window.createTextEditorDecorationType({});
        }
        const bgColor = this.configManager.getBackgroundColor();
        const borderColor = this.configManager.getBorderColor();
        const options = {
            isWholeLine: true,
        };
        switch (style) {
            case config_1.HighlightStyle.Bold:
                options.backgroundColor = bgColor;
                options.border = `1px solid ${borderColor}`;
                options.borderRadius = '2px';
                break;
            case config_1.HighlightStyle.Outline:
                options.border = `1px solid ${borderColor}`;
                options.borderRadius = '2px';
                break;
            case config_1.HighlightStyle.Subtle:
            default:
                options.backgroundColor = bgColor;
                options.borderRadius = '2px';
                break;
        }
        return vscode.window.createTextEditorDecorationType(options);
    }
}
exports.DecoratorManager = DecoratorManager;
//# sourceMappingURL=decoratorManager.js.map