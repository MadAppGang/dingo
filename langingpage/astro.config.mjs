// @ts-check
import { defineConfig } from 'astro/config';

import tailwindcss from '@tailwindcss/vite';

// https://astro.build/config
export default defineConfig({
  output: 'static',
  site: 'https://dingolang.com',
  // No base configuration needed for custom domain at root

  markdown: {
    shikiConfig: {
      theme: 'dark-plus',
      langs: ['go', 'typescript', 'javascript', 'dingo'],
    },
  },

  // Build optimizations for GitHub Pages
  vite: {
    build: {
      assetsInlineLimit: 0, // Don't inline assets for better caching
      minify: 'esbuild',    // Fast minification
      cssMinify: true,      // Minify CSS
    },

    plugins: [tailwindcss()],
  },
});