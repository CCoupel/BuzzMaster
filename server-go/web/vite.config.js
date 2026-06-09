import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  base: '/',
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./src/setupTests.js'],
    alias: {
      'framer-motion': new URL('./src/mocks/framer-motion.jsx', import.meta.url).pathname,
    },
  },
  build: {
    outDir: 'dist',
    assetsDir: 'assets',
    emptyOutDir: true,
  },
  server: {
    port: 3000,
    proxy: {
      '/ws': {
        target: 'ws://localhost:80',
        ws: true,
      },
      '/api': 'http://localhost:80',
      '/questions': 'http://localhost:80',
      '/history': 'http://localhost:80',
      '/palmares': 'http://localhost:80',
      '/backup': 'http://localhost:80',
      '/config.json': 'http://localhost:80',
      '/files': 'http://localhost:80',
    }
  }
})
