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
exports.ConfigManager = exports.HighlightStyle = void 0;
const vscode = __importStar(require("vscode"));
var HighlightStyle;
(function (HighlightStyle) {
    HighlightStyle["Subtle"] = "subtle";
    HighlightStyle["Bold"] = "bold";
    HighlightStyle["Outline"] = "outline";
    HighlightStyle["Disabled"] = "disabled";
})(HighlightStyle || (exports.HighlightStyle = HighlightStyle = {}));
class ConfigManager {
    constructor() {
        this.config = vscode.workspace.getConfiguration('dingo');
    }
    /**
     * Reload configuration (call when settings change)
     */
    reload() {
        this.config = vscode.workspace.getConfiguration('dingo');
    }
    /**
     * Check if highlighting is enabled
     */
    isHighlightingEnabled() {
        return this.config.get('highlightGeneratedCode', true);
    }
    /**
     * Get the highlight style
     */
    getHighlightStyle() {
        const style = this.config.get('generatedCodeStyle', 'subtle');
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
    getBackgroundColor() {
        return this.config.get('generatedCodeColor', '#3b82f620');
    }
    /**
     * Get the border color
     */
    getBorderColor() {
        return this.config.get('generatedCodeBorderColor', '#3b82f660');
    }
}
exports.ConfigManager = ConfigManager;
//# sourceMappingURL=config.js.map