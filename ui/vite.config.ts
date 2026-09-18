/// <reference types="vitest/config" />

import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

// The devcontainer runs the API on its own port to stay clear of other projects.
const API = `http://localhost:${process.env.SKILLSHARE_API_PORT || 19420}`

// SSE endpoints need explicit Accept header to prevent Vite proxy from buffering responses.
const SSE_PROXY = { target: API, headers: { Accept: 'text/event-stream' } }

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
    // inotify events are unreliable over the devcontainer's virtiofs mount, so
    // file saves are sometimes silently missed and HMR doesn't fire. Poll for
    // changes when CHOKIDAR_USEPOLLING is set (the devcontainer sets it); the
    // host and CI keep faster native watching.
    watch:
      process.env.CHOKIDAR_USEPOLLING === 'true'
        ? { usePolling: true, interval: 300 }
        : undefined,
    proxy: {
      '/api/audit/stream': SSE_PROXY,
      '/api/update/stream': SSE_PROXY,
      '/api/check/stream': SSE_PROXY,
      '/api/diff/stream': SSE_PROXY,
      // Keep the browser's Host header: the MCP endpoints compare it against Origin,
      // and changeOrigin (the string shorthand's default) would rewrite it to the target.
      '/api': { target: API, changeOrigin: false },
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
