// @ts-check
import { defineConfig } from 'astro/config';

import tailwindcss from '@tailwindcss/vite';

import react from '@astrojs/react';

// https://astro.build/config
export default defineConfig({
  output: 'static',
  site: 'https://dingolang.com',

  // No base configuration needed for custom domain at root

  markdown: {
    shikiConfig: {
      theme: 'dark-plus',
      // Removed langs config - Shiki v3 loads all bundled languages by default
      // Custom 'dingo' language can be added later with proper LanguageRegistration
    },
  },

  // Build optimizations for GitHub Pages
  vite: {
    build: {
      assetsInlineLimit: 0, // Don't inline assets for better caching
      minify: 'esbuild',    // Fast minification
      cssMinify: true,      // Minify CSS
    },

    // Configure esbuild to suppress CSS warnings from Tailwind arbitrary values
    // Tailwind's [file:line] syntax generates {file:line} CSS which esbuild
    // incorrectly flags as an unknown CSS property. This is a false positive.
    esbuild: {
      logOverride: {
        'unsupported-css-property': 'silent',
      },
    },

    plugins: [tailwindcss()],
  },

  integrations: [react()],
});