import { codeToHtml } from 'shiki';
import { dingoLanguage } from './dingoLanguage';
import type { TokensList, Tokens } from 'marked';

/**
 * Walk through marked tokens and replace fenced code blocks with
 * pre-rendered Shiki HTML. Supports `dingo` blocks via the custom grammar.
 */
export async function highlightMarkdownTokens(tokens: TokensList, theme = 'github-light') {
  for (const token of tokens as Tokens.Generic[]) {
    if (token.type === 'code') {
      const lang =
        typeof token.lang === 'string' && token.lang.trim() ? token.lang.trim().toLowerCase() : 'text';
      const langInput = lang === 'dingo' ? dingoLanguage : lang;
      const highlighted = await codeToHtml(token.text ?? '', {
        lang: langInput as any,
        theme,
      });
      const wrapped = `<div class="markdown-shiki-block">${highlighted}</div>`;
      token.type = 'html';
      token.text = wrapped;
      token.raw = wrapped;
    }

    if (Array.isArray((token as any).tokens)) {
      await highlightMarkdownTokens((token as any).tokens, theme);
    }

    if (Array.isArray((token as any).items)) {
      for (const item of (token as any).items) {
        if (Array.isArray(item.tokens)) {
          await highlightMarkdownTokens(item.tokens, theme);
        }
      }
    }
  }
}
