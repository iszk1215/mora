import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import path from 'path'

// https://vitejs.dev/config/
export default defineConfig({
  build: {
    outDir: '../server/static/public',
    emptyOutDir: true,
  },
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
  plugins: [tailwindcss(), react()],
})
