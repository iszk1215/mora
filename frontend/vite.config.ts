import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vitejs.dev/config/
export default defineConfig({
  build: {
    outDir: '../server/static/public',
    emptyOutDir: true,
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
  plugins: [react()],
})
