import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    port: 5173,
    proxy: {
      '/api': 'http://localhost:19420',
    },
  },
  build: {
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (!id.includes('node_modules')) return;
          // React core — rarely changes, cache separately
          if (id.includes('/react-dom/') || id.includes('/react/') || id.includes('/scheduler/')) {
            return 'vendor-react';
          }
          // CodeMirror ecosystem — heavy, only used in FileViewerModal + ConfigPage
          if (id.includes('@codemirror') || id.includes('@uiw/') || id.includes('@lezer/')) {
            return 'vendor-codemirror';
          }
          // Markdown ecosystem — only used in SkillDetailPage + FileViewerModal
          if (
            id.includes('react-markdown') || id.includes('remark-') ||
            id.includes('micromark') || id.includes('mdast-') ||
            id.includes('unified') || id.includes('unist-') ||
            id.includes('hast-') || id.includes('vfile') ||
            id.includes('devlop')
          ) {
            return 'vendor-markdown';
          }
        },
      },
    },
  },
})
