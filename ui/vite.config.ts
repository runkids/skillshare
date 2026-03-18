/// <reference types="vitest/config" />

import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

// SSE endpoints need explicit Accept header to prevent Vite proxy from buffering responses.
const SSE_PROXY = { target: 'http://localhost:19420', headers: { Accept: 'text/event-stream' } }

export default defineConfig({
  base: './',
  plugins: [react(), tailwindcss()],
  test: {
    environment: 'jsdom',
    setupFiles: './src/test/setup.ts',
  },
  resolve: {
    dedupe: ['react', 'react-dom'],
  },
  server: {
    host: true,
    port: 5173,
    proxy: {
      '/api/audit/stream': SSE_PROXY,
      '/api/update/stream': SSE_PROXY,
      '/api/check/stream': SSE_PROXY,
      '/api/diff/stream': SSE_PROXY,
      '/api': 'http://localhost:19420',
    },
  },
  build: {
    rolldownOptions: {
      output: {
        codeSplitting: {
          groups: [
            {
              name: 'vendor-react',
              test: /\/react-dom\/|\/react\/|\/scheduler\//,
              priority: 20,
            },
            {
              name: 'vendor-codemirror',
              test: /@codemirror\/(?!lang-)|@uiw\/|codemirror/,
              priority: 15,
            },
            {
              name: 'vendor-codemirror-lang',
              test: /@codemirror\/lang-|@lezer\//,
              priority: 16,
            },
            {
              name: 'vendor-markdown',
              test: /react-markdown|remark-|micromark|mdast-|unified|unist-|hast-|vfile|devlop/,
              priority: 15,
            },
            {
              name: 'vendor-tanstack-query',
              test: /@tanstack\/react-query/,
              priority: 10,
            },
          ],
        },
      },
    },
  },
})
