import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import { visualizer } from 'rollup-plugin-visualizer'
import path from 'path'

// https://vitejs.dev/config/
export default defineConfig({
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  server: {
      open: true,
      port: 4000,
      proxy: {
          '/api': {
              target: 'http://localhost:4001',
          },
          '/login': {
              target: 'http://localhost:4001',
          },
          '/logout': {
              target: 'http://localhost:4001',
          },
      },
  },
  build: {
    outDir: '../server/static/public',
    emptyOutDir: true,
    // The server reads this manifest to tell content-hashed bundle output
    // (cacheable forever) apart from verbatim copies of public/ (revalidated).
    manifest: true,
    chunkSizeWarningLimit: 1300,
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (!id.includes('node_modules')) return undefined
          if (id.includes('echarts') || id.includes('zrender')) return 'echarts'
          if (id.includes('highlight.js')) return 'highlight'
          if (id.includes('react-md-editor') || id.includes('@uiw') || id.includes('@babel') || id.includes('@codemirror') || id.includes('rehype') || id.includes('hast') || id.includes('micromark') || id.includes('mdast') || id.includes('unified') || id.includes('unist') || id.includes('remark') || id.includes('vfile') || id.includes('bail') || id.includes('trough')) return 'mdeditor'
          if (id.includes('react-datepicker') || id.includes('date-fns')) return 'datepicker'
          return undefined
        },
      },
    },
  },
  plugins: [
    tailwindcss(),
    react(),
    visualizer({
      emitFile: false,
      filename: '../../frontend/stats.html',
      gzipSize: true,
    }),
  ],
})
